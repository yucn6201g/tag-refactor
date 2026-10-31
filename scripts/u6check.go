//go:build ignore

// u6check checks the plans, run records and tag snapshots of the U6 and U7
// checkpoints for scripts/verify.sh (construction/u6-apply/ and
// construction/u7-rollback/infrastructure-design/cicd-pipeline.md). It reads
// files only and never calls AWS. Usage:
//
//	go run scripts/u6check.go plan <plan.json> <add|remove|rename> [--key <key>] [--dst <key>] [--optional <arn>]... <arn>...
//	go run scripts/u6check.go record <record.json> <plan.json> <arn>...
//	go run scripts/u6check.go rollback <record.json> [--id <rollback-id>] [--restored-min <n>] [--all-already]
//	go run scripts/u6check.go entries <record-before.json> <record-after.json>
//	go run scripts/u6check.go same <snapshot-before.json> <snapshot-after.json> <arn>...
//
// plan: every resource with changes changes the check key only (added for
// add, removed for remove; for rename, the key renamed to --dst and the value
// a of --dst changed to b), has no change with impact but the one the U1
// fixture P2 gives every key (a policy variable in its condition key, which
// plan cannot decide), and is a fixture (tagged project=tag-refactor-fixture)
// or one of the listed ARNs; every listed ARN but the --optional ones is in
// the plan. The key is tag-refactor-u6-check unless --key names another.
// record: the record is completed, has no failed or pending entry, no entry
// outside the plan, and every listed ARN applied.
// rollback: the rollback --id of the record (the last one without --id) is
// completed and has no failed or skipped result; --restored-min asks for at
// least that many restored results, --all-already for every result
// alreadyRestored.
// entries: the entries and completed of the two records are the same, the
// rollbacks of the first are kept, and the second has one more completed
// rollback.
// same: the tags of the listed ARNs, without aws: keys, are the same in the
// two snapshots (the JSON of "aws resourcegroupstaggingapi get-resources", or
// a list of them); an ARN a snapshot leaves out has no tags. It names the
// keys that differ, never their values.
// It prints one line on success and exits 1 with the reason otherwise.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
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
			ChangeID  string `json:"changeId"`
			Kind      string `json:"kind"`
			FromKey   string `json:"fromKey"`
			ToKey     string `json:"toKey"`
			FromValue string `json:"fromValue"`
			ToValue   string `json:"toValue"`
		} `json:"changes"`
	} `json:"resources"`
}

type record struct {
	RunID     string `json:"runId"`
	Completed bool   `json:"completed"`
	Entries   []struct {
		ARN    string `json:"arn"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"entries"`
	Rollbacks []struct {
		RollbackID string `json:"rollbackId"`
		Completed  bool   `json:"completed"`
		Results    []struct {
			ARN    string `json:"arn"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"results"`
	} `json:"rollbacks"`
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
		fail("usage: u6check plan|record|rollback|entries|same <file> ...")
	}
	switch os.Args[1] {
	case "plan":
		if len(os.Args) < 4 {
			fail("usage: u6check plan <plan.json> <add|remove|rename> ...")
		}
		checkPlan(os.Args[2], os.Args[3], os.Args[4:])
	case "record":
		if len(os.Args) < 4 {
			fail("usage: u6check record <record.json> <plan.json> <arn>...")
		}
		checkRecord(os.Args[2], os.Args[3], os.Args[4:])
	case "rollback":
		checkRollback(os.Args[2], os.Args[3:])
	case "entries":
		if len(os.Args) != 4 {
			fail("usage: u6check entries <record-before.json> <record-after.json>")
		}
		checkEntries(os.Args[2], os.Args[3])
	case "same":
		if len(os.Args) < 4 {
			fail("usage: u6check same <before.json> <after.json> <arn>...")
		}
		checkSame(os.Args[2], os.Args[3], os.Args[4:])
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

// planArgs are the options of plan after the mode.
type planArgs struct {
	key, dst           string
	required, optional []string
}

func parsePlanArgs(args []string) planArgs {
	a := planArgs{key: checkKey}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--key", "--dst", "--optional":
			if i+1 >= len(args) {
				fail("%s needs a value", args[i])
			}
			switch args[i] {
			case "--key":
				a.key = args[i+1]
			case "--dst":
				a.dst = args[i+1]
			default:
				a.optional = append(a.optional, args[i+1])
			}
			i++
		default:
			a.required = append(a.required, args[i])
		}
	}
	return a
}

func checkPlan(path, mode string, args []string) {
	a := parsePlanArgs(args)
	if mode == "rename" && a.dst == "" {
		fail("rename needs --dst")
	}
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
	listed := map[string]bool{}
	for _, arn := range append(append([]string{}, a.required...), a.optional...) {
		listed[arn] = true
	}
	in := map[string]bool{}
	for _, r := range p.Resources {
		in[r.ARN] = true
		if r.BeforeTags["project"] != "tag-refactor-fixture" && !listed[r.ARN] {
			fail("%s is in the plan but is neither a fixture nor listed", r.ARN)
		}
		for _, c := range r.Changes {
			ok := false
			switch mode {
			case "remove":
				ok = c.Kind == "removeKey" && c.FromKey == a.key
			case "add":
				ok = c.Kind == "addRequired" && c.ToKey == a.key
			case "rename":
				ok = c.Kind == "renameKey" && c.FromKey == a.key && c.ToKey == a.dst ||
					c.Kind == "changeValue" && c.FromKey == a.dst && c.FromValue == "a" && c.ToValue == "b"
			}
			if !ok {
				fail("%s has the change %s %s%s, not only the check key %s", r.ARN, c.Kind, c.FromKey, c.ToKey, a.key)
			}
			if other[c.ChangeID] {
				fail("%s has a change with impact: a policy of the fixtures other than P2 uses %s", r.ARN, a.key)
			}
		}
	}
	for _, arn := range a.required {
		if !in[arn] {
			fail("%s is not in the plan: it was not collected", arn)
		}
	}
	fmt.Printf("plan (%s): %d resources, only %s, no impact but that of P2\n", mode, len(p.Resources), a.key)
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

func checkRollback(path string, args []string) {
	id, restoredMin, allAlready := "", 0, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id", "--restored-min":
			if i+1 >= len(args) {
				fail("%s needs a value", args[i])
			}
			if args[i] == "--id" {
				id = args[i+1]
			} else if n, err := strconv.Atoi(args[i+1]); err == nil {
				restoredMin = n
			} else {
				fail("--restored-min %q is not a number", args[i+1])
			}
			i++
		case "--all-already":
			allAlready = true
		default:
			fail("unknown option %q", args[i])
		}
	}
	var r record
	read(path, &r)
	if len(r.Rollbacks) == 0 {
		fail("the record of %s has no rollback", r.RunID)
	}
	rb := r.Rollbacks[len(r.Rollbacks)-1]
	if id != "" {
		found := false
		for _, x := range r.Rollbacks {
			if x.RollbackID == id {
				rb, found = x, true
			}
		}
		if !found {
			fail("the record of %s has no rollback %s", r.RunID, id)
		}
	}
	if !rb.Completed {
		fail("the rollback %s of %s is not completed", rb.RollbackID, r.RunID)
	}
	counts := map[string]int{}
	for _, res := range rb.Results {
		counts[res.Status]++
		if res.Status == "failed" || res.Status == "skipped" {
			fail("the rollback %s has %s %s (%s)", rb.RollbackID, res.ARN, res.Status, res.Reason)
		}
	}
	if counts["restored"] < restoredMin {
		fail("the rollback %s restored %d resources, fewer than %d", rb.RollbackID, counts["restored"], restoredMin)
	}
	if allAlready && (len(rb.Results) == 0 || counts["alreadyRestored"] != len(rb.Results)) {
		fail("the rollback %s found %d of %d resources already restored", rb.RollbackID, counts["alreadyRestored"], len(rb.Results))
	}
	fmt.Printf("rollback %s of %s: completed, restored %d, already restored %d\n", rb.RollbackID, r.RunID,
		counts["restored"], counts["alreadyRestored"])
}

func checkEntries(beforePath, afterPath string) {
	var before, after map[string]any
	read(beforePath, &before)
	read(afterPath, &after)
	for _, k := range []string{"entries", "completed", "runId", "accountId"} {
		if !reflect.DeepEqual(before[k], after[k]) {
			fail("the %s of the record changed with the rollback", k)
		}
	}
	earlier, _ := before["rollbacks"].([]any)
	later, _ := after["rollbacks"].([]any)
	if len(later) != len(earlier)+1 {
		fail("the record has %d rollbacks after the rollback, %d before", len(later), len(earlier))
	}
	if len(earlier) > 0 && !reflect.DeepEqual(earlier, later[:len(earlier)]) {
		fail("an earlier rollback of the record changed")
	}
	if own, _ := later[len(later)-1].(map[string]any); own["completed"] != true {
		fail("the new rollback of the record is not completed")
	}
	fmt.Printf("entries: unchanged, one more completed rollback (%d in all)\n", len(later))
}

// snapshot reads the tags of a snapshot file: the output of
// "aws resourcegroupstaggingapi get-resources", or a list of them, without
// the keys starting with aws:.
func snapshot(path string) map[string]map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail("cannot read %s: %v", path, err)
	}
	type page struct {
		ResourceTagMappingList []struct {
			ResourceARN string
			Tags        []struct{ Key, Value string }
		}
	}
	var pages []page
	if err := json.Unmarshal(data, &pages); err != nil {
		var one page
		if err := json.Unmarshal(data, &one); err != nil {
			fail("%s is not a snapshot of tags: %v", path, err)
		}
		pages = []page{one}
	}
	out := map[string]map[string]string{}
	for _, p := range pages {
		for _, m := range p.ResourceTagMappingList {
			tags := map[string]string{}
			for _, t := range m.Tags {
				if !strings.HasPrefix(strings.ToLower(t.Key), "aws:") {
					tags[t.Key] = t.Value
				}
			}
			out[m.ResourceARN] = tags
		}
	}
	return out
}

func checkSame(beforePath, afterPath string, arns []string) {
	before, after := snapshot(beforePath), snapshot(afterPath)
	if len(arns) == 0 {
		for arn := range before {
			arns = append(arns, arn)
		}
		for arn := range after {
			if _, ok := before[arn]; !ok {
				arns = append(arns, arn)
			}
		}
	}
	sort.Strings(arns)
	var diffs []string
	for _, arn := range arns {
		b, a := before[arn], after[arn]
		var keys []string
		for k, v := range b {
			if w, ok := a[k]; !ok || w != v {
				keys = append(keys, k)
			}
		}
		for k := range a {
			if _, ok := b[k]; !ok {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			sort.Strings(keys)
			diffs = append(diffs, arn+" ("+strings.Join(keys, ", ")+")")
		}
	}
	if len(diffs) > 0 {
		fail("the tags differ from the snapshot on %s", strings.Join(diffs, "; "))
	}
	fmt.Printf("same: %d resources have the tags of the snapshot (aws: keys left out)\n", len(arns))
}
