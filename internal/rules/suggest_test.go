package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Unit tests of the draft of rules suggest (BR6.1 to BR6.5, BR6.7, NFR8.12,
// NFR9.4), written after the implementation. The grouping uses both
// inventory.CanonicalKey, as rules suggest does, and a simple test function.

var draftTime = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

// dictionaryPreferred is what rules suggest passes as Preferred: the full
// words of the abbreviation dictionary.
func dictionaryPreferred(group string) string {
	for _, full := range inventory.Abbreviations() {
		if full == group {
			return group
		}
	}
	return ""
}

func suggestInput(resources ...TaggedResource) SuggestInput {
	return SuggestInput{
		Resources: resources, KeyGroup: inventory.CanonicalKey, Preferred: dictionaryPreferred,
		ToolVersion: "v0.1.0", AccountID: "123456789012", RegionsRead: 17, RegionsEnabled: 17, GeneratedAt: draftTime,
	}
}

func res(arn string, tags ...string) TaggedResource {
	m := map[string]string{}
	for i := 0; i+1 < len(tags); i += 2 {
		m[tags[i]] = tags[i+1]
	}
	return TaggedResource{ARN: arn, Tags: m}
}

func expectedDraftInput() SuggestInput {
	resources := []TaggedResource{
		res("arn:a", "env", "prod", "Owner", "team-a", "Name", "web-1"),
		res("arn:b", "env", "Prod", "Name", "web-2"),
		res("arn:c", "env", "production", "cost-center", "1"),
		res("arn:d", "Env", "dev", "cost_center", "1"),
		res("arn:e", "Env", "prod"),
		res("arn:f", "environment", "stg", "aws:cloudformation:stack-name", "s"),
		res("arn:g", "Cost Center", "1"),
	}
	for i := 1; i <= 21; i++ {
		resources = append(resources, res(fmt.Sprintf("arn:h%02d", i), "id", fmt.Sprintf("i-%02d", i)))
	}
	return suggestInput(resources...)
}

// NFR9.4: the same collection gives the same bytes on every OS, compared
// with an expected file kept with LF newlines (.gitattributes).
func TestSuggestWritesTheExpectedDraft(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "suggest", "draft.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ { // map order must not matter
		d := Suggest(expectedDraftInput())
		if string(d.Text) != string(want) {
			t.Fatalf("draft:\n%s\nwant:\n%s", d.Text, want)
		}
		if d.TooLarge || len(d.Keys) != 5 || d.Keys[0].ValuesOmitted != OmitTooManyValues || d.Keys[1].ResourceCount != 6 {
			t.Fatalf("keys = %+v", d.Keys)
		}
	}
	rs, err := parse("draft.yaml", want)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Warnings) != 0 || len(rs.KeyRules) != 5 || rs.KeyRules[1].CanonicalKey != "env" ||
		!reflect.DeepEqual(rs.KeyRules[1].Values, []ValueRule{{CanonicalValue: "prod", Spellings: []string{"Prod", "production"}, Line: 11}}) {
		t.Fatalf("the draft read back as %+v", rs)
	}
}

// BR6.1: the most used spelling is the canonical key; on a tie the full word
// of the dictionary wins, then the byte order. Aliases follow by count, then
// by name.
func TestTheCanonicalKeyIsTheMostUsedSpelling(t *testing.T) {
	cases := []struct {
		name      string
		resources []TaggedResource
		canonical string
		aliases   []string
	}{
		{"most used wins over the full word", []TaggedResource{res("1", "env", "a"), res("2", "env", "a"), res("3", "Environment", "a")},
			"env", []string{"Environment"}},
		{"on a tie the full word wins", []TaggedResource{res("1", "env", "a"), res("2", "Environment", "a")},
			"Environment", []string{"env"}},
		{"then the byte order", []TaggedResource{res("1", "team", "a"), res("2", "Team", "a"), res("3", "TEAM", "a")},
			"TEAM", []string{"Team", "team"}},
		{"aliases by count", []TaggedResource{res("1", "ab", "x"), res("2", "ab", "x"), res("3", "ab", "x"),
			res("4", "a-b", "x"), res("5", "A_B", "x"), res("6", "A_B", "x")},
			"ab", []string{"A_B", "a-b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Suggest(suggestInput(tc.resources...))
			if len(d.Keys) != 1 || d.Keys[0].CanonicalKey != tc.canonical || !reflect.DeepEqual(d.Keys[0].Aliases, tc.aliases) {
				t.Fatalf("keys = %+v", d.Keys)
			}
		})
	}
	// With a simple grouping function and no dictionary, only counts and
	// names decide; a resource with two spellings counts once for the key.
	in := SuggestInput{Resources: []TaggedResource{res("1", "Env", "a", "ENV", "a"), res("2", "Env", "b"), res("1", "Env", "dup")},
		KeyGroup: strings.ToLower, GeneratedAt: draftTime}
	d := Suggest(in)
	if len(d.Keys) != 1 || d.Keys[0].CanonicalKey != "Env" || d.Keys[0].ResourceCount != 2 {
		t.Fatalf("keys = %+v", d.Keys)
	}
}

// BR6.2 and BR6.5: every key but aws: ones is in the draft; with none, the
// keys are empty and the draft still loads.
func TestEveryKeyIsInTheDraftAndNoKeysGiveEmptyKeys(t *testing.T) {
	d := Suggest(suggestInput(res("1", "single", "x", "aws:createdBy", "me", "AWS:Other", "y")))
	if len(d.Keys) != 1 || d.Keys[0].CanonicalKey != "single" || !strings.Contains(string(d.Text), "\n  \"single\": {}\n") {
		t.Fatalf("draft:\n%s", d.Text)
	}
	for _, in := range []SuggestInput{suggestInput(), suggestInput(res("1", "aws:only", "x"))} {
		d := Suggest(in)
		if !strings.Contains(string(d.Text), "\nversion: 1\nkeys: {}\n# required:\n") || len(d.Keys) != 0 {
			t.Fatalf("draft:\n%s", d.Text)
		}
		if _, err := parse("draft.yaml", d.Text); err != nil {
			t.Fatal(err)
		}
	}
}

// BR6.3: value groups with two or more spellings are suggested, up to 20
// different values per key; a value is counted once per resource.
func TestValueCandidates(t *testing.T) {
	var twenty, twentyOne []TaggedResource
	for i := 0; i < 21; i++ {
		v := fmt.Sprintf("v%02d", i/2)
		if i%2 == 1 {
			v = strings.ToUpper(v)
		}
		r := res(fmt.Sprint(i), "k", v, "Ali-as", v)
		if i < 20 {
			twenty = append(twenty, r)
		}
		twentyOne = append(twentyOne, res(fmt.Sprint(i), "k", fmt.Sprintf("x%02d", i), "K", "y"))
	}
	d := Suggest(suggestInput(twenty...))
	var k SuggestedKey
	for _, key := range d.Keys {
		if key.CanonicalKey == "k" {
			k = key
		}
	}
	if k.ValuesOmitted != OmitNone || len(k.Values) != 10 || k.Values[0].CanonicalValue != "V00" || !reflect.DeepEqual(k.Values[0].Spellings, []string{"v00"}) {
		t.Fatalf("twenty values: %+v", k)
	}
	d = Suggest(suggestInput(twentyOne...))
	if d.Keys[0].ValuesOmitted != OmitTooManyValues || len(d.Keys[0].Values) != 0 ||
		!strings.Contains(string(d.Text), "  \"K\":\n    aliases: [\"k\"]\n    # values not suggested: more than 20 different values\n") {
		t.Fatalf("twenty-one values:\n%s", d.Text)
	}
	// Spellings of one value on two key spellings of one resource count once,
	// and a value with one spelling is not suggested.
	d = Suggest(suggestInput(res("1", "env", "prod", "Env", "prod"), res("2", "env", "Prod"), res("3", "env", "dev")))
	if !reflect.DeepEqual(d.Keys[0].Values, []ValueRule{{CanonicalValue: "Prod", Spellings: []string{"prod"}}}) {
		t.Fatalf("values = %+v", d.Keys[0].Values)
	}
	// On a tie the full word wins as the canonical value.
	d = Suggest(suggestInput(res("1", "e", "prod"), res("2", "e", "Production")))
	if d.Keys[0].Values[0].CanonicalValue != "Production" {
		t.Fatalf("values = %+v", d.Keys[0].Values)
	}
}

// BR6.5 and security design §3: the first comments hold only text of the
// expected form.
func TestTheFirstCommentsHoldOnlyFixedForms(t *testing.T) {
	in := suggestInput(res("1", "a", "b"))
	in.FailedRegions = []string{"eu-west-1", "bad\nregion"}
	in.ToolVersion = "v1\n# injected"
	in.AccountID = "1234"
	in.RegionsRead = 15
	d := Suggest(in)
	head := strings.SplitN(string(d.Text), "\nversion: 1\n", 2)[0]
	want := "# generated by tag-refactor unknown\n" +
		"# account unknown, 15 of 17 regions, 2026-10-24T09:00:00Z\n" +
		"# Results are incomplete: eu-west-1, unknown-region\n" +
		"# Review each key: the most used spelling is proposed as the canonical key\n" +
		"# and the other spellings as its aliases."
	if head != want {
		t.Fatalf("head:\n%s\nwant:\n%s", head, want)
	}
}

// NFR8.12 and BR6.4: keys and values are double-quoted; what is not graphic
// is escaped, so nothing in the draft rewrites a terminal.
func TestQuoteEscapesWhatCouldRewriteTheTerminal(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", `"plain"`},
		{`a"b\c`, `"a\"b\\c"`},
		{"a: #b c", `"a: #b c"`},
		{"環境 本番", `"環境 本番"`},
		{"esc\x1b[2J", `"esc~u001B~u005B2J"`},
		{"csi\xc2\x9b", `"csi\u009B"`},
		{"rlo\xe2\x80\xae", `"rlo~u202E"`},
		{"ls\xe2\x80\xa8", `"ls~u2028"`},
		{"tab\tnl\n", `"tab\u0009nl\u000A"`},
		{"tag\xf3\xa0\x80\x81\xf3\xa0\x81\x81", `"tag\U000E0001\U000E0041"`},
		{"bad\xff", "\"bad\xef\xbf\xbd\""},
		{"{x}[y]", `"~u007Bx}~u005By]"`},
		{"emoji\xf0\x9f\x98\x80", "\"emoji\xf0\x9f\x98\x80\""},
		{"private\xee\x80\x80", `"private~uE000"`},
	}
	for _, tc := range cases {
		// "~u" and "~U" stand for the escapes, so this source holds none.
		want := strings.NewReplacer("~u", `\`+"u", "~U", `\`+"U").Replace(tc.want)
		if got := quote(tc.in); got != want {
			t.Errorf("quote(%q) = %s, want %s", tc.in, got, want)
		}
	}
}

// Keys that differ only in bytes that are not UTF-8 become one key, so the
// draft never has a duplicate key.
func TestInvalidUTF8KeysBecomeOneKey(t *testing.T) {
	d := Suggest(suggestInput(res("1", "a\xff", "x"), res("2", "a\xfe", "x"), res("3", "a\xef\xbf\xbd", "x")))
	if len(d.Keys) != 1 || d.Keys[0].CanonicalKey != "a�" || d.Keys[0].ResourceCount != 3 {
		t.Fatalf("keys = %+v", d.Keys)
	}
	if _, err := parse("draft.yaml", d.Text); err != nil {
		t.Fatal(err)
	}
}

// bigDraftInput is an account whose draft is over 64 KiB with its value
// candidates: keys on 20 to 32 resources, each with ten values written in
// two ways, and an alias on half of them.
func bigDraftInput(keys int) SuggestInput {
	var resources []TaggedResource
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("team-key-%04d", i)
		for j := 0; j < 20+i%13; j++ {
			v := fmt.Sprintf("value-number-%02d", j%10)
			if j%20 >= 10 {
				v = strings.ToUpper(v)
			}
			r := res(fmt.Sprintf("arn:%d:%d", i, j), key, v)
			if i%2 == 0 && j == 0 {
				r = res(fmt.Sprintf("arn:%d:%d", i, j), strings.ToUpper(key), v)
			}
			resources = append(resources, r)
		}
	}
	return suggestInput(resources...)
}

// BR6.7: a draft over 64 KiB drops the value candidates of the keys with the
// fewest resources first, says so, and its real size fits.
func TestTheDraftIsKeptWithin64KiB(t *testing.T) {
	in := bigDraftInput(300)
	full := fullDraftSize(in)
	if full <= MaxFileSize {
		t.Fatalf("the input must give a draft over 64 KiB, got %d bytes", full)
	}
	d := Suggest(in)
	if d.TooLarge || len(d.Text) > MaxFileSize {
		t.Fatalf("draft of %d bytes, too large = %v", len(d.Text), d.TooLarge)
	}
	maxOmitted, minKept, omitted, kept := 0, 1<<30, 0, 0
	for _, k := range d.Keys {
		switch k.ValuesOmitted {
		case OmitSizeLimit:
			omitted++
			maxOmitted = max(maxOmitted, k.ResourceCount)
		case OmitNone:
			kept++
			minKept = min(minKept, k.ResourceCount)
		}
	}
	if omitted == 0 || kept == 0 || maxOmitted > minKept {
		t.Fatalf("omitted %d (most resources %d), kept %d (fewest resources %d)", omitted, maxOmitted, kept, minKept)
	}
	text := string(d.Text)
	if !strings.Contains(text, "    # values omitted: the draft would exceed 64 KiB\n") ||
		!strings.Contains(text, "\": {}  # values omitted: the draft would exceed 64 KiB\n") {
		t.Fatal("the omitted values are not explained")
	}
	rs, err := parse("draft.yaml", d.Text)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.KeyRules) != 300 {
		t.Fatalf("%d keys read back", len(rs.KeyRules))
	}
}

// fullDraftSize is the size of the draft with every value candidate.
func fullDraftSize(in SuggestInput) int {
	n := len(draftHeader(in, false)) + len(draftRequiredRemove)
	for name, g := range collectGroups(in.Resources, in.KeyGroup) {
		n += len(renderKey(g.suggest(name, in.KeyGroup, in.Preferred)))
	}
	return n
}

// BR6.7: when the keys alone are over 64 KiB, the draft is still written
// and marked too large.
func TestKeysAloneOver64KiBMakeTheDraftTooLarge(t *testing.T) {
	tags := map[string]string{}
	for i := 0; i < 2000; i++ {
		tags[fmt.Sprintf("key-%04d-%s", i, strings.Repeat("x", 30))] = "v"
	}
	d := Suggest(suggestInput(TaggedResource{ARN: "arn:1", Tags: tags}))
	if !d.TooLarge || len(d.Text) <= MaxFileSize || len(d.Keys) != 2000 {
		t.Fatalf("too large = %v, %d bytes, %d keys", d.TooLarge, len(d.Text), len(d.Keys))
	}
}
