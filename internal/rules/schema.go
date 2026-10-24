package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Allowed keys at each level of the rule file schema (contract C1). Unknown
// keys are errors so that typos are never ignored (BR1.2).
var (
	rootKeys     = []string{"version", "keys", "required", "remove"}
	keyRuleKeys  = []string{"aliases", "values"}
	requiredKeys = []string{"default", "when"}
	whenKeys     = []string{"match", "value"}
	matchKeys    = []string{"resourceType", "region", "tags"}
)

// regionPattern is the form of an AWS region name (BR3.5). It matches
// awsaccess.IsRegionName; a test keeps them equal.
var regionPattern = regexp.MustCompile(`^[a-z]{2,4}(-[a-z0-9]+)+-[0-9]+$`)

// IsRegionName reports whether name has the form of an AWS region name.
func IsRegionName(name string) bool {
	return regionPattern.MatchString(name)
}

// resourceTypePattern is the form scan shows resource types in (BR3.4): the
// service alone ("s3") or "service:type" ("ec2:instance").
var resourceTypePattern = regexp.MustCompile(`^[a-z0-9-]+(:[^\s:]+)?$`)

const (
	typeFormHint  = `only types that scan shows as "service" or "service:type" (for example s3, sqs, ec2:instance) can be used`
	awsKeyMessage = "keys starting with aws: belong to AWS; "
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
		// A file without key rules is valid but renames nothing; say so rather
		// than report "No changes needed" as if the keys had been checked.
		l.warn(keysLine, "keys", `no key rules: "keys" is missing or empty, so no tag key is renamed`)
	}
	return nil
}

// checkVersion runs first, so a file written for another version is reported
// as such instead of as unknown keys (BR1.4).
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
// how to fix it (BR1.3).
func (l *loader) stringValue(n ast.Node, what string) (string, error) {
	if s, ok := n.(*ast.StringNode); ok {
		return s.Value, nil
	}
	return "", l.fail(errkind.InvalidInput, lineOf(n), scalarText(n), what+" must be a text value",
		"put the value in quotes, for example \"123\"")
}

// tagKey returns a tag key: text of 1 to 128 characters (BR1.5).
func (l *loader) tagKey(n ast.Node, what string) (string, int, error) {
	key, err := l.stringValue(n, what)
	if err != nil {
		return "", 0, err
	}
	line := lineOf(n)
	if key == "" {
		return "", 0, l.fail(errkind.InvalidInput, line, "", "a tag key must not be empty", "remove the empty key")
	}
	if utf8.RuneCountInString(key) > MaxKeyLength {
		return "", 0, l.fail(errkind.InvalidInput, line, key,
			fmt.Sprintf("the tag key %q is longer than %d characters", key, MaxKeyLength),
			fmt.Sprintf("AWS allows tag keys of 1 to %d characters", MaxKeyLength))
	}
	return key, line, nil
}

// tagValue returns a tag value: text of 0 to 256 characters (BR1.5).
func (l *loader) tagValue(n ast.Node, what string) (string, int, error) {
	value, err := l.stringValue(n, what)
	if err != nil {
		return "", 0, err
	}
	line := lineOf(n)
	if utf8.RuneCountInString(value) > MaxValueLength {
		return "", 0, l.fail(errkind.InvalidInput, line, value,
			fmt.Sprintf("the tag value %q is longer than %d characters", value, MaxValueLength),
			fmt.Sprintf("AWS allows tag values of 0 to %d characters", MaxValueLength))
	}
	return value, line, nil
}

// sequence returns the items of a list; a missing list is empty.
func (l *loader) sequence(n ast.Node, what string) ([]ast.Node, error) {
	if isNull(n) {
		return nil, nil
	}
	seq, ok := n.(*ast.SequenceNode)
	if !ok {
		return nil, l.fail(errkind.InvalidInput, lineOf(n), "", what+" must be a list",
			"write the items as [a, b] or one \"- item\" per line")
	}
	return seq.Values, nil
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
		canonical, line, err := l.tagKey(e.Key, "a tag key")
		if err != nil {
			return 0, err
		}
		rule, aliasLines, err := l.loadKeyRule(e, canonical)
		if err != nil {
			return 0, err
		}
		if IsAWSReservedKey(canonical) {
			l.warn(line, canonical, fmt.Sprintf(awsKeyMessage+"the rule for %q is ignored", canonical))
			continue
		}
		rule.CanonicalKey, rule.Line = canonical, line
		l.rs.KeyRules = append(l.rs.KeyRules, rule)
		l.aliasLines = append(l.aliasLines, aliasLines)
	}
	return len(entries), nil
}

// loadKeyRule reads the body of one canonical key: its aliases and values.
func (l *loader) loadKeyRule(e *ast.MappingValueNode, canonical string) (KeyRule, map[string]int, error) {
	rule := KeyRule{Aliases: []string{}, Values: []ValueRule{}}
	lines := map[string]int{}
	entries, err := l.mapping(e.Value, fmt.Sprintf("the rule for %q", canonical), "write \"aliases: [old-name]\" under the key")
	if err != nil {
		return rule, nil, err
	}
	for _, be := range entries {
		name := scalarText(be.Key)
		switch name {
		case "aliases":
			err = l.loadAliases(be.Value, canonical, &rule, lines)
		case "values":
			rule.Values, err = l.loadValues(be, canonical)
		default:
			err = l.unknownKey(be, name, keyRuleKeys)
		}
		if err != nil {
			return rule, nil, err
		}
	}
	return rule, lines, nil
}

func (l *loader) loadAliases(n ast.Node, canonical string, rule *KeyRule, lines map[string]int) error {
	items, err := l.sequence(n, `"aliases"`)
	if err != nil {
		return err
	}
	for _, item := range items {
		alias, line, err := l.tagKey(item, `each item of "aliases"`)
		if err != nil {
			return err
		}
		switch {
		case IsAWSReservedKey(alias):
			l.warn(line, alias, fmt.Sprintf(awsKeyMessage+"the alias %q is ignored", alias))
		case lines[alias] != 0:
			l.warn(line, alias, fmt.Sprintf("the alias %q is listed more than once under %q", alias, canonical))
		default:
			lines[alias] = line
			rule.Aliases = append(rule.Aliases, alias)
		}
	}
	return nil
}

// loadValues reads "values": canonical values and their other spellings,
// and checks BR2.2 within the key.
func (l *loader) loadValues(e *ast.MappingValueNode, key string) ([]ValueRule, error) {
	entries, err := l.mapping(e.Value, `"values"`, "write \"canonical-value: [other-spelling]\" under values")
	if err != nil {
		return nil, err
	}
	values := make([]ValueRule, 0, len(entries))
	spellingLines := make([][]int, 0, len(entries))
	for _, ve := range entries {
		canonical, line, err := l.tagValue(ve.Key, "a tag value")
		if err != nil {
			return nil, err
		}
		items, err := l.sequence(ve.Value, "the spellings of a value")
		if err != nil {
			return nil, err
		}
		v := ValueRule{CanonicalValue: canonical, Spellings: make([]string, 0, len(items)), Line: line}
		lines := make([]int, 0, len(items))
		for _, item := range items {
			s, sl, err := l.tagValue(item, "each spelling of a value")
			if err != nil {
				return nil, err
			}
			v.Spellings = append(v.Spellings, s)
			lines = append(lines, sl)
		}
		values = append(values, v)
		spellingLines = append(spellingLines, lines)
	}
	return l.checkValues(key, values, spellingLines)
}

func (l *loader) loadRequired(e *ast.MappingValueNode) error {
	entries, err := l.mapping(e.Value, `"required"`, "write \"TagKey:\" entries under required")
	if err != nil {
		return err
	}
	for _, re := range entries {
		key, line, err := l.tagKey(re.Key, "a required tag key")
		if err != nil {
			return err
		}
		tag := RequiredTag{Key: key, Line: line, When: []RequiredCondition{}}
		body, err := l.mapping(re.Value, "a required tag", "write \"default:\" or \"when:\" under the key")
		if err != nil {
			return err
		}
		for _, be := range body {
			name := scalarText(be.Key)
			switch name {
			case "default":
				tag.Default, tag.DefaultLine, err = l.tagValue(be.Value, `"default"`)
				tag.HasDefault = true
			case "when":
				tag.When, err = l.loadWhen(be.Value)
			default:
				err = l.unknownKey(be, name, requiredKeys)
			}
			if err != nil {
				return err
			}
		}
		if IsAWSReservedKey(key) {
			l.warn(line, key, fmt.Sprintf(awsKeyMessage+"the required tag %q is ignored", key))
			continue
		}
		l.rs.RequiredTags = append(l.rs.RequiredTags, tag)
	}
	return nil
}

func (l *loader) loadWhen(n ast.Node) ([]RequiredCondition, error) {
	if isNull(n) {
		return []RequiredCondition{}, nil
	}
	seq, ok := n.(*ast.SequenceNode)
	if !ok {
		return nil, l.fail(errkind.InvalidInput, lineOf(n), "when", `"when" must be a list`, "write one \"- match: ...\" entry per condition")
	}
	conditions := make([]RequiredCondition, 0, len(seq.Values))
	for _, item := range seq.Values {
		entries, err := l.mapping(item, `each entry of "when"`, "write \"match:\" and \"value:\" in each entry")
		if err != nil {
			return nil, err
		}
		c := RequiredCondition{Line: lineOf(item), Tags: map[string]string{}, tagLines: map[string]int{}}
		found := map[string]bool{}
		for _, we := range entries {
			name := scalarText(we.Key)
			found[name] = true
			switch name {
			case "match":
				err = l.loadMatch(we, &c)
			case "value":
				c.Value, _, err = l.tagValue(we.Value, `"value"`)
			default:
				err = l.unknownKey(we, name, whenKeys)
			}
			if err != nil {
				return nil, err
			}
		}
		for _, req := range whenKeys {
			if !found[req] {
				return nil, l.fail(errkind.InvalidInput, lineOf(item), req, fmt.Sprintf("an entry of \"when\" has no %q", req),
					fmt.Sprintf("add %q to the entry", req))
			}
		}
		conditions = append(conditions, c)
	}
	return conditions, nil
}

// loadMatch reads one "match" (BR3.3 to BR3.5).
func (l *loader) loadMatch(e *ast.MappingValueNode, c *RequiredCondition) error {
	entries, err := l.mapping(e.Value, `"match"`, "write resourceType, region or tags under match")
	if err != nil {
		return err
	}
	for _, me := range entries {
		name := scalarText(me.Key)
		line := lineOf(me.Key)
		switch name {
		case "resourceType":
			if c.ResourceType, err = l.stringValue(me.Value, `"resourceType"`); err == nil {
				err = l.checkResourceType(c.ResourceType, line)
			}
		case "region":
			if c.Region, err = l.stringValue(me.Value, `"region"`); err == nil && !IsRegionName(c.Region) {
				err = l.fail(errkind.InvalidInput, line, "region", fmt.Sprintf("%q is not an AWS region name", c.Region),
					"write a region name such as us-east-1")
			}
		case "tags":
			err = l.loadMatchTags(me.Value, c)
		default:
			err = l.unknownKey(me, name, matchKeys)
		}
		if err != nil {
			return err
		}
	}
	if c.ResourceType == "" && c.Region == "" && len(c.Tags) == 0 {
		return l.fail(errkind.InvalidInput, lineOf(e.Key), "match", "a condition needs at least one of resourceType, region or tags",
			`use "default" for a value that applies to every resource`)
	}
	return nil
}

func (l *loader) loadMatchTags(n ast.Node, c *RequiredCondition) error {
	tags, err := l.mapping(n, `"tags"`, "write \"key: value\" pairs under tags")
	if err != nil {
		return err
	}
	for _, te := range tags {
		key, line, err := l.tagKey(te.Key, "a tag key")
		if err != nil {
			return err
		}
		value, _, err := l.tagValue(te.Value, "a tag value")
		if err != nil {
			return err
		}
		c.Tags[key] = value
		c.tagLines[key] = line
	}
	return nil
}

// checkResourceType applies BR3.4: the type is written as scan shows it.
func (l *loader) checkResourceType(t string, line int) error {
	switch {
	case strings.HasPrefix(t, "AWS::"):
		if l.opts.TypeHint != nil {
			if arnForm, ok := l.opts.TypeHint(t); ok {
				return l.fail(errkind.InvalidInput, line, "resourceType", "write the resource type as scan shows it",
					fmt.Sprintf("write %q", arnForm))
			}
		}
		return l.fail(errkind.InvalidInput, line, "resourceType", "this resource type cannot be used in a condition", typeFormHint)
	case t == "unknown":
		return l.fail(errkind.InvalidInput, line, "resourceType",
			`"unknown" is not a resource type: scan shows it for resources whose ARN could not be read`, typeFormHint)
	case !resourceTypePattern.MatchString(t):
		return l.fail(errkind.InvalidInput, line, "resourceType", fmt.Sprintf("%q is not a resource type as scan shows it", t), typeFormHint)
	}
	return nil
}

func (l *loader) loadRemove(e *ast.MappingValueNode) error {
	items, err := l.sequence(e.Value, `"remove"`)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, item := range items {
		key, line, err := l.tagKey(item, `each item of "remove"`)
		if err != nil {
			return err
		}
		switch {
		case IsAWSReservedKey(key):
			l.warn(line, key, fmt.Sprintf(awsKeyMessage+"removing %q is ignored", key))
		case seen[key]:
			l.warn(line, key, fmt.Sprintf("the key %q is listed more than once under \"remove\"", key))
		default:
			seen[key] = true
			l.rs.RemoveKeys = append(l.rs.RemoveKeys, RemoveKey{Key: key, Line: line})
		}
	}
	return nil
}

// IsAWSReservedKey reports whether a key starts with "aws:" in any case
// (BR5.1). It matches inventory.IsAWSReservedKey.
func IsAWSReservedKey(key string) bool {
	return len(key) >= 4 && strings.EqualFold(key[:4], "aws:")
}

// NormalizeKey is the key normalization of U2 (lowercase, without "-", "_"
// and space). It matches inventory.NormalizeKey; a test keeps them equal.
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
