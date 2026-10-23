package inventory

import (
	"sort"
	"strings"
)

// abbreviations is the abbreviation dictionary of BR5.1: a normalized key
// that is exactly one of these is grouped with the full word. Parts of a key
// (envname) are never replaced. U3's rules suggest uses the same dictionary.
var abbreviations = map[string]string{
	"env":  "environment",
	"prod": "production",
	"dev":  "development",
	"stg":  "staging",
}

// Abbreviations returns a copy of the abbreviation dictionary (normalized
// abbreviation to full word).
func Abbreviations() map[string]string {
	out := make(map[string]string, len(abbreviations))
	for k, v := range abbreviations {
		out[k] = v
	}
	return out
}

// NormalizeKey lowercases a key and removes the separators "-", "_" and
// space (BR2.2 of U1).
func NormalizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', ' ':
			return -1
		}
		return r
	}, strings.ToLower(key))
}

// CanonicalKey is the normalized key with a whole-key abbreviation replaced
// by its full word (BR5.1).
func CanonicalKey(key string) string {
	norm := NormalizeKey(key)
	if full, ok := abbreviations[norm]; ok {
		return full
	}
	return norm
}

// Spelling is one way a key is written, with the number of resources that
// carry it.
type Spelling struct {
	Key           string
	ResourceCount int
}

// KeyVariantGroup holds the keys that have the same canonical key (BR5.1).
type KeyVariantGroup struct {
	// NormalizedKey is the canonical key: normalized, with an abbreviation
	// replaced by its full word.
	NormalizedKey string
	// Spellings are sorted by count, highest first, then by key.
	Spellings []Spelling
	// TotalResources counts each resource once, even when it carries more
	// than one spelling.
	TotalResources int
	// IsVariant is true when two or more spellings differ only in case and
	// separators: an issue to review (exit code 2).
	IsVariant bool
	// AbbreviationOnly is true when only the dictionary groups the keys: the
	// spellings differ in more than case and separators, or the one spelling
	// is an abbreviation. Such a group is shown, marked, and is not an issue
	// (BR5.1).
	AbbreviationOnly bool
}

// Shown reports whether the group is listed among the key variants.
func (g KeyVariantGroup) Shown() bool { return g.IsVariant || g.AbbreviationOnly }

// GroupKeys groups the tag keys of the resources by their canonical key,
// leaving out aws: keys (BR5.1). Groups are sorted by total resources,
// highest first, then by canonical key.
func GroupKeys(resources []Resource) []KeyVariantGroup {
	spellings := map[string]map[string]int{}
	totals := map[string]int{}
	for _, r := range resources {
		counted := make(map[string]bool, len(r.Tags))
		for key := range r.Tags {
			if IsAWSReservedKey(key) {
				continue
			}
			canon := CanonicalKey(key)
			if spellings[canon] == nil {
				spellings[canon] = map[string]int{}
			}
			spellings[canon][key]++
			if !counted[canon] {
				counted[canon] = true
				totals[canon]++
			}
		}
	}
	groups := make([]KeyVariantGroup, 0, len(spellings))
	for canon, keys := range spellings {
		g := KeyVariantGroup{NormalizedKey: canon, TotalResources: totals[canon]}
		perNorm := map[string]int{}
		for key, n := range keys {
			g.Spellings = append(g.Spellings, Spelling{Key: key, ResourceCount: n})
			perNorm[NormalizeKey(key)]++
		}
		for norm, n := range perNorm {
			if n >= 2 {
				g.IsVariant = true
			}
			if norm != canon {
				// An abbreviation joined the group.
				g.AbbreviationOnly = true
			}
		}
		if g.IsVariant {
			g.AbbreviationOnly = false
		}
		sort.Slice(g.Spellings, func(i, j int) bool {
			a, b := g.Spellings[i], g.Spellings[j]
			if a.ResourceCount != b.ResourceCount {
				return a.ResourceCount > b.ResourceCount
			}
			return a.Key < b.Key
		})
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalResources != groups[j].TotalResources {
			return groups[i].TotalResources > groups[j].TotalResources
		}
		return groups[i].NormalizedKey < groups[j].NormalizedKey
	})
	return groups
}

// Variants returns the groups to list: real variants and groups made by the
// abbreviation dictionary, in order.
func Variants(groups []KeyVariantGroup) []KeyVariantGroup {
	var out []KeyVariantGroup
	for _, g := range groups {
		if g.Shown() {
			out = append(out, g)
		}
	}
	return out
}

// CountVariants returns the number of groups that are issues: those with two
// spellings that differ only in case and separators.
func CountVariants(groups []KeyVariantGroup) int {
	n := 0
	for _, g := range groups {
		if g.IsVariant {
			n++
		}
	}
	return n
}

// MissingRequiredTag is a required key and the resources without it (BR5.2).
type MissingRequiredTag struct {
	Key string
	// ResourceCount counts the resources without the key whose tags can be
	// changed.
	ResourceCount int
	// NotTaggableCount counts the resources without the key whose type
	// cannot be tagged; they are shown apart.
	NotTaggableCount int
	// LowerBound is true when never-tagged resources could not be looked for
	// everywhere, so the count may be too small (BR3.2).
	LowerBound bool
	// ARNs are the ARNs counted in ResourceCount, sorted (for U4's JSON).
	ARNs []string
}

// MissingRequired counts, for each required key, the resources that do not
// carry it: tagged and never-tagged resources alike (BR5.2). Keys are
// compared with their case. Keys starting with aws: must be removed by the
// caller. The result keeps the order of keys.
func MissingRequired(snap *Snapshot, keys []string) []MissingRequiredTag {
	lowerBound := snap.Discovery == nil || snap.Discovery.Status != StatusComplete
	out := make([]MissingRequiredTag, 0, len(keys))
	for _, key := range keys {
		m := MissingRequiredTag{Key: key, LowerBound: lowerBound, ARNs: []string{}}
		for _, r := range snap.Resources {
			if _, ok := r.Tags[key]; ok {
				continue
			}
			if r.NotTaggable {
				m.NotTaggableCount++
				continue
			}
			m.ResourceCount++
			m.ARNs = append(m.ARNs, r.ARN)
		}
		out = append(out, m)
	}
	return out
}

// TypeCount is a resource type and its number of resources.
type TypeCount struct {
	ResourceType string
	Count        int
}

// UnsupportedResourceTypes counts the resources whose tags cannot be changed,
// by type, highest count first, then by type (BR3.3).
func UnsupportedResourceTypes(resources []Resource) []TypeCount {
	counts := map[string]int{}
	for _, r := range resources {
		if r.NotTaggable {
			counts[r.ResourceType]++
		}
	}
	out := make([]TypeCount, 0, len(counts))
	for t, n := range counts {
		out = append(out, TypeCount{ResourceType: t, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].ResourceType < out[j].ResourceType
	})
	return out
}

// NeverTagged counts the resources found without user tags by the discovery
// (BR5.5).
func NeverTagged(resources []Resource) int {
	n := 0
	for _, r := range resources {
		if !r.EverTagged {
			n++
		}
	}
	return n
}

// ScanReport is what scan shows (ScanResult of the functional design).
type ScanReport struct {
	Snapshot *Snapshot
	// Groups are every key group; Variants(Groups) are the ones listed.
	Groups []KeyVariantGroup
	// Missing is nil when no rule file was given ("not checked").
	Missing []MissingRequiredTag
	// RulesPath is the rule file the required keys came from.
	RulesPath   string
	NeverTagged int
	Unsupported []TypeCount
}

// NewScanReport analyzes the snapshot. required is nil without a rule file.
func NewScanReport(snap *Snapshot, rulesPath string, required []string) *ScanReport {
	r := &ScanReport{
		Snapshot:    snap,
		Groups:      GroupKeys(snap.Resources),
		RulesPath:   rulesPath,
		NeverTagged: NeverTagged(snap.Resources),
		Unsupported: UnsupportedResourceTypes(snap.Resources),
	}
	if required != nil {
		r.Missing = MissingRequired(snap, required)
	}
	return r
}

// MissingKeys is the number of required keys that some taggable resource
// lacks.
func (r *ScanReport) MissingKeys() int {
	n := 0
	for _, m := range r.Missing {
		if m.ResourceCount > 0 {
			n++
		}
	}
	return n
}

// IssuesFound is true when there are key variants (not counting groups made
// by the dictionary only) or missing required tags (BR5.3).
func (r *ScanReport) IssuesFound() bool {
	return CountVariants(r.Groups) > 0 || r.MissingKeys() > 0
}
