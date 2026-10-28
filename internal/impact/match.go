package impact

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Binding tag references to changes and deciding the verdicts (U5 BR3.1 to
// BR3.3, BR4.1 to BR4.7).

// verdict is the result of matching one reference with one change.
type verdict int

const (
	noImpact verdict = iota
	affected
	undecided
)

// changeIndex looks changes up by the keys they touch (BR4.1), by resource
// ARN (BR3.2) and among the changes of IAM users and roles (BR3.3). It is
// built once at the start of an analysis and only read afterwards, so the
// reading jobs share it without a lock.
type changeIndex struct {
	changes []planner.TagChangeInput
	// all lists every change; principals the changes of IAM users and roles.
	all        *keyIndex
	principals *keyIndex
	byARN      map[string][]int
}

// keyIndex maps the lowercased change keys of a set of changes to the
// changes, in change order.
type keyIndex struct {
	ids   []int
	byKey map[string][]int
	keys  []string
}

func newKeyIndex(changes []planner.TagChangeInput, ids []int) *keyIndex {
	x := &keyIndex{ids: ids, byKey: map[string][]int{}}
	for _, i := range ids {
		for _, k := range changeKeys(changes[i]) {
			lower := strings.ToLower(k)
			list := x.byKey[lower]
			if len(list) == 0 || list[len(list)-1] != i {
				x.byKey[lower] = append(list, i)
			}
		}
	}
	for k := range x.byKey {
		x.keys = append(x.keys, k)
	}
	sort.Strings(x.keys)
	return x
}

// principalARN matches the ARNs of IAM users and roles (BR3.3).
var principalARN = regexp.MustCompile(`^arn:[^:]+:iam::[^:]*:(user|role)/`)

func newChangeIndex(changes []planner.TagChangeInput) *changeIndex {
	x := &changeIndex{changes: changes, byARN: map[string][]int{}}
	all := make([]int, len(changes))
	var principals []int
	for i, c := range changes {
		all[i] = i
		x.byARN[c.ResourceARN] = append(x.byARN[c.ResourceARN], i)
		if principalARN.MatchString(c.ResourceARN) {
			principals = append(principals, i)
		}
	}
	x.all = newKeyIndex(changes, all)
	x.principals = newKeyIndex(changes, principals)
	return x
}

// changeKeys are the keys a change touches (BR4.1): both keys of a rename,
// the key of a value change or a removal, the key added.
func changeKeys(c planner.TagChangeInput) []string {
	switch c.Kind {
	case planner.KindRenameKey:
		return []string{c.FromKey, c.ToKey}
	case planner.KindAddRequired:
		return []string{c.ToKey}
	}
	return []string{c.FromKey}
}

// source is a policy being matched.
type source struct {
	kind string
	ref  string
	// resourceARN is the resource a resource policy belongs to; empty for
	// IAM policies.
	resourceARN string
	region      string
}

func (s source) isIAM() bool {
	return s.kind == planner.SourceIAMManaged || s.kind == planner.SourceIAMInline
}

// candidates returns the changes bound to the reference (BR3) whose keys can
// match it (BR4.1), in change order. TagKeys and keys with a policy variable
// cannot be narrowed by key, so every bound change is returned for them.
func (m *matcher) candidates(src source, r *reference) []int {
	var keys *keyIndex
	switch {
	case r.kind == kindPrincipalTag:
		keys = m.index.principals
	case src.isIAM():
		keys = m.index.all
	default:
		return m.resourceCandidates(src.resourceARN, r)
	}
	if r.kind == kindTagKeys || strings.Contains(r.tagKey, "${") {
		return keys.ids
	}
	pattern := strings.ToLower(r.tagKey)
	if !strings.ContainsAny(pattern, "*?") {
		return keys.byKey[pattern]
	}
	re := m.patterns.get(pattern, false)
	seen := map[int]bool{}
	var ids []int
	for _, k := range keys.keys {
		if !re.MatchString(k) {
			continue
		}
		for _, i := range keys.byKey[k] {
			if !seen[i] {
				seen[i] = true
				ids = append(ids, i)
			}
		}
	}
	sort.Ints(ids)
	return ids
}

// resourceCandidates are the changes of the resource a resource policy
// belongs to whose keys can match the reference (BR3.2).
func (m *matcher) resourceCandidates(arn string, r *reference) []int {
	own := m.index.byARN[arn]
	if r.kind == kindTagKeys || strings.Contains(r.tagKey, "${") {
		return own
	}
	var ids []int
	for _, i := range own {
		if m.keyMatches(r.tagKey, m.index.changes[i]) {
			ids = append(ids, i)
		}
	}
	return ids
}

// keyMatches compares a tag key of a reference with the keys of a change
// without letter case, * and ? being wildcards (BR2.2).
func (m *matcher) keyMatches(tagKey string, c planner.TagChangeInput) bool {
	pattern := strings.ToLower(tagKey)
	wild := strings.ContainsAny(pattern, "*?")
	for _, k := range changeKeys(c) {
		k = strings.ToLower(k)
		if (!wild && k == pattern) || (wild && m.patterns.get(pattern, false).MatchString(k)) {
			return true
		}
	}
	return false
}

// matcher matches the references of policies with the changes.
type matcher struct {
	index    *changeIndex
	patterns *patternCache
}

// judge decides what one change does to one reference (BR4.1 to BR4.7).
// The change has been bound to the reference already.
func (m *matcher) judge(r *reference, c planner.TagChangeInput) verdict {
	if r.kind == kindTagKeys {
		return m.judgeTagKeys(r, c)
	}
	if strings.Contains(r.tagKey, "${") {
		// BR4.6: the key is only known when AWS evaluates the policy.
		if caseOnlyRename(r, c) {
			return noImpact
		}
		return undecided
	}
	if !m.keyMatches(r.tagKey, c) {
		return noImpact
	}
	switch c.Kind {
	case planner.KindRenameKey:
		if caseOnlyRename(r, c) {
			return noImpact
		}
		return affected
	case planner.KindRemoveKey:
		return affected
	case planner.KindChangeValue:
		return m.judgeValueChange(r, c)
	case planner.KindAddRequired:
		return m.judgeAdd(r, c)
	}
	return noImpact
}

// caseOnlyRename reports the one rename that cannot break a reference: a
// change of letter case only, under the global aws:ResourceTag condition
// key, whose tag keys AWS compares without case (BR4.2). Variables do not
// get the exception.
func caseOnlyRename(r *reference, c planner.TagChangeInput) bool {
	return c.Kind == planner.KindRenameKey && r.loc == locConditionKey && r.kind == kindResourceTag && r.awsPrefix &&
		strings.EqualFold(c.FromKey, c.ToKey)
}

// judgeValueChange is BR4.3.
func (m *matcher) judgeValueChange(r *reference, c planner.TagChangeInput) verdict {
	if r.loc.isVariable() {
		// The value is only known when AWS evaluates the policy.
		return undecided
	}
	if r.cond.kind == opNull {
		// The tag is there before and after.
		return noImpact
	}
	return m.compareValues(r.cond, []string{c.FromValue, c.ToValue}, false)
}

// judgeAdd is BR4.4: the tag goes from missing to the value added.
func (m *matcher) judgeAdd(r *reference, c planner.TagChangeInput) verdict {
	if r.loc.isVariable() || r.cond.kind == opNull || r.cond.ifExists || r.cond.set != setNone {
		return affected
	}
	return m.compareValues(r.cond, []string{c.ToValue}, false)
}

// judgeTagKeys is BR4.5 with the decision on Null of the U5 NFR design.
func (m *matcher) judgeTagKeys(r *reference, c planner.TagChangeInput) verdict {
	switch {
	case r.loc != locConditionKey:
		return noImpact
	case c.Kind == planner.KindChangeValue:
		return noImpact
	case r.cond.kind == opNull:
		// Whether keys are present can change, depending on the request.
		return undecided
	case (c.Kind == planner.KindRenameKey || c.Kind == planner.KindAddRequired) &&
		(r.cond.negated || r.cond.set == setForAllValues):
		// A list of allowed keys: the new key may be missing from it.
		return affected
	}
	return m.compareValues(r.cond, changeKeys(c), true)
}

// compareValues compares the condition's values with the candidates by the
// operator's kind (BR4.7): affected when one matches; otherwise undecided
// when a value holds a policy variable or the operator does not compare
// strings, and no impact. Values of TagKeys are tag keys and are compared
// without letter case.
func (m *matcher) compareValues(cond *condition, candidates []string, tagKeys bool) verdict {
	if cond.kind == opOther {
		return undecided
	}
	hasVariable := false
	for _, v := range cond.values {
		if strings.Contains(v, "${") {
			hasVariable = true
			continue
		}
		for _, cand := range candidates {
			if m.valueMatches(cond.kind, v, cand, tagKeys) {
				return affected
			}
		}
	}
	if hasVariable {
		return undecided
	}
	return noImpact
}

func (m *matcher) valueMatches(kind operatorKind, value, cand string, tagKeys bool) bool {
	switch kind {
	case opEquals:
		if tagKeys {
			return strings.EqualFold(value, cand)
		}
		return value == cand
	case opIgnoreCase:
		return strings.EqualFold(value, cand)
	case opPattern:
		if !strings.ContainsAny(value, "*?") {
			if tagKeys {
				return strings.EqualFold(value, cand)
			}
			return value == cand
		}
		return m.patterns.get(value, tagKeys).MatchString(cand)
	}
	return false
}

// maxCachedPatterns bounds the patterns kept for reuse (security design §2).
// Patterns longer than maxCachedPatternBytes are not kept either, so a few
// huge patterns cannot fill the memory.
const (
	maxCachedPatterns     = 1024
	maxCachedPatternBytes = 4096
)

// maxPatternRunes bounds the characters other than * of a pattern that is
// compiled. AWS limits tag keys to 128 and tag values to 256 characters, so a
// longer pattern cannot match any tag and is not compiled.
const maxPatternRunes = 4096

// neverMatches is the expression of a pattern too long to match any tag.
var neverMatches = regexp.MustCompile(`[^\x00-\x{10FFFF}]`)

// patternCache keeps the regular expressions made from wildcard patterns
// during one analysis, so the same pattern is compiled once. The reading
// jobs share it, so a lock guards it. Beyond maxCachedPatterns, a pattern is
// compiled for each use and not kept, so the memory stays bounded.
type patternCache struct {
	mu      sync.Mutex
	entries map[string]*regexp.Regexp
}

func newPatternCache() *patternCache {
	return &patternCache{entries: map[string]*regexp.Regexp{}}
}

// get returns the regular expression of an IAM wildcard pattern, in which
// only * and ? are special. Go regular expressions run in time linear in the
// input, so patterns such as a*a*a*a*b stay fast (NFR1.5). foldCase makes it
// ignore letter case.
func (p *patternCache) get(pattern string, foldCase bool) *regexp.Regexp {
	key := "s:" + pattern
	if foldCase {
		key = "i:" + pattern
	}
	p.mu.Lock()
	re, ok := p.entries[key]
	p.mu.Unlock()
	if ok {
		return re
	}
	re = compileWildcard(pattern, foldCase)
	if len(pattern) > maxCachedPatternBytes {
		return re
	}
	p.mu.Lock()
	if _, ok := p.entries[key]; !ok && len(p.entries) < maxCachedPatterns {
		p.entries[key] = re
	}
	p.mu.Unlock()
	return re
}

// size returns the number of patterns kept.
func (p *patternCache) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// compileWildcard turns a wildcard pattern into a regular expression. Runs of
// * are one *, and a pattern with more than maxPatternRunes other characters
// never matches, so the expression stays small whatever the policy holds.
func compileWildcard(pattern string, foldCase bool) *regexp.Regexp {
	if n := len([]rune(strings.ReplaceAll(pattern, "*", ""))); n > maxPatternRunes {
		return neverMatches
	}
	var b strings.Builder
	b.WriteString(`(?s)`)
	if foldCase {
		b.WriteString(`(?i)`)
	}
	b.WriteString(`^`)
	star := false
	for _, r := range pattern {
		if r == '*' && star {
			continue
		}
		star = r == '*'
		switch r {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(`$`)
	return regexp.MustCompile(b.String())
}
