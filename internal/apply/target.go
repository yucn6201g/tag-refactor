package apply

import (
	"maps"
	"slices"
	"sort"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Reason says why a resource was skipped or failed, or why it is still
// pending (U6 entities ApplyTarget.reason).
type Reason string

// The reasons of U6 (functional-spec.md, state S1).
const (
	ReasonNone             Reason = ""
	ReasonChangedSincePlan Reason = "changedSincePlan"
	ReasonResourceNotFound Reason = "resourceNotFound"
	ReasonTagLimitExceeded Reason = "tagLimitExceeded"
	ReasonNothingToWrite   Reason = "nothingToWrite"
	ReasonReadFailed       Reason = "readFailed"
	ReasonWriteFailed      Reason = "writeFailed"
	ReasonPartialWrite     Reason = "partialWrite"
	ReasonResultUnknown    Reason = "resultUnknown"
	ReasonInterrupted      Reason = "interrupted"
)

// MaxTags is the number of tags, not counting those starting with aws:, a
// resource may have after apply (BR3.6: 50 for every type).
const MaxTags = 50

// Target is one resource of the plan while apply works on it (U6 entities
// ApplyTarget).
type Target struct {
	ARN, Region, ResourceType string
	// PlanBefore are the tags when the plan was made, aws: keys included.
	PlanBefore map[string]string
	Changes    []planner.Change
	// Current are the tags read just before writing, aws: keys included.
	Current map[string]string
	// PlannedAfter are the tags expected after writing (BR3.4); aws: keys
	// are those of Current.
	PlannedAfter map[string]string
	TagsToSet    map[string]string
	KeysToRemove []string
	// TouchedKeys are the keys of TagsToSet and KeysToRemove, sorted.
	TouchedKeys  []string
	Impacted     bool
	IaCManagedBy string
	Status       journal.Status
	Reason       Reason
	// RemoveFirst is set when adding first would pass 50 tags on the way
	// (BR3.6, BR4.1).
	RemoveFirst bool
	// TagCount is the number of tags the resource would have, for
	// tagLimitExceeded.
	TagCount int
	// ErrorKind is the error kind (contract C12) of a failure, and
	// Operation the operation that failed. No error text is kept.
	ErrorKind string
	Operation string
	// AfterTags are the tags read back after writing; AfterConfirmed is set
	// when, without aws: keys, they are PlannedAfter (BR4.3).
	AfterTags      map[string]string
	AfterConfirmed bool

	// recordBefore and recordPlanned are what the record keeps for the
	// resource (functional-spec.md, the values of the record).
	recordBefore, recordPlanned map[string]string
}

// newTargets returns one target per resource of the plan, in plan order.
func newTargets(p *planner.Plan) []*Target {
	targets := make([]*Target, 0, len(p.Resources))
	for _, r := range p.Resources {
		t := &Target{ARN: r.ARN, Region: r.Region, ResourceType: r.ResourceType, PlanBefore: r.BeforeTags,
			Changes: r.Changes, IaCManagedBy: r.IaCManagedBy, Status: journal.StatusPending}
		for _, c := range r.Changes {
			t.Impacted = t.Impacted || c.Impacted
		}
		targets = append(targets, t)
	}
	return targets
}

// userTags returns the tags without the keys starting with aws:.
func userTags(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if !awsaccess.IsAWSKey(k) {
			out[k] = v
		}
	}
	return out
}

// sameUserTags reports whether two sets of tags are the same without the
// aws: keys (contract C4: aws: keys are left out of every comparison).
func sameUserTags(a, b map[string]string) bool {
	return maps.Equal(userTags(a), userTags(b))
}

// skip marks the target skipped with the tags read as both the before and
// the planned tags of the record, and no touched keys.
func (t *Target) skip(reason Reason, read map[string]string) {
	t.Status, t.Reason = journal.StatusSkipped, reason
	t.TouchedKeys, t.TagsToSet, t.KeysToRemove = nil, nil, nil
	t.recordBefore, t.recordPlanned = read, read
}

// compute decides the tags to write from Current (BR3.4 to BR3.6). warn is
// told of each change that names a key starting with aws:, which is never
// written (BR3.5).
func (t *Target) compute(warn func(arn, key string)) {
	for _, c := range t.Changes {
		for _, k := range []string{c.FromKey, c.ToKey} {
			if k != "" && awsaccess.IsAWSKey(k) {
				warn(t.ARN, k)
			}
		}
	}
	after := planner.AfterTags(t.Current, t.Changes)
	for k := range after {
		if awsaccess.IsAWSKey(k) {
			delete(after, k)
		}
	}
	for k, v := range t.Current {
		if awsaccess.IsAWSKey(k) {
			after[k] = v
		}
	}
	t.PlannedAfter = after
	t.TagsToSet = map[string]string{}
	touched := map[string]bool{}
	for k, v := range userTags(after) {
		if cur, ok := t.Current[k]; !ok || cur != v {
			t.TagsToSet[k] = v
			touched[k] = true
		}
	}
	t.KeysToRemove = nil
	for k := range userTags(t.Current) {
		if _, ok := after[k]; !ok {
			t.KeysToRemove = append(t.KeysToRemove, k)
			touched[k] = true
		}
	}
	sort.Strings(t.KeysToRemove)
	t.TouchedKeys = slices.Sorted(maps.Keys(touched))
	t.recordBefore, t.recordPlanned = t.Current, t.PlannedAfter
	if len(t.TouchedKeys) == 0 {
		t.skip(ReasonNothingToWrite, t.Current)
		return
	}
	if n := len(userTags(after)); n > MaxTags {
		t.TagCount = n
		t.skip(ReasonTagLimitExceeded, t.Current)
		return
	}
	union := userTags(t.Current)
	maps.Copy(union, userTags(after))
	t.RemoveFirst = len(union) > MaxTags
}

// entry is the record entry of the target (functional-spec.md, the values
// of the record).
func (t *Target) entry() journal.Entry {
	return journal.Entry{ARN: t.ARN, Region: t.Region, TouchedKeys: t.TouchedKeys, BeforeTags: t.recordBefore,
		PlannedAfterTags: t.recordPlanned, AfterTags: t.AfterTags, Status: t.Status, Reason: t.recordReason()}
}

// recordReason is the reason of the record: the reason, and for a failure
// the error kind, as "writeFailed:AccessDenied". No error text.
func (t *Target) recordReason() string {
	if t.ErrorKind == "" || t.Reason == ReasonNone {
		return string(t.Reason)
	}
	return string(t.Reason) + ":" + t.ErrorKind
}
