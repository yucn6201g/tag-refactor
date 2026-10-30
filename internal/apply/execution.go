package apply

import (
	"context"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// The parts apply and rollback share (U7 logical-components.md): the state
// of one run, its regions, the write of one batch, the abandonment by the
// second Ctrl+C and the lock. None of them knows the targets of apply, the
// targets of rollback or the record type: each conductor keeps its own and
// hands the execution what to record.

// closeTimeout bounds the work after a stop: the last save, the read back
// and the release of the lock run on a context that Ctrl+C does not cancel
// (U6 security design §3).
var closeTimeout = 30 * time.Second

// abandonWait bounds the wait for the regions after the second Ctrl+C,
// once the record is saved: their calls were cancelled and end quickly.
var abandonWait = 10 * time.Second

func closingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
}

// readContext is the context of a read after writing: ctx, or a closing
// context once the second Ctrl+C cancelled ctx, so a cancelled write is still
// read back.
func readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() != nil {
		return closingContext(ctx)
	}
	return ctx, func() {}
}

// execution is the state of one run: the stop signal, the marks of the
// resources whose write started, the abandonment and the progress. Its lock
// also guards the targets of the conductor that holds it.
type execution struct {
	// op names the run in the Internal error of a panic: apply or rollback.
	op string
	st *stopper

	mu sync.Mutex
	// started holds the ARNs whose first write call has started.
	started map[string]bool
	// abandoned is set when the record is closed, which the second Ctrl+C
	// does while the regions still run: from then on they change nothing.
	abandoned bool
	done      int
}

// newExecution returns the state of a run named op; stop is the first
// Ctrl+C of the caller.
func newExecution(op string, stop <-chan struct{}) *execution {
	st := newStopper()
	st.signal = stop
	return &execution{op: op, st: st, started: map[string]bool{}}
}

// watchStop raises the stop when the caller's stop channel closes, until
// finished closes.
func (e *execution) watchStop(stop <-chan struct{}, finished <-chan struct{}) {
	go func() {
		select {
		case <-stop:
			e.st.stop(StopSignal, nil)
		case <-finished:
		}
	}()
}

// forEachRegion runs work for every region, at most regionWorkers at once,
// each in a goroutine that turns a panic into a stop of the run (U6 security
// design §3). It returns a channel closed when every region is done.
func (e *execution) forEachRegion(regions []string, work func(region string)) <-chan struct{} {
	var wg sync.WaitGroup
	sem := make(chan struct{}, regionWorkers)
	for _, region := range regions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					e.st.stop(StopInternal, &errkind.Error{Kind: errkind.Internal, Op: e.op, Region: region,
						Detail: "an unexpected internal error"})
				}
			}()
			sem <- struct{}{}
			defer func() { <-sem }()
			work(region)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// writeRegions runs work for every region (forEachRegion). It returns when
// every region is done, or at once when ctx is cancelled (the second
// Ctrl+C): then closeRecord saves and closes the record, and the regions are
// given abandonWait.
func (e *execution) writeRegions(ctx context.Context, regions []string, work func(region string), closeRecord func()) {
	done := e.forEachRegion(regions, work)
	select {
	case <-done:
	case <-ctx.Done():
		closeRecord()
		select {
		case <-done:
		case <-time.After(abandonWait):
		}
	}
}

// settle changes the result of a resource and records it, unless the run was
// abandoned: change returns the error of the record, which stops the run
// (U6 BR4.4).
func (e *execution) settle(change func() error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.abandoned {
		return
	}
	if err := change(); err != nil {
		e.st.stop(StopStorage, err)
	}
}

// abandon closes the record once: unknown records what the writes that
// started and never ended leave unknown, and from then on settle records
// nothing. It reports false when the record was already closed.
func (e *execution) abandon(unknown func()) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.abandoned {
		return false
	}
	e.abandoned = true
	unknown()
	return true
}

// writeStarted reports whether a write of the resource has started; the
// caller holds the lock.
func (e *execution) writeStarted(arn string) bool { return e.started[arn] }

// progress counts n more resources done and tells sink.
func (e *execution) progress(sink ProgressSink, n, total int) {
	e.mu.Lock()
	e.done += n
	done := e.done
	e.mu.Unlock()
	sink.ItemsProcessed(done, total)
}

// batchWrite is the write of one batch: the tags it sets, the keys it
// removes, and whether it removes first (U6 BR4.1, U7 BR5.1).
type batchWrite struct {
	set         map[string]string
	remove      []string
	removeFirst bool
}

// writeBatch writes one batch of a region: the tags to set first and the keys
// to remove after, or the other way round when it removes first. A resource
// goes to the second call only when the first worked for it. It returns the
// outcome of every resource by ARN (U7 security design §8).
func (e *execution) writeBatch(ctx context.Context, w awsaccess.TagWriter, region string, arns []string, b batchWrite) map[string]write {
	e.mu.Lock()
	for _, arn := range arns {
		e.started[arn] = true
	}
	e.mu.Unlock()
	type step struct {
		op   string
		call func(arns []string) (awsaccess.WriteResult, error)
	}
	tag := step{awsaccess.OpTagResources, func(arns []string) (awsaccess.WriteResult, error) {
		return w.TagResources(ctx, region, arns, b.set)
	}}
	untag := step{awsaccess.OpUntagResources, func(arns []string) (awsaccess.WriteResult, error) {
		return w.UntagResources(ctx, region, arns, b.remove)
	}}
	var steps []step
	if len(b.set) > 0 {
		steps = append(steps, tag)
	}
	if len(b.remove) > 0 {
		steps = append(steps, untag)
	}
	if b.removeFirst && len(steps) == 2 {
		steps[0], steps[1] = steps[1], steps[0]
	}
	writes := make(map[string]write, len(arns))
	live := arns
	for i, s := range steps {
		res, err := s.call(live)
		e.st.callEnded(err)
		if err != nil {
			callErr := errkind.Ensure(err, s.op)
			for _, arn := range live {
				writes[arn] = write{out: writeOutcome(ctx, i, callErr), err: callErr}
			}
			return writes
		}
		var next []string
		for _, arn := range live {
			if fe, failed := res.Failed[arn]; failed {
				if fe.Kind == errkind.CredentialsMissing {
					e.st.stop(StopCredentials, fe)
				}
				writes[arn] = write{out: writeOutcome(ctx, i, fe), err: fe}
				continue
			}
			next = append(next, arn)
		}
		live = next
	}
	for _, arn := range live {
		writes[arn] = write{out: outApplied}
	}
	return writes
}

// takeLock takes the lock of the account for runID with the purpose of the
// run, removing a lock in the way first with forceUnlock and telling
// onRemoved what it removed (U6 BR3.1, BR3.2; U7 BR2.1, BR2.2). A lock in the
// way without forceUnlock is the Conflict of journal.Lock, whose cause says
// what the lock holds.
func takeLock(ctx context.Context, j *journal.Journal, accountID string, loc journal.Location, purpose journal.Purpose,
	runID string, forceUnlock bool, onRemoved func([]journal.HeldLock)) (*journal.LockHandle, []journal.HeldLock, error) {
	h, err := j.Lock(ctx, accountID, loc, purpose, runID)
	if errkind.KindOf(err) != errkind.Conflict || !forceUnlock {
		return h, nil, err
	}
	removed, ferr := j.ForceUnlock(ctx, accountID, loc)
	if onRemoved != nil {
		onRemoved(removed)
	}
	if ferr != nil {
		return nil, removed, ferr
	}
	h, err = j.Lock(ctx, accountID, loc, purpose, runID)
	return h, removed, err
}
