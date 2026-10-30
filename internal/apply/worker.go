package apply

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

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
	done := c.ex.forEachRegion(regions, func(region string) {
		ts := byRegion[region]
		for start := 0; start < len(ts); start += unitSize {
			if c.ex.st.stopped() {
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
				c.ex.st.stop(StopCredentials, err)
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
	if reason, err := c.ex.st.why(); reason == StopInternal {
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
	c.ex.writeRegions(ctx, regions, func(region string) {
		for _, u := range units[region] {
			if c.ex.st.stopped() {
				return
			}
			c.writeUnit(ctx, u, total)
		}
	}, func() { c.closeRun(ctx) })
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
	// outCancelled: the second Ctrl+C cancelled the write. AWS may still
	// apply it after the read back, so the result stays unknown (BR4.5).
	outCancelled
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
		c.ex.st.callEnded(credentialsOnly(err))
		e := errkind.Ensure(err, awsaccess.OpGetResources)
		for _, t := range all {
			c.settle(t, func() {
				t.Status, t.Reason, t.ErrorKind, t.Operation = journal.StatusFailed, ReasonReadFailed, e.Kind.String(), e.Op
			})
		}
		c.progress(len(all), total)
		return
	}
	writes := map[string]write{}
	var attempted []*Target
	for _, b := range u.batches {
		if c.ex.st.stopped() {
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
	readCtx, cancel := readContext(ctx)
	defer cancel()
	back, readErr := c.readBack(readCtx, u.region, attempted)
	for _, t := range attempted {
		c.settle(t, func() { decide(t, writes[t.ARN], back, readErr == nil) })
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
	c.ex.settle(func() error {
		change()
		return c.run.RecordAfter(t.ARN, t.AfterTags, t.Status, t.recordReason())
	})
}

// writeBatch writes one batch with the shared write (BR4.1): every target of
// a batch has the same tags to set, keys to remove and order.
func (c *conductor) writeBatch(ctx context.Context, region string, ts []*Target, writes map[string]write) {
	first := ts[0]
	arns := make([]string, len(ts))
	for i, t := range ts {
		arns[i] = t.ARN
	}
	b := batchWrite{set: first.TagsToSet, remove: first.KeysToRemove, removeFirst: first.RemoveFirst}
	maps.Copy(writes, c.ex.writeBatch(ctx, c.a.opts.AWS, region, arns, b))
}

// writeOutcome is the outcome of a write that failed in step i: unknown when
// the run was cancelled while it ran, else failedOutcome.
func writeOutcome(ctx context.Context, i int, e *errkind.Error) outcome {
	if ctx.Err() != nil {
		return outCancelled
	}
	return failedOutcome(i, e)
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
	case outCancelled:
		t.Status, t.Reason = journal.StatusPending, ReasonResultUnknown
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
