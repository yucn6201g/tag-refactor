package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
)

// iacArgs are the options of iac (contract C5, U8 BR1.1, v0.2.0 X1).
type iacArgs struct {
	plan, dir, tfState string
	write, pr          bool
	// prBranch, prTitle and prBodyFile are the options of the pull request
	// (v0.2.0 U2, X1); prGiven names those given on the command line.
	prBranch, prTitle, prBodyFile string
	prGiven                       []string
	// cdkApps and synthTimeout are --cdk-app and --synth-timeout (v0.2.0 U3,
	// X1). A value of --cdk-app is one folder even with a comma in it.
	cdkApps      []string
	synthTimeout string
}

// minSynthTimeout is the shortest --synth-timeout (U3 NFR6.21).
const minSynthTimeout = time.Second

// parseSynthTimeout reads --synth-timeout: a duration of Go such as 90s, 10m
// or 1h30m, of 1s or more. A number without a unit is an error.
func parseSynthTimeout(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil || d < minSynthTimeout {
		return 0, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--synth-timeout",
			DetailID: "iac.badSynthTimeout", DetailArgs: []any{s}, HintID: "iac.badSynthTimeoutFix"}
	}
	return d, nil
}

// iacRunner is how the command reaches iac. Its type has no room for an AWS
// connection or a writer of AWS: iac is put together without them, and the
// tests can see that it needs none (U8 NFR3.16).
type iacRunner func(context.Context, iac.Request) (*iac.Result, error)

const iacLong = "iac reflects the changes of a saved plan in the infrastructure code, so that applying the code later does\n" +
	"not put the old tags back. It never calls AWS, terraform, a stack update or a CDK deploy, and needs no AWS\n" +
	"credentials.\n" +
	"\n" +
	"--plan is a plan saved by plan --out. --dir is the folder of the sources: CloudFormation templates (.yaml,\n" +
	".yml, .json and .template, also in the folders below it), CDK apps (a folder with cdk.json, also below\n" +
	"it) and, directly in it, the .tf files of the root module of Terraform. --tf-state is the output of\n" +
	"terraform show -json run in that root module: Terraform resources are found by their ARN in it, and\n" +
	"without it they count as not managed by IaC. CloudFormation resources are found by their\n" +
	"aws:cloudformation tags in the plan, their logical ID and their type.\n" +
	"\n" +
	"CDK: when the plan has resources made by CloudFormation and --dir holds CDK apps, iac runs cdk synth on\n" +
	"every app (TypeScript or Python, CDK v2), one after the other in name order, to find which construct made\n" +
	"each resource; --cdk-app picks the apps to use instead (a folder relative to --dir; repeat it for more).\n" +
	"An app that fails does not stop the others. Each cdk synth stops after --synth-timeout (default 10m).\n" +
	"TypeScript apps need Node.js 18 or later and the node_modules of the app, and their sources are read with\n" +
	"the TypeScript of the app; when the sources do not show where a resource is made, the app is synthesized\n" +
	"once more with --debug to find it. Python apps need Python 3.10 or later; the .venv of the app is used\n" +
	"first. This version reflects the rename of a key written as a string in Tags.of(x).add('key', 'value') of\n" +
	"TypeScript; other ways of writing, other changes and the resources of Python apps are listed to fix by\n" +
	"hand. cdk synth runs the code of the app and the packages it loads, of npm or of Python: use iac only in\n" +
	"repositories you trust. cdk synth gets no AWS and no GitHub credentials; an app that needs AWS to synth\n" +
	"fails, with how to keep the values it looks up in cdk.context.json.\n" +
	"\n" +
	"Without --write iac only shows the difference and changes no file. With --write it first checks with git\n" +
	"that --dir is a working tree and that the files to change are committed and unchanged; then it changes\n" +
	"only the ranges of the keys and values, so comments, line breaks and everything else stay. Run terraform\n" +
	"fmt yourself if you want the = lined up. A second run finds the changes already reflected.\n" +
	"\n" +
	"--pr writes the files (it implies --write) and opens a pull request of them with git and gh: before\n" +
	"writing it checks the repository, the remote on github.com (the one the branch follows, else origin),\n" +
	"that the new branch is neither here nor on the remote, and gh auth status; after writing it makes the\n" +
	"branch from HEAD, commits the written files only (git commit --only; your hooks and signing run as\n" +
	"usual), pushes it and opens the pull request against the branch you are on. It asks nothing; the\n" +
	"working tree stays on the new branch. The URL is printed on standard output. --pr-branch names the\n" +
	"branch (default tag-refactor/iac-<date>-<time>Z, in UTC), --pr-title the title and the commit message\n" +
	"(default from the plan file; ARNs and the like become <redacted>), and --pr-body-file a UTF-8 file of up\n" +
	"to 60,000 characters put first in the body as it is. A body with a GitHub token, an AWS access key ID or\n" +
	"a URL with a password stops iac before anything is written; an ARN or a 12-digit number is told about.\n" +
	"When a step fails or is stopped, iac prints the commands that finish it by hand (for PowerShell on\n" +
	"Windows, for sh elsewhere and in Git Bash) and keeps the body in a file in the folder of git.\n" +
	"\n" +
	"These are left to fix by hand and listed with the reason: tags or values that are expressions (!Ref,\n" +
	"var.x, merge()), YAML anchors, block scalars, Terraform modules, count and for_each, tag blocks, keys not\n" +
	"in the source (they may come from a stack tag or default_tags), values that differ from the plan, and\n" +
	"removing or adding tags in a list written on one line.\n" +
	"\n" +
	"Exit codes:\n" +
	"  0 nothing to fix by hand (also when nothing had to change, or nothing is managed by IaC)\n" +
	"  1 errors: wrong input, --write stopped by the git check or a failed write, or a step of --pr failed\n" +
	"  2 some changes to fix by hand, or a CDK app that failed or was skipped"

func (a *app) iacCommand() *cobra.Command {
	var o iacArgs
	cmd := &cobra.Command{
		Use:   "iac",
		Short: "Reflect the changes of a saved plan in CloudFormation templates, CDK apps and Terraform files",
		Long:  iacLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, name := range []string{"pr-branch", "pr-title", "pr-body-file"} {
				if cmd.Flags().Changed(name) {
					o.prGiven = append(o.prGiven, "--"+name)
				}
			}
			return ensureKind(a.runIac(cmd.Context(), o))
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.plan, "plan", "", "plan saved by plan --out (required)")
	f.StringVar(&o.dir, "dir", "", "folder of the sources: the templates and CDK apps below it and the root module of Terraform directly in it (required)")
	f.StringVar(&o.tfState, "tf-state", "", "output of terraform show -json run in the root module, to find Terraform resources")
	f.BoolVar(&o.write, "write", false, "change the files, after checking with git that they are committed and unchanged")
	f.BoolVar(&o.pr, "pr", false, "change the files (--write) and open a pull request of them with git and gh, asking nothing")
	f.StringVar(&o.prBranch, "pr-branch", "", "branch of the pull request (with --pr; default tag-refactor/iac-<date>-<time>Z, in UTC)")
	f.StringVar(&o.prTitle, "pr-title", "", "title of the pull request and the commit message (with --pr; one line of up to 256 characters)")
	f.StringVar(&o.prBodyFile, "pr-body-file", "", "file put first in the body of the pull request as it is (with --pr; UTF-8, up to 60,000 characters)")
	f.StringArrayVar(&o.cdkApps, "cdk-app", nil, "folder of a CDK app to use, relative to --dir; repeat it for more (default every app below --dir)")
	f.StringVar(&o.synthTimeout, "synth-timeout", "10m", "limit of each cdk synth, such as 90s or 10m (1s or more)")
	return cmd
}

// runIac is workflow W1 of U8 and, with --pr, W2 of v0.2.0: the options, the
// check of the pull request, the writer, the result, and the pull request
// with its exit code (BR6.2, X1).
func (a *app) runIac(ctx context.Context, o iacArgs) error {
	a.started = a.deps.Now()
	switch {
	case o.plan == "":
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--plan",
			DetailID: "iac.needsPlan", HintID: "iac.needsPlanFix"}
	case o.dir == "":
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--dir",
			DetailID: "iac.needsDir", HintID: "iac.notFolderFix"}
	}
	synthTimeout, err := parseSynthTimeout(o.synthTimeout)
	if err != nil {
		return err
	}
	if !o.pr && len(o.prGiven) > 0 {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: o.prGiven[0],
			DetailID: "iac.prNeedsPr", DetailArgs: []any{o.prGiven[0]}, HintID: "iac.prNeedsPrFix"}
	}
	// Review N4: an empty value given on purpose is an error, as for
	// --pr-title, not a request for the default.
	for _, empty := range []struct {
		option, value string
	}{{"--pr-branch", o.prBranch}, {"--pr-body-file", o.prBodyFile}} {
		if empty.value == "" && slices.Contains(o.prGiven, empty.option) {
			return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: empty.option,
				DetailID: "iac.prEmptyValue", DetailArgs: []any{empty.option}, HintID: "iac.prEmptyValueFix", HintArgs: []any{empty.option}}
		}
	}
	var publisher *pullrequest.Publisher
	var plan pullrequest.Plan
	if o.pr {
		o.write = true
		publisher = a.publisher()
		if plan, err = a.checkPullRequest(ctx, publisher, o); err != nil {
			return err
		}
	}
	run := a.deps.iac
	if run == nil {
		run = iac.NewWithOptions(iac.Options{Runner: a.deps.Exec, Now: a.deps.Now}).Run
	}
	res, err := run(ctx, iac.Request{PlanPath: o.plan, Dir: o.dir, TFStatePath: o.tfState, Write: o.write, Progress: a.p,
		CDKApps: o.cdkApps, SynthTimeout: synthTimeout})
	if err != nil {
		return err
	}
	a.result = &outcome{command: "iac", resources: res.Resources}
	a.p.IacResult(res)
	code := res.ExitCode()
	if res.WriteErr != nil {
		what, fix := describe(errkind.Ensure(res.WriteErr, "iac"), a.p.Lang())
		a.p.Error(what, fix)
	}
	meaning := iacMeaning(code)
	if publisher != nil {
		code, meaning = a.publish(ctx, publisher, plan, res, code, meaning)
	}
	a.result.fixed = &fixedExit{code, meaning}
	return nil
}

// checkPullRequest checks the pull request before anything is written (BR1.1,
// BR2.1, U1 BR4.1) and shows where it goes, what the body holds that is told
// about, and whether the title was redacted (U2 NFR1.7, NFR1.8). A body with a
// shape that stops the run gets one line for each shape before the error.
func (a *app) checkPullRequest(ctx context.Context, publisher *pullrequest.Publisher, o iacArgs) (pullrequest.Plan, error) {
	req := pullrequest.CheckRequest{Dir: o.dir, Now: a.deps.Now(), BranchName: o.prBranch,
		Title: display.PullRequestTitle(a.p.Lang(), filepath.Base(o.plan))}
	if slices.Contains(o.prGiven, "--pr-title") {
		req.Title = o.prTitle
	}
	if o.prBodyFile != "" {
		// FD2-Q3: the file is named from the folder the command runs in.
		abs, err := filepath.Abs(o.prBodyFile)
		if err != nil {
			abs = o.prBodyFile
		}
		req.BodyFile = abs
	}
	plan, err := publisher.Check(ctx, req, a.p)
	if err != nil {
		var shapes *pullrequest.BodyShapesError
		if errors.As(err, &shapes) {
			a.p.BodyShapes(shapes.Findings)
		}
		return plan, err
	}
	a.p.PullRequestPlan(plan)
	a.p.BodyNotices(plan.BodyNotices)
	if plan.TitleRedacted {
		a.p.TitleRedacted()
	}
	return plan, nil
}

func iacMeaning(code int) string {
	switch code {
	case ExitError:
		return display.ExitMeaningIacFailed
	case ExitNeedsReview:
		return display.ExitMeaningIacManual
	}
	return display.ExitMeaningIacDone
}

// publish opens the pull request of the written files, when every file was
// written (BR4.2, NFR4.2), and returns the exit code: 1 when a step failed or
// was stopped or the body does not fit, otherwise the one of the result (X1:
// 1 > 2 > 0, BR4.1, BR4.2).
func (a *app) publish(ctx context.Context, publisher *pullrequest.Publisher, plan pullrequest.Plan, res *iac.Result,
	code int, meaning string) (int, string) {
	switch {
	case res.WriteErr != nil || res.Interrupted:
		a.p.PullRequestNotOpened(true)
		return code, meaning
	case len(res.WrittenFiles) == 0 && code == ExitOK:
		a.p.NothingToReflect()
		return code, meaning
	case len(res.WrittenFiles) == 0:
		a.p.PullRequestNotOpened(false)
		return code, meaning
	}
	lang := a.p.Lang()
	body, ok := display.PullRequestBody(lang, res, plan.Prefix, plan.UserBody)
	if !ok {
		a.p.Error(display.T(lang, "iac.prBodyTooLongError"), display.T(lang, "iac.prBodyTooLongFix"))
		return ExitError, display.ExitMeaningIacPRFailed
	}
	out := publisher.Publish(ctx, plan, pullrequest.PublishInput{Files: res.WrittenFiles, Title: plan.Title, Body: body}, a.p)
	if out.FailedStep != "" {
		return a.stopped(ctx, plan, out)
	}
	if out.URL == "" {
		sh := pullrequest.DefaultShell(getenv)
		a.p.PullRequestWithoutURL(pullrequest.ListCommand(plan, sh), sh)
	} else {
		a.p.PullRequestOpened(out.URL)
	}
	return code, meaning
}

// stopped shows, after a step that failed or was stopped, the error (not for a
// stop by the user, which the exit line tells), what was done, the resume
// commands and the body file (security design §5), and returns exit code 1.
func (a *app) stopped(ctx context.Context, plan pullrequest.Plan, out pullrequest.PublishOutcome) (int, string) {
	interrupted := ctx.Err() != nil
	if !interrupted {
		what, fix := describe(errkind.Ensure(out.Err, "iac"), a.p.Lang())
		a.p.Error(what, fix)
	}
	sh := pullrequest.DefaultShell(getenv)
	a.p.PullRequestStopped(out, pullrequest.ResumeCommands(plan, plan.Title, out, sh), sh)
	if interrupted {
		return ExitError, display.ExitMeaningInterrupted
	}
	return ExitError, display.ExitMeaningIacPRFailed
}

// publisher puts the pull request together: git and gh run with the
// environment of the user and, for commit and push, with the terminal.
func (a *app) publisher() *pullrequest.Publisher {
	var in io.Reader = os.Stdin
	if a.deps.Stdin != nil {
		in = a.deps.Stdin
	}
	return &pullrequest.Publisher{Runner: a.deps.Exec, TermIn: in, TermErr: a.stderr, Debug: a.p.DebugLine,
		Timeouts: a.deps.prTimeouts}
}
