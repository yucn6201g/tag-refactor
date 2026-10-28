package impact

import (
	"slices"
	"sort"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Grouping the verdicts into findings and ordering the result (U5 BR5.1,
// BR5.2).

// group is the place of a finding in a policy: a condition (operator and
// key) or the Resource or NotResource element of a statement.
type group struct {
	stmt int
	// place orders conditions before Resource and NotResource (BR5.2).
	place    int
	operator string
	key      string
}

const (
	placeCondition = iota
	placeResource
	placeNotResource
)

// groupOf returns the group of a reference and the value shown with it.
func groupOf(stmt int, r *reference) (group, string) {
	switch r.loc {
	case locResource, locNotResource:
		place := placeResource
		if r.loc == locNotResource {
			place = placeNotResource
		}
		return group{stmt: stmt, place: place, key: r.resource.name}, strings.Join(r.resource.values, ", ")
	}
	return group{stmt: stmt, place: placeCondition, operator: r.cond.operator, key: r.cond.key}, r.cond.value()
}

// policyFindings collects the verdicts of one policy per group.
type policyFindings struct {
	src    source
	order  []group
	byKey  map[group]*groupFindings
	stmtID []string
}

type groupFindings struct {
	value     string
	affected  map[int]bool
	undecided map[int]bool
}

func newPolicyFindings(src source) *policyFindings {
	return &policyFindings{src: src, byKey: map[group]*groupFindings{}}
}

// add records the verdict of change i for the reference.
func (p *policyFindings) add(stmt int, r *reference, i int, v verdict) {
	if v == noImpact {
		return
	}
	g, value := groupOf(stmt, r)
	f, ok := p.byKey[g]
	if !ok {
		f = &groupFindings{value: value, affected: map[int]bool{}, undecided: map[int]bool{}}
		p.byKey[g] = f
		p.order = append(p.order, g)
	}
	if v == affected {
		f.affected[i] = true
	} else {
		f.undecided[i] = true
	}
}

// finding is a finding before the change indexes become change IDs.
type finding struct {
	src     source
	grp     group
	stmtID  string
	value   string
	verdict planner.Verdict
	ids     []int
}

// findings returns the findings of the policy: one per group and verdict,
// with a change that is both affected and undecided in the same group kept
// as affected only (BR5.1).
func (p *policyFindings) findings() []finding {
	var out []finding
	for _, g := range p.order {
		f := p.byKey[g]
		if ids := sortedIDs(f.affected, nil); len(ids) > 0 {
			out = append(out, finding{src: p.src, grp: g, stmtID: p.stmtID[g.stmt], value: f.value, verdict: planner.VerdictAffected, ids: ids})
		}
		if ids := sortedIDs(f.undecided, f.affected); len(ids) > 0 {
			out = append(out, finding{src: p.src, grp: g, stmtID: p.stmtID[g.stmt], value: f.value, verdict: planner.VerdictUndecided, ids: ids})
		}
	}
	return out
}

// sortedIDs returns the members of set that are not in except, in change
// order.
func sortedIDs(set, except map[int]bool) []int {
	ids := make([]int, 0, len(set))
	for i := range set {
		if !except[i] {
			ids = append(ids, i)
		}
	}
	sort.Ints(ids)
	return ids
}

// kindOrder is the position of a source kind in contract C2's order.
func kindOrder(kind string) int {
	if i := slices.Index(planner.SourceKinds, kind); i >= 0 {
		return i
	}
	return len(planner.SourceKinds)
}

// sortFindings orders the findings by the seven keys of BR5.2: source kind,
// policy reference, statement, place, operator, condition key, verdict.
func sortFindings(fs []finding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		switch {
		case a.src.kind != b.src.kind:
			return kindOrder(a.src.kind) < kindOrder(b.src.kind)
		case a.src.ref != b.src.ref:
			return a.src.ref < b.src.ref
		case a.grp.stmt != b.grp.stmt:
			return a.grp.stmt < b.grp.stmt
		case a.grp.place != b.grp.place:
			return a.grp.place < b.grp.place
		case a.grp.operator != b.grp.operator:
			return a.grp.operator < b.grp.operator
		case a.grp.key != b.grp.key:
			return a.grp.key < b.grp.key
		}
		return a.verdict == planner.VerdictAffected && b.verdict != planner.VerdictAffected
	})
}

// sortUnread orders the unread policies by source kind, region and policy
// reference; the empty region and reference come first (BR5.2).
func sortUnread(us []planner.UnreadPolicy) {
	sort.SliceStable(us, func(i, j int) bool {
		a, b := us[i], us[j]
		switch {
		case a.SourceKind != b.SourceKind:
			return kindOrder(a.SourceKind) < kindOrder(b.SourceKind)
		case a.Region != b.Region:
			return a.Region < b.Region
		case a.PolicyRef != b.PolicyRef:
			return a.PolicyRef < b.PolicyRef
		}
		return a.Reason < b.Reason
	})
}

// toImpactFinding turns a finding into the contract's form (C9).
func (f finding) toImpactFinding(changes []planner.TagChangeInput) planner.ImpactFinding {
	ids := make([]string, len(f.ids))
	for k, i := range f.ids {
		ids[k] = changes[i].ChangeID
	}
	return planner.ImpactFinding{
		Verdict:           f.verdict,
		PolicyRef:         f.src.ref,
		SourceKind:        f.src.kind,
		StatementID:       f.stmtID,
		ConditionOperator: f.grp.operator,
		ConditionKey:      f.grp.key,
		ConditionValue:    f.value,
		ChangeIDs:         ids,
	}
}
