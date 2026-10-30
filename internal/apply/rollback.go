package apply

import (
	"context"
	"sort"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// Rollbacker runs rollback (U7 workflow W1, steps 3 to 13): it takes the lock,
// reads and checks the record of the apply, judges every resource apply
// wrote from its current tags, asks for the confirmation through the command
// line, writes the touched keys back in units read again just before, reads
// back what it wrote, and adds its result to the rollbacks of the record.
// It shares the regions, the batch write, the stops and the lock with apply.
type Rollbacker struct{ opts Options }

// NewRollbacker returns a rollbacker.
func NewRollbacker(opts Options) *Rollbacker {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Rollbacker{opts: opts}
}

// RollbackRequest is one rollback (U7 entities RollbackRequest, after the
// checks of the command line).
type RollbackRequest struct {
	RunID     string
	AccountID string
	// Location is where the record is: --record-from, or the default.
	Location journal.Location
	// Force writes back the resources changed after apply too (BR3.3).
	Force bool
	// ForceUnlock removes a lock in the way before taking it again (BR2.2).
	ForceUnlock bool
	// OnLocksRemoved is told what --force-unlock removed.
	OnLocksRemoved func([]journal.HeldLock)
	// Confirm is asked once the resources are judged and something is to be
	// written back (BR4.1 to BR4.4): it reports whether to write; its error
	// ends the run without writing (standard input is not a terminal). nil
	// writes without asking.
	Confirm func(RollbackSummary) (bool, error)
}

// RollbackSummary is what the confirmation shows (U7 entities
// RollbackSummary).
type RollbackSummary struct {
	AccountID, RunID string
	// Record is the path or the S3 URI of the record.
	Record string
	// ApplyCompleted is the completed of the record: false for an apply that
	// stopped half way.
	ApplyCompleted bool
	// UndoneAt is when an earlier rollback of the record that completed
	// started, or empty (BR4.1).
	UndoneAt string
	// The counts of the judgement: to restore, changed after apply (restored
	// too with Force), already restored, gone, over 50 tags, unreadable.
	ToRestore, ChangedSinceApply, AlreadyRestored, Gone, OverLimit, Unreadable int
	// Force is --force.
	Force bool
}

// RollbackOutcome is the end of a rollback (U7 entities RollbackOutcome).
type RollbackOutcome struct {
	RunID, RollbackID string
	// Record is where the record is, once it was read.
	Record  string
	Summary RollbackSummary
	Targets []*RollbackTarget
	// NothingToUndo is set when apply wrote nothing (BR1.4), Cancelled when
	// the confirmation was not yes; the record is then unchanged.
	NothingToUndo, Cancelled bool
	// Started is set once the result was added to the record.
	Started bool
	// Restored counts the resources written back; Unconfirmed those of them
	// the read back could not confirm. AlreadyRestored counts those found as
	// before apply.
	Restored, Unconfirmed, AlreadyRestored int
	// Skipped and Failed are the resources skipped and failed, by ARN.
	// NotWritten counts the resources a stop left unwritten.
	Skipped, Failed []*RollbackTarget
	NotWritten      int
	// Stop says why the run stopped writing, and StopErr is the error behind
	// it, if any.
	Stop    StopReason
	StopErr error
	// Completed is set when the rollback was completed in the record.
	// SaveErr is the failure of the last save.
	Completed bool
	SaveErr   error
	// ReleaseErr is the failure to remove the lock.
	ReleaseErr error
	// LogFile is the log of the skipped and failed resources, and LogErr the
	// failure to write it.
	LogFile string
	LogErr  error
	// RemovedLocks are the locks --force-unlock removed.
	RemovedLocks []journal.HeldLock
}

// ExitCode is the exit code of the rollback (BR7.2): 1 for a lock that
// could not be removed, failures, a stop or a record that could not be
// saved; 2 for skipped resources; 0 otherwise, and when apply wrote nothing
// or the confirmation was not yes.
func (o *RollbackOutcome) ExitCode() int {
	switch {
	case o.ReleaseErr != nil:
		return 1
	case o.Cancelled || o.NothingToUndo:
		return 0
	case len(o.Failed) > 0 || o.NotWritten > 0 || o.Stop != StopNone || o.SaveErr != nil:
		return 1
	case len(o.Skipped) > 0:
		return 2
	}
	return 0
}

// rollbackConductor holds the state of one rollback; ex holds what it shares
// with apply, and its lock guards the targets.
type rollbackConductor struct {
	r   *Rollbacker
	req RollbackRequest
	out *RollbackOutcome
	ex  *execution
	run *journal.RollbackRun
}

// Run runs one rollback. ctx is the context of the AWS calls: the second
// Ctrl+C cancels it. stop is closed on the first Ctrl+C: no new unit starts,
// and the calls that run finish. The error is set when the run ended before
// its result was added to the record; the outcome is set once the lock was
// taken, and the lock is always released (BR2.3).
func (r *Rollbacker) Run(ctx context.Context, req RollbackRequest, stop <-chan struct{}) (out *RollbackOutcome, err error) {
	rollbackID := journal.NewRunID(r.opts.Now(), r.opts.Rand)
	out = &RollbackOutcome{RunID: req.RunID, RollbackID: rollbackID}
	h, removed, err := takeLock(ctx, r.opts.Journal, req.AccountID, req.Location, journal.PurposeRollback, rollbackID,
		req.ForceUnlock, req.OnLocksRemoved)
	out.RemovedLocks = removed
	if err != nil {
		return out, err
	}
	c := &rollbackConductor{r: r, req: req, out: out, ex: newExecution("rollback", stop)}
	defer func() {
		cctx, cancel := closingContext(ctx)
		defer cancel()
		out.ReleaseErr = r.opts.Journal.Release(cctx, h)
	}()
	defer func() {
		if p := recover(); p != nil {
			err = &errkind.Error{Kind: errkind.Internal, Op: "rollback", Detail: "an unexpected internal error"}
			if out.Started {
				c.closeRecord(ctx)
				err = nil
				out.Stop, out.StopErr = StopInternal, &errkind.Error{Kind: errkind.Internal, Op: "rollback"}
			}
		}
	}()
	finished := make(chan struct{})
	defer close(finished)
	c.ex.watchStop(stop, finished)
	return out, c.rollback(ctx, h)
}

// rollback is steps 4 to 13 of workflow W1.
func (c *rollbackConductor) rollback(ctx context.Context, h *journal.LockHandle) error {
	run, err := c.r.opts.Journal.OpenForRollback(ctx, h, c.req.RunID)
	if err != nil {
		return err
	}
	c.run, c.out.Record = run, run.Where()
	rec := run.Record()
	if err := checkRecord(rec, c.req.AccountID, c.req.RunID, run.Where()); err != nil {
		return err
	}
	targets := newRollbackTargets(rec)
	c.out.Targets = targets
	if len(targets) == 0 {
		c.out.NothingToUndo = true
		return nil
	}
	if err := c.judgeAll(ctx, targets); err != nil {
		return err
	}
	if c.ex.st.stopped() {
		c.out.Stop, c.out.StopErr = c.ex.st.why()
		return nil
	}
	c.out.Summary = c.summary(rec)
	writes := pendingOf(targets)
	if len(writes) > 0 && c.req.Confirm != nil {
		ok, err := c.req.Confirm(c.out.Summary)
		if err != nil {
			return err
		}
		if !ok {
			c.out.Cancelled = true
			return nil
		}
	}
	if err := run.BeginRollback(ctx, c.out.RollbackID, c.r.opts.Now()); err != nil {
		return err
	}
	c.out.Started = true
	c.ex.settle(func() error { return run.RecordRollbackResults(settledResults(targets)) })
	c.writeAll(ctx, writes)
	c.closeRecord(ctx)
	c.collect()
	c.writeLog()
	return nil
}

// pendingOf returns the targets still to be written.
func pendingOf(targets []*RollbackTarget) []*RollbackTarget {
	var out []*RollbackTarget
	for _, t := range targets {
		if t.Status == RollbackPending {
			out = append(out, t)
		}
	}
	return out
}

// settledResults returns the results of the targets the judgement settled.
func settledResults(targets []*RollbackTarget) []journal.RollbackResult {
	var out []journal.RollbackResult
	for _, t := range targets {
		if t.Status != RollbackPending {
			out = append(out, t.result())
		}
	}
	return out
}

// summary counts the judgement for the confirmation (BR4.1).
func (c *rollbackConductor) summary(rec *journal.RunRecord) RollbackSummary {
	s := RollbackSummary{AccountID: rec.AccountID, RunID: rec.RunID, Record: c.out.Record, ApplyCompleted: rec.Completed,
		Force: c.req.Force}
	for _, rb := range rec.Rollbacks {
		if rb.Completed {
			s.UndoneAt = rb.StartedAt
		}
	}
	for _, t := range c.out.Targets {
		switch {
		case t.Reason == ReasonTagLimitExceeded:
			s.OverLimit++
		case t.Verdict == VerdictChangedSinceApply:
			s.ChangedSinceApply++
		case t.Verdict == VerdictRestore:
			s.ToRestore++
		case t.Verdict == VerdictAlreadyRestored:
			s.AlreadyRestored++
		case t.Verdict == VerdictGone:
			s.Gone++
		case t.Verdict == VerdictUnknown:
			s.Unreadable++
		}
	}
	return s
}

// closeRecord records what the second Ctrl+C left unknown and saves the
// record a last time: completed when nothing stopped the run (BR5.5, BR6.2).
// It runs once.
func (c *rollbackConductor) closeRecord(ctx context.Context) {
	cctx, cancel := closingContext(ctx)
	defer cancel()
	if !c.ex.abandon(func() {
		var unknown []journal.RollbackResult
		for _, t := range c.out.Targets {
			if t.Status == RollbackPending && c.ex.writeStarted(t.ARN) {
				t.Status, t.Reason = RollbackFailed, ReasonResultUnknown
				unknown = append(unknown, t.result())
			}
		}
		_ = c.run.RecordRollbackResults(unknown)
	}) {
		return
	}
	if ctx.Err() != nil {
		// The calls were cancelled (the second Ctrl+C), perhaps before the
		// stop of the first one was seen: the run is stopped, never complete.
		c.ex.st.stop(StopSignal, nil)
	}
	reason, err := c.ex.st.why()
	c.out.Stop, c.out.StopErr = reason, err
	if reason == StopNone {
		c.out.SaveErr = c.run.CompleteRollback(cctx)
		c.out.Completed = c.out.SaveErr == nil
		return
	}
	c.out.SaveErr = c.run.Close(cctx)
}

// collect sorts the targets into the counts and lists of the outcome (BR7.1).
func (c *rollbackConductor) collect() {
	c.ex.mu.Lock()
	defer c.ex.mu.Unlock()
	o := c.out
	for _, t := range o.Targets {
		switch t.Status {
		case RollbackRestored:
			o.Restored++
			if !t.Confirmed {
				o.Unconfirmed++
			}
		case RollbackAlreadyRestored:
			o.AlreadyRestored++
		case RollbackSkipped:
			o.Skipped = append(o.Skipped, t)
		case RollbackFailed:
			o.Failed = append(o.Failed, t)
		case RollbackPending:
			o.NotWritten++
		}
	}
	byARN := func(ts []*RollbackTarget) { sort.Slice(ts, func(i, j int) bool { return ts[i].ARN < ts[j].ARN }) }
	byARN(o.Skipped)
	byARN(o.Failed)
}

// writeLog writes the skipped and failed resources to the log file
// <rollbackId>.log (BR6.4). A log that cannot be written changes nothing
// else.
func (c *rollbackConductor) writeLog() {
	o := c.out
	if len(o.Skipped)+len(o.Failed) == 0 {
		return
	}
	var lines []journal.LogLine
	for _, t := range append(append([]*RollbackTarget{}, o.Skipped...), o.Failed...) {
		lines = append(lines, journal.LogLine{ARN: t.ARN, Region: t.Region, Status: string(t.Status), Reason: string(t.Reason),
			ErrorKind: t.ErrorKind, Operation: t.Operation})
	}
	o.LogFile, o.LogErr = c.r.opts.Journal.WriteLog(c.req.AccountID, o.RollbackID, lines)
}
