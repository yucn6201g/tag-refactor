package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Allowed keys at each level of the rule file schema (contract C1). Unknown
// keys are errors so that typos are never ignored.
var (
	rootKeys     = []string{"version", "keys", "required", "remove"}
	keyRuleKeys  = []string{"aliases", "values"}
	requiredKeys = []string{"default", "when"}
	whenKeys     = []string{"match", "value"}
	matchKeys    = []string{"resourceType", "region", "tags"}
)

func (l *loader) loadRoot(root ast.Node) error {
	entries, ok := mappingEntries(root)
	if !ok {
		return l.fail(errkind.InvalidInput, lineOf(root), "",
			`the rule file must be a mapping with "version" and "keys"`, `start the file with "version: 1"`)
	}
	if err := l.checkVersion(entries); err != nil {
		return err
	}
	keysLine, keyCount := 1, 0
	for _, e := range entries {
		name := scalarText(e.Key)
		var err error
		switch name {
		case "version":
		case "keys":
			keysLine = lineOf(e.Key)
			keyCount, err = l.loadKeys(e.Value)
		case "required":
			err = l.loadRequired(e)
		case "remove":
			err = l.loadRemove(e)
		default:
			err = l.unknownKey(e, name, rootKeys)
		}
		if err != nil {
			return err
		}
	}
	if keyCount == 0 {
		// A file without key rules is valid but plans nothing; say so rather
		// than report "No changes needed" as if the keys had been checked.
		l.warn(keysLine, "keys", `no key rules: "keys" is missing or empty, so no tag key is renamed`)
	}
	return nil
}

// checkVersion runs first, so a file written for another version is reported
// as such instead of as unknown keys.
func (l *loader) checkVersion(entries []*ast.MappingValueNode) error {
	for _, e := range entries {
		if scalarText(e.Key) != "version" {
			continue
		}
		line := lineOf(e.Key)
		n, ok := e.Value.(*ast.IntegerNode)
		if !ok {
			return l.fail(errkind.InvalidInput, line, "version", "version must be a whole number", `write "version: 1"`)
		}
		v, err := strconv.Atoi(n.GetToken().Value)
		if err != nil || v != SupportedVersion {
			return l.fail(errkind.UnsupportedVersion, line, "version",
				fmt.Sprintf("rule file version %s is not supported", n.GetToken().Value),
				fmt.Sprintf("this version of tag-refactor reads rule file version %d", SupportedVersion))
		}
		l.rs.Version = v
		return nil
	}
	return l.fail(errkind.InvalidInput, 1, "version", "the rule file has no version", `add "version: 1" as the first line`)
}

func (l *loader) unknownKey(e *ast.MappingValueNode, name string, allowed []string) error {
	hint := "allowed keys here: " + strings.Join(allowed, ", ")
	if s := suggest(name, allowed); s != "" {
		hint = fmt.Sprintf("did you mean %q? (%s)", s, hint)
	}
	return l.fail(errkind.InvalidInput, lineOf(e.Key), name, fmt.Sprintf("unknown key %q", name), hint)
}

// stringValue returns the string of a scalar node, or a type error that says
// how to fix it.
func (l *loader) stringValue(n ast.Node, what string) (string, error) {
	if s, ok := n.(*ast.StringNode); ok {
		return s.Value, nil
	}
	return "", l.fail(errkind.InvalidInput, lineOf(n), scalarText(n), what+" must be a text value",
		"put the value in quotes, for example \"123\"")
}

// keyName returns the text of a mapping key, which must be a string.
func (l *loader) keyName(e *ast.MappingValueNode, what string) (string, error) {
	return l.stringValue(e.Key, what)
}

func (l *loader) stringList(n ast.Node, what string) ([]string, []int, error) {
	if isNull(n) {
		return nil, nil, nil
	}
	seq, ok := n.(*ast.SequenceNode)
	if !ok {
		return nil, nil, l.fail(errkind.InvalidInput, lineOf(n), "", what+" must be a list",
			"write the items as [a, b] or one \"- item\" per line")
	}
	values := make([]string, 0, len(seq.Values))
	lines := make([]int, 0, len(seq.Values))
	for _, item := range seq.Values {
		s, err := l.stringValue(item, "each item of "+what)
		if err != nil {
			return nil, nil, err
		}
		values = append(values, s)
		lines = append(lines, lineOf(item))
	}
	return values, lines, nil
}

func (l *loader) mapping(n ast.Node, what, hint string) ([]*ast.MappingValueNode, error) {
	if isNull(n) {
		return nil, nil
	}
	entries, ok := mappingEntries(n)
	if !ok {
		return nil, l.fail(errkind.InvalidInput, lineOf(n), "", what+" must be a mapping", hint)
	}
	return entries, nil
}

// loadKeys reads the key rules and returns how many entries "keys" has.
func (l *loader) loadKeys(n ast.Node) (int, error) {
	entries, err := l.mapping(n, `"keys"`, "list each canonical tag key under keys, for example \"Environment:\"")
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		canonical, err := l.keyName(e, "a tag key")
		if err != nil {
			return 0, err
		}
		line := lineOf(e.Key)
		if canonical == "" {
			return 0, l.fail(errkind.InvalidInput, line, "", "a tag key must not be empty", "remove the empty key")
		}
		aliases, aliasLines, err := l.loadKeyRule(e, canonical)
		if err != nil {
			return 0, err
		}
		if IsAWSReservedKey(canonical) {
			l.warn(line, canonical, fmt.Sprintf("keys starting with aws: belong to AWS; the rule for %q is ignored", canonical))
			continue
		}
		rule := KeyRule{CanonicalKey: canonical, Line: line, Aliases: []string{}}
		lines := map[string]int{}
		for i, a := range aliases {
			switch {
			case IsAWSReservedKey(a):
				l.warn(aliasLines[i], a, fmt.Sprintf("keys starting with aws: belong to AWS; the alias %q is ignored", a))
			case lines[a] != 0:
				l.warn(aliasLines[i], a, fmt.Sprintf("the alias %q is listed more than once under %q", a, canonical))
			default:
				lines[a] = aliasLines[i]
				rule.Aliases = append(rule.Aliases, a)
			}
		}
		l.rs.KeyRules = append(l.rs.KeyRules, rule)
		l.aliasLines = append(l.aliasLines, lines)
	}
	return len(entries), nil
}

// loadKeyRule reads the body of one canonical key: its aliases and values.
func (l *loader) loadKeyRule(e *ast.MappingValueNode, canonical string) ([]string, []int, error) {
	entries, err := l.mapping(e.Value, fmt.Sprintf("the rule for %q", canonical), "write \"aliases: [old-name]\" under the key")
	if err != nil {
		return nil, nil, err
	}
	var aliases []string
	var lines []int
	for _, be := range entries {
		name := scalarText(be.Key)
		switch name {
		case "aliases":
			aliases, lines, err = l.stringList(be.Value, `"aliases"`)
		case "values":
			err = l.loadValues(be)
		default:
			err = l.unknownKey(be, name, keyRuleKeys)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return aliases, lines, nil
}

func (l *loader) loadValues(e *ast.MappingValueNode) error {
	entries, err := l.mapping(e.Value, `"values"`, "write \"canonical-value: [other-spelling]\" under values")
	if err != nil {
		return err
	}
	for _, ve := range entries {
		if _, err := l.keyName(ve, "a tag value"); err != nil {
			return err
		}
		if _, _, err := l.stringList(ve.Value, `the spellings of a value`); err != nil {
			return err
		}
	}
	l.warn(lineOf(e.Key), "values", `"values" is not used in this version; tag values are not changed`)
	return nil
}

func (l *loader) loadRequired(e *ast.MappingValueNode) error {
	entries, err := l.mapping(e.Value, `"required"`, "write \"TagKey:\" entries under required")
	if err != nil {
		return err
	}
	for _, re := range entries {
		if _, err := l.keyName(re, "a required tag key"); err != nil {
			return err
		}
		body, err := l.mapping(re.Value, "a required tag", "write \"default:\" or \"when:\" under the key")
		if err != nil {
			return err
		}
		for _, be := range body {
			name := scalarText(be.Key)
			switch name {
			case "default":
				_, err = l.stringValue(be.Value, `"default"`)
			case "when":
				err = l.loadWhen(be.Value)
			default:
				err = l.unknownKey(be, name, requiredKeys)
			}
			if err != nil {
				return err
			}
		}
	}
	l.warn(lineOf(e.Key), "required", `"required" is not used in this version`)
	return nil
}

func (l *loader) loadWhen(n ast.Node) error {
	if isNull(n) {
		return nil
	}
	seq, ok := n.(*ast.SequenceNode)
	if !ok {
		return l.fail(errkind.InvalidInput, lineOf(n), "when", `"when" must be a list`, "write one \"- match: ...\" entry per condition")
	}
	for _, item := range seq.Values {
		entries, err := l.mapping(item, `each entry of "when"`, "write \"match:\" and \"value:\" in each entry")
		if err != nil {
			return err
		}
		found := map[string]bool{}
		for _, we := range entries {
			name := scalarText(we.Key)
			found[name] = true
			switch name {
			case "match":
				err = l.loadMatch(we.Value)
			case "value":
				_, err = l.stringValue(we.Value, `"value"`)
			default:
				err = l.unknownKey(we, name, whenKeys)
			}
			if err != nil {
				return err
			}
		}
		for _, req := range whenKeys {
			if !found[req] {
				return l.fail(errkind.InvalidInput, lineOf(item), req, fmt.Sprintf("an entry of \"when\" has no %q", req),
					fmt.Sprintf("add %q to the entry", req))
			}
		}
	}
	return nil
}

func (l *loader) loadMatch(n ast.Node) error {
	entries, err := l.mapping(n, `"match"`, "write resourceType, region or tags under match")
	if err != nil {
		return err
	}
	for _, me := range entries {
		name := scalarText(me.Key)
		switch name {
		case "resourceType", "region":
			_, err = l.stringValue(me.Value, fmt.Sprintf("%q", name))
		case "tags":
			var tags []*ast.MappingValueNode
			tags, err = l.mapping(me.Value, `"tags"`, "write \"key: value\" pairs under tags")
			for _, te := range tags {
				if err != nil {
					break
				}
				if _, err = l.keyName(te, "a tag key"); err == nil {
					_, err = l.stringValue(te.Value, "a tag value")
				}
			}
		default:
			err = l.unknownKey(me, name, matchKeys)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *loader) loadRemove(e *ast.MappingValueNode) error {
	if _, _, err := l.stringList(e.Value, `"remove"`); err != nil {
		return err
	}
	l.warn(lineOf(e.Key), "remove", `"remove" is not used in this version`)
	return nil
}

// checkAliases enforces BR3.2: an alias belongs to one canonical key only and
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

// IsAWSReservedKey reports whether a key starts with "aws:" in any case
// (BR3.3). It matches inventory.IsAWSReservedKey.
func IsAWSReservedKey(key string) bool {
	return len(key) >= 4 && strings.EqualFold(key[:4], "aws:")
}

// NormalizeKey is the key normalization of BR2.2 (lowercase, without "-",
// "_" and space). It matches inventory.NormalizeKey; a test keeps them equal.
func NormalizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', ' ':
			return -1
		}
		return r
	}, strings.ToLower(key))
}

// suggest returns the allowed key closest to name when it is a likely typo.
func suggest(name string, allowed []string) string {
	best, bestDist := "", 3
	for _, a := range allowed {
		if d := editDistance(strings.ToLower(name), strings.ToLower(a)); d < bestDist {
			best, bestDist = a, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
