package rules

// The queries U4's plan asks of a rule set (contract C8). They never call
// AWS and answer from the rule file alone; the same input always gives the
// same answer, with the line that decided it.

// index holds the lookup tables built when a rule set is read, so each query
// costs a few map lookups (NFR1.12).
type index struct {
	// canonicalOf maps an alias to its canonical key.
	canonicalOf map[string]string
	// canonicalLine maps a canonical key to its line.
	canonicalLine map[string]int
	// aliasRank orders the aliases of one canonical key as the file does.
	aliasRank map[string]int
	// values maps a canonical key and a spelling to its canonical value.
	values map[string]map[string]valueEntry
	// removed maps a removed key to its line.
	removed map[string]int
	// required maps a required key to its position in RequiredTags.
	required map[string]int
}

type valueEntry struct {
	value string
	line  int
}

func newIndex(rs *RuleSet) *index {
	idx := &index{
		canonicalOf:   map[string]string{},
		canonicalLine: make(map[string]int, len(rs.KeyRules)),
		aliasRank:     map[string]int{},
		values:        map[string]map[string]valueEntry{},
		removed:       make(map[string]int, len(rs.RemoveKeys)),
		required:      make(map[string]int, len(rs.RequiredTags)),
	}
	for _, r := range rs.KeyRules {
		idx.canonicalLine[r.CanonicalKey] = r.Line
		for i, a := range r.Aliases {
			idx.canonicalOf[a] = r.CanonicalKey
			idx.aliasRank[a] = i
		}
		if len(r.Values) == 0 {
			continue
		}
		spellings := map[string]valueEntry{}
		for _, v := range r.Values {
			for _, s := range v.Spellings {
				spellings[s] = valueEntry{value: v.CanonicalValue, line: v.Line}
			}
		}
		idx.values[r.CanonicalKey] = spellings
	}
	for _, k := range rs.RemoveKeys {
		idx.removed[k.Key] = k.Line
	}
	for i, t := range rs.RequiredTags {
		idx.required[t.Key] = i
	}
	return idx
}

// lookup returns the tables of the rule set. A rule set that was not built
// by Parse gets fresh tables on each call.
func (rs *RuleSet) lookup() *index {
	if rs.idx != nil {
		return rs.idx
	}
	return newIndex(rs)
}

// CanonicalKeyOf returns the canonical key of an alias and true, or the key
// unchanged and false when it is not an alias (BR2.1).
func (rs *RuleSet) CanonicalKeyOf(key string) (string, bool) {
	if c, ok := rs.lookup().canonicalOf[key]; ok {
		return c, true
	}
	return key, false
}

// ValueAnswer is the answer of CanonicalValue (BR2.3).
type ValueAnswer struct {
	// Value is the canonical value, or the value unchanged.
	Value string
	// Changed is true when the value is another spelling of Value.
	Changed bool
	// Line is the line of the canonical value that decided it; 0 when the
	// value is unchanged.
	Line int
}

// CanonicalValue returns the canonical value of a value of the canonical key
// (BR2.3): for another spelling, its canonical value; for anything else, the
// value unchanged. Values are compared exactly. Ask with the canonical key:
// an alias has no values of its own.
func (rs *RuleSet) CanonicalValue(key, value string) ValueAnswer {
	if v, ok := rs.lookup().values[key][value]; ok {
		return ValueAnswer{Value: v.value, Changed: true, Line: v.line}
	}
	return ValueAnswer{Value: value}
}

// RemoveAnswer is the answer of IsRemoved (BR4.2).
type RemoveAnswer struct {
	Remove bool
	// Line is the line of the key under "remove"; 0 when not removed.
	Line int
}

// IsRemoved reports whether the key is listed under "remove", compared
// exactly: no normalization and no abbreviation (BR4.2).
func (rs *RuleSet) IsRemoved(key string) RemoveAnswer {
	if line, ok := rs.lookup().removed[key]; ok {
		return RemoveAnswer{Remove: true, Line: line}
	}
	return RemoveAnswer{}
}

// Resource is what the required value query needs to know of a resource.
type Resource struct {
	// ResourceType is the type as scan shows it, for example "ec2:instance".
	ResourceType string
	Region       string
	// Global is true when the ARN has no region.
	Global bool
	// Tags are the tags as collected, before any rule is applied.
	Tags map[string]string
}

// AnswerStatus says whether a required value was decided.
type AnswerStatus string

// DecidedBy names what decided a required value.
type DecidedBy string

// Values of RequiredValueAnswer.
const (
	StatusResolved    AnswerStatus = "resolved"
	StatusUnspecified AnswerStatus = "unspecified"

	DecidedByWhen    DecidedBy = "when"
	DecidedByDefault DecidedBy = "default"
	DecidedByNone    DecidedBy = "none"
)

// RequiredValueAnswer is the value of a required tag for one resource.
type RequiredValueAnswer struct {
	Key    string
	Status AnswerStatus
	// Value is set when Status is StatusResolved.
	Value     string
	DecidedBy DecidedBy
	// Line is the line of the "when" entry or of "default" that decided the
	// value; 0 when unspecified.
	Line int
}

// RequiredValue returns the value of the required tag key for the resource
// (BR3.2): the value of the first "when" entry that matches, else the
// default, else unspecified. The conditions see the resource's tags after
// UnifiedTags. A key that is not required is unspecified.
func (rs *RuleSet) RequiredValue(key string, res Resource) RequiredValueAnswer {
	idx := rs.lookup()
	i, ok := idx.required[key]
	if !ok {
		return RequiredValueAnswer{Key: key, Status: StatusUnspecified, DecidedBy: DecidedByNone}
	}
	tag := rs.RequiredTags[i]
	if len(tag.When) > 0 {
		tags := rs.unifiedTags(idx, res.Tags)
		for _, c := range tag.When {
			if c.matches(res, tags) {
				return RequiredValueAnswer{Key: key, Status: StatusResolved, Value: c.Value, DecidedBy: DecidedByWhen, Line: c.Line}
			}
		}
	}
	if tag.HasDefault {
		return RequiredValueAnswer{Key: key, Status: StatusResolved, Value: tag.Default, DecidedBy: DecidedByDefault, Line: tag.DefaultLine}
	}
	return RequiredValueAnswer{Key: key, Status: StatusUnspecified, DecidedBy: DecidedByNone}
}

// matches reports whether every field the condition sets matches (BR3.2). A
// global resource has no region of its own: the region it was read in
// depends on the regions read, so a region condition never matches it.
func (c RequiredCondition) matches(res Resource, tags map[string]string) bool {
	if c.ResourceType != "" && c.ResourceType != res.ResourceType {
		return false
	}
	if c.Region != "" && (res.Global || c.Region != res.Region) {
		return false
	}
	for k, v := range c.Tags {
		if got, ok := tags[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// UnifiedTags returns the tags as the conditions compare them (BR3.2): each
// alias renamed to its canonical key, each value spelling replaced by its
// canonical value, and removed keys taken away. When a resource carries a
// canonical key and its alias, the canonical key's current value is kept;
// when it carries several aliases of a key without the key itself, the alias
// listed first in the rule file wins. Keys starting with aws: are kept as
// they are. tags is not changed.
func (rs *RuleSet) UnifiedTags(tags map[string]string) map[string]string {
	return rs.unifiedTags(rs.lookup(), tags)
}

func (rs *RuleSet) unifiedTags(idx *index, tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		if _, isAlias := idx.canonicalOf[k]; !isAlias {
			out[k] = v
		}
	}
	// fromAlias holds, for each canonical key taken from an alias, the rank of
	// that alias in the rule file.
	var fromAlias map[string]int
	for k, v := range tags {
		canonical, isAlias := idx.canonicalOf[k]
		if !isAlias {
			continue
		}
		if _, own := tags[canonical]; own {
			continue
		}
		rank := idx.aliasRank[k]
		if taken, ok := fromAlias[canonical]; ok && taken < rank {
			continue
		}
		if fromAlias == nil {
			fromAlias = map[string]int{}
		}
		out[canonical] = v
		fromAlias[canonical] = rank
	}
	for k, v := range out {
		if _, ok := idx.removed[k]; ok {
			delete(out, k)
			continue
		}
		if e, ok := idx.values[k][v]; ok {
			out[k] = e.value
		}
	}
	return out
}
