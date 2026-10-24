package rules

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of reading every item of contract C1 (BR1.2 to BR1.5, BR3.3 to
// BR3.5, BR5.1), written after the implementation.

const fullRules = `version: 1
keys:
  Environment:
    aliases: [env, Env]
    values:
      production: [prod, Prod]
      development: [dev]
  Owner: {}
required:
  CostCenter:
    default: "0000"
    when:
      - match: {resourceType: ec2:instance}
        value: "1111"
      - match:
          region: us-east-1
          tags: {Environment: production, "aws:cloudformation:stack-name": web}
        value: "2222"
  Team:
    when:
      - match: {resourceType: s3}
        value: storage
remove: [tmp-owner, old]
`

func TestEveryItemOfTheRuleFileIsRead(t *testing.T) {
	rs, err := parse("r.yaml", []byte(fullRules))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Warnings) != 0 {
		t.Fatalf("warnings = %+v", rs.Warnings)
	}
	wantKeys := []KeyRule{
		{CanonicalKey: "Environment", Aliases: []string{"env", "Env"}, Line: 3, Values: []ValueRule{
			{CanonicalValue: "production", Spellings: []string{"prod", "Prod"}, Line: 6},
			{CanonicalValue: "development", Spellings: []string{"dev"}, Line: 7},
		}},
		{CanonicalKey: "Owner", Aliases: []string{}, Values: []ValueRule{}, Line: 8},
	}
	if !reflect.DeepEqual(rs.KeyRules, wantKeys) {
		t.Fatalf("key rules = %+v", rs.KeyRules)
	}
	if len(rs.RequiredTags) != 2 {
		t.Fatalf("required = %+v", rs.RequiredTags)
	}
	cost := rs.RequiredTags[0]
	if cost.Key != "CostCenter" || cost.Line != 10 || !cost.HasDefault || cost.Default != "0000" || cost.DefaultLine != 11 || len(cost.When) != 2 {
		t.Fatalf("CostCenter = %+v", cost)
	}
	first, second := cost.When[0], cost.When[1]
	if first.ResourceType != "ec2:instance" || first.Region != "" || len(first.Tags) != 0 || first.Value != "1111" || first.Line != 13 {
		t.Fatalf("first condition = %+v", first)
	}
	wantTags := map[string]string{"Environment": "production", "aws:cloudformation:stack-name": "web"}
	if second.Region != "us-east-1" || !reflect.DeepEqual(second.Tags, wantTags) || second.Value != "2222" || second.Line != 15 {
		t.Fatalf("second condition = %+v", second)
	}
	team := rs.RequiredTags[1]
	if team.Key != "Team" || team.HasDefault || len(team.When) != 1 || team.When[0].ResourceType != "s3" {
		t.Fatalf("Team = %+v", team)
	}
	if fmt.Sprint(rs.RemoveKeys) != "[{tmp-owner 23} {old 23}]" {
		t.Fatalf("remove = %v", rs.RemoveKeys)
	}
}

func TestTextLengthsFollowTheAWSTagLimits(t *testing.T) {
	key128, key129 := strings.Repeat("k", 128), strings.Repeat("k", 129)
	value256, value257 := strings.Repeat("v", 256), strings.Repeat("v", 257)
	ok := []string{
		"version: 1\nkeys:\n  " + key128 + ": {aliases: [" + strings.Repeat("a", 128) + "]}\n",
		"version: 1\nkeys:\n  " + strings.Repeat("鍵", 128) + ": {}\n",
		"version: 1\nkeys:\n  A:\n    values: {" + value256 + ": [\"\"]}\n",
		"version: 1\nrequired:\n  A: {default: \"\"}\n",
		"version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: {" + key128 + ": " + value256 + "}}\n        value: " + value256 + "\n",
	}
	for i, content := range ok {
		if _, err := parse("r.yaml", []byte(content)); err != nil {
			t.Fatalf("case %d must be read: %v", i, err)
		}
	}
	bad := []struct {
		name    string
		content string
		line    int
		hint    string
	}{
		{"canonical key", "version: 1\nkeys:\n  " + key129 + ": {}\n", 3, "1 to 128 characters"},
		{"alias", "version: 1\nkeys:\n  A:\n    aliases: [" + key129 + "]\n", 4, "1 to 128 characters"},
		{"required key", "version: 1\nrequired:\n  " + key129 + ": {}\n", 3, "1 to 128 characters"},
		{"removed key", "version: 1\nremove: [" + key129 + "]\n", 2, "1 to 128 characters"},
		{"empty removed key", "version: 1\nremove: [\"\"]\n", 2, "remove the empty key"},
		{"condition tag key", "version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: {" + key129 + ": x}}\n        value: y\n", 5, "1 to 128 characters"},
		{"canonical value", "version: 1\nkeys:\n  A:\n    values:\n      " + value257 + ": []\n", 5, "0 to 256 characters"},
		{"spelling", "version: 1\nkeys:\n  A:\n    values:\n      a: [" + value257 + "]\n", 5, "0 to 256 characters"},
		{"default", "version: 1\nrequired:\n  A:\n    default: " + value257 + "\n", 4, "0 to 256 characters"},
		{"condition value", "version: 1\nrequired:\n  A:\n    when:\n      - match: {region: us-east-1}\n        value: " + value257 + "\n", 6, "0 to 256 characters"},
		{"condition tag value", "version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: {k: " + value257 + "}}\n        value: y\n", 5, "0 to 256 characters"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			e, isRuleError := errkind.As(err)
			if !isRuleError || e.Kind != errkind.InvalidInput || e.Line != tc.line || !strings.Contains(e.Hint, tc.hint) {
				t.Fatalf("err = %+v, want line %d and a hint with %q", e, tc.line, tc.hint)
			}
		})
	}
}

func TestMatchIsWrittenAsScanShowsIt(t *testing.T) {
	hint := func(t string) (string, bool) {
		if t == "AWS::EC2::Instance" {
			return "ec2:instance", true
		}
		return "", false
	}
	when := func(match string) string {
		return "version: 1\nrequired:\n  Owner:\n    when:\n      - match: " + match + "\n        value: x\n"
	}
	bad := []struct {
		name   string
		match  string
		opts   Options
		key    string
		detail string
		fix    string
	}{
		{"CloudFormation type with a hint", "{resourceType: AWS::EC2::Instance}", Options{TypeHint: hint}, "resourceType",
			"write the resource type as scan shows it", `write "ec2:instance"`},
		{"CloudFormation type the table lacks", "{resourceType: AWS::Example::Thing}", Options{TypeHint: hint}, "resourceType",
			"this resource type cannot be used in a condition", `only types that scan shows as "service" or "service:type"`},
		{"CloudFormation type without a hint", "{resourceType: AWS::EC2::Instance}", Options{}, "resourceType",
			"this resource type cannot be used in a condition", "for example s3, sqs, ec2:instance"},
		{"unknown", "{resourceType: unknown}", Options{}, "resourceType", `"unknown" is not a resource type`, "ec2:instance"},
		{"upper case service", "{resourceType: EC2}", Options{}, "resourceType", `"EC2" is not a resource type as scan shows it`, "ec2:instance"},
		{"three parts", "{resourceType: \"ec2:instance:x\"}", Options{}, "resourceType", "is not a resource type", "service:type"},
		{"region", "{region: useast1}", Options{}, "region", `"useast1" is not an AWS region name`, "us-east-1"},
		{"empty match", "{}", Options{}, "match", "a condition needs at least one of resourceType, region or tags", `use "default"`},
		{"null match", "", Options{}, "match", "a condition needs at least one", `use "default"`},
		{"empty tags", "{tags: {}}", Options{}, "match", "a condition needs at least one", `use "default"`},
		{"number as type", "{resourceType: 3}", Options{}, "3", `"resourceType" must be a text value`, "quotes"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("r.yaml", []byte(when(tc.match)), tc.opts)
			e := requireRuleError(t, err, errkind.InvalidInput, 5, tc.key)
			if !strings.Contains(e.Detail, tc.detail) || !strings.Contains(e.Hint, tc.fix) {
				t.Fatalf("detail = %q, hint = %q", e.Detail, e.Hint)
			}
		})
	}
	for _, match := range []string{"{resourceType: s3}", "{resourceType: sqs}", "{resourceType: ec2:instance}",
		"{resourceType: states:stateMachine}", "{resourceType: elasticloadbalancing:loadbalancer, region: ap-northeast-1}",
		"{tags: {\"aws:cloudformation:stack-name\": web}}"} {
		rs, err := Parse("r.yaml", []byte(when(match)), Options{TypeHint: hint})
		if err != nil || len(rs.Warnings) != 1 { // the "no key rules" warning only
			t.Fatalf("%s must be read: %+v %v", match, rs, err)
		}
	}
}

func TestAWSKeysAreIgnoredWithAWarningExceptInConditions(t *testing.T) {
	content := "version: 1\nkeys:\n  Owner:\n    aliases: [aws:owner]\nrequired:\n  AWS:Team:\n    default: x\n  Team:\n" +
		"    when:\n      - match: {tags: {\"aws:createdBy\": me}}\n        value: y\nremove: [aws:tmp, old]\n"
	rs, err := parse("r.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range rs.Warnings {
		got = append(got, fmt.Sprintf("%d %s", w.Line, w.Key))
		if !strings.HasPrefix(w.Message, "keys starting with aws: belong to AWS") {
			t.Fatalf("warning = %+v", w)
		}
	}
	if strings.Join(got, ",") != "4 aws:owner,6 AWS:Team,12 aws:tmp" {
		t.Fatalf("warnings = %v", got)
	}
	if len(rs.KeyRules[0].Aliases) != 0 || len(rs.RequiredTags) != 1 || rs.RequiredTags[0].When[0].Tags["aws:createdBy"] != "me" ||
		fmt.Sprint(rs.RemoveKeys) != "[{old 12}]" {
		t.Fatalf("rs = %+v", rs)
	}
}

func TestRepeatedRemovedKeysAreKeptOnceWithAWarning(t *testing.T) {
	rs, err := parse("r.yaml", []byte("version: 1\nkeys: {A: {}}\nremove:\n  - tmp\n  - Tmp\n  - tmp\n"))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(rs.RemoveKeys) != "[{tmp 4} {Tmp 5}]" {
		t.Fatalf("remove = %v", rs.RemoveKeys)
	}
	if len(rs.Warnings) != 1 || rs.Warnings[0].Line != 6 || !strings.Contains(rs.Warnings[0].Message, "more than once") {
		t.Fatalf("warnings = %+v", rs.Warnings)
	}
}

func TestStructureErrorsOfTheNewItems(t *testing.T) {
	cases := []struct {
		name    string
		content string
		line    int
		detail  string
	}{
		{"spellings not a list", "version: 1\nkeys:\n  A:\n    values:\n      a: b\n", 5, "must be a list"},
		{"value as a number", "version: 1\nkeys:\n  A:\n    values:\n      a: [1]\n", 5, "must be a text value"},
		{"when entry not a mapping", "version: 1\nrequired:\n  A:\n    when: [x]\n", 4, "must be a mapping"},
		{"when entry without match", "version: 1\nrequired:\n  A:\n    when:\n      - value: x\n", 5, `has no "match"`},
		{"tags not a mapping", "version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: [a]}\n        value: x\n", 5, "must be a mapping"},
		{"tag key as a number", "version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: {1: a}}\n        value: x\n", 5, "must be a text value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			e, ok := errkind.As(err)
			if !ok || e.Line != tc.line || !strings.Contains(e.Detail, tc.detail) || e.Hint == "" {
				t.Fatalf("err = %v, want line %d and %q", err, tc.line, tc.detail)
			}
		})
	}
}
