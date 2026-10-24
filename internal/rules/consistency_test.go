package rules

import (
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the checks between items (BR2.1, BR2.2, BR3.1, BR3.6, BR4.1),
// written after the implementation.

const envKeys = "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      production: [prod]\n"

func TestItemsThatContradictEachOtherAreErrors(t *testing.T) {
	cases := []struct {
		name    string
		content string
		line    int
		key     string
		detail  string
		fix     string
	}{
		{"spelling under two values", "version: 1\nkeys:\n  E:\n    values:\n      production: [prod]\n      prd: [prod]\n",
			6, "prod", `the spelling "prod" of "E" is listed under the values "production" (line 5) and "prd" (line 6)`, "one canonical value only"},
		{"spelling that is another value", "version: 1\nkeys:\n  E:\n    values:\n      production: [prod]\n      prod: [p]\n",
			5, "prod", `is also a canonical value (lines 5 and 6)`, "remove it from the spellings"},
		{"required alias", envKeys + "required:\n  env: {default: x}\n",
			8, "env", `"env" is an alias of "Environment"; a required tag must use the canonical key`, `write "Environment" here`},
		{"required and removed", envKeys + "required:\n  Owner: {default: x}\nremove: [Owner]\n",
			8, "Owner", `"Owner" is required and also listed under "remove" (line 9)`, "keep the key in one of them only"},
		{"condition on an alias", envKeys + "required:\n  Owner:\n    when:\n      - match: {region: us-east-1, tags: {env: production}}\n        value: x\n",
			10, "env", `"env" is an alias of "Environment"; conditions compare tags after aliases are renamed`, `write "Environment" here`},
		{"condition on a spelling", envKeys + "required:\n  Owner:\n    when:\n      - match:\n          tags:\n            Environment: prod\n        value: x\n",
			12, "Environment", `"prod" is another spelling of "production" for "Environment"; conditions compare values after they are unified`, `write "production" here`},
		{"condition on a removed key", envKeys + "required:\n  Owner:\n    when:\n      - match: {tags: {tmp: x}}\n        value: x\nremove: [tmp]\n",
			10, "tmp", `"tmp" is listed under "remove" (line 12); conditions compare tags after removed keys are taken away`, "remove the tag from the condition"},
		{"removed canonical key", envKeys + "remove: [Environment]\n",
			7, "Environment", `"Environment" is a canonical key under "keys" (line 3)`, `remove it from "remove", or remove its rule from "keys"`},
		{"removed alias", envKeys + "remove: [env]\n",
			7, "env", `"env" is an alias of "Environment"`, `the alias is renamed by the plan; remove it from "remove"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			e := requireRuleError(t, err, errkind.InvalidInput, tc.line, tc.key)
			if !strings.Contains(e.Detail, tc.detail) || !strings.Contains(e.Hint, tc.fix) {
				t.Fatalf("detail = %q\nhint = %q", e.Detail, e.Hint)
			}
		})
	}
}

// BR1.1: of several mistakes, the first one checked is reported.
func TestTheFirstContradictionStopsTheReading(t *testing.T) {
	content := "version: 1\nkeys:\n  A:\n    aliases: [a]\n  B:\n    aliases: [a]\nrequired:\n  a: {}\nremove: [A]\n"
	_, err := parse("r.yaml", []byte(content))
	e := requireRuleError(t, err, errkind.InvalidInput, 6, "a")
	if !strings.Contains(e.Detail, "is listed under") {
		t.Fatalf("detail = %q", e.Detail)
	}
}

func TestHarmlessRepeatsOfValuesGiveWarnings(t *testing.T) {
	rs, err := parse("r.yaml", []byte("version: 1\nkeys:\n  E:\n    values:\n      production: [prod, production, prod, Prod]\n      dev: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	v := rs.KeyRules[0].Values
	if len(v) != 2 || strings.Join(v[0].Spellings, ",") != "prod,Prod" || len(v[1].Spellings) != 0 {
		t.Fatalf("values = %+v", v)
	}
	if len(rs.Warnings) != 2 || !strings.Contains(rs.Warnings[0].Message, "is the value itself") ||
		!strings.Contains(rs.Warnings[1].Message, "more than once") {
		t.Fatalf("warnings = %+v", rs.Warnings)
	}
	// The same spelling under two keys is fine: values belong to their key.
	if _, err := parse("r.yaml", []byte("version: 1\nkeys:\n  A:\n    values: {x: [y]}\n  B:\n    values: {x: [y]}\n")); err != nil {
		t.Fatal(err)
	}
}

func TestItemsThatDoNotContradictEachOtherAreRead(t *testing.T) {
	content := envKeys + "required:\n" +
		"  Environment: {default: production}\n" + // a canonical key may be required
		"  NewKey:\n" + // a key that is not under keys is a new tag
		"    when:\n" +
		"      - match: {tags: {Environment: production, Other: prod}}\n" + // a spelling of another key is a plain value
		"        value: x\n" +
		"remove: [Env]\n" // the case differs from the alias env
	rs, err := parse("r.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.RequiredTags) != 2 || len(rs.RemoveKeys) != 1 {
		t.Fatalf("rs = %+v", rs)
	}
}
