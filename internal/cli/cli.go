// Package cli implements the tag-refactor command line: the scan, plan and
// rules suggest subcommands and their options (contract C5, BR1.7), the assembly of the
// read-only parts (BR1.5), and the mapping from error kinds to exit codes and
// two-line messages (contract C12, BR6.3). Only this package decides exit
// codes; only the display package writes output.
package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
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
	// Stdin is where apply reads the confirmation (U6 BR2.2). Nil means the
	// process standard input.
	Stdin io.Reader
	// StdinIsTerminal reports whether Stdin is a terminal (U6 BR2.3). Nil
	// asks the operating system about the process standard input.
	StdinIsTerminal func() bool

	// savingHook, when set, is called once the temporary file of plan --out
	// or rules suggest --output exists, before anything is written to it.
	// The tests of this package use it to interrupt a run while it saves.
	savingHook func()
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
	incomplete bool
	// draftTooLarge is true when the draft of rules suggest cannot be read as
	// it is (U3 BR6.7).
	draftTooLarge bool
	needsReview   bool
	resources     int
	// plan is the plan of a plan command; its exit code is planner's
	// Plan.ExitCode (U4 BR2.6), the one place that rule is written.
	plan *planner.Plan
	// saveHint is the rule file of a plan that was not saved: the last line
	// says how to save it (screen 4).
	saveHint string
}

// exitCode applies BR6.3 and BR5.3 to scan and rules suggest: error or
// incomplete is 1, otherwise anything to review is 2, otherwise 0. plan
// takes its code from Plan.ExitCode (U4 BR2.6), which the JSON output also
// carries. An error is handled before an outcome exists. Never-tagged
// resources that could not be looked for change nothing, and neither do
// remove candidates (U4 Q4). rules suggest finds nothing to review: its
// draft gives 0 even with variants, and 1 when it is incomplete or too large
// (U3 BR6.6, BR6.7). The meaning is a display.ExitMeaning identifier.
func (o outcome) exitCode() (int, string) {
	if o.plan != nil {
		return planExitCode(o.plan)
	}
	switch {
	case o.regionsMissing:
		return ExitError, display.ExitMeaningResultsIncomplete
	case o.incomplete:
		return ExitError, display.ExitMeaningImpactIncomplete
	case o.draftTooLarge:
		return ExitError, display.ExitMeaningDraftTooLarge
	case o.command == suggestCommandName:
		return ExitOK, display.ExitMeaningDraftReady
	case o.needsReview && o.command == "scan":
		return ExitNeedsReview, display.ExitMeaningIssuesFound
	case o.needsReview:
		return ExitNeedsReview, display.ExitMeaningReviewNeeded
	case o.command == "scan":
		return ExitOK, display.ExitMeaningNoIssues
	default:
		return ExitOK, display.ExitMeaningNoChangesNeeded
	}
}

// planExitCode is the exit code of a plan (Plan.ExitCode) with the meaning
// shown next to it.
func planExitCode(p *planner.Plan) (int, string) {
	code := p.ExitCode()
	switch {
	case code == ExitError && !p.InventoryComplete:
		return code, display.ExitMeaningResultsIncomplete
	case code == ExitError:
		return code, display.ExitMeaningImpactIncomplete
	case code == ExitNeedsReview:
		return code, display.ExitMeaningReviewNeeded
	default:
		return code, display.ExitMeaningNoChangesNeeded
	}
}

type app struct {
	deps    Deps
	p       *display.Printer
	debug   bool
	lang    string
	started time.Time
	result  *outcome
}

// closeDisplay closes the display, which puts the terminal back as it was
// (NFR9.3). It is a variable so that tests can see every exit of Run use it.
var closeDisplay = (*display.Printer).Close

// getenv reads the environment for the language (BR3.8); a variable for the
// tests of this package.
var getenv = os.Getenv

// Run executes the command line with args (without the program name) and
// returns the process exit code. When ctx is cancelled (Ctrl+C) and the
// command stops with an error, whatever its kind, Run says the command was
// interrupted and returns 1 (U4 NFR design §3). A cancellation of the
// command's own (inner) context, such as the stop on missing credentials,
// leaves ctx as it is and is reported as its error.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) int {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	a := &app{deps: deps, p: display.New(stdout, stderr)}
	defer closeDisplay(a.p)
	a.p.SetLang(display.ResolveLang(getenv))
	root := a.rootCommand()
	root.SetArgs(args)
	root.SetOut(a.p.Stdout())
	root.SetErr(a.p.Stderr())
	if err := root.ExecuteContext(ctx); err != nil {
		if ctx.Err() != nil {
			a.p.Interrupted()
			a.p.ExitCode(ExitError, display.ExitMeaningInterrupted)
			return ExitError
		}
		e, ok := errkind.As(err)
		if !ok {
			// Errors that are not classified come from parsing the command line.
			e = &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: err.Error(), Cause: err}
		}
		what, fix := describe(e, a.p.Lang())
		a.p.Error(what, fix)
		a.p.ExitCode(ExitError, display.ExitMeaningError)
		return ExitError
	}
	if a.result == nil {
		// Help or version output: nothing ran.
		return writeResult(a.p, ExitOK)
	}
	code, meaning := a.result.exitCode()
	a.p.Done(a.deps.Now().Sub(a.started), a.result.resources)
	a.p.ExitCode(code, meaning)
	if a.result.saveHint != "" {
		a.p.SavePlanHint(a.result.saveHint)
	}
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
			"different ways, counts missing required tags, drafts a rule file from the tags in use, and plans\n" +
			"key renames, value changes, required tags and removals from a rule file, showing which IAM\n" +
			"policies use the tags. It only reads from AWS.",
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRunE: func(*cobra.Command, []string) error {
			a.p.SetDebug(a.debug)
			return a.applyLang()
		},
	}
	root.PersistentFlags().BoolVar(&a.debug, "debug", false,
		"print one line per AWS API call (operation, region, time, retries) to standard error")
	root.PersistentFlags().StringVar(&a.lang, "lang", "",
		"language of the output: en or ja (default: LC_ALL, LC_MESSAGES or LANG, then on Windows the display language)")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: err.Error(), Cause: err}
	})
	root.AddCommand(a.scanCommand(), a.planCommand(), a.rulesCommand())
	for _, name := range []string{"apply", "rollback", "iac"} {
		root.AddCommand(unavailableCommand(name))
	}
	return root
}

// applyLang sets the language of --lang, which wins over the environment
// (BR3.8). An unknown value is a usage error before anything else.
func (a *app) applyLang() error {
	if a.lang == "" {
		return nil
	}
	lang, ok := display.ParseLang(a.lang)
	if !ok {
		l := a.p.Lang()
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--lang",
			Detail: display.T(l, "err.badLang", a.lang), Hint: display.T(l, "err.badLangFix")}
	}
	a.p.SetLang(lang)
	return nil
}

// parseFormat checks a --format value before anything else (BR3.1).
func (a *app) parseFormat(value string, allowed []display.Format) (display.Format, error) {
	f, ok := display.ParseFormat(value, allowed)
	if !ok {
		l := a.p.Lang()
		list := display.FormatList(allowed)
		return "", &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--format",
			Detail: display.T(l, "err.badFormat", value, list), Hint: display.T(l, "err.badFormatFix", list)}
	}
	return f, nil
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
	var rulesPath, profile, format string
	var regions []string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "List tag key variants, missing required tags and never-tagged resources (every region)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.parseFormat(format, display.ScanFormats)
			if err != nil {
				return err
			}
			return ensureKind(a.runScan(cmd.Context(), rulesPath, profile, regions, f))
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "rule file (YAML); its required keys are counted as missing tags (optional)")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	cmd.Flags().StringVar(&format, "format", string(display.FormatTable),
		"format of the result on standard output: "+display.FormatList(display.ScanFormats))
	return cmd
}

func (a *app) planCommand() *cobra.Command {
	var rulesPath, profile, format, out string
	var regions []string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan tag changes from a rule file and show the IAM policies they affect",
		Long: "plan reads the tagged resources of every enabled region (or the --region values) and plans, from\n" +
			"the rule file, key renames, value changes, required tags to add and keys to remove, with the IAM\n" +
			"policies that use the tags. It changes nothing. Save the plan with --out (UTF-8 JSON); on Windows\n" +
			"PowerShell 5.1 a redirection writes UTF-16, so use --out.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.parseFormat(format, display.PlanFormats)
			if err != nil {
				return err
			}
			return ensureKind(a.runPlan(cmd.Context(), planArgs{rules: rulesPath, profile: profile, regions: regions, format: f, out: out}))
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "rule file (YAML) with the canonical keys, values, required tags and removed keys (required)")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	cmd.Flags().StringVar(&format, "format", string(display.FormatTable),
		"format of the result on standard output: "+display.FormatList(display.PlanFormats))
	cmd.Flags().StringVar(&out, "out", "",
		"save the plan to this file (JSON) for apply; an existing file is never overwritten")
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
