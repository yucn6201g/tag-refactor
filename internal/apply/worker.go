package apply

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// abandonWait bounds the wait for the regions after the second Ctrl+C,
// once the record is saved: their calls were cancelled and end quickly.
var abandonWait = 10 * time.Second

// forEachRegion runs work for every region, at most regionWorkers at once,
// each in a goroutine that turns a panic into a stop of the run (security
// design §3). It returns a channel closed when every region is done.
func (c *conductor) forEachRegion(regions []string, work func(region string)) <-chan struct{} {
	var wg sync.WaitGroup
	sem := make(chan struct{}, regionWorkers)
	for _, region := range regions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					c.st.stop(StopInternal, &errkind.Error{Kind: errkind.Internal, Op: "apply", Region: region,
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

// readCurrent reads the current tags of every target, region by region in
// reads of unitSize ARNs (BR3.3, workflow W2). A target whose tags, without
// aws: keys, differ from the plan is skipped; one the answer leaves out is
// skipped as gone, unless the plan had no tag of its own for it (a resource
// that never had a tag is not returned). A read that fails fails its
// targets; missing or expired credentials end the run before anything is
// recorded.
func (c *conductor) readCurrent(ctx context.Context, targets []*Target) error {
	byRegion := map[string][]*Target{}
	var regions []string
	for _, t := range targets {
		if byRegion[t.Region] == nil {
			regions = append(regions, t.Region)
		}
		byRegion[t.Region] = append(byRegion[t.Region], t)
	}
	var mu sync.Mutex
	var credErr error
	done := c.forEachRegion(regions, func(region string) {
		ts := byRegion[region]
		for start := 0; start < len(ts); start += unitSize {
			if c.st.stopped() {
				return
			}
			part := ts[start:min(start+unitSize, len(ts))]
			arns := make([]string, len(part))
			for i, t := range part {
				arns[i] = t.ARN
			}
			found, err := c.a.opts.AWS.GetTags(ctx, region, arns)
			if errkind.KindOf(err) == errkind.CredentialsMissing {
				mu.Lock()
				credErr = err
				mu.Unlock()
				c.st.stop(StopCredentials, err)
				return
			}
			for _, t := range part {
				c.checkRead(t, found, err)
			}
		}
	})
	<-done
	if credErr != nil {
		return credErr
	}
	if reason, err := c.st.why(); reason == StopInternal {
		return err
	}
	return nil
}

// checkRead applies the first read to one target.
func (c *conductor) checkRead(t *Target, found map[string]map[string]string, err error) {
	if err != nil {
		e := errkind.Ensure(err, awsaccess.OpGetResources)
		t.Status, t.Reason, t.ErrorKind, t.Operation = journal.StatusFailed, ReasonReadFailed, e.Kind.String(), e.Op
		t.recordBefore, t.recordPlanned = t.PlanBefore, t.PlanBefore
		return
	}
	tags, ok := found[t.ARN]
	switch {
	case !ok && len(userTags(t.PlanBefore)) == 0:
		t.Current = map[string]string{}
	case !ok:
		t.Operation = awsaccess.OpGetResources
		t.skip(ReasonResourceNotFound, t.PlanBefore)
	case !sameUserTags(tags, t.PlanBefore):
		t.skip(ReasonChangedSincePlan, tags)
	default:
		t.Current = tags
	}
}

// writeAll writes the targets: the regions in parallel, the units of a
// region in order (NFR5.13). It returns when every region is done, or at
// once when ctx is cancelled (the second Ctrl+C); then the regions are given
// abandonWait after the record is saved.
func (c *conductor) writeAll(ctx context.Context, writes []*Target) {
	regions, units := planUnits(writes)
	total := len(writes)
	done := c.forEachRegion(regions, func(region string) {
		for _, u := range units[region] {
			if c.st.stopped() {
				return
			}
			c.writeUnit(ctx, u, total)
		}
	})
	select {
	case <-done:
	case <-ctx.Done():
		c.closeRun(ctx)
		select {
		case <-done:
		case <-time.After(abandonWait):
		}
	}
}

// outcome is what the writes of a resource tell before the read back.
type outcome int

// The outcomes start at 1: the zero value is no outcome, which decide
// refuses.
const (
	// outApplied: every write worked.
	outApplied outcome = iota + 1
	// outNothing: the first write was refused; nothing was written.
	outNothing
	// outPartial: the first write worked and the second was refused.
	outPartial
	// outGone: the resource no longer exists.
	outGone
	// outDecide: the first write ended without an answer; the read back
	// decides.
	outDecide
	// outDecidePartial: the second write ended without an answer.
	outDecidePartial
)

// write is the outcome of one target with the error behind it.
type write struct {
	out outcome
	err *errkind.Error
}

// writeUnit reads a unit again, writes its batches and reads it back (BR4.7,
// BR4.1, BR4.3), then records the results.
func (c *conductor) writeUnit(ctx context.Context, u unit, total int) {
	all := u.targets()
	arns := make([]string, len(all))
	for i, t := range all {
		arns[i] = t.ARN
	}
	found, err := c.a.opts.AWS.GetTags(ctx, u.region, arns)
	if err != nil {
		c.st.callEnded(credentialsOnly(err))
		e := errkind.Ensure(err, awsaccess.OpGetResources)
		for _, t := range all {
			c.settle(t, func() {
				t.Status, t.Reason, t.ErrorKind, t.Operation = journal.StatusFailed, ReasonReadFailed, e.Kind.String(), e.Op
			})
		}
		c.progress(len(all), total)
		return
	}
	writes := map[*Target]write{}
	var attempted []*Target
	for _, b := range u.batches {
		if c.st.stopped() {
			break
		}
		var live []*Target
		for _, t := range b.targets {
			if c.recheck(t, found) {
				live = append(live, t)
			}
		}
		if len(live) == 0 {
			continue
		}
		c.writeBatch(ctx, u.region, live, writes)
		attempted = append(attempted, live...)
	}
	if len(attempted) == 0 {
		c.progress(len(all)-countPending(all), total)
		return
	}
	readCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		readCtx, cancel = closingContext(ctx)
		defer cancel()
	}
	back, readErr := c.readBack(readCtx, u.region, attempted)
	for _, t := range attempted {
		c.settle(t, func() { decide(t, writes[t], back, readErr == nil) })
	}
	c.progress(len(all)-countPending(all), total)
}

// credentialsOnly keeps an error of a read for the stop counter only when it
// is about credentials: reads do not count as write calls.
func credentialsOnly(err error) error {
	if errkind.KindOf(err) == errkind.CredentialsMissing {
		return err
	}
	return nil
}

func countPending(ts []*Target) int {
	n := 0
	for _, t := range ts {
		if t.Status == journal.StatusPending {
			n++
		}
	}
	return n
}

// recheck applies the read again to a target before its batch (BR4.7).
func (c *conductor) recheck(t *Target, found map[string]map[string]string) bool {
	tags, ok := found[t.ARN]
	switch {
	case !ok && len(userTags(t.Current)) > 0:
		c.settle(t, func() {
			t.Status, t.Reason, t.Operation = journal.StatusSkipped, ReasonResourceNotFound, awsaccess.OpGetResources
		})
		return false
	case ok && !sameUserTags(tags, t.Current):
		c.settle(t, func() { t.Status, t.Reason = journal.StatusSkipped, ReasonChangedSincePlan })
		return false
	}
	return true
}

// settle changes a target and records it, unless the run was abandoned. A
// record that cannot take it stops the run (BR4.4).
func (c *conductor) settle(t *Target, change func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.abandoned {
		return
	}
	change()
	if err := c.run.RecordAfter(t.ARN, t.AfterTags, t.Status, t.recordReason()); err != nil {
		c.st.stop(StopStorage, err)
	}
}

// writeBatch writes one batch: the tags to set first and the keys to remove
// after, or the other way round for a batch that removes first (BR4.1). A
// resource goes to the second call only when the first worked for it.
func (c *conductor) writeBatch(ctx context.Context, region string, ts []*Target, writes map[*Target]write) {
	c.mu.Lock()
	for _, t := range ts {
		t.writing = true
	}
	c.mu.Unlock()
	first := ts[0]
	type step struct {
		op   string
		call func(arns []string) (awsaccess.WriteResult, error)
	}
	tag := step{awsaccess.OpTagResources, func(arns []string) (awsaccess.WriteResult, error) {
		return c.a.opts.AWS.TagResources(ctx, region, arns, first.TagsToSet)
	}}
	untag := step{awsaccess.OpUntagResources, func(arns []string) (awsaccess.WriteResult, error) {
		return c.a.opts.AWS.UntagResources(ctx, region, arns, first.KeysToRemove)
	}}
	var steps []step
	if len(first.TagsToSet) > 0 {
		steps = append(steps, tag)
	}
	if len(first.KeysToRemove) > 0 {
		steps = append(steps, untag)
	}
	if first.RemoveFirst && len(steps) == 2 {
		steps[0], steps[1] = steps[1], steps[0]
	}
	live := ts
	for i, s := range steps {
		arns := make([]string, len(live))
		for j, t := range live {
			arns[j] = t.ARN
		}
		res, err := s.call(arns)
		c.st.callEnded(err)
		if err != nil {
			e := errkind.Ensure(err, s.op)
			for _, t := range live {
				writes[t] = write{out: failedOutcome(i, e), err: e}
			}
			return
		}
		var next []*Target
		for _, t := range live {
			if e, failed := res.Failed[t.ARN]; failed {
				if e.Kind == errkind.CredentialsMissing {
					c.st.stop(StopCredentials, e)
				}
				writes[t] = write{out: failedOutcome(i, e), err: e}
				continue
			}
			next = append(next, t)
		}
		live = next
	}
	for _, t := range live {
		writes[t] = write{out: outApplied}
	}
}

// failedOutcome is the outcome of a write that failed in step i (0 the
// first call, 1 the second) with e: a resource that is gone, one whose
// result only the read back can tell (Transient, or no answer at all), or a
// refusal.
func failedOutcome(i int, e *errkind.Error) outcome {
	switch {
	case e.Kind == errkind.NotFound:
		return outGone
	case e.Kind == errkind.Transient || e.Kind == errkind.Internal:
		if i == 0 {
			return outDecide
		}
		return outDecidePartial
	case i == 0:
		return outNothing
	default:
		return outPartial
	}
}

// readBack reads the tags of the attempted resources after writing (BR4.3).
// A resource the answer leaves out has no tags.
func (c *conductor) readBack(ctx context.Context, region string, ts []*Target) (map[string]map[string]string, error) {
	arns := make([]string, len(ts))
	for i, t := range ts {
		arns[i] = t.ARN
	}
	found, err := c.a.opts.AWS.GetTags(ctx, region, arns)
	if err != nil {
		return nil, err
	}
	for _, arn := range arns {
		if _, ok := found[arn]; !ok {
			found[arn] = map[string]string{}
		}
	}
	return found, nil
}

// decide sets the status of an attempted target from its write and the read
// back (BR4.2, BR4.3, security design §5 "読み戻しで決める").
func decide(t *Target, w write, back map[string]map[string]string, read bool) {
	after := back[t.ARN]
	if read {
		t.AfterTags = maps.Clone(after)
	}
	if w.err != nil {
		t.ErrorKind, t.Operation = w.err.Kind.String(), w.err.Op
	}
	allBefore, allPlanned := read && touchedMatch(t, after, t.Current), read && touchedMatch(t, after, t.PlannedAfter)
	switch w.out {
	case outApplied:
		t.Status, t.Reason = journal.StatusApplied, ReasonNone
		t.AfterConfirmed = read && sameUserTags(after, t.PlannedAfter)
	case outNothing:
		t.Status, t.Reason, t.AfterTags = journal.StatusFailed, ReasonWriteFailed, nil
	case outPartial:
		t.Status, t.Reason = journal.StatusFailed, ReasonPartialWrite
	case outGone:
		t.Status, t.Reason, t.AfterTags = journal.StatusSkipped, ReasonResourceNotFound, nil
	case outDecide, outDecidePartial:
		switch {
		case allPlanned:
			t.Status, t.Reason, t.AfterConfirmed = journal.StatusApplied, ReasonNone, true
			t.ErrorKind, t.Operation = "", ""
		case allBefore && w.out == outDecide:
			t.Status, t.Reason, t.AfterTags = journal.StatusFailed, ReasonWriteFailed, nil
		default:
			t.Status, t.Reason = journal.StatusPending, ReasonResultUnknown
		}
	default:
		panic(fmt.Sprintf("apply: no write outcome for %s", t.ARN))
	}
}

// touchedMatch reports whether every touched key of the target is in tags as
// in want: the same value, or absent from both (contract C4).
func touchedMatch(t *Target, tags, want map[string]string) bool {
	for _, k := range t.TouchedKeys {
		v, ok := tags[k]
		w, wok := want[k]
		if ok != wok || v != w {
			return false
		}
	}
	return true
}
