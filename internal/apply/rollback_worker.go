package apply

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// The two passes of rollback over the regions (U7 logical-components.md):
// the first only judges, before the confirmation and without touching the
// record; the second writes back, unit by unit.

// rereadWaits are the waits before the reads again of the resources whose
// tags read back are not yet as before apply (NFR5.24): 7 seconds at most
// for a unit.
var rereadWaits = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// newTimer starts one wait; a variable so the tests replace the clock.
var newTimer = func(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTimer(d)
	return t.C, func() { t.Stop() }
}

// byRegion groups the targets by region, the regions sorted and each
// region's targets in ARN order.
func byRegion(targets []*RollbackTarget) ([]string, map[string][]*RollbackTarget) {
	groups := map[string][]*RollbackTarget{}
	for _, t := range targets {
		groups[t.Region] = append(groups[t.Region], t)
	}
	regions := slices.Sorted(maps.Keys(groups))
	for _, ts := range groups {
		sort.Slice(ts, func(i, j int) bool { return ts[i].ARN < ts[j].ARN })
	}
	return regions, groups
}

func arnsOf(ts []*RollbackTarget) []string {
	arns := make([]string, len(ts))
	for i, t := range ts {
		arns[i] = t.ARN
	}
	return arns
}

// judgeAll is the first pass (BR3.1 to BR3.6): it reads the current tags of
// every target, unitSize ARNs at a time, and judges them. A read that fails
// fails its targets (readFailed). Missing or expired credentials end the run
// before anything is recorded, and so does a panic; a stop leaves the
// targets that were not read pending.
func (c *rollbackConductor) judgeAll(ctx context.Context, targets []*RollbackTarget) error {
	regions, groups := byRegion(targets)
	var mu sync.Mutex
	var credErr error
	done := c.ex.forEachRegion(regions, func(region string) {
		ts := groups[region]
		for start := 0; start < len(ts); start += unitSize {
			if c.ex.st.stopped() {
				return
			}
			part := ts[start:min(start+unitSize, len(ts))]
			found, err := c.r.opts.AWS.GetTags(ctx, region, arnsOf(part))
			if errkind.KindOf(err) == errkind.CredentialsMissing {
				mu.Lock()
				credErr = err
				mu.Unlock()
				c.ex.st.stop(StopCredentials, err)
				return
			}
			c.ex.mu.Lock()
			for _, t := range part {
				c.judge(t, found, err)
			}
			c.ex.mu.Unlock()
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

// judge applies one read to a target; the caller holds the lock.
func (c *rollbackConductor) judge(t *RollbackTarget, found map[string]map[string]string, err error) {
	if err != nil {
		e := errkind.Ensure(err, awsaccess.OpGetResources)
		t.Verdict = VerdictUnknown
		t.settle(RollbackFailed, ReasonReadFailed)
		t.ErrorKind, t.Operation = e.Kind.String(), e.Op
		return
	}
	if !t.review(found, c.req.Force) && t.Verdict == VerdictGone {
		t.Operation = awsaccess.OpGetResources
	}
}

// rollbackUnit is at most unitSize targets of a region in ARN order: read
// again before writing and read back after (BR5.6, BR5.3).
type rollbackUnit struct {
	region  string
	targets []*RollbackTarget
}

// rollbackUnits cuts the targets to write of each region, in ARN order, into
// units of unitSize (NFR5.19).
func rollbackUnits(targets []*RollbackTarget) ([]string, map[string][]rollbackUnit) {
	regions, groups := byRegion(targets)
	units := map[string][]rollbackUnit{}
	for region, ts := range groups {
		for start := 0; start < len(ts); start += unitSize {
			units[region] = append(units[region], rollbackUnit{region: region, targets: ts[start:min(start+unitSize, len(ts))]})
		}
	}
	return regions, units
}

// rollbackBatches groups the targets of a unit that write the same thing,
// in unit order, into batches of batchSize; the batches never leave the
// unit (NFR5.19).
func rollbackBatches(ts []*RollbackTarget) [][]*RollbackTarget {
	byKey := map[string][]*RollbackTarget{}
	for _, t := range ts {
		k := batchKey(t.TagsToSet, t.KeysToRemove, t.RemoveFirst)
		byKey[k] = append(byKey[k], t)
	}
	var batches [][]*RollbackTarget
	for _, k := range slices.Sorted(maps.Keys(byKey)) {
		group := byKey[k]
		for start := 0; start < len(group); start += batchSize {
			batches = append(batches, group[start:min(start+batchSize, len(group))])
		}
	}
	return batches
}

// writeAll is the second pass: the regions in parallel, the units of a
// region in order (NFR5.19). It returns when every region is done, or at
// once when ctx is cancelled (the second Ctrl+C); then the record is closed
// and the regions are given abandonWait.
func (c *rollbackConductor) writeAll(ctx context.Context, writes []*RollbackTarget) {
	regions, units := rollbackUnits(writes)
	total := len(writes)
	c.ex.writeRegions(ctx, regions, func(region string) {
		for _, u := range units[region] {
			if c.ex.st.stopped() {
				return
			}
			c.writeUnit(ctx, u, total)
		}
	}, func() { c.closeRecord(ctx) })
}

// settle changes a target with change and records its result, unless change
// reports that it is still pending or the run was abandoned.
func (c *rollbackConductor) settle(t *RollbackTarget, change func() (pending bool)) (pending bool) {
	c.ex.settle(func() error {
		if pending = change(); pending {
			return nil
		}
		return c.run.RecordRollbackResults([]journal.RollbackResult{t.result()})
	})
	return pending
}

// writeUnit reads a unit again and judges it again (BR5.6), writes its
// batches, reads it back and reads again what is not yet as before apply
// (BR5.3, NFR5.24).
func (c *rollbackConductor) writeUnit(ctx context.Context, u rollbackUnit, total int) {
	defer c.unitDone(u, total)
	found, err := c.r.opts.AWS.GetTags(ctx, u.region, arnsOf(u.targets))
	if err != nil {
		c.ex.st.callEnded(credentialsOnly(err))
		for _, t := range u.targets {
			c.settle(t, func() bool {
				c.judge(t, nil, err)
				return false
			})
		}
		return
	}
	var live []*RollbackTarget
	for _, t := range u.targets {
		var keep bool
		c.settle(t, func() bool {
			c.judge(t, found, nil)
			keep = t.Status == RollbackPending
			return keep
		})
		if keep {
			live = append(live, t)
		}
	}
	writes := map[string]write{}
	var attempted []*RollbackTarget
	for _, b := range rollbackBatches(live) {
		if c.ex.st.stopped() {
			break
		}
		first := b[0]
		w := batchWrite{set: first.TagsToSet, remove: first.KeysToRemove, removeFirst: first.RemoveFirst}
		maps.Copy(writes, c.ex.writeBatch(ctx, c.r.opts.AWS, u.region, arnsOf(b), w))
		attempted = append(attempted, b...)
	}
	if len(attempted) == 0 {
		return
	}
	readCtx, cancel := readContext(ctx)
	defer cancel()
	back, readErr := c.r.opts.AWS.GetTags(readCtx, u.region, arnsOf(attempted))
	var again []*RollbackTarget
	for _, t := range attempted {
		if c.settle(t, func() bool { return decideRollback(t, writes[t.ARN], back, readErr == nil) }) {
			again = append(again, t)
		}
	}
	c.readAgain(ctx, u.region, again)
}

// unitDone reports the progress of a unit: its targets that are no longer
// pending.
func (c *rollbackConductor) unitDone(u rollbackUnit, total int) {
	c.ex.mu.Lock()
	n := 0
	for _, t := range u.targets {
		if t.Status != RollbackPending {
			n++
		}
	}
	c.ex.mu.Unlock()
	c.ex.progress(c.r.opts.Sink, n, total)
}

// readAgain reads again, after waits of 1, 2 and 4 seconds, the targets
// written whose tags read back were not as before apply (NFR5.24): those that
// become as before are restored, those that never do failed (notRestored).
// A stop, a cancellation or a read that fails leaves the rest failed
// (resultUnknown): written, but not checked.
func (c *rollbackConductor) readAgain(ctx context.Context, region string, ts []*RollbackTarget) {
	for i, d := range rereadWaits {
		if len(ts) == 0 {
			return
		}
		if !c.wait(ctx, d) {
			break
		}
		found, err := c.r.opts.AWS.GetTags(ctx, region, arnsOf(ts))
		if err != nil {
			break
		}
		last := i == len(rereadWaits)-1
		var still []*RollbackTarget
		for _, t := range ts {
			if c.settle(t, func() bool {
				if tags, ok := t.currentTags(found); ok && t.restored(tags) {
					t.Status, t.Reason, t.Confirmed = RollbackRestored, ReasonNone, true
					return false
				}
				if last {
					t.Status, t.Reason = RollbackFailed, ReasonNotRestored
					return false
				}
				return true
			}) {
				still = append(still, t)
			}
		}
		ts = still
	}
	for _, t := range ts {
		c.settle(t, func() bool {
			t.Status, t.Reason = RollbackFailed, ReasonResultUnknown
			return false
		})
	}
}

// wait waits d, and reports false when a stop or the cancellation of ctx
// ended the wait first.
func (c *rollbackConductor) wait(ctx context.Context, d time.Duration) bool {
	timer, stop := newTimer(d)
	defer stop()
	select {
	case <-timer:
		return true
	case <-ctx.Done():
		return false
	case <-c.ex.st.done():
		return false
	}
}

// decideRollback sets the result of an attempted target from its write and
// the read back (U7 security design §8). It reports true, leaving the target
// pending, when every write worked but the tags read back are not yet as
// before apply: they are read again.
func decideRollback(t *RollbackTarget, w write, back map[string]map[string]string, read bool) bool {
	var after map[string]string
	readBack := false
	if read {
		after, readBack = t.currentTags(back)
	}
	if w.err != nil {
		t.ErrorKind, t.Operation = w.err.Kind.String(), w.err.Op
	}
	restored := readBack && t.restored(after)
	switch w.out {
	case outApplied:
		switch {
		case restored:
			t.Status, t.Reason, t.Confirmed = RollbackRestored, ReasonNone, true
		case !readBack:
			t.Status, t.Reason, t.Confirmed = RollbackRestored, ReasonNone, false
		default:
			return true
		}
	case outNothing:
		t.Status, t.Reason = RollbackFailed, ReasonWriteFailed
	case outPartial:
		t.Status, t.Reason = RollbackFailed, ReasonPartialWrite
	case outGone:
		t.Status, t.Reason = RollbackSkipped, ReasonResourceNotFound
	case outDecide, outDecidePartial:
		switch {
		case restored:
			t.Status, t.Reason, t.Confirmed = RollbackRestored, ReasonNone, true
			t.ErrorKind, t.Operation = "", ""
		case w.out == outDecide && readBack && keysMatch(t.TouchedKeys, after, t.Current):
			t.Status, t.Reason = RollbackFailed, ReasonWriteFailed
		default:
			t.Status, t.Reason = RollbackFailed, ReasonResultUnknown
		}
	case outCancelled:
		t.Status, t.Reason = RollbackFailed, ReasonResultUnknown
	default:
		panic(fmt.Sprintf("rollback: no write outcome for %s", t.ARN))
	}
	return false
}
