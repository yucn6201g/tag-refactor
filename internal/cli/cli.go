// Package cli implements the tag-refactor command line: the scan and plan
// subcommands and their options (contract C5, BR1.7), the assembly of the
// read-only parts (BR1.5), and the mapping from error kinds to exit codes and
// two-line messages (contract C12, BR6.3). Only this package decides exit
// codes; only the display package writes output.
package cli

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Deps holds what the command line takes from its environment. The zero value
// is the production setting; tests replace individual fields.
type Deps struct {
	// Transport, when set, receives every HTTP request the AWS SDK sends,
	// including credential resolution. Tests point it at a fake AWS. Nil uses
	// the SDK's own HTTP transport.
	Transport http.RoundTripper
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
	// AttemptTimeout bounds one HTTP attempt of an AWS call. Zero means 30s.
	AttemptTimeout time.Duration
	// CallTimeout bounds one AWS API call including its retries. Zero means
	// five minutes.
	CallTimeout time.Duration
	// CredentialTimeout bounds resolving the AWS credentials. Zero means 30s.
	CredentialTimeout time.Duration
	// RetryDelay, when set, replaces the SDK's back-off delay between retry
	// attempts. Tests use it to avoid real waiting. Nil keeps the SDK default.
	RetryDelay func(attempt int) time.Duration
	// ExplorerPageSize, when set, replaces the page size of Resource
	// Explorer's ListResources (tests only). Zero keeps 500.
	ExplorerPageSize int
}

// Exit codes (contract C5, BR6.3).
const (
	ExitOK          = 0
	ExitError       = 1
	ExitNeedsReview = 2
)

// outcome is the conclusion of one command (CommandOutcome).
type outcome struct {
	command string
	// regionsMissing is true when the tagging API could not read a region
	// (BR4.1, BR5.6).
	regionsMissing bool
	// incomplete is true when the impact analysis is incomplete (plan).
	incomplete  bool
	needsReview bool
	resources   int
}

// exitCode applies BR6.3 and BR5.3: error or incomplete is 1, otherwise
// anything to review is 2, otherwise 0. An error is handled before an outcome
// exists. Never-tagged resources that could not be looked for change nothing.
func (o outcome) exitCode() (int, string) {
	switch {
	case o.regionsMissing:
		return ExitError, "results incomplete"
	case o.incomplete:
		return ExitError, "impact analysis incomplete"
	case o.needsReview && o.command == "scan":
		return ExitNeedsReview, "issues found"
	case o.needsReview:
		return ExitNeedsReview, "review needed"
	case o.command == "scan":
		return ExitOK, "no issues"
	default:
		return ExitOK, "no changes needed"
	}
}

type app struct {
	deps    Deps
	p       *display.Printer
	debug   bool
	started time.Time
	result  *outcome
}

// Run executes the command line with args (without the program name) and
// returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) int {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	a := &app{deps: deps, p: display.New(stdout, stderr)}
	root := a.rootCommand()
	root.SetArgs(args)
	root.SetOut(a.p.Stdout())
	root.SetErr(a.p.Stderr())
	if err := root.ExecuteContext(ctx); err != nil {
		e, ok := errkind.As(err)
		if !ok {
			// Errors that are not classified come from parsing the command line.
			e = &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: err.Error(), Cause: err}
		}
		what, fix := describe(e)
		a.p.Error(what, fix)
		a.p.ExitCode(ExitError, "error")
		return ExitError
	}
	if a.result == nil {
		// Help or version output: nothing ran.
		return writeResult(a.p, ExitOK)
	}
	code, meaning := a.result.exitCode()
	a.p.Done(a.deps.Now().Sub(a.started), a.result.resources)
	a.p.ExitCode(code, meaning)
	return writeResult(a.p, code)
}

// writeResult turns a failed write of the output into exit code 1.
func writeResult(p *display.Printer, code int) int {
	if p.WriteErr() != nil {
		return ExitError
	}
	return code
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "tag-refactor",
		Short: "Find AWS tag keys that are written in different ways and plan how to unify them",
		Long: "tag-refactor reads the tags of every enabled AWS region, groups tag keys that are written in\n" +
			"different ways, counts missing required tags, and plans key renames from a rule file, showing\n" +
			"which IAM policies use the tags. It only reads from AWS.",
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRun: func(*cobra.Command, []string) {
			a.p.SetDebug(a.debug)
		},
	}
	root.PersistentFlags().BoolVar(&a.debug, "debug", false,
		"print one line per AWS API call (operation, region, time, retries) to standard error")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: err.Error(), Cause: err}
	})
	root.AddCommand(a.scanCommand(), a.planCommand())
	for _, name := range []string{"rules", "apply", "rollback", "iac"} {
		root.AddCommand(unavailableCommand(name))
	}
	return root
}

// unavailableOptions are the options of contract C5 that this version does
// not support yet; they are accepted by the parser only to say so (BR1.7).
var unavailableOptions = map[string][]string{
	"scan": {"format", "lang"},
	"plan": {"format", "out", "lang"},
}

func addUnavailableFlags(cmd *cobra.Command) {
	for _, name := range unavailableOptions[cmd.Name()] {
		cmd.Flags().String(name, "", "not available in this version")
		_ = cmd.Flags().MarkHidden(name)
	}
}

func rejectUnavailableFlags(cmd *cobra.Command) error {
	for _, name := range unavailableOptions[cmd.Name()] {
		if cmd.Flags().Changed(name) {
			return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOptionNotAvailable, Subject: "--" + name}
		}
	}
	return nil
}

func unavailableCommand(name string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(*cobra.Command, []string) error {
			return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOptionNotAvailable, Subject: name}
		},
	}
}

// regionFlagHelp is the help of --region for scan and plan (BR1.1).
const regionFlagHelp = "AWS region to read; repeat it to read several (default: every region enabled for the account)"

func (a *app) scanCommand() *cobra.Command {
	var rulesPath, profile string
	var regions []string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "List tag key variants, missing required tags and never-tagged resources (every region)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectUnavailableFlags(cmd); err != nil {
				return err
			}
			return ensureKind(a.runScan(cmd.Context(), rulesPath, profile, regions))
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "rule file (YAML); its required keys are counted as missing tags (optional)")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	addUnavailableFlags(cmd)
	return cmd
}

func (a *app) planCommand() *cobra.Command {
	var rulesPath, profile string
	var regions []string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan tag key renames from a rule file and show the IAM policies they affect",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectUnavailableFlags(cmd); err != nil {
				return err
			}
			return ensureKind(a.runPlan(cmd.Context(), rulesPath, profile, regions))
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "rule file (YAML) with the canonical keys and their aliases (required)")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	addUnavailableFlags(cmd)
	return cmd
}

// ensureKind makes sure every error leaving a command has a kind, so only
// command-line parsing errors arrive unclassified at Run.
func ensureKind(err error) error {
	if err == nil {
		return nil
	}
	return errkind.Ensure(err, "")
}
