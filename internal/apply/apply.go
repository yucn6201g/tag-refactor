// Package apply rewrites the tags of a plan (U6 workflow W1, steps 7 to 13):
// it takes the lock, asks for the confirmation through the command line,
// reads the current tags and skips what changed since the plan, works out
// what to write, saves the record before the first write, writes the
// regions in parallel in batches, reads back what it wrote, and closes the
// record and the lock however the run ends.
//
// It reaches AWS only through the interfaces of awsaccess and keeps the
// record through journal; it prints nothing and reports progress through a
// ProgressSink (contract C11).
package apply

import (
	"context"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// AWS is what apply uses of AWS: the reads by ARN and the tag writer.
type AWS interface {
	awsaccess.ARNTagReader
	awsaccess.TagWriter
}

// ProgressSink receives the progress of a run (contract C11).
type ProgressSink interface {
	// ItemsProcessed says how many of the resources to write are done.
	ItemsProcessed(done, total int)
	// Notice reports a warning, as a message of the display's catalog with
	// its values.
	Notice(id string, args ...any)
}

// NoticeAWSKey is the warning of a change of the plan that names a key
// starting with aws:, which is not written (BR3.5): its values are the ARN
// and the key.
const NoticeAWSKey = "apply.awsKey"

// Options are what an Applier works with.
type Options struct {
	AWS     AWS
	Journal *journal.Journal
	Sink    ProgressSink
	// Now gives the run ID's time; nil means time.Now. Rand gives its
	// random part; nil means crypto/rand.
	Now  func() time.Time
	Rand io.Reader
}

// Applier runs apply.
type Applier struct{ opts Options }

// New returns an applier.
func New(opts Options) *Applier {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Applier{opts: opts}
}

// Request is one apply (U6 entities ApplyRequest, after the checks of the
// command line).
type Request struct {
	Plan      *planner.Plan
	AccountID string
	Location  journal.Location
	// ForceUnlock removes a lock in the way before taking it again (BR3.2).
	ForceUnlock bool
	// OnLocksRemoved is told what --force-unlock removed.
	OnLocksRemoved func([]journal.HeldLock)
	// Confirm asks for the confirmation once the lock is held (BR2.2) and
	// reports whether to write; nil writes without asking (--yes).
	Confirm func() bool
}

// Result is the end of a run (U6 entities ApplyResult).
type Result struct {
	RunID string
	// Record is where the record is, empty when no record was started.
	Record string
	// Cancelled is set when the confirmation was not yes.
	Cancelled bool
	Targets   []*Target
	// Applied counts the resources written; Unconfirmed those of them whose
	// tags read back were not the planned ones, or could not be read.
	Applied, Unconfirmed int
	// Skipped are the skipped resources, without nothingToWrite. Failed are
	// the failed ones and those whose result is unknown. NotWritten counts
	// the resources a stop left unwritten.
	Skipped, Failed []*Target
	NotWritten      int
	// Stop says why the run stopped writing, and StopErr is the error
	// behind it, if any.
	Stop    StopReason
	StopErr error
	// Completed is set when the record was completed. SaveErr is the
	// failure of its last save.
	Completed bool
	SaveErr   error
	// ReleaseErr is the failure to remove the lock.
	ReleaseErr error
	// LogFile is the log of the skipped and failed resources, and LogErr
	// the failure to write it.
	LogFile string
	LogErr  error
	// RemovedLocks are the locks --force-unlock removed.
	RemovedLocks []journal.HeldLock
}

// ExitCode is the exit code of the run (BR6.2): 1 for failures, a stop, a
// record or a lock that could not be saved or removed, or a plan whose
// collection was incomplete; 2 for skipped resources and plans with
// conflicts or required tags without a value; 0 otherwise, and 0 when the
// confirmation was not yes.
func (r *Result) ExitCode(p *planner.Plan) int {
	switch {
	case r.Cancelled:
		return 0
	case len(r.Failed) > 0 || r.NotWritten > 0 || r.Stop != StopNone || r.SaveErr != nil || r.ReleaseErr != nil ||
		!p.InventoryComplete:
		return 1
	case len(r.Skipped) > 0 || len(p.Conflicts) > 0 || len(p.UnsetRequiredTags) > 0:
		return 2
	}
	return 0
}

// closeTimeout bounds the work after a stop: the last save, the read back
// and the release of the lock run on a context that Ctrl+C does not cancel
// (security design §3).
var closeTimeout = 30 * time.Second

func closingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
}

// conductor holds the state of one run.
type conductor struct {
	a    *Applier
	req  Request
	res  *Result
	st   *stopper
	run  *journal.Run
	mu   sync.Mutex
	done int
	// abandoned is set when the record is closed, which the second Ctrl+C
	// does while the regions still run: from then on they change nothing.
	abandoned bool
	closed    bool
}

// Run runs one apply. ctx is the context of the AWS calls: the second
// Ctrl+C cancels it (the second stage of security design §5). stop is closed
// on the first Ctrl+C: no new batch starts, and the calls that run finish.
// The error is set when the run ended before any record was started; the
// result is set once the lock was taken, and the lock is always released.
func (a *Applier) Run(ctx context.Context, req Request, stop <-chan struct{}) (res *Result, err error) {
	runID := journal.NewRunID(a.opts.Now(), a.opts.Rand)
	res = &Result{RunID: runID}
	h, err := a.lock(ctx, req, res)
	if err != nil {
		return res, err
	}
	c := &conductor{a: a, req: req, res: res, st: newStopper()}
	defer func() {
		cctx, cancel := closingContext(ctx)
		defer cancel()
		res.ReleaseErr = a.opts.Journal.Release(cctx, h)
	}()
	defer func() {
		if p := recover(); p != nil {
			err = &errkind.Error{Kind: errkind.Internal, Op: "apply", Detail: "an unexpected internal error"}
			if c.run != nil {
				c.closeRun(ctx)
				err = nil
				res.Stop, res.StopErr = StopInternal, &errkind.Error{Kind: errkind.Internal, Op: "apply"}
			}
		}
	}()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-stop:
			c.st.stop(StopSignal, nil)
		case <-finished:
		}
	}()
	if req.Confirm != nil && !req.Confirm() {
		res.Cancelled = true
		return res, nil
	}
	return res, c.apply(ctx, h)
}

// lock takes the lock, removing a lock in the way first with --force-unlock
// (BR3.1, BR3.2).
func (a *Applier) lock(ctx context.Context, req Request, res *Result) (*journal.LockHandle, error) {
	j := a.opts.Journal
	h, err := j.Lock(ctx, req.AccountID, req.Location, journal.PurposeApply, res.RunID)
	if errkind.KindOf(err) != errkind.Conflict || !req.ForceUnlock {
		return h, err
	}
	removed, ferr := j.ForceUnlock(ctx, req.AccountID, req.Location)
	res.RemovedLocks = removed
	if req.OnLocksRemoved != nil {
		req.OnLocksRemoved(removed)
	}
	if ferr != nil {
		return nil, ferr
	}
	return j.Lock(ctx, req.AccountID, req.Location, journal.PurposeApply, res.RunID)
}

// apply is steps 9 to 13 of workflow W1.
func (c *conductor) apply(ctx context.Context, h *journal.LockHandle) error {
	targets := newTargets(c.req.Plan)
	c.res.Targets = targets
	if err := c.readCurrent(ctx, targets); err != nil {
		return err
	}
	if c.st.stopped() {
		c.res.Stop, c.res.StopErr = c.st.why()
		c.res.NotWritten = len(targets)
		return nil
	}
	var writes []*Target
	var entries []journal.Entry
	for _, t := range targets {
		if t.Status == journal.StatusPending && t.Current != nil {
			t.compute(func(arn, key string) { c.a.opts.Sink.Notice(NoticeAWSKey, arn, key) })
		}
		if t.Reason == ReasonNothingToWrite {
			continue
		}
		if t.Status == journal.StatusPending {
			writes = append(writes, t)
		}
		entries = append(entries, t.entry())
	}
	if len(entries) > 0 {
		run, err := c.a.opts.Journal.Begin(ctx, h, c.req.Plan.PlanID)
		if err != nil {
			return err
		}
		if err := run.RecordBefore(ctx, entries); err != nil {
			return err
		}
		c.run = run
		c.res.Record = run.Where()
		run.StartAutosave()
		c.writeAll(ctx, writes)
		c.closeRun(ctx)
	}
	c.collect()
	c.writeLog()
	return nil
}

// closeRun records what the second Ctrl+C left unknown, and saves the record
// a last time: completed when nothing stopped the run (BR4.5, BR4.6).
func (c *conductor) closeRun(ctx context.Context) {
	cctx, cancel := closingContext(ctx)
	defer cancel()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed, c.abandoned = true, true
	for _, t := range c.res.Targets {
		if t.Status == journal.StatusPending && t.writing && t.Reason == ReasonNone {
			t.Reason = ReasonResultUnknown
			_ = c.run.RecordAfter(t.ARN, nil, t.Status, t.recordReason())
		}
	}
	c.mu.Unlock()
	reason, err := c.st.why()
	c.res.Stop, c.res.StopErr = reason, err
	if reason == StopNone {
		c.res.SaveErr = c.run.Complete(cctx)
		c.res.Completed = c.res.SaveErr == nil
		return
	}
	c.res.SaveErr = c.run.Close(cctx)
}

// collect sorts the targets into the lists of the result (BR6.1).
func (c *conductor) collect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.res
	for _, t := range r.Targets {
		switch {
		case t.Status == journal.StatusApplied:
			r.Applied++
			if !t.AfterConfirmed {
				r.Unconfirmed++
			}
		case t.Status == journal.StatusSkipped && t.Reason != ReasonNothingToWrite:
			r.Skipped = append(r.Skipped, t)
		case t.Status == journal.StatusFailed || t.Reason == ReasonResultUnknown:
			r.Failed = append(r.Failed, t)
		case t.Status == journal.StatusPending:
			t.Reason = ReasonInterrupted
			r.NotWritten++
		}
	}
	byARN := func(ts []*Target) { sort.Slice(ts, func(i, j int) bool { return ts[i].ARN < ts[j].ARN }) }
	byARN(r.Skipped)
	byARN(r.Failed)
}

// writeLog writes the skipped and failed resources to the log file (BR5.5).
// A log that cannot be written changes nothing else.
func (c *conductor) writeLog() {
	r := c.res
	if r.Record == "" || len(r.Skipped)+len(r.Failed) == 0 {
		return
	}
	var lines []journal.LogLine
	for _, t := range append(append([]*Target{}, r.Skipped...), r.Failed...) {
		lines = append(lines, journal.LogLine{ARN: t.ARN, Region: t.Region, Status: string(t.Status), Reason: string(t.Reason),
			ErrorKind: t.ErrorKind, Operation: t.Operation})
	}
	r.LogFile, r.LogErr = c.a.opts.Journal.WriteLog(c.req.AccountID, r.RunID, lines)
}

// progress counts n more resources done.
func (c *conductor) progress(n, total int) {
	c.mu.Lock()
	c.done += n
	done := c.done
	c.mu.Unlock()
	c.a.opts.Sink.ItemsProcessed(done, total)
}
