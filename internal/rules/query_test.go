package rules

import (
	"maps"
	"testing"
)

// Unit tests of the queries U4 will use (BR2.3, BR3.2, BR4.2, the rules of
// AC2.1.3 and AC2.1.4), written after the implementation.

const queryRules = `version: 1
keys:
  Environment:
    aliases: [env, Env]
    values:
      production: [prod, Prod]
  Team: {aliases: [team]}
required:
  Owner:
    default: platform-team
    when:
      - match: {resourceType: ec2:instance}
        value: infra-team
      - match: {resourceType: ec2:instance, region: eu-west-1}
        value: never-reached
      - match: {region: us-east-1}
        value: us-team
      - match: {tags: {Environment: production}}
        value: prod-team
      - match: {resourceType: s3}
        value: storage-team
      - match: {resourceType: sqs}
        value: queue-team
  CostCenter:
    when:
      - match: {tags: {Team: a, "aws:cloudformation:stack-name": web}}
        value: "1111"
remove: [tmp-owner]
`

func mustParse(t *testing.T, content string) *RuleSet {
	t.Helper()
	rs, err := parse("r.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

// AC2.1.3 (rule): the condition on the resource type wins over the default,
// conditions are tried from the top, and a resource no condition matches
// gets the default.
func TestRequiredValueTakesTheFirstMatchingConditionThenTheDefault(t *testing.T) {
	rs := mustParse(t, queryRules)
	cases := []struct {
		name  string
		res   Resource
		value string
		by    DecidedBy
		line  int
	}{
		{"EC2 instance (first entry wins over the third)", Resource{ResourceType: "ec2:instance", Region: "us-east-1"}, "infra-team", DecidedByWhen, 12},
		{"EC2 instance in eu-west-1 (first entry wins over the second)", Resource{ResourceType: "ec2:instance", Region: "eu-west-1"}, "infra-team", DecidedByWhen, 12},
		{"region", Resource{ResourceType: "lambda:function", Region: "us-east-1"}, "us-team", DecidedByWhen, 16},
		{"S3 bucket", Resource{ResourceType: "s3", Region: "eu-west-1", Global: true}, "storage-team", DecidedByWhen, 20},
		{"SQS queue", Resource{ResourceType: "sqs", Region: "eu-west-1"}, "queue-team", DecidedByWhen, 22},
		{"nothing matches", Resource{ResourceType: "lambda:function", Region: "eu-west-1"}, "platform-team", DecidedByDefault, 10},
		{"tags after aliases and spellings", Resource{ResourceType: "lambda:function", Region: "eu-west-1",
			Tags: map[string]string{"env": "Prod"}}, "prod-team", DecidedByWhen, 18},
		// A global resource has no region of its own: the region condition
		// does not match even when it was read in us-east-1.
		{"global resource read in us-east-1", Resource{ResourceType: "iam:role", Region: "us-east-1", Global: true}, "platform-team", DecidedByDefault, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rs.RequiredValue("Owner", tc.res)
			want := RequiredValueAnswer{Key: "Owner", Status: StatusResolved, Value: tc.value, DecidedBy: tc.by, Line: tc.line}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestRequiredValueIsUnspecifiedWithoutADefault(t *testing.T) {
	rs := mustParse(t, queryRules)
	unspecified := func(key string) RequiredValueAnswer {
		return RequiredValueAnswer{Key: key, Status: StatusUnspecified, DecidedBy: DecidedByNone}
	}
	if got := rs.RequiredValue("CostCenter", Resource{Tags: map[string]string{"Team": "b"}}); got != unspecified("CostCenter") {
		t.Fatalf("no condition matches and no default: %+v", got)
	}
	if got := rs.RequiredValue("NotRequired", Resource{}); got != unspecified("NotRequired") {
		t.Fatalf("a key that is not required: %+v", got)
	}
	// aws: tags can be used in a condition, and the alias team counts as Team.
	got := rs.RequiredValue("CostCenter", Resource{Tags: map[string]string{"team": "a", "aws:cloudformation:stack-name": "web"}})
	if got.Status != StatusResolved || got.Value != "1111" || got.Line != 26 {
		t.Fatalf("aws: tag in a condition: %+v", got)
	}
}

func TestUnifiedTagsRenameAliasesUnifyValuesAndDropRemovedKeys(t *testing.T) {
	rs := mustParse(t, queryRules)
	cases := []struct {
		name string
		in   map[string]string
		want map[string]string
	}{
		{"alias and spelling", map[string]string{"env": "prod", "Owner": "x"}, map[string]string{"Environment": "production", "Owner": "x"}},
		{"removed key and aws: key", map[string]string{"tmp-owner": "a", "aws:createdBy": "me"}, map[string]string{"aws:createdBy": "me"}},
		{"the canonical key's value wins", map[string]string{"Environment": "dev", "env": "prod"}, map[string]string{"Environment": "dev"}},
		{"the alias listed first wins", map[string]string{"Env": "b", "env": "a"}, map[string]string{"Environment": "a"}},
		{"values are unified after the rename", map[string]string{"Environment": "Prod"}, map[string]string{"Environment": "production"}},
		{"unknown values and keys stay", map[string]string{"Environment": "qa", "Other": "prod"}, map[string]string{"Environment": "qa", "Other": "prod"}},
		{"no tags", nil, map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := maps.Clone(tc.in)
			got := rs.UnifiedTags(tc.in)
			if !maps.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if !maps.Equal(tc.in, before) {
				t.Fatal("the input tags were changed")
			}
		})
	}
}

// BR2.3: the canonical value of a spelling, and "unchanged" for anything
// else; values are compared exactly.
func TestCanonicalValue(t *testing.T) {
	rs := mustParse(t, queryRules)
	cases := []struct {
		key, value string
		want       ValueAnswer
	}{
		{"Environment", "prod", ValueAnswer{Value: "production", Changed: true, Line: 6}},
		{"Environment", "Prod", ValueAnswer{Value: "production", Changed: true, Line: 6}},
		{"Environment", "production", ValueAnswer{Value: "production"}},
		{"Environment", "PROD", ValueAnswer{Value: "PROD"}},
		{"env", "prod", ValueAnswer{Value: "prod"}}, // ask with the canonical key
		{"Team", "prod", ValueAnswer{Value: "prod"}},
	}
	for _, tc := range cases {
		if got := rs.CanonicalValue(tc.key, tc.value); got != tc.want {
			t.Errorf("CanonicalValue(%q, %q) = %+v, want %+v", tc.key, tc.value, got, tc.want)
		}
	}
}

// AC2.1.4 (rule): only the key listed under remove is removed, compared
// exactly.
func TestIsRemovedComparesExactly(t *testing.T) {
	rs := mustParse(t, queryRules)
	if got := rs.IsRemoved("tmp-owner"); got != (RemoveAnswer{Remove: true, Line: 28}) {
		t.Fatalf("tmp-owner: %+v", got)
	}
	for _, key := range []string{"Tmp-Owner", "tmp_owner", "tmpowner", "Owner", "env"} {
		if got := rs.IsRemoved(key); got.Remove || got.Line != 0 {
			t.Errorf("IsRemoved(%q) = %+v", key, got)
		}
	}
	if c, ok := rs.CanonicalKeyOf("Env"); !ok || c != "Environment" {
		t.Fatalf("CanonicalKeyOf(Env) = %q %v", c, ok)
	}
	if c, ok := rs.CanonicalKeyOf("Environment"); ok || c != "Environment" {
		t.Fatalf("CanonicalKeyOf(Environment) = %q %v", c, ok)
	}
}

// A rule set built by hand (as U4's tests may) answers the same, without the
// tables Parse builds.
func TestARuleSetBuiltByHandAnswersTheSame(t *testing.T) {
	parsed := mustParse(t, queryRules)
	byHand := &RuleSet{KeyRules: parsed.KeyRules, RequiredTags: parsed.RequiredTags, RemoveKeys: parsed.RemoveKeys}
	res := Resource{ResourceType: "lambda:function", Region: "eu-west-1", Tags: map[string]string{"Env": "prod"}}
	if got, want := byHand.RequiredValue("Owner", res), parsed.RequiredValue("Owner", res); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if byHand.CanonicalValue("Environment", "Prod") != parsed.CanonicalValue("Environment", "Prod") ||
		byHand.IsRemoved("tmp-owner") != parsed.IsRemoved("tmp-owner") {
		t.Fatal("answers differ")
	}
}
