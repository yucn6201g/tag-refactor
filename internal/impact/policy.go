package impact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// condition is one resource-tag condition of a policy statement, reduced to
// what matching and display need. The policy document itself is dropped after
// parsing (scalability design).
type condition struct {
	statementID string
	operator    string
	// key is the condition key as written, for example aws:ResourceTag/env.
	key string
	// value is the condition value; several values are joined with ", ".
	value string
	// tagKey is the tag key part of the condition key.
	tagKey string
	// isAWSPrefix is true for the global aws: prefix.
	isAWSPrefix bool
}

var errNotPolicy = errors.New("not a policy document")

// prefixPattern is the service prefix of a condition key: "aws" or a service
// name of lowercase letters, digits and hyphens (BR5.2).
var prefixPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

const resourceTagMarker = "resourcetag/"

// resourceTagKey extracts the tag key of a "<prefix>:ResourceTag/<key>"
// condition key. The prefix and "ResourceTag" are compared without case.
func resourceTagKey(conditionKey string) (tagKey string, isAWSPrefix, ok bool) {
	prefix, rest, found := strings.Cut(conditionKey, ":")
	if !found {
		return "", false, false
	}
	prefix = strings.ToLower(prefix)
	if !prefixPattern.MatchString(prefix) {
		return "", false, false
	}
	if len(rest) <= len(resourceTagMarker) || !strings.EqualFold(rest[:len(resourceTagMarker)], resourceTagMarker) {
		return "", false, false
	}
	return rest[len(resourceTagMarker):], prefix == "aws", true
}

type statement struct {
	Sid       json.RawMessage                       `json:"Sid"`
	Condition map[string]map[string]json.RawMessage `json:"Condition"`
}

// parsePolicy reads a policy document as IAM returns it (URL-encoded JSON)
// and returns its resource-tag conditions in statement order, then operator
// and key order. A document that cannot be read is an error, so the policy
// is reported as unread rather than as unaffected.
func parsePolicy(raw string) ([]condition, error) {
	text := raw
	if decoded, err := url.PathUnescape(raw); err == nil {
		text = decoded
	}
	var doc struct {
		Statement json.RawMessage `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	statements, err := splitStatements(doc.Statement)
	if err != nil {
		return nil, err
	}
	var out []condition
	for i, raw := range statements {
		var st statement
		if err := json.Unmarshal(raw, &st); err != nil {
			return nil, err
		}
		id, err := statementID(st.Sid, i)
		if err != nil {
			return nil, err
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
				tagKey, isAWS, ok := resourceTagKey(k)
				if !ok {
					continue
				}
				value, err := conditionValue(st.Condition[op][k])
				if err != nil {
					return nil, err
				}
				out = append(out, condition{statementID: id, operator: op, key: k, value: value, tagKey: tagKey, isAWSPrefix: isAWS})
			}
		}
	}
	return out, nil
}

// splitStatements accepts a single statement object or a list of them.
func splitStatements(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	switch {
	case len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")):
		return nil, nil
	case trimmed[0] == '[':
		var list []json.RawMessage
		err := json.Unmarshal(trimmed, &list)
		return list, err
	case trimmed[0] == '{':
		return []json.RawMessage{trimmed}, nil
	}
	return nil, errNotPolicy
}

// statementID is the Sid, or "#<index>" when the statement has none.
func statementID(raw json.RawMessage, index int) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "#" + strconv.Itoa(index), nil
	}
	var sid string
	if err := json.Unmarshal(raw, &sid); err != nil {
		return "", err
	}
	if sid == "" {
		return "#" + strconv.Itoa(index), nil
	}
	return sid, nil
}

// conditionValue renders a condition value; lists are joined with ", ".
func conditionValue(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", "), nil
	case map[string]any:
		return "", errNotPolicy
	default:
		return fmt.Sprint(t), nil
	}
}
