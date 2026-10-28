// Package planner builds the change plan (contract C2): key renames, value
// changes, required tags to add and keys to remove, with the conflicts, the
// required tags whose value is not set, the remove candidates and the change
// IDs (U4 functional design BR1.1 to BR2.7). It saves a plan as JSON and
// reads it back (BR4.1 to BR4.4), and it defines the impact analysis
// interface and its input and output types (contract C9); the impact package
// implements it, so the dependency points from impact to planner.
package planner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// ChangeKind is the kind of a change (contract C2).
type ChangeKind string

// The four kinds of change, in the order a resource's changes are applied
// (BR1.8).
const (
	// KindRenameKey renames a tag key; see Change.MergesIntoExisting.
	KindRenameKey ChangeKind = "renameKey"
	// KindChangeValue replaces the value of a key with its canonical value.
	KindChangeValue ChangeKind = "changeValue"
	// KindAddRequired adds a required tag.
	KindAddRequired ChangeKind = "addRequired"
	// KindRemoveKey removes a tag key.
	KindRemoveKey ChangeKind = "removeKey"
)

// Kinds lists the kinds of change in display order (BR2.1).
var Kinds = []ChangeKind{KindRenameKey, KindChangeValue, KindAddRequired, KindRemoveKey}

// Valid reports whether k is one of the four kinds.
func (k ChangeKind) Valid() bool {
	switch k {
	case KindRenameKey, KindChangeValue, KindAddRequired, KindRemoveKey:
		return true
	}
	return false
}

// Mark is the impact mark of a change (BR2.5).
type Mark string

// Marks of a change: affected wins over undecided.
const (
	MarkNone      Mark = "none"
	MarkAffected  Mark = "affected"
	MarkUndecided Mark = "undecided"
)

// Change is one change to the tags of one resource (contract C2).
type Change struct {
	ChangeID    string
	ResourceARN string
	Kind        ChangeKind
	// FromKey is set for renameKey, changeValue and removeKey. For a
	// changeValue it may be the ToKey of a rename of the same resource, the
	// key after the rename (Q1).
	FromKey string
	// ToKey is set for renameKey and addRequired.
	ToKey string
	// FromValue is set for changeValue, and for renameKey and removeKey as
	// the value before the change (for reference).
	FromValue string
	// ToValue is set for changeValue and addRequired.
	ToValue string
	// MergesIntoExisting is set for renameKey only: true when ToKey already
	// exists on the resource when the change is applied. Such a rename does
	// not write ToKey and only removes FromKey (BR1.1, BR1.9).
	MergesIntoExisting bool
	// Impacted is true when an impact finding names this change (BR2.5).
	Impacted bool
	// Mark is the display mark: affected (!) or undecided (?).
	Mark Mark
}

// PlannedResource is a resource with at least one change (contract C2
// resources[]).
type PlannedResource struct {
	ARN string
	// Region is the region whose tagging API is called for the resource.
	Region string
	Global bool
	// ResourceType is the ARN form of the type, for example "ec2:instance".
	ResourceType string
	// BeforeTags are all tags as collected, aws: keys included.
	BeforeTags map[string]string
	// IaCManagedBy is for example "cloudformation:logging", or empty.
	IaCManagedBy string
	// Changes are in the order of BR1.8; there is at least one.
	Changes []Change
}

// KeyConflict is a resource whose canonical key and aliases carry values
// that differ even after the value spellings are unified, so nothing is
// changed for that key (BR1.2).
type KeyConflict struct {
	ResourceARN  string
	CanonicalKey string
	// PresentTags are the conflicting keys with their values as collected.
	PresentTags map[string]string
	// KeyOrder lists the keys of PresentTags for display: the aliases in rule
	// file order, then the canonical key. It is not saved.
	KeyOrder []string
}

// UnsetRequiredTag is a required key whose value could not be decided for
// some resources (BR1.4, BR2.2). Such resources get no addRequired.
type UnsetRequiredTag struct {
	Key string
	// ARNs are in ARN order; there is at least one.
	ARNs []string
}

// RemoveCandidate is a key that no rule names (BR2.3). It is never removed.
type RemoveCandidate struct {
	Key string
	// ResourceCount is the number of resources that carry the key. A plan
	// read from a file does not have it (contract C2 keeps key names only)
	// and has 0.
	ResourceCount int
}

// Plan is the result of one plan command.
type Plan struct {
	PlanID      string
	ToolVersion string
	AccountID   string
	CreatedAt   time.Time
	Regions     []string
	// RuleFile is the rule file as given to --rules (NFR8.19).
	RuleFile         string
	ScannedResources int
	// Resources are the resources with changes, in ARN order.
	Resources         []PlannedResource
	Conflicts         []KeyConflict
	UnsetRequiredTags []UnsetRequiredTag
	RemoveCandidates  []RemoveCandidate
	// UnsupportedResourceTypes are the types whose tags cannot be changed.
	UnsupportedResourceTypes []inventory.TypeCount
	ImpactFindings           []ImpactFinding
	UnreadPolicies           []UnreadPolicy
	// FailedRegions are the regions the tagging API could not read. They are
	// shown, not saved: InventoryComplete says the same in a saved plan.
	FailedRegions []inventory.FailedRegion
	// InventoryComplete is false when the tagging API could not read a
	// region.
	InventoryComplete      bool
	ImpactAnalysisComplete bool
}

// Options are the settings of Build that do not come from the snapshot or
// the rule set.
type Options struct {
	// Now is the creation time; it is kept in UTC to the second.
	Now time.Time
	// RuleFile is the rule file as given to --rules.
	RuleFile string
	// ToolVersion is the version of the tool, for example "v0.1.0".
	ToolVersion string
	// Rand gives the random suffix of the plan ID. Nil means crypto/rand.
	Rand io.Reader
}

// NewPlanID returns plan-YYYYMMDDTHHMMSSZ-xxxx: the creation time in UTC to
// the second and four random hexadecimal digits, so two plans made in the
// same second get different IDs (BR4.3).
func NewPlanID(now time.Time, random io.Reader) string {
	if random == nil {
		random = rand.Reader
	}
	var b [2]byte
	if _, err := io.ReadFull(random, b[:]); err != nil {
		// A source that cannot give two bytes still gets a valid ID; the
		// time keeps it unique to the second.
		b = [2]byte{}
	}
	return "plan-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// notTaggable reports whether the tags of the resource cannot be changed: a
// type the tagging API cannot change, or no ARN (BR1.7).
func notTaggable(r inventory.Resource) bool {
	return r.ARN == "" || r.NotTaggable || inventory.IsNotTaggable(r.ResourceType)
}

// builder holds the state of one Build.
type builder struct {
	rs       *rules.RuleSet
	keyRules []rules.KeyRule
	required []rules.RequiredTag
	// named holds every key a rule names: canonical keys, aliases, required
	// keys and removed keys (BR2.3).
	named      map[string]bool
	plan       *Plan
	unset      map[string][]string
	candidates map[string]int
	unsupport  map[string]int
	nextID     int
}

// Build creates the plan for the snapshot under the rule set (BR1.1 to
// BR2.4). Resources are taken in ARN order and the changes of each in the
// order of BR1.8, numbered c-000001 onward, so the same input always gives
// the same plan apart from its ID and creation time.
func Build(snap *inventory.Snapshot, rs *rules.RuleSet, opts Options) *Plan {
	now := opts.Now.UTC().Truncate(time.Second)
	b := &builder{
		rs:         rs,
		named:      map[string]bool{},
		unset:      map[string][]string{},
		candidates: map[string]int{},
		unsupport:  map[string]int{},
		plan: &Plan{
			PlanID:                   NewPlanID(now, opts.Rand),
			ToolVersion:              opts.ToolVersion,
			AccountID:                snap.AccountID,
			CreatedAt:                now,
			Regions:                  append([]string{}, snap.ScannedRegions...),
			RuleFile:                 opts.RuleFile,
			ScannedResources:         len(snap.Resources),
			Resources:                []PlannedResource{},
			Conflicts:                []KeyConflict{},
			UnsetRequiredTags:        []UnsetRequiredTag{},
			RemoveCandidates:         []RemoveCandidate{},
			UnsupportedResourceTypes: []inventory.TypeCount{},
			ImpactFindings:           []ImpactFinding{},
			UnreadPolicies:           []UnreadPolicy{},
			FailedRegions:            append([]inventory.FailedRegion{}, snap.FailedRegions...),
			InventoryComplete:        snap.Complete(),
			ImpactAnalysisComplete:   true,
		},
	}
	b.keyRules = append([]rules.KeyRule(nil), rs.KeyRules...)
	sort.SliceStable(b.keyRules, func(i, j int) bool { return b.keyRules[i].CanonicalKey < b.keyRules[j].CanonicalKey })
	b.required = append([]rules.RequiredTag(nil), rs.RequiredTags...)
	sort.SliceStable(b.required, func(i, j int) bool { return b.required[i].Key < b.required[j].Key })
	for _, k := range rs.KeyRules {
		b.named[k.CanonicalKey] = true
		for _, a := range k.Aliases {
			b.named[a] = true
		}
	}
	for _, t := range rs.RequiredTags {
		b.named[t.Key] = true
	}
	for _, k := range rs.RemoveKeys {
		b.named[k.Key] = true
	}

	resources := append([]inventory.Resource(nil), snap.Resources...)
	sort.SliceStable(resources, func(i, j int) bool { return resources[i].ARN < resources[j].ARN })
	for _, res := range resources {
		if notTaggable(res) {
			b.unsupport[res.ResourceType]++
			continue
		}
		b.planResource(res)
	}
	b.finish()
	return b.plan
}

// planResource makes the changes of one resource (BR1.1 to BR1.6, BR2.3).
// after follows the tags as the changes so far leave them (BR1.9).
func (b *builder) planResource(res inventory.Resource) {
	after := make(map[string]string, len(res.Tags))
	for k, v := range res.Tags {
		after[k] = v
	}
	var changes []Change
	conflicted := map[string]bool{}
	add := func(c Change) {
		c.ResourceARN = res.ARN
		c.Mark = MarkNone
		changes = append(changes, c)
		applyChange(after, c)
	}

	// (1) For each canonical key: its renames, then its value change.
	for _, rule := range b.keyRules {
		canonical := rule.CanonicalKey
		if inventory.IsAWSReservedKey(canonical) {
			continue
		}
		_, hasCanonical := res.Tags[canonical]
		var aliases []string
		for _, a := range rule.Aliases {
			if _, ok := res.Tags[a]; ok && !inventory.IsAWSReservedKey(a) {
				aliases = append(aliases, a)
			}
		}
		if len(aliases) > 0 {
			if b.valuesDiffer(res, canonical, hasCanonical, aliases) {
				conflicted[canonical] = true
				b.plan.Conflicts = append(b.plan.Conflicts, newConflict(res, canonical, aliases, hasCanonical))
				continue
			}
			targetExists := hasCanonical
			for _, a := range aliases {
				add(Change{Kind: KindRenameKey, FromKey: a, ToKey: canonical, FromValue: res.Tags[a], MergesIntoExisting: targetExists})
				targetExists = true
			}
		}
		if v, ok := after[canonical]; ok {
			if ans := b.rs.CanonicalValue(canonical, v); ans.Changed {
				add(Change{Kind: KindChangeValue, FromKey: canonical, FromValue: v, ToValue: ans.Value})
			}
		}
	}

	// (2) Required tags that are not set, in key order (BR1.4).
	ruleRes := rules.Resource{ResourceType: res.ResourceType, Region: res.Region, Global: res.Global, Tags: res.Tags}
	for _, req := range b.required {
		key := req.Key
		if inventory.IsAWSReservedKey(key) {
			continue
		}
		if _, ok := after[key]; ok || conflicted[key] {
			continue
		}
		ans := b.rs.RequiredValue(key, ruleRes)
		if ans.Status != rules.StatusResolved {
			b.unset[key] = append(b.unset[key], res.ARN)
			continue
		}
		add(Change{Kind: KindAddRequired, ToKey: key, ToValue: b.rs.CanonicalValue(key, ans.Value).Value})
	}

	// (3) Removed keys, in key order (BR1.5); and the remove candidates
	// (BR2.3), from the tags as collected.
	keys := make([]string, 0, len(res.Tags))
	for k := range res.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if inventory.IsAWSReservedKey(k) {
			continue
		}
		if b.rs.IsRemoved(k).Remove {
			if _, ok := after[k]; ok {
				add(Change{Kind: KindRemoveKey, FromKey: k, FromValue: res.Tags[k]})
			}
			continue
		}
		if !b.named[k] {
			b.candidates[k]++
		}
	}

	if len(changes) == 0 {
		return
	}
	for i := range changes {
		b.nextID++
		changes[i].ChangeID = changeID(b.nextID)
	}
	before := make(map[string]string, len(res.Tags))
	for k, v := range res.Tags {
		before[k] = v
	}
	b.plan.Resources = append(b.plan.Resources, PlannedResource{
		ARN: res.ARN, Region: res.Region, Global: res.Global, ResourceType: res.ResourceType,
		BeforeTags: before, IaCManagedBy: res.IaCManagedBy, Changes: changes,
	})
}

// changeID formats the n-th change ID: c-000001 (BR1.8).
func changeID(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 6 {
		s = "000000"[len(s):] + s
	}
	return "c-" + s
}

// valuesDiffer reports whether the present keys of one canonical key carry
// different values once each is replaced by its canonical value (BR1.1,
// BR1.2).
func (b *builder) valuesDiffer(res inventory.Resource, canonical string, hasCanonical bool, aliases []string) bool {
	keys := aliases
	if hasCanonical {
		keys = append([]string{canonical}, aliases...)
	}
	first := b.rs.CanonicalValue(canonical, res.Tags[keys[0]]).Value
	for _, k := range keys[1:] {
		if b.rs.CanonicalValue(canonical, res.Tags[k]).Value != first {
			return true
		}
	}
	return false
}

// newConflict records the present keys in reading order: the aliases in rule
// file order, then the canonical key (as in the mockup "env=... Environment=...").
func newConflict(res inventory.Resource, canonical string, aliases []string, hasCanonical bool) KeyConflict {
	c := KeyConflict{ResourceARN: res.ARN, CanonicalKey: canonical, PresentTags: map[string]string{}}
	for _, a := range aliases {
		c.PresentTags[a] = res.Tags[a]
		c.KeyOrder = append(c.KeyOrder, a)
	}
	if hasCanonical {
		c.PresentTags[canonical] = res.Tags[canonical]
		c.KeyOrder = append(c.KeyOrder, canonical)
	}
	return c
}

// finish sorts the lists the plan keeps by key.
func (b *builder) finish() {
	p := b.plan
	for key, arns := range b.unset {
		p.UnsetRequiredTags = append(p.UnsetRequiredTags, UnsetRequiredTag{Key: key, ARNs: arns})
	}
	sort.Slice(p.UnsetRequiredTags, func(i, j int) bool { return p.UnsetRequiredTags[i].Key < p.UnsetRequiredTags[j].Key })
	for key, n := range b.candidates {
		p.RemoveCandidates = append(p.RemoveCandidates, RemoveCandidate{Key: key, ResourceCount: n})
	}
	sort.Slice(p.RemoveCandidates, func(i, j int) bool { return p.RemoveCandidates[i].Key < p.RemoveCandidates[j].Key })
	for t, n := range b.unsupport {
		p.UnsupportedResourceTypes = append(p.UnsupportedResourceTypes, inventory.TypeCount{ResourceType: t, Count: n})
	}
	sort.Slice(p.UnsupportedResourceTypes, func(i, j int) bool {
		a, c := p.UnsupportedResourceTypes[i], p.UnsupportedResourceTypes[j]
		if a.Count != c.Count {
			return a.Count > c.Count
		}
		return a.ResourceType < c.ResourceType
	})
}

// applyChange applies one change to tags (BR1.9). Keys starting with aws:
// are never touched: no change names them.
func applyChange(tags map[string]string, c Change) {
	switch c.Kind {
	case KindRenameKey:
		if !c.MergesIntoExisting {
			tags[c.ToKey] = tags[c.FromKey]
		}
		delete(tags, c.FromKey)
	case KindChangeValue:
		tags[c.FromKey] = c.ToValue
	case KindAddRequired:
		tags[c.ToKey] = c.ToValue
	case KindRemoveKey:
		delete(tags, c.FromKey)
	}
}

// AfterTags returns the tags after the changes are applied in order to
// before (BR1.9): the diff format shows them, and U6 expects them after
// writing (contract C4 plannedAfterTags). A rename that merges into an
// existing key only removes its key. before is not changed.
func AfterTags(before map[string]string, changes []Change) map[string]string {
	after := make(map[string]string, len(before))
	for k, v := range before {
		after[k] = v
	}
	for _, c := range changes {
		applyChange(after, c)
	}
	return after
}

// Changes returns every change of the plan in order.
func (p *Plan) Changes() []Change {
	n := 0
	for _, r := range p.Resources {
		n += len(r.Changes)
	}
	out := make([]Change, 0, n)
	for _, r := range p.Resources {
		out = append(out, r.Changes...)
	}
	return out
}

// ChangeCount is the number of changes.
func (p *Plan) ChangeCount() int {
	n := 0
	for _, r := range p.Resources {
		n += len(r.Changes)
	}
	return n
}

// ResourcesToChange counts the resources with at least one change; a
// resource with several changes counts once (BR2.1).
func (p *Plan) ResourcesToChange() int { return len(p.Resources) }

// ConflictResources counts the resources with at least one conflict.
func (p *Plan) ConflictResources() int {
	seen := map[string]bool{}
	for _, c := range p.Conflicts {
		seen[c.ResourceARN] = true
	}
	return len(seen)
}

// UnsetRequiredResources counts the resources with at least one required
// tag whose value is not set.
func (p *Plan) UnsetRequiredResources() int {
	seen := map[string]bool{}
	for _, u := range p.UnsetRequiredTags {
		for _, arn := range u.ARNs {
			seen[arn] = true
		}
	}
	return len(seen)
}

// NothingToDo reports whether the plan has no change, conflict or required
// tag without a value (BR2.7): "No changes needed". Remove candidates do not
// count.
func (p *Plan) NothingToDo() bool {
	return len(p.Resources) == 0 && len(p.Conflicts) == 0 && len(p.UnsetRequiredTags) == 0
}

// NeedsAnalysis reports whether the impact analysis has to run: only when
// there is a change (BR2.7). Without changes no policy is read.
func (p *Plan) NeedsAnalysis() bool { return len(p.Resources) > 0 }

// ImpactInputs converts the changes into the impact analysis input.
func (p *Plan) ImpactInputs() []TagChangeInput {
	inputs := make([]TagChangeInput, 0, p.ChangeCount())
	for _, c := range p.Changes() {
		in := TagChangeInput{
			ChangeID:    c.ChangeID,
			ResourceARN: c.ResourceARN,
			Kind:        c.Kind,
			FromKey:     c.FromKey,
			ToKey:       c.ToKey,
			FromValue:   c.FromValue,
			ToValue:     c.ToValue,
		}
		if c.Kind == KindRenameKey {
			// A rename keeps the value.
			in.ToValue = c.FromValue
		}
		inputs = append(inputs, in)
	}
	return inputs
}

// AttachImpact copies the impact result into the plan and marks the changes
// the findings name (BR2.5).
func (p *Plan) AttachImpact(r ImpactResult) {
	p.ImpactFindings = append([]ImpactFinding{}, r.Findings...)
	p.UnreadPolicies = append([]UnreadPolicy{}, r.UnreadPolicies...)
	p.ImpactAnalysisComplete = r.Complete
	p.markChanges()
}

// markChanges sets the mark and impacted of every change from the findings
// (BR2.5): a change named by an affected finding is "!", one named only by
// undecided findings is "?"; both are impacted (contract C5).
func (p *Plan) markChanges() {
	marks := map[string]Mark{}
	for _, f := range p.ImpactFindings {
		for _, id := range f.ChangeIDs {
			if f.Verdict == VerdictAffected {
				marks[id] = MarkAffected
			} else if marks[id] != MarkAffected {
				marks[id] = MarkUndecided
			}
		}
	}
	for i := range p.Resources {
		for j := range p.Resources[i].Changes {
			c := &p.Resources[i].Changes[j]
			c.Mark = MarkNone
			if m, ok := marks[c.ChangeID]; ok {
				c.Mark = m
			}
			c.Impacted = c.Mark != MarkNone
		}
	}
}

// CountFindings returns the number of affected and undecided findings.
func (p *Plan) CountFindings() (affected, undecided int) {
	for _, f := range p.ImpactFindings {
		switch f.Verdict {
		case VerdictAffected:
			affected++
		case VerdictUndecided:
			undecided++
		}
	}
	return affected, undecided
}

// NeedsReview reports whether the plan has changes, conflicts, required tags
// without a value or findings (BR2.6). Remove candidates alone do not count.
func (p *Plan) NeedsReview() bool {
	return len(p.Resources) > 0 || len(p.Conflicts) > 0 || len(p.UnsetRequiredTags) > 0 || len(p.ImpactFindings) > 0
}

// The exit codes of plan (contract C5).
const (
	ExitOK          = 0
	ExitError       = 1
	ExitNeedsReview = 2
)

// ExitCode is the exit code of the plan (BR2.6): 1 when the collection or
// the impact analysis is incomplete, otherwise 2 when there is anything to
// review, otherwise 0. The JSON output carries the same value.
func (p *Plan) ExitCode() int {
	switch {
	case !p.InventoryComplete || !p.ImpactAnalysisComplete:
		return ExitError
	case p.NeedsReview():
		return ExitNeedsReview
	default:
		return ExitOK
	}
}

// TagChangeInput is one change given to the impact analysis (contract C9). It
// is the impact analysis's own type, independent of Change.
type TagChangeInput struct {
	ChangeID    string
	ResourceARN string
	Kind        ChangeKind
	FromKey     string
	ToKey       string
	FromValue   string
	ToValue     string
}

// Verdict is the result of matching one condition.
type Verdict string

// Verdicts of an impact finding.
const (
	VerdictAffected  Verdict = "affected"
	VerdictUndecided Verdict = "undecided"
)

// The policy source kinds of contract C2.
const (
	SourceIAMManaged = "iamManaged"
	SourceIAMInline  = "iamInline"
	SourceS3Bucket   = "s3Bucket"
	SourceKMSKey     = "kmsKey"
	SourceSQSQueue   = "sqsQueue"
	SourceSNSTopic   = "snsTopic"
	SourceSecret     = "secretsManagerSecret"
)

// SourceKinds are the policy source kinds of contract C2, in the order of
// the impact findings (U5 BR5.2).
var SourceKinds = []string{SourceIAMManaged, SourceIAMInline, SourceS3Bucket, SourceKMSKey, SourceSQSQueue, SourceSNSTopic, SourceSecret}

// ImpactFinding is one condition affected by, or undecidable for, changes.
type ImpactFinding struct {
	Verdict           Verdict
	PolicyRef         string
	SourceKind        string
	StatementID       string
	ConditionOperator string
	ConditionKey      string
	ConditionValue    string
	// ChangeIDs are the changes that cause the finding, in change order.
	ChangeIDs []string
}

// UnreadPolicy is a policy, or a whole policy list, that could not be read.
type UnreadPolicy struct {
	SourceKind string
	// PolicyRef refers to one unreadable policy (U5 BR1.6), or to the user,
	// role or group whose inline policies could not be listed; it is empty
	// when a whole list could not be read.
	PolicyRef string
	// Region is the region of a resource policy or of a list of them; empty
	// for IAM and the S3 bucket list (U5).
	Region string
	// Reason names the error kind and the operation, never the error text.
	Reason string
}

// ImpactResult is the result of the impact analysis (contract C9).
type ImpactResult struct {
	Findings       []ImpactFinding
	UnreadPolicies []UnreadPolicy
	// Complete is false when any policy could not be read.
	Complete bool
}

// ImpactAnalyzer finds the policies affected by the changes (contract C9).
// The implementation holds its own policy reader, so this package does not
// depend on the AWS access package.
type ImpactAnalyzer interface {
	Analyze(ctx context.Context, changes []TagChangeInput) (ImpactResult, error)
}
