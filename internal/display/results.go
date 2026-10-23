package display

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// ScanResult prints the result of scan (screen 2 of the mockups): the regions
// that could not be read, the key variant groups, the missing required tags,
// the never-tagged resources and the summary. Sections are separated by a
// blank line.
func (p *Printer) ScanResult(r *inventory.ScanReport) {
	p.locked(func() {
		p.endProgress()
		snap := r.Snapshot
		p.resultsIncomplete(snap.FailedRegions)
		if !r.IssuesFound() {
			if d := snap.Discovery; d != nil && d.Status != inventory.StatusComplete {
				p.line(p.out, "No issues found (untagged-resource discovery: %s)", d.Status)
			} else {
				p.line(p.out, "No issues found")
			}
		}
		if variants := inventory.Variants(r.Groups); len(variants) > 0 {
			p.scanVariants(variants)
			p.blank(p.out)
		}
		p.scanRequired(r)
		p.blank(p.out)
		p.scanDiscovery(r)
		p.blank(p.out)
		summary := fmt.Sprintf("Summary: %s scanned in %d of %d regions · %s",
			plural(len(snap.Resources), "resource"), snap.DoneRegions(), len(snap.EnabledRegions),
			plural(inventory.CountVariants(r.Groups), "variant group"))
		if r.Missing != nil {
			summary += " · " + plural(r.MissingKeys(), "required tag") + " missing"
		}
		p.line(p.out, "%s", rawString(summary))
	})
}

func (p *Printer) scanVariants(variants []inventory.KeyVariantGroup) {
	p.line(p.out, "Tag key variants (candidates; confirm them in your rule file)")
	for _, g := range variants {
		mark := ""
		if g.AbbreviationOnly {
			mark = "  (abbreviation match)"
		}
		p.line(p.out, "  %-20s %4d %s%s", g.NormalizedKey, g.TotalResources, pluralWord(g.TotalResources, "resource"), mark)
		for _, s := range g.Spellings {
			p.line(p.out, "    %-20s %4d", s.Key, s.ResourceCount)
		}
	}
}

// scanRequired prints the missing required tags (BR5.2).
func (p *Printer) scanRequired(r *inventory.ScanReport) {
	if r.Missing == nil {
		p.line(p.out, "Required tags missing: not checked (no rule file)")
		return
	}
	var listed []inventory.MissingRequiredTag
	for _, m := range r.Missing {
		if m.ResourceCount > 0 || m.NotTaggableCount > 0 {
			listed = append(listed, m)
		}
	}
	if len(listed) == 0 {
		p.line(p.out, "Required tags missing (from %s): none", r.RulesPath)
		return
	}
	p.line(p.out, "Required tags missing (from %s)", r.RulesPath)
	for _, m := range listed {
		more := ""
		if m.NotTaggableCount > 0 {
			more = fmt.Sprintf("  (%d more on resource types that cannot be tagged)", m.NotTaggableCount)
		}
		p.line(p.out, "  %-12s %4d %s%s", m.Key, m.ResourceCount, pluralWord(m.ResourceCount, "resource"), more)
	}
}

// scanDiscovery prints the never-tagged resources, the state of their
// discovery and the types that cannot be tagged (BR3.2, BR3.3, BR5.5).
func (p *Printer) scanDiscovery(r *inventory.ScanReport) {
	p.line(p.out, "Resources with no user tags: %d", r.NeverTagged)
	if d := r.Snapshot.Discovery; d != nil {
		p.line(p.out, "Untagged-resource discovery: %s", rawString(discoveryState(d)))
		if d.Status != inventory.StatusComplete && r.Missing != nil {
			p.line(p.out, "  Required tag counts are a lower bound.")
		}
	}
	if len(r.Unsupported) == 0 {
		return
	}
	total := 0
	for _, t := range r.Unsupported {
		total += t.Count
	}
	p.line(p.out, "Resource types not supported for tag changes: %s, %s",
		plural(len(r.Unsupported), "type"), plural(total, "resource"))
	for _, t := range r.Unsupported {
		p.line(p.out, "  %-20s %4d", t.ResourceType, t.Count)
	}
}

// discoveryState is "complete", or the status with the regions that were not
// checked and why, for example "partial (not checked in me-south-1:
// AccessDenied resource-explorer-2:Search)". Its parts are sanitized here.
func discoveryState(d *inventory.Discovery) string {
	groups := d.NotChecked()
	if len(groups) == 0 {
		return d.Status
	}
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		regions := make([]string, 0, len(g.Regions))
		for _, r := range g.Regions {
			regions = append(regions, Sanitize(r))
		}
		parts = append(parts, strings.Join(regions, ", ")+": "+Sanitize(g.Reason))
	}
	if d.Status == inventory.StatusUnavailable {
		return d.Status + " (" + strings.Join(parts, "; ") + ")"
	}
	return d.Status + " (not checked in " + strings.Join(parts, "; ") + ")"
}

// ResultsIncomplete prints, at the top of standard output, the regions the
// tagging API could not read and why (BR4.1, BR5.6). It prints nothing when
// every region was read.
func (p *Printer) ResultsIncomplete(failed []inventory.FailedRegion) {
	p.locked(func() {
		p.endProgress()
		p.resultsIncomplete(failed)
	})
}

func (p *Printer) resultsIncomplete(failed []inventory.FailedRegion) {
	if len(failed) == 0 {
		return
	}
	p.line(p.out, "! Results are incomplete")
	for _, f := range failed {
		p.line(p.out, "    %s: %s (%s)", f.Region, f.Kind, f.Operation)
	}
	p.blank(p.out)
}

// ScanScope prints to standard error what scan read (BR5.7), so that "No
// issues found" is read within that scope.
func (p *Printer) ScanScope(snap *inventory.Snapshot) {
	p.locked(func() {
		line := taggedScope(snap) + "; " + discoveryScope(snap.Discovery)
		p.line(p.errOut, "Checked: %s", rawString(line))
	})
}

func taggedScope(snap *inventory.Snapshot) string {
	return fmt.Sprintf("tagged resources in %d of %d regions (tagging API)", snap.DoneRegions(), len(snap.EnabledRegions))
}

// discoveryScope says how never-tagged resources were looked for. The AWS
// Config path never covers the global IAM types (reliability design §3).
func discoveryScope(d *inventory.Discovery) string {
	if d == nil {
		return "never-tagged resources were not looked for"
	}
	config, explorer, aggregator, notChecked := d.MethodCounts()
	var parts []string
	if config > 0 {
		parts = append(parts, "AWS Config in "+plural(config, "region")+" (without the global IAM types)")
	}
	if explorer > 0 {
		parts = append(parts, "Resource Explorer in "+plural(explorer, "region"))
	}
	if aggregator > 0 {
		parts = append(parts, "the Resource Explorer aggregator index for "+plural(aggregator, "region"))
	}
	line := "never-tagged resources not checked"
	if len(parts) > 0 {
		line = "never-tagged resources via " + strings.Join(parts, ", ")
	}
	if notChecked > 0 && len(parts) > 0 {
		line += "; not checked in " + plural(notChecked, "region")
	}
	return line
}

// PlanScope prints to standard error what plan read (BR5.7), so that "0
// affected" is read within that scope: the resources the tagging API returns
// and, when there were changes to analyze, the customer managed IAM policies
// attached to a user, role or group (not inline, resource-based or
// organization policies).
func (p *Printer) PlanScope(snap *inventory.Snapshot, policiesRead bool) {
	p.locked(func() {
		if policiesRead {
			p.line(p.errOut, "Checked: %s, and customer managed IAM policies attached to a user, role or group", rawString(taggedScope(snap)))
			return
		}
		p.line(p.errOut, "Checked: %s; no IAM policies were read: nothing to change", rawString(taggedScope(snap)))
	})
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// PlanResult prints the plan (screen 4 of the mockups, minimal version).
func (p *Printer) PlanResult(plan *planner.Plan) {
	p.locked(func() {
		p.endProgress()
		p.planResult(plan)
	})
}

func (p *Printer) planResult(plan *planner.Plan) {
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
