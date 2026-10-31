package apply

import (
	"maps"
	"slices"

	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// Verdict is the judgement of a resource of rollback (U7 BR3.2, entities
// RollbackTarget.verdict).
type Verdict string

// The verdicts of U7.
const (
	// VerdictAlreadyRestored: every touched key is as before apply.
	VerdictAlreadyRestored Verdict = "alreadyRestored"
	// VerdictRestore: every touched key is as before or as after apply, so
	// apply wrote it, perhaps half way.
	VerdictRestore Verdict = "restore"
	// VerdictChangedSinceApply: a touched key is neither, so someone changed
	// it after apply.
	VerdictChangedSinceApply Verdict = "changedSinceApply"
	// VerdictGone: the read left the resource out.
	VerdictGone Verdict = "gone"
	// VerdictUnknown: its tags could not be read.
	VerdictUnknown Verdict = "unknown"
)

// RollbackStatus is the status of a resource of rollback (U7 entities
// RollbackTarget.status); pending is never written to the record.
type RollbackStatus string

// The statuses of U7.
const (
	RollbackPending         RollbackStatus = "pending"
	RollbackRestored        RollbackStatus = "restored"
	RollbackSkipped         RollbackStatus = "skipped"
	RollbackFailed          RollbackStatus = "failed"
	RollbackAlreadyRestored RollbackStatus = "alreadyRestored"
)

// The reasons rollback adds to those of U6 (contract C4, U7 BR5.2, BR5.3).
const (
	ReasonChangedSinceApply Reason = "changedSinceApply"
	ReasonNotRestored       Reason = "notRestored"
	// ReasonNotConfirmed is only the reason of the record for a resource
	// restored that the read back could not confirm.
	ReasonNotConfirmed Reason = "notConfirmed"
)

// RollbackTarget is one resource of the record that rollback judges: an
// entry with touched keys whose status is not skipped (U7 BR1.4, entities
// RollbackTarget).
type RollbackTarget struct {
	ARN, Region string
	// TouchedKeys, BeforeTags and PlannedAfterTags are those of the record.
	TouchedKeys                  []string
	BeforeTags, PlannedAfterTags map[string]string
	// Current are the tags of the last read, aws: keys included.
	Current map[string]string
	Verdict Verdict
	// TagsToSet and KeysToRemove put the touched keys back as before apply
	// (BR3.4); RemoveFirst is set when setting first would pass 50 tags on
	// the way (BR3.5).
	TagsToSet    map[string]string
	KeysToRemove []string
	RemoveFirst  bool
	// TagCount is the number of tags the resource would have, for
	// tagLimitExceeded.
	TagCount int
	Status   RollbackStatus
	Reason   Reason
	// ErrorKind is the error kind (contract C12) of a failure, and Operation
	// the operation that failed. No error text is kept.
	ErrorKind, Operation string
	// Confirmed is set for a restored resource whose tags read back were as
	// before apply (BR5.3).
	Confirmed bool
}

// newRollbackTargets returns the targets of the record: the entries with
// touched keys whose status is not skipped, in ARN order (BR1.4).
func newRollbackTargets(rec *journal.RunRecord) []*RollbackTarget {
	var targets []*RollbackTarget
	for _, e := range rec.Entries {
		if len(e.TouchedKeys) == 0 || e.Status == journal.StatusSkipped {
			continue
		}
		targets = append(targets, &RollbackTarget{ARN: e.ARN, Region: e.Region, TouchedKeys: e.TouchedKeys,
			BeforeTags: nonNil(e.BeforeTags), PlannedAfterTags: nonNil(e.PlannedAfterTags), Status: RollbackPending})
	}
	slices.SortFunc(targets, func(a, b *RollbackTarget) int {
		switch {
		case a.ARN < b.ARN:
			return -1
		case a.ARN > b.ARN:
			return 1
		}
		return 0
	})
	return targets
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// currentTags returns the tags a read found for the target. A target the
// answer leaves out has no tags when the record has no tag of its own before
// or after apply: the tagging API may leave out a resource without tags (U7
// security design §3, BR3.1, BR5.3). Otherwise ok is false: the resource is
// gone, or could not be read back.
func (t *RollbackTarget) currentTags(found map[string]map[string]string) (tags map[string]string, ok bool) {
	if tags, ok := found[t.ARN]; ok {
		return tags, true
	}
	if len(userTags(t.BeforeTags)) == 0 || len(userTags(t.PlannedAfterTags)) == 0 {
		return map[string]string{}, true
	}
	return nil, false
}

// keysMatch reports whether every key is in tags as in want: the same value,
// or absent from both.
func keysMatch(keys []string, tags, want map[string]string) bool {
	for _, k := range keys {
		v, ok := tags[k]
		w, wok := want[k]
		if ok != wok || v != w {
			return false
		}
	}
	return true
}

// judge compares the touched keys of the current tags with the record
// (BR3.2): only the touched keys count, never the others or aws: keys.
func (t *RollbackTarget) judge(current map[string]string) Verdict {
	if keysMatch(t.TouchedKeys, current, t.BeforeTags) {
		return VerdictAlreadyRestored
	}
	for _, k := range t.TouchedKeys {
		if !keysMatch([]string{k}, current, t.BeforeTags) && !keysMatch([]string{k}, current, t.PlannedAfterTags) {
			return VerdictChangedSinceApply
		}
	}
	return VerdictRestore
}

// review reads the target again from a read (the first one or the read
// again of its unit) and settles what it can (BR3.1 to BR3.6, BR5.6): it
// reports true when the target is still to be written. found is nil when the
// read failed.
func (t *RollbackTarget) review(found map[string]map[string]string, force bool) bool {
	current, ok := t.currentTags(found)
	if !ok {
		t.Verdict, t.Current = VerdictGone, nil
		t.settle(RollbackSkipped, ReasonResourceNotFound)
		return false
	}
	t.Current, t.Verdict = current, t.judge(current)
	switch {
	case t.Verdict == VerdictAlreadyRestored:
		t.settle(RollbackAlreadyRestored, ReasonNone)
		return false
	case t.Verdict == VerdictChangedSinceApply && !force:
		t.settle(RollbackSkipped, ReasonChangedSinceApply)
		return false
	}
	t.compute()
	if t.Reason == ReasonTagLimitExceeded {
		return false
	}
	t.Status, t.Reason = RollbackPending, ReasonNone
	return true
}

// settle sets the status and the reason of a target that will not be
// written.
func (t *RollbackTarget) settle(status RollbackStatus, reason Reason) {
	t.Status, t.Reason = status, reason
	t.TagsToSet, t.KeysToRemove, t.RemoveFirst = nil, nil, false
}

// compute works out the tags to set and the keys to remove that put the
// touched keys back as before apply, from Current (BR3.4), and skips a target
// that would have more than 50 tags (BR3.5).
func (t *RollbackTarget) compute() {
	t.TagsToSet, t.KeysToRemove = map[string]string{}, nil
	for _, k := range t.TouchedKeys {
		b, inBefore := t.BeforeTags[k]
		cur, inCurrent := t.Current[k]
		switch {
		case inBefore && (!inCurrent || cur != b):
			t.TagsToSet[k] = b
		case !inBefore && inCurrent:
			t.KeysToRemove = append(t.KeysToRemove, k)
		}
	}
	slices.Sort(t.KeysToRemove)
	after := userTags(t.Current)
	maps.Copy(after, t.TagsToSet)
	for _, k := range t.KeysToRemove {
		delete(after, k)
	}
	if n := len(after); n > MaxTags {
		t.TagCount = n
		t.settle(RollbackSkipped, ReasonTagLimitExceeded)
		return
	}
	union := userTags(t.Current)
	maps.Copy(union, t.TagsToSet)
	t.RemoveFirst = len(union) > MaxTags
}

// restored reports whether tags have every touched key as before apply.
func (t *RollbackTarget) restored(tags map[string]string) bool {
	return keysMatch(t.TouchedKeys, tags, t.BeforeTags)
}

// result is the result of the target for the record (contract C4): restored
// without confirmation has the reason notConfirmed, and a failure its error
// kind, as "writeFailed:AccessDenied". A skip keeps its reason alone, so a
// resource gone at the write reads like one gone at the first read. No error
// text.
func (t *RollbackTarget) result() journal.RollbackResult {
	reason := string(t.Reason)
	switch {
	case t.Status == RollbackRestored && !t.Confirmed:
		reason = string(ReasonNotConfirmed)
	case t.Status == RollbackRestored || t.Status == RollbackAlreadyRestored:
		reason = ""
	case t.Status == RollbackFailed && t.ErrorKind != "" && t.Reason != ReasonNone:
		reason += ":" + t.ErrorKind
	}
	return journal.RollbackResult{ARN: t.ARN, Status: string(t.Status), Reason: reason}
}
