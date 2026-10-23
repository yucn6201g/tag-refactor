package planner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

var now = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func snapshot(resources ...inventory.Resource) *inventory.Snapshot {
	return &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"us-east-1"}, Resources: resources}
}

func res(arn string, tags map[string]string) inventory.Resource {
	return inventory.Resource{ARN: arn, Region: "us-east-1", Tags: tags}
}

func ruleSet(keyRules ...rules.KeyRule) *rules.RuleSet {
	return &rules.RuleSet{Version: 1, KeyRules: keyRules}
}

var envRule = rules.KeyRule{CanonicalKey: "Environment", Aliases: []string{"env", "Env"}}

func describeChanges(p *Plan) string {
	var parts []string
	for _, c := range p.Changes {
		parts = append(parts, fmt.Sprintf("%s %s %s->%s=%s merge=%t", c.ChangeID, c.ResourceARN, c.FromKey, c.ToKey, c.Value, c.MergesIntoExisting))
	}
	return strings.Join(parts, "\n")
}

func TestBuildRenamesASingleAlias(t *testing.T) {
	p := Build(snapshot(res("arn:a", map[string]string{"env": "dev", "Owner": "x"})), ruleSet(envRule), now)

	if got := describeChanges(p); got != "c-000001 arn:a env->Environment=dev merge=false" {
		t.Fatalf("changes:\n%s", got)
	}
	if p.Changes[0].Kind != KindRenameKey || p.PlanID != "plan-20261020T090000Z" || p.ScannedResources != 1 ||
		p.AccountID != "123456789012" || !p.InventoryComplete || !p.ImpactAnalysisComplete || len(p.Conflicts) != 0 {
		t.Fatalf("plan = %+v", p)
	}
}

func TestBuildMergesAliasesWithTheSameValue(t *testing.T) {
	p := Build(snapshot(
		res("arn:a", map[string]string{"Environment": "dev", "env": "dev"}), // canonical present: every rename merges
		res("arn:b", map[string]string{"Env": "dev", "env": "dev"}),         // no canonical: the first alias creates it
	), ruleSet(envRule), now)

	want := "c-000001 arn:a env->Environment=dev merge=true\n" +
		"c-000002 arn:b env->Environment=dev merge=false\n" +
		"c-000003 arn:b Env->Environment=dev merge=true"
	if got := describeChanges(p); got != want {
		t.Fatalf("changes:\n%s\nwant:\n%s", got, want)
	}
	if p.ResourcesToChange() != 2 {
		t.Fatalf("ResourcesToChange = %d, want 2 (one count per resource)", p.ResourcesToChange())
	}
}

func TestBuildRecordsConflictsInsteadOfChangingDifferentValues(t *testing.T) {
	p := Build(snapshot(
		res("arn:c", map[string]string{"env": "dev", "Environment": "prod"}),
		res("arn:d", map[string]string{"env": "dev", "Env": "test"}),
	), ruleSet(envRule), now)

	if len(p.Changes) != 0 || len(p.Conflicts) != 2 || p.ConflictResources() != 2 {
		t.Fatalf("plan = %+v", p)
	}
	c := p.Conflicts[0]
	if c.CanonicalKey != "Environment" || strings.Join(c.KeyOrder, ",") != "env,Environment" || c.PresentTags["Environment"] != "prod" {
		t.Fatalf("conflict = %+v", c)
	}
	if strings.Join(p.Conflicts[1].KeyOrder, ",") != "env,Env" {
		t.Fatalf("conflict without canonical key = %+v", p.Conflicts[1])
	}
	if p.NothingToDo() || !p.NeedsReview() {
		t.Fatal("a conflict needs review")
	}
}

func TestBuildNumbersChangesByARNThenCanonicalKey(t *testing.T) {
	costRule := rules.KeyRule{CanonicalKey: "CostCenter", Aliases: []string{"cost-center"}}
	p := Build(snapshot(
		res("arn:z", map[string]string{"env": "1", "cost-center": "9"}),
		res("arn:a", map[string]string{"Env": "2"}),
		res("arn:m", map[string]string{"Environment": "3"}), // already canonical
	), ruleSet(envRule, costRule), now)

	want := "c-000001 arn:a Env->Environment=2 merge=false\n" +
		"c-000002 arn:z cost-center->CostCenter=9 merge=false\n" +
		"c-000003 arn:z env->Environment=1 merge=false"
	if got := describeChanges(p); got != want {
		t.Fatalf("changes:\n%s\nwant:\n%s", got, want)
	}
	again := Build(snapshot(
		res("arn:m", map[string]string{"Environment": "3"}),
		res("arn:a", map[string]string{"Env": "2"}),
		res("arn:z", map[string]string{"cost-center": "9", "env": "1"}),
	), ruleSet(costRule, envRule), now)
	if describeChanges(again) != want {
		t.Fatal("the plan must not depend on input order")
	}
}

func TestNothingToDoWithoutChangesOrConflicts(t *testing.T) {
	p := Build(snapshot(res("arn:a", map[string]string{"Environment": "dev"}), res("arn:b", nil)), ruleSet(envRule), now)

	if !p.NothingToDo() || p.NeedsReview() || p.ResourcesToChange() != 0 || p.ScannedResources != 2 {
		t.Fatalf("plan = %+v", p)
	}
	if len(p.ImpactInputs()) != 0 {
		t.Fatal("no impact input expected")
	}
}

func TestInventoryIsIncompleteWhenARegionFailed(t *testing.T) {
	snap := snapshot(res("arn:a", map[string]string{"Environment": "dev"}))
	snap.FailedRegions = []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}}

	p := Build(snap, ruleSet(envRule), now)

	if p.InventoryComplete || !p.NothingToDo() {
		t.Fatalf("plan = %+v", p)
	}
}

func TestImpactInputsAndAttachImpact(t *testing.T) {
	p := Build(snapshot(res("arn:a", map[string]string{"env": "dev"}), res("arn:b", map[string]string{"Env": "x"})), ruleSet(envRule), now)
	inputs := p.ImpactInputs()
	if len(inputs) != 2 || inputs[0] != (TagChangeInput{ChangeID: "c-000001", ResourceARN: "arn:a", Kind: KindRenameKey,
		FromKey: "env", ToKey: "Environment", FromValue: "dev", ToValue: "dev"}) {
		t.Fatalf("inputs = %+v", inputs)
	}

	p.AttachImpact(ImpactResult{
		Findings: []ImpactFinding{
			{Verdict: VerdictAffected, ChangeIDs: []string{"c-000002"}},
			{Verdict: VerdictUndecided, ChangeIDs: []string{"c-000002"}},
		},
		UnreadPolicies: []UnreadPolicy{{SourceKind: SourceIAMManaged, Reason: "denied"}},
		Complete:       false,
	})

	if p.Changes[0].Impacted || !p.Changes[1].Impacted {
		t.Fatalf("impacted flags = %+v", p.Changes)
	}
	if affected, undecided := p.CountFindings(); affected != 1 || undecided != 1 {
		t.Fatalf("counts = %d, %d", affected, undecided)
	}
	if p.ImpactAnalysisComplete || len(p.UnreadPolicies) != 1 {
		t.Fatalf("plan = %+v", p)
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
}
