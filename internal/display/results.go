package display

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Format is the output format of a result (BR3.1, contract C5).
type Format string

// The four formats. scan has no diff.
const (
	FormatTable    Format = "table"
	FormatDiff     Format = "diff"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
)

// The formats each command accepts, in the order --help lists them.
var (
	ScanFormats = []Format{FormatTable, FormatJSON, FormatMarkdown}
	PlanFormats = []Format{FormatTable, FormatDiff, FormatJSON, FormatMarkdown}
)

// ParseFormat reads a --format value among allowed.
func ParseFormat(s string, allowed []Format) (Format, bool) {
	for _, f := range allowed {
		if string(f) == s {
			return f, true
		}
	}
	return "", false
}

// FormatList writes the formats as --help and the errors show them:
// "table|json|markdown".
func FormatList(formats []Format) string {
	parts := make([]string, 0, len(formats))
	for _, f := range formats {
		parts = append(parts, string(f))
	}
	return strings.Join(parts, "|")
}

// Output says how a result is written: its format, the exit code the
// command ends with (JSON carries it, BR3.4) and the tool version.
type Output struct {
	Format      Format
	ExitCode    int
	ToolVersion string
}

// ScanResult prints the result of scan in the format of out (screen 2 of the
// mockups for the table): the regions that could not be read, the key
// variant groups, the missing required tags, the never-tagged resources and
// the summary.
func (p *Printer) ScanResult(r *inventory.ScanReport, out Output) {
	p.locked(func() {
		p.endProgress()
		res := p.newResults()
		switch out.Format {
		case FormatJSON:
			res.scanJSON(r, out)
		case FormatMarkdown:
			res.scanMarkdown(r)
		default:
			res.scanTable(r)
		}
		p.flush(res)
	})
}

func (r *results) scanTable(rep *inventory.ScanReport) {
	snap := rep.Snapshot
	r.incompleteRegions(snap.FailedRegions)
	// "No issues found" goes with exit code 0 only (BR5.4): not when a
	// region could not be read.
	if !rep.IssuesFound() && snap.Complete() {
		if d := snap.Discovery; d != nil && d.Status != inventory.StatusComplete {
			r.line(styleNone, "scan.noIssuesDiscovery", rawString(statusWord(r.lang, d.Status)))
		} else {
			r.line(styleNone, "scan.noIssues")
		}
	}
	if variants := inventory.Variants(rep.Groups); len(variants) > 0 {
		r.scanVariants(variants)
		r.blank()
	}
	r.scanRequired(rep)
	r.blank()
	r.scanDiscovery(rep)
	r.blank()
	r.text(styleNone, r.scanSummary(rep))
}

// scanSummary is the summary line of scan, the same in the table and in
// Markdown.
func (r *results) scanSummary(rep *inventory.ScanReport) string {
	snap := rep.Snapshot
	summary := r.tr("scan.summary", r.count("unit.resource", len(snap.Resources)), snap.DoneRegions(), len(snap.EnabledRegions),
		r.count("unit.variantGroup", inventory.CountVariants(rep.Groups)))
	if rep.Missing != nil {
		summary = r.tr("scan.summaryMissing", summary, r.count("unit.requiredTag", rep.MissingKeys()))
	}
	return summary
}

func (r *results) scanVariants(variants []inventory.KeyVariantGroup) {
	r.line(styleNone, "scan.variants")
	for _, g := range variants {
		mark := ""
		if g.AbbreviationOnly {
			mark = r.tr("scan.abbreviation")
		}
		r.text(styleNone, "  "+padRight(Sanitize(g.NormalizedKey), 20)+" "+padLeft(FormatCount(g.TotalResources), 4)+" "+
			word(r.lang, "word.resources", g.TotalResources)+mark)
		for _, s := range g.Spellings {
			r.text(styleNone, "    "+padRight(Sanitize(s.Key), 20)+" "+padLeft(FormatCount(s.ResourceCount), 4))
		}
	}
}

// scanRequired prints the missing required tags (BR5.2).
func (r *results) scanRequired(rep *inventory.ScanReport) {
	if rep.Missing == nil {
		r.line(styleNone, "scan.requiredNoRules")
		return
	}
	var listed []inventory.MissingRequiredTag
	for _, m := range rep.Missing {
		if m.ResourceCount > 0 || m.NotTaggableCount > 0 {
			listed = append(listed, m)
		}
	}
	if len(listed) == 0 {
		r.line(styleNone, "scan.requiredNone", rep.RulesPath)
		return
	}
	r.line(styleNone, "scan.required", rep.RulesPath)
	for _, m := range listed {
		more := ""
		if m.NotTaggableCount > 0 {
			more = r.tr("scan.notTaggableMore", FormatCount(m.NotTaggableCount))
		}
		r.text(styleNone, "  "+padRight(Sanitize(m.Key), 12)+" "+padLeft(FormatCount(m.ResourceCount), 4)+" "+
			word(r.lang, "word.resources", m.ResourceCount)+more)
	}
}

// scanDiscovery prints the never-tagged resources, the state of their
// discovery and the types that cannot be tagged (BR3.2, BR3.3, BR5.5).
func (r *results) scanDiscovery(rep *inventory.ScanReport) {
	r.line(styleNone, "scan.noUserTags", FormatCount(rep.NeverTagged))
	if d := rep.Snapshot.Discovery; d != nil {
		r.line(styleNone, "scan.discovery", rawString(discoveryState(r.lang, d)))
		if d.Status != inventory.StatusComplete && rep.Missing != nil {
			r.line(styleNone, "scan.lowerBound")
		}
	}
	if len(rep.Unsupported) == 0 {
		return
	}
	total := 0
	for _, t := range rep.Unsupported {
		total += t.Count
	}
	r.line(styleNone, "scan.unsupported", rawString(r.count("unit.type", len(rep.Unsupported))), rawString(r.count("unit.resource", total)))
	for _, t := range rep.Unsupported {
		r.text(styleNone, "  "+padRight(Sanitize(t.ResourceType), 20)+" "+padLeft(FormatCount(t.Count), 4))
	}
}

// discoveryState is "complete", or the status with the regions that were not
// checked and why, for example "partial (not checked in me-south-1:
// AccessDenied resource-explorer-2:Search)". Its parts are sanitized here.
func discoveryState(lang Lang, d *inventory.Discovery) string {
	status := statusWord(lang, d.Status)
	groups := d.NotChecked()
	if len(groups) == 0 {
		return status
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
		return T(lang, "discovery.withReasons", status, strings.Join(parts, "; "))
	}
	return T(lang, "discovery.notCheckedIn", status, strings.Join(parts, "; "))
}

// incompleteRegions prints, at the top of the result, the regions the
// tagging API could not read and why (BR4.1, BR5.6). It prints nothing when
// every region was read.
func (r *results) incompleteRegions(failed []inventory.FailedRegion) {
	if len(failed) == 0 {
		return
	}
	r.line(styleImpact, "result.incomplete")
	for _, f := range failed {
		r.line(styleNone, "result.failedRegion", f.Region, f.Kind, f.Operation)
	}
	r.blank()
}

// ScanScope prints to standard error what scan read (BR5.7), so that "No
// issues found" is read within that scope.
func (p *Printer) ScanScope(snap *inventory.Snapshot) {
	p.locked(func() {
		p.say(styleNone, "stderr.checkedScan", rawString(taggedScope(p.lang, snap)), rawString(discoveryScope(p.lang, snap.Discovery)))
	})
}

func taggedScope(lang Lang, snap *inventory.Snapshot) string {
	return T(lang, "scope.tagged", snap.DoneRegions(), len(snap.EnabledRegions))
}

// discoveryScope says how never-tagged resources were looked for. The AWS
// Config path never covers the global IAM types (reliability design §3).
func discoveryScope(lang Lang, d *inventory.Discovery) string {
	if d == nil {
		return T(lang, "scope.notLookedFor")
	}
	config, explorer, aggregator, notChecked := d.MethodCounts()
	var parts []string
	if config > 0 {
		parts = append(parts, T(lang, "scope.config", Count(lang, "unit.region", config)))
	}
	if explorer > 0 {
		parts = append(parts, T(lang, "scope.explorer", Count(lang, "unit.region", explorer)))
	}
	if aggregator > 0 {
		parts = append(parts, T(lang, "scope.aggregator", Count(lang, "unit.region", aggregator)))
	}
	line := T(lang, "scope.notChecked")
	if len(parts) > 0 {
		line = T(lang, "scope.via", join(lang, parts))
	}
	if notChecked > 0 && len(parts) > 0 {
		line = T(lang, "scope.notCheckedIn", line, Count(lang, "unit.region", notChecked))
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
			p.say(styleNone, "stderr.checkedPolicies", rawString(taggedScope(p.lang, snap)))
			return
		}
		p.say(styleNone, "stderr.checkedNoPolicy", rawString(taggedScope(p.lang, snap)))
	})
}

// PlanResult prints the plan in the format of out (screens 4 to 6 of the
// mockups). Every format takes its counts from plan.Summary, so they agree
// (BR3.1).
func (p *Printer) PlanResult(plan *planner.Plan, out Output) {
	p.locked(func() {
		p.endProgress()
		res := p.newResults()
		s := plan.Summary()
		switch out.Format {
		case FormatDiff:
			res.planDiff(plan, s)
		case FormatJSON:
			res.planJSON(plan, s, out)
		case FormatMarkdown:
			res.planMarkdown(plan, s)
		default:
			res.planTable(plan, s)
		}
		p.flush(res)
	})
}

// planIncomplete prints the incomplete markers of the table and the diff at
// the top of the result.
func (r *results) planIncomplete(plan *planner.Plan) {
	r.incompleteRegions(plan.FailedRegions)
	if plan.ImpactAnalysisComplete {
		return
	}
	r.line(styleImpact, "plan.impactIncomplete")
	for _, u := range plan.UnreadPolicies {
		switch {
		case u.PolicyRef == "" && u.Region == "":
			r.line(styleNone, "plan.unreadList", rawString(unreadListLabel(r.lang, u.SourceKind)), u.Reason)
		case u.PolicyRef == "":
			r.line(styleNone, "plan.unreadListIn", rawString(unreadListLabel(r.lang, u.SourceKind)), u.Region, u.Reason)
		case u.Region == "":
			r.line(styleNone, "plan.unreadPolicy", rawString(unreadLabel(r.lang, u)), u.PolicyRef, u.Reason)
		default:
			r.line(styleNone, "plan.unreadPolicyIn", rawString(unreadLabel(r.lang, u)), u.PolicyRef, u.Region, u.Reason)
		}
	}
	r.blank()
}

// policyLabel names the kind of the policy of a finding (U5), for example
// "IAM inline policy". A kind outside contract C2 is shown as written.
func policyLabel(lang Lang, kind string) string {
	if slices.Contains(planner.SourceKinds, kind) {
		return T(lang, "policy."+kind)
	}
	return Sanitize(kind)
}

// unreadLabel names the kind of one unread policy. A user, role or group
// whose inline policies could not be listed is named with "of".
func unreadLabel(lang Lang, u planner.UnreadPolicy) string {
	switch {
	case u.SourceKind == planner.SourceIAMManaged:
		return T(lang, "unread.iamManaged")
	case u.SourceKind == planner.SourceIAMInline && !strings.Contains(u.PolicyRef, "#"):
		return T(lang, "unread.iamInlineOf")
	}
	return policyLabel(lang, u.SourceKind)
}

// unreadListLabel names the kind of a list that could not be read.
func unreadListLabel(lang Lang, kind string) string {
	if slices.Contains(planner.SourceKinds, kind) {
		return T(lang, "unreadList."+kind)
	}
	return Sanitize(kind)
}

// undecidedBecauseOfVariable reports whether a finding cannot be decided
// because of a policy variable; otherwise it is the condition operator (a
// Null of TagKeys, or an operator that does not compare strings).
func undecidedBecauseOfVariable(f planner.ImpactFinding) bool {
	return strings.Contains(f.ConditionKey, "${") || strings.Contains(f.ConditionValue, "${")
}

// planNothing prints "No changes needed" (BR2.7), with the remove candidates
// below it. It says so with exit code 0 only (BR5.4): not when a region
// could not be read.
func (r *results) planNothing(plan *planner.Plan) {
	if plan.InventoryComplete {
		r.line(styleNone, "plan.noChanges")
	}
	if len(plan.RemoveCandidates) > 0 {
		if plan.InventoryComplete {
			r.blank()
		}
		r.planCandidates(plan)
		r.blank()
		r.line(styleNone, "plan.summaryNothingRC", rawString(r.count("unit.resource", plan.ScannedResources)),
			rawString(r.count("unit.removeCandidate", len(plan.RemoveCandidates))))
		return
	}
	r.line(styleNone, "plan.summaryNothing", rawString(r.count("unit.resource", plan.ScannedResources)))
}

func (r *results) planTable(plan *planner.Plan, s planner.PlanSummary) {
	r.planIncomplete(plan)
	if plan.NothingToDo() {
		r.planNothing(plan)
		return
	}
	if len(s.ChangeLines) > 0 {
		r.planChangeLines(s)
	}
	if len(plan.Conflicts) > 0 {
		r.planConflicts(plan, s)
	}
	if len(plan.UnsetRequiredTags) > 0 {
		r.planUnset(plan, s)
	}
	if len(plan.RemoveCandidates) > 0 {
		r.planCandidates(plan)
		r.blank()
	}
	if s.IaCManagedResources > 0 {
		r.line(styleNone, "plan.iac", rawString(r.count("unit.resource", s.IaCManagedResources)))
		r.blank()
	}
	if len(plan.ImpactFindings) > 0 {
		r.planFindings(plan)
	}
	r.text(styleNone, r.planSummaryLine(s))
}

// planSummaryLine is the summary line of the table, the diff and Markdown.
func (r *results) planSummaryLine(s planner.PlanSummary) string {
	// Conflicts and remove candidates are counted nouns: "1 conflict", "2
	// conflicts". The other parts read the same for any count.
	return r.tr("plan.summary", FormatCount(s.ResourcesToChange), r.count("unit.conflict", s.ConflictResources),
		FormatCount(s.AffectedReferences), FormatCount(s.UndecidedReferences), FormatCount(s.UnsetRequiredResources),
		r.count("unit.removeCandidate", s.RemoveCandidateKeys), FormatCount(s.IaCManagedResources))
}

// kindSymbol is the symbol and the color of a kind of change.
func kindSymbol(k planner.ChangeKind) (string, style) {
	switch k {
	case planner.KindAddRequired:
		return "+", styleAdd
	case planner.KindRemoveKey:
		return "-", styleRemove
	}
	return "~", styleChange
}

// lineText is the content of a change line, sanitized.
func (r *results) lineText(l planner.ChangeLine) string {
	switch l.Kind {
	case planner.KindRenameKey:
		return Sanitize(l.FromKey) + " -> " + Sanitize(l.ToKey)
	case planner.KindChangeValue:
		return Sanitize(l.FromKey) + ": " + Sanitize(l.FromValue) + " -> " + Sanitize(l.ToValue)
	case planner.KindAddRequired:
		if l.SeveralValues {
			return Sanitize(l.ToKey) + " = " + r.tr("plan.fromRules")
		}
		return Sanitize(l.ToKey) + " = " + Sanitize(l.ToValue)
	}
	return Sanitize(l.FromKey)
}

func (r *results) planChangeLines(s planner.PlanSummary) {
	r.line(styleNone, "plan.changes", rawString(r.count("unit.resource", s.ResourcesToChange)))
	// The columns are measured in terminal columns (Width) and fit the
	// longest label and content, with three spaces after the label and at
	// least 30 columns of content.
	labelWidth, textWidth := 0, 30
	for _, k := range planner.Kinds {
		labelWidth = max(labelWidth, Width(r.tr("kind."+string(k)))+3)
	}
	texts := make([]string, len(s.ChangeLines))
	for i, l := range s.ChangeLines {
		texts[i] = r.lineText(l)
		textWidth = max(textWidth, Width(texts[i]))
	}
	for i, l := range s.ChangeLines {
		symbol, st := kindSymbol(l.Kind)
		r.text(st, "  "+symbol+" "+padRight(r.tr("kind."+string(l.Kind)), labelWidth)+padRight(texts[i], textWidth)+" "+
			padLeft(FormatCount(l.Resources), 5)+" "+word(r.lang, "word.resources", l.Resources))
	}
	r.line(styleNote, "plan.countedOnce")
	r.blank()
}

// conflictTags writes the tags of a conflict as key="value" pairs.
func conflictTags(c planner.KeyConflict) string {
	parts := make([]string, 0, len(c.KeyOrder))
	for _, k := range c.KeyOrder {
		parts = append(parts, Sanitize(k)+`="`+Sanitize(c.PresentTags[k])+`"`)
	}
	return strings.Join(parts, "  ")
}

func (r *results) planConflicts(plan *planner.Plan, s planner.PlanSummary) {
	r.line(styleImpact, "plan.conflicts", rawString(r.count("unit.resource", s.ConflictResources)))
	for _, c := range plan.Conflicts {
		r.text(styleNone, "  "+Sanitize(c.ResourceARN)+"   "+conflictTags(c))
	}
	r.blank()
}

func (r *results) planUnset(plan *planner.Plan, s planner.PlanSummary) {
	r.line(styleImpact, "plan.unset", rawString(r.count("unit.resource", s.UnsetRequiredResources)))
	for _, u := range plan.UnsetRequiredTags {
		r.text(styleNone, "  "+Sanitize(u.Key)+"   "+r.count("unit.resource", len(u.ARNs))+r.tr("plan.unsetReason"))
	}
	r.blank()
}

func (r *results) planCandidates(plan *planner.Plan) {
	r.line(styleNone, "plan.candidates", rawString(r.count("unit.key", len(plan.RemoveCandidates))))
	parts := make([]string, 0, len(plan.RemoveCandidates))
	for _, c := range plan.RemoveCandidates {
		parts = append(parts, r.tr("plan.candidate", Sanitize(c.Key), FormatCount(c.ResourceCount)))
	}
	r.text(styleNone, "  "+join(r.lang, parts))
}

func (r *results) planFindings(plan *planner.Plan) {
	byID := make(map[string]planner.Change, plan.ChangeCount())
	for _, c := range plan.Changes() {
		byID[c.ChangeID] = c
	}
	r.line(styleNone, "plan.impact")
	for _, f := range plan.ImpactFindings {
		symbol, st := "!", styleImpact
		if f.Verdict == planner.VerdictUndecided {
			symbol, st = "?", styleUndecided
		}
		r.line(st, "plan.policy", rawString(symbol), rawString(policyLabel(r.lang, f.SourceKind)), f.PolicyRef)
		// A plan read from a file has no operator: the word and its space go.
		switch {
		case f.ConditionOperator == "" && f.ConditionValue != "":
			r.line(styleNone, "plan.statementValueNoOp", f.StatementID, f.ConditionKey, f.ConditionValue)
		case f.ConditionOperator == "":
			r.line(styleNone, "plan.statementNoOp", f.StatementID, f.ConditionKey)
		case f.ConditionValue != "":
			r.line(styleNone, "plan.statementValue", f.StatementID, f.ConditionOperator, f.ConditionKey, f.ConditionValue)
		default:
			r.line(styleNone, "plan.statement", f.StatementID, f.ConditionOperator, f.ConditionKey)
		}
		if f.Verdict == planner.VerdictUndecided {
			if undecidedBecauseOfVariable(f) {
				r.line(styleNone, "plan.cannotDecide")
			} else {
				r.line(styleNone, "plan.cannotDecideOp")
			}
			continue
		}
		r.line(styleNone, "plan.affectedBy", rawString(causes(r.lang, f.ChangeIDs, byID)))
	}
	r.blank()
}

// cause describes one change in words, sanitized.
func cause(lang Lang, c planner.Change) string {
	switch c.Kind {
	case planner.KindRenameKey:
		return T(lang, "cause.renameKey", Sanitize(c.FromKey), Sanitize(c.ToKey))
	case planner.KindChangeValue:
		return T(lang, "cause.changeValue", Sanitize(c.FromKey), Sanitize(c.FromValue), Sanitize(c.ToValue))
	case planner.KindAddRequired:
		return T(lang, "cause.addRequired", Sanitize(c.ToKey), Sanitize(c.ToValue))
	}
	return T(lang, "cause.removeKey", Sanitize(c.FromKey))
}

// causes lists the distinct changes behind a finding, in change order.
func causes(lang Lang, ids []string, byID map[string]planner.Change) string {
	seen := map[string]bool{}
	var parts []string
	for _, id := range ids {
		c, ok := byID[id]
		if !ok {
			continue
		}
		d := cause(lang, c)
		if !seen[d] {
			seen[d] = true
			parts = append(parts, d)
		}
	}
	return join(lang, parts)
}

// PlanHeader prints to standard error, before a table or a diff, the
// account, the number of regions and the creation time in local time
// (screen 4; Markdown and JSON carry them in the result, BR3.10).
func (p *Printer) PlanHeader(plan *planner.Plan) {
	p.locked(func() {
		p.endProgress()
		p.say(styleNone, "stderr.planHeader", plan.AccountID, rawString(p.count("unit.region", len(plan.Regions))),
			plan.CreatedAt.Local().Format("2006-01-02 15:04"))
	})
}

// PlanSaved says on standard error where --out saved the plan (BR4.1).
func (p *Printer) PlanSaved(path string, plan *planner.Plan) {
	p.locked(func() {
		p.say(styleNone, "stderr.planSaved", path, rawString(p.count("unit.resource", plan.ResourcesToChange())), plan.AccountID)
	})
}

// SavePlanHint shows on standard error how to save the plan (the next step
// of screen 4).
func (p *Printer) SavePlanHint(rulesPath string) {
	p.locked(func() { p.say(styleNone, "stderr.savePlan", quoteArg(rulesPath)) })
}

// quoteArg quotes a command-line argument with spaces for the hint.
func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t\"'") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
