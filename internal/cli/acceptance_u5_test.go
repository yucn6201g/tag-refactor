package cli_test

// Acceptance tests for U5 (the complete impact analysis: IAM inline
// policies, permissions boundaries, and the resource policies of S3, KMS,
// SQS, SNS and Secrets Manager, every tag condition key and policy
// variables, for the four kinds of change). Like the U1 to U4 tests, each
// one is an acceptance criterion of US3.2 written as Given/When/Then and run
// from the command entry point (cli.Run) against the HTTP-level fake AWS,
// with the AWS environment isolated and the language fixed.
//
// These tests were written and committed before the U5 implementation (team
// testing posture: acceptance criteria first). Every combination of policy
// kind, reference and change is checked by the detection table of the impact
// package (NFR2.1); AC3.2.6 here checks a representative of each through the
// whole command.

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// impactFinding is one entry of impactFindings in plan --format json
// (contract C3, with the conditionOperator U5 adds).
type impactFinding struct {
	Verdict, PolicyRef, SourceKind, StatementID string
	ConditionOperator, ConditionKey             string
	ConditionValue                              string
	ChangeIDs                                   []string
}

// unreadPolicy is one entry of unreadPolicies, with the policyRef and region
// U5 adds.
type unreadPolicy struct{ SourceKind, PolicyRef, Region, Reason string }

// impactView is the part of plan --format json the U5 tests read.
type impactView struct {
	ExitCode *int
	Complete struct{ ImpactAnalysis *bool }
	Summary  struct{ AffectedReferences, UndecidedReferences int }
	Details  struct {
		ImpactFindings []impactFinding
		UnreadPolicies []unreadPolicy
		Resources      []struct {
			ARN     string
			Changes []planChange
		}
	}
}

// changeID returns the ID of the change of the resource that matches kind,
// failing the test when there is none.
func (v impactView) changeID(t *testing.T, arn, kind string) string {
	t.Helper()
	for _, r := range v.Details.Resources {
		if r.ARN != arn {
			continue
		}
		for _, c := range r.Changes {
			if c.Kind == kind {
				return c.ChangeID
			}
		}
	}
	t.Fatalf("no %s change for %s: %+v", kind, arn, v.Details.Resources)
	return ""
}

// finding returns the findings of the policy with the condition key and
// verdict.
func (v impactView) finding(ref, conditionKey, verdict string) (impactFinding, bool) {
	for _, f := range v.Details.ImpactFindings {
		if f.PolicyRef == ref && f.ConditionKey == conditionKey && f.Verdict == verdict {
			return f, true
		}
	}
	return impactFinding{}, false
}

// requireFinding is finding, failing the test when it is missing.
func (v impactView) requireFinding(t *testing.T, ref, conditionKey, verdict string) impactFinding {
	t.Helper()
	f, ok := v.finding(ref, conditionKey, verdict)
	if !ok {
		t.Fatalf("no %s finding for %s %s; findings:\n%+v", verdict, ref, conditionKey, v.Details.ImpactFindings)
	}
	return f
}

// planImpact runs plan --format json and decodes it.
func (w *world) planImpact(args ...string) (result, impactView) {
	w.t.Helper()
	r := w.run(append(append([]string{"plan"}, args...), "--format", "json")...)
	var v impactView
	decodeOne(w.t, r.stdout, &v)
	return r, v
}

func policyDoc(statements ...string) string {
	return `{"Version":"2012-10-17","Statement":[` + strings.Join(statements, ",") + `]}`
}

// conditionStatement is a statement with one condition.
func conditionStatement(sid, operator, key, value string) string {
	return `{"Sid":"` + sid + `","Effect":"Allow","Action":"sqs:GetQueueAttributes","Resource":"*",` +
		`"Condition":{"` + operator + `":{"` + key + `":` + value + `}}}`
}

// AC3.2.1: Given an inline policy of an IAM role uses the env tag in a
// condition, When a plan renames env to Environment, Then the policy is shown
// as affected (!) with its reference <role ARN>#<name>, the statement and
// the condition, and the saved and JSON forms carry the operator.
func TestAcceptanceU5_AC3_2_1_RoleInlinePolicyIsAffectedByARename(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{Roles: []testutil.Principal{{Name: "app", Inline: map[string]string{
		"allow-env": policyDoc(conditionStatement("AllowEnv", "StringEquals", "aws:ResourceTag/env", `"dev"`)),
	}}}})
	rules := w.givenRuleFile(aliasRules)
	ref := testutil.RoleARN("app") + "#allow-env"

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	j, v := w.planImpact("--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	j.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! IAM inline policy\s+`+regexp.QuoteMeta(ref)+`$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+Statement "AllowEnv"\s+Condition StringEquals aws:ResourceTag/env = "dev"$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+affected by: rename key env -> Environment$`)
	requireContains(t, "stdout", r.stdout, "1 affected · 0 undecided")
	f := v.requireFinding(t, ref, "aws:ResourceTag/env", "affected")
	if f.SourceKind != "iamInline" || f.ConditionOperator != "StringEquals" || f.StatementID != "AllowEnv" || f.ConditionValue != "dev" ||
		!slices.Equal(f.ChangeIDs, []string{v.changeID(t, arnQA, "renameKey")}) {
		t.Fatalf("finding = %+v", f)
	}
}

// AC3.2.2: Given an S3 bucket policy uses the value Prod of the Stage tag,
// When a plan changes Prod to prod on that bucket, Then the bucket policy is
// shown as affected (!). The same change on another bucket does not affect
// it (a resource policy belongs to its own resource only, BR3.2).
func TestAcceptanceU5_AC3_2_2_BucketPolicyIsAffectedByAValueChange(t *testing.T) {
	w := newWorld(t)
	other := "arn:aws:s3:::other-bucket"
	w.givenResources(
		testutil.TaggedResource{ARN: testutil.BucketARN("logs-bucket"), Tags: map[string]string{"Stage": "Prod"}},
		testutil.TaggedResource{ARN: other, Tags: map[string]string{"Stage": "Prod"}},
	)
	deny := `{"Sid":"DenyNonProd","Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::logs-bucket/*",` +
		`"Condition":{"StringNotEquals":{"aws:ResourceTag/Stage":"Prod"}}}`
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{Buckets: []testutil.BucketPolicy{
		{Name: "logs-bucket", Region: "us-east-1", Policy: policyDoc(deny)},
		{Name: "other-bucket", Region: "us-east-1"},
	}})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Stage:\n    values:\n      prod: [Prod]\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	j, v := w.planImpact("--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	j.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! S3 bucket policy\s+arn:aws:s3:::logs-bucket$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+Statement "DenyNonProd"\s+Condition StringNotEquals aws:ResourceTag/Stage = "Prod"$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+affected by: value Stage: Prod -> prod$`)
	f := v.requireFinding(t, "arn:aws:s3:::logs-bucket", "aws:ResourceTag/Stage", "affected")
	if f.SourceKind != "s3Bucket" || !slices.Equal(f.ChangeIDs, []string{v.changeID(t, testutil.BucketARN("logs-bucket"), "changeValue")}) {
		t.Fatalf("the bucket policy must name only its own bucket's change: %+v", f)
	}
	if len(v.Details.ImpactFindings) != 1 || *v.Complete.ImpactAnalysis != true {
		t.Fatalf("findings = %+v", v.Details.ImpactFindings)
	}
}

// AC3.2.3: Given a policy condition uses a tag, When a plan removes that tag,
// Then the policy is shown as affected (!).
func TestAcceptanceU5_AC3_2_3_RemovingATagAffectsThePolicy(t *testing.T) {
	w := newWorld(t)
	topic := testutil.TopicARN("us-east-1", "alerts")
	w.givenResources(testutil.TaggedResource{ARN: topic, Tags: map[string]string{"Environment": "dev", "tmp": "x"}})
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{Topics: []testutil.RegionalPolicy{{Region: "us-east-1", Name: "alerts",
		Policy: policyDoc(conditionStatement("OnlyTmp", "StringEquals", "aws:ResourceTag/tmp", `"x"`))}}})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Environment:\n    aliases: [env]\nremove: [tmp]\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	_, v := w.planImpact("--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! SNS topic policy\s+`+regexp.QuoteMeta(topic)+`$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+affected by: remove tmp$`)
	f := v.requireFinding(t, topic, "aws:ResourceTag/tmp", "affected")
	if f.SourceKind != "snsTopic" || !slices.Equal(f.ChangeIDs, []string{v.changeID(t, topic, "removeKey")}) {
		t.Fatalf("finding = %+v", f)
	}
}

// AC3.2.4: Given conditions with a policy variable or a wildcard, When plan
// runs, Then the policy variable is shown as "cannot decide" (?), the
// wildcard that matches is shown as affected (!), and neither is shown as
// having no impact.
func TestAcceptanceU5_AC3_2_4_VariablesAreUndecidedAndWildcardsAreMatched(t *testing.T) {
	w := newWorld(t)
	secret := testutil.SecretARN("us-east-1", "db")
	w.givenResources(
		testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: secret, Tags: map[string]string{"team": "Data"}},
	)
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{
		Managed: []testutil.ManagedPolicyDoc{{Name: "wildcard", Document: policyDoc(conditionStatement("Wildcard", "StringLike", "aws:ResourceTag/en*", `"dev"`))}},
		Roles: []testutil.Principal{{Name: "app", Inline: map[string]string{
			"abac": policyDoc(conditionStatement("KeyVariable", "StringEquals", "aws:ResourceTag/${aws:PrincipalTag/k}", `"yes"`)),
		}}},
		Secrets: []testutil.RegionalPolicy{{Region: "us-east-1", Name: "db",
			Policy: policyDoc(conditionStatement("ValueVariable", "StringEquals", "aws:ResourceTag/team", `"${aws:PrincipalTag/team}"`))}},
	})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n  team:\n    values:\n      data: [Data]\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")
	_, v := w.planImpact("--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	rename, value := v.changeID(t, arnQA, "renameKey"), v.changeID(t, secret, "changeValue")
	wildcard := v.requireFinding(t, testutil.ManagedPolicyARN("wildcard"), "aws:ResourceTag/en*", "affected")
	keyVariable := v.requireFinding(t, testutil.RoleARN("app")+"#abac", "aws:ResourceTag/${aws:PrincipalTag/k}", "undecided")
	valueVariable := v.requireFinding(t, secret, "aws:ResourceTag/team", "undecided")
	both := []string{rename, value}
	slices.Sort(both) // change order
	if !slices.Equal(wildcard.ChangeIDs, []string{rename}) || !slices.Equal(keyVariable.ChangeIDs, both) ||
		!slices.Equal(valueVariable.ChangeIDs, []string{value}) || valueVariable.SourceKind != "secretsManagerSecret" {
		t.Fatalf("findings = %+v", v.Details.ImpactFindings)
	}
	if len(v.Details.ImpactFindings) != 3 {
		t.Fatalf("unexpected findings: %+v", v.Details.ImpactFindings)
	}
	requireMatch(t, "stdout", r.stdout, `(?m)^  \? Secrets Manager secret policy\s+`+regexp.QuoteMeta(secret)+`$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+cannot decide \(policy variable\)$`)
	requireContains(t, "stdout", r.stdout, "1 affected · 2 undecided")
}

// AC3.2.5: Given the IAM policies of the account cannot be read, When plan
// runs, Then the kinds that could not be read and the reason are shown, the
// impact analysis is reported as incomplete, the exit code is 1, and the
// findings of the other policies are still shown. IAM is read in one go
// (iam:GetAccountAuthorizationDetails), so a denied read leaves both IAM
// kinds unread as whole lists and matches no IAM policy. A list denied in
// one region names that region; the other regions are read (BR1.5).
func TestAcceptanceU5_AC3_2_5_DeniedListsMakeTheAnalysisIncomplete(t *testing.T) {
	w := newWorld(t)
	queueEU := testutil.QueueARN("eu-west-1", "eu-queue")
	w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
		"us-east-1": testutil.TagPages([]testutil.TaggedResource{{ARN: arnQA, Tags: map[string]string{"env": "dev"}}}),
		"eu-west-1": testutil.TagPages([]testutil.TaggedResource{{ARN: queueEU, Tags: map[string]string{"env": "dev"}}}),
	}, testutil.TagPages(nil)))
	denied := testutil.QueryError(http.StatusForbidden, "AccessDenied", "User: tester is not authorized to perform: iam:GetAccountAuthorizationDetails")
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{
		Managed: []testutil.ManagedPolicyDoc{{Name: "DevAccess", Document: policyEnvCondition}},
		Roles:   []testutil.Principal{{Name: "app", Inline: map[string]string{"hidden": policyEnvCondition}}},
		Queues: []testutil.RegionalPolicy{{Region: "us-east-1", Name: "qa",
			Policy: policyDoc(conditionStatement("Q", "StringEquals", "aws:ResourceTag/env", `"dev"`))}},
		Fail: map[string]testutil.Reply{
			testutil.OpGetAccountAuthorizationDetails: denied,
			testutil.OpListQueues + " eu-west-1": testutil.SQSError(http.StatusBadRequest, "AccessDenied", "AccessDenied",
				"User: tester is not authorized to perform: sqs:ListQueues"),
		},
	})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--region", "eu-west-1")
	j, v := w.planImpact("--rules", rules, "--region", "us-east-1", "--region", "eu-west-1")

	r.requireExit(t, 1)
	j.requireExit(t, 1)
	if !strings.HasPrefix(r.stdout, "! Impact analysis is incomplete\n") {
		t.Fatalf("stdout must start with the incomplete marker:\n%s", r.stdout)
	}
	requireMatch(t, "stdout", r.stdout, `(?m)^    IAM managed policies: access denied to iam:GetAccountAuthorizationDetails$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^    IAM inline policies: access denied to iam:GetAccountAuthorizationDetails$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^    SQS queue policies in eu-west-1: access denied to sqs:ListQueues$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! SQS queue policy\s+`+regexp.QuoteMeta(testutil.QueueARN("us-east-1", "qa"))+`$`)
	requireNotContains(t, "stdout", r.stdout, testutil.ManagedPolicyARN("DevAccess"))
	requireNotContains(t, "stdout", r.stdout, "not authorized")
	requireContains(t, "stderr", r.stderr, "Exit code: 1")
	if len(v.Details.ImpactFindings) != 1 || v.Details.ImpactFindings[0].SourceKind != "sqsQueue" {
		t.Fatalf("only the queue policy can be matched: %+v", v.Details.ImpactFindings)
	}
	want := []unreadPolicy{
		{SourceKind: "iamManaged", Reason: "access denied to iam:GetAccountAuthorizationDetails"},
		{SourceKind: "iamInline", Reason: "access denied to iam:GetAccountAuthorizationDetails"},
		{SourceKind: "sqsQueue", Region: "eu-west-1", Reason: "access denied to sqs:ListQueues"},
	}
	if !slices.Equal(v.Details.UnreadPolicies, want) || *v.Complete.ImpactAnalysis {
		t.Fatalf("unread = %+v, want %+v", v.Details.UnreadPolicies, want)
	}
}

// AC3.2.6: Given policies of every kind (customer managed, the inline
// policies of a user, a role and a group, and the resource policies of S3,
// KMS, SQS, SNS and Secrets Manager) that use every kind of tag reference,
// When plan runs, Then every one of them is detected. The full table of
// combinations is checked by the detection table of the impact package
// (NFR2.1).
func TestAcceptanceU5_AC3_2_6_EveryPolicyKindAndReferenceIsDetected(t *testing.T) {
	w := newWorld(t)
	region := "us-east-1"
	role := testutil.RoleARN("app")
	queue, bucket, key := testutil.QueueARN(region, "q1"), testutil.BucketARN("b1"), testutil.KeyARN(region, "k1")
	topic, secret := testutil.TopicARN(region, "t1"), testutil.SecretARN(region, "s1")
	w.givenResources(
		testutil.TaggedResource{ARN: queue, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: bucket, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: key, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: topic, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: secret, Tags: map[string]string{"env": "dev"}},
		testutil.TaggedResource{ARN: role, Tags: map[string]string{"team": "Data", "k": "v1"}},
	)
	queueResource := `{"Sid":"QueueVar","Effect":"Allow","Principal":"*","Action":"sqs:SendMessage",` +
		`"Resource":"arn:aws:sqs:us-east-1:123456789012:${aws:PrincipalTag/team}"}`
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{
		Managed: []testutil.ManagedPolicyDoc{{Name: "boundary", NotAttached: true, Boundaries: 1,
			Document: policyDoc(conditionStatement("ResourceTag", "StringEquals", "aws:ResourceTag/env", `"dev"`))}},
		Users:  []testutil.Principal{{Name: "u1", Inline: map[string]string{"p": policyDoc(conditionStatement("TagKeys", "ForAnyValue:StringEquals", "aws:TagKeys", `["env"]`))}}},
		Roles:  []testutil.Principal{{Name: "app", Inline: map[string]string{"p": policyDoc(conditionStatement("PrincipalTag", "StringEquals", "aws:PrincipalTag/team", `"data"`))}}},
		Groups: []testutil.Principal{{Name: "g", Inline: map[string]string{"p": policyDoc(conditionStatement("RequestTag", "StringEquals", "aws:RequestTag/env", `"dev"`))}}},
		Buckets: []testutil.BucketPolicy{{Name: "b1", Region: region,
			Policy: policyDoc(conditionStatement("ServiceKey", "StringEquals", "s3:ResourceTag/env", `"dev"`))}},
		Keys: []testutil.KeyPolicy{{Region: region, ID: "k1",
			Policy: policyDoc(conditionStatement("KeyVariable", "StringEquals", "aws:ResourceTag/${aws:PrincipalTag/k}", `"yes"`))},
			{Region: region, ID: "aws-managed", AWSManaged: true, Policy: policyDoc(conditionStatement("X", "StringEquals", "aws:ResourceTag/env", `"dev"`))}},
		Queues: []testutil.RegionalPolicy{{Region: region, Name: "q1", Policy: policyDoc(queueResource)}},
		Topics: []testutil.RegionalPolicy{{Region: region, Name: "t1",
			Policy: policyDoc(conditionStatement("IfExists", "StringEqualsIfExists", "aws:ResourceTag/env", `"dev"`))}},
		Secrets: []testutil.RegionalPolicy{{Region: region, Name: "s1",
			Policy: policyDoc(conditionStatement("ValueVariable", "StringEquals", "aws:ResourceTag/team", `"${aws:PrincipalTag/team}"`))}},
	})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n  team:\n    values:\n      data: [Data]\n  K:\n    aliases: [k]\n")

	j, v := w.planImpact("--rules", rules, "--region", region)

	j.requireExit(t, 2)
	if !*v.Complete.ImpactAnalysis || len(v.Details.UnreadPolicies) != 0 {
		t.Fatalf("the analysis must be complete: %+v", v.Details.UnreadPolicies)
	}
	roleValue, roleRename := v.changeID(t, role, "changeValue"), v.changeID(t, role, "renameKey")
	for _, want := range []struct {
		kind, ref, conditionKey, verdict string
		changeIDs                        []string
	}{
		{"iamManaged", testutil.ManagedPolicyARN("boundary"), "aws:ResourceTag/env", "affected", nil},
		{"iamInline", testutil.UserARN("u1") + "#p", "aws:TagKeys", "affected", nil},
		{"iamInline", role + "#p", "aws:PrincipalTag/team", "affected", []string{roleValue}},
		{"iamInline", testutil.GroupARN("g") + "#p", "aws:RequestTag/env", "affected", nil},
		{"s3Bucket", bucket, "s3:ResourceTag/env", "affected", []string{v.changeID(t, bucket, "renameKey")}},
		{"kmsKey", key, "aws:ResourceTag/${aws:PrincipalTag/k}", "affected", []string{roleRename}},
		{"kmsKey", key, "aws:ResourceTag/${aws:PrincipalTag/k}", "undecided", []string{v.changeID(t, key, "renameKey")}},
		{"sqsQueue", queue, "Resource", "undecided", []string{roleValue}},
		{"snsTopic", topic, "aws:ResourceTag/env", "affected", []string{v.changeID(t, topic, "renameKey")}},
		{"secretsManagerSecret", secret, "aws:ResourceTag/team", "undecided", []string{roleValue}},
	} {
		f, ok := v.finding(want.ref, want.conditionKey, want.verdict)
		if !ok || f.SourceKind != want.kind || (want.changeIDs != nil && !slices.Equal(f.ChangeIDs, want.changeIDs)) {
			t.Errorf("missing %s %s %s %s (%v); got %+v", want.verdict, want.kind, want.ref, want.conditionKey, want.changeIDs, f)
		}
	}
	for _, f := range v.Details.ImpactFindings {
		if strings.Contains(f.PolicyRef, "aws-managed") {
			t.Errorf("an AWS managed KMS key must not be read: %+v", f)
		}
	}
	if f, _ := v.finding(queue, "Resource", "undecided"); f.ConditionOperator != "" || f.ConditionValue != "arn:aws:sqs:us-east-1:123456789012:${aws:PrincipalTag/team}" {
		t.Errorf("a Resource finding has no operator and the Resource values: %+v", f)
	}
}
