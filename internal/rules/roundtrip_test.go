package rules

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// NFR10.4: a draft reads back to the canonical keys, aliases and value
// spellings it was written from, whatever characters they hold. The random
// strings come from a fixed seed, so CI checks the same input every time.

// roundTripRunes are the characters the random strings are made of:
// letters, control characters (C0, DEL, C1), quotes, backslash, ":", "#",
// blanks, brackets, Japanese, format characters, line and paragraph
// separators, a byte order mark, private use, and characters outside the
// basic plane (an emoji and tag characters). They are given as numbers so
// this source holds no invisible character.
var roundTripRunes = []rune{
	'a', 'B', 'z', '0', '9', '-', '_', '.', '/', '=', '+', '@',
	0x00, 0x01, 0x07, 0x09, 0x0A, 0x0D, 0x1B, 0x1F, 0x7F, 0x85, 0x9B,
	'"', '\'', '\\', ':', '#', ' ', '[', ']', '{', '}', ',', '&', '*', '!', '|', '>', '%', '?', '-',
	0x3042, 0x74B0, 0x5883, 0x672C, 0x756A, 0x3000, 0x00A0,
	0x200B, 0x200E, 0x202E, 0x2066, 0x2028, 0x2029, 0xFEFF, 0xE000, 0xFFFD,
	0x1F600, 0xE0001, 0xE0041, 0xE007F, 0x1D173,
}

// randomText returns n random characters. The first character is a letter,
// so a key never starts with "aws:".
func randomText(r *rand.Rand, n int) string {
	var b strings.Builder
	b.WriteRune('k')
	for i := 1; i < n; i++ {
		b.WriteRune(roundTripRunes[r.IntN(len(roundTripRunes))])
	}
	return b.String()
}

type roundTripKey struct {
	spellings []string // canonical first, then aliases by count
	values    [][]string
}

func TestRoundTripOfRandomKeysAndValues(t *testing.T) {
	r := rand.New(rand.NewPCG(20261024, 3))
	seen := map[string]bool{}
	unique := func(n int) string {
		for {
			s := randomText(r, n)
			if !seen[s] {
				seen[s] = true
				return s
			}
		}
	}
	group := map[string]string{}
	var keys []roundTripKey
	for i := 0; i < 16; i++ {
		keyLen := 1 + r.IntN(MaxKeyLength)
		valueLen := r.IntN(MaxValueLength)
		if i == 0 {
			keyLen, valueLen = MaxKeyLength, MaxValueLength // the limits themselves
		}
		k := roundTripKey{spellings: []string{unique(keyLen), unique(1 + r.IntN(MaxKeyLength))}}
		if i%3 == 0 {
			k.spellings = append(k.spellings, unique(1+r.IntN(40)))
		}
		for _, s := range k.spellings {
			group[s] = fmt.Sprint("key", i)
		}
		for v := 0; v < 1+i%2; v++ {
			values := []string{unique(max(1, valueLen)), unique(1 + r.IntN(60))}
			if i == 0 {
				// 256 characters that are all escaped: far past the 1024
				// characters YAML allows for an implicit key.
				values[0] = "v" + strings.Repeat(string(rune(0xE0041)), MaxValueLength-1)
			}
			for _, s := range values {
				group[s] = fmt.Sprint("key", i, "value", v)
			}
			k.values = append(k.values, values)
		}
		keys = append(keys, k)
	}

	// The canonical spelling of each key and value is on more resources than
	// the others, so the draft's choice is known.
	var resources []TaggedResource
	arn := 0
	add := func(key, value string, times int) {
		for n := 0; n < times; n++ {
			arn++
			resources = append(resources, TaggedResource{ARN: fmt.Sprint("arn:", arn), Tags: map[string]string{key: value}})
		}
	}
	for _, k := range keys {
		for vi, values := range k.values {
			add(k.spellings[0], values[0], 4+vi*10)
			add(k.spellings[0], values[1], 1)
		}
		for si, s := range k.spellings[1:] {
			add(s, k.values[0][0], 3-si)
		}
	}
	in := SuggestInput{Resources: resources, KeyGroup: func(s string) string { return group[s] }, GeneratedAt: draftTime}

	d := Suggest(in)
	if d.TooLarge {
		t.Fatalf("the draft is %d bytes; make the test input smaller", len(d.Text))
	}
	rs, err := Parse("draft.yaml", d.Text, Options{})
	if err != nil {
		t.Fatalf("the draft does not read back: %v", err)
	}

	if len(rs.KeyRules) != len(keys) {
		t.Fatalf("%d keys read back, want %d", len(rs.KeyRules), len(keys))
	}
	byKey := map[string]KeyRule{}
	for _, k := range rs.KeyRules {
		byKey[k.CanonicalKey] = k
	}
	for i, k := range keys {
		got, ok := byKey[k.spellings[0]]
		if !ok {
			t.Fatalf("key %d (%q) did not read back", i, k.spellings[0])
		}
		if !slices.Equal(got.Aliases, k.spellings[1:]) {
			t.Fatalf("key %d: aliases %q, want %q", i, got.Aliases, k.spellings[1:])
		}
		var want []ValueRule
		for vi := len(k.values) - 1; vi >= 0; vi-- { // by resources, highest first
			want = append(want, ValueRule{CanonicalValue: k.values[vi][0], Spellings: []string{k.values[vi][1]}})
		}
		if len(got.Values) != len(want) {
			t.Fatalf("key %d: %d values, want %d", i, len(got.Values), len(want))
		}
		for vi := range want {
			if got.Values[vi].CanonicalValue != want[vi].CanonicalValue || !slices.Equal(got.Values[vi].Spellings, want[vi].Spellings) {
				t.Fatalf("key %d: value %d is %+q, want %+q", i, vi, got.Values[vi].Spellings, want[vi].Spellings)
			}
		}
	}
	if n := utf8.RuneCountInString(keys[0].spellings[0]); n != MaxKeyLength {
		t.Fatalf("the longest key has %d characters", n)
	}
	// YAML limits implicit keys to 1024 characters; the escaped values used
	// as keys under "values" go beyond that and still read back.
	longest := 0
	for _, line := range strings.Split(string(d.Text), "\n") {
		if strings.HasPrefix(line, "      \"") {
			longest = max(longest, strings.Index(line, "\": ["))
		}
	}
	if longest <= 1024 {
		t.Fatalf("the longest value key has %d bytes; the input must reach past 1024", longest)
	}
	for _, c := range []string{"\x1b", "\xc2\x9b", "\xe2\x80\xae", "\xf3\xa0\x80\x81", "\xe2\x80\xa8", "\x00"} {
		if strings.Contains(string(d.Text), c) {
			t.Fatalf("the draft holds %q as it is", c)
		}
	}
}
