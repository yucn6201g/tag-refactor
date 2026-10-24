package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxSuggestedValues is the largest number of different values a key may
// have for its values to be suggested (BR6.3). Keys such as names and IDs
// have more, and grouping them helps nobody.
const MaxSuggestedValues = 20

// TaggedResource is one resource given to Suggest: its ARN and all its tags.
type TaggedResource struct {
	ARN  string
	Tags map[string]string
}

// SuggestInput is what Suggest needs (contract C8). It holds only plain data
// and functions, so the rules package does not depend on the inventory.
type SuggestInput struct {
	Resources []TaggedResource
	// KeyGroup returns the grouping key of a tag key or value: keys (and
	// values) with the same grouping key are spellings of one key (value).
	// rules suggest passes inventory.CanonicalKey. Nil means NormalizeKey.
	KeyGroup func(key string) string
	// Preferred returns the full word of the abbreviation dictionary for a
	// grouping key, or "" when the group has none. A spelling that normalizes
	// to it wins a tie (BR6.1). Nil means no dictionary.
	Preferred func(group string) string

	// The facts of the first comments (BR6.5). They are written only when
	// they have the expected form; nothing read from AWS reaches a comment
	// otherwise (security design §3).
	ToolVersion    string
	AccountID      string
	RegionsRead    int
	RegionsEnabled int
	FailedRegions  []string
	GeneratedAt    time.Time
}

// OmitReason says why a key has no value candidates.
type OmitReason string

// Reasons for SuggestedKey.ValuesOmitted.
const (
	OmitNone          OmitReason = ""
	OmitTooManyValues OmitReason = "tooManyValues"
	OmitSizeLimit     OmitReason = "sizeLimit"
)

// SuggestedKey is one group of keys in the draft.
type SuggestedKey struct {
	// CanonicalKey is the spelling used by the most resources (BR6.1).
	CanonicalKey string
	// Aliases are the other spellings, by resources, then by name.
	Aliases []string
	// ResourceCount counts the resources with any spelling of the key.
	ResourceCount int
	// Values are the value groups with two or more spellings (BR6.3).
	Values        []ValueRule
	ValuesOmitted OmitReason
}

// Draft is the result of Suggest.
type Draft struct {
	// Keys are by resources, highest first, then by canonical key.
	Keys []SuggestedKey
	// Text is the draft rule file: UTF-8 with LF newlines (NFR9.4).
	Text []byte
	// TooLarge is true when the draft is larger than MaxFileSize even
	// without any value candidate, so it cannot be read as it is (BR6.7).
	TooLarge bool
}

// The comments of the draft.
const (
	draftReview1        = "# Review each key: the most used spelling is proposed as the canonical key\n"
	draftReview2        = "# and the other spellings as its aliases.\n"
	tooManyValuesNote   = "values not suggested: more than 20 different values"
	sizeLimitNote       = "values omitted: the draft would exceed 64 KiB"
	draftRequiredRemove = "# required:\n" +
		"#   \"Owner\":\n" +
		"#     default: \"platform-team\"\n" +
		"#     when:\n" +
		"#       - match: { resourceType: \"ec2:instance\" }\n" +
		"#         value: \"infra-team\"\n" +
		"# remove: [\"tmp-owner\"]\n"
)

var (
	accountPattern = regexp.MustCompile(`^[0-9]{12}$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z.+_-]{1,64}$`)
)

// Suggest builds the draft rule file of rules suggest (BR6.1 to BR6.7). It
// reads each resource once; sorting is done per group, and the 64 KiB
// adjustment works on byte counts (NFR1.11).
func Suggest(in SuggestInput) *Draft {
	keyGroup := in.KeyGroup
	if keyGroup == nil {
		keyGroup = NormalizeKey
	}
	groups := collectGroups(in.Resources, keyGroup)
	keys := make([]SuggestedKey, 0, len(groups))
	for name, g := range groups {
		keys = append(keys, g.suggest(name, keyGroup, in.Preferred))
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ResourceCount != keys[j].ResourceCount {
			return keys[i].ResourceCount > keys[j].ResourceCount
		}
		return keys[i].CanonicalKey < keys[j].CanonicalKey
	})
	head, foot := draftHeader(in, len(keys) == 0), draftRequiredRemove
	text := fitDraft(keys, head, foot)
	return &Draft{Keys: keys, Text: []byte(text), TooLarge: len(text) > MaxFileSize}
}

// keyGroup collects one group of keys while the resources are read.
type keyGroup struct {
	resources int
	// spellings counts the resources of each spelling of the key.
	spellings map[string]int
	// values counts the resources of each value of any spelling; nil once
	// the key has more than MaxSuggestedValues values.
	values map[string]int
}

func collectGroups(resources []TaggedResource, group func(string) string) map[string]*keyGroup {
	groups := map[string]*keyGroup{}
	seenARN := make(map[string]bool, len(resources))
	for _, r := range resources {
		if seenARN[r.ARN] {
			continue
		}
		seenARN[r.ARN] = true
		// counted holds the groups and the group values this resource was
		// already counted for, so a resource counts once per group and value.
		counted := make(map[string]bool, 2*len(r.Tags))
		for rawKey, rawValue := range r.Tags {
			key, value := validText(rawKey), validText(rawValue)
			if IsAWSReservedKey(key) || key == "" || utf8.RuneCountInString(key) > MaxKeyLength {
				continue
			}
			name := group(key)
			g := groups[name]
			if g == nil {
				g = &keyGroup{spellings: map[string]int{}, values: map[string]int{}}
				groups[name] = g
			}
			g.spellings[key]++
			if !counted[name] {
				counted[name] = true
				g.resources++
			}
			valueMark := name + "\x00" + value
			if g.values == nil || counted[valueMark] || utf8.RuneCountInString(value) > MaxValueLength {
				continue
			}
			counted[valueMark] = true
			g.values[value]++
			if len(g.values) > MaxSuggestedValues {
				g.values = nil
			}
		}
	}
	return groups
}

// validText replaces bytes that are not UTF-8 with U+FFFD before anything
// else, so two keys that differ only in such bytes become one key and the
// draft has no duplicate keys (NFR8.12).
func validText(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, string(utf8.RuneError))
}

type spelling struct {
	text  string
	count int
	// official is true for the full word of the abbreviation dictionary.
	official bool
}

// rank sorts by count, highest first; then the official spelling; then by
// name (BR6.1, BR6.3).
func rank(items []spelling) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if a.official != b.official {
			return a.official
		}
		return a.text < b.text
	})
}

func isOfficial(s, group string, preferred func(string) string) bool {
	if preferred == nil {
		return false
	}
	p := preferred(group)
	return p != "" && NormalizeKey(s) == p
}

func (g *keyGroup) suggest(name string, group, preferred func(string) string) SuggestedKey {
	spellings := make([]spelling, 0, len(g.spellings))
	for s, n := range g.spellings {
		spellings = append(spellings, spelling{text: s, count: n, official: isOfficial(s, name, preferred)})
	}
	rank(spellings)
	k := SuggestedKey{CanonicalKey: spellings[0].text, Aliases: []string{}, ResourceCount: g.resources, Values: []ValueRule{}}
	aliases := spellings[1:]
	for i := range aliases {
		aliases[i].official = false
	}
	rank(aliases)
	for _, a := range aliases {
		k.Aliases = append(k.Aliases, a.text)
	}
	if g.values == nil {
		k.ValuesOmitted = OmitTooManyValues
		return k
	}
	k.Values = valueRules(g.values, group, preferred)
	return k
}

// valueRules groups the values of one key the way keys are grouped and keeps
// the groups with two or more spellings (BR6.3).
func valueRules(values map[string]int, group, preferred func(string) string) []ValueRule {
	byGroup := map[string][]spelling{}
	for v, n := range values {
		g := group(v)
		byGroup[g] = append(byGroup[g], spelling{text: v, count: n, official: isOfficial(v, g, preferred)})
	}
	type ruleWithCount struct {
		rule  ValueRule
		total int
	}
	var rules []ruleWithCount
	for _, items := range byGroup {
		if len(items) < 2 {
			continue
		}
		rank(items)
		total := 0
		for _, it := range items {
			total += it.count
		}
		rest := items[1:]
		for i := range rest {
			rest[i].official = false
		}
		rank(rest)
		r := ValueRule{CanonicalValue: items[0].text, Spellings: make([]string, 0, len(rest))}
		for _, it := range rest {
			r.Spellings = append(r.Spellings, it.text)
		}
		rules = append(rules, ruleWithCount{rule: r, total: total})
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].total != rules[j].total {
			return rules[i].total > rules[j].total
		}
		return rules[i].rule.CanonicalValue < rules[j].rule.CanonicalValue
	})
	out := make([]ValueRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.rule)
	}
	return out
}

// draftHeader writes the first comments (BR6.5) and the start of the keys.
func draftHeader(in SuggestInput, noKeys bool) string {
	version := in.ToolVersion
	if !versionPattern.MatchString(version) {
		version = "unknown"
	}
	account := in.AccountID
	if !accountPattern.MatchString(account) {
		account = "unknown"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# generated by tag-refactor %s\n", version)
	fmt.Fprintf(&b, "# account %s, %d of %d regions, %s\n", account, in.RegionsRead, in.RegionsEnabled,
		in.GeneratedAt.UTC().Format(time.RFC3339))
	if len(in.FailedRegions) > 0 {
		names := make([]string, 0, len(in.FailedRegions))
		for _, r := range in.FailedRegions {
			if !IsRegionName(r) {
				r = "unknown-region"
			}
			names = append(names, r)
		}
		fmt.Fprintf(&b, "# Results are incomplete: %s\n", strings.Join(names, ", "))
	}
	b.WriteString(draftReview1)
	b.WriteString(draftReview2)
	b.WriteString("version: 1\n")
	if noKeys {
		b.WriteString("keys: {}\n")
	} else {
		b.WriteString("keys:\n")
	}
	return b.String()
}

// renderKey writes one key of the draft.
func renderKey(k SuggestedKey) string {
	var b strings.Builder
	name := "  " + quote(k.CanonicalKey) + ":"
	note := ""
	switch k.ValuesOmitted {
	case OmitTooManyValues:
		note = tooManyValuesNote
	case OmitSizeLimit:
		note = sizeLimitNote
	}
	if len(k.Aliases) == 0 && len(k.Values) == 0 {
		b.WriteString(name + " {}")
		if note != "" {
			b.WriteString("  # " + note)
		}
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString(name + "\n")
	if len(k.Aliases) > 0 {
		b.WriteString("    aliases: " + quoteList(k.Aliases) + "\n")
	}
	if len(k.Values) > 0 {
		b.WriteString("    values:\n")
		for _, v := range k.Values {
			b.WriteString("      " + quote(v.CanonicalValue) + ": " + quoteList(v.Spellings) + "\n")
		}
	} else if note != "" {
		b.WriteString("    # " + note + "\n")
	}
	return b.String()
}

// fitDraft writes the draft within MaxFileSize when it can (BR6.7): while it
// is too large, the value candidates of the key with the fewest resources
// are left out. The size is kept as a running total of exact byte counts, so
// the draft is not written again for each key; the written draft is measured
// once more at the end, and a draft over the limit is never taken as fitting.
func fitDraft(keys []SuggestedKey, head, foot string) string {
	texts := make([]string, len(keys))
	total := len(head) + len(foot)
	for i, k := range keys {
		texts[i] = renderKey(k)
		total += len(texts[i])
	}
	var candidates []int
	if total > MaxFileSize {
		for i, k := range keys {
			if len(k.Values) > 0 {
				candidates = append(candidates, i)
			}
		}
		sort.SliceStable(candidates, func(a, b int) bool {
			ka, kb := keys[candidates[a]], keys[candidates[b]]
			if ka.ResourceCount != kb.ResourceCount {
				return ka.ResourceCount < kb.ResourceCount
			}
			return ka.CanonicalKey > kb.CanonicalKey
		})
	}
	next := 0
	for {
		for total > MaxFileSize && next < len(candidates) {
			i := candidates[next]
			next++
			keys[i].Values, keys[i].ValuesOmitted = []ValueRule{}, OmitSizeLimit
			text := renderKey(keys[i])
			total += len(text) - len(texts[i])
			texts[i] = text
		}
		var b strings.Builder
		b.Grow(total)
		b.WriteString(head)
		for _, t := range texts {
			b.WriteString(t)
		}
		b.WriteString(foot)
		draft := b.String()
		if len(draft) <= MaxFileSize || next >= len(candidates) {
			return draft
		}
		total = len(draft)
	}
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = quote(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// quote writes s as a YAML double-quoted scalar that reads back as the same
// string (BR6.4, NFR8.12): '"' and '\' are escaped; every character that is
// not graphic (control and format characters, line and paragraph separators,
// unassigned and private-use code points, which YAML may not allow as they
// are) is written as \uXXXX, or \UXXXXXXXX outside the basic plane; invalid
// UTF-8 becomes U+FFFD. "[" and "{" are escaped too, so the nesting check of
// the reader never counts them.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case unicode.IsGraphic(r) && r != '[' && r != '{':
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			fmt.Fprintf(&b, `\U%08X`, r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
