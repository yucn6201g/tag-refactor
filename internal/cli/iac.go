package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"

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
	"CDK: when the plan has resources made by CloudFormation and --dir holds a CDK app, iac runs cdk synth on\n" +
	"the first app (TypeScript, CDK v2) to find which construct made each resource, and reads its sources with\n" +
	"the TypeScript of the app (Node.js 18 or later and the node_modules of the app are needed). This version\n" +
	"reflects the rename of a key written as a string in Tags.of(x).add('key', 'value'); other ways of writing\n" +
	"and other changes are listed to fix by hand. cdk synth runs the code of the app: use iac only in\n" +
	"repositories you trust. cdk synth gets no AWS and no GitHub credentials; an app that needs AWS to synth\n" +
	"fails, with how to keep the values it looks up in cdk.context.json.\n" +
	"\n" +
	"Without --write iac only shows the difference and changes no file. With --write it first checks with git\n" +
	"that --dir is a working tree and that the files to change are committed and unchanged; then it changes\n" +
	"only the ranges of the keys and values, so comments, line breaks and everything else stay. Run terraform\n" +
	"fmt yourself if you want the = lined up. A second run finds the changes already reflected.\n" +
	"\n" +
	"--pr writes the files (it implies --write) and opens a pull request of them with git and gh: before\n" +
	"writing it checks the repository, the remote on github.com and gh auth status; after writing it makes the\n" +
	"branch tag-refactor/iac-<date>-<time> from HEAD, commits the written files only (git commit --only; your\n" +
	"hooks and signing run as usual), pushes it and opens the pull request against the branch you are on.\n" +
	"It asks nothing; the working tree stays on the new branch. The URL is printed on standard output.\n" +
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
			return ensureKind(a.runIac(cmd.Context(), o))
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.plan, "plan", "", "plan saved by plan --out (required)")
	f.StringVar(&o.dir, "dir", "", "folder of the sources: the templates and CDK apps below it and the root module of Terraform directly in it (required)")
	f.StringVar(&o.tfState, "tf-state", "", "output of terraform show -json run in the root module, to find Terraform resources")
	f.BoolVar(&o.write, "write", false, "change the files, after checking with git that they are committed and unchanged")
	f.BoolVar(&o.pr, "pr", false, "change the files (--write) and open a pull request of them with git and gh, asking nothing")
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
	var publisher *pullrequest.Publisher
	var plan pullrequest.Plan
	if o.pr {
		o.write = true
		publisher = a.publisher()
		var err error
		plan, err = publisher.Check(ctx, pullrequest.CheckRequest{Dir: o.dir, Now: a.deps.Now(),
			Title: display.PullRequestTitle(a.p.Lang(), filepath.Base(o.plan))}, a.p)
		if err != nil {
			return err
		}
		a.p.PullRequestPlan(plan)
	}
	run := a.deps.iac
	if run == nil {
		run = iac.NewWithOptions(iac.Options{Runner: a.deps.Exec, Now: a.deps.Now}).Run
	}
	res, err := run(ctx, iac.Request{PlanPath: o.plan, Dir: o.dir, TFStatePath: o.tfState, Write: o.write, Progress: a.p})
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
// written (BR4.2, NFR4.2), and returns the exit code: 1 when a step failed,
// otherwise the one of the result (X1: 1 > 2 > 0).
func (a *app) publish(ctx context.Context, publisher *pullrequest.Publisher, plan pullrequest.Plan, res *iac.Result,
	code int, meaning string) (int, string) {
	switch {
	case res.WriteErr != nil || res.Interrupted:
		a.p.PullRequestNotOpened(true)
		return code, meaning
	case len(res.WrittenFiles) == 0:
		a.p.PullRequestNotOpened(false)
		return code, meaning
	}
	body := display.PullRequestBody(a.p.Lang(), res, plan.Prefix)
	out := publisher.Publish(ctx, plan, pullrequest.PublishInput{Files: res.WrittenFiles, Title: plan.Title, Body: body}, a.p)
	if out.FailedStep != "" {
		if ctx.Err() != nil {
			return ExitError, display.ExitMeaningInterrupted
		}
		what, fix := describe(errkind.Ensure(out.Err, "iac"), a.p.Lang())
		a.p.Error(what, fix)
		a.p.PullRequestStopped(out)
		return ExitError, display.ExitMeaningIacPRFailed
	}
	if out.URL != "" {
		a.p.PullRequestOpened(out.URL)
	}
	return code, meaning
}

// publisher puts the pull request together: git and gh run with the
// environment of the user and, for commit and push, with the terminal.
func (a *app) publisher() *pullrequest.Publisher {
	var in io.Reader = os.Stdin
	if a.deps.Stdin != nil {
		in = a.deps.Stdin
	}
	return &pullrequest.Publisher{Runner: a.deps.Exec, TermIn: in, TermErr: a.stderr, Debug: a.p.DebugLine}
}
