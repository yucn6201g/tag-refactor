package display

import (
	"sort"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// The diff, JSON and Markdown formats of plan (BR3.3 to BR3.5).

// diffBlock is one resource of the diff: its changes, conflicts and the
// required tags whose value is not set.
type diffBlock struct {
	arn       string
	resource  *planner.PlannedResource
	conflicts []planner.KeyConflict
	unset     []string
}

// diffBlocks gathers every resource with a change, a conflict or a required
// tag without a value, in ARN order (BR3.3).
func diffBlocks(plan *planner.Plan) []*diffBlock {
	byARN := map[string]*diffBlock{}
	get := func(arn string) *diffBlock {
		b, ok := byARN[arn]
		if !ok {
			b = &diffBlock{arn: arn}
			byARN[arn] = b
		}
		return b
	}
	for i := range plan.Resources {
		get(plan.Resources[i].ARN).resource = &plan.Resources[i]
	}
	for _, c := range plan.Conflicts {
		b := get(c.ResourceARN)
		b.conflicts = append(b.conflicts, c)
	}
	for _, u := range plan.UnsetRequiredTags {
		for _, arn := range u.ARNs {
			b := get(arn)
			b.unset = append(b.unset, u.Key)
		}
	}
	blocks := make([]*diffBlock, 0, len(byARN))
	for _, b := range byARN {
		blocks = append(blocks, b)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].arn < blocks[j].arn })
	return blocks
}

// iacTag is the IaC mark of a resource header: [IaC: CloudFormation stack
// "logging"].
func (r *results) iacTag(managedBy string) string {
	if stack, ok := strings.CutPrefix(managedBy, "cloudformation:"); ok {
		return r.tr("diff.iac", Sanitize(stack))
	}
	return r.tr("diff.iacOther", Sanitize(managedBy))
}

// markSymbol is the first column of a diff line: ! for an affected change,
// ? for one that cannot be decided (BR2.5).
func markSymbol(m planner.Mark) (string, style) {
	switch m {
	case planner.MarkAffected:
		return "!", styleImpact
	case planner.MarkUndecided:
		return "?", styleUndecided
	}
	return " ", styleNone
}

// stronger returns the stronger of two marks: affected, then undecided.
func stronger(a, b planner.Mark) planner.Mark {
	if a == planner.MarkAffected || b == planner.MarkAffected {
		return planner.MarkAffected
	}
	if a == planner.MarkUndecided || b == planner.MarkUndecided {
		return planner.MarkUndecided
	}
	return planner.MarkNone
}

// diffLine is one line of a resource: a key that goes (-), comes (+), or a
// rename that keeps the value (~).
type diffLine struct {
	mark   planner.Mark
	symbol string
	key    string
	value  string
	// rename is "from -> to" for a ~ line.
	rename string
}

// resourceLines compares the tags before and after the changes (BR1.9) and
// lists the keys that differ, in the order the changes touch them. A rename
// whose value stays the same is one ~ line.
func resourceLines(res *planner.PlannedResource) []diffLine {
	before := res.BeforeTags
	after := planner.AfterTags(before, res.Changes)
	marks := map[string]planner.Mark{}
	var order []string
	touch := func(k string, m planner.Mark) {
		if _, ok := marks[k]; !ok {
			order = append(order, k)
			marks[k] = planner.MarkNone
		}
		marks[k] = stronger(marks[k], m)
	}
	for _, c := range res.Changes {
		switch c.Kind {
		case planner.KindRenameKey:
			touch(c.FromKey, c.Mark)
			touch(c.ToKey, c.Mark)
		case planner.KindAddRequired:
			touch(c.ToKey, c.Mark)
		default:
			touch(c.FromKey, c.Mark)
		}
	}
	// keptRename is the rename of the key, if any, that moves a value to a
	// key that did not exist and is not changed afterwards: one ~ line.
	keptRename := func(k string) (planner.Change, bool) {
		for _, c := range res.Changes {
			if c.Kind != planner.KindRenameKey || (c.FromKey != k && c.ToKey != k) {
				continue
			}
			_, fromAfter := after[c.FromKey]
			_, toBefore := before[c.ToKey]
			if v, ok := after[c.ToKey]; ok && !fromAfter && !toBefore && v == before[c.FromKey] {
				return c, true
			}
		}
		return planner.Change{}, false
	}
	done := map[string]bool{}
	var out []diffLine
	for _, k := range order {
		if done[k] {
			continue
		}
		if c, ok := keptRename(k); ok {
			done[c.FromKey], done[c.ToKey] = true, true
			out = append(out, diffLine{mark: stronger(marks[c.FromKey], marks[c.ToKey]), symbol: "~",
				rename: Sanitize(c.FromKey) + " -> " + Sanitize(c.ToKey), value: before[c.FromKey]})
			continue
		}
		done[k] = true
		b, inBefore := before[k]
		a, inAfter := after[k]
		switch {
		case inBefore && !inAfter:
			out = append(out, diffLine{mark: marks[k], symbol: "-", key: k, value: b})
		case !inBefore && inAfter:
			out = append(out, diffLine{mark: marks[k], symbol: "+", key: k, value: a})
		case inBefore && inAfter && a != b:
			out = append(out, diffLine{mark: marks[k], symbol: "-", key: k, value: b}, diffLine{mark: marks[k], symbol: "+", key: k, value: a})
		}
	}
	return out
}

func (r *results) planDiff(plan *planner.Plan, s planner.PlanSummary) {
	r.planIncomplete(plan)
	if plan.NothingToDo() {
		r.planNothing(plan)
		return
	}
	for _, b := range diffBlocks(plan) {
		header := Sanitize(b.arn)
		if b.resource != nil && b.resource.IaCManagedBy != "" {
			header += "   " + r.iacTag(b.resource.IaCManagedBy)
		}
		r.text(styleNone, header)
		if b.resource != nil {
			lines := resourceLines(b.resource)
			width := 0
			for _, l := range lines {
				if l.rename == "" {
					width = max(width, Width(Sanitize(l.key)))
				}
			}
			for _, l := range lines {
				mark, markStyle := markSymbol(l.mark)
				st := styleChange
				switch l.symbol {
				case "+":
					st = styleAdd
				case "-":
					st = styleRemove
				}
				if markStyle != styleNone {
					st = markStyle
				}
				body := l.rename
				if body == "" {
					body = padRight(Sanitize(l.key), width)
				}
				r.text(st, mark+" "+l.symbol+" "+body+` = "`+Sanitize(l.value)+`"`)
			}
		}
		for _, c := range b.conflicts {
			r.text(styleImpact, "  "+r.tr("diff.conflict", conflictTags(c)))
		}
		for _, k := range b.unset {
			r.text(styleImpact, "  "+r.tr("diff.unset", Sanitize(k)))
		}
		r.blank()
	}
	r.line(styleNone, "diff.changes", rawString(r.count("unit.resource", s.ResourcesToChange)),
		FormatCount(s.ChangesByKind[planner.KindRenameKey]), FormatCount(s.ChangesByKind[planner.KindChangeValue]),
		FormatCount(s.ChangesByKind[planner.KindAddRequired]), FormatCount(s.ChangesByKind[planner.KindRemoveKey]))
	r.text(styleNone, r.planSummaryLine(s))
}

// planJSON writes contract C3 for plan: the details hold the fields of the
// saved plan (contract C2), written through the same SafeJSONWriter, so the
// two agree byte for byte (NFR8.15, NFR9.6). JSON has no colors, keeps plain
// numbers and is never translated.
func (r *results) planJSON(plan *planner.Plan, s planner.PlanSummary, out Output) {
	w := planner.NewSafeJSONWriter(r.w)
	o := planner.NewJSONObject(w, "")
	o.Field("version", 1)
	o.Field("toolVersion", out.ToolVersion)
	o.Field("command", "plan")
	o.Field("exitCode", out.ExitCode)
	o.Field("accountId", plan.AccountID)
	o.Object("complete", func(c *planner.JSONObject) {
		c.Field("inventory", plan.InventoryComplete)
		c.Field("impactAnalysis", plan.ImpactAnalysisComplete)
	})
	failedRegionsJSON(o, plan.FailedRegions)
	types := planner.TypeCountsJSON(plan.UnsupportedResourceTypes)
	o.Array("unsupportedResourceTypes", len(types), func(i int) any { return types[i] })
	o.Object("summary", func(c *planner.JSONObject) {
		c.Field("scannedResources", s.ScannedResources)
		c.Field("resourcesToChange", s.ResourcesToChange)
		c.Field("changesByKind", s.ChangesByKind)
		c.Field("conflictResources", s.ConflictResources)
		c.Field("unsetRequiredResources", s.UnsetRequiredResources)
		c.Field("affectedReferences", s.AffectedReferences)
		c.Field("undecidedReferences", s.UndecidedReferences)
		c.Field("iacManagedResources", s.IaCManagedResources)
	})
	o.Object("details", func(d *planner.JSONObject) {
		d.Field("planId", plan.PlanID)
		d.Field("createdAt", planner.FormatTime(plan.CreatedAt))
		d.Field("regions", nonNilStrings(plan.Regions))
		planner.WriteDetails(d, plan)
	})
	_ = o.Close()
	_, _ = w.Write([]byte("\n"))
	_ = w.Close()
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// planMarkdown writes the plan as Markdown (BR3.5): the sections of the
// table as headings and tables, the changes of every resource in a folded
// table, and the summary last.
func (r *results) planMarkdown(plan *planner.Plan, s planner.PlanSummary) {
	r.mdLine("md.target")
	r.mdLine("md.targetLine", mdSpan(plan.AccountID), mdTool(r.count("unit.region", len(plan.Regions))),
		mdTool(planner.FormatTime(plan.CreatedAt)))
	r.blank()
	r.mdIncompleteRegions(plan.FailedRegions)
	if !plan.ImpactAnalysisComplete {
		r.mdLine("md.impactInc")
		r.mdTable("md.unreadHead")
		for _, u := range plan.UnreadPolicies {
			var ref mdArg = mdTool(r.tr("md.allPolicies"))
			if u.PolicyRef != "" {
				ref = mdCell(u.PolicyRef)
			}
			r.mdRow(ref, mdCell(u.Reason))
		}
		r.blank()
	}
	if plan.NothingToDo() {
		if plan.InventoryComplete {
			r.mdLine("plan.noChanges")
			r.blank()
		}
	} else {
		r.mdChanges(plan, s)
	}
	if len(plan.Conflicts) > 0 {
		r.mdHeading("md.conflicts", "unit.resource", s.ConflictResources)
		r.mdTable("md.conflictHead")
		for _, c := range plan.Conflicts {
			parts := make([]mdArg, 0, len(c.KeyOrder))
			for _, k := range c.KeyOrder {
				parts = append(parts, mdTool(mdCell(k).mdText()+" = "+mdCell(c.PresentTags[k]).mdText()))
			}
			r.mdRow(mdCell(c.ResourceARN), mdJoin(r.tr("list.separator"), parts))
		}
		r.blank()
	}
	if len(plan.UnsetRequiredTags) > 0 {
		r.mdHeading("md.unset", "unit.resource", s.UnsetRequiredResources)
		r.mdLine("md.unsetNote")
		r.blank()
		r.mdTable("md.keyHead")
		n := 0
		for _, u := range plan.UnsetRequiredTags {
			r.mdRow(mdCell(u.Key), mdCount(len(u.ARNs)))
			n += len(u.ARNs)
		}
		r.blank()
		r.mdLine("md.unsetDetails", mdCount(n))
		r.blank()
		r.mdTable("md.keyARNHead")
		for _, u := range plan.UnsetRequiredTags {
			for _, arn := range u.ARNs {
				r.mdRow(mdCell(u.Key), mdCell(arn))
			}
		}
		r.blank()
		_, _ = r.w.WriteString("</details>\n\n")
	}
	if len(plan.RemoveCandidates) > 0 {
		r.mdHeading("md.candidates", "unit.key", len(plan.RemoveCandidates))
		r.mdLine("md.candNote")
		r.blank()
		r.mdTable("md.keyHead")
		for _, c := range plan.RemoveCandidates {
			r.mdRow(mdCell(c.Key), mdCount(c.ResourceCount))
		}
		r.blank()
	}
	if s.IaCManagedResources > 0 {
		r.mdHeading("md.iac", "unit.resource", s.IaCManagedResources)
		r.mdLine("md.iacNote")
		r.blank()
	}
	if len(plan.ImpactFindings) > 0 {
		r.mdFindings(plan)
	}
	r.mdLine("md.summary")
	_, _ = r.w.WriteString(r.planSummaryLine(s) + "\n")
}

// mdIncompleteRegions writes the regions that could not be read.
func (r *results) mdIncompleteRegions(failed []inventory.FailedRegion) {
	if len(failed) == 0 {
		return
	}
	r.mdLine("md.incomplete")
	r.mdTable("md.regionHead")
	for _, f := range failed {
		r.mdRow(mdCell(f.Region), mdTool(mdCell(f.Kind).mdText()+" ("+mdCell(f.Operation).mdText()+")"))
	}
	r.blank()
}

// mdChange describes one change of a resource row.
func mdChange(c planner.Change) mdArg {
	switch c.Kind {
	case planner.KindRenameKey:
		return mdTool(mdCell(c.FromKey).mdText() + " → " + mdCell(c.ToKey).mdText())
	case planner.KindChangeValue:
		return mdTool(mdCell(c.FromKey).mdText() + ": " + mdCell(c.FromValue).mdText() + " → " + mdCell(c.ToValue).mdText())
	case planner.KindAddRequired:
		return mdTool("+ " + mdCell(c.ToKey).mdText() + " = " + mdCell(c.ToValue).mdText())
	}
	return mdTool("- " + mdCell(c.FromKey).mdText())
}

func (r *results) mdChanges(plan *planner.Plan, s planner.PlanSummary) {
	if len(s.ChangeLines) == 0 {
		return
	}
	r.mdHeading("md.changes", "unit.resource", s.ResourcesToChange)
	r.mdTable("md.changesHead")
	for _, l := range s.ChangeLines {
		var content mdArg
		switch l.Kind {
		case planner.KindRenameKey:
			content = mdTool(mdCell(l.FromKey).mdText() + " → " + mdCell(l.ToKey).mdText())
		case planner.KindChangeValue:
			content = mdTool(mdCell(l.FromKey).mdText() + ": " + mdCell(l.FromValue).mdText() + " → " + mdCell(l.ToValue).mdText())
		case planner.KindAddRequired:
			value := mdArg(mdTool(r.tr("plan.fromRules")))
			if !l.SeveralValues {
				value = mdCell(l.ToValue)
			}
			content = mdTool(mdCell(l.ToKey).mdText() + " = " + value.mdText())
		default:
			content = mdCell(l.FromKey)
		}
		r.mdRow(mdTool(r.tr("kind."+string(l.Kind))), content, mdCount(l.Resources))
	}
	r.blank()
	r.mdLine("md.byResource", mdCount(len(plan.Resources)))
	r.blank()
	r.mdTable("md.resourceHead")
	for _, res := range plan.Resources {
		parts := make([]mdArg, 0, len(res.Changes))
		mark := planner.MarkNone
		for _, c := range res.Changes {
			parts = append(parts, mdChange(c))
			mark = stronger(mark, c.Mark)
		}
		var iac mdArg = mdTool("")
		if res.IaCManagedBy != "" {
			name := res.IaCManagedBy
			if stack, ok := strings.CutPrefix(name, "cloudformation:"); ok {
				name = stack
			}
			iac = mdCell(name)
		}
		impact, _ := markSymbol(mark)
		r.mdRow(mdCell(res.ARN), mdJoin(r.tr("list.semicolon"), parts), iac, mdTool(strings.TrimSpace(impact)))
	}
	r.blank()
	_, _ = r.w.WriteString("</details>\n\n")
}

func (r *results) mdFindings(plan *planner.Plan) {
	byID := make(map[string]planner.Change, plan.ChangeCount())
	for _, c := range plan.Changes() {
		byID[c.ChangeID] = c
	}
	r.mdLine("md.impact")
	r.mdTable("md.impactHead")
	for _, f := range plan.ImpactFindings {
		symbol := "!"
		if f.Verdict == planner.VerdictUndecided {
			symbol = "?"
		}
		condition := mdCell(f.ConditionKey).mdText()
		if f.ConditionOperator != "" {
			// A plan read from a file has no operator (contract C2).
			condition = mdCell(f.ConditionOperator).mdText() + " " + condition
		}
		if f.ConditionValue != "" {
			condition += " = " + mdCell(f.ConditionValue).mdText()
		}
		var cause mdArg = mdTool(r.tr("md.undecided"))
		if f.Verdict != planner.VerdictUndecided {
			cause = r.mdCauses(f.ChangeIDs, byID)
		}
		r.mdRow(mdTool(symbol), mdCell(f.PolicyRef), mdCell(f.StatementID), mdTool(condition), cause)
	}
	r.blank()
}

// mdCauses lists the distinct changes behind a finding.
func (r *results) mdCauses(ids []string, byID map[string]planner.Change) mdArg {
	seen := map[string]bool{}
	var parts []mdArg
	for _, id := range ids {
		c, ok := byID[id]
		if !ok {
			continue
		}
		d := mdTool(r.tr("kind."+string(c.Kind)) + " " + mdChange(c).mdText())
		if !seen[string(d)] {
			seen[string(d)] = true
			parts = append(parts, d)
		}
	}
	return mdJoin(r.tr("list.separator"), parts)
}
