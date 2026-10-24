package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// NFR10.6: the four formats give the same counts for 200 plans made from a
// fixed random seed, read back from each format by small readers of their
// lines. NFR1.14: each format and the saving of the plan of the performance
// input take at most 3 seconds.

// counts are the numbers the formats must agree on.
type counts struct {
	total                   int
	rename, value, add, rem int
	conflicts, unset        int
	removeKeys, iac         int
	affected, undecided     int
}

func num(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		t.Fatalf("not a count: %q", s)
	}
	return n
}

func first(t *testing.T, text, pattern string) int {
	t.Helper()
	if m := regexp.MustCompile(pattern).FindStringSubmatch(text); m != nil {
		return num(t, m[1])
	}
	return 0
}

var summaryLine = regexp.MustCompile(`(?m)^Summary: ([\d,]+) to change · ([\d,]+) conflicts · ([\d,]+) affected · ([\d,]+) undecided · ` +
	`([\d,]+) value not set · ([\d,]+) remove candidates · ([\d,]+) IaC-managed$`)

var nothingLine = regexp.MustCompile(`(?m)^Summary: [\d,]+ resources? scanned · 0 to change(?: · ([\d,]+) remove candidates)?$`)

// fromSummary reads the summary line, or the one of a plan with nothing to
// do.
func fromSummary(t *testing.T, text string) counts {
	t.Helper()
	if m := summaryLine.FindStringSubmatch(text); m != nil {
		return counts{total: num(t, m[1]), conflicts: num(t, m[2]), affected: num(t, m[3]), undecided: num(t, m[4]),
			unset: num(t, m[5]), removeKeys: num(t, m[6]), iac: num(t, m[7])}
	}
	if m := nothingLine.FindStringSubmatch(text); m != nil {
		c := counts{}
		if m[1] != "" {
			c.removeKeys = num(t, m[1])
		}
		return c
	}
	t.Fatalf("no summary line:\n%s", text)
	return counts{}
}

func (c *counts) kind(t *testing.T, name string, n int) {
	switch name {
	case "rename key":
		c.rename += n
	case "value":
		c.value += n
	case "add":
		c.add += n
	case "remove":
		c.rem += n
	default:
		t.Fatalf("kind %q", name)
	}
}

// fromTable reads the sections of the table; affected and undecided come
// from the summary line, which must agree with the sections.
func fromTable(t *testing.T, out string) counts {
	t.Helper()
	var c counts
	c.total = first(t, out, `(?m)^Changes: ([\d,]+) resources?$`)
	for _, m := range regexp.MustCompile(`(?m)^  [~+-] (rename key|value|add|remove) .* ([\d,]+) resources?$`).FindAllStringSubmatch(out, -1) {
		c.kind(t, m[1], num(t, m[2]))
	}
	c.conflicts = first(t, out, `(?m)^Conflicts \(not changed; values differ\): ([\d,]+) resources?$`)
	c.unset = first(t, out, `(?m)^Required tag value not set \(not changed\): ([\d,]+) resources?$`)
	c.removeKeys = first(t, out, `(?m)^Remove candidates \(keys not in your rules; not changed\): ([\d,]+) keys?$`)
	c.iac = first(t, out, `(?m)^Managed by IaC \(changes may be reverted by the next deploy\): ([\d,]+) resources?$`)
	s := fromSummary(t, out)
	c.affected, c.undecided = s.affected, s.undecided
	if s.total != c.total || s.conflicts != c.conflicts || s.unset != c.unset || s.removeKeys != c.removeKeys || s.iac != c.iac {
		t.Fatalf("the summary disagrees with the sections: %+v vs %+v\n%s", s, c, out)
	}
	return c
}

// fromDiff counts the resources, conflicts and IaC marks of the diff, and
// reads the changes line and the summary.
func fromDiff(t *testing.T, out string) counts {
	t.Helper()
	c := fromSummary(t, out)
	if m := regexp.MustCompile(`(?m)^Changes: ([\d,]+) resources? \(rename key ([\d,]+) · value ([\d,]+) · add ([\d,]+) · remove ([\d,]+)\)$`).FindStringSubmatch(out); m != nil {
		if num(t, m[1]) != c.total {
			t.Fatalf("the changes line disagrees with the summary:\n%s", out)
		}
		c.rename, c.value, c.add, c.rem = num(t, m[2]), num(t, m[3]), num(t, m[4]), num(t, m[5])
	}
	conflictResources := map[string]bool{}
	resource := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "arn:") {
			resource = line
		}
		if strings.HasPrefix(line, "  ! conflict: ") {
			conflictResources[resource] = true
		}
	}
	if len(conflictResources) != c.conflicts {
		t.Fatalf("%d resources with conflicts in the diff, %d in the summary:\n%s", len(conflictResources), c.conflicts, out)
	}
	if n := len(regexp.MustCompile(`(?m)^arn:\S+ +\[IaC: `).FindAllString(out, -1)); n != c.iac {
		t.Fatalf("%d IaC resources in the diff, %d in the summary:\n%s", n, c.iac, out)
	}
	return c
}

func fromMarkdown(t *testing.T, out string) counts {
	t.Helper()
	var c counts
	c.total = first(t, out, `(?m)^## Changes \(([\d,]+) resources?\)$`)
	for _, m := range regexp.MustCompile(`(?m)^\| (rename key|value|add|remove) \| .+ \| ([\d,]+) \|$`).FindAllStringSubmatch(out, -1) {
		c.kind(t, m[1], num(t, m[2]))
	}
	c.conflicts = first(t, out, `(?m)^## Conflicts \(([\d,]+) resources?\)$`)
	c.unset = first(t, out, `(?m)^## Required tag value not set \(([\d,]+) resources?\)$`)
	c.removeKeys = first(t, out, `(?m)^## Remove candidates \(([\d,]+) keys?\)$`)
	c.iac = first(t, out, `(?m)^## Managed by IaC \(([\d,]+) resources?\)$`)
	c.affected = len(regexp.MustCompile(`(?m)^\| ! \| `).FindAllString(out, -1))
	c.undecided = len(regexp.MustCompile(`(?m)^\| \? \| `).FindAllString(out, -1))
	s := fromSummary(t, out)
	if s.total != c.total || s.affected != c.affected || s.undecided != c.undecided || s.removeKeys != c.removeKeys {
		t.Fatalf("the summary disagrees with the sections: %+v vs %+v\n%s", s, c, out)
	}
	return c
}

func fromJSON(t *testing.T, out string) counts {
	t.Helper()
	var v struct {
		Summary struct {
			ResourcesToChange, ConflictResources, UnsetRequiredResources int
			AffectedReferences, UndecidedReferences, IaCManagedResources int
			ChangesByKind                                                map[string]int
		}
		Details struct{ RemoveCandidates []string }
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	s := v.Summary
	return counts{total: s.ResourcesToChange, rename: s.ChangesByKind["renameKey"], value: s.ChangesByKind["changeValue"],
		add: s.ChangesByKind["addRequired"], rem: s.ChangesByKind["removeKey"], conflicts: s.ConflictResources,
		unset: s.UnsetRequiredResources, removeKeys: len(v.Details.RemoveCandidates), iac: s.IaCManagedResources,
		affected: s.AffectedReferences, undecided: s.UndecidedReferences}
}

const randomRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n    values:\n      prod: [Prod, production]\n" +
	"  CostCenter:\n    aliases: [cost-center]\nrequired:\n  Owner:\n    default: platform\n    when:\n" +
	"      - match: {resourceType: \"ec2:instance\"}\n        value: compute-team\n  Team:\n    when:\n" +
	"      - match: {region: eu-west-1}\n        value: eu\nremove: [tmp, old]\n"

// randomPlan makes a plan of up to 30 resources from random tags, with
// random findings.
func randomPlan(t *testing.T, rnd *rand.Rand, rs *rules.RuleSet) *planner.Plan {
	keys := []string{"env", "Env", "Environment", "cost-center", "CostCenter", "Owner", "Team", "tmp", "old", "Name", "x" + hostile, "aws:cloudformation:stack-name"}
	values := []string{"dev", "Prod", "prod", "production", "test", "a|b", hostile, "日本語"}
	snap := &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"eu-west-1", "us-east-1"}}
	for i, n := 0, rnd.Intn(30); i < n; i++ {
		region := []string{"eu-west-1", "us-east-1"}[rnd.Intn(2)]
		kind := []string{"sqs", "ec2:instance", "s3"}[rnd.Intn(3)]
		r := inventory.Resource{ARN: fmt.Sprintf("arn:aws:%s:%s:123456789012:r-%03d", strings.Split(kind, ":")[0], region, i),
			Region: region, ResourceType: kind, Tags: map[string]string{}}
		for k := 0; k < rnd.Intn(6); k++ {
			r.Tags[keys[rnd.Intn(len(keys))]] = values[rnd.Intn(len(values))]
		}
		if stack, ok := r.Tags["aws:cloudformation:stack-name"]; ok {
			r.IaCManagedBy = "cloudformation:" + stack
		}
		snap.Resources = append(snap.Resources, r)
	}
	if rnd.Intn(5) == 0 {
		snap.FailedRegions = []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}}
	}
	p := planner.Build(snap, rs, planner.Options{Now: goldenTime, Rand: rnd})
	ids := p.Changes()
	var result planner.ImpactResult
	result.Complete = rnd.Intn(4) != 0
	for f := 0; f < rnd.Intn(5) && len(ids) > 0; f++ {
		verdict := planner.VerdictAffected
		if rnd.Intn(2) == 0 {
			verdict = planner.VerdictUndecided
		}
		finding := planner.ImpactFinding{Verdict: verdict, PolicyRef: fmt.Sprintf("arn:p%d", f), SourceKind: planner.SourceIAMManaged,
			StatementID: "S", ConditionKey: "aws:ResourceTag/env", ConditionValue: values[rnd.Intn(len(values))]}
		for j := 0; j < 1+rnd.Intn(3); j++ {
			finding.ChangeIDs = append(finding.ChangeIDs, ids[rnd.Intn(len(ids))].ChangeID)
		}
		result.Findings = append(result.Findings, finding)
	}
	if p.NeedsAnalysis() {
		p.AttachImpact(result)
	}
	return p
}

func TestFormatCountsAgree(t *testing.T) {
	rs, err := rules.Parse("tagrules.yaml", []byte(randomRules), rules.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(20261019))
	for i := 0; i < 200; i++ {
		p := randomPlan(t, rnd, rs)
		s := p.Summary()
		want := counts{total: s.ResourcesToChange, rename: s.ChangesByKind[planner.KindRenameKey], value: s.ChangesByKind[planner.KindChangeValue],
			add: s.ChangesByKind[planner.KindAddRequired], rem: s.ChangesByKind[planner.KindRemoveKey], conflicts: s.ConflictResources,
			unset: s.UnsetRequiredResources, removeKeys: s.RemoveCandidateKeys, iac: s.IaCManagedResources,
			affected: s.AffectedReferences, undecided: s.UndecidedReferences}
		outputs := map[Format]string{}
		for _, f := range PlanFormats {
			outputs[f] = render(t, English, func(pr *Printer) { pr.PlanResult(p, Output{Format: f}) })
		}
		got := map[Format]counts{FormatJSON: fromJSON(t, outputs[FormatJSON])}
		if p.NothingToDo() {
			got[FormatTable], got[FormatDiff] = fromSummary(t, outputs[FormatTable]), fromSummary(t, outputs[FormatDiff])
			got[FormatMarkdown] = fromSummary(t, outputs[FormatMarkdown])
		} else {
			got[FormatTable], got[FormatDiff], got[FormatMarkdown] = fromTable(t, outputs[FormatTable]), fromDiff(t, outputs[FormatDiff]),
				fromMarkdown(t, outputs[FormatMarkdown])
		}
		for f, c := range got {
			if p.NothingToDo() && f != FormatJSON {
				c.affected, c.undecided = want.affected, want.undecided
			}
			if c != want {
				t.Fatalf("plan %d, %s: counts %+v, want %+v\n%s", i, f, c, want, outputs[f])
			}
		}
	}
}

// perfPlan is the plan of the performance requirements (10,000 resources,
// 20 tags, 5,000 to change, half of the values to replace or escape).
func perfPlan(t testing.TB) *planner.Plan {
	t.Helper()
	n := 10000 / perfScale
	rs, err := rules.Parse("tagrules.yaml", []byte(randomRules), rules.Options{})
	if err != nil {
		t.Fatal(err)
	}
	snap := &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"us-east-1"}}
	for i := 0; i < n; i++ {
		tags := map[string]string{"Owner": "team"}
		for k := 0; k < 17; k++ {
			v := fmt.Sprintf("value-%d-%d", i, k)
			if (i+k)%2 == 0 {
				v += "|`<x>&" + string(rune(0x202e)) + string(rune(0x1b))
			}
			tags[fmt.Sprintf("key-%02d", k)] = v
		}
		switch {
		case i < n/2 && i%4 == 0:
			tags["env"] = "Prod"
		case i < n/2 && i%4 == 1:
			tags["Environment"] = "production"
		case i < n/2 && i%4 == 2:
			tags["Environment"], tags["tmp"] = "dev", "x"
		case i < n/2:
			tags["Environment"] = "dev"
			delete(tags, "Owner")
		case i < n/2+n/100:
			tags["env"], tags["Environment"] = "dev", "test"
		default:
			tags["Environment"] = "dev"
		}
		snap.Resources = append(snap.Resources, inventory.Resource{ARN: fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:q-%06d", i),
			Region: "us-east-1", ResourceType: "sqs", Tags: tags})
	}
	p := planner.Build(snap, rs, planner.Options{Now: goldenTime})
	ids := p.Changes()
	var result planner.ImpactResult
	result.Complete = true
	for f := 0; f < 100; f++ {
		finding := planner.ImpactFinding{Verdict: planner.VerdictAffected, PolicyRef: fmt.Sprintf("arn:p%03d", f), SourceKind: planner.SourceIAMManaged}
		for j := f; j < len(ids) && len(finding.ChangeIDs) < 50; j += 100 {
			finding.ChangeIDs = append(finding.ChangeIDs, ids[j].ChangeID)
		}
		result.Findings = append(result.Findings, finding)
	}
	p.AttachImpact(result)
	return p
}

func TestPerformanceU4FormatsAndSave(t *testing.T) {
	p := perfPlan(t)
	limit := time.Duration(timeFactor) * 3 * time.Second
	for _, f := range PlanFormats {
		for _, lang := range []Lang{English, Japanese} {
			var out bytes.Buffer
			pr := New(&out, &bytes.Buffer{})
			pr.SetLang(lang)
			start := time.Now()
			pr.PlanResult(p, Output{Format: f})
			if elapsed := time.Since(start); elapsed > limit {
				t.Fatalf("%s %s of %d resources took %v (limit %v)", f, lang, p.ResourcesToChange(), elapsed, limit)
			} else {
				t.Logf("NFR1.14: %s (%s) in %v, %.1f MiB", f, lang, elapsed, float64(out.Len())/(1<<20))
			}
		}
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	start := time.Now()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := planner.NewSafeJSONWriter(file)
	if err := planner.WritePlan(w, p); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > limit {
		t.Fatalf("saving the plan took %v (limit %v)", elapsed, limit)
	} else {
		t.Logf("NFR1.14: saved in %v", elapsed)
	}
}
