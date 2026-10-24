package rules

import (
	"fmt"
	"sort"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// checkValues applies BR2.2 within one key: a spelling belongs to one
// canonical value only and is not itself a canonical value of the key. A
// spelling equal to its own value, or listed twice, gives a warning and is
// dropped. Values are compared case-sensitively.
func (l *loader) checkValues(key string, values []ValueRule, spellingLines [][]int) ([]ValueRule, error) {
	canonicalLine := make(map[string]int, len(values))
	for _, v := range values {
		canonicalLine[v.CanonicalValue] = v.Line
	}
	owner := map[string]int{}
	ownerLine := map[string]int{}
	for i := range values {
		v := &values[i]
		kept := make([]string, 0, len(v.Spellings))
		for j, s := range v.Spellings {
			line := spellingLines[i][j]
			if s == v.CanonicalValue {
				l.warn(line, s, fmt.Sprintf("the spelling %q of the value %q of %q is the value itself; it is ignored", s, v.CanonicalValue, key))
				continue
			}
			if cl, ok := canonicalLine[s]; ok {
				return nil, l.fail(errkind.InvalidInput, line, s,
					fmt.Sprintf("the spelling %q of the value %q of %q is also a canonical value (lines %d and %d)", s, v.CanonicalValue, key, line, cl),
					"remove it from the spellings or remove its own entry")
			}
			if o, ok := owner[s]; ok {
				if o == i {
					l.warn(line, s, fmt.Sprintf("the spelling %q is listed more than once under the value %q of %q", s, v.CanonicalValue, key))
					continue
				}
				return nil, l.fail(errkind.InvalidInput, line, s,
					fmt.Sprintf("the spelling %q of %q is listed under the values %q (line %d) and %q (line %d)",
						s, key, values[o].CanonicalValue, ownerLine[s], v.CanonicalValue, line),
					"keep the spelling under one canonical value only")
			}
			owner[s], ownerLine[s] = i, line
			kept = append(kept, s)
		}
		v.Spellings = kept
	}
	return values, nil
}

// checkConsistency checks the items against each other once the whole file
// is read, stopping at the first error (BR1.1): aliases (BR2.1), required
// keys (BR3.1), condition tags (BR3.6) and removed keys (BR4.1).
func (l *loader) checkConsistency() error {
	if err := l.checkAliases(); err != nil {
		return err
	}
	c := newIndex(l.rs)
	if err := l.checkRequired(c); err != nil {
		return err
	}
	return l.checkRemove(c)
}

// checkAliases enforces BR2.1: an alias belongs to one canonical key only and
// is never itself a canonical key (compared case-sensitively). Canonical keys
// that normalize to the same key only produce a warning.
func (l *loader) checkAliases() error {
	canonicalLine := map[string]int{}
	for _, r := range l.rs.KeyRules {
		canonicalLine[r.CanonicalKey] = r.Line
	}
	owner := map[string]int{}
	for i, r := range l.rs.KeyRules {
		for _, a := range r.Aliases {
			aliasLine := l.aliasLines[i][a]
			if line, ok := canonicalLine[a]; ok {
				return l.fail(errkind.InvalidInput, aliasLine, a,
					fmt.Sprintf("the alias %q of %q is also a canonical key (lines %d and %d)", a, r.CanonicalKey, aliasLine, line),
					"remove it from the aliases or remove its own rule")
			}
			if j, ok := owner[a]; ok {
				other := l.rs.KeyRules[j]
				return l.fail(errkind.InvalidInput, aliasLine, a,
					fmt.Sprintf("the alias %q is listed under %q (line %d) and %q (line %d)",
						a, other.CanonicalKey, l.aliasLines[j][a], r.CanonicalKey, aliasLine),
					"keep the alias under one canonical key only")
			}
			owner[a] = i
		}
	}
	byNorm := map[string][]KeyRule{}
	for _, r := range l.rs.KeyRules {
		n := NormalizeKey(r.CanonicalKey)
		byNorm[n] = append(byNorm[n], r)
	}
	norms := make([]string, 0, len(byNorm))
	for n := range byNorm {
		norms = append(norms, n)
	}
	sort.Strings(norms)
	for _, n := range norms {
		group := byNorm[n]
		for _, r := range group[1:] {
			l.warn(r.Line, r.CanonicalKey, fmt.Sprintf("the canonical keys %q (line %d) and %q differ only in case or separators",
				group[0].CanonicalKey, group[0].Line, r.CanonicalKey))
		}
	}
	return nil
}

// checkRequired applies BR3.1 to the required keys and BR3.6 to the tags of
// their conditions.
func (l *loader) checkRequired(c *index) error {
	for _, tag := range l.rs.RequiredTags {
		if canonical, ok := c.canonicalOf[tag.Key]; ok {
			return l.fail(errkind.InvalidInput, tag.Line, tag.Key,
				fmt.Sprintf("%q is an alias of %q; a required tag must use the canonical key", tag.Key, canonical),
				fmt.Sprintf("write %q here", canonical))
		}
		if rl, ok := c.removed[tag.Key]; ok {
			return l.fail(errkind.InvalidInput, tag.Line, tag.Key,
				fmt.Sprintf("%q is required and also listed under \"remove\" (line %d)", tag.Key, rl),
				"keep the key in one of them only")
		}
		for _, cond := range tag.When {
			if err := l.checkConditionTags(c, cond); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkConditionTags applies BR3.6: conditions compare the tags after
// aliases and value spellings are unified and removed keys are taken away
// (BR3.2), so an alias, a spelling or a removed key would never match.
func (l *loader) checkConditionTags(c *index, cond RequiredCondition) error {
	keys := make([]string, 0, len(cond.Tags))
	for k := range cond.Tags {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if cond.tagLines[keys[i]] != cond.tagLines[keys[j]] {
			return cond.tagLines[keys[i]] < cond.tagLines[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		line := cond.tagLines[k]
		if line == 0 {
			line = cond.Line
		}
		if canonical, ok := c.canonicalOf[k]; ok {
			return l.fail(errkind.InvalidInput, line, k,
				fmt.Sprintf("%q is an alias of %q; conditions compare tags after aliases are renamed", k, canonical),
				fmt.Sprintf("write %q here", canonical))
		}
		if rl, ok := c.removed[k]; ok {
			return l.fail(errkind.InvalidInput, line, k,
				fmt.Sprintf("%q is listed under \"remove\" (line %d); conditions compare tags after removed keys are taken away", k, rl),
				"remove the tag from the condition")
		}
		if v, ok := c.values[k][cond.Tags[k]]; ok {
			return l.fail(errkind.InvalidInput, line, k,
				fmt.Sprintf("%q is another spelling of %q for %q; conditions compare values after they are unified", cond.Tags[k], v.value, k),
				fmt.Sprintf("write %q here", v.value))
		}
	}
	return nil
}

// checkRemove applies BR4.1: a removed key is neither a canonical key nor an
// alias.
func (l *loader) checkRemove(c *index) error {
	for _, r := range l.rs.RemoveKeys {
		if line, ok := c.canonicalLine[r.Key]; ok {
			return l.fail(errkind.InvalidInput, r.Line, r.Key,
				fmt.Sprintf("%q is a canonical key under \"keys\" (line %d)", r.Key, line),
				`remove it from "remove", or remove its rule from "keys"`)
		}
		if canonical, ok := c.canonicalOf[r.Key]; ok {
			return l.fail(errkind.InvalidInput, r.Line, r.Key,
				fmt.Sprintf("%q is an alias of %q", r.Key, canonical),
				`the alias is renamed by the plan; remove it from "remove"`)
		}
	}
	return nil
}
