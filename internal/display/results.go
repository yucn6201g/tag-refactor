package display

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// ScanResult prints the key variant groups and the summary (screen 2 of the
// mockups, minimal version).
func (p *Printer) ScanResult(snap *inventory.Snapshot, groups []inventory.KeyVariantGroup) {
	variants := inventory.Variants(groups)
	if len(variants) == 0 {
		p.line(p.out, "No issues found")
	} else {
		p.line(p.out, "Tag key variants (same key except for case and separators)")
		for _, g := range variants {
			p.line(p.out, "  %-20s %4d %s", g.Spellings[0].Key, g.TotalResources, pluralWord(g.TotalResources, "resource"))
			for _, s := range g.Spellings {
				p.line(p.out, "    %-20s %4d", s.Key, s.ResourceCount)
			}
		}
		p.blank(p.out)
	}
	p.line(p.out, "Summary: %s scanned in %s (%s) · %s",
		plural(len(snap.Resources), "resource"), plural(len(snap.ScannedRegions), "region"),
		strings.Join(snap.ScannedRegions, ", "), plural(len(variants), "variant group"))
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// PlanResult prints the plan (screen 4 of the mockups, minimal version).
func (p *Printer) PlanResult(plan *planner.Plan) {
	if !plan.ImpactAnalysisComplete {
		p.planIncomplete(plan)
	}
	if plan.NothingToDo() {
		p.line(p.out, "No changes needed")
		p.line(p.out, "Summary: %s scanned · 0 to change", plural(plan.ScannedResources, "resource"))
		return
	}
	if len(plan.Changes) > 0 {
		p.planChanges(plan)
	}
	if len(plan.Conflicts) > 0 {
		p.planConflicts(plan)
	}
	if len(plan.ImpactFindings) > 0 {
		p.planFindings(plan)
	}
	affected, undecided := plan.CountFindings()
	p.line(p.out, "Summary: %d to change · %d conflicts · %d affected · %d undecided",
		plan.ResourcesToChange(), plan.ConflictResources(), affected, undecided)
}

func (p *Printer) planIncomplete(plan *planner.Plan) {
	p.line(p.out, "! Impact analysis is incomplete")
	for _, u := range plan.UnreadPolicies {
		if u.PolicyRef == "" {
			p.line(p.out, "    IAM managed policies: %s", u.Reason)
		} else {
			p.line(p.out, "    IAM managed policy %s: %s", u.PolicyRef, u.Reason)
		}
	}
	p.blank(p.out)
}

type renameGroup struct {
	from, to string
	merge    bool
	count    int
}

func (p *Printer) planChanges(plan *planner.Plan) {
	p.line(p.out, "Changes: %s", plural(plan.ResourcesToChange(), "resource"))
	index := map[renameGroup]int{}
	var groups []renameGroup
	for _, c := range plan.Changes {
		key := renameGroup{from: c.FromKey, to: c.ToKey, merge: c.MergesIntoExisting}
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, key)
		}
		groups[i].count++
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if a.to != b.to {
			return a.to < b.to
		}
		if a.from != b.from {
			return a.from < b.from
		}
		return !a.merge && b.merge
	})
	for _, g := range groups {
		suffix := ""
		if g.merge {
			suffix = "  (merge into existing)"
		}
		p.line(p.out, "  ~ rename key   %-30s %5d %s%s", g.from+" -> "+g.to, g.count, pluralWord(g.count, "resource"), suffix)
	}
	p.line(p.out, "  (A resource with several changes is counted once in the total.)")
	p.blank(p.out)
}

func (p *Printer) planConflicts(plan *planner.Plan) {
	p.line(p.out, "Conflicts (not changed; values differ): %s", plural(plan.ConflictResources(), "resource"))
	for _, c := range plan.Conflicts {
		parts := make([]string, 0, len(c.KeyOrder))
		for _, k := range c.KeyOrder {
			parts = append(parts, Sanitize(k)+`="`+Sanitize(c.PresentTags[k])+`"`)
		}
		p.line(p.out, "  %s   %s", c.ResourceARN, rawString(strings.Join(parts, "  ")))
	}
	p.blank(p.out)
}

func (p *Printer) planFindings(plan *planner.Plan) {
	byID := make(map[string]planner.Change, len(plan.Changes))
	for _, c := range plan.Changes {
		byID[c.ChangeID] = c
	}
	p.line(p.out, "Impact (references that use the tags being changed)")
	for _, f := range plan.ImpactFindings {
		symbol := "!"
		if f.Verdict == planner.VerdictUndecided {
			symbol = "?"
		}
		p.line(p.out, "  %s IAM policy  %s", symbol, f.PolicyRef)
		if f.ConditionValue != "" {
			p.line(p.out, "      Statement \"%s\"  Condition %s %s = \"%s\"", f.StatementID, f.ConditionOperator, f.ConditionKey, f.ConditionValue)
		} else {
			p.line(p.out, "      Statement \"%s\"  Condition %s %s", f.StatementID, f.ConditionOperator, f.ConditionKey)
		}
		if f.Verdict == planner.VerdictUndecided {
			p.line(p.out, "      cannot decide (policy variable)")
			continue
		}
		p.line(p.out, "      affected by: %s", causes(f.ChangeIDs, byID))
	}
	p.blank(p.out)
}

// causes lists the distinct renames behind a finding, in change order.
func causes(ids []string, byID map[string]planner.Change) string {
	seen := map[string]bool{}
	var parts []string
	for _, id := range ids {
		c := byID[id]
		d := fmt.Sprintf("rename key %s -> %s", c.FromKey, c.ToKey)
		if !seen[d] {
			seen[d] = true
			parts = append(parts, d)
		}
	}
	return strings.Join(parts, ", ")
}

// rawString marks a string that has already been sanitized piece by piece,
// so line does not sanitize it again (which would double the backslashes).
type rawString string

func (r rawString) String() string { return string(r) }
