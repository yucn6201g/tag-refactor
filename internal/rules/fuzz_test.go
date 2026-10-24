package rules

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// FuzzParse checks that reading a rule file never stops the tool (NFR8.13):
// whatever the input, Parse does not panic, returns within one second (ten
// under the race detector), reports an error on a line of the file, and
// returns only rule sets that keep the rules of the reader. In CI only the
// seeds run, as a normal test; the files under testdata/fuzz/FuzzParse add
// to the seeds below. Run the fuzzing itself locally with
//
//	go test -run '^$' -fuzz FuzzParse -fuzztime 5m ./internal/rules/
func FuzzParse(f *testing.F) {
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}
	limit := time.Second * timeLimitFactor
	f.Fuzz(func(t *testing.T, data []byte) {
		start := time.Now()
		rs, err := Parse("fuzz.yaml", data, Options{})
		if elapsed := time.Since(start); elapsed > limit {
			t.Fatalf("Parse took %v for %d bytes", elapsed, len(data))
		}
		if err != nil {
			checkFuzzError(t, data, err)
			return
		}
		checkRuleSet(t, rs)
	})
}

// fuzzSeeds are valid files (U1 and U3 examples, a draft), every error
// example of the functional design, deep nesting, long lines, invalid UTF-8
// and UTF-16 with and without a byte order mark.
func fuzzSeeds(tb testing.TB) [][]byte {
	draft, err := os.ReadFile(filepath.Join("testdata", "suggest", "draft.yaml"))
	if err != nil {
		tb.Fatal(err)
	}
	texts := []string{
		fullRules,
		queryRules,
		japaneseRules,
		aliasRulesU1,
		string(draft),
		string(Suggest(expectedDraftInput()).Text),
		// The error examples of the functional design.
		"version: 1\nkeys:\n  Environment:\n    aliases: [env]\nrequried:\n  Owner: {default: x}\n",
		"version: 1\nkeys:\n  Environment:\n    aliases: env\n",
		"version: 1\nkeys:\n  Environment:\n    aliases: [env]\n  Stage:\n    aliases: [env]\n",
		"version: 1\nrequired:\n  Owner:\n    when:\n      - match: {resourceType: AWS::EC2::Instance}\n        value: x\n",
		"version: 1\nrequired:\n  Owner:\n    when:\n      - match: {resourceType: AWS::Example::Thing}\n        value: x\n",
		envKeys + "required:\n  Owner:\n    when:\n      - match: {tags: {env: prod}}\n        value: x\n",
		"version: 1\nrequired:\n  Owner:\n    when:\n      - match: {}\n        value: x\n",
		envKeys + "remove: [env]\n",
		"version: 2\nkeys: {}\n",
		// The structure the reader rejects.
		"version: 1\nkeys:\n  A: &a {aliases: [b]}\n  B: *a\n",
		"version: 1\nkeys:\n  A:\n    <<: {aliases: [a]}\n",
		"version: 1\n---\nversion: 1\n",
		"version: 1\nremove: " + strings.Repeat("[", 5000) + strings.Repeat("]", 5000) + "\n",
		"version: 1\nremove:\n" + strings.Repeat("- ", 5000) + "x\n",
		"version: 1\nremove: [\"" + strings.Repeat("{", 2000) + "\"]\n",
		"version: 1\nkeys:\n  \"" + strings.Repeat("k", 60_000) + "\": {}\n",
		"version: 1\n#" + strings.Repeat("x", MaxFileSize-len("version: 1\n#")),
		"version: 1\nkeys:\n  \"a\xff\": {}\n",
		"\xef\xbb\xbfversion: 1\r\nkeys: {A: {aliases: [a]}}\r\n",
	}
	seeds := make([][]byte, 0, len(texts)+6)
	for _, s := range texts {
		seeds = append(seeds, []byte(s))
	}
	return append(seeds,
		encodeUTF16(japaneseRules, binary.LittleEndian, true),
		encodeUTF16(japaneseRules, binary.BigEndian, true),
		encodeUTF16(fullRules, binary.LittleEndian, false),
		encodeUTF16(japaneseRules, binary.BigEndian, false),
		append(encodeUTF16(fullRules, binary.LittleEndian, true), 0x41),
		[]byte{0xFF, 0xFE, 0x00, 0xD8, '\n', 0x00},
	)
}

const aliasRulesU1 = "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n"

// checkFuzzError checks that an error is a rule file error with a fix, on a
// line of the file; only the size error has no line.
func checkFuzzError(t *testing.T, data []byte, err error) {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok || (e.Kind != errkind.InvalidInput && e.Kind != errkind.UnsupportedVersion) || e.Detail == "" || e.Hint == "" {
		t.Fatalf("not a rule file error with a fix: %#v", err)
	}
	if e.Line == 0 {
		if e.Detail != tooLargeDetail {
			t.Fatalf("an error without a line: %q", e.Detail)
		}
		return
	}
	lines := 1
	if text, decodeErr := (&loader{}).decode(data); decodeErr == nil {
		lines = strings.Count(normalizeNewlines(text), "\n") + 1
	}
	if e.Line < 1 || e.Line > lines {
		t.Fatalf("error on line %d of a file of %d lines: %s", e.Line, lines, e.Detail)
	}
}

// checkRuleSet checks the rules every rule set Parse returns keeps.
func checkRuleSet(t *testing.T, rs *RuleSet) {
	t.Helper()
	if rs.Version != SupportedVersion {
		t.Fatalf("version %d", rs.Version)
	}
	owner := map[string]string{}
	canonical := map[string]bool{}
	for _, k := range rs.KeyRules {
		checkKeyText(t, k.CanonicalKey)
		canonical[k.CanonicalKey] = true
	}
	for _, k := range rs.KeyRules {
		for _, a := range k.Aliases {
			checkKeyText(t, a)
			if canonical[a] || owner[a] != "" {
				t.Fatalf("alias %q of %q is a canonical key or another key's alias", a, k.CanonicalKey)
			}
			owner[a] = k.CanonicalKey
		}
		values := map[string]bool{}
		for _, v := range k.Values {
			values[v.CanonicalValue] = true
		}
		spellings := map[string]bool{}
		for _, v := range k.Values {
			for _, s := range v.Spellings {
				if values[s] || spellings[s] || utf8.RuneCountInString(s) > MaxValueLength {
					t.Fatalf("spelling %q of %q breaks BR2.2", s, k.CanonicalKey)
				}
				spellings[s] = true
			}
		}
	}
	removed := map[string]bool{}
	for _, r := range rs.RemoveKeys {
		checkKeyText(t, r.Key)
		if removed[r.Key] || canonical[r.Key] || owner[r.Key] != "" {
			t.Fatalf("removed key %q breaks BR4.1", r.Key)
		}
		removed[r.Key] = true
	}
	for _, req := range rs.RequiredTags {
		checkKeyText(t, req.Key)
		if owner[req.Key] != "" || removed[req.Key] {
			t.Fatalf("required key %q breaks BR3.1", req.Key)
		}
		for _, c := range req.When {
			if c.ResourceType == "" && c.Region == "" && len(c.Tags) == 0 {
				t.Fatalf("an empty condition of %q", req.Key)
			}
			for k, v := range c.Tags {
				if owner[k] != "" || removed[k] || rs.CanonicalValue(k, v).Changed {
					t.Fatalf("condition tag %q=%q of %q breaks BR3.6", k, v, req.Key)
				}
			}
		}
	}
}

func checkKeyText(t *testing.T, key string) {
	t.Helper()
	if key == "" || utf8.RuneCountInString(key) > MaxKeyLength || IsAWSReservedKey(key) {
		t.Fatalf("key %q breaks BR1.5 or BR5.1", key)
	}
}
