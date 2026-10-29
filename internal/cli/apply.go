package cli

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yucn6201g/tag-refactor/internal/apply"
	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// applyArgs are the options of apply (contract C5, U6 BR1.1).
type applyArgs struct {
	plan, rules, profile, recordTo string
	yes, allowImpact, forceUnlock  bool
}

const applyLong = "apply rewrites the tags of a plan: one of --plan and --rules is needed. With --plan it reads a plan\n" +
	"saved by plan --out; with --rules it makes the plan now, as plan does, without saving it. It shows a\n" +
	"summary, with the changes that affect IAM or resource policies marked ! (affected) and ? (cannot decide),\n" +
	"and writes only after you type yes. --yes writes without asking when no change has impact; with impact\n" +
	"it stops (exit code 2) unless --allow-impact is given too. Resources whose tags changed after the plan,\n" +
	"or that would get more than 50 tags, are skipped; keys starting with aws: are never changed.\n" +
	"\n" +
	"Before the first write, apply saves a run record to ~/.tag-refactor/runs/<account-id>/<run-id>.json\n" +
	"(or under --record-to), which rollback uses to undo the run, and holds a lock there so that two applies\n" +
	"or rollbacks of one account never run at once; a lock left by a killed run is removed with --force-unlock.\n" +
	"\n" +
	"Exit codes:\n" +
	"  0 every resource was written, or nothing was confirmed\n" +
	"  1 errors, failed resources, a stop, a record or lock that could not be saved or removed, or a plan\n" +
	"    whose collection was incomplete\n" +
	"  2 skipped resources, conflicts or required tags without a value in the plan, or --yes stopped on impact"

func (a *app) applyCommand() *cobra.Command {
	var o applyArgs
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Rewrite the tags of a plan after a confirmation, keeping a record to undo it",
		Long:  applyLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return ensureKind(a.runApply(cmd.Context(), o))
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.plan, "plan", "", "plan file saved by plan --out (one of --plan and --rules)")
	f.StringVar(&o.rules, "rules", "", "rule file to make the plan from now (one of --plan and --rules)")
	f.BoolVar(&o.yes, "yes", false, "write without asking when no change has impact")
	f.BoolVar(&o.allowImpact, "allow-impact", false, "with --yes, also write changes that have impact")
	f.StringVar(&o.recordTo, "record-to", "",
		"where the run record goes: a folder, or s3://<bucket>/<prefix> (default ~/.tag-refactor/runs)")
	f.BoolVar(&o.forceUnlock, "force-unlock", false,
		"remove a lock left by a run that is no longer running, then go on (check that none runs first)")
	f.StringVar(&o.profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	return cmd
}

// applyReaders are the readers apply connects with: those of plan, for
// --rules, and the session, from which only apply builds the writer (U6
// NFR3.10).
type applyReaders struct {
	readers
	session *awsaccess.Session
}

func applyReadersOf(s *awsaccess.Session) applyReaders {
	return applyReaders{readers: allReadersOf(s), session: s}
}

// applyAWS joins the reads by ARN and the tag writer for the applier.
type applyAWS struct {
	awsaccess.ARNTagReader
	awsaccess.TagWriter
}

// userHomeDir finds the home folder of the records and the lock (BR5.1); a
// variable for the tests.
var userHomeDir = os.UserHomeDir

// notifyInterrupts subscribes to Ctrl+C and SIGTERM for the second stage of
// the stop (U6 security design §5); a variable for the tests. While apply
// holds the subscription, a second Ctrl+C no longer ends the process at once.
var notifyInterrupts = func() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return ch, func() { signal.Stop(ch) }
}

// runApply is workflow W1 of U6.
func (a *app) runApply(ctx context.Context, o applyArgs) error {
	a.started = a.deps.Now()
	l := a.p.Lang()
	if (o.plan == "") == (o.rules == "") {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
			Detail: display.T(l, "err.applyNeedsOne"), Hint: display.T(l, "err.applyNeedsOneFix")}
	}
	loc, err := journal.ParseLocation(o.recordTo)
	if err != nil {
		return err
	}
	home, err := userHomeDir()
	if err != nil || home == "" {
		return &errkind.Error{Kind: errkind.InvalidInput, Detail: display.T(l, "err.home"), Hint: display.T(l, "err.homeFix"), Cause: err}
	}
	conn, plan, err := a.applyPlan(ctx, o)
	if err != nil {
		return err
	}
	a.result = &outcome{command: "apply", resources: plan.ResourcesToChange()}
	if !plan.NeedsAnalysis() {
		a.p.ApplyNothing(plan)
		a.result.fixed = nothingToApply(plan)
		return nil
	}
	if !o.yes && !a.stdinIsTerminal() {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
			Detail: display.T(l, "err.notTerminal"), Hint: display.T(l, "err.notTerminalFix")}
	}
	if o.allowImpact && !o.yes {
		a.p.AllowImpactIgnored()
	}
	view := display.ApplyView{Record: loc.RecordDir(home, plan.AccountID)}
	if o.rules != "" {
		view.RuleFile = o.rules
	}
	if o.yes && !o.allowImpact && hasImpact(plan) {
		a.p.ApplySummary(plan, view)
		a.p.ApplyImpactStop()
		a.result.fixed = &fixedExit{ExitNeedsReview, display.ExitMeaningImpactStop}
		return nil
	}
	return a.writeTags(ctx, o, plan, conn, loc, home, view)
}

// applyPlan loads the plan of --plan and checks its account (BR1.2), or
// makes the plan of --rules (BR1.3), and connects.
func (a *app) applyPlan(ctx context.Context, o applyArgs) (applyReaders, *planner.Plan, error) {
	if o.rules != "" {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		r, plan, _, err := makePlan(ctx, cancel, a, o.rules, o.profile, nil, applyReadersOf)
		return r, plan, err
	}
	plan, err := planner.LoadPlan(o.plan)
	if err != nil {
		return applyReaders{}, nil, err
	}
	r, acc, err := connect(ctx, a, o.profile, nil, applyReadersOf)
	if err != nil {
		return applyReaders{}, nil, err
	}
	if plan.AccountID != acc.AccountID {
		return applyReaders{}, nil, &errkind.Error{Kind: errkind.AccountMismatch, Subject: o.plan,
			DetailID: "err.accountMismatch", DetailArgs: []any{plan.AccountID, acc.AccountID},
			HintID: "err.accountMismatchFix", HintArgs: []any{plan.AccountID}}
	}
	return r, plan, nil
}

// hasImpact reports whether the plan has a change with impact, counting an
// incomplete impact analysis as one (BR1.5, BR2.5).
func hasImpact(p *planner.Plan) bool {
	if !p.ImpactAnalysisComplete {
		return true
	}
	for _, c := range p.Changes() {
		if c.Impacted {
			return true
		}
	}
	return false
}

// nothingToApply is the exit of a plan without changes (BR1.4, BR6.2).
func nothingToApply(p *planner.Plan) *fixedExit {
	switch {
	case !p.InventoryComplete:
		return &fixedExit{ExitError, display.ExitMeaningResultsIncomplete}
	case len(p.Conflicts) > 0 || len(p.UnsetRequiredTags) > 0:
		return &fixedExit{ExitNeedsReview, display.ExitMeaningReviewNeeded}
	}
	return &fixedExit{ExitOK, display.ExitMeaningNoChangesNeeded}
}

// writeTags runs the applier (steps 7 to 13) and shows the result (step 14).
// The AWS calls run on a context of their own: the first Ctrl+C (the
// command's ctx) only stops new writes, and a second one cancels the calls.
func (a *app) writeTags(ctx context.Context, o applyArgs, plan *planner.Plan, conn applyReaders, loc journal.Location,
	home string, view display.ApplyView) error {
	writer := awsaccess.NewWriter(conn.session)
	j := journal.New(journal.Options{Home: home, Store: writer, ToolVersion: toolVersion(), Now: a.deps.Now, Warn: a.p.Notice})
	applier := apply.New(apply.Options{AWS: applyAWS{ARNTagReader: conn.session, TagWriter: writer}, Journal: j, Sink: a.p, Now: a.deps.Now})
	runCtx, cancelRun := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRun()
	stop := make(chan struct{})
	finished := make(chan struct{})
	defer close(finished)
	a.watchInterrupts(ctx, stop, cancelRun, finished)
	req := apply.Request{Plan: plan, AccountID: plan.AccountID, Location: loc, ForceUnlock: o.forceUnlock,
		OnLocksRemoved: func(locks []journal.HeldLock) { a.p.LocksHeld(locks, true) },
		Confirm: func() bool {
			a.p.ApplySummary(plan, view)
			if o.yes {
				return true
			}
			a.p.ApplyPrompt()
			ok := a.readYes(ctx)
			if !ok {
				a.p.ApplyCancelled()
			}
			return ok
		}}
	res, err := applier.Run(runCtx, req, stop)
	if err != nil {
		if held, ok := journal.Held(err); ok {
			a.p.LocksHeld([]journal.HeldLock{held}, false)
		}
		if res != nil && res.ReleaseErr != nil {
			a.p.LockNotReleased(res.ReleaseErr)
		}
		return err
	}
	if res.Cancelled {
		a.result.fixed = &fixedExit{ExitOK, display.ExitMeaningCancelled}
		return nil
	}
	rollback := ""
	if res.Record != "" {
		rollback = rollbackCommand(res.RunID, loc, o.profile)
	}
	a.p.ApplyResult(res, rollback)
	code := res.ExitCode(plan)
	meaning := display.ExitMeaningApplied
	switch {
	case code == ExitError:
		meaning = display.ExitMeaningApplyFailed
	case code == ExitNeedsReview && len(res.Skipped) > 0:
		meaning = display.ExitMeaningApplySkipped
	case code == ExitNeedsReview:
		meaning = display.ExitMeaningReviewNeeded
	}
	a.result.fixed = &fixedExit{code, meaning}
	return nil
}

// watchInterrupts is the signal role of apply (U6 security design §5): the
// command's ctx ending (the first Ctrl+C) or a first signal closes stop, and
// a second signal cancels the AWS calls. The command's ctx and the first
// signal come from the same Ctrl+C, so only signals count towards the
// second.
func (a *app) watchInterrupts(ctx context.Context, stop chan struct{}, cancelRun context.CancelFunc, finished <-chan struct{}) {
	subscribe := a.deps.interrupts
	if subscribe == nil {
		subscribe = notifyInterrupts
	}
	signals, unsubscribe := subscribe()
	go func() {
		defer unsubscribe()
		stopped, count := false, 0
		raise := func() {
			if !stopped {
				stopped = true
				close(stop)
			}
		}
		done := ctx.Done()
		for {
			select {
			case <-finished:
				return
			case <-done:
				done = nil
				raise()
			case <-signals:
				count++
				raise()
				if count >= 2 {
					cancelRun()
				}
			}
		}
	}()
}

// stdinIsTerminal reports whether standard input is a terminal (BR2.3).
func (a *app) stdinIsTerminal() bool {
	if a.deps.StdinIsTerminal != nil {
		return a.deps.StdinIsTerminal()
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// readYes reads one line and reports whether it is yes, without the spaces
// around it (BR2.2). The end of the input, a line without its line break and
// Ctrl+C while waiting are not yes.
func (a *app) readYes(ctx context.Context) bool {
	var in io.Reader = os.Stdin
	if a.deps.Stdin != nil {
		in = a.deps.Stdin
	}
	line := make(chan string, 1)
	go func() {
		s, err := bufio.NewReader(in).ReadString('\n')
		if err != nil {
			s = ""
		}
		line <- s
	}()
	select {
	case s := <-line:
		return strings.TrimSpace(s) == "yes"
	case <-ctx.Done():
		return false
	}
}

// rollbackCommand is the command that undoes the run (BR6.3).
func rollbackCommand(runID string, loc journal.Location, profile string) string {
	parts := []string{"tag-refactor", "rollback", "--run", runID}
	if loc.Value != "" {
		parts = append(parts, "--record-from", shellArg(loc.Value))
	}
	if profile != "" {
		parts = append(parts, "--profile", shellArg(profile))
	}
	return strings.Join(parts, " ")
}

// shellArg quotes an argument with spaces or quotes for the command shown.
func shellArg(s string) string {
	if strings.ContainsAny(s, " \t\"'") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
