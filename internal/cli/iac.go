package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/iac"
)

// iacArgs are the options of iac (contract C5, U8 BR1.1).
type iacArgs struct {
	plan, dir, tfState string
	write              bool
}

// iacRunner is how the command reaches iac. Its type has no room for an AWS
// connection or a writer of AWS: iac is put together without them, and the
// tests can see that it needs none (U8 NFR3.16).
type iacRunner func(context.Context, iac.Request) (*iac.Result, error)

const iacLong = "iac reflects the changes of a saved plan in the infrastructure code, so that applying the code later does\n" +
	"not put the old tags back. It never calls AWS, terraform or a stack update, and needs no AWS credentials.\n" +
	"\n" +
	"--plan is a plan saved by plan --out. --dir is the folder of the sources: CloudFormation templates (.yaml,\n" +
	".yml, .json and .template, also in the folders below it) and, directly in it, the .tf files of the root\n" +
	"module of Terraform. --tf-state is the output of terraform show -json run in that root module: Terraform\n" +
	"resources are found by their ARN in it, and without it they count as not managed by IaC. CloudFormation\n" +
	"resources are found by their aws:cloudformation tags in the plan, their logical ID and their type.\n" +
	"\n" +
	"Without --write iac only shows the difference and changes no file. With --write it first checks with git\n" +
	"that --dir is a working tree and that the files to change are committed and unchanged; then it changes\n" +
	"only the ranges of the keys and values, so comments, line breaks and everything else stay. Run terraform\n" +
	"fmt yourself if you want the = lined up. A second run finds the changes already reflected.\n" +
	"\n" +
	"These are left to fix by hand and listed with the reason: tags or values that are expressions (!Ref,\n" +
	"var.x, merge()), YAML anchors, block scalars, Terraform modules, count and for_each, tag blocks, keys not\n" +
	"in the source (they may come from a stack tag or default_tags), values that differ from the plan, and\n" +
	"removing or adding tags in a list written on one line. CDK and opening a pull request are not handled yet.\n" +
	"\n" +
	"Exit codes:\n" +
	"  0 nothing to fix by hand (also when nothing had to change, or nothing is managed by IaC)\n" +
	"  1 errors: wrong input, or --write stopped by the git check or a failed write\n" +
	"  2 some changes to fix by hand"

func (a *app) iacCommand() *cobra.Command {
	var o iacArgs
	cmd := &cobra.Command{
		Use:   "iac",
		Short: "Reflect the changes of a saved plan in CloudFormation templates and Terraform files",
		Long:  iacLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return ensureKind(a.runIac(cmd.Context(), o))
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.plan, "plan", "", "plan saved by plan --out (required)")
	f.StringVar(&o.dir, "dir", "", "folder of the sources: the templates below it and the root module of Terraform directly in it (required)")
	f.StringVar(&o.tfState, "tf-state", "", "output of terraform show -json run in the root module, to find Terraform resources")
	f.BoolVar(&o.write, "write", false, "change the files, after checking with git that they are committed and unchanged")
	return cmd
}

// runIac is workflow W1 of U8: the options, then the writer, then the result
// and its exit code (BR6.2).
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
	run := a.deps.iac
	if run == nil {
		run = iac.New().Run
	}
	res, err := run(ctx, iac.Request{PlanPath: o.plan, Dir: o.dir, TFStatePath: o.tfState, Write: o.write})
	if err != nil {
		return err
	}
	a.result = &outcome{command: "iac", resources: res.Resources}
	a.p.IacResult(res)
	code := res.ExitCode()
	meaning := display.ExitMeaningIacDone
	switch code {
	case ExitError:
		meaning = display.ExitMeaningIacFailed
	case ExitNeedsReview:
		meaning = display.ExitMeaningIacManual
	}
	if res.WriteErr != nil {
		what, fix := describe(errkind.Ensure(res.WriteErr, "iac"), a.p.Lang())
		a.p.Error(what, fix)
	}
	a.result.fixed = &fixedExit{code, meaning}
	return nil
}
