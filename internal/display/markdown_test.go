package display

import (
	"regexp"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Unit tests of the Markdown output (NFR8.16): data is only ever written in
// code spans, so it cannot make HTML, headings, links or table columns.

// attacks are values that would break Markdown or work as HTML if they were
// written as they are.
var attacks = []string{
	"</details>", "<img src=x onerror=alert(1)>", "`", "``", "a`b``c", "|", `a\|b`, "# heading", "[a](http://x)",
	"a<b&c", "line\nbreak", "tab\there", "`` start", "end ``", "", " ", hostile,
}

// unspan takes a value back out of a code span of a table cell: the fences
// and their spaces off, \| back to |.
func unspan(t *testing.T, cell string) string {
	t.Helper()
	m := regexp.MustCompile("^(`+) (.*) (`+)$").FindStringSubmatch(cell)
	if m == nil || m[1] != m[3] {
		t.Fatalf("not a code span: %q", cell)
	}
	return strings.ReplaceAll(m[2], `\|`, "|")
}

func TestMdSpanKeepsEveryValueInsideACodeSpan(t *testing.T) {
	for _, v := range attacks {
		span := string(mdSpan(v))
		cell := string(mdCell(v))
		if got := unspan(t, cell); got != Sanitize(v) {
			t.Errorf("%q: the cell gives back %q, want %q", v, got, Sanitize(v))
		}
		// The fence is longer than any run of backticks inside.
		fence := span[:strings.Index(span, " ")]
		if strings.Contains(Sanitize(v), fence) {
			t.Errorf("%q: the fence %q appears inside the value", v, fence)
		}
		if strings.ContainsAny(span, "\n\r") {
			t.Errorf("%q: a newline reached the Markdown: %q", v, span)
		}
		// Every bar of a cell is escaped.
		if strings.Contains(strings.ReplaceAll(cell, `\|`, ""), "|") {
			t.Errorf("%q: an unescaped | in the cell %q", v, cell)
		}
		// <, > and & are kept as they are: an entity would change the value.
		if strings.Contains(v, "<") && !strings.Contains(span, "<") {
			t.Errorf("%q: < was replaced: %q", v, span)
		}
	}
}

// structure counts what data must never change: the <details> blocks
// outside code spans, the headings and the columns of every table row.
func structure(md string) (details, closings, headings int, columns []int) {
	for _, line := range strings.Split(md, "\n") {
		outside := stripSpans(line)
		details += strings.Count(outside, "<details>")
		closings += strings.Count(outside, "</details>")
		if strings.HasPrefix(line, "#") {
			headings++
		}
		if strings.HasPrefix(line, "|") {
			// Bars outside escapes split the columns.
			columns = append(columns, strings.Count(strings.ReplaceAll(line, `\|`, ""), "|")-1)
		}
	}
	return details, closings, headings, columns
}

// planWith puts value into every place of a plan that Markdown shows: keys,
// values, ARNs, the conflict, the required key, a candidate, the stack
// name, the policy and its statement, and a failed region.
func planWith(value string) *planner.Plan {
	arn := "arn:" + value
	change := func(id string, kind planner.ChangeKind) planner.Change {
		return planner.Change{ChangeID: id, ResourceARN: arn, Kind: kind, FromKey: value, ToKey: value, FromValue: value, ToValue: value}
	}
	return &planner.Plan{
		AccountID: value, CreatedAt: goldenTime, Regions: []string{value}, InventoryComplete: false, ImpactAnalysisComplete: false,
		FailedRegions: []inventory.FailedRegion{{Region: value, Kind: value, Operation: value}},
		Resources: []planner.PlannedResource{{ARN: arn, BeforeTags: map[string]string{value: value}, IaCManagedBy: "cloudformation:" + value,
			Changes: []planner.Change{change("c-000001", planner.KindRenameKey), change("c-000002", planner.KindChangeValue),
				change("c-000003", planner.KindAddRequired), change("c-000004", planner.KindRemoveKey)}}},
		Conflicts:         []planner.KeyConflict{{ResourceARN: arn, CanonicalKey: value, PresentTags: map[string]string{value: value}, KeyOrder: []string{value}}},
		UnsetRequiredTags: []planner.UnsetRequiredTag{{Key: value, ARNs: []string{arn}}},
		RemoveCandidates:  []planner.RemoveCandidate{{Key: value, ResourceCount: 1}},
		ImpactFindings: []planner.ImpactFinding{
			{Verdict: planner.VerdictAffected, PolicyRef: value, StatementID: value, ConditionOperator: value, ConditionKey: value, ConditionValue: value,
				ChangeIDs: []string{"c-000001"}},
			{Verdict: planner.VerdictUndecided, PolicyRef: value, StatementID: value, ConditionKey: value, ChangeIDs: []string{"c-000002"}},
		},
		UnreadPolicies: []planner.UnreadPolicy{{PolicyRef: value, Reason: value}, {Reason: value}},
	}
}

func TestMarkdownStructureDoesNotDependOnTheData(t *testing.T) {
	plain := render(t, English, func(p *Printer) { p.PlanResult(planWith("plain"), Output{Format: FormatMarkdown}) })
	d0, c0, h0, cols0 := structure(plain)
	if d0 != 2 || c0 != 2 || h0 < 8 {
		t.Fatalf("the plain plan has %d <details>, %d </details>, %d headings:\n%s", d0, c0, h0, plain)
	}
	for _, v := range attacks {
		md := render(t, English, func(p *Printer) { p.PlanResult(planWith(v), Output{Format: FormatMarkdown}) })
		d, c, h, cols := structure(md)
		if d != d0 || c != c0 || h != h0 || len(cols) != len(cols0) {
			t.Errorf("%q changed the structure: %d/%d details, %d headings, %d rows:\n%s", v, d, c, h, len(cols), md)
			continue
		}
		for i := range cols {
			if cols[i] != cols0[i] {
				t.Errorf("%q changed the columns of row %d: %d, want %d", v, i, cols[i], cols0[i])
			}
		}
		// Outside code spans only the tool's words remain. "</details>" is
		// also the tool's own closing tag; the counts above cover it.
		if strings.Contains(stripSpans(md), Sanitize(v)) && strings.TrimSpace(v) != "" && len(v) > 2 && v != "</details>" {
			t.Errorf("%q appears outside a code span:\n%s", v, md)
		}
	}
}

// stripSpans removes every code span of a Markdown text as CommonMark reads
// it: a run of backticks opens a span that the next run of the same length
// closes. A run without its closing run is kept as text.
func stripSpans(md string) string {
	var b strings.Builder
	for i := 0; i < len(md); {
		if md[i] != '`' {
			b.WriteByte(md[i])
			i++
			continue
		}
		n := 0
		for i+n < len(md) && md[i+n] == '`' {
			n++
		}
		closing := -1
		for j := i + n; j < len(md); {
			if md[j] != '`' {
				j++
				continue
			}
			m := 0
			for j+m < len(md) && md[j+m] == '`' {
				m++
			}
			if m == n {
				closing = j + m
				break
			}
			j += m
		}
		if closing < 0 {
			b.WriteString(md[i : i+n])
			i += n
			continue
		}
		i = closing
	}
	return b.String()
}

func TestScanMarkdownWritesDataInCodeSpansOnly(t *testing.T) {
	for _, v := range attacks[:12] {
		rep := goldenScan()
		rep.Snapshot.Resources = append(rep.Snapshot.Resources, inventory.Resource{ARN: "arn:x", Tags: map[string]string{v: "1", "ENV" + v: "2"}})
		rep.Groups = inventory.GroupKeys(rep.Snapshot.Resources)
		rep.RulesPath = v
		rep.Snapshot.AccountID = v
		md := render(t, English, func(p *Printer) { p.ScanResult(rep, Output{Format: FormatMarkdown}) })
		d, c, _, _ := structure(md)
		if d != 0 || c != 0 {
			t.Errorf("%q made HTML in the scan Markdown:\n%s", v, md)
		}
		if strings.Contains(stripSpans(md), Sanitize(v)) && len(v) > 2 {
			t.Errorf("%q appears outside a code span:\n%s", v, md)
		}
	}
}
