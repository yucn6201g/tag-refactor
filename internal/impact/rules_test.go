package impact

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

func change(id, arn string, kind planner.ChangeKind, fromKey, toKey, fromValue, toValue string) planner.TagChangeInput {
	return planner.TagChangeInput{ChangeID: id, ResourceARN: arn, Kind: kind, FromKey: fromKey, ToKey: toKey, FromValue: fromValue, ToValue: toValue}
}

func managed(docs ...string) *accountReader {
	a := &accountReader{managed: map[string]string{}}
	for i, d := range docs {
		a.managed[fmt.Sprintf("arn:aws:iam::123456789012:policy/p%d", i)] = d
	}
	return a
}

func doc(statements ...string) string {
	return `{"Version":"2012-10-17","Statement":[` + strings.Join(statements, ",") + `]}`
}

// summary renders findings as "verdict ref key ids" lines.
func summary(res planner.ImpactResult) string {
	var lines []string
	for _, f := range res.Findings {
		lines = append(lines, fmt.Sprintf("%s %s %s %s", f.Verdict, f.PolicyRef, f.ConditionKey, strings.Join(f.ChangeIDs, ",")))
	}
	return strings.Join(lines, "\n")
}

// --- Verdicts (BR4.1 to BR4.7) ---

// BR4.2: only a case-only rename under the aws:ResourceTag condition key
// cannot break the reference; service keys, RequestTag and variables are
// affected.
func TestCaseOnlyRenameIsHarmlessOnlyForTheGlobalResourceTagKey(t *testing.T) {
	a := managed(doc(
		condStatement("StringEquals", "aws:ResourceTag/env", `"x"`),
		condStatement("StringEquals", "ec2:ResourceTag/env", `"x"`),
		condStatement("StringEquals", "aws:RequestTag/env", `"x"`),
	))
	a.roles = map[string]map[string]string{"app": {"p": doc(`{"Effect":"Allow","Action":"*","Resource":"arn:${aws:PrincipalTag/env}"}`)}}
	res := runAnalysis(t, a, change("c-000001", roleARN, planner.KindRenameKey, "env", "Env", "v", "v"))
	want := "affected arn:aws:iam::123456789012:policy/p0 ec2:ResourceTag/env c-000001\n" +
		"affected arn:aws:iam::123456789012:policy/p0 aws:RequestTag/env c-000001\n" +
		"affected arn:aws:iam::123456789012:role/app#p Resource c-000001"
	if got := summary(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
}

// BR4.3: a value change compares the values; Null does not see it, a
// variable in the value cannot be decided, and a value that matches
// neither side has no impact.
func TestValueChangesCompareTheConditionValues(t *testing.T) {
	a := managed(doc(
		condStatement("StringEquals", "aws:ResourceTag/env", `"dev"`),
		condStatement("Null", "aws:ResourceTag/env", `"false"`),
		condStatement("StringEquals", "aws:ResourceTag/stage", `["${aws:PrincipalTag/stage}","x"]`),
		condStatement("StringLike", "aws:ResourceTag/tier", `"Gold*"`),
	))
	res := runAnalysis(t, a,
		change("c-000001", otherARN, planner.KindChangeValue, "env", "env", "Prod", "prod"),
		change("c-000002", otherARN, planner.KindChangeValue, "stage", "stage", "a", "b"),
		change("c-000003", otherARN, planner.KindChangeValue, "tier", "tier", "gold-1", "Gold-1"),
	)
	want := "undecided arn:aws:iam::123456789012:policy/p0 aws:ResourceTag/stage c-000002\n" +
		"affected arn:aws:iam::123456789012:policy/p0 aws:ResourceTag/tier c-000003"
	if got := summary(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
}

// BR4.4: adding a required tag is affected under IfExists, a set operator or
// Null whatever the value; otherwise only when the value added matches.
func TestAddingARequiredTagGoesFromMissingToTheValue(t *testing.T) {
	a := managed(doc(
		condStatement("StringEqualsIfExists", "aws:ResourceTag/env", `"dev"`),
		condStatement("StringEquals", "aws:ResourceTag/Owner", `"dev"`),
		condStatement("ForAllValues:StringEquals", "aws:ResourceTag/team", `"x"`),
		condStatement("Null", "aws:ResourceTag/cc", `"true"`),
		condStatement("StringNotEquals", "aws:ResourceTag/app", `"web"`),
	))
	res := runAnalysis(t, a,
		change("c-000001", otherARN, planner.KindAddRequired, "", "env", "", "prod"),
		change("c-000002", otherARN, planner.KindAddRequired, "", "Owner", "", "prod"),
		change("c-000003", otherARN, planner.KindAddRequired, "", "team", "", "y"),
		change("c-000004", otherARN, planner.KindAddRequired, "", "cc", "", "1"),
		change("c-000005", otherARN, planner.KindAddRequired, "", "app", "", "web"),
	)
	got := summary(res)
	for _, want := range []string{"aws:ResourceTag/env c-000001", "aws:ResourceTag/team c-000003", "aws:ResourceTag/cc c-000004", "aws:ResourceTag/app c-000005"} {
		if !strings.Contains(got, "affected arn:aws:iam::123456789012:policy/p0 "+want) {
			t.Errorf("missing %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Owner") {
		t.Errorf("Owner = prod does not match dev, so the result is the same before and after:\n%s", got)
	}
}

// BR4.5 and the NFR design's decision on Null: TagKeys compares the keys
// without case; a negated operator or ForAllValues sees every new key; Null
// cannot be decided; a value change has no impact.
func TestTagKeysCompareTheKeys(t *testing.T) {
	a := managed(
		doc(condStatement("ForAnyValue:StringNotEquals", "aws:TagKeys", `["env","owner"]`)),
		doc(condStatement("ForAnyValue:StringLike", "aws:TagKeys", `"ENV*"`)),
		doc(condStatement("Null", "aws:TagKeys", `"true"`)),
		doc(condStatement("ForAllValues:StringEquals", "aws:TagKeys", `["a"]`)),
	)
	res := runAnalysis(t, a,
		change("c-000001", otherARN, planner.KindRenameKey, "team", "Team", "v", "v"),
		change("c-000002", otherARN, planner.KindRemoveKey, "environment", "", "v", ""),
		change("c-000003", otherARN, planner.KindChangeValue, "env", "env", "a", "b"),
	)
	want := "affected arn:aws:iam::123456789012:policy/p0 aws:TagKeys c-000001\n" +
		"affected arn:aws:iam::123456789012:policy/p1 aws:TagKeys c-000002\n" +
		"undecided arn:aws:iam::123456789012:policy/p2 aws:TagKeys c-000001,c-000002\n" +
		"affected arn:aws:iam::123456789012:policy/p3 aws:TagKeys c-000001"
	if got := summary(res); got != want {
		t.Fatalf("findings:\n%s\nwant:\n%s", got, want)
	}
}

// BR2.3 and the NFR design: the negated operators that do not compare
// strings are "other" and cannot be decided; operator names ignore case.
func TestOperatorsAreClassifiedWithoutCase(t *testing.T) {
	for op, want := range map[string]operatorKind{
		"stringequals": opEquals, "STRINGNOTEQUALS": opEquals, "ForAnyValue:StringEqualsIgnoreCaseIfExists": opIgnoreCase,
		"forallvalues:stringnotlike": opPattern, "Null": opNull, "ArnNotLike": opOther, "NotIpAddress": opOther,
		"NumericNotEquals": opOther, "DateNotEquals": opOther, "Bool": opOther,
	} {
		c := newCondition(op, "k", nil)
		if c.kind != want {
			t.Errorf("%s: kind %d, want %d", op, c.kind, want)
		}
		if c.negated != (strings.Contains(strings.ToLower(op), "stringnot")) {
			t.Errorf("%s: negated = %v", op, c.negated)
		}
	}
	c := newCondition("ForAllValues:StringLikeIfExists", "k", nil)
	if c.set != setForAllValues || !c.ifExists || c.kind != opPattern {
		t.Fatalf("condition = %+v", c)
	}
	res := runAnalysis(t, managed(doc(condStatement("ArnNotLike", "aws:ResourceTag/env", `"arn:x"`))),
		change("c-000001", otherARN, planner.KindChangeValue, "env", "env", "arn:x", "arn:y"))
	if got := summary(res); got != "undecided arn:aws:iam::123456789012:policy/p0 aws:ResourceTag/env c-000001" {
		t.Fatalf("findings: %s", got)
	}
}

// BR4.7: patterns in values are case-sensitive, except TagKeys values.
func TestValuePatternsFollowTheOperator(t *testing.T) {
	m := &matcher{index: newChangeIndex(nil), patterns: newPatternCache()}
	cases := []struct {
		kind         operatorKind
		value, cand  string
		tagKeys, hit bool
	}{
		{opPattern, "Pro?", "Prod", false, true},
		{opPattern, "pro*", "Prod", false, false},
		{opPattern, "pro*", "Prod", true, true},
		{opEquals, "Prod", "prod", false, false},
		{opEquals, "Prod", "prod", true, true},
		{opIgnoreCase, "PROD", "prod", false, true},
		{opPattern, "a.b", "axb", false, false},
		{opNull, "true", "true", false, false},
	}
	for _, c := range cases {
		if got := m.valueMatches(c.kind, c.value, c.cand, c.tagKeys); got != c.hit {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

// BR4.6: a variable in the condition key cannot be decided for any bound
// change, except a case-only rename under aws:ResourceTag.
func TestAVariableInTheConditionKeyCannotBeDecided(t *testing.T) {
	a := managed(doc(condStatement("StringEquals", "aws:ResourceTag/${aws:PrincipalTag/k}", `"x"`)))
	res := runAnalysis(t, a,
		change("c-000001", otherARN, planner.KindRenameKey, "env", "ENV", "v", "v"),
		change("c-000002", otherARN, planner.KindChangeValue, "a", "a", "1", "2"),
		change("c-000003", otherARN, planner.KindAddRequired, "", "b", "", "1"),
	)
	if got := summary(res); got != "undecided arn:aws:iam::123456789012:policy/p0 aws:ResourceTag/${aws:PrincipalTag/k} c-000002,c-000003" {
		t.Fatalf("findings: %s", got)
	}
}

// --- Binding (BR3.1 to BR3.3) ---

// BR3.1: an IAM policy sees the changes of every resource.
func TestIAMPoliciesSeeEveryResource(t *testing.T) {
	a := &accountReader{groups: map[string]map[string]string{"g": {"p": doc(condStatement("StringEquals", "aws:RequestTag/env", `"x"`))}}}
	res := runAnalysis(t, a,
		change("c-000001", "arn:aws:s3:::a", planner.KindRemoveKey, "env", "", "x", ""),
		change("c-000002", "arn:aws:ec2:eu-west-1:123456789012:instance/i-1", planner.KindRemoveKey, "env", "", "x", ""))
	if got := summary(res); got != "affected arn:aws:iam::123456789012:group/g#p aws:RequestTag/env c-000001,c-000002" {
		t.Fatalf("findings: %s", got)
	}
}

// BR3.2: a resource policy sees only the changes of its own resource,
// compared as strings.
func TestResourcePoliciesSeeOnlyTheirOwnResource(t *testing.T) {
	queue := "arn:aws:sqs:us-east-1:123456789012:q"
	a := &accountReader{queues: map[string]map[string]string{testRegion: {queue: doc(condStatement("StringEquals", "aws:ResourceTag/env", `"x"`))}}}
	res := runAnalysis(t, a,
		change("c-000001", queue, planner.KindRemoveKey, "env", "", "x", ""),
		change("c-000002", "arn:aws:sqs:us-east-1:123456789012:other", planner.KindRemoveKey, "env", "", "x", ""),
		change("c-000003", strings.ToUpper(queue), planner.KindRemoveKey, "env", "", "x", ""))
	if got := summary(res); got != "affected "+queue+" aws:ResourceTag/env c-000001" {
		t.Fatalf("findings: %s", got)
	}
}

// BR3.3: PrincipalTag sees the changes of IAM users and roles only, in any
// policy; not groups, not other resources.
func TestPrincipalTagsSeeOnlyUsersAndRoles(t *testing.T) {
	bucket := "arn:aws:s3:::b"
	a := &accountReader{buckets: map[string]map[string]string{testRegion: {bucket: doc(condStatement("StringEquals", "aws:PrincipalTag/team", `"x"`))}}}
	res := runAnalysis(t, a,
		change("c-000001", "arn:aws:iam::123456789012:user/u", planner.KindRemoveKey, "team", "", "x", ""),
		change("c-000002", "arn:aws:iam::123456789012:role/path/r", planner.KindRemoveKey, "team", "", "x", ""),
		change("c-000003", "arn:aws:iam::123456789012:group/g", planner.KindRemoveKey, "team", "", "x", ""),
		change("c-000004", bucket, planner.KindRemoveKey, "team", "", "x", ""),
		change("c-000005", "arn:aws:ec2:us-east-1:123456789012:instance/i-1", planner.KindRemoveKey, "team", "", "x", ""))
	if got := summary(res); got != "affected "+bucket+" aws:PrincipalTag/team c-000001,c-000002" {
		t.Fatalf("findings: %s", got)
	}
}

// BR2.2: a wildcard in the tag key is matched as a pattern without case; a
// key it does not match has no impact.
func TestWildcardTagKeysAreMatchedAsPatterns(t *testing.T) {
	a := managed(doc(condStatement("StringEquals", "aws:ResourceTag/ENV*", `"x"`)))
	res := runAnalysis(t, a,
		change("c-000001", otherARN, planner.KindRemoveKey, "environment", "", "x", ""),
		change("c-000002", otherARN, planner.KindRemoveKey, "team", "", "x", ""))
	if got := summary(res); got != "affected arn:aws:iam::123456789012:policy/p0 aws:ResourceTag/ENV* c-000001" {
		t.Fatalf("findings: %s", got)
	}
}

// --- Reading the documents (BR1.7, BR2.1, BR2.4, NFR8.20) ---

// NFR8.20: a document of exactly 1 MiB is read; one byte more is not. IAM
// documents are measured before the URL encoding is undone.
func TestDocumentsLargerThanOneMiBAreNotRead(t *testing.T) {
	base := `{"Statement":[{"Sid":"S","Condition":{"StringEquals":{"aws:ResourceTag/env":"x"}}}],"Pad":""}`
	exact := base[:len(base)-2] + strings.Repeat("a", maxDocumentBytes-len(base)) + `"}`
	if len(exact) != maxDocumentBytes {
		t.Fatalf("length %d", len(exact))
	}
	if _, err := parseDocument(exact, false); err != nil {
		t.Fatalf("exactly 1 MiB: %v", err)
	}
	if _, err := parseDocument(exact+" ", false); !errors.Is(err, errTooLarge) {
		t.Fatalf("1 MiB + 1: %v", err)
	}
	encoded := url.PathEscape(exact[:maxDocumentBytes/3])
	if len(encoded) <= maxDocumentBytes {
		encoded += strings.Repeat("%20", (maxDocumentBytes-len(encoded))/3+1)
	}
	if _, err := parseDocument(encoded, true); !errors.Is(err, errTooLarge) {
		t.Fatalf("an encoded IAM document over 1 MiB must not be read, whatever its decoded size: %v", err)
	}

	queue := "arn:aws:sqs:us-east-1:123456789012:q"
	a := &accountReader{queues: map[string]map[string]string{testRegion: {queue: exact + " "}},
		managed: map[string]string{"arn:aws:iam::123456789012:policy/ok": base}}
	res, err := New(a, []string{testRegion}).Analyze(context.Background(),
		[]planner.TagChangeInput{change("c-000001", queue, planner.KindRemoveKey, "env", "", "x", "")})
	if err != nil || res.Complete || len(res.Findings) != 1 ||
		!slices.Equal(res.UnreadPolicies, []planner.UnreadPolicy{{SourceKind: planner.SourceSQSQueue, PolicyRef: queue, Region: testRegion, Reason: reasonTooLarge}}) {
		t.Fatalf("result = %+v, %v", res, err)
	}
}

// BR1.7: only IAM documents are URL-decoded; a % in a resource policy stays.
func TestOnlyIAMDocumentsAreURLDecoded(t *testing.T) {
	raw := `{"Statement":{"Condition":{"StringEquals":{"aws:ResourceTag/env":"%41"}}}}`
	resource, err := parseDocument(raw, false)
	if err != nil || resource[0].conditions[0].values[0] != "%41" {
		t.Fatalf("resource policy: %+v %v", resource, err)
	}
	iam, err := parseDocument(url.PathEscape(raw), true)
	if err != nil || iam[0].conditions[0].values[0] != "%41" {
		t.Fatalf("IAM policy (encoded once by IAM): %+v %v", iam, err)
	}
	twice, err := parseDocument(raw, true)
	if err != nil || twice[0].conditions[0].values[0] != "A" {
		t.Fatalf("decoding an IAM document undoes its encoding: %+v %v", twice, err)
	}
}

// Statement may be one object; the Sid or #<index> names a statement;
// numbers, booleans and null are values; Resource and NotResource may be a
// string or a list.
func TestStatementsValuesAndResourcesAreRead(t *testing.T) {
	statements, err := parseDocument(`{"Statement":[{"Sid":"","Resource":["a","b"],"NotResource":"c",`+
		`"Condition":{"NumericLessThan":{"k":[5,1.50,true,null,"s"]}}},{"Sid":"Two"}]}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if statements[0].id != "#0" || statements[1].id != "Two" {
		t.Fatalf("ids %q %q", statements[0].id, statements[1].id)
	}
	if got := statements[0].conditions[0].value(); got != "5, 1.50, true, s" {
		t.Fatalf("values %q", got)
	}
	if len(statements[0].resources) != 2 || strings.Join(statements[0].resources[0].values, ",") != "a,b" ||
		statements[0].resources[1].name != "NotResource" {
		t.Fatalf("resources %+v", statements[0].resources)
	}
	one, err := parseDocument(`{"Statement":{"Sid":"Only"}}`, false)
	if err != nil || len(one) != 1 || one[0].id != "Only" {
		t.Fatalf("one statement: %+v %v", one, err)
	}
	for _, bad := range []string{`{"Statement":[{"Resource":5}]}`, `{"Statement":[{"Resource":[1]}]}`, `{"Statement":["x"]}`,
		`{"Statement":[{"Condition":{"StringEquals":{"k":[[1]]}}}]}`, `{"Statement":[{"Condition":{"StringEquals":"x"}}]}`} {
		if _, err := parseDocument(bad, false); !errors.Is(err, errNotPolicy) {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

// BR2.1: the four tag condition keys of any service, without case; others
// are not references.
func TestTagConditionKeys(t *testing.T) {
	for key, want := range map[string]string{
		"aws:ResourceTag/env": "0 env true", "S3:resourcetag/Env": "0 Env false", "aws:RequestTag/a/b": "1 a/b true",
		"iam:PrincipalTag/team": "2 team false", "aws:TagKeys": "3  true", "AWS:tagkeys": "3  true",
		"s3:ExistingObjectTag/env": "", "aws:ResourceTag/": "", "bad prefix:ResourceTag/x": "", "env": "", "aws:TagKeysX": "",
	} {
		kind, tagKey, awsPrefix, ok := parseTagKey(key)
		got := ""
		if ok {
			got = fmt.Sprintf("%d %s %v", kind, tagKey, awsPrefix)
		}
		if got != want {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
}

// BR2.4: variables are found in condition keys, values, Resource and
// NotResource; a default value and spaces are allowed; an unclosed ${ and
// variables that name no tag are not references.
func TestPolicyVariablesAreReferences(t *testing.T) {
	statements, err := parseDocument(`{"Statement":[{"Resource":"arn:${aws:PrincipalTag/team, 'none'}","NotResource":"${aws:ResourceTag/x}",`+
		`"Condition":{"StringEquals":{"s3:prefix":["${aws:RequestTag/a}","${aws:username}","${aws:PrincipalTag/b"],`+
		`"aws:ResourceTag/${ aws:PrincipalTag/k }":"${aws:TagKeys}"}}}]}`, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range statements[0].references() {
		got = append(got, fmt.Sprintf("%d/%d/%s", r.loc, r.kind, r.tagKey))
	}
	want := []string{"0/0/${ aws:PrincipalTag/k }", "1/2/k", "2/1/a", "3/2/team", "4/0/x"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("references %v, want %v", got, want)
	}
}

// --- Findings and their order (BR5.1, BR5.2) ---

// BR5.1: one finding per condition and verdict; a change that is both
// affected and undecided in a condition is affected only; Resource findings
// have no operator and all the Resource values.
func TestFindingsAreOnePerConditionAndVerdict(t *testing.T) {
	a := &accountReader{roles: map[string]map[string]string{"app": {"p": doc(
		`{"Sid":"S","Effect":"Allow","Action":"*","Resource":["arn:a","arn:${aws:PrincipalTag/team}"],` +
			`"Condition":{"StringEquals":{"aws:ResourceTag/${aws:PrincipalTag/team}":["x","y"]}}}`)}}}
	res := runAnalysis(t, a,
		change("c-000001", roleARN, planner.KindRemoveKey, "team", "", "x", ""),
		change("c-000002", otherARN, planner.KindRemoveKey, "env", "", "x", ""),
		change("c-000003", roleARN, planner.KindChangeValue, "team", "team", "a", "b"))
	var got []string
	for _, f := range res.Findings {
		got = append(got, fmt.Sprintf("%s|%s|%s|%s|%s", f.Verdict, f.ConditionOperator, f.ConditionKey, f.ConditionValue, strings.Join(f.ChangeIDs, ",")))
	}
	want := []string{
		"affected|StringEquals|aws:ResourceTag/${aws:PrincipalTag/team}|x, y|c-000001",
		"undecided|StringEquals|aws:ResourceTag/${aws:PrincipalTag/team}|x, y|c-000002,c-000003",
		"affected||Resource|arn:a, arn:${aws:PrincipalTag/team}|c-000001",
		"undecided||Resource|arn:a, arn:${aws:PrincipalTag/team}|c-000003",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if res.Findings[0].StatementID != "S" || res.Findings[0].SourceKind != planner.SourceIAMInline {
		t.Fatalf("finding = %+v", res.Findings[0])
	}
}

// BR5.2: findings are ordered by kind, reference, statement, place,
// operator, key and verdict; unread policies by kind, region and reference.
func TestFindingsAndUnreadPoliciesAreOrdered(t *testing.T) {
	cond := condStatement("StringEquals", "aws:ResourceTag/env", `"x"`)
	a := &accountReader{
		managed: map[string]string{"arn:aws:iam::123456789012:policy/b": doc(cond, `{"Resource":"${aws:ResourceTag/env}"}`),
			"arn:aws:iam::123456789012:policy/a": doc(condStatement("StringLike", "aws:ResourceTag/env", `"x"`), cond)},
		queues:  map[string]map[string]string{"eu-west-1": {"arn:aws:sqs:eu-west-1:123456789012:q": doc(cond)}},
		buckets: map[string]map[string]string{"us-east-1": {"arn:aws:s3:::b": doc(cond)}},
		errs: map[string]error{
			"ListTopics eu-west-1": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpListTopics},
			"ListTopics us-east-1": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpListTopics},
			"ListPrincipals group": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpListGroups},
			"GetKeyPolicy":         &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpGetKeyPolicy},
		},
		keys: map[string]map[string]string{"eu-west-1": {"arn:aws:kms:eu-west-1:123456789012:key/k": "{}"}},
	}
	res, err := New(a, []string{"us-east-1", "eu-west-1"}).Analyze(context.Background(), []planner.TagChangeInput{
		change("c-000001", "arn:aws:s3:::b", planner.KindRemoveKey, "env", "", "x", ""),
		change("c-000002", "arn:aws:sqs:eu-west-1:123456789012:q", planner.KindRemoveKey, "env", "", "x", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.SourceKind+" "+f.PolicyRef[strings.LastIndexAny(f.PolicyRef, "/:")+1:]+" "+f.StatementID+" "+f.ConditionOperator+" "+f.ConditionKey)
	}
	// The statements of policy a share the Sid S: the statement order puts
	// StringLike (the first) before StringEquals.
	want := []string{"iamManaged a S StringLike aws:ResourceTag/env", "iamManaged a S StringEquals aws:ResourceTag/env",
		"iamManaged b S StringEquals aws:ResourceTag/env", "iamManaged b #1  Resource", "s3Bucket b S StringEquals aws:ResourceTag/env",
		"sqsQueue q S StringEquals aws:ResourceTag/env"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var unread []string
	for _, u := range res.UnreadPolicies {
		unread = append(unread, u.SourceKind+"|"+u.Region+"|"+u.PolicyRef+"|"+u.Reason)
	}
	wantUnread := []string{
		"iamInline|||access denied to iam:ListGroups",
		"kmsKey|eu-west-1|arn:aws:kms:eu-west-1:123456789012:key/k|AWS did not answer kms:GetKeyPolicy (temporary failure)",
		"snsTopic|eu-west-1||access denied to sns:ListTopics",
		"snsTopic|us-east-1||access denied to sns:ListTopics",
	}
	if strings.Join(unread, "\n") != strings.Join(wantUnread, "\n") || res.Complete {
		t.Fatalf("unread:\n%s\nwant:\n%s", strings.Join(unread, "\n"), strings.Join(wantUnread, "\n"))
	}
}

// BR1.1, BR1.4: managed policies are read when attached or used as a
// permissions boundary only; AWS managed KMS keys are not read.
func TestOnlyUsedManagedPoliciesAndCustomerKeysAreRead(t *testing.T) {
	cond := doc(condStatement("StringEquals", "aws:ResourceTag/env", `"x"`))
	key := "arn:aws:kms:us-east-1:123456789012:key/aws"
	a := &accountReader{
		boundary: map[string]string{"arn:aws:iam::123456789012:policy/boundary": cond},
		detached: map[string]string{"arn:aws:iam::123456789012:policy/detached": cond},
		keys:     map[string]map[string]string{testRegion: {key: cond}},
		awsKeys:  map[string]bool{key: true},
	}
	res := runAnalysis(t, a, change("c-000001", key, planner.KindRemoveKey, "env", "", "x", ""))
	if got := summary(res); got != "affected arn:aws:iam::123456789012:policy/boundary aws:ResourceTag/env c-000001" {
		t.Fatalf("findings: %s", got)
	}
	if a.count("GetKeyPolicy") != 0 || a.count("GetPolicyDocument") != 1 {
		t.Fatalf("calls = %v", a.calls)
	}
}

// BR1.3, BR1.5: resources without a policy, or deleted after the list, are
// not unread; buckets outside the plan's regions are not read; a bucket the
// list gave no region is located first; an unparsable document is unread.
func TestResourcesWithoutPoliciesAndUnlocatedBuckets(t *testing.T) {
	a := &accountReader{
		buckets: map[string]map[string]string{
			testRegion:  {"arn:aws:s3:::none": "", "arn:aws:s3:::moved": doc(condStatement("StringEquals", "aws:ResourceTag/env", `"x"`))},
			"eu-west-1": {"arn:aws:s3:::far": "not read"},
		},
		unlocated: map[string]bool{"arn:aws:s3:::moved": true},
		secrets:   map[string]map[string]string{testRegion: {"arn:aws:secretsmanager:us-east-1:123456789012:secret:gone": "x", "arn:aws:secretsmanager:us-east-1:123456789012:secret:bad": "{"}},
		errs: map[string]error{
			"GetSecretPolicy arn:aws:secretsmanager:us-east-1:123456789012:secret:gone": &errkind.Error{Kind: errkind.NotFound},
		},
	}
	res, err := New(a, []string{testRegion}).Analyze(context.Background(),
		[]planner.TagChangeInput{change("c-000001", "arn:aws:s3:::moved", planner.KindRemoveKey, "env", "", "x", "")})
	if err != nil {
		t.Fatal(err)
	}
	if got := summary(res); got != "affected arn:aws:s3:::moved aws:ResourceTag/env c-000001" {
		t.Fatalf("findings: %s", got)
	}
	want := []planner.UnreadPolicy{{SourceKind: planner.SourceSecret, PolicyRef: "arn:aws:secretsmanager:us-east-1:123456789012:secret:bad",
		Region: testRegion, Reason: reasonUnparsable}}
	if !slices.Equal(res.UnreadPolicies, want) {
		t.Fatalf("unread = %+v", res.UnreadPolicies)
	}
	if a.count("GetBucketRegion") != 1 || a.count("GetBucketPolicy") != 2 {
		t.Fatalf("calls = %v", a.calls)
	}
}

// BR1.5: an inline policy list or document that fails is unread with the
// principal's reference; a principal deleted after the list is not.
func TestInlinePolicyFailures(t *testing.T) {
	a := &accountReader{
		users: map[string]map[string]string{"gone": {"p": "{}"}, "denied": {"p": "{}"}, "ok": {"a": "{}", "b": "{}"}},
		errs: map[string]error{
			"ListInlinePolicyNames arn:aws:iam::123456789012:user/gone":   &errkind.Error{Kind: errkind.NotFound},
			"ListInlinePolicyNames arn:aws:iam::123456789012:user/denied": errors.New("no kind"),
			"GetInlinePolicyDocument arn:aws:iam::123456789012:user/ok#a": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetUserPolicy},
			"GetInlinePolicyDocument arn:aws:iam::123456789012:user/ok#b": &errkind.Error{Kind: errkind.NotFound},
		},
	}
	res, err := New(a, nil).Analyze(context.Background(), []planner.TagChangeInput{change("c-000001", otherARN, planner.KindRemoveKey, "env", "", "x", "")})
	if err != nil {
		t.Fatal(err)
	}
	want := []planner.UnreadPolicy{
		{SourceKind: planner.SourceIAMInline, PolicyRef: "arn:aws:iam::123456789012:user/denied", Reason: "unexpected error (Internal) in iam:ListUserPolicies"},
		{SourceKind: planner.SourceIAMInline, PolicyRef: "arn:aws:iam::123456789012:user/ok#a", Reason: "access denied to iam:GetUserPolicy"},
	}
	if !slices.Equal(res.UnreadPolicies, want) {
		t.Fatalf("unread = %+v", res.UnreadPolicies)
	}
}

// --- The pattern cache (security design §2) ---

func TestPatternCacheReusesPatternsUpToItsLimit(t *testing.T) {
	p := newPatternCache()
	first := p.get("a*b", false)
	if p.get("a*b", false) != first || p.size() != 1 {
		t.Fatal("the same pattern must be compiled once")
	}
	if p.get("a*b", true) == first || p.size() != 2 {
		t.Fatal("a pattern without case is another entry")
	}
	for i := 0; i < maxCachedPatterns+10; i++ {
		p.get(fmt.Sprintf("p%d*", i), false)
	}
	if p.size() != maxCachedPatterns {
		t.Fatalf("the cache holds %d patterns, limit %d", p.size(), maxCachedPatterns)
	}
	if !p.get("p5000*", false).MatchString("p5000x") {
		t.Fatal("a pattern beyond the limit still matches")
	}
}

func TestPatternCacheBoundsLongPatterns(t *testing.T) {
	p := newPatternCache()
	long := strings.Repeat("a", maxCachedPatternBytes) + "*"
	if !p.get(long, false).MatchString(long[:len(long)-1]+"zz") || p.size() != 0 {
		t.Fatalf("a long pattern is used but not kept (size %d)", p.size())
	}
	huge := strings.Repeat("a?", maxPatternRunes)
	if p.get(huge, false).MatchString(strings.Repeat("ab", maxPatternRunes)) {
		t.Fatal("a pattern too long for any tag never matches")
	}
	stars := strings.Repeat("*", 1<<20) + "x"
	if !p.get(stars, false).MatchString("abcx") {
		t.Fatal("runs of * are one *")
	}
}

// The jobs share the cache: under -race this shows it is safe.
func TestPatternCacheIsSafeForConcurrentUse(t *testing.T) {
	p := newPatternCache()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if !p.get(fmt.Sprintf("k%d*", i%50), i%2 == 0).MatchString(fmt.Sprintf("k%dz", i%50)) {
					t.Error("no match")
					return
				}
			}
		}()
	}
	wg.Wait()
	// i%50 and i%2 go together, so there are 50 distinct entries.
	if p.size() != 50 {
		t.Fatalf("size %d", p.size())
	}
}

func TestQueueARNFromItsURL(t *testing.T) {
	for url, want := range map[string]string{
		"https://sqs.us-east-1.amazonaws.com/123456789012/q":     "arn:aws:sqs:us-east-1:123456789012:q",
		"https://sqs.cn-north-1.amazonaws.com.cn/123456789012/q": "arn:aws-cn:sqs:cn-north-1:123456789012:q",
		"https://sqs.us-gov-west-1.amazonaws.com/123456789012/q": "arn:aws-us-gov:sqs:us-gov-west-1:123456789012:q",
		"not a url": "not a url",
	} {
		region := strings.Split(strings.TrimPrefix(url, "https://sqs."), ".")[0]
		if got := queueARN(url, region); got != want {
			t.Errorf("%s: %s", url, got)
		}
	}
}
