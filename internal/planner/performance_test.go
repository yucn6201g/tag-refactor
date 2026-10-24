package planner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// The input of the performance requirements (performance-requirements.md):
// 10,000 resources with 20 tags each, 5,000 to change with the four kinds,
// 100 conflicts, 100 required values not set and 100 findings; half of the
// values need replacing or escaping. Under -race it is ten times smaller.

const perfRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n    values:\n      prod: [Prod, production]\n" +
	"required:\n  Owner:\n    default: platform\n    when:\n      - match: {resourceType: \"ec2:instance\"}\n        value: compute-team\n" +
	"  CostCenter:\n    when:\n      - match: {resourceType: \"ec2:instance\"}\n        value: cc\nremove: [tmp]\n"

// perfValue is a tag value; every other one has characters that the table
// replaces and JSON and Markdown escape.
func perfValue(i, k int) string {
	if (i+k)%2 == 0 {
		return fmt.Sprintf("value-%d-%d|`<x>&", i, k) + string(rune(0x202e)) + string(rune(0x1b))
	}
	return fmt.Sprintf("value-%d-%d", i, k)
}

func perfSnapshot(n int) *inventory.Snapshot {
	snap := &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"us-east-1"}}
	changed, conflicts, unset := n/2, n/100, n/100
	for i := 0; i < n; i++ {
		tags := map[string]string{"Owner": "team", "CostCenter": "c"}
		for k := 0; k < 17; k++ {
			tags[fmt.Sprintf("key-%02d", k)] = perfValue(i, k)
		}
		switch {
		case i < changed && i%4 == 0:
			tags["env"] = "Prod" // rename and value change
		case i < changed && i%4 == 1:
			tags["Environment"] = "production" // value change
		case i < changed && i%4 == 2:
			tags["Environment"], tags["tmp"] = "dev", "x" // removal
		case i < changed:
			tags["Environment"] = "dev"
			delete(tags, "Owner") // required tag added
		case i < changed+conflicts:
			tags["env"], tags["Environment"] = "dev", "test"
		case i < changed+conflicts+unset:
			tags["Environment"] = "dev"
			delete(tags, "CostCenter")
		default:
			tags["Environment"] = "dev"
		}
		snap.Resources = append(snap.Resources, inventory.Resource{
			ARN: fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:queue-%06d", i), Region: "us-east-1", ResourceType: "sqs", Tags: tags})
	}
	return snap
}

// perfFindings names 100 findings, each with up to 50 changes.
func perfFindings(p *Plan, n int) ImpactResult {
	ids := make([]string, 0, p.ChangeCount())
	for _, c := range p.Changes() {
		ids = append(ids, c.ChangeID)
	}
	var r ImpactResult
	r.Complete = true
	for f := 0; f < n; f++ {
		verdict := VerdictAffected
		if f%2 == 1 {
			verdict = VerdictUndecided
		}
		var named []string
		for j := f; j < len(ids) && len(named) < 50; j += n {
			named = append(named, ids[j])
		}
		r.Findings = append(r.Findings, ImpactFinding{Verdict: verdict, PolicyRef: fmt.Sprintf("arn:aws:iam::123456789012:policy/P%03d", f),
			SourceKind: SourceIAMManaged, StatementID: "S", ConditionKey: "aws:ResourceTag/env", ConditionValue: perfValue(f, 0), ChangeIDs: named})
	}
	return r
}

// NFR1.13: making the plan, the rule queries included, takes at most 2
// seconds.
func TestPerformanceU4BuildThePlan(t *testing.T) {
	n := 10000 / perfScale
	snap := perfSnapshot(n)
	rs, err := rules.Parse("tagrules.yaml", []byte(perfRules), rules.Options{})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	p := Build(snap, rs, fixed())
	elapsed := time.Since(start)

	s := p.Summary()
	if s.ResourcesToChange != n/2 || len(p.Conflicts) != n/100 || p.UnsetRequiredResources() != n/100 ||
		s.ChangesByKind[KindRenameKey] == 0 || s.ChangesByKind[KindChangeValue] == 0 || s.ChangesByKind[KindAddRequired] == 0 ||
		s.ChangesByKind[KindRemoveKey] == 0 {
		t.Fatalf("the input does not have the size of the requirement: %+v", s)
	}
	if limit := time.Duration(timeFactor) * 2 * time.Second; elapsed > limit {
		t.Fatalf("making the plan of %d resources took %v (limit %v)", n, elapsed, limit)
	}
	t.Logf("NFR1.13: plan of %d resources and %d changes made in %v", n, p.ChangeCount(), elapsed)
}

// NFR1.15: reading the saved plan of the performance input (about 20 MiB)
// takes at most 3 seconds, and the memory it allocates is at most ten times
// the size of the file.
func TestPerformanceU4LoadThePlan(t *testing.T) {
	n := 10000 / perfScale
	rs, err := rules.Parse("tagrules.yaml", []byte(perfRules), rules.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := Build(perfSnapshot(n), rs, fixed())
	p.AttachImpact(perfFindings(p, 100))
	path := filepath.Join(t.TempDir(), "plan.json")
	var b bytes.Buffer
	w := NewSafeJSONWriter(&b)
	if err := WritePlan(w, p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	size := int64(b.Len())
	b = bytes.Buffer{}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	loaded, err := LoadPlan(path)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ResourcesToChange() != n/2 {
		t.Fatalf("read %d resources, want %d", loaded.ResourcesToChange(), n/2)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	ratio := float64(allocated) / float64(size)
	if limit := time.Duration(timeFactor) * 3 * time.Second; elapsed > limit {
		t.Fatalf("reading a plan of %.1f MiB took %v (limit %v)", float64(size)/(1<<20), elapsed, limit)
	}
	if ratio > 10 {
		t.Fatalf("reading a plan of %d bytes allocated %d bytes, %.1f times its size (limit 10)", size, allocated, ratio)
	}
	t.Logf("NFR1.15: plan of %.1f MiB read in %v, %.1f MiB allocated (%.1f times the file)",
		float64(size)/(1<<20), elapsed, float64(allocated)/(1<<20), ratio)
}
