package cli_test

// Acceptance tests for U4 (the complete plan, its four output formats, the
// saved plan, and the display in English or Japanese with or without
// colors). Like the U1 to U3 tests, each one is an acceptance criterion
// written as Given/When/Then and run from the command entry point (cli.Run)
// against the HTTP-level fake AWS, with the AWS environment isolated and the
// language fixed (fixLanguage).
//
// These tests were written and committed before the U4 implementation (team
// testing posture: acceptance criteria first). They also check AC2.1.3 and
// AC2.1.4, which U3 handed over to U4 (U3 functional design Q2).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// fixLanguage makes the language of a test independent of the computer it
// runs on (U4 BR3.8): no LC_ALL or LC_MESSAGES, and LANG=C, which is English
// and keeps Windows from looking at its display language.
func fixLanguage(t *testing.T) {
	t.Helper()
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "C")
	t.Setenv("NO_COLOR", "")
}

// setLanguage sets the three language variables of BR3.8.
func setLanguage(t *testing.T, lcAll, lcMessages, lang string) {
	t.Helper()
	t.Setenv("LC_ALL", lcAll)
	t.Setenv("LC_MESSAGES", lcMessages)
	t.Setenv("LANG", lang)
}

// ARNs of the U4 tests, besides those of the U1 tests.
const (
	arnI1 = "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc"
	arnB2 = "arn:aws:s3:::my-logs-bucket"
)

// fullRules has every part of the rule file: aliases, value spellings, a
// required tag with a default and a condition, and a removed key.
const fullRules = "version: 1\n" +
	"keys:\n" +
	"  Environment:\n" +
	"    aliases: [env, Env]\n" +
	"    values:\n" +
	"      prod: [Prod, production]\n" +
	"required:\n" +
	"  Owner:\n" +
	"    default: platform\n" +
	"    when:\n" +
	"      - match: {resourceType: \"ec2:instance\"}\n" +
	"        value: compute-team\n" +
	"remove: [tmp-owner]\n"

// fullResources give every kind of change once or more:
//
//	qa: rename env->Environment, value Prod->prod, add Owner, remove tmp-owner
//	qb: value production->prod (Owner is already set, so it is kept)
//	b2: rename Env->Environment (managed by CloudFormation), add Owner
//	i1: add Owner from the ec2:instance condition
//	qc: nothing to change, only a key that no rule names (Name)
func fullResources() []testutil.TaggedResource {
	return []testutil.TaggedResource{
		{ARN: arnQA, Tags: map[string]string{"env": "Prod", "tmp-owner": "x", "Name": "a"}},
		{ARN: arnQB, Tags: map[string]string{"Environment": "production", "Owner": "data-team"}},
		{ARN: arnB2, Tags: map[string]string{"Env": "dev", "aws:cloudformation:stack-name": "logging", "Name": "logs"}},
		{ARN: arnI1, Tags: map[string]string{"Environment": "dev"}},
		{ARN: arnQC, Tags: map[string]string{"Environment": "dev", "Owner": "x", "Name": "c", "team": "t"}},
	}
}

// planDetails is the part of the saved plan (contract C2) and of the details
// of plan --format json (contract C3) that the tests read. Field names match
// the JSON names without regard to case.
type planDetails struct {
	Resources []struct {
		ARN          string
		Region       string
		Global       *bool
		ResourceType string
		BeforeTags   map[string]string
		IaCManagedBy *string
		Changes      []planChange
	}
	KeyConflicts []struct {
		ARN          string
		CanonicalKey string
		PresentTags  map[string]string
	}
	ImpactFindings []struct {
		Verdict, PolicyRef, SourceKind, StatementID string
		ChangeIDs                                   []string
	}
	UnreadPolicies    []struct{ SourceKind, Reason string }
	UnsetRequiredTags []struct {
		Key  string
		ARNs []string
	}
	RemoveCandidates []string
}

type planChange struct {
	ChangeID, Kind                     string
	FromKey, ToKey, FromValue, ToValue *string
	MergesIntoExisting, Impacted       *bool
}

// describe renders a change as "kind from->to (value -> value)".
func (c planChange) describe() string {
	s := func(p *string) string {
		if p == nil {
			return "-"
		}
		return *p
	}
	return c.Kind + " " + s(c.FromKey) + "->" + s(c.ToKey) + " " + s(c.FromValue) + "->" + s(c.ToValue)
}

// changesByResource lists the changes of each resource in order.
func (d planDetails) changesByResource() map[string][]string {
	out := map[string][]string{}
	for _, r := range d.Resources {
		for _, c := range r.Changes {
			out[r.ARN] = append(out[r.ARN], c.describe())
		}
	}
	return out
}

// planView is plan --format json (contract C3).
type planView struct {
	Version     int
	ToolVersion string
	Command     string
	ExitCode    *int
	AccountID   string
	Complete    struct {
		Inventory      *bool
		ImpactAnalysis *bool
	}
	FailedRegions []struct{ Region, Reason string }
	Summary       struct {
		ScannedResources       int
		ResourcesToChange      int
		ChangesByKind          map[string]int
		ConflictResources      int
		UnsetRequiredResources int
		AffectedReferences     int
		UndecidedReferences    int
		IaCManagedResources    int
	}
	Details struct {
		PlanID    string
		CreatedAt string
		Regions   []string
		planDetails
	}
}

// savedPlan is the file plan --out writes (contract C2).
type savedPlan struct {
	Version                int
	ToolVersion            string
	PlanID                 string
	AccountID              string
	CreatedAt              string
	Regions                []string
	InventoryComplete      *bool
	ImpactAnalysisComplete *bool
	RuleFile               string
	planDetails
	UnsupportedResourceTypes []struct {
		ResourceType string
		Count        int
	}
}

// scanView is scan --format json (contract C3).
type scanView struct {
	Version     int
	ToolVersion string
	Command     string
	ExitCode    *int
	AccountID   string
	Complete    struct {
		Inventory         *bool
		UntaggedDiscovery string
	}
	FailedRegions []struct{ Region, Reason string }
	Summary       struct {
		ScannedResources int
		VariantGroups    int
		MissingRequired  map[string]int
	}
	Details struct {
		VariantGroups []struct {
			Target   string
			Variants []struct {
				Spelling      string
				ResourceCount int
			}
		}
		MissingRequired []struct {
			Key  string
			ARNs []string
		}
	}
}

// decodeOne decodes standard output as exactly one JSON value.
func decodeOne(t *testing.T, stdout string, v any) {
	t.Helper()
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("standard output is not one valid JSON value:\n%s", stdout)
	}
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
}

// planCounts are the counts the four formats must agree on (AC3.1.5,
// NFR10.6).
type planCounts struct {
	total                   int
	rename, value, add, rem int
	conflicts, unset        int
	removeKeys, iac         int
	affected, undecided     int
}

// number reads "1,234" as 1234.
func number(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		t.Fatalf("not a count: %q", s)
	}
	return n
}

// firstCount returns the first group of the first match of pattern, or 0.
func firstCount(t *testing.T, text, pattern string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	return number(t, m[1])
}

// summaryPattern is the summary line of the table, the diff and Markdown.
const summaryPattern = `(?m)^Summary: ([\d,]+) to change · ([\d,]+) conflicts · ([\d,]+) affected · ([\d,]+) undecided · ` +
	`([\d,]+) value not set · ([\d,]+) remove candidates · ([\d,]+) IaC-managed$`

// summaryCounts reads the summary line into c.
func summaryCounts(t *testing.T, text string, c *planCounts) {
	t.Helper()
	m := regexp.MustCompile(summaryPattern).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no summary line:\n%s", text)
	}
	c.affected, c.undecided = number(t, m[3]), number(t, m[4])
	if c.total != number(t, m[1]) || c.conflicts != number(t, m[2]) || c.unset != number(t, m[5]) ||
		c.removeKeys != number(t, m[6]) || c.iac != number(t, m[7]) {
		t.Fatalf("the summary line disagrees with the sections: %+v\n%s", *c, m[0])
	}
}

// addKind adds n changes of the kind named in English.
func (c *planCounts) addKind(t *testing.T, kind string, n int) {
	t.Helper()
	switch kind {
	case "rename key":
		c.rename += n
	case "value":
		c.value += n
	case "add":
		c.add += n
	case "remove":
		c.rem += n
	default:
		t.Fatalf("unknown kind %q", kind)
	}
}

// countsFromTable reads the counts of the table format (English).
func countsFromTable(t *testing.T, out string) planCounts {
	t.Helper()
	var c planCounts
	c.total = firstCount(t, out, `(?m)^Changes: ([\d,]+) resources?$`)
	for _, m := range regexp.MustCompile(`(?m)^  [~+-] (rename key|value|add|remove) .* ([\d,]+) resources?$`).FindAllStringSubmatch(out, -1) {
		c.addKind(t, m[1], number(t, m[2]))
	}
	c.conflicts = firstCount(t, out, `(?m)^Conflicts \(not changed; values differ\): ([\d,]+) resources?$`)
	c.unset = firstCount(t, out, `(?m)^Required tag value not set \(not changed\): ([\d,]+) resources?$`)
	c.removeKeys = firstCount(t, out, `(?m)^Remove candidates \(keys not in your rules; not changed\): ([\d,]+) keys?$`)
	c.iac = firstCount(t, out, `(?m)^Managed by IaC \(changes may be reverted by the next deploy\): ([\d,]+) resources?$`)
	summaryCounts(t, out, &c)
	return c
}

// countsFromDiff reads the counts of the diff format (English).
func countsFromDiff(t *testing.T, out string) planCounts {
	t.Helper()
	var c planCounts
	m := regexp.MustCompile(`(?m)^Changes: ([\d,]+) resources? \(rename key ([\d,]+) · value ([\d,]+) · add ([\d,]+) · remove ([\d,]+)\)$`).FindStringSubmatch(out)
	if m != nil {
		c.total, c.rename, c.value, c.add, c.rem = number(t, m[1]), number(t, m[2]), number(t, m[3]), number(t, m[4]), number(t, m[5])
	}
	c.conflicts = len(regexp.MustCompile(`(?m)^. ! conflict: `).FindAllString(out, -1))
	c.iac = len(regexp.MustCompile(`(?m)^arn:\S+ +\[IaC: `).FindAllString(out, -1))
	sm := regexp.MustCompile(summaryPattern).FindStringSubmatch(out)
	if sm == nil {
		t.Fatalf("no summary line:\n%s", out)
	}
	c.unset, c.removeKeys = number(t, sm[5]), number(t, sm[6])
	summaryCounts(t, out, &c)
	return c
}

// countsFromMarkdown reads the counts of the Markdown format (English).
func countsFromMarkdown(t *testing.T, out string) planCounts {
	t.Helper()
	var c planCounts
	c.total = firstCount(t, out, `(?m)^## Changes \(([\d,]+) resources?\)$`)
	for _, m := range regexp.MustCompile(`(?m)^\| (rename key|value|add|remove) \| .+ \| ([\d,]+) \|$`).FindAllStringSubmatch(out, -1) {
		c.addKind(t, m[1], number(t, m[2]))
	}
	c.conflicts = firstCount(t, out, `(?m)^## Conflicts \(([\d,]+) resources?\)$`)
	c.unset = firstCount(t, out, `(?m)^## Required tag value not set \(([\d,]+) resources?\)$`)
	c.removeKeys = firstCount(t, out, `(?m)^## Remove candidates \(([\d,]+) keys?\)$`)
	c.iac = firstCount(t, out, `(?m)^## Managed by IaC \(([\d,]+) resources?\)$`)
	summaryCounts(t, out, &c)
	return c
}

// countsFromJSON reads the counts of plan --format json.
func countsFromJSON(t *testing.T, out string) planCounts {
	t.Helper()
	var v planView
	decodeOne(t, out, &v)
	s := v.Summary
	return planCounts{
		total: s.ResourcesToChange, rename: s.ChangesByKind["renameKey"], value: s.ChangesByKind["changeValue"],
		add: s.ChangesByKind["addRequired"], rem: s.ChangesByKind["removeKey"],
		conflicts: s.ConflictResources, unset: s.UnsetRequiredResources, removeKeys: len(v.Details.RemoveCandidates),
		iac: s.IaCManagedResources, affected: s.AffectedReferences, undecided: s.UndecidedReferences,
	}
}

// guidanceLines are lines of standard error that must never reach standard
// output, whatever the format (AC3.1.8, BR3.10).
var guidanceLines = []string{"Account 123456789012 · region", "scan: us-east-1", "Checked: ", "Done in ", "Exit code: ",
	"Save this plan", "Saved plan", "warning: "}

// AC3.1.1: Given rules and tags that call for a key rename, a value change,
// a required tag and a removal, When plan runs, Then the four kinds of
// change are listed with the right counts.
func TestAcceptanceU4_AC3_1_1_FourKindsOfChangeWithTheirCounts(t *testing.T) {
	w := newWorld(t)
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(fullRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Changes: 4 resources$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+Env -> Environment\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ value\s+Environment: Prod -> prod\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ value\s+Environment: production -> prod\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  \+ add\s+Owner = \(from rules\)\s+3 resources$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  - remove\s+tmp-owner\s+1 resource$`)
	requireContains(t, "stdout", r.stdout, "  (A resource with several changes is counted once in the total.)")
	requireNotContains(t, "stderr", r.stderr, "not applied to the plan yet")

	j := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")

	j.requireExit(t, 2)
	var v planView
	decodeOne(t, j.stdout, &v)
	want := map[string]int{"renameKey": 2, "changeValue": 2, "addRequired": 3, "removeKey": 1}
	for kind, n := range want {
		if v.Summary.ChangesByKind[kind] != n {
			t.Errorf("changesByKind[%s] = %d, want %d (%v)", kind, v.Summary.ChangesByKind[kind], n, v.Summary.ChangesByKind)
		}
	}
	if v.Summary.ResourcesToChange != 4 || v.Summary.ScannedResources != 5 {
		t.Fatalf("summary = %+v", v.Summary)
	}
}

// AC3.1.2 (with functional design Q1 and BR1.8): Given one resource needs a
// key rename and a value change, When plan runs, Then the plan has the two
// changes in that order and the resource counts once in the total.
func TestAcceptanceU4_AC3_1_2_OverlappingChangesCountTheResourceOnce(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "Prod"}})
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      prod: [Prod]\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	j := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")
	d := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "diff")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Changes: 1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ value\s+Environment: Prod -> prod\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 1 to change · `)
	var v planView
	decodeOne(t, j.stdout, &v)
	got := strings.Join(v.Details.changesByResource()[arnQA], "; ")
	if got != "renameKey env->Environment Prod->-; changeValue Environment->- Prod->prod" {
		t.Fatalf("changes of %s = %q", arnQA, got)
	}
	if ids := []string{v.Details.Resources[0].Changes[0].ChangeID, v.Details.Resources[0].Changes[1].ChangeID}; ids[0] != "c-000001" || ids[1] != "c-000002" {
		t.Fatalf("change IDs = %v", ids)
	}
	// The diff compares the tags before and after both changes (BR1.9).
	requireMatch(t, "diff", d.stdout, `(?m)^`+regexp.QuoteMeta(arnQA)+`$\n  - env\s+= "Prod"\n  \+ Environment = "prod"$`)
}

// AC3.1.3: Given a required tag whose value no condition or default decides
// for a resource, When plan runs, Then the resource is listed under "value
// not set" and its other changes stay in the plan.
func TestAcceptanceU4_AC3_1_3_UnsetRequiredValueKeepsTheOtherChanges(t *testing.T) {
	w := newWorld(t)
	w.givenResources(
		testutil.TaggedResource{ARN: arnB1, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: arnI1, Tags: map[string]string{"Environment": "dev"}},
	)
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(aliasRules + "required:\n  CostCenter:\n    when:\n      - match: {resourceType: \"ec2:instance\"}\n        value: cc-1\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	j := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Required tag value not set \(not changed\): 1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  CostCenter\s+1 resource\s+\(no default or matching condition\)$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  \+ add\s+CostCenter = cc-1\s+1 resource$`)
	var v planView
	decodeOne(t, j.stdout, &v)
	if len(v.Details.UnsetRequiredTags) != 1 || v.Details.UnsetRequiredTags[0].Key != "CostCenter" ||
		strings.Join(v.Details.UnsetRequiredTags[0].ARNs, ",") != arnB1 {
		t.Fatalf("unsetRequiredTags = %+v", v.Details.UnsetRequiredTags)
	}
	if got := v.Details.changesByResource()[arnB1]; len(got) != 1 || !strings.HasPrefix(got[0], "renameKey env->Environment") {
		t.Fatalf("the other change of %s must stay: %v", arnB1, got)
	}
	if v.Summary.UnsetRequiredResources != 1 {
		t.Fatalf("unsetRequiredResources = %d", v.Summary.UnsetRequiredResources)
	}
}

// AC3.1.4 (with functional design Q4): Given a key that no rule names, When
// plan runs, Then it is listed as a remove candidate and not removed; keys
// starting with aws: are never candidates; and candidates alone give exit
// code 0 with "No changes needed".
func TestAcceptanceU4_AC3_1_4_RemoveCandidatesAreNotRemoved(t *testing.T) {
	w := newWorld(t)
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(fullRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	j := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Remove candidates \(keys not in your rules; not changed\): 2 keys$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  Name \(3\), team \(1\)$`)
	requireNotContains(t, "stdout", r.stdout, "remove       Name")
	requireNotContains(t, "stdout", r.stdout, "aws:cloudformation")
	var v planView
	decodeOne(t, j.stdout, &v)
	if strings.Join(v.Details.RemoveCandidates, ",") != "Name,team" {
		t.Fatalf("removeCandidates = %v", v.Details.RemoveCandidates)
	}
	for arn, changes := range v.Details.changesByResource() {
		for _, c := range changes {
			if strings.HasPrefix(c, "removeKey Name") || strings.HasPrefix(c, "removeKey team") {
				t.Fatalf("%s: a remove candidate became a change: %s", arn, c)
			}
		}
	}

	only := newWorld(t)
	only.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev", "Name": "a", "AWS:Foo": "x"}})
	quiet := only.run("plan", "--rules", only.givenRuleFile(aliasRules), "--region", "us-east-1")

	quiet.requireExit(t, 0)
	requireMatch(t, "stdout", quiet.stdout, `(?m)^No changes needed$`)
	requireMatch(t, "stdout", quiet.stdout, `(?m)^Remove candidates \(keys not in your rules; not changed\): 1 key$`)
	requireMatch(t, "stdout", quiet.stdout, `(?m)^  Name \(1\)$`)
	requireNotContains(t, "stdout", quiet.stdout, "AWS:Foo")
	requireContains(t, "stderr", quiet.stderr, "Exit code: 0 (no changes needed)")
}

// AC3.1.5 (NFR10.6): Given the same rules and tags, When plan prints the
// table, the diff, JSON and Markdown, Then the four formats give the same
// counts of changes and impacts.
func TestAcceptanceU4_AC3_1_5_FourFormatsAgreeOnTheCounts(t *testing.T) {
	w := newWorld(t)
	resources := append(fullResources(), testutil.TaggedResource{ARN: arnQB + "-x", Tags: map[string]string{"env": "dev", "Environment": "test"}})
	w.givenResources(resources...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition, policyP2: policyVariableAndWildcard})
	rules := w.givenRuleFile(strings.Replace(fullRules, "remove: [tmp-owner]\n", "  CostCenter: {}\nremove: [tmp-owner]\n", 1))

	table := w.run("plan", "--rules", rules, "--region", "us-east-1")
	diff := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "diff")
	js := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")
	md := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "markdown")

	for _, r := range []result{table, diff, js, md} {
		r.requireExit(t, 2)
	}
	want := countsFromJSON(t, js.stdout)
	if want.total == 0 || want.rename == 0 || want.value == 0 || want.add == 0 || want.rem == 0 || want.conflicts == 0 ||
		want.unset == 0 || want.removeKeys == 0 || want.iac == 0 || want.affected == 0 || want.undecided == 0 {
		t.Fatalf("the scenario must give every count: %+v\n%s", want, js.stdout)
	}
	for name, got := range map[string]planCounts{
		"table":    countsFromTable(t, table.stdout),
		"diff":     countsFromDiff(t, diff.stdout),
		"markdown": countsFromMarkdown(t, md.stdout),
	} {
		if got != want {
			t.Errorf("%s counts = %+v, JSON counts = %+v", name, got, want)
		}
	}
}

// AC3.1.6 (with BR4.1, BR4.2, NFR8.17): Given plan --out, When the saved file
// is checked, Then it holds the creation time, the account, the tags before
// the change and the changes (contract C2); standard output is the same as
// without --out; and an existing file is never overwritten, before any AWS
// call.
func TestAcceptanceU4_AC3_1_6_OutSavesThePlanAndNeverOverwrites(t *testing.T) {
	w := newWorld(t)
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(fullRules)
	out := w.outputPath("plan.json")

	plain := w.run("plan", "--rules", rules, "--region", "us-east-1")
	r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--out", out)

	r.requireExit(t, 2)
	if r.stdout != plain.stdout {
		t.Fatalf("standard output must be the same with --out:\n%s\nwithout:\n%s", r.stdout, plain.stdout)
	}
	requireMatch(t, "stderr", r.stderr, `(?m)^Saved plan to .*`+regexp.QuoteMeta(filepath.Base(out))+` \(4 resources, account 123456789012\)\.$`)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the plan was not saved: %v", err)
	}
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) || bytes.Contains(data, []byte("\r")) {
		t.Fatal("the saved plan must be UTF-8 without a BOM and with LF newlines")
	}
	var p savedPlan
	decodeOne(t, string(data), &p)
	if p.Version != 1 || p.ToolVersion == "" || p.AccountID != "123456789012" || p.RuleFile != rules ||
		strings.Join(p.Regions, ",") != "us-east-1" || p.InventoryComplete == nil || !*p.InventoryComplete ||
		p.ImpactAnalysisComplete == nil || !*p.ImpactAnalysisComplete {
		t.Fatalf("saved plan header = %+v", p)
	}
	requireMatch(t, "planId", p.PlanID, `^plan-20261020T0900\d\dZ-[0-9a-f]{4}$`)
	requireMatch(t, "createdAt", p.CreatedAt, `^2026-10-20T09:00:\d\dZ$`)
	if len(p.Resources) != 4 {
		t.Fatalf("resources = %d, want the 4 with changes", len(p.Resources))
	}
	n := 0
	for _, res := range p.Resources {
		if res.Region != "us-east-1" || res.ResourceType == "" || res.Global == nil || len(res.Changes) == 0 {
			t.Errorf("resource %s = %+v", res.ARN, res)
		}
		for _, c := range res.Changes {
			n++
			if want := "c-" + strings.Repeat("0", 6-len(strconv.Itoa(n))) + strconv.Itoa(n); c.ChangeID != want {
				t.Errorf("change %d has ID %s, want %s", n, c.ChangeID, want)
			}
			if c.Impacted == nil {
				t.Errorf("change %s has no impacted field", c.ChangeID)
			}
			if c.Kind == "renameKey" && c.MergesIntoExisting == nil {
				t.Errorf("rename %s has no mergesIntoExisting", c.ChangeID)
			}
		}
		if res.ARN == arnB2 {
			if res.BeforeTags["aws:cloudformation:stack-name"] != "logging" || res.BeforeTags["Env"] != "dev" {
				t.Errorf("beforeTags must hold every tag, aws: included: %v", res.BeforeTags)
			}
			if res.IaCManagedBy == nil || *res.IaCManagedBy != "cloudformation:logging" {
				t.Errorf("iacManagedBy = %v", res.IaCManagedBy)
			}
		}
	}
	// The condition on env is affected by the renames from env and from Env:
	// tag keys are compared without case (U1 BR5.3).
	if len(p.ImpactFindings) != 1 || p.ImpactFindings[0].Verdict != "affected" || len(p.ImpactFindings[0].ChangeIDs) != 2 {
		t.Fatalf("impactFindings = %+v", p.ImpactFindings)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(out); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("the saved plan must be readable by its owner only: %v %v", info.Mode(), err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(out))
	if err != nil || len(entries) != 1 {
		t.Fatalf("the folder must hold the plan only (no temporary file): %v %v", entries, err)
	}

	calls := len(w.fake.Calls())
	again := w.run("plan", "--rules", rules, "--region", "us-east-1", "--out", out)
	missing := w.run("plan", "--rules", rules, "--region", "us-east-1", "--out", filepath.Join(filepath.Dir(out), "no-such-folder", "plan.json"))

	again.requireExit(t, 1)
	requireTwoLineError(t, again)
	requireContains(t, "stderr", again.stderr, "already exists")
	missing.requireExit(t, 1)
	requireTwoLineError(t, missing)
	if after, _ := os.ReadFile(out); !bytes.Equal(after, data) {
		t.Fatal("the existing plan was overwritten")
	}
	if n := len(w.fake.Calls()); n != calls {
		t.Fatalf("AWS was called %d times before the --out problem was reported", n-calls)
	}
}

// AC3.1.7: Given a resource made by CloudFormation needs a change, When plan
// runs, Then it is reported as managed by IaC in the table, the diff, JSON
// and Markdown.
func TestAcceptanceU4_AC3_1_7_IaCManagedResourcesAreWarned(t *testing.T) {
	w := newWorld(t)
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(fullRules)

	table := w.run("plan", "--rules", rules, "--region", "us-east-1")
	diff := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "diff")
	js := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")
	md := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "markdown")

	requireMatch(t, "table", table.stdout, `(?m)^Managed by IaC \(changes may be reverted by the next deploy\): 1 resource$`)
	requireMatch(t, "diff", diff.stdout, `(?m)^`+regexp.QuoteMeta(arnB2)+` +\[IaC: CloudFormation stack "logging"\]$`)
	requireMatch(t, "diff", diff.stdout, `(?m)^  ~ Env -> Environment = "dev"$`)
	requireMatch(t, "markdown", md.stdout, `(?m)^## Managed by IaC \(1 resource\)$`)
	requireMatch(t, "markdown", md.stdout, `(?m)^\| `+"` "+regexp.QuoteMeta(arnB2)+" `"+` \| .* \| `+"` logging `"+` \| `)
	var v planView
	decodeOne(t, js.stdout, &v)
	if v.Summary.IaCManagedResources != 1 {
		t.Fatalf("iacManagedResources = %d", v.Summary.IaCManagedResources)
	}
	for _, r := range v.Details.Resources {
		managed := r.IaCManagedBy != nil && *r.IaCManagedBy == "cloudformation:logging"
		if managed != (r.ARN == arnB2) {
			t.Errorf("%s: iacManagedBy = %v", r.ARN, r.IaCManagedBy)
		}
	}
}

// AC3.1.8 (BR3.10): Given the result of plan is saved from standard output,
// When the file is checked, Then it holds the result only, in every format:
// no progress, account line, exit code, next step or saved-plan line.
func TestAcceptanceU4_AC3_1_8_StandardOutputHoldsOnlyTheResult(t *testing.T) {
	w := newWorld(t)
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	// The required key starting with aws: is ignored with a warning (U3 BR5.1),
	// which must stay on standard error.
	rules := w.givenRuleFile(strings.Replace(fullRules, "remove: [tmp-owner]\n", "  \"aws:Bad\": {}\nremove: [tmp-owner]\n", 1))

	for _, format := range []string{"table", "diff", "json", "markdown"} {
		t.Run(format, func(t *testing.T) {
			r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", format)

			r.requireExit(t, 2)
			for _, line := range guidanceLines {
				requireNotContains(t, "stdout", r.stdout, line)
			}
			for _, line := range []string{"Account 123456789012 · region", "scan: us-east-1", "Exit code: 2 (review needed)", "Save this plan: ", "warning: "} {
				requireContains(t, "stderr", r.stderr, line)
			}
			switch format {
			case "json":
				var v planView
				decodeOne(t, r.stdout, &v)
				if v.ExitCode == nil || *v.ExitCode != 2 || v.Command != "plan" || v.Version != 1 {
					t.Fatalf("JSON header = %+v", v)
				}
			case "markdown":
				if !strings.HasPrefix(r.stdout, "## Target\n") {
					t.Fatalf("Markdown must start with the target section:\n%s", r.stdout)
				}
				requireMatch(t, "stdout", r.stdout, `Summary: [^\n]*\n$`)
			default:
				requireMatch(t, "stdout", r.stdout, `Summary: [^\n]*\n$`)
			}
		})
	}
}

// AC6.1.1: Given the tool, When the help of each subcommand is shown, Then
// scan, rules suggest and plan have their help with the U4 options (apply
// and rollback come with U6 and U7).
func TestAcceptanceU4_AC6_1_1_HelpOfEachSubcommand(t *testing.T) {
	w := newWorld(t)
	for _, tc := range []struct {
		args    []string
		options []string
	}{
		{[]string{"scan", "--help"}, []string{"--rules", "--profile", "--region", "--format", "table|json|markdown", "--lang"}},
		{[]string{"rules", "suggest", "--help"}, []string{"--profile", "--region", "--output", "--lang"}},
		{[]string{"plan", "--help"}, []string{"--rules", "--profile", "--region", "--format", "table|diff|json|markdown", "--out", "--lang"}},
	} {
		r := w.run(tc.args...)

		r.requireExit(t, 0)
		requireContains(t, strings.Join(tc.args, " "), r.stdout, "Usage:")
		for _, option := range tc.options {
			requireContains(t, strings.Join(tc.args, " "), r.stdout, option)
		}
	}
	if calls := w.fake.Calls(); len(calls) != 0 {
		t.Fatalf("help called AWS: %d calls", len(calls))
	}
}

// AC6.1.2 (BR3.8): Given the language setting of the OS is Japanese, When
// scan runs without --lang, Then it prints Japanese; a language that is
// neither Japanese nor English prints English; --lang wins over the
// environment; and an unknown --lang is a usage error before any AWS call.
func TestAcceptanceU4_AC6_1_2_LanguageFollowsTheEnvironmentOrLang(t *testing.T) {
	cases := []struct {
		name                    string
		lcAll, lcMessages, lang string
		args                    []string
		japanese                bool
	}{
		{name: "LANG ja_JP", lang: "ja_JP.UTF-8", japanese: true},
		{name: "LANG fr_FR", lang: "fr_FR.UTF-8"},
		{name: "LC_ALL wins over LANG", lcAll: "ja_JP.UTF-8", lang: "en_US.UTF-8", japanese: true},
		{name: "LC_MESSAGES before LANG", lcMessages: "ja", lang: "en_US.UTF-8", japanese: true},
		{name: "--lang en wins", lang: "ja_JP.UTF-8", args: []string{"--lang", "en"}},
		{name: "--lang ja wins", lang: "C", args: []string{"--lang", "ja"}, japanese: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			setLanguage(t, tc.lcAll, tc.lcMessages, tc.lang)
			w.givenResources(sandboxResources()...)

			r := w.run(append([]string{"scan", "--region", "us-east-1"}, tc.args...)...)

			r.requireExit(t, 2)
			if tc.japanese {
				requireContains(t, "stdout", r.stdout, "同じ意味で書き方が違うタグキー")
				requireMatch(t, "stdout", r.stdout, `(?m)^要約: `)
				requireContains(t, "stderr", r.stderr, "終了コード: 2(問題あり)")
				requireContains(t, "stderr", r.stderr, "アカウント 123456789012")
				requireNotContains(t, "stdout", r.stdout, "Summary: ")
			} else {
				requireContains(t, "stdout", r.stdout, "Tag key variants")
				requireMatch(t, "stdout", r.stdout, `(?m)^Summary: `)
				requireContains(t, "stderr", r.stderr, "Exit code: 2 (issues found)")
			}
			// Numbers, keys and ARNs are never translated (BR3.9).
			requireMatch(t, "stdout", r.stdout, `(?m)^    env\s+3$`)
		})
	}

	w := newWorld(t)
	r := w.run("scan", "--region", "us-east-1", "--lang", "fr")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "--lang")
	if calls := w.fake.Calls(); len(calls) != 0 {
		t.Fatalf("AWS was called with an unknown --lang: %d calls", len(calls))
	}
}

// AC6.1.3 (NFR11.7): Given NO_COLOR is set, When plan runs in each text
// format, Then no color sequence is printed and every kind of change and
// impact is readable from symbols and words.
func TestAcceptanceU4_AC6_1_3_NoColorKeepsSymbolsAndWords(t *testing.T) {
	w := newWorld(t)
	t.Setenv("NO_COLOR", "1")
	w.givenResources(fullResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition, policyP2: policyVariableAndWildcard})
	rules := w.givenRuleFile(fullRules)

	table := w.run("plan", "--rules", rules, "--region", "us-east-1")
	diff := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "diff")

	for _, r := range []result{table, diff} {
		r.requireExit(t, 2)
		requireNotContains(t, "stdout", r.stdout, "\x1b")
		requireNotContains(t, "stderr", r.stderr, "\x1b")
	}
	for _, pattern := range []string{`(?m)^  ~ rename key `, `(?m)^  ~ value `, `(?m)^  \+ add `, `(?m)^  - remove `,
		`(?m)^  ! IAM policy `, `(?m)^  \? IAM policy `} {
		requireMatch(t, "table", table.stdout, pattern)
	}
	for _, pattern := range []string{`(?m)^[ !?] - tmp-owner\s+= "x"$`, `(?m)^[ !?] \+ Owner\s+= "`, `(?m)^[ !?] ~ Env -> Environment = "dev"$`, `(?m)^! - env\s+= "Prod"$`} {
		requireMatch(t, "diff", diff.stdout, pattern)
	}
}

// AC6.1.4: Given scan found problems, When the exit code and the exitCode of
// scan --format json are checked, Then both are 2; without problems both are
// 0; with a region that could not be read both are 1.
func TestAcceptanceU4_AC6_1_4_ScanJSONExitCodeIsTheExitCode(t *testing.T) {
	cases := []struct {
		name      string
		resources []testutil.TaggedResource
		denied    bool
		want      int
	}{
		{name: "issues found", resources: sandboxResources(), want: 2},
		{name: "no issues", resources: []testutil.TaggedResource{{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}}}, want: 0},
		{name: "incomplete", denied: true, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			if tc.denied {
				w.fake.Handle("tag:GetResources", testutil.Static(testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "denied")))
			} else {
				w.givenResources(tc.resources...)
			}

			r := w.run("scan", "--region", "us-east-1", "--format", "json")

			r.requireExit(t, tc.want)
			var v scanView
			decodeOne(t, r.stdout, &v)
			if v.ExitCode == nil || *v.ExitCode != tc.want || v.Command != "scan" {
				t.Fatalf("exitCode = %v, command = %q, want %d", v.ExitCode, v.Command, tc.want)
			}
			if v.Complete.Inventory == nil || *v.Complete.Inventory != !tc.denied {
				t.Fatalf("complete.inventory = %v", v.Complete.Inventory)
			}
			if tc.denied && (len(v.FailedRegions) != 1 || v.FailedRegions[0].Region != "us-east-1") {
				t.Fatalf("failedRegions = %+v", v.FailedRegions)
			}
		})
	}
}

// AC6.1.5 (BR3.4): Given scan prints JSON to standard output, When only
// standard output is read, Then it is valid JSON (contract C3); progress
// goes to standard error.
func TestAcceptanceU4_AC6_1_5_ScanJSONIsValidAndProgressStaysOnStandardError(t *testing.T) {
	w := newWorld(t)
	w.givenResources(sandboxResources()...)
	rules := w.givenRuleFile(aliasRules + "required:\n  Owner:\n    default: platform\n")

	r := w.run("scan", "--region", "us-east-1", "--rules", rules, "--format", "json")
	md := w.run("scan", "--region", "us-east-1", "--rules", rules, "--format", "markdown")
	bad := w.run("scan", "--region", "us-east-1", "--format", "diff")

	r.requireExit(t, 2)
	var v scanView
	decodeOne(t, r.stdout, &v)
	if v.Version != 1 || v.ToolVersion == "" || v.AccountID != "123456789012" || v.Summary.ScannedResources != 4 ||
		v.Summary.VariantGroups != 2 || v.Summary.MissingRequired["Owner"] != 4 || v.Complete.UntaggedDiscovery != "unavailable" {
		t.Fatalf("scan JSON = %+v", v)
	}
	if len(v.Details.MissingRequired) != 1 || len(v.Details.MissingRequired[0].ARNs) != 4 || len(v.Details.VariantGroups) < 2 {
		t.Fatalf("details = %+v", v.Details)
	}
	for _, line := range []string{"scan: us-east-1 started", "Checked: ", "Exit code: 2 (issues found)"} {
		requireContains(t, "stderr", r.stderr, line)
		requireNotContains(t, "stdout", r.stdout, line)
	}
	md.requireExit(t, 2)
	if !strings.HasPrefix(md.stdout, "## Target\n") {
		t.Fatalf("scan Markdown must start with the target section:\n%s", md.stdout)
	}
	requireMatch(t, "markdown", md.stdout, "(?m)^\\| ` environment ` \\| ` env ` \\| 3 \\|$")
	requireMatch(t, "markdown", md.stdout, `(?m)^Summary: 4 resources scanned`)
	bad.requireExit(t, 1)
	requireTwoLineError(t, bad)
	requireContains(t, "stderr", bad.stderr, "--format")
}

// AC2.1.3: Given a default value of Owner and a value for EC2 instances, When
// plan runs on an EC2 instance, Then the value of the resource type
// condition is used; other resources get the default.
func TestAcceptanceU4_AC2_1_3_ConditionValueWinsOverTheDefault(t *testing.T) {
	w := newWorld(t)
	w.givenResources(
		testutil.TaggedResource{ARN: arnI1, Tags: map[string]string{"Environment": "dev"}},
		testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}},
	)
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(fullRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")

	r.requireExit(t, 2)
	var v planView
	decodeOne(t, r.stdout, &v)
	got := v.Details.changesByResource()
	if strings.Join(got[arnI1], ";") != "addRequired -->Owner -->compute-team" {
		t.Fatalf("EC2 instance changes = %v", got[arnI1])
	}
	if strings.Join(got[arnQA], ";") != "addRequired -->Owner -->platform" {
		t.Fatalf("queue changes = %v", got[arnQA])
	}
}

// AC2.1.4: Given the rule file removes tmp-owner, When plan runs, Then only
// tmp-owner is removed; a key with a similar name is not.
func TestAcceptanceU4_AC2_1_4_OnlyTheRemovedKeyIsRemoved(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"tmp-owner": "a", "tmp-Owner": "b", "tmp": "c", "Owner": "x"}})
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile("version: 1\nremove: [tmp-owner]\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--format", "json")
	table := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	var v planView
	decodeOne(t, r.stdout, &v)
	if got := strings.Join(v.Details.changesByResource()[arnQA], ";"); got != "removeKey tmp-owner->- a->-" {
		t.Fatalf("changes = %q", got)
	}
	requireMatch(t, "table", table.stdout, `(?m)^  - remove\s+tmp-owner\s+1 resource$`)
	requireMatch(t, "table", table.stdout, `(?m)^  Owner \(1\), tmp \(1\), tmp-Owner \(1\)$`)
}
