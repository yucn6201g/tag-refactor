package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// load and parse read a rule file without options, as the U1 tests did
// before U3 added them.
func load(path string) (*RuleSet, error) { return Load(path, Options{}) }

func parse(path string, data []byte) (*RuleSet, error) { return Parse(path, data, Options{}) }

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tagrules.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// requireRuleError checks the kind, line and key of a rule file error and
// that it carries a fix.
func requireRuleError(t *testing.T, err error, kind errkind.Kind, line int, key string) *errkind.Error {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok {
		t.Fatalf("error has no kind: %v", err)
	}
	if e.Kind != kind || e.Line != line || e.Key != key {
		t.Fatalf("got %s line %d key %q (%s), want %s line %d key %q", e.Kind, e.Line, e.Key, e.Detail, kind, line, key)
	}
	if e.Hint == "" || e.Detail == "" {
		t.Fatalf("error without detail or fix: %+v", e)
	}
	return e
}

func TestLoadReadsAliasesFromCRLFFileWithBOM(t *testing.T) {
	content := "\xef\xbb\xbf# tag rules\r\nversion: 1\r\nkeys:\r\n  Environment:\r\n    aliases:\r\n      - env\r\n      - Env\r\n  CostCenter:\r\n    aliases: [cost-center, cost_center]\r\n"
	path := writeFile(t, content)

	rs, err := load(path)

	if err != nil {
		t.Fatal(err)
	}
	if rs.Version != 1 || rs.SourcePath != path || len(rs.Warnings) != 0 || len(rs.KeyRules) != 2 {
		t.Fatalf("rule set = %+v", rs)
	}
	env := rs.KeyRules[0]
	if env.CanonicalKey != "Environment" || env.Line != 4 || strings.Join(env.Aliases, ",") != "env,Env" {
		t.Fatalf("Environment rule = %+v", env)
	}
	same, err := parse("lf.yaml", []byte(strings.ReplaceAll(content, "\r\n", "\n")))
	if err != nil || same.KeyRules[1].Line != rs.KeyRules[1].Line {
		t.Fatalf("CRLF and LF files differ: %+v %v", same, err)
	}
}

func TestLoadAcceptsOnlyReadableRegularFilesUpTo64KiB(t *testing.T) {
	_, err := load(filepath.Join(t.TempDir(), "missing.yaml"))
	_ = requireRuleError(t, err, errkind.NotFound, 0, "")

	_, err = load(t.TempDir())
	_ = requireRuleError(t, err, errkind.InvalidInput, 0, "")

	if MaxFileSize != 64*1024 {
		t.Fatalf("MaxFileSize = %d, want 64 KiB", MaxFileSize)
	}
	big := "version: 1\n" + strings.Repeat("#", MaxFileSize)
	_, err = load(writeFile(t, big))
	e := requireRuleError(t, err, errkind.InvalidInput, 0, "")
	if !strings.Contains(e.Detail, "64 KiB") || !strings.Contains(e.Hint, "64 KiB") {
		t.Fatalf("detail = %q, hint = %q", e.Detail, e.Hint)
	}

	exact := "version: 1\n#" + strings.Repeat("x", MaxFileSize-len("version: 1\n#"))
	if _, err := load(writeFile(t, exact)); err != nil {
		t.Fatalf("a file of exactly 64 KiB must be read: %v", err)
	}
	if e := storageError("p", os.ErrPermission).(*errkind.Error); e.Kind != errkind.StorageFailed || e.Subject != "p" {
		t.Fatalf("storage error = %+v", e)
	}
}

func TestParseRejectsUnsafeYAMLFeatures(t *testing.T) {
	cases := []struct {
		name    string
		content string
		line    int
		key     string
	}{
		{"anchor", "version: 1\nkeys:\n  A: &rule\n    aliases: [a]\n", 3, ""},
		{"alias", "version: 1\nkeys:\n  A:\n    aliases: [a]\n  B: *rule\n", 5, ""},
		{"merge key", "version: 1\nkeys:\n  A:\n    <<: {aliases: [a]}\n", 4, "<<"},
		{"tag", "version: 1\nkeys:\n  A:\n    aliases: !!seq [a]\n", 4, ""},
		{"duplicate key", "version: 1\nkeys:\n  A:\n    aliases: [a]\n  A:\n    aliases: [b]\n", 5, "A"},
		{"complex key", "version: 1\n? keys\n: {}\n", 2, ""},
		{"two documents", "version: 1\n---\nversion: 1\n", 3, ""},
		{"deep nesting", "version: 1\nremove: " + strings.Repeat("[", 40) + strings.Repeat("]", 40) + "\n", 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			_ = requireRuleError(t, err, errkind.InvalidInput, tc.line, tc.key)
		})
	}
	// Many aliases expanding a large structure (a "YAML bomb") never get
	// expanded: the first anchor is rejected.
	bomb := "version: 1\na: &a [x, x, x, x, x, x, x, x, x, x]\nb: &b [*a, *a, *a, *a, *a, *a, *a, *a, *a, *a]\nc: [*b, *b, *b, *b, *b, *b, *b, *b, *b, *b]\n"
	_, err := parse("bomb.yaml", []byte(bomb))
	_ = requireRuleError(t, err, errkind.InvalidInput, 2, "")
}

// Deep nesting makes the YAML parser use time and memory that grow with the
// square of the depth (200k "[" ran out of memory), so it is rejected before
// the parser runs, quickly and with the line and a fix.
func TestDeepNestingIsRejectedBeforeTheParserRuns(t *testing.T) {
	var indented strings.Builder
	indented.WriteString("version: 1\n")
	for i := 0; i < 40; i++ {
		indented.WriteString(strings.Repeat(" ", 2*i) + "k:\n")
	}
	cases := []struct {
		name    string
		content string
		line    int
		detail  string
	}{
		// Since U3, Parse rejects more than 64 KiB first, so the inputs stay
		// just below it to reach the nesting check.
		{"60k flow sequences", "version: 1\nremove: " + strings.Repeat("[", 60_000), 2, tooDeep},
		{"flow mappings", "version: 1\nremove: " + strings.Repeat("{a: ", 15_000), 2, tooDeep},
		{"after a document marker", "--- " + strings.Repeat("[", 60_000), 1, tooDeep},
		{"after anchors", "version: 1\nremove: " + strings.Repeat("[&a ']', ", 100), 2, tooDeep},
		{"compact sequences", "version: 1\nremove:\n" + strings.Repeat("- ", 30_000) + "x\n", 3, tooDeep},
		{"indentation", indented.String(), 34, tooDeep},
		{"brackets in one quoted value", "version: 1\nremove: ['" + strings.Repeat("[", 2000) + "']\n", 2, tooManyBrackets},
		{"brackets in a quoted value over lines", "version: 1\nremove: [\"a\n" + strings.Repeat("[[[[[[[[[[\n", 200) + "\"]\n", 105, tooManyBrackets},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, err := parse("r.yaml", []byte(tc.content))
			elapsed := time.Since(start)

			e := requireRuleError(t, err, errkind.InvalidInput, tc.line, "")
			if e.Detail != tc.detail {
				t.Fatalf("detail = %q, want %q", e.Detail, tc.detail)
			}
			if elapsed > 2*time.Second {
				t.Fatalf("took %v; deep nesting must be rejected quickly", elapsed)
			}
		})
	}

	// The same through Load, with a file of brackets as large as allowed.
	start := time.Now()
	_, err := load(writeFile(t, "version: 1\nremove: "+strings.Repeat("[", MaxFileSize-len("version: 1\nremove: "))))
	e := requireRuleError(t, err, errkind.InvalidInput, 2, "")
	if e.Detail != tooDeep || time.Since(start) > 2*time.Second {
		t.Fatalf("detail = %q after %v", e.Detail, time.Since(start))
	}
}

// Brackets in quoted values, comments, block scalars and plain text are not
// nesting, and nesting within the limit reaches the parser.
func TestNestingCheckIgnoresQuotesCommentsAndBlockScalars(t *testing.T) {
	forty := strings.Repeat("[", 40)
	content := "version: 1 # " + forty + "\n" +
		"# " + forty + "\n" +
		"keys:\n" +
		"  Environment:\n" +
		"    aliases: [\"" + forty + "\\\"\", '" + strings.Repeat("{", 40) + " it''s', env]  # " + forty + "\n" +
		"required:\n" +
		"  CostCenter:\n" +
		"    when: []\n" +
		"remove:\n" +
		"  - a[b\n" +
		"  - \"c\"\n"
	rs, err := parse("r.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rs.KeyRules[0].Aliases, ","); got != forty+`",`+strings.Repeat("{", 40)+" it's,env" {
		t.Fatalf("aliases = %q", got)
	}

	// The content of a block scalar is text: the parser reads it, and the
	// schema (which wants plain or quoted text) rejects it.
	blockScalar := "version: 1\nrequired:\n  CostCenter:\n    default: |\n      " + forty + "\n      '\n    when: []\n"
	_, err = parse("r.yaml", []byte(blockScalar))
	e, ok := errkind.As(err)
	if !ok || e.Line != 4 || e.Detail != `"default" must be a text value` {
		t.Fatalf("block scalar content must reach the parser: %v", err)
	}

	// 20 levels pass the check; the schema then rejects the nested lists.
	_, err = parse("r.yaml", []byte("version: 1\nremove: "+strings.Repeat("[", 20)+strings.Repeat("]", 20)+"\n"))
	e, ok = errkind.As(err)
	if !ok || e.Kind != errkind.InvalidInput || e.Detail == tooDeep || !strings.Contains(e.Detail, `"remove"`) {
		t.Fatalf("20 levels must reach the parser and the schema check: %v", err)
	}
}

func TestParseVersionAndStructureErrors(t *testing.T) {
	cases := []struct {
		name    string
		content string
		kind    errkind.Kind
		line    int
		key     string
	}{
		{"empty", "", errkind.InvalidInput, 1, ""},
		{"only comments", "# nothing\n", errkind.InvalidInput, 1, ""},
		{"not a mapping", "- a\n- b\n", errkind.InvalidInput, 1, ""},
		{"no version", "keys: {}\n", errkind.InvalidInput, 1, "version"},
		{"version as text", "version: \"1\"\n", errkind.InvalidInput, 1, "version"},
		{"newer version", "keys:\n  A: {}\nversion: 2\nfuture: true\n", errkind.UnsupportedVersion, 3, "version"},
		{"broken YAML", "version: 1\nkeys:\n  A:\n    aliases: [a, b\n", errkind.InvalidInput, 4, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			_ = requireRuleError(t, err, tc.kind, tc.line, tc.key)
		})
	}
}

func TestParseReportsUnknownKeysAndTypesWithAFix(t *testing.T) {
	cases := []struct {
		name    string
		content string
		line    int
		key     string
		hint    string
	}{
		{"typo in aliases", "version: 1\nkeys:\n  Environment:\n    aliasses: [env]\n", 4, "aliasses", `did you mean "aliases"?`},
		{"typo at the top", "version: 1\nkyes: {}\n", 2, "kyes", `did you mean "keys"?`},
		{"far from any key", "version: 1\nsettings: {}\n", 2, "settings", "allowed keys here: version, keys, required, remove"},
		{"number as alias", "version: 1\nkeys:\n  A:\n    aliases: [env, 123]\n", 4, "123", "put the value in quotes"},
		{"number as key", "version: 1\nkeys:\n  404:\n    aliases: [x]\n", 3, "404", "put the value in quotes"},
		{"aliases not a list", "version: 1\nkeys:\n  A:\n    aliases: env\n", 4, "", "one \"- item\" per line"},
		{"keys not a mapping", "version: 1\nkeys: [A]\n", 2, "", "canonical tag key"},
		{"rule not a mapping", "version: 1\nkeys:\n  A: [a]\n", 3, "", "aliases: [old-name]"},
		{"empty key", "version: 1\nkeys:\n  \"\":\n    aliases: [a]\n", 3, "", "remove the empty key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(tc.content))
			e := requireRuleError(t, err, errkind.InvalidInput, tc.line, tc.key)
			if !strings.Contains(e.Hint, tc.hint) {
				t.Fatalf("hint = %q, want it to contain %q", e.Hint, tc.hint)
			}
		})
	}
}

func TestAliasesBelongToOneCanonicalKey(t *testing.T) {
	_, err := parse("r.yaml", []byte("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n  Stage:\n    aliases: [env]\n"))
	e := requireRuleError(t, err, errkind.InvalidInput, 6, "env")
	if !strings.Contains(e.Detail, "line 4") || !strings.Contains(e.Detail, "line 6") {
		t.Fatalf("both lines must be named: %q", e.Detail)
	}

	_, err = parse("r.yaml", []byte("version: 1\nkeys:\n  Environment:\n    aliases: [Env]\n  Env:\n    aliases: [e]\n"))
	_ = requireRuleError(t, err, errkind.InvalidInput, 4, "Env")

	rs, err := parse("r.yaml", []byte("version: 1\nkeys:\n  Environment:\n    aliases: [env, env, ENV]\n  environment:\n    aliases: [x]\n"))
	if err != nil {
		t.Fatalf("case differences are allowed: %v", err)
	}
	if strings.Join(rs.KeyRules[0].Aliases, ",") != "env,ENV" {
		t.Fatalf("aliases = %v", rs.KeyRules[0].Aliases)
	}
	if len(rs.Warnings) != 2 || !strings.Contains(rs.Warnings[0].Message, "more than once") ||
		rs.Warnings[1].Line != 5 || !strings.Contains(rs.Warnings[1].Message, "differ only in case") {
		t.Fatalf("warnings = %+v", rs.Warnings)
	}
}

// U3: values, required and remove are read and no longer give a "not used"
// warning; only the aws: keys do (BR5.1).
func TestAWSKeysGiveWarnings(t *testing.T) {
	content := `version: 1
keys:
  aws:createdBy:
    aliases: [creator]
  Owner:
    aliases: [owner, AWS:owner]
    values:
      team-a: [TeamA, team_a]
required:
  CostCenter:
    default: "0000"
    when:
      - match: {resourceType: "ec2:instance", region: us-east-1, tags: {env: prod}}
        value: "1111"
remove: [tmp-owner]
`
	rs, err := parse("r.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.KeyRules) != 1 || rs.KeyRules[0].CanonicalKey != "Owner" || strings.Join(rs.KeyRules[0].Aliases, ",") != "owner" {
		t.Fatalf("key rules = %+v", rs.KeyRules)
	}
	var got []string
	for _, w := range rs.Warnings {
		got = append(got, w.Key)
	}
	if strings.Join(got, ",") != "aws:createdBy,AWS:owner" {
		t.Fatalf("warning keys = %v", got)
	}
}

// Review R-07: a file without key rules is accepted but warned about, so a
// forgotten or misplaced "keys" does not pass silently as "No changes needed".
func TestMissingOrEmptyKeysGiveAWarning(t *testing.T) {
	cases := []struct {
		name    string
		content string
		line    int
	}{
		{"no keys", "version: 1\n", 1},
		{"keys without a value", "version: 1\nkeys:\n", 2},
		{"empty keys", "version: 1\nremove: [x]\nkeys: {}\n", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs, err := parse("r.yaml", []byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			if len(rs.KeyRules) != 0 {
				t.Fatalf("key rules = %+v", rs.KeyRules)
			}
			last := rs.Warnings[len(rs.Warnings)-1]
			if last.Line != tc.line || last.Key != "keys" || !strings.HasPrefix(last.Message, "no key rules") {
				t.Fatalf("warnings = %+v", rs.Warnings)
			}
		})
	}
	// Keys that are all ignored (aws:) are already warned about one by one.
	rs, err := parse("r.yaml", []byte("version: 1\nkeys:\n  aws:x:\n    aliases: [y]\n"))
	if err != nil || len(rs.Warnings) != 1 || strings.Contains(rs.Warnings[0].Message, "no key rules") {
		t.Fatalf("warnings = %+v, err = %v", rs, err)
	}
}

func TestUnusedItemsAreStillChecked(t *testing.T) {
	cases := map[string]string{
		"values not a mapping":   "version: 1\nkeys:\n  A:\n    values: [a]\n",
		"value spellings":        "version: 1\nkeys:\n  A:\n    values:\n      a: b\n",
		"required not a mapping": "version: 1\nrequired: [a]\n",
		"required unknown key":   "version: 1\nrequired:\n  A:\n    defualt: x\n",
		"default not text":       "version: 1\nrequired:\n  A:\n    default: [x]\n",
		"when not a list":        "version: 1\nrequired:\n  A:\n    when: {match: {}}\n",
		"when without value":     "version: 1\nrequired:\n  A:\n    when:\n      - match: {region: us-east-1}\n",
		"when unknown key":       "version: 1\nrequired:\n  A:\n    when:\n      - match: {}\n        value: x\n        else: y\n",
		"match unknown key":      "version: 1\nrequired:\n  A:\n    when:\n      - match: {account: x}\n        value: y\n",
		"match tag value":        "version: 1\nrequired:\n  A:\n    when:\n      - match: {tags: {env: [a]}}\n        value: y\n",
		"remove not a list":      "version: 1\nremove: tmp\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parse("r.yaml", []byte(content))
			if errkind.KindOf(err) != errkind.InvalidInput {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if _, err := parse("r.yaml", []byte("version: 1\nkeys:\nrequired:\n  A:\n    when:\nremove:\n")); err != nil {
		t.Fatalf("empty sections are allowed: %v", err)
	}
}

// The rule file of the sandbox checkpoint must load without warnings.
func TestSandboxRuleFileIsValid(t *testing.T) {
	rs, err := load(filepath.Join("..", "..", "test", "sandbox", "u1-rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.KeyRules) != 1 || rs.KeyRules[0].CanonicalKey != "Environment" ||
		strings.Join(rs.KeyRules[0].Aliases, ",") != "env,Env" || len(rs.Warnings) != 0 {
		t.Fatalf("rule set = %+v", rs)
	}
}

// The two rule files of the U6 checkpoint load with no warning but the one
// that they have no key rules: the resource types are in the form scan shows
// (U6 cicd-pipeline.md).
func TestSandboxU6RuleFilesAreValid(t *testing.T) {
	for name, check := range map[string]func(*RuleSet) bool{
		"u6-add-rules.yaml": func(rs *RuleSet) bool {
			return len(rs.RequiredTags) == 1 && rs.RequiredTags[0].Key == "tag-refactor-u6-check"
		},
		"u6-remove-rules.yaml": func(rs *RuleSet) bool {
			return len(rs.RemoveKeys) == 1 && rs.RemoveKeys[0].Key == "tag-refactor-u6-check"
		},
	} {
		rs, err := load(filepath.Join("..", "..", "test", "sandbox", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !check(rs) || len(rs.Warnings) != 1 || rs.Warnings[0].Key != "keys" {
			t.Errorf("%s: rule set = %+v", name, rs)
		}
	}
}

// The rule files of the U7 checkpoint (construction/u7-rollback/
// infrastructure-design/cicd-pipeline.md) read without a warning other than
// the one for a file without key rules: the rename file renames the source
// key to the destination key and changes its value a to b (renameKey and
// changeValue), and the zero file matches by type and region, since a queue
// without tags cannot be matched by its tags.
func TestSandboxU7RuleFilesAreValid(t *testing.T) {
	for name, check := range map[string]func(*RuleSet) bool{
		"u7-src-rules.yaml": func(rs *RuleSet) bool {
			r := rs.RequiredTags
			return len(r) == 1 && r[0].Key == "tag-refactor-u7-src" && !r[0].HasDefault && len(r[0].When) == 5 &&
				r[0].When[0].Value == "a" && r[0].When[0].Tags["project"] == "tag-refactor-fixture"
		},
		"u7-rename-rules.yaml": func(rs *RuleSet) bool {
			k := rs.KeyRules
			return len(k) == 1 && k[0].CanonicalKey == "tag-refactor-u7-dst" && strings.Join(k[0].Aliases, ",") == "tag-refactor-u7-src" &&
				len(k[0].Values) == 1 && k[0].Values[0].CanonicalValue == "b" && strings.Join(k[0].Values[0].Spellings, ",") == "a" &&
				len(rs.RequiredTags) == 0 && len(rs.RemoveKeys) == 0
		},
		"u7-only-remove-rules.yaml": func(rs *RuleSet) bool {
			return len(rs.RemoveKeys) == 1 && rs.RemoveKeys[0].Key == "tag-refactor-u7-only"
		},
		"u7-zero-add-rules.yaml": func(rs *RuleSet) bool {
			r := rs.RequiredTags
			return len(r) == 1 && r[0].Key == "tag-refactor-u7-zero" && !r[0].HasDefault && len(r[0].When) == 1 &&
				r[0].When[0].ResourceType == "sqs" && r[0].When[0].Region == "ap-northeast-1" && len(r[0].When[0].Tags) == 0
		},
		"u7-cleanup-remove-rules.yaml": func(rs *RuleSet) bool {
			var keys []string
			for _, k := range rs.RemoveKeys {
				keys = append(keys, k.Key)
			}
			return strings.Join(keys, ",") == "tag-refactor-u6-check,tag-refactor-u7-src,tag-refactor-u7-dst,tag-refactor-u7-zero"
		},
	} {
		rs, err := load(filepath.Join("..", "..", "test", "sandbox", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		withKeys := len(rs.KeyRules) > 0
		if !check(rs) || withKeys && len(rs.Warnings) != 0 || !withKeys && (len(rs.Warnings) != 1 || rs.Warnings[0].Key != "keys") {
			t.Errorf("%s: rule set = %+v", name, rs)
		}
	}
}

func TestSuggestAndEditDistance(t *testing.T) {
	if got := suggest("Aliases", keyRuleKeys); got != "aliases" {
		t.Fatalf("suggest = %q", got)
	}
	if got := suggest("zzzzzzz", keyRuleKeys); got != "" {
		t.Fatalf("suggest = %q", got)
	}
	if editDistance("kitten", "sitting") != 3 || editDistance("", "abc") != 3 || editDistance("same", "same") != 0 {
		t.Fatal("edit distance is wrong")
	}
}

// U2 BR5.2: the required keys are kept in file order with their lines. Since
// U3 a required key starting with aws: is left out by the reader with a
// warning (BR5.1), instead of by scan.
func TestRequiredKeysAreKeptWithTheirLines(t *testing.T) {
	rs, err := parse("r.yaml", []byte("version: 1\nrequired:\n  Owner:\n    default: x\n  aws:createdBy:\n    default: y\n  Team: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rs.RequiredTags {
		got = append(got, fmt.Sprintf("%s@%d", r.Key, r.Line))
	}
	if strings.Join(got, ",") != "Owner@3,Team@7" {
		t.Fatalf("required = %v", got)
	}
	if len(rs.Warnings) != 2 || rs.Warnings[0].Line != 5 || !strings.Contains(rs.Warnings[0].Message, `the required tag "aws:createdBy" is ignored`) {
		t.Fatalf("warnings = %+v", rs.Warnings)
	}
	none, err := parse("r.yaml", []byte("version: 1\n"))
	if err != nil || len(none.RequiredTags) != 0 {
		t.Fatalf("required = %v, err %v", none.RequiredTags, err)
	}
}
