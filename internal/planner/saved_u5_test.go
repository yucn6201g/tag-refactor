package planner

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Unit tests of the fields U5 adds to the saved plan (contract C2):
// impactFindings[].conditionOperator, unreadPolicies[].policyRef and
// unreadPolicies[].region. They are optional, and version stays 1.

// u5Plan is samplePlan with a Resource finding and unread policies of every
// shape: a whole list, a list in a region, and one policy in a region.
func u5Plan(t testing.TB) *Plan {
	t.Helper()
	p := samplePlan(t)
	p.ImpactFindings = append(p.ImpactFindings, ImpactFinding{Verdict: VerdictUndecided, PolicyRef: "arn:aws:iam::123456789012:role/r#inline",
		SourceKind: SourceIAMInline, StatementID: "#1", ConditionKey: "Resource", ConditionValue: "arn:a, arn:${aws:PrincipalTag/t}",
		ChangeIDs: []string{"c-000002"}})
	p.UnreadPolicies = []UnreadPolicy{
		{SourceKind: SourceIAMInline, Reason: "access denied to iam:GetAccountAuthorizationDetails"},
		{SourceKind: SourceSQSQueue, Region: "eu-west-1", Reason: "access denied to sqs:ListQueues"},
		{SourceKind: SourceKMSKey, PolicyRef: "arn:aws:kms:eu-west-1:123456789012:key/k", Region: "eu-west-1", Reason: "access denied to kms:GetKeyPolicy"},
	}
	return p
}

// The operator is written as is, and empty for a Resource finding.
func TestSavedPlanKeepsTheConditionOperator(t *testing.T) {
	data := encode(t, u5Plan(t))
	if !bytes.Contains(data, []byte(`"statementId":"S","conditionOperator":"StringEquals","conditionKey":"aws:ResourceTag/env"`)) ||
		!bytes.Contains(data, []byte(`"statementId":"#1","conditionOperator":"","conditionKey":"Resource"`)) {
		t.Fatalf("the findings do not carry their operator:\n%s", data)
	}
	loaded, err := ParsePlan("plan.json", data)
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, f := range loaded.ImpactFindings {
		ops = append(ops, f.ConditionOperator)
	}
	if !slices.Equal(ops, []string{"StringEquals", "", ""}) {
		t.Fatalf("operators read back: %q", ops)
	}
}

// policyRef and region are written only when they are set, and read back.
func TestSavedPlanKeepsThePolicyRefAndRegionOfUnreadPolicies(t *testing.T) {
	p := u5Plan(t)
	data := encode(t, p)
	for _, want := range []string{
		`{"sourceKind":"iamInline","reason":"access denied to iam:GetAccountAuthorizationDetails"}`,
		`{"sourceKind":"sqsQueue","region":"eu-west-1","reason":"access denied to sqs:ListQueues"}`,
		`{"sourceKind":"kmsKey","policyRef":"arn:aws:kms:eu-west-1:123456789012:key/k","region":"eu-west-1","reason":"access denied to kms:GetKeyPolicy"}`,
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("the plan has no %s:\n%s", want, data)
		}
	}
	loaded, err := ParsePlan("plan.json", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(loaded.UnreadPolicies, p.UnreadPolicies) {
		t.Fatalf("unread read back: %+v", loaded.UnreadPolicies)
	}
	if !bytes.Equal(encode(t, loaded), data) {
		t.Fatal("saving the loaded plan again must give the same bytes")
	}
}

// A plan saved before U5, without the three fields, still reads, with
// version 1.
func TestAPlanSavedBeforeU5StillReads(t *testing.T) {
	data := encode(t, samplePlan(t))
	old := regexp.MustCompile(`"conditionOperator":"[^"]*",|"policyRef":"arn:x",`).ReplaceAll(data, nil)
	if bytes.Contains(old, []byte("conditionOperator")) || !bytes.Contains(old, []byte(`"version": 1`)) {
		t.Fatalf("the old form still has the U5 fields:\n%s", old)
	}
	loaded, err := ParsePlan("plan.json", old)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ImpactFindings[0].ConditionOperator != "" || loaded.UnreadPolicies[0].PolicyRef != "" ||
		loaded.UnreadPolicies[0].Reason != "the default version has no document in iam:GetAccountAuthorizationDetails" {
		t.Fatalf("loaded = %+v %+v", loaded.ImpactFindings[0], loaded.UnreadPolicies[0])
	}
}

// The details of plan --format json are written by WriteDetails too, so
// they carry the same fields (contract C3).
func TestDetailsCarryTheU5Fields(t *testing.T) {
	var b bytes.Buffer
	w := NewSafeJSONWriter(&b)
	o := NewJSONObject(w, "")
	WriteDetails(o, u5Plan(t))
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var details struct {
		ImpactFindings []map[string]any `json:"impactFindings"`
		UnreadPolicies []map[string]any `json:"unreadPolicies"`
	}
	if err := json.Unmarshal(b.Bytes(), &details); err != nil {
		t.Fatalf("%v\n%s", err, b.String())
	}
	if _, ok := details.ImpactFindings[2]["conditionOperator"]; !ok {
		t.Fatalf("finding = %v", details.ImpactFindings[2])
	}
	if details.UnreadPolicies[2]["policyRef"] != "arn:aws:kms:eu-west-1:123456789012:key/k" || details.UnreadPolicies[1]["region"] != "eu-west-1" {
		t.Fatalf("unread = %v", details.UnreadPolicies)
	}
	if _, ok := details.UnreadPolicies[0]["policyRef"]; ok || strings.Contains(b.String(), `"region":""`) {
		t.Fatalf("empty fields must be left out: %s", b.String())
	}
}
