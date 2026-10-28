package impact

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// memoryPolicies replaces the PolicyReader of contract C6 with in-memory
// attached customer managed policies (ARN to JSON) and nothing else (U1).
// readErr makes the read of the IAM policies fail; missing gives single
// policies without a document.
type memoryPolicies struct {
	emptyReader
	docs    map[string]string
	readErr error
	missing map[string]bool
	reads   int
}

func (m *memoryPolicies) ReadIAMPolicies(context.Context) (awsaccess.IAMPolicies, error) {
	m.reads++
	if m.readErr != nil {
		return awsaccess.IAMPolicies{}, m.readErr
	}
	var out awsaccess.IAMPolicies
	for arn, doc := range m.docs {
		p := awsaccess.ManagedPolicy{ARN: arn, AttachmentCount: 1, Document: url.PathEscape(doc), DocumentMissing: m.missing[arn]}
		if p.DocumentMissing {
			p.Document = ""
		}
		out.Managed = append(out.Managed, p)
	}
	return out, nil
}

func rename(id, from, to string) planner.TagChangeInput {
	return planner.TagChangeInput{ChangeID: id, ResourceARN: "arn:r", Kind: planner.KindRenameKey, FromKey: from, ToKey: to, FromValue: "v", ToValue: "v"}
}

// policyWith returns a one-statement policy with one condition.
func policyWith(operator, key, value string) string {
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*","Condition":{%q:{%q:%q}}}]}`, operator, key, value)
}

func analyze(t *testing.T, docs map[string]string, changes ...planner.TagChangeInput) planner.ImpactResult {
	t.Helper()
	res, err := New(&memoryPolicies{docs: docs}, nil).Analyze(context.Background(), changes)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func verdicts(res planner.ImpactResult) string {
	var parts []string
	for _, f := range res.Findings {
		parts = append(parts, fmt.Sprintf("%s %s %s %s", f.Verdict, f.PolicyRef, f.ConditionKey, strings.Join(f.ChangeIDs, ",")))
	}
	return strings.Join(parts, "\n")
}

func TestResourceTagKeysOfAnyServicePrefixAreMatched(t *testing.T) {
	docs := map[string]string{
		"p1": policyWith("StringEquals", "aws:ResourceTag/env", "dev"),
		"p2": policyWith("StringEquals", "ec2:ResourceTag/env", "dev"),
		"p3": policyWith("StringEquals", "AWS:resourcetag/ENV", "dev"),
		"p4": policyWith("StringEquals", "aws:RequestTag/env", "dev"),
		"p5": policyWith("ForAnyValue:StringEquals", "aws:TagKeys", "env"),
		"p6": policyWith("StringEquals", "bad prefix:ResourceTag/env", "dev"),
		"p7": policyWith("StringEquals", "aws:ResourceTag/", "dev"),
		"p8": policyWith("StringEquals", "nocolon", "dev"),
	}
	res := analyze(t, docs, rename("c-000001", "env", "Environment"))

	// Since U5 the RequestTag and TagKeys keys are matched too (BR2.1).
	want := "affected p1 aws:ResourceTag/env c-000001\naffected p2 ec2:ResourceTag/env c-000001\naffected p3 AWS:resourcetag/ENV c-000001\n" +
		"affected p4 aws:RequestTag/env c-000001\naffected p5 aws:TagKeys c-000001"
	if got := verdicts(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
	if !res.Complete || len(res.UnreadPolicies) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestCaseOnlyRenamesAreIgnoredOnlyForTheAWSPrefix(t *testing.T) {
	docs := map[string]string{
		"global":  policyWith("StringEquals", "aws:ResourceTag/env", "dev"),
		"service": policyWith("StringEquals", "s3:ResourceTag/env", "dev"),
	}
	res := analyze(t, docs, rename("c-000001", "env", "Env"), rename("c-000002", "Env", "Environment"))

	want := "affected global aws:ResourceTag/env c-000002\naffected service s3:ResourceTag/env c-000001,c-000002"
	if got := verdicts(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
}

func TestPolicyVariablesCannotBeDecided(t *testing.T) {
	docs := map[string]string{"p": policyWith("StringEquals", "aws:ResourceTag/${aws:PrincipalTag/team}", "yes")}
	res := analyze(t, docs, rename("c-000001", "env", "Env"), rename("c-000002", "team", "Team-Name"))

	if got := verdicts(res); got != "undecided p aws:ResourceTag/${aws:PrincipalTag/team} c-000002" {
		t.Fatalf("findings: %s", got)
	}
}

func TestWildcardsMatchAsPatternsWithoutCase(t *testing.T) {
	docs := map[string]string{
		"star":     policyWith("StringLike", "aws:ResourceTag/ENV*", "x"),
		"question": policyWith("StringLike", "aws:ResourceTag/c?st-center", "x"),
		"none":     policyWith("StringLike", "aws:ResourceTag/owner*", "x"),
		"literal":  policyWith("StringLike", "aws:ResourceTag/cost.center*", "x"),
	}
	res := analyze(t, docs, rename("c-000001", "env", "Environment"), rename("c-000002", "cost-center", "CostCenter"))

	want := "affected question aws:ResourceTag/c?st-center c-000002\naffected star aws:ResourceTag/ENV* c-000001"
	if got := verdicts(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
}

func TestPathologicalWildcardsStayFast(t *testing.T) {
	var changes []planner.TagChangeInput
	for i := 0; i < 1000; i++ {
		changes = append(changes, rename(fmt.Sprintf("c-%06d", i+1), strings.Repeat("a", 60)+fmt.Sprint(i), "Key"+fmt.Sprint(i)))
	}
	docs := map[string]string{}
	for i := 0; i < 50; i++ {
		docs[fmt.Sprintf("p%02d", i)] = policyWith("StringLike", "aws:ResourceTag/a*a*a*a*a*a*a*a*a*a*b", "x")
	}
	start := time.Now()
	res := analyze(t, docs, changes...)
	if elapsed := time.Since(start); elapsed > timeLimitFactor*5*time.Second {
		t.Fatalf("matching took %v", elapsed)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("unexpected findings: %d", len(res.Findings))
	}
}

func TestUnreadablePoliciesMakeTheResultIncomplete(t *testing.T) {
	docs := map[string]string{
		"p-missing":  policyWith("StringEquals", "aws:ResourceTag/env", "dev"),
		"p-broken":   `{"Statement": "not a statement"}`,
		"p-readable": policyWith("StringEquals", "aws:ResourceTag/env", "dev"),
	}
	reader := &memoryPolicies{docs: docs, missing: map[string]bool{"p-missing": true}}
	res, err := New(reader, nil).Analyze(context.Background(), []planner.TagChangeInput{rename("c-000001", "env", "Environment")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete || len(res.UnreadPolicies) != 2 || len(res.Findings) != 1 || res.Findings[0].PolicyRef != "p-readable" {
		t.Fatalf("result = %+v", res)
	}
	if u := res.UnreadPolicies[0]; u.PolicyRef != "p-broken" || !strings.Contains(u.Reason, "could not be parsed") {
		t.Fatalf("unread = %+v", u)
	}
	if u := res.UnreadPolicies[1]; u.PolicyRef != "p-missing" || u.Reason != reasonNoDefaultDocument || u.SourceKind != planner.SourceIAMManaged {
		t.Fatalf("unread = %+v", u)
	}

	// A read that fails leaves both IAM kinds unread as whole lists, and no
	// policy of the account is matched.
	readDenied := &memoryPolicies{docs: docs, readErr: &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetAccountAuthorizationDetails}}
	res, err = New(readDenied, nil).Analyze(context.Background(), []planner.TagChangeInput{rename("c-000001", "env", "E")})
	want := []planner.UnreadPolicy{
		{SourceKind: planner.SourceIAMManaged, Reason: "access denied to iam:GetAccountAuthorizationDetails"},
		{SourceKind: planner.SourceIAMInline, Reason: "access denied to iam:GetAccountAuthorizationDetails"},
	}
	if err != nil || res.Complete || len(res.Findings) != 0 || !slices.Equal(res.UnreadPolicies, want) {
		t.Fatalf("result = %+v, %v", res, err)
	}
}

func TestUnreadReasonsNameTheKindAndOperationOnly(t *testing.T) {
	cases := map[string]error{
		"credentials were rejected for iam:GetAccountAuthorizationDetails":          &errkind.Error{Kind: errkind.CredentialsMissing},
		"AWS did not answer iam:GetAccountAuthorizationDetails (temporary failure)": &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpGetAccountAuthorizationDetails},
		"unexpected error (Internal) in iam:GetAccountAuthorizationDetails":         fmt.Errorf("secret text AKIAFAKE"),
		"unexpected error (InvalidInput) in kms:GetKeyPolicy":                       &errkind.Error{Kind: errkind.InvalidInput, Op: awsaccess.OpGetKeyPolicy},
	}
	for want, err := range cases {
		if got := unreadReason(err, awsaccess.OpGetAccountAuthorizationDetails); got != want {
			t.Errorf("unreadReason = %q, want %q", got, want)
		}
	}
}

func TestFindingsAreGroupedPerConditionAndOrdered(t *testing.T) {
	doc := `{"Statement":{"Sid":"One","Condition":{` +
		`"StringEquals":{"aws:ResourceTag/env":["dev","prod"],"aws:ResourceTag/cost-center":"a"},` +
		`"Bool":{"aws:ResourceTag/env":true},"Null":{"aws:ResourceTag/env":null}}}}`
	twoStatements := `{"Statement":[{"Effect":"Deny"},{"Sid":"","Condition":{"NumericLessThan":{"aws:ResourceTag/env":5}}}]}`
	res := analyze(t, map[string]string{"b-policy": doc, "a-policy": twoStatements},
		rename("c-000001", "env", "Environment"), rename("c-000002", "cost-center", "CostCenter"), rename("c-000003", "Env", "Environment"))

	var got []string
	for _, f := range res.Findings {
		got = append(got, fmt.Sprintf("%s|%s|%s|%s|%s|%s", f.PolicyRef, f.StatementID, f.ConditionOperator, f.ConditionKey, f.ConditionValue, strings.Join(f.ChangeIDs, ",")))
	}
	want := []string{
		"a-policy|#1|NumericLessThan|aws:ResourceTag/env|5|c-000001,c-000003",
		"b-policy|One|Bool|aws:ResourceTag/env|true|c-000001,c-000003",
		"b-policy|One|Null|aws:ResourceTag/env||c-000001,c-000003",
		"b-policy|One|StringEquals|aws:ResourceTag/cost-center|a|c-000002",
		"b-policy|One|StringEquals|aws:ResourceTag/env|dev, prod|c-000001,c-000003",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParsePolicyRejectsDocumentsItCannotRead(t *testing.T) {
	for _, doc := range []string{
		`not json`,
		`{"Statement":"x"}`,
		`{"Statement":[{"Sid":5}]}`,
		`{"Statement":[{"Condition":["x"]}]}`,
		`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/env":{"nested":1}}}}]}`,
		`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/env":[1,}}}]}`,
	} {
		if _, err := parseDocument(doc, true); err == nil {
			t.Errorf("parseDocument(%s) succeeded", doc)
		}
	}
	statements, err := parseDocument(`{"Version":"2012-10-17"}`, true)
	if err != nil || len(statements) != 0 {
		t.Fatalf("a policy without statements: %v %v", statements, err)
	}
	statements, err = parseDocument(`{"Statement":[{"Condition":{"StringEquals":{"aws:ResourceTag/a%b":"50%"}}}]}`, true)
	if err != nil || len(statements) != 1 || len(statements[0].references()) != 1 || statements[0].references()[0].tagKey != "a%b" {
		t.Fatalf("a document that is not URL-encoded is read as is: %+v %v", statements, err)
	}
}

func TestNoChangesReadNoPoliciesAndCancellationStops(t *testing.T) {
	reader := &memoryPolicies{docs: map[string]string{"p": policyWith("StringEquals", "aws:ResourceTag/env", "x")}}
	res, err := New(reader, nil).Analyze(context.Background(), nil)
	if err != nil || !res.Complete || reader.reads != 0 {
		t.Fatalf("result = %+v, reads = %d, err = %v", res, reader.reads, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(reader, nil).Analyze(ctx, []planner.TagChangeInput{rename("c-000001", "env", "E")}); err == nil {
		t.Fatal("a cancelled context must stop the analysis")
	}
	readFails := &memoryPolicies{readErr: context.Canceled}
	if _, err := New(readFails, nil).Analyze(ctx, []planner.TagChangeInput{rename("c-000001", "env", "E")}); err == nil {
		t.Fatal("a cancelled read must stop the analysis")
	}
}
