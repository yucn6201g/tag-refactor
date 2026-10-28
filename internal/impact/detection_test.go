package impact

import (
	"context"
	"fmt"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// The detection table of NFR2.1: every kind of policy (9: managed, the
// inline policies of a user, a role and a group, and the five resource
// policies) with every kind of tag reference (7) and every kind of change
// (4) gives the expected verdict, and the condition operators (8) give the
// expected verdict for the four changes. A combination that should be
// affected or undecided and comes out as "no impact" fails the test.

// policyKinds are the nine kinds of policies of the table.
var policyKinds = []struct {
	name string
	// put adds a policy with doc to the account and returns the ARN of the
	// resource the policy belongs to ("" for IAM policies).
	put func(a *accountReader, doc string) string
}{
	{"iamManaged", func(a *accountReader, doc string) string {
		a.managed = map[string]string{"arn:aws:iam::123456789012:policy/p": doc}
		return ""
	}},
	{"iamInline user", func(a *accountReader, doc string) string {
		a.users = map[string]map[string]string{"u": {"p": doc}}
		return ""
	}},
	{"iamInline role", func(a *accountReader, doc string) string {
		a.roles = map[string]map[string]string{"r": {"p": doc}}
		return ""
	}},
	{"iamInline group", func(a *accountReader, doc string) string {
		a.groups = map[string]map[string]string{"g": {"p": doc}}
		return ""
	}},
	{"s3Bucket", func(a *accountReader, doc string) string {
		a.buckets = map[string]map[string]string{testRegion: {"arn:aws:s3:::b": doc}}
		return "arn:aws:s3:::b"
	}},
	{"kmsKey", func(a *accountReader, doc string) string {
		arn := "arn:aws:kms:" + testRegion + ":123456789012:key/k"
		a.keys = map[string]map[string]string{testRegion: {arn: doc}}
		return arn
	}},
	{"sqsQueue", func(a *accountReader, doc string) string {
		arn := "arn:aws:sqs:" + testRegion + ":123456789012:q"
		a.queues = map[string]map[string]string{testRegion: {arn: doc}}
		return arn
	}},
	{"snsTopic", func(a *accountReader, doc string) string {
		arn := "arn:aws:sns:" + testRegion + ":123456789012:t"
		a.topics = map[string]map[string]string{testRegion: {arn: doc}}
		return arn
	}},
	{"secretsManagerSecret", func(a *accountReader, doc string) string {
		arn := "arn:aws:secretsmanager:" + testRegion + ":123456789012:secret:s"
		a.secrets = map[string]map[string]string{testRegion: {arn: doc}}
		return arn
	}},
}

const (
	testRegion = "us-east-1"
	// roleARN is an IAM role whose tag changes PrincipalTag references see.
	roleARN = "arn:aws:iam::123456789012:role/app"
	// otherARN is a resource that no resource policy of the table belongs
	// to; IAM policies see its changes.
	otherARN = "arn:aws:sqs:us-east-1:123456789012:other"
)

// tagReferences are the seven kinds of reference, each as one statement
// with the tag key env and the value v1. principal is true for references
// that see the tags of IAM users and roles (BR3.3).
var tagReferences = []struct {
	name      string
	statement string
	principal bool
}{
	{"aws:ResourceTag", condStatement("StringEquals", "aws:ResourceTag/env", `"v1"`), false},
	{"service ResourceTag", condStatement("StringEquals", "ec2:ResourceTag/env", `"v1"`), false},
	{"aws:RequestTag", condStatement("StringEquals", "aws:RequestTag/env", `"v1"`), false},
	{"aws:TagKeys", condStatement("StringEquals", "aws:TagKeys", `["env"]`), false},
	{"aws:PrincipalTag", condStatement("StringEquals", "aws:PrincipalTag/env", `"v1"`), true},
	{"variable in the condition key", condStatement("StringEquals", "aws:ResourceTag/${aws:PrincipalTag/env}", `"v1"`), true},
	{"variable in Resource", `{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::data/${aws:PrincipalTag/env}/*"}`, true},
}

func condStatement(operator, key, value string) string {
	return fmt.Sprintf(`{"Sid":"S","Effect":"Allow","Action":"*","Resource":"*","Condition":{%q:{%q:%s}}}`, operator, key, value)
}

// tableChanges are the four kinds of change of the tag env (value v1) on
// a resource.
func tableChange(kind planner.ChangeKind, arn string) planner.TagChangeInput {
	c := planner.TagChangeInput{ChangeID: "c-000001", ResourceARN: arn, Kind: kind}
	switch kind {
	case planner.KindRenameKey:
		c.FromKey, c.ToKey, c.FromValue, c.ToValue = "env", "Environment", "v1", "v1"
	case planner.KindChangeValue:
		c.FromKey, c.ToKey, c.FromValue, c.ToValue = "env", "env", "v1", "v2"
	case planner.KindAddRequired:
		c.ToKey, c.ToValue = "env", "v1"
	case planner.KindRemoveKey:
		c.FromKey, c.FromValue = "env", "v1"
	}
	return c
}

var tableChangeKinds = []planner.ChangeKind{planner.KindRenameKey, planner.KindChangeValue, planner.KindAddRequired, planner.KindRemoveKey}

// expectedByReference is the expected verdict of each reference (rows) for
// rename, value change, add and remove (columns): A affected, U undecided, -
// no impact. A variable in the condition key is affected through the
// variable (BR4.2) even though the key itself cannot be decided (BR4.6);
// TagKeys does not see a value change (BR4.5).
var expectedByReference = map[string][4]string{
	"aws:ResourceTag":               {"A", "A", "A", "A"},
	"service ResourceTag":           {"A", "A", "A", "A"},
	"aws:RequestTag":                {"A", "A", "A", "A"},
	"aws:TagKeys":                   {"A", "-", "A", "A"},
	"aws:PrincipalTag":              {"A", "A", "A", "A"},
	"variable in the condition key": {"A", "U", "A", "A"},
	"variable in Resource":          {"A", "U", "A", "A"},
}

type detectionRow struct {
	kind, reference string
	change          planner.ChangeKind
	want            string
}

func detectionRows() []detectionRow {
	var rows []detectionRow
	for _, k := range policyKinds {
		for _, r := range tagReferences {
			for i, c := range tableChangeKinds {
				rows = append(rows, detectionRow{kind: k.name, reference: r.name, change: c, want: expectedByReference[r.name][i]})
			}
		}
	}
	return rows
}

// strongest returns the verdict of the change over the findings: A when an
// affected finding names it, U when only undecided ones do, - otherwise.
func strongest(res planner.ImpactResult, changeID string) string {
	out := "-"
	for _, f := range res.Findings {
		for _, id := range f.ChangeIDs {
			if id != changeID {
				continue
			}
			if f.Verdict == planner.VerdictAffected {
				return "A"
			}
			out = "U"
		}
	}
	return out
}

func runAnalysis(t testing.TB, a *accountReader, changes ...planner.TagChangeInput) planner.ImpactResult {
	t.Helper()
	res, err := New(a, []string{testRegion}).Analyze(context.Background(), changes)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Complete {
		t.Fatalf("unread policies: %+v", res.UnreadPolicies)
	}
	return res
}

func TestDetectionTableCoversEveryCombination(t *testing.T) {
	rows := detectionRows()
	if len(rows) != 252 || len(policyKinds)*len(tagReferences)*len(tableChangeKinds) != 252 {
		t.Fatalf("the table has %d rows, want 9 x 7 x 4 = 252", len(rows))
	}
	seen := map[detectionRow]bool{}
	for _, r := range rows {
		key := detectionRow{kind: r.kind, reference: r.reference, change: r.change}
		if seen[key] || r.want == "" {
			t.Fatalf("row %+v repeats or has no expectation", r)
		}
		seen[key] = true
	}
	if len(operatorRows) != 32 {
		t.Fatalf("the operator table has %d rows, want 8 x 4 = 32", len(operatorRows))
	}
}

// NFR2.1, AC3.2.6: every row of the table gives its verdict, so nothing that
// may be affected is shown as having no impact.
func TestDetectionTable(t *testing.T) {
	for _, row := range detectionRows() {
		var kind func(*accountReader, string) string
		for _, k := range policyKinds {
			if k.name == row.kind {
				kind = k.put
			}
		}
		var statement string
		var principal bool
		for _, r := range tagReferences {
			if r.name == row.reference {
				statement, principal = r.statement, r.principal
			}
		}
		a := &accountReader{}
		own := kind(a, `{"Version":"2012-10-17","Statement":[`+statement+`]}`)
		target := own
		switch {
		case principal:
			target = roleARN
		case own == "":
			target = otherARN
		}
		res := runAnalysis(t, a, tableChange(row.change, target))
		if got := strongest(res, "c-000001"); got != row.want {
			t.Errorf("%s / %s / %s: got %s, want %s (%+v)", row.kind, row.reference, row.change, got, row.want, res.Findings)
		}
	}
}

// operatorRows: aws:ResourceTag/env with each kind of operator, against a
// rename env -> Environment, a value change Prod -> prod, an added env =
// staging and a removal (BR2.3, BR4.2 to BR4.4, BR4.7).
var operatorRows = []struct {
	operator, value string
	change          planner.ChangeKind
	want            string
}{
	{"StringEquals", `"prod"`, planner.KindRenameKey, "A"},
	{"StringEquals", `"prod"`, planner.KindChangeValue, "A"},
	{"StringEquals", `"prod"`, planner.KindAddRequired, "-"},
	{"StringEquals", `"prod"`, planner.KindRemoveKey, "A"},
	{"StringNotEquals", `"prod"`, planner.KindRenameKey, "A"},
	{"StringNotEquals", `"prod"`, planner.KindChangeValue, "A"},
	{"StringNotEquals", `"prod"`, planner.KindAddRequired, "-"},
	{"StringNotEquals", `"prod"`, planner.KindRemoveKey, "A"},
	{"StringEqualsIgnoreCase", `"PROD"`, planner.KindRenameKey, "A"},
	{"StringEqualsIgnoreCase", `"PROD"`, planner.KindChangeValue, "A"},
	{"StringEqualsIgnoreCase", `"PROD"`, planner.KindAddRequired, "-"},
	{"StringEqualsIgnoreCase", `"PROD"`, planner.KindRemoveKey, "A"},
	{"StringLike", `"st*"`, planner.KindRenameKey, "A"},
	{"StringLike", `"st*"`, planner.KindChangeValue, "-"},
	{"StringLike", `"st*"`, planner.KindAddRequired, "A"},
	{"StringLike", `"st*"`, planner.KindRemoveKey, "A"},
	{"Null", `"true"`, planner.KindRenameKey, "A"},
	{"Null", `"true"`, planner.KindChangeValue, "-"},
	{"Null", `"true"`, planner.KindAddRequired, "A"},
	{"Null", `"true"`, planner.KindRemoveKey, "A"},
	{"StringEqualsIfExists", `"prod"`, planner.KindRenameKey, "A"},
	{"StringEqualsIfExists", `"prod"`, planner.KindChangeValue, "A"},
	{"StringEqualsIfExists", `"prod"`, planner.KindAddRequired, "A"},
	{"StringEqualsIfExists", `"prod"`, planner.KindRemoveKey, "A"},
	{"ForAnyValue:StringEquals", `"dev"`, planner.KindRenameKey, "A"},
	{"ForAnyValue:StringEquals", `"dev"`, planner.KindChangeValue, "-"},
	{"ForAnyValue:StringEquals", `"dev"`, planner.KindAddRequired, "A"},
	{"ForAnyValue:StringEquals", `"dev"`, planner.KindRemoveKey, "A"},
	{"ArnLike", `"arn:*"`, planner.KindRenameKey, "A"},
	{"ArnLike", `"arn:*"`, planner.KindChangeValue, "U"},
	{"ArnLike", `"arn:*"`, planner.KindAddRequired, "U"},
	{"ArnLike", `"arn:*"`, planner.KindRemoveKey, "A"},
}

func operatorChange(kind planner.ChangeKind) planner.TagChangeInput {
	c := planner.TagChangeInput{ChangeID: "c-000001", ResourceARN: otherARN, Kind: kind}
	switch kind {
	case planner.KindRenameKey:
		c.FromKey, c.ToKey, c.FromValue, c.ToValue = "env", "Environment", "Prod", "Prod"
	case planner.KindChangeValue:
		c.FromKey, c.ToKey, c.FromValue, c.ToValue = "env", "env", "Prod", "prod"
	case planner.KindAddRequired:
		c.ToKey, c.ToValue = "env", "staging"
	case planner.KindRemoveKey:
		c.FromKey, c.FromValue = "env", "Prod"
	}
	return c
}

func TestDetectionTableOfOperators(t *testing.T) {
	for _, row := range operatorRows {
		a := &accountReader{managed: map[string]string{
			"arn:aws:iam::123456789012:policy/p": `{"Statement":[` + condStatement(row.operator, "aws:ResourceTag/env", row.value) + `]}`,
		}}
		res := runAnalysis(t, a, operatorChange(row.change))
		if got := strongest(res, "c-000001"); got != row.want {
			t.Errorf("%s %s / %s: got %s, want %s", row.operator, row.value, row.change, got, row.want)
		}
	}
}
