package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/apply"
	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// rollbackArgs are the options of rollback (contract C5, U7 BR1.1).
type rollbackArgs struct {
	run, profile, recordFrom string
	force, yes, forceUnlock  bool
}

const rollbackLong = "rollback undoes an apply: it puts the tag keys the apply changed back as they were before it, from the\n" +
	"run record apply saved. --run is the run ID that apply showed. The record is read from\n" +
	"~/.tag-refactor/runs/<account-id>/<run-id>.json, or, when apply used --record-to, from the same place\n" +
	"given to --record-from. Only the keys the apply touched are compared and written; other keys and keys\n" +
	"starting with aws: are never changed. An apply that stopped half way is undone as far as it wrote.\n" +
	"\n" +
	"A resource whose touched keys were changed after the apply is skipped and listed; --force puts it back\n" +
	"too. Resources that are gone, or would get more than 50 tags, are skipped. rollback shows a summary and\n" +
	"writes only after you type yes; --yes writes without asking (with --force too, it also overwrites the\n" +
	"resources changed after the apply). When everything is already restored it asks nothing. The result is\n" +
	"added to the record, so a second rollback of the same run finds the resources already restored.\n" +
	"\n" +
	"rollback holds the same lock as apply, so two applies or rollbacks of one account never run at once; a\n" +
	"lock left by a killed run is removed with --force-unlock.\n" +
	"\n" +
	"Exit codes:\n" +
	"  0 every resource was restored or already restored, nothing was to undo, or nothing was confirmed\n" +
	"  1 errors, failed resources, a stop, a record that is missing, of another account, or could not be\n" +
	"    saved, or a lock that is held or could not be removed\n" +
	"  2 skipped resources: changed after the apply, gone, or over 50 tags"

func (a *app) rollbackCommand() *cobra.Command {
	var o rollbackArgs
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Undo an apply from its run record, after a confirmation",
		Long:  rollbackLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return ensureKind(a.runRollback(cmd.Context(), o))
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.run, "run", "", "run ID of the apply to undo, as apply showed it (required)")
	f.BoolVar(&o.force, "force", false, "also restore the resources whose touched keys were changed after the apply")
	f.StringVar(&o.recordFrom, "record-from", "",
		"where the run record is: the --record-to of the apply, a folder or s3://<bucket>/<prefix> (default ~/.tag-refactor/runs)")
	f.BoolVar(&o.yes, "yes", false, "undo without asking; resources changed after the apply are skipped unless --force is given")
	f.BoolVar(&o.forceUnlock, "force-unlock", false,
		"remove a lock left by a run that is no longer running, then go on (check that none runs first)")
	f.StringVar(&o.profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	return cmd
}

// rollbackReaders are what rollback connects with: the account, and the
// session, from which only apply and rollback build the writer (U7 NFR3.13).
type rollbackReaders struct {
	tagReaders
	session *awsaccess.Session
}

func rollbackReadersOf(s *awsaccess.Session) rollbackReaders {
	return rollbackReaders{tagReaders: tagReadersOf(s), session: s}
}

// rollbackSink reports the progress of rollback with its own message.
type rollbackSink struct{ p *display.Printer }

func (s rollbackSink) ItemsProcessed(done, total int) { s.p.RollbackProgress(done, total) }
func (s rollbackSink) Notice(id string, args ...any)  { s.p.Notice(id, args...) }

// runRollback is workflow W1 of U7: the options and the account (steps 1
// and 2), then the rollbacker (steps 3 to 12) and the result (step 13).
func (a *app) runRollback(ctx context.Context, o rollbackArgs) error {
	a.started = a.deps.Now()
	if err := checkRunID(o.run); err != nil {
		return err
	}
	loc, err := journal.ParseLocation(o.recordFrom)
	if err != nil {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--record-from",
			DetailID: "rollback.badLocation", DetailArgs: []any{o.recordFrom}, HintID: "journal.badLocationFix", Cause: err}
	}
	home, err := userHomeDir()
	if err != nil || home == "" {
		l := a.p.Lang()
		return &errkind.Error{Kind: errkind.InvalidInput, Detail: display.T(l, "err.home"), Hint: display.T(l, "err.homeFix"), Cause: err}
	}
	conn, acc, err := connect(ctx, a, o.profile, nil, rollbackReadersOf)
	if err != nil {
		return err
	}
	a.result = &outcome{command: "rollback"}
	out, err := a.undo(ctx, o, conn, acc.AccountID, loc, home)
	if err != nil {
		if held, ok := journal.Held(err); ok {
			a.p.LocksHeld([]journal.HeldLock{held}, false)
		}
		if out != nil && out.ReleaseErr != nil {
			a.p.LockNotReleased(out.ReleaseErr)
		}
		return recordError(err, acc.AccountID)
	}
	a.result.resources = len(out.Targets)
	a.showRollback(out, rollbackCmds(out, o, loc))
	return nil
}

// checkRunID checks --run before anything else (BR1.1).
func checkRunID(run string) error {
	switch {
	case run == "":
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--run",
			DetailID: "rollback.needsRun", HintID: "rollback.runFix"}
	case !journal.ValidRunID(run):
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--run",
			DetailID: "rollback.badRunID", DetailArgs: []any{run}, HintID: "rollback.runFix"}
	}
	return nil
}

// undo runs the rollbacker. The AWS calls run on a context of their own: the
// first Ctrl+C (the command's ctx) only stops new writes, and a second one
// cancels the calls, as for apply (BR5.5).
func (a *app) undo(ctx context.Context, o rollbackArgs, conn rollbackReaders, accountID string, loc journal.Location,
	home string) (*apply.RollbackOutcome, error) {
	writer := awsaccess.NewWriter(conn.session)
	j := journal.New(journal.Options{Home: home, Store: writer, ToolVersion: toolVersion(), Now: a.deps.Now, Warn: a.p.Notice})
	r := apply.NewRollbacker(apply.Options{AWS: applyAWS{ARNTagReader: conn.session, TagWriter: writer}, Journal: j,
		Sink: rollbackSink{a.p}, Now: a.deps.Now})
	runCtx, cancelRun := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRun()
	stop := make(chan struct{})
	finished := make(chan struct{})
	defer close(finished)
	a.watchInterrupts(ctx, stop, cancelRun, finished)
	req := apply.RollbackRequest{RunID: o.run, AccountID: accountID, Location: loc, Force: o.force, ForceUnlock: o.forceUnlock,
		OnLocksRemoved: func(locks []journal.HeldLock) { a.p.LocksHeld(locks, true) },
		Confirm:        func(s apply.RollbackSummary) (bool, error) { return a.confirmRollback(ctx, o, s) }}
	return r.Run(runCtx, req, stop)
}

// confirmRollback shows the summary and asks for yes (BR4.1 to BR4.4). It is
// asked only when something is to be written back, so standard input is
// checked only then (BR4.3).
func (a *app) confirmRollback(ctx context.Context, o rollbackArgs, s apply.RollbackSummary) (bool, error) {
	if !o.yes && !a.stdinIsTerminal() {
		return false, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
			DetailID: "rollback.notTerminal", HintID: "err.notTerminalFix"}
	}
	a.p.RollbackSummary(s)
	if o.yes {
		return true, nil
	}
	a.p.RollbackPrompt()
	ok := a.readYes(ctx)
	if !ok {
		a.p.RollbackCancelled()
	}
	return ok, nil
}

// recordError adds to the errors about the record what to do about them
// (BR1.2): a missing record names the account, and a record of another
// version says which tag-refactor reads it.
func recordError(err error, accountID string) error {
	e, ok := errkind.As(err)
	if !ok {
		return err
	}
	switch e.Kind {
	case errkind.NotFound:
		c := *e
		c.DetailID, c.DetailArgs = "rollback.recordMissing", []any{accountID}
		c.HintID, c.HintArgs = "rollback.recordMissingFix", nil
		return &c
	case errkind.UnsupportedVersion:
		c := *e
		c.HintID = "rollback.recordVersionFix"
		return &c
	}
	return err
}

// rollbackCmds are the commands the result shows (BR7.1): --force when
// resources changed after the apply were skipped, and the same rollback again
// after a stop or a resource that was not restored.
func rollbackCmds(out *apply.RollbackOutcome, o rollbackArgs, loc journal.Location) display.RollbackCommands {
	base := rollbackCommand(o.run, loc, o.profile)
	var cmds display.RollbackCommands
	for _, t := range out.Skipped {
		if t.Reason == apply.ReasonChangedSinceApply && !o.force {
			cmds.Force = base + " --force"
		}
	}
	again := out.Started && (out.Stop != apply.StopNone || out.NotWritten > 0 || out.SaveErr != nil)
	for _, t := range out.Failed {
		again = again || t.Reason == apply.ReasonNotRestored || t.Reason == apply.ReasonResultUnknown
	}
	if again {
		cmds.Again = base
	}
	return cmds
}

// showRollback prints the result and decides the exit code (BR7.2).
func (a *app) showRollback(out *apply.RollbackOutcome, cmds display.RollbackCommands) {
	code := out.ExitCode()
	meaning := display.ExitMeaningRestored
	switch {
	case out.NothingToUndo:
		a.p.RollbackNothing()
		meaning = display.ExitMeaningNothingToUndo
	case out.Cancelled:
		meaning = display.ExitMeaningNotUndone
	default:
		a.p.RollbackResult(out, cmds)
	}
	if (out.NothingToUndo || out.Cancelled) && out.ReleaseErr != nil {
		a.p.LockNotReleased(out.ReleaseErr)
	}
	switch code {
	case ExitError:
		meaning = display.ExitMeaningRollbackFailed
	case ExitNeedsReview:
		meaning = display.ExitMeaningRollbackSkipped
	}
	a.result.fixed = &fixedExit{code, meaning}
}
