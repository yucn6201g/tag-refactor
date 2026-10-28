package cli_test

// Unit tests of plan with the U5 impact analysis, written after the
// implementation: expired credentials met by any of the reads U5 adds stop
// the run (U5 security design §4, NFR5.10), and neither --debug nor the
// unread reasons show a document or AWS's error text (NFR8.22).

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// requireStoppedForCredentials checks the end of a run stopped by expired
// credentials: the two-line message, exit code 1, no partial result, and no
// request left running or arriving afterwards.
func requireStoppedForCredentials(t *testing.T, w *world, r result) {
	t.Helper()
	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: the AWS credentials have expired`)
	if n := strings.Count(r.stderr, "Error:"); n != 1 {
		t.Fatalf("%d error messages, want one:\n%s", n, r.stderr)
	}
	for _, secret := range []string{testutil.FakeAccessKeyID, testutil.FakeSecretAccessKey, testutil.FakeSessionToken} {
		requireNotContains(t, "stderr", r.stderr, secret)
	}
	if n := w.fake.InFlight(); n != 0 {
		t.Fatalf("%d requests still running after the command returned", n)
	}
	before := len(w.fake.Calls())
	time.Sleep(100 * time.Millisecond)
	if after := len(w.fake.Calls()); after != before {
		t.Fatalf("%d requests arrived after the command returned", after-before)
	}
}

// expiredAccount has policies of every reader U5 adds, many regions and a
// slow answer for every call, so a stop has calls left to cancel. The IAM
// policies take four pages of five items.
func expiredAccount(fail map[string]testutil.Reply) *testutil.PolicyAccount {
	doc := policyDoc(conditionStatement("S", "StringEquals", "aws:ResourceTag/env", `"dev"`))
	a := &testutil.PolicyAccount{Fail: fail, DetailsPageItems: 5}
	for i := 0; i < 20; i++ {
		name := string(rune('a'+i)) + "x"
		a.Roles = append(a.Roles, testutil.Principal{Name: name, Inline: map[string]string{"p": doc}})
		for _, region := range allRegions {
			a.Queues = append(a.Queues, testutil.RegionalPolicy{Region: region, Name: name, Policy: doc})
		}
	}
	return a
}

func expiredReply(service string) testutil.Reply {
	switch service {
	case "sqs":
		return testutil.SQSError(http.StatusBadRequest, "ExpiredToken", "ExpiredToken", "token "+testutil.FakeSessionToken+" expired")
	case "kms", "secretsmanager":
		return testutil.JSONError(http.StatusBadRequest, "ExpiredTokenException", "token "+testutil.FakeSessionToken+" expired")
	}
	return testutil.QueryError(http.StatusForbidden, "ExpiredToken", "token "+testutil.FakeSessionToken+" expired")
}

// U5 security design §4 (1): expired credentials in the read of the IAM
// policies (on its first or a later page) or in a resource policy read of
// one region stop the whole plan.
func TestExpiredCredentialsInTheU5ReadsStopPlan(t *testing.T) {
	for name, fail := range map[string]map[string]testutil.Reply{
		"IAM policies, first page":  {testutil.OpGetAccountAuthorizationDetails: expiredReply("iam")},
		"IAM policies, third page":  {testutil.OpGetAccountAuthorizationDetails + " page-10": expiredReply("iam")},
		"queue policy of a region":  {testutil.OpGetQueueAttributes + " " + testutil.QueueARN("eu-west-1", "bx"): expiredReply("sqs")},
		"key list of a region":      {testutil.OpListKeys + " sa-east-1": expiredReply("kms")},
		"bucket list":               {testutil.OpListBuckets: testutil.S3Error(http.StatusBadRequest, "ExpiredToken", "expired")},
		"secret policy everywhere":  {testutil.OpListSecrets: expiredReply("secretsmanager")},
		"topics list of the region": {testutil.OpListTopics + " us-east-1": expiredReply("sns")},
	} {
		t.Run(name, func(t *testing.T) {
			w := newAccountWorld(t)
			w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}}}})
			w.fake.HandlePolicyAccount(expiredAccount(fail))
			w.fake.SetDelay(func(testutil.Call) time.Duration { return 2 * time.Millisecond })
			rules := w.givenRuleFile(aliasRules)

			r := w.run("plan", "--rules", rules)

			requireStoppedForCredentials(t, w, r)
			if r.stdout != "" {
				t.Fatalf("a partial result was printed:\n%s", r.stdout)
			}
		})
	}
}

// U5 security design §4 (2): two reads that meet expired credentials at the
// same time give one message.
func TestTwoCredentialErrorsAtOnceGiveOneMessage(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}}}})
	w.fake.HandlePolicyAccount(expiredAccount(map[string]testutil.Reply{
		testutil.OpGetAccountAuthorizationDetails: expiredReply("iam"),
		testutil.OpListKeys:                       testutil.JSONError(http.StatusBadRequest, "UnrecognizedClientException", "rejected"),
	}))
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules)

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	if n := strings.Count(r.stderr, "Error:"); n != 1 {
		t.Fatalf("%d error messages, want one:\n%s", n, r.stderr)
	}
	// Which of the two comes first may change from run to run; both are
	// about the credentials.
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: (the AWS credentials have expired|AWS rejected the credentials)`)
}

// NFR8.20: a resource policy of 1 MiB + 1 byte is unread and plan exits 1
// with the other findings; one of exactly 1 MiB is read.
func TestPolicyDocumentsOverOneMiBAreUnread(t *testing.T) {
	statement := conditionStatement("Big", "StringEquals", "aws:ResourceTag/env", `"dev"`)
	pad := func(size int) string {
		base := `{"Version":"2012-10-17","Statement":[` + statement + `],"Pad":""}`
		return base[:len(base)-2] + strings.Repeat("a", size-len(base)) + `"}`
	}
	for name, sc := range map[string]struct {
		size int
		exit int
	}{"exactly 1 MiB": {1 << 20, 2}, "1 MiB + 1 byte": {1<<20 + 1, 1}} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			queue := testutil.QueueARN("us-east-1", "big")
			w.givenResources(testutil.TaggedResource{ARN: queue, Tags: map[string]string{"env": "dev"}})
			w.fake.HandlePolicyAccount(&testutil.PolicyAccount{
				Managed: []testutil.ManagedPolicyDoc{{Name: "DevAccess", Document: policyEnvCondition}},
				Queues:  []testutil.RegionalPolicy{{Region: "us-east-1", Name: "big", Policy: pad(sc.size)}},
			})
			rules := w.givenRuleFile(aliasRules)

			r, v := w.planImpact("--rules", rules, "--region", "us-east-1")

			r.requireExit(t, sc.exit)
			v.requireFinding(t, testutil.ManagedPolicyARN("DevAccess"), "aws:ResourceTag/env", "affected")
			_, read := v.finding(queue, "aws:ResourceTag/env", "affected")
			unread := []unreadPolicy{{SourceKind: "sqsQueue", PolicyRef: queue, Region: "us-east-1", Reason: "the policy document is larger than 1 MiB"}}
			if sc.exit == 1 && (read || !slices.Equal(v.Details.UnreadPolicies, unread)) {
				t.Fatalf("unread = %+v", v.Details.UnreadPolicies)
			}
			if sc.exit == 2 && (!read || len(v.Details.UnreadPolicies) != 0) {
				t.Fatalf("a document of exactly 1 MiB must be read: %+v", v.Details.UnreadPolicies)
			}
		})
	}
}

// NFR8.22: --debug names the calls but never shows a document, and the
// reason of an unread policy never shows AWS's error text.
func TestDebugAndReasonsNeverShowDocumentsOrAWSText(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	marker := "DOCUMENT-MARKER-0123"
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{
		Roles: []testutil.Principal{{Name: "app", Inline: map[string]string{"p": policyDoc(conditionStatement(marker, "StringEquals", "aws:ResourceTag/x", `"`+marker+`"`))}}},
		Queues: []testutil.RegionalPolicy{{Region: "us-east-1", Name: "qa",
			Policy: policyDoc(conditionStatement("S", "StringEquals", "aws:ResourceTag/env", `"`+marker+`"`))}},
		Fail: map[string]testutil.Reply{testutil.OpListTopics: testutil.QueryError(http.StatusForbidden, "AuthorizationError", "ERROR-TEXT-FROM-AWS "+marker)},
	})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1", "--debug")

	r.requireExit(t, 1)
	for _, op := range []string{"iam:GetAccountAuthorizationDetails", "sqs:GetQueueAttributes", "sns:ListTopics", "s3:ListBuckets", "kms:ListKeys", "secretsmanager:ListSecrets"} {
		requireMatch(t, "stderr", r.stderr, `(?m)^debug: `+op+` us-east-1 \d+ms retries=\d+`)
	}
	requireNotContains(t, "stderr", r.stderr, marker)
	requireNotContains(t, "stdout", r.stdout, "ERROR-TEXT-FROM-AWS")
	requireMatch(t, "stdout", r.stdout, `(?m)^    SNS topic policies in us-east-1: access denied to sns:ListTopics$`)
}
