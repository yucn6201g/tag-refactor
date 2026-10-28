package impact

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Reading a policy document and finding its references to tags (U5 BR1.7,
// BR2.1 to BR2.4, security design §2). The document itself is dropped after
// reading: only the statements' conditions and Resource values are kept, and
// only until the policy has been matched.

// maxDocumentBytes is the largest document that is read (NFR8.20). IAM
// documents are measured before their URL encoding is undone.
const maxDocumentBytes = 1 << 20

// The reasons of documents that cannot be read. They are fixed texts: the
// document's content never reaches a reason (NFR8.22).
const (
	reasonTooLarge   = "the policy document is larger than 1 MiB"
	reasonUnparsable = "the policy document could not be parsed"
)

var (
	errTooLarge  = errors.New(reasonTooLarge)
	errNotPolicy = errors.New(reasonUnparsable)
)

// operatorKind is the kind of a condition operator once ForAnyValue:,
// ForAllValues: and IfExists are taken off (BR2.3).
type operatorKind int

const (
	// opEquals compares strings exactly (StringEquals, StringNotEquals).
	opEquals operatorKind = iota
	// opIgnoreCase compares without letter case (StringEqualsIgnoreCase,
	// StringNotEqualsIgnoreCase).
	opIgnoreCase
	// opPattern compares with * and ? (StringLike, StringNotLike).
	opPattern
	// opNull tests whether the key is present.
	opNull
	// opOther is every operator that does not compare strings (Arn,
	// Numeric, Date, Bool, IpAddress and their negations), which cannot be
	// compared with tag values.
	opOther
)

// setOperator is the set prefix of an operator.
type setOperator int

const (
	setNone setOperator = iota
	setForAnyValue
	setForAllValues
)

// condition is one condition of a statement: one operator and one key.
type condition struct {
	// operator and key are as written.
	operator string
	key      string
	// values holds the values as text; a single value is a list of one, and
	// null is an empty list.
	values []string
	kind   operatorKind
	// negated is true for the string operators with Not.
	negated  bool
	ifExists bool
	set      setOperator
}

// value is the condition value as shown: the values joined with ", ".
func (c *condition) value() string { return strings.Join(c.values, ", ") }

// resourceField is the Resource or NotResource element of a statement.
type resourceField struct {
	// name is "Resource" or "NotResource".
	name   string
	values []string
}

// statement is one statement of a policy, reduced to what matching needs.
type statement struct {
	// id is the Sid, or "#<index>" when the statement has none.
	id         string
	conditions []condition
	resources  []resourceField
}

// parseDocument reads a policy document (BR1.7). IAM returns its documents
// URL-encoded, so urlEncoded undoes that first; resource policies are read as
// returned, so a % in their values stays. Statement may be one object or a
// list. A document larger than maxDocumentBytes, or one that is not a policy,
// is an error, so the policy is reported as unread rather than as unaffected.
func parseDocument(raw string, urlEncoded bool) ([]statement, error) {
	if len(raw) > maxDocumentBytes {
		return nil, errTooLarge
	}
	text := raw
	if urlEncoded {
		if decoded, err := url.PathUnescape(raw); err == nil {
			text = decoded
		}
	}
	var doc struct {
		Statement json.RawMessage `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, errNotPolicy
	}
	raws, err := splitStatements(doc.Statement)
	if err != nil {
		return nil, err
	}
	statements := make([]statement, 0, len(raws))
	for i, r := range raws {
		st, err := parseStatement(r, i)
		if err != nil {
			return nil, err
		}
		statements = append(statements, st)
	}
	return statements, nil
}

// splitStatements accepts a single statement object or a list of them.
func splitStatements(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	switch {
	case len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")):
		return nil, nil
	case trimmed[0] == '[':
		var list []json.RawMessage
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, errNotPolicy
		}
		return list, nil
	case trimmed[0] == '{':
		return []json.RawMessage{trimmed}, nil
	}
	return nil, errNotPolicy
}

func parseStatement(raw json.RawMessage, index int) (statement, error) {
	var st struct {
		Sid         json.RawMessage                       `json:"Sid"`
		Condition   map[string]map[string]json.RawMessage `json:"Condition"`
		Resource    json.RawMessage                       `json:"Resource"`
		NotResource json.RawMessage                       `json:"NotResource"`
	}
	if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] != '{' {
		return statement{}, errNotPolicy
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return statement{}, errNotPolicy
	}
	id, err := statementID(st.Sid, index)
	if err != nil {
		return statement{}, err
	}
	out := statement{id: id}
	for _, field := range []struct {
		name string
		raw  json.RawMessage
	}{{"Resource", st.Resource}, {"NotResource", st.NotResource}} {
		values, present, err := resourceValues(field.raw)
		if err != nil {
			return statement{}, err
		}
		if present {
			out.resources = append(out.resources, resourceField{name: field.name, values: values})
		}
	}
	operators := make([]string, 0, len(st.Condition))
	for op := range st.Condition {
		operators = append(operators, op)
	}
	sort.Strings(operators)
	for _, op := range operators {
		keys := make([]string, 0, len(st.Condition[op]))
		for k := range st.Condition[op] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			values, err := conditionValues(st.Condition[op][k])
			if err != nil {
				return statement{}, err
			}
			out.conditions = append(out.conditions, newCondition(op, k, values))
		}
	}
	return out, nil
}

// statementID is the Sid, or "#<index>" when the statement has none.
func statementID(raw json.RawMessage, index int) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "#" + strconv.Itoa(index), nil
	}
	var sid string
	if err := json.Unmarshal(raw, &sid); err != nil {
		return "", errNotPolicy
	}
	if sid == "" {
		return "#" + strconv.Itoa(index), nil
	}
	return sid, nil
}

// resourceValues reads Resource or NotResource: a string or a list of
// strings. present is false when the element is missing.
func resourceValues(raw json.RawMessage) (values []string, present bool, err error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}
	var one string
	if json.Unmarshal(trimmed, &one) == nil {
		return []string{one}, true, nil
	}
	var list []string
	if err := json.Unmarshal(trimmed, &list); err != nil {
		return nil, false, errNotPolicy
	}
	return list, true, nil
}

// conditionValues reads a condition value: a string, number or boolean, or
// a list of them; null is no value. Numbers keep their text.
func conditionValues(raw json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, errNotPolicy
		}
		values := make([]string, 0, len(list))
		for _, item := range list {
			v, ok, err := scalar(item)
			if err != nil {
				return nil, err
			}
			if ok {
				values = append(values, v)
			}
		}
		return values, nil
	}
	v, ok, err := scalar(trimmed)
	if err != nil || !ok {
		return nil, err
	}
	return []string{v}, nil
}

// scalar renders one JSON string, number or boolean; ok is false for null.
func scalar(raw json.RawMessage) (value string, ok bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false, errNotPolicy
	}
	switch t := v.(type) {
	case nil:
		return "", false, nil
	case string:
		return t, true, nil
	case json.Number:
		return t.String(), true, nil
	case bool:
		return strconv.FormatBool(t), true, nil
	}
	return "", false, errNotPolicy
}

// newCondition classifies the operator (BR2.3). The names are compared
// without letter case; the operator is kept as written for the display.
func newCondition(operator, key string, values []string) condition {
	c := condition{operator: operator, key: key, values: values}
	base := strings.ToLower(operator)
	switch {
	case strings.HasPrefix(base, "foranyvalue:"):
		c.set, base = setForAnyValue, strings.TrimPrefix(base, "foranyvalue:")
	case strings.HasPrefix(base, "forallvalues:"):
		c.set, base = setForAllValues, strings.TrimPrefix(base, "forallvalues:")
	}
	if strings.HasSuffix(base, "ifexists") {
		c.ifExists, base = true, strings.TrimSuffix(base, "ifexists")
	}
	switch base {
	case "stringequals":
		c.kind = opEquals
	case "stringnotequals":
		c.kind, c.negated = opEquals, true
	case "stringequalsignorecase":
		c.kind = opIgnoreCase
	case "stringnotequalsignorecase":
		c.kind, c.negated = opIgnoreCase, true
	case "stringlike":
		c.kind = opPattern
	case "stringnotlike":
		c.kind, c.negated = opPattern, true
	case "null":
		c.kind = opNull
	default:
		// Including the negations of the operators that do not compare
		// strings (ArnNotLike, NotIpAddress, ...): they are not compared.
		c.kind = opOther
	}
	return c
}

// tagKind is what a tag reference refers to (BR2.1).
type tagKind int

const (
	kindResourceTag tagKind = iota
	kindRequestTag
	kindPrincipalTag
	kindTagKeys
)

// location is where a tag reference was found (BR2.4).
type location int

const (
	// locConditionKey is the condition key itself, for example
	// aws:ResourceTag/env.
	locConditionKey location = iota
	// locKeyVariable is a policy variable inside the condition key.
	locKeyVariable
	// locConditionValue is a policy variable inside a condition value.
	locConditionValue
	// locResource and locNotResource are policy variables inside those
	// elements.
	locResource
	locNotResource
)

// isVariable reports whether the reference is a policy variable.
func (l location) isVariable() bool { return l != locConditionKey }

// reference is one reference to a tag in a statement.
type reference struct {
	loc  location
	kind tagKind
	// awsPrefix is true for the global aws: prefix.
	awsPrefix bool
	// tagKey is the tag key, a pattern when it holds * or ?, and unknown
	// when it holds ${. It is empty for TagKeys.
	tagKey string
	// cond is the condition of a reference in a condition; resource is the
	// Resource or NotResource element of the others.
	cond     *condition
	resource *resourceField
}

// prefixPattern is the service prefix of a condition key: "aws" or a service
// name of lowercase letters, digits and hyphens (BR2.1).
var prefixPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// tagMarkers are the condition key names that name a tag key after them.
var tagMarkers = []struct {
	marker string
	kind   tagKind
}{
	{"resourcetag/", kindResourceTag},
	{"requesttag/", kindRequestTag},
	{"principaltag/", kindPrincipalTag},
}

// parseTagKey reads a condition key of the form <prefix>:ResourceTag/<key>,
// RequestTag/<key>, PrincipalTag/<key> or TagKeys (BR2.1). The prefix and
// the names are compared without letter case.
func parseTagKey(key string) (kind tagKind, tagKey string, awsPrefix, ok bool) {
	prefix, rest, found := strings.Cut(key, ":")
	if !found {
		return 0, "", false, false
	}
	prefix = strings.ToLower(prefix)
	if !prefixPattern.MatchString(prefix) {
		return 0, "", false, false
	}
	awsPrefix = prefix == "aws"
	if strings.EqualFold(rest, "TagKeys") {
		return kindTagKeys, "", awsPrefix, true
	}
	for _, m := range tagMarkers {
		if len(rest) > len(m.marker) && strings.EqualFold(rest[:len(m.marker)], m.marker) {
			return m.kind, rest[len(m.marker):], awsPrefix, true
		}
	}
	return 0, "", false, false
}

// variables returns the tag references among the policy variables of text
// (BR2.4): every ${...} whose name, before a default value (", '...'"), is a
// ResourceTag, RequestTag or PrincipalTag key. An unclosed ${ is not a
// variable.
func variables(text string) []reference {
	var refs []reference
	for {
		start := strings.Index(text, "${")
		if start < 0 {
			return refs
		}
		end := strings.IndexByte(text[start:], '}')
		if end < 0 {
			return refs
		}
		name := text[start+2 : start+end]
		text = text[start+end+1:]
		if i := strings.Index(name, ","); i >= 0 {
			name = name[:i]
		}
		kind, tagKey, awsPrefix, ok := parseTagKey(strings.TrimSpace(name))
		if ok && kind != kindTagKeys {
			refs = append(refs, reference{kind: kind, tagKey: tagKey, awsPrefix: awsPrefix})
		}
	}
}

// references returns the tag references of a statement: for each condition
// the condition key, the variables in it and in its values; then the
// variables of Resource and NotResource (BR2.1, BR2.4).
func (st *statement) references() []reference {
	var refs []reference
	for i := range st.conditions {
		c := &st.conditions[i]
		if kind, tagKey, awsPrefix, ok := parseTagKey(c.key); ok {
			refs = append(refs, reference{loc: locConditionKey, kind: kind, tagKey: tagKey, awsPrefix: awsPrefix, cond: c})
		}
		for _, r := range variables(c.key) {
			r.loc, r.cond = locKeyVariable, c
			refs = append(refs, r)
		}
		for _, v := range c.values {
			for _, r := range variables(v) {
				r.loc, r.cond = locConditionValue, c
				refs = append(refs, r)
			}
		}
	}
	for i := range st.resources {
		f := &st.resources[i]
		loc := locResource
		if f.name == "NotResource" {
			loc = locNotResource
		}
		for _, v := range f.values {
			for _, r := range variables(v) {
				r.loc, r.resource = loc, f
				refs = append(refs, r)
			}
		}
	}
	return refs
}
