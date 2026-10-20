// Package planner builds the change plan: key renames, key conflicts and
// change IDs (contract C2, minimal U1 version). It also defines the impact
// analysis interface and its input and output types (contract C9); the impact
// package implements it, so the dependency points from impact to planner.
package planner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// ChangeKind is the kind of a change. U1 only renames keys.
type ChangeKind string

// KindRenameKey renames a tag key and keeps its value.
const KindRenameKey ChangeKind = "renameKey"

// Change is one change to one key of one resource.
type Change struct {
	ChangeID    string
	ResourceARN string
	Kind        ChangeKind
	FromKey     string
	ToKey       string
	Value       string
	// MergesIntoExisting is true when ToKey already exists on the resource at
	// the moment the change is applied (BR4.2).
	MergesIntoExisting bool
	// Impacted is true when an impact finding names this change.
	Impacted bool
}

// KeyConflict is a resource whose canonical key and aliases carry different
// values, so nothing is changed for that key (BR4.2).
type KeyConflict struct {
	ResourceARN  string
	CanonicalKey string
	PresentTags  map[string]string
	// KeyOrder lists the keys of PresentTags for display: the aliases in rule
	// file order, then the canonical key.
	KeyOrder []string
}

// Plan is the result of one plan command.
type Plan struct {
	PlanID           string
	AccountID        string
	CreatedAt        time.Time
	Regions          []string
	ScannedResources int
	Changes          []Change
	Conflicts        []KeyConflict
	ImpactFindings   []ImpactFinding
	UnreadPolicies   []UnreadPolicy
	// InventoryComplete is always true in U1: a failed read stops the command.
	InventoryComplete      bool
	ImpactAnalysisComplete bool
}

// Build creates the renames and conflicts for the snapshot under the rule set
// (BR4.1 to BR4.3). Changes are numbered c-000001 onward in ARN order, then
// canonical key order, so the same input always gives the same plan.
func Build(snap *inventory.Snapshot, rs *rules.RuleSet, now time.Time) *Plan {
	p := &Plan{
		PlanID:                 "plan-" + now.UTC().Format("20060102T150405Z"),
		AccountID:              snap.AccountID,
		CreatedAt:              now.UTC(),
		Regions:                append([]string(nil), snap.ScannedRegions...),
		ScannedResources:       len(snap.Resources),
		Changes:                []Change{},
		Conflicts:              []KeyConflict{},
		ImpactFindings:         []ImpactFinding{},
		UnreadPolicies:         []UnreadPolicy{},
		InventoryComplete:      true,
		ImpactAnalysisComplete: true,
	}
	keyRules := append([]rules.KeyRule(nil), rs.KeyRules...)
	sort.SliceStable(keyRules, func(i, j int) bool { return keyRules[i].CanonicalKey < keyRules[j].CanonicalKey })
	resources := append([]inventory.Resource(nil), snap.Resources...)
	sort.SliceStable(resources, func(i, j int) bool { return resources[i].ARN < resources[j].ARN })

	for _, res := range resources {
		for _, rule := range keyRules {
			p.planKey(res, rule)
		}
	}
	return p
}

// planKey applies one key rule to one resource.
func (p *Plan) planKey(res inventory.Resource, rule rules.KeyRule) {
	_, hasCanonical := res.Tags[rule.CanonicalKey]
	var present []string
	if hasCanonical {
		present = append(present, rule.CanonicalKey)
	}
	for _, a := range rule.Aliases {
		if _, ok := res.Tags[a]; ok {
			present = append(present, a)
		}
	}
	if len(present) == 0 || (len(present) == 1 && hasCanonical) {
		return
	}
	value := res.Tags[present[0]]
	for _, k := range present[1:] {
		if res.Tags[k] != value {
			p.Conflicts = append(p.Conflicts, newConflict(res, rule.CanonicalKey, present, hasCanonical))
			return
		}
	}
	targetExists := hasCanonical
	for _, k := range present {
		if k == rule.CanonicalKey {
			continue
		}
		p.Changes = append(p.Changes, Change{
			ChangeID:           fmt.Sprintf("c-%06d", len(p.Changes)+1),
			ResourceARN:        res.ARN,
			Kind:               KindRenameKey,
			FromKey:            k,
			ToKey:              rule.CanonicalKey,
			Value:              value,
			MergesIntoExisting: targetExists,
		})
		targetExists = true
	}
}

// newConflict records the present keys in reading order: the aliases in rule
// file order, then the canonical key (as in the mockup "env=... Environment=...").
func newConflict(res inventory.Resource, canonical string, present []string, hasCanonical bool) KeyConflict {
	c := KeyConflict{ResourceARN: res.ARN, CanonicalKey: canonical, PresentTags: map[string]string{}}
	for _, k := range present {
		c.PresentTags[k] = res.Tags[k]
		if k != canonical {
			c.KeyOrder = append(c.KeyOrder, k)
		}
	}
	if hasCanonical {
		c.KeyOrder = append(c.KeyOrder, canonical)
	}
	return c
}

// ResourcesToChange counts the resources with at least one change; a
// resource with several changes counts once (BR4.3).
func (p *Plan) ResourcesToChange() int {
	seen := map[string]bool{}
	for _, c := range p.Changes {
		seen[c.ResourceARN] = true
	}
	return len(seen)
}

// ConflictResources counts the resources with at least one conflict.
func (p *Plan) ConflictResources() int {
	seen := map[string]bool{}
	for _, c := range p.Conflicts {
		seen[c.ResourceARN] = true
	}
	return len(seen)
}

// NothingToDo reports whether the plan has neither changes nor conflicts
// (BR4.4); the impact analysis is then skipped.
func (p *Plan) NothingToDo() bool {
	return len(p.Changes) == 0 && len(p.Conflicts) == 0
}

// ImpactInputs converts the changes into the impact analysis input.
func (p *Plan) ImpactInputs() []TagChangeInput {
	inputs := make([]TagChangeInput, 0, len(p.Changes))
	for _, c := range p.Changes {
		inputs = append(inputs, TagChangeInput{
			ChangeID:    c.ChangeID,
			ResourceARN: c.ResourceARN,
			Kind:        c.Kind,
			FromKey:     c.FromKey,
			ToKey:       c.ToKey,
			FromValue:   c.Value,
			ToValue:     c.Value,
		})
	}
	return inputs
}

// AttachImpact copies the impact result into the plan and marks every change
// named by a finding as impacted (BR5.6).
func (p *Plan) AttachImpact(r ImpactResult) {
	p.ImpactFindings = append([]ImpactFinding{}, r.Findings...)
	p.UnreadPolicies = append([]UnreadPolicy{}, r.UnreadPolicies...)
	p.ImpactAnalysisComplete = r.Complete
	impacted := map[string]bool{}
	for _, f := range r.Findings {
		for _, id := range f.ChangeIDs {
			impacted[id] = true
		}
	}
	for i := range p.Changes {
		p.Changes[i].Impacted = impacted[p.Changes[i].ChangeID]
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

// NeedsReview reports whether the plan has changes, conflicts or findings.
func (p *Plan) NeedsReview() bool {
	return len(p.Changes) > 0 || len(p.Conflicts) > 0 || len(p.ImpactFindings) > 0
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

// SourceIAMManaged is the policy source kind of customer managed IAM policies.
const SourceIAMManaged = "iamManaged"

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
	// PolicyRef is the ARN of one unreadable policy, empty when the list
	// itself could not be read.
	PolicyRef string
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
