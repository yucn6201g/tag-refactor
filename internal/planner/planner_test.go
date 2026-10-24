package planner

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// Unit tests of the plan (U4 BR1.1 to BR2.7), written after the
// implementation. The U1 tests of renames and conflicts are kept with the U4
// model: the changes belong to their resources.

var now = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// fixed gives the same plan ID suffix every time.
func fixed() Options {
	return Options{Now: now, RuleFile: "tagrules.yaml", ToolVersion: "v0.0.0-test", Rand: bytes.NewReader([]byte{0xab, 0xcd})}
}

func snapshot(resources ...inventory.Resource) *inventory.Snapshot {
	return &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"us-east-1"}, Resources: resources}
}

func res(arn string, tags map[string]string) inventory.Resource {
	return inventory.Resource{ARN: arn, Region: "us-east-1", ResourceType: inventory.ResourceTypeOf(arn), Tags: tags}
}

func ruleSet(keyRules ...rules.KeyRule) *rules.RuleSet {
	return &rules.RuleSet{Version: 1, KeyRules: keyRules}
}

// parse reads a rule file from YAML, as the unit test instructions ask.
func parse(t testing.TB, yaml string) *rules.RuleSet {
	t.Helper()
	rs, err := rules.Parse("tagrules.yaml", []byte(yaml), rules.Options{})
	if err != nil {
		t.Fatalf("rule file: %v", err)
	}
	return rs
}

var envRule = rules.KeyRule{CanonicalKey: "Environment", Aliases: []string{"env", "Env"}}

// describe renders a change on one line.
func describe(c Change) string {
	switch c.Kind {
	case KindRenameKey:
		return fmt.Sprintf("%s %s rename %s->%s (%s) merge=%t", c.ChangeID, c.ResourceARN, c.FromKey, c.ToKey, c.FromValue, c.MergesIntoExisting)
	case KindChangeValue:
		return fmt.Sprintf("%s %s value %s: %s->%s", c.ChangeID, c.ResourceARN, c.FromKey, c.FromValue, c.ToValue)
	case KindAddRequired:
		return fmt.Sprintf("%s %s add %s=%s", c.ChangeID, c.ResourceARN, c.ToKey, c.ToValue)
	}
	return fmt.Sprintf("%s %s remove %s (%s)", c.ChangeID, c.ResourceARN, c.FromKey, c.FromValue)
}

func describeChanges(p *Plan) string {
	var parts []string
	for _, c := range p.Changes() {
		parts = append(parts, describe(c))
	}
	return strings.Join(parts, "\n")
}

func TestBuildRenamesASingleAlias(t *testing.T) {
	p := Build(snapshot(res("arn:a", map[string]string{"env": "dev", "Owner": "x"})), ruleSet(envRule), fixed())

	if got := describeChanges(p); got != "c-000001 arn:a rename env->Environment (dev) merge=false" {
		t.Fatalf("changes:\n%s", got)
	}
	if p.PlanID != "plan-20261020T090000Z-abcd" || p.ScannedResources != 1 || p.AccountID != "123456789012" ||
		p.RuleFile != "tagrules.yaml" || p.ToolVersion != "v0.0.0-test" || !p.InventoryComplete || !p.ImpactAnalysisComplete ||
		len(p.Conflicts) != 0 || !p.CreatedAt.Equal(now) {
		t.Fatalf("plan = %+v", p)
	}
	r := p.Resources[0]
	if r.ARN != "arn:a" || r.BeforeTags["env"] != "dev" || r.BeforeTags["Owner"] != "x" || r.Changes[0].Mark != MarkNone {
		t.Fatalf("resource = %+v", r)
	}
}

func TestBuildMergesAliasesWithTheSameValue(t *testing.T) {
	p := Build(snapshot(
		res("arn:a", map[string]string{"Environment": "dev", "env": "dev"}), // canonical present: every rename merges
		res("arn:b", map[string]string{"Env": "dev", "env": "dev"}),         // no canonical: the first alias creates it
	), ruleSet(envRule), fixed())

	want := "c-000001 arn:a rename env->Environment (dev) merge=true\n" +
		"c-000002 arn:b rename env->Environment (dev) merge=false\n" +
		"c-000003 arn:b rename Env->Environment (dev) merge=true"
	if got := describeChanges(p); got != want {
		t.Fatalf("changes:\n%s\nwant:\n%s", got, want)
	}
	if p.ResourcesToChange() != 2 || p.ChangeCount() != 3 {
		t.Fatalf("ResourcesToChange = %d, ChangeCount = %d (one count per resource)", p.ResourcesToChange(), p.ChangeCount())
	}
}

func TestBuildRecordsConflictsInsteadOfChangingDifferentValues(t *testing.T) {
	p := Build(snapshot(
		res("arn:c", map[string]string{"env": "dev", "Environment": "prod"}),
		res("arn:d", map[string]string{"env": "dev", "Env": "test"}),
	), ruleSet(envRule), fixed())

	if p.ChangeCount() != 0 || len(p.Conflicts) != 2 || p.ConflictResources() != 2 {
		t.Fatalf("plan = %+v", p)
	}
	c := p.Conflicts[0]
	if c.CanonicalKey != "Environment" || strings.Join(c.KeyOrder, ",") != "env,Environment" || c.PresentTags["Environment"] != "prod" {
		t.Fatalf("conflict = %+v", c)
	}
	if strings.Join(p.Conflicts[1].KeyOrder, ",") != "env,Env" {
		t.Fatalf("conflict without canonical key = %+v", p.Conflicts[1])
	}
	if p.NothingToDo() || !p.NeedsReview() || p.NeedsAnalysis() {
		t.Fatal("a conflict needs review, but no impact analysis (BR2.7)")
	}
}

// BR1.8: resources in ARN order; inside one, the canonical keys in order
// with their renames then value change, then the required tags in key
// order, then the removals in key order.
func TestBuildOrdersAndNumbersTheChanges(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      prod: [Prod]\n"+
		"  CostCenter:\n    aliases: [cost-center]\nrequired:\n  Team:\n    default: t\n  Owner:\n    default: o\nremove: [zz, aa]\n")
	p := Build(snapshot(
		res("arn:z", map[string]string{"env": "Prod", "cost-center": "9", "zz": "1", "aa": "2", "Team": "x", "Owner": "y"}),
		res("arn:a", map[string]string{"Environment": "dev"}),
	), rs, fixed())

	want := "c-000001 arn:a add Owner=o\n" +
		"c-000002 arn:a add Team=t\n" +
		"c-000003 arn:z rename cost-center->CostCenter (9) merge=false\n" +
		"c-000004 arn:z rename env->Environment (Prod) merge=false\n" +
		"c-000005 arn:z value Environment: Prod->prod\n" +
		"c-000006 arn:z remove aa (2)\n" +
		"c-000007 arn:z remove zz (1)"
	if got := describeChanges(p); got != want {
		t.Fatalf("changes:\n%s\nwant:\n%s", got, want)
	}
	again := Build(snapshot(p.Resources[1].toResource(), p.Resources[0].toResource()), rs, fixed())
	if describeChanges(again) != want {
		t.Fatal("the plan must not depend on the input order")
	}
	if changeID(1234567) != "c-1234567" {
		t.Fatalf("changeID beyond six digits = %s", changeID(1234567))
	}
}

func (r PlannedResource) toResource() inventory.Resource {
	return inventory.Resource{ARN: r.ARN, Region: r.Region, ResourceType: r.ResourceType, Tags: r.BeforeTags}
}

// BR1.1, BR1.2, BR1.9 and the table of functional-spec.md: values are
// compared after the value spellings, and every case ends with
// Environment=prod, or a conflict.
func TestBuildComparesValuesAfterTheirSpellings(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env, ENV]\n    values:\n      prod: [Prod]\n")
	cases := []struct {
		tags     map[string]string
		changes  string
		conflict bool
	}{
		{map[string]string{"env": "Prod"},
			"rename env->Environment (Prod) merge=false; value Environment: Prod->prod", false},
		{map[string]string{"env": "Prod", "Environment": "prod"},
			"rename env->Environment (Prod) merge=true", false},
		{map[string]string{"env": "Prod", "ENV": "prod"},
			"rename env->Environment (Prod) merge=false; rename ENV->Environment (prod) merge=true; value Environment: Prod->prod", false},
		{map[string]string{"env": "Prod", "Environment": "dev"}, "", true},
		{map[string]string{"Environment": "Prod"}, "value Environment: Prod->prod", false},
	}
	for i, tc := range cases {
		p := Build(snapshot(res("arn:a", tc.tags)), rs, fixed())
		var got []string
		for _, c := range p.Changes() {
			got = append(got, strings.TrimPrefix(describe(c), c.ChangeID+" arn:a "))
		}
		if strings.Join(got, "; ") != tc.changes || (len(p.Conflicts) == 1) != tc.conflict {
			t.Errorf("case %d: changes %q, conflicts %d; want %q, conflict %v", i, strings.Join(got, "; "), len(p.Conflicts), tc.changes, tc.conflict)
			continue
		}
		if tc.conflict {
			if p.Conflicts[0].PresentTags["env"] != "Prod" {
				t.Errorf("case %d: the conflict must show the collected values: %+v", i, p.Conflicts[0])
			}
			continue
		}
		after := AfterTags(tc.tags, p.Resources[0].Changes)
		if !reflect.DeepEqual(after, map[string]string{"Environment": "prod"}) {
			t.Errorf("case %d: tags after = %v, want Environment=prod", i, after)
		}
	}
}

// BR1.4 and Q2: a required tag is added only when it is missing, with its
// value given its canonical spelling; a key in conflict counts as present;
// an undecided value goes to the list and keeps the other changes.
func TestBuildAddsRequiredTagsOnlyWhenMissing(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      prod: [Prod]\n"+
		"required:\n  Environment:\n    default: Prod\n  Owner:\n    default: platform\n    when:\n"+
		"      - match: {resourceType: \"ec2:instance\"}\n        value: compute-team\n"+
		"      - match: {tags: {Environment: prod}}\n        value: prod-team\n  CostCenter: {}\n")
	p := Build(snapshot(
		res("arn:aws:ec2:us-east-1:123456789012:instance/i-1", map[string]string{"Owner": "data-team", "CostCenter": "c"}),
		res("arn:aws:ec2:us-east-1:123456789012:instance/i-2", map[string]string{"env": "dev", "CostCenter": "c"}),
		res("arn:aws:sqs:us-east-1:123456789012:q1", map[string]string{"env": "Prod"}),
		res("arn:aws:sqs:us-east-1:123456789012:q2", map[string]string{"env": "dev", "Environment": "test"}),
	), rs, fixed())

	got := map[string][]string{}
	for _, c := range p.Changes() {
		got[c.ResourceARN] = append(got[c.ResourceARN], strings.TrimPrefix(describe(c), c.ChangeID+" "+c.ResourceARN+" "))
	}
	want := map[string][]string{
		// Owner is already set and kept; Environment gets the canonical value
		// of "Prod".
		"arn:aws:ec2:us-east-1:123456789012:instance/i-1": {"add Environment=prod"},
		"arn:aws:ec2:us-east-1:123456789012:instance/i-2": {"rename env->Environment (dev) merge=false", "add Owner=compute-team"},
		// The condition sees the tags after the value spellings (U3 BR3.2).
		"arn:aws:sqs:us-east-1:123456789012:q1": {"rename env->Environment (Prod) merge=false", "value Environment: Prod->prod", "add Owner=prod-team"},
		// Environment is in conflict: it counts as present and is not added.
		"arn:aws:sqs:us-east-1:123456789012:q2": {"add Owner=platform"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %v\nwant %v", got, want)
	}
	if len(p.UnsetRequiredTags) != 1 || p.UnsetRequiredTags[0].Key != "CostCenter" ||
		strings.Join(p.UnsetRequiredTags[0].ARNs, ",") != "arn:aws:sqs:us-east-1:123456789012:q1,arn:aws:sqs:us-east-1:123456789012:q2" {
		t.Fatalf("unset = %+v", p.UnsetRequiredTags)
	}
	if p.UnsetRequiredResources() != 2 || !p.NeedsReview() {
		t.Fatal("two resources have a required value not set")
	}
}

// BR1.5, BR2.3, BR1.6: removed keys are matched exactly; every other key no
// rule names is a candidate, counted by resource; keys starting with aws: in
// any case are neither.
func TestBuildRemovesKeysAndListsCandidates(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env]\nrequired:\n  Owner:\n    default: o\nremove: [tmp-owner]\n")
	p := Build(snapshot(
		res("arn:a", map[string]string{"tmp-owner": "1", "tmp-Owner": "2", "Name": "a", "aws:cloudformation:stack-name": "s", "AWS:Foo": "x", "Owner": "o"}),
		res("arn:b", map[string]string{"Name": "b", "Aws:Bar": "y", "env": "dev", "Owner": "o"}),
	), rs, fixed())

	if got := describeChanges(p); got != "c-000001 arn:a remove tmp-owner (1)\nc-000002 arn:b rename env->Environment (dev) merge=false" {
		t.Fatalf("changes:\n%s", got)
	}
	want := []RemoveCandidate{{Key: "Name", ResourceCount: 2}, {Key: "tmp-Owner", ResourceCount: 1}}
	if !reflect.DeepEqual(p.RemoveCandidates, want) {
		t.Fatalf("candidates = %+v", p.RemoveCandidates)
	}

	only := Build(snapshot(res("arn:c", map[string]string{"Environment": "dev", "Owner": "o", "team": "x"})), rs, fixed())
	if !only.NothingToDo() || only.NeedsReview() || only.ExitCode() != ExitOK || len(only.RemoveCandidates) != 1 {
		t.Fatalf("remove candidates alone give exit code 0 (Q4): %+v", only)
	}
}

// BR1.6: a rule set that names aws: keys (which the rule file reader never
// builds) still makes no change to them.
func TestBuildNeverTouchesAWSKeys(t *testing.T) {
	rs := &rules.RuleSet{Version: 1,
		KeyRules:     []rules.KeyRule{{CanonicalKey: "aws:Canonical", Aliases: []string{"x"}}, {CanonicalKey: "Environment", Aliases: []string{"AWS:Env"}}},
		RequiredTags: []rules.RequiredTag{{Key: "aws:Required", Default: "v", HasDefault: true}},
		RemoveKeys:   []rules.RemoveKey{{Key: "aws:Remove"}, {Key: "AWS:Remove"}},
	}
	p := Build(snapshot(res("arn:a", map[string]string{"x": "1", "AWS:Env": "dev", "aws:Remove": "r", "AWS:Remove": "r"})), rs, fixed())

	for _, c := range p.Changes() {
		if inventory.IsAWSReservedKey(c.FromKey) || inventory.IsAWSReservedKey(c.ToKey) {
			t.Fatalf("a change names an aws: key: %s", describe(c))
		}
	}
	if p.ChangeCount() != 0 || len(p.RemoveCandidates) != 0 {
		t.Fatalf("plan = %s, candidates %v", describeChanges(p), p.RemoveCandidates)
	}
}

// BR1.7: resources whose tags cannot be changed get no change, are not
// counted as unset, and are listed by type.
func TestBuildSkipsResourcesWhoseTagsCannotChange(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env]\nrequired:\n  Owner: {}\n")
	compliance := res("arn:aws:ssm:us-east-1:123456789012:patch-compliance/x", map[string]string{"env": "dev"})
	compliance.ResourceType = "ssm:patch-compliance"
	flagged := res("arn:aws:sqs:us-east-1:123456789012:q", map[string]string{"env": "dev"})
	flagged.NotTaggable = true
	noARN := inventory.Resource{ResourceType: "config:resource-compliance", Region: "us-east-1", Tags: map[string]string{"env": "dev"}}

	p := Build(snapshot(compliance, flagged, noARN), rs, fixed())

	if p.ChangeCount() != 0 || len(p.UnsetRequiredTags) != 0 || len(p.RemoveCandidates) != 0 {
		t.Fatalf("plan = %+v", p)
	}
	want := []inventory.TypeCount{{ResourceType: "config:resource-compliance", Count: 1}, {ResourceType: "sqs", Count: 1},
		{ResourceType: "ssm:patch-compliance", Count: 1}}
	if !reflect.DeepEqual(p.UnsupportedResourceTypes, want) {
		t.Fatalf("unsupported = %+v", p.UnsupportedResourceTypes)
	}
}

func TestNothingToDoWithoutChangesOrConflicts(t *testing.T) {
	p := Build(snapshot(res("arn:a", map[string]string{"Environment": "dev"}), res("arn:b", nil)), ruleSet(envRule), fixed())

	if !p.NothingToDo() || p.NeedsReview() || p.ResourcesToChange() != 0 || p.ScannedResources != 2 || p.NeedsAnalysis() {
		t.Fatalf("plan = %+v", p)
	}
	if len(p.ImpactInputs()) != 0 {
		t.Fatal("no impact input expected")
	}
}

func TestInventoryIsIncompleteWhenARegionFailed(t *testing.T) {
	snap := snapshot(res("arn:a", map[string]string{"Environment": "dev"}))
	snap.FailedRegions = []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}}

	p := Build(snap, ruleSet(envRule), fixed())

	if p.InventoryComplete || !p.NothingToDo() || len(p.FailedRegions) != 1 || p.ExitCode() != ExitError {
		t.Fatalf("plan = %+v", p)
	}
}

// BR2.5: affected wins over undecided; both are impacted (contract C5).
func TestImpactInputsAndAttachImpact(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n    values:\n      prod: [Prod]\nrequired:\n  Owner:\n    default: o\n")
	p := Build(snapshot(res("arn:a", map[string]string{"env": "dev", "Owner": "x"}), res("arn:b", map[string]string{"Env": "Prod"})), rs, fixed())
	inputs := p.ImpactInputs()
	want := []TagChangeInput{
		{ChangeID: "c-000001", ResourceARN: "arn:a", Kind: KindRenameKey, FromKey: "env", ToKey: "Environment", FromValue: "dev", ToValue: "dev"},
		{ChangeID: "c-000002", ResourceARN: "arn:b", Kind: KindRenameKey, FromKey: "Env", ToKey: "Environment", FromValue: "Prod", ToValue: "Prod"},
		{ChangeID: "c-000003", ResourceARN: "arn:b", Kind: KindChangeValue, FromKey: "Environment", FromValue: "Prod", ToValue: "prod"},
		{ChangeID: "c-000004", ResourceARN: "arn:b", Kind: KindAddRequired, ToKey: "Owner", ToValue: "o"},
	}
	if !reflect.DeepEqual(inputs, want) {
		t.Fatalf("inputs = %+v", inputs)
	}

	p.AttachImpact(ImpactResult{
		Findings: []ImpactFinding{
			{Verdict: VerdictUndecided, ChangeIDs: []string{"c-000002", "c-000003"}},
			{Verdict: VerdictAffected, ChangeIDs: []string{"c-000002"}},
		},
		UnreadPolicies: []UnreadPolicy{{SourceKind: SourceIAMManaged, Reason: "denied"}},
		Complete:       false,
	})

	var marks []string
	for _, c := range p.Changes() {
		marks = append(marks, fmt.Sprintf("%s=%s/%t", c.ChangeID, c.Mark, c.Impacted))
	}
	if strings.Join(marks, " ") != "c-000001=none/false c-000002=affected/true c-000003=undecided/true c-000004=none/false" {
		t.Fatalf("marks = %v", marks)
	}
	if affected, undecided := p.CountFindings(); affected != 1 || undecided != 1 {
		t.Fatalf("counts = %d, %d", affected, undecided)
	}
	if p.ImpactAnalysisComplete || len(p.UnreadPolicies) != 1 || p.ExitCode() != ExitError {
		t.Fatalf("plan = %+v", p)
	}
}

// BR1.9: renames that merge only remove their key; aws: keys stay; before is
// not changed.
func TestAfterTagsAppliesTheChangesInOrder(t *testing.T) {
	before := map[string]string{"env": "Prod", "Environment": "prod", "tmp": "x", "aws:cloudformation:stack-name": "s"}
	changes := []Change{
		{Kind: KindRenameKey, FromKey: "env", ToKey: "Environment", MergesIntoExisting: true},
		{Kind: KindChangeValue, FromKey: "Environment", FromValue: "prod", ToValue: "production"},
		{Kind: KindAddRequired, ToKey: "Owner", ToValue: "o"},
		{Kind: KindRemoveKey, FromKey: "tmp"},
	}
	after := AfterTags(before, changes)

	want := map[string]string{"Environment": "production", "Owner": "o", "aws:cloudformation:stack-name": "s"}
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("after = %v", after)
	}
	if len(before) != 4 || before["env"] != "Prod" {
		t.Fatal("before was changed")
	}
	if got := AfterTags(map[string]string{"a": "1"}, []Change{{Kind: KindRenameKey, FromKey: "a", ToKey: "b"}}); !reflect.DeepEqual(got, map[string]string{"b": "1"}) {
		t.Fatalf("rename without merge = %v", got)
	}
}

// BR2.1: lines are grouped by kind and content, ordered by kind, then by the
// number of resources, then by key; renames ignore their reference value;
// required tags with several values show "(from rules)".
func TestSummaryGroupsTheChangeLines(t *testing.T) {
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n    values:\n      prod: [Prod]\n"+
		"required:\n  Owner:\n    default: platform\n    when:\n      - match: {region: eu-west-1}\n        value: eu-team\n  Team:\n    default: t\nremove: [tmp]\n")
	eu := res("arn:aws:sqs:eu-west-1:123456789012:e", map[string]string{"Team": "x", "tmp": "1"})
	eu.Region = "eu-west-1"
	p := Build(snapshot(
		res("arn:a", map[string]string{"env": "dev", "Owner": "x", "Team": "x"}),
		res("arn:b", map[string]string{"env": "Prod", "Owner": "x", "Team": "x"}),
		res("arn:c", map[string]string{"Env": "dev", "Owner": "x", "Team": "x", "aws:cloudformation:stack-name": "s"}),
		res("arn:d", map[string]string{"Environment": "Prod", "tmp": "2"}),
		eu,
	), rs, fixed())
	p.Resources[2].IaCManagedBy = "cloudformation:s"

	s := p.Summary()

	var lines []string
	for _, l := range s.ChangeLines {
		lines = append(lines, fmt.Sprintf("%s %s>%s %s>%s several=%t %d", l.Kind, l.FromKey, l.ToKey, l.FromValue, l.ToValue, l.SeveralValues, l.Resources))
	}
	want := []string{
		"renameKey env>Environment > several=false 2",
		"renameKey Env>Environment > several=false 1",
		"changeValue Environment> Prod>prod several=false 2",
		"addRequired >Owner > several=true 2",
		"addRequired >Team >t several=false 1",
		"removeKey tmp> > several=false 2",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if s.ChangeLines[3].ToValue != "" {
		t.Fatalf("several values must not show one: %+v", s.ChangeLines[3])
	}
	byKind := map[ChangeKind]int{KindRenameKey: 3, KindChangeValue: 2, KindAddRequired: 3, KindRemoveKey: 2}
	if !reflect.DeepEqual(s.ChangesByKind, byKind) || s.ResourcesToChange != 5 || s.ScannedResources != 5 || s.IaCManagedResources != 1 {
		t.Fatalf("summary = %+v", s)
	}
	empty := (&Plan{}).Summary()
	if len(empty.ChangesByKind) != 4 || empty.ChangeLines == nil {
		t.Fatalf("an empty plan still has every kind at 0: %+v", empty)
	}
}

// BR2.6: incomplete is 1, then anything to review is 2, else 0.
func TestExitCodeFollowsBR2_6(t *testing.T) {
	change := []PlannedResource{{ARN: "a", Changes: []Change{{ChangeID: "c-000001", Kind: KindRemoveKey}}}}
	cases := []struct {
		p    Plan
		want int
	}{
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true}, 0},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true, RemoveCandidates: []RemoveCandidate{{Key: "x"}}}, 0},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true, Resources: change}, 2},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true, Conflicts: []KeyConflict{{}}}, 2},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true, UnsetRequiredTags: []UnsetRequiredTag{{Key: "k"}}}, 2},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: true, ImpactFindings: []ImpactFinding{{}}}, 2},
		{Plan{InventoryComplete: false, ImpactAnalysisComplete: true, Resources: change}, 1},
		{Plan{InventoryComplete: true, ImpactAnalysisComplete: false, Resources: change}, 1},
	}
	for i, tc := range cases {
		if got := tc.p.ExitCode(); got != tc.want {
			t.Errorf("case %d: exit code %d, want %d", i, got, tc.want)
		}
	}
}

// BR4.3: the plan ID is the time to the second and four hexadecimal digits;
// two plans of the same second differ.
func TestPlanIDHasTheTimeAndARandomSuffix(t *testing.T) {
	a, b := NewPlanID(now.Add(500*time.Millisecond), nil), NewPlanID(now, nil)
	pattern := regexp.MustCompile(`^plan-20261020T090000Z-[0-9a-f]{4}$`)
	if !pattern.MatchString(a) || !pattern.MatchString(b) {
		t.Fatalf("IDs = %s, %s", a, b)
	}
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		seen[NewPlanID(now, nil)] = true
	}
	if len(seen) < 2 {
		t.Fatal("plan IDs of the same second must differ")
	}
	if got := NewPlanID(now, bytes.NewReader(nil)); got != "plan-20261020T090000Z-0000" {
		t.Fatalf("an empty random source = %s", got)
	}
}

// The rule file and the scan must agree on what "the same key" means
// (BR2.2, BR3.3); the two packages keep their own copy of the functions.
func TestRulesAndInventoryShareTheKeyNormalization(t *testing.T) {
	for _, k := range []string{"Cost-Center", "cost_center", "Cost Center", "ENV", "aws:x", "AWS:x", "Ümlaut", "a"} {
		if rules.NormalizeKey(k) != inventory.NormalizeKey(k) {
			t.Errorf("NormalizeKey(%q) differs", k)
		}
		if rules.IsAWSReservedKey(k) != inventory.IsAWSReservedKey(k) {
			t.Errorf("IsAWSReservedKey(%q) differs", k)
		}
	}
	for _, k := range Kinds {
		if !k.Valid() {
			t.Errorf("%s is not valid", k)
		}
	}
	if ChangeKind("renamekey").Valid() {
		t.Fatal("kinds are compared exactly")
	}
}
