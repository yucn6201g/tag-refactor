//go:build ignore

// u6check checks the plans and run records of the U6 checkpoint for
// scripts/verify.sh (construction/u6-apply/infrastructure-design/
// cicd-pipeline.md, steps 2, 3 and 5). It reads files only and never calls
// AWS. Usage:
//
//	go run scripts/u6check.go plan <plan.json> <add|remove|untagged> <u2-queue-arn> <arn>...
//	go run scripts/u6check.go record <record.json> <plan.json> <arn>...
//	go run scripts/u6check.go entry <record.json> <arn>
//
// plan: every resource with changes changes the check key only (added for
// add and untagged, removed for remove), has no change with impact but the
// one the U1 fixture P2 gives every key (a policy variable in its condition
// key, which plan cannot decide), and is a fixture (tagged project=tag-refactor-fixture) or, for untagged, the
// never-tagged queue of U2; every listed ARN is in the plan.
// record: the record is completed, has no failed or pending entry, no entry
// outside the plan, and every listed ARN applied.
// entry: the entry of the ARN is applied and its tags before were empty.
// It prints one line on success and exits 1 with the reason otherwise.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const checkKey = "tag-refactor-u6-check"

type plan struct {
	ImpactAnalysisComplete bool `json:"impactAnalysisComplete"`
	ImpactFindings         []struct {
		Verdict     string   `json:"verdict"`
		PolicyRef   string   `json:"policyRef"`
		StatementID string   `json:"statementId"`
		ChangeIDs   []string `json:"changeIds"`
	} `json:"impactFindings"`
	Resources []struct {
		ARN        string            `json:"arn"`
		BeforeTags map[string]string `json:"beforeTags"`
		Changes    []struct {
			ChangeID string `json:"changeId"`
			Kind     string `json:"kind"`
			FromKey  string `json:"fromKey"`
			ToKey    string `json:"toKey"`
			Impacted bool   `json:"impacted"`
		} `json:"changes"`
	} `json:"resources"`
}

type record struct {
	RunID     string `json:"runId"`
	Completed bool   `json:"completed"`
	Entries   []struct {
		ARN        string            `json:"arn"`
		Status     string            `json:"status"`
		Reason     string            `json:"reason"`
		BeforeTags map[string]string `json:"beforeTags"`
	} `json:"entries"`
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "u6check: "+format+"\n", args...)
	os.Exit(1)
}

func read(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		fail("cannot read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		fail("%s is not valid JSON: %v", path, err)
	}
}

func main() {
	if len(os.Args) < 3 {
		fail("usage: u6check plan|record|entry <file> ...")
	}
	switch os.Args[1] {
	case "plan":
		checkPlan(os.Args[2], os.Args[3], os.Args[4], os.Args[5:])
	case "record":
		checkRecord(os.Args[2], os.Args[3], os.Args[4:])
	case "entry":
		checkEntry(os.Args[2], os.Args[3])
	default:
		fail("unknown check %q", os.Args[1])
	}
}

// p2Finding reports whether a finding is the one the U1 fixture P2 gives
// every tag key: undecided, from its statement PolicyVariableKey, whose
// condition key is aws:ResourceTag/${aws:PrincipalTag/team}.
func p2Finding(verdict, policyRef, statementID string) bool {
	return verdict == "undecided" && statementID == "PolicyVariableKey" &&
		strings.Contains(policyRef, ":policy/tag-refactor-u1-fixtures-P2-")
}

func checkPlan(path, mode, u2Queue string, want []string) {
	var p plan
	read(path, &p)
	if !p.ImpactAnalysisComplete {
		fail("the impact analysis of %s is not complete", path)
	}
	// other holds the changes that a finding other than that of P2 names.
	other := map[string]bool{}
	for _, f := range p.ImpactFindings {
		if !p2Finding(f.Verdict, f.PolicyRef, f.StatementID) {
			for _, id := range f.ChangeIDs {
				other[id] = true
			}
		}
	}
	in := map[string]bool{}
	for _, r := range p.Resources {
		in[r.ARN] = true
		fixture := r.BeforeTags["project"] == "tag-refactor-fixture"
		if !fixture && !(mode == "untagged" && r.ARN == u2Queue) {
			fail("%s is in the plan but is not a fixture", r.ARN)
		}
		for _, c := range r.Changes {
			ok := (mode == "remove" && c.Kind == "removeKey" && c.FromKey == checkKey) ||
				(mode != "remove" && c.Kind == "addRequired" && c.ToKey == checkKey)
			if !ok {
				fail("%s has the change %s %s%s, not only the check key", r.ARN, c.Kind, c.FromKey, c.ToKey)
			}
			if other[c.ChangeID] {
				fail("%s has a change with impact: a policy of the fixtures other than P2 uses %s", r.ARN, checkKey)
			}
		}
	}
	for _, arn := range want {
		if !in[arn] {
			fail("%s is not in the plan: it was not collected", arn)
		}
	}
	fmt.Printf("plan (%s): %d resources, only %s, no impact but that of P2\n", mode, len(p.Resources), checkKey)
}

func checkRecord(path, planPath string, want []string) {
	var r record
	var p plan
	read(path, &r)
	read(planPath, &p)
	inPlan := map[string]bool{}
	for _, res := range p.Resources {
		inPlan[res.ARN] = true
	}
	status := map[string]string{}
	counts := map[string]int{}
	for _, e := range r.Entries {
		status[e.ARN] = e.Status
		counts[e.Status]++
		if !inPlan[e.ARN] {
			fail("the record of %s has %s, which is not in the plan", r.RunID, e.ARN)
		}
		if e.Status == "failed" || e.Status == "pending" {
			fail("the record of %s has %s %s (%s)", r.RunID, e.ARN, e.Status, e.Reason)
		}
	}
	if !r.Completed {
		fail("the record of %s is not completed", r.RunID)
	}
	for _, arn := range want {
		if status[arn] != "applied" {
			fail("%s is %q in the record of %s, not applied", arn, status[arn], r.RunID)
		}
	}
	fmt.Printf("record %s: completed, applied %d, skipped %d\n", r.RunID, counts["applied"], counts["skipped"])
}

func checkEntry(path, arn string) {
	var r record
	read(path, &r)
	for _, e := range r.Entries {
		if e.ARN != arn {
			continue
		}
		if e.Status != "applied" || len(e.BeforeTags) != 0 {
			fail("%s is %s with the tags before %v; want applied with none", arn, e.Status, e.BeforeTags)
		}
		fmt.Printf("record %s: %s applied, no tags before (aws: ones included)\n", r.RunID, arn)
		return
	}
	fail("%s is not in the record of %s", arn, r.RunID)
}
