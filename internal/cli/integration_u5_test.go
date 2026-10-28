//go:build integration

package cli_test

// Sandbox account test for the U5 checkpoint. Like the U1 to U4 tests it
// calls a real AWS account, so it only runs locally with the build tag
// "integration" and never in CI. scripts/verify.sh runs it after the U4
// check, with the read-only profile R2 and the outputs of the U5 fixtures
// stack (test/sandbox/u5-fixtures.yaml):
//
//	TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly \
//	TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<sandbox account ID> \
//	TAG_REFACTOR_IT_U5_ROLE_ARN=... TAG_REFACTOR_IT_U5_BUCKET_ARN=... \
//	TAG_REFACTOR_IT_U5_KEY_ARN=... TAG_REFACTOR_IT_U5_QUEUE_ARN=... \
//	TAG_REFACTOR_IT_U5_TOPIC_ARN=... TAG_REFACTOR_IT_U5_SECRET_ARN=... \
//	go test -mod=readonly -tags integration -run '^TestIntegrationU5' ./internal/cli/...
//
// It assumes every policy of the sandbox account can be read with R2: the
// account holds no customer resource policies besides the fixture stacks.
// It has no subtests, so verify.sh can count its one PASS line.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const u5Region = "ap-northeast-1"

// u5SandboxView is plan --format json with the account.
type u5SandboxView struct {
	AccountID string
	impactView
}

func TestIntegrationU5ImpactSandbox(t *testing.T) {
	profile, account := os.Getenv("TAG_REFACTOR_IT_PROFILE"), os.Getenv("TAG_REFACTOR_SANDBOX_ACCOUNT_ID")
	stack := map[string]string{
		"role": os.Getenv("TAG_REFACTOR_IT_U5_ROLE_ARN"), "s3Bucket": os.Getenv("TAG_REFACTOR_IT_U5_BUCKET_ARN"),
		"kmsKey": os.Getenv("TAG_REFACTOR_IT_U5_KEY_ARN"), "sqsQueue": os.Getenv("TAG_REFACTOR_IT_U5_QUEUE_ARN"),
		"snsTopic": os.Getenv("TAG_REFACTOR_IT_U5_TOPIC_ARN"), "secretsManagerSecret": os.Getenv("TAG_REFACTOR_IT_U5_SECRET_ARN"),
	}
	if profile == "" || account == "" || slices.Contains(mapValues(stack), "") {
		t.Skip("set TAG_REFACTOR_IT_PROFILE, TAG_REFACTOR_SANDBOX_ACCOUNT_ID and the TAG_REFACTOR_IT_U5_* ARNs to run this test (scripts/verify.sh does)")
	}
	// 1. The language is fixed, whatever the computer's.
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "C")
	rules := filepath.Join("..", "..", "test", "sandbox", "u5-rules.yaml")

	// 2. plan with the read-only profile, so a permission missing from R2
	// shows.
	r := runAgainstAWS(t, "plan", "--profile", profile, "--region", u5Region, "--rules", rules, "--lang", "en", "--format", "json")

	// 3. The sandbox account, and changes to review.
	var v u5SandboxView
	decodeOne(t, r.stdout, &v)
	if v.AccountID != account {
		t.Fatalf("the plan is for account %s, not the sandbox account", v.AccountID)
	}

	// 4. Every policy was read; otherwise tell what could not be.
	if v.Complete.ImpactAnalysis == nil || !*v.Complete.ImpactAnalysis {
		outside := 0
		for _, u := range v.Details.UnreadPolicies {
			where := "outside the stacks"
			switch {
			case u.PolicyRef == "":
				where = "a whole list"
			case isStackPolicy(u.PolicyRef, stack):
				where = "a policy of the U5 stack"
			default:
				outside++
			}
			t.Logf("unread (%s): %s %s %s: %s", where, u.SourceKind, u.Region, u.PolicyRef, u.Reason)
		}
		if outside == len(v.Details.UnreadPolicies) {
			t.Fatal("R2 cannot read policies of the account outside the stacks; check the permissions of R2 or the state of the account")
		}
		t.Fatalf("the impact analysis is incomplete (exit code %d)", r.code)
	}
	r.requireExit(t, 2)

	// 5. The fixtures are found: the inline policy of RU5, the five
	// resource policies and P1 of the U1 stack.
	v.requireFinding(t, stack["role"]+"#u5-inline", "aws:ResourceTag/env", "affected")
	owner := changeOwners(v.impactView)
	for _, kind := range []string{"s3Bucket", "kmsKey", "sqsQueue", "snsTopic", "secretsManagerSecret"} {
		f := v.requireFinding(t, stack[kind], "aws:ResourceTag/env", "affected")
		if f.SourceKind != kind {
			t.Errorf("%s has the kind %s, want %s", stack[kind], f.SourceKind, kind)
		}
		// 6. A resource policy names only its own resource's changes.
		for _, id := range f.ChangeIDs {
			if owner[id] != stack[kind] {
				t.Errorf("the policy of %s names the change %s of %s", stack[kind], id, owner[id])
			}
		}
	}
	p1 := false
	for _, f := range v.Details.ImpactFindings {
		if f.SourceKind == "iamManaged" && f.StatementID == "AllowDevQueues" && f.ConditionKey == "aws:ResourceTag/env" && f.Verdict == "affected" {
			p1 = true
		}
	}
	if !p1 {
		t.Error("P1 of the U1 stack is not affected")
	}

	// 7. A role has no region: whether the tagging API returns RU5 for
	// ap-northeast-1 is not known yet, so the Resource finding is checked
	// only when the plan changes the team tag of RU5.
	teamChange := ""
	for _, res := range v.Details.Resources {
		for _, c := range res.Changes {
			if res.ARN == stack["role"] && c.FromKey != nil && *c.FromKey == "team" {
				teamChange = c.ChangeID
			}
		}
	}
	if teamChange == "" {
		t.Log("RU5 was not collected: PrincipalTag and Resource are checked by the fake AWS tests")
	} else {
		f := v.requireFinding(t, stack["role"]+"#u5-inline", "Resource", "affected")
		if !slices.Contains(f.ChangeIDs, teamChange) {
			t.Errorf("the Resource finding does not name the team change %s: %+v", teamChange, f)
		}
		t.Log("RU5 was collected: its Resource finding is affected by the team change")
	}
	t.Logf("U5 findings: %d, unread: %d", len(v.Details.ImpactFindings), len(v.Details.UnreadPolicies))
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// isStackPolicy reports whether ref belongs to the U5 stack.
func isStackPolicy(ref string, stack map[string]string) bool {
	for _, arn := range stack {
		if ref == arn || strings.HasPrefix(ref, arn+"#") {
			return true
		}
	}
	return false
}

// changeOwners maps each change to the ARN of its resource.
func changeOwners(v impactView) map[string]string {
	out := map[string]string{}
	for _, res := range v.Details.Resources {
		for _, c := range res.Changes {
			out[c.ChangeID] = res.ARN
		}
	}
	return out
}
