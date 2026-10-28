package display

import (
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Unit tests of how the U5 impact analysis is shown: the kinds of policies,
// the unread policies with their region and reference, and why a finding
// cannot be decided, in English and Japanese, in the table and Markdown.

func impactPlan(findings []planner.ImpactFinding, unread []planner.UnreadPolicy) *planner.Plan {
	p := samplePlan()
	p.ImpactFindings = findings
	p.UnreadPolicies = unread
	p.ImpactAnalysisComplete = len(unread) == 0
	return p
}

// u5Unread has every form of unread policy. The IAM policies are read in one
// go, so the inline policies of one principal (a reference without #) are
// unread only in plans saved before that; such plans still load and show.
var u5Unread = []planner.UnreadPolicy{
	{SourceKind: planner.SourceIAMInline, Reason: "access denied to iam:GetAccountAuthorizationDetails"},
	{SourceKind: planner.SourceIAMInline, PolicyRef: "arn:aws:iam::123456789012:role/app", Reason: "access denied to iam:ListRolePolicies"},
	{SourceKind: planner.SourceIAMInline, PolicyRef: "arn:aws:iam::123456789012:role/app#p", Reason: "the policy has no document in iam:GetAccountAuthorizationDetails"},
	{SourceKind: planner.SourceSQSQueue, Region: "eu-west-1", Reason: "access denied to sqs:ListQueues"},
	{SourceKind: planner.SourceKMSKey, PolicyRef: "arn:aws:kms:eu-west-1:123456789012:key/k", Region: "eu-west-1", Reason: "the policy document is larger than 1 MiB"},
}

func TestUnreadPoliciesNameTheirKindRegionAndReference(t *testing.T) {
	for lang, want := range map[Lang]string{
		English: "! Impact analysis is incomplete\n" +
			"    IAM inline policies: access denied to iam:GetAccountAuthorizationDetails\n" +
			"    IAM inline policies of arn:aws:iam::123456789012:role/app: access denied to iam:ListRolePolicies\n" +
			"    IAM inline policy arn:aws:iam::123456789012:role/app#p: the policy has no document in iam:GetAccountAuthorizationDetails\n" +
			"    SQS queue policies in eu-west-1: access denied to sqs:ListQueues\n" +
			"    KMS key policy arn:aws:kms:eu-west-1:123456789012:key/k in eu-west-1: the policy document is larger than 1 MiB\n",
		Japanese: "! 影響分析が不完全です\n" +
			"    IAM インラインポリシー: access denied to iam:GetAccountAuthorizationDetails\n" +
			"    IAM インラインポリシーの一覧 arn:aws:iam::123456789012:role/app: access denied to iam:ListRolePolicies\n" +
			"    IAM インラインポリシー arn:aws:iam::123456789012:role/app#p: the policy has no document in iam:GetAccountAuthorizationDetails\n" +
			"    SQS キューポリシー(eu-west-1): access denied to sqs:ListQueues\n" +
			"    KMS キーポリシー arn:aws:kms:eu-west-1:123456789012:key/k(eu-west-1): the policy document is larger than 1 MiB\n",
	} {
		got := render(t, lang, func(p *Printer) { p.PlanResult(impactPlan(nil, u5Unread), Output{Format: FormatTable}) })
		if !strings.HasPrefix(got, want) {
			t.Errorf("%s:\n%s\nwant prefix:\n%s", lang, got, want)
		}
	}
}

func TestFindingsNameTheKindOfEveryPolicy(t *testing.T) {
	var findings []planner.ImpactFinding
	for _, kind := range planner.SourceKinds {
		findings = append(findings, planner.ImpactFinding{Verdict: planner.VerdictAffected, PolicyRef: "arn:" + kind, SourceKind: kind,
			StatementID: "S", ConditionOperator: "StringEquals", ConditionKey: "aws:ResourceTag/env", ConditionValue: "dev", ChangeIDs: []string{"c-000001"}})
	}
	en := render(t, English, func(p *Printer) { p.PlanResult(impactPlan(findings, nil), Output{Format: FormatTable}) })
	ja := render(t, Japanese, func(p *Printer) { p.PlanResult(impactPlan(findings, nil), Output{Format: FormatTable}) })
	for _, want := range []string{"! IAM policy  arn:iamManaged", "! IAM inline policy  arn:iamInline", "! S3 bucket policy  arn:s3Bucket",
		"! KMS key policy  arn:kmsKey", "! SQS queue policy  arn:sqsQueue", "! SNS topic policy  arn:snsTopic",
		"! Secrets Manager secret policy  arn:secretsManagerSecret"} {
		if !strings.Contains(en, want) {
			t.Errorf("English has no %q", want)
		}
	}
	for _, want := range []string{"! IAM ポリシー  arn:iamManaged", "! IAM インラインポリシー  arn:iamInline", "! S3 バケットポリシー  arn:s3Bucket",
		"! Secrets Manager シークレットのポリシー  arn:secretsManagerSecret"} {
		if !strings.Contains(ja, want) {
			t.Errorf("Japanese has no %q", want)
		}
	}
}

// An undecided finding without a policy variable is undecided because of
// its operator (a Null of TagKeys, an operator that does not compare
// strings); a Resource finding has no operator word.
func TestUndecidedFindingsSayWhy(t *testing.T) {
	findings := []planner.ImpactFinding{
		{Verdict: planner.VerdictUndecided, PolicyRef: "arn:p", SourceKind: planner.SourceIAMManaged, StatementID: "S",
			ConditionOperator: "ArnLike", ConditionKey: "aws:ResourceTag/env", ConditionValue: "arn:*", ChangeIDs: []string{"c-000001"}},
		{Verdict: planner.VerdictUndecided, PolicyRef: "arn:aws:sqs:us-east-1:123456789012:q", SourceKind: planner.SourceSQSQueue, StatementID: "#0",
			ConditionKey: "Resource", ConditionValue: "arn:${aws:PrincipalTag/team}", ChangeIDs: []string{"c-000001"}},
	}
	table := render(t, English, func(p *Printer) { p.PlanResult(impactPlan(findings, nil), Output{Format: FormatTable}) })
	for _, want := range []string{"      Statement \"S\"  Condition ArnLike aws:ResourceTag/env = \"arn:*\"\n      cannot decide (condition operator)\n",
		"  ? SQS queue policy  arn:aws:sqs:us-east-1:123456789012:q\n      Statement \"#0\"  Condition Resource = \"arn:${aws:PrincipalTag/team}\"\n      cannot decide (policy variable)\n"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table has no %q:\n%s", want, table)
		}
	}
	md := render(t, Japanese, func(p *Printer) { p.PlanResult(impactPlan(findings, nil), Output{Format: FormatMarkdown}) })
	if !strings.Contains(md, "| 判定できません(条件の演算子) |") || !strings.Contains(md, "| SQS キューポリシー ` arn:aws:sqs:us-east-1:123456789012:q ` |") {
		t.Errorf("Markdown:\n%s", md)
	}
}

// Markdown: the unread table has a region column; a kind outside contract
// C2, which a saved plan may hold, is a code span.
func TestMarkdownUnreadPoliciesHaveTheirRegion(t *testing.T) {
	unread := append(append([]planner.UnreadPolicy{}, u5Unread...), planner.UnreadPolicy{SourceKind: "## heading", Reason: "x"})
	md := render(t, English, func(p *Printer) { p.PlanResult(impactPlan(nil, unread), Output{Format: FormatMarkdown}) })
	for _, want := range []string{
		"| Policy | Region | Reason |\n|---|---|---|\n",
		"| (the list of IAM inline policies) |  | ` access denied to iam:GetAccountAuthorizationDetails ` |",
		"| IAM inline policies of ` arn:aws:iam::123456789012:role/app ` |  | ` access denied to iam:ListRolePolicies ` |",
		"| (the list of SQS queue policies) | ` eu-west-1 ` | ` access denied to sqs:ListQueues ` |",
		"| (the list of ` ## heading `) |  | ` x ` |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown has no %q:\n%s", want, md)
		}
	}
}

// NFR8.22: control characters and HTML in the data of a finding or an
// unread policy are shown in their visible form in the table, stay in code
// spans in Markdown, and are escaped in JSON.
func TestHostileImpactDataIsGuardedInEveryFormat(t *testing.T) {
	hostile := "a\x1b[2J</details>|`b\nc"
	findings := []planner.ImpactFinding{{Verdict: planner.VerdictAffected, PolicyRef: "arn:aws:iam::123456789012:role/r#" + hostile,
		SourceKind: planner.SourceIAMInline, StatementID: hostile, ConditionOperator: "StringEquals", ConditionKey: "aws:ResourceTag/" + hostile,
		ConditionValue: hostile, ChangeIDs: []string{"c-000001"}}}
	unread := []planner.UnreadPolicy{{SourceKind: planner.SourceS3Bucket, PolicyRef: "arn:aws:s3:::" + hostile, Region: hostile, Reason: "x"}}
	plan := impactPlan(findings, unread)
	for _, f := range []Format{FormatTable, FormatDiff, FormatJSON, FormatMarkdown} {
		out := render(t, English, func(p *Printer) { p.PlanResult(plan, Output{Format: f}) })
		if strings.Contains(out, "\x1b") || strings.Contains(out, "c\nc") || strings.Contains(out, "b\nc") {
			t.Errorf("%s writes the control characters:\n%s", f, out)
		}
		if f != FormatMarkdown && f != FormatJSON && !strings.Contains(out, `\u{1B}[2J`) && f != FormatDiff {
			t.Errorf("%s does not show the escape in its visible form:\n%s", f, out)
		}
		if f == FormatJSON && strings.Contains(out, "</details>") {
			t.Errorf("JSON writes < and > as they are:\n%s", out)
		}
		if f == FormatMarkdown {
			// Only the tool's own folded table of the changes by resource.
			d, c, h, cols := structure(out)
			plain := render(t, English, func(p *Printer) {
				p.PlanResult(impactPlan([]planner.ImpactFinding{{Verdict: planner.VerdictAffected, PolicyRef: "p", SourceKind: planner.SourceIAMInline,
					StatementID: "s", ConditionOperator: "StringEquals", ConditionKey: "k", ConditionValue: "v", ChangeIDs: []string{"c-000001"}}},
					[]planner.UnreadPolicy{{SourceKind: planner.SourceS3Bucket, PolicyRef: "p", Region: "r", Reason: "x"}}), Output{Format: f})
			})
			d0, c0, h0, cols0 := structure(plain)
			if d != d0 || c != c0 || h != h0 || !slices.Equal(cols, cols0) {
				t.Errorf("the data changes the Markdown: %d/%d/%d %v, plain %d/%d/%d %v:\n%s", d, c, h, cols, d0, c0, h0, cols0, out)
			}
		}
	}
}
