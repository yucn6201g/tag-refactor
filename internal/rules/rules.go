// Package rules reads and validates the rule file (contracts C1 and C8),
// answers the questions U4's plan asks about it (the value of a required tag
// for a resource, the canonical value of a tag value, whether a key is
// removed), and builds the draft rule file of rules suggest.
//
// The file is read defensively: only regular files; UTF-8, or UTF-16 with a
// byte order mark converted to UTF-8; at most 64 KiB once in UTF-8; its
// nesting is bounded before the YAML parser reads it, and the YAML syntax tree
// is checked before it is mapped, so anchors, aliases, merge keys, tags and
// duplicate keys are rejected by the tool itself rather than by library
// settings (NFR8.4, NFR9.5). The package depends on errkind only: what it
// needs from the inventory comes in as function values (Options.TypeHint,
// SuggestInput.KeyGroup).
package rules

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// MaxFileSize is the largest rule file that is read, measured in UTF-8
// (64 KiB). A rule file lists tag keys and needs far less; a small limit also
// bounds the work of the YAML parser.
const MaxFileSize = 64 << 10

// MaxInputSize is the largest input Parse accepts before decoding (128 KiB):
// a UTF-16 file takes up to twice the bytes of its UTF-8 form (NFR9.5).
const MaxInputSize = 128 << 10

// SupportedVersion is the rule file version this tool reads.
const SupportedVersion = 1

// The AWS limits on tags, counted in characters (BR1.5).
const (
	MaxKeyLength   = 128
	MaxValueLength = 256
)

// Options changes how a rule file is read.
type Options struct {
	// TypeHint returns the ARN form ("ec2:instance") of a CloudFormation
	// resource type ("AWS::EC2::Instance"). It is used only in the fix of an
	// error (BR3.4). Nil treats every CloudFormation type as one that cannot
	// be used in a condition.
	TypeHint func(cfnType string) (arnForm string, ok bool)
}

// KeyRule is a canonical tag key, its aliases and its canonical values.
type KeyRule struct {
	CanonicalKey string
	// Aliases are in file order; aws: aliases and repeats are removed.
	Aliases []string
	// Values are in file order (BR2.2).
	Values []ValueRule
	// Line is the line of the canonical key in the rule file.
	Line int
}

// ValueRule is a canonical tag value and its other spellings (BR2.2).
type ValueRule struct {
	CanonicalValue string
	// Spellings are in file order; repeats and the value itself are removed.
	Spellings []string
	// Line is the line of the canonical value; 0 in a draft.
	Line int
}

// RequiredTag is a key listed under "required" and how its value is decided
// (BR3.2).
type RequiredTag struct {
	Key string
	// Default is the value for resources no condition matches; it is set
	// when HasDefault is true.
	Default     string
	HasDefault  bool
	DefaultLine int
	// When are the conditions in file order; the first that matches wins.
	When []RequiredCondition
	// Line is the line of the key in the rule file.
	Line int
}

// RequiredCondition is one entry of "when". Every field that is set must
// match (BR3.2); at least one is set (BR3.3).
type RequiredCondition struct {
	// ResourceType is the type as scan shows it ("ec2:instance", "s3"), or "".
	ResourceType string
	// Region is a region name, or "".
	Region string
	// Tags are compared with the tags after aliases and value spellings are
	// unified and removed keys are taken away. Keys starting with aws: are
	// allowed here (BR5.1).
	Tags  map[string]string
	Value string
	// Line is the line of the entry ("- match:").
	Line int
	// tagLines holds the line of each key of Tags.
	tagLines map[string]int
}

// RemoveKey is a key listed under "remove".
type RemoveKey struct {
	Key  string
	Line int
}

// Warning is a problem that does not stop the command.
type Warning struct {
	Line    int
	Key     string
	Message string
}

// RuleSet is a loaded rule file.
type RuleSet struct {
	SourcePath string
	Version    int
	// KeyRules, RequiredTags and RemoveKeys are in file order.
	KeyRules     []KeyRule
	RequiredTags []RequiredTag
	RemoveKeys   []RemoveKey
	Warnings     []Warning
	// idx holds the lookup tables of the queries; Parse builds it.
	idx *index
}

// Load reads and validates the rule file at path (BR1.1 to BR5.1). It only
// checks that the path is a regular file and reads at most MaxInputSize+1
// bytes; Parse does everything else, so the fuzz test of Parse reaches every
// check (security design §4).
func Load(path string, opts Options) (*RuleSet, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data, opts)
}

func readLimited(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &errkind.Error{Kind: errkind.NotFound, Reason: errkind.ReasonRuleFile, Subject: path,
				Detail: "the rule file does not exist", Hint: "check the path given to --rules", Cause: err}
		}
		return nil, storageError(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRuleFile, Subject: path,
			Detail: "the rule file is not a regular file", Hint: "pass the path of a YAML file to --rules"}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, storageError(path, err)
	}
	defer func() { _ = f.Close() }()
	// Read at most one byte more than the limit, and judge by what was read,
	// not by the size the file system reports.
	data, err := io.ReadAll(io.LimitReader(f, MaxInputSize+1))
	if err != nil {
		return nil, storageError(path, err)
	}
	return data, nil
}

func storageError(path string, err error) error {
	return &errkind.Error{Kind: errkind.StorageFailed, Reason: errkind.ReasonRuleFile, Subject: path,
		Detail: "the rule file could not be read", Hint: "check that the file is readable", Cause: err}
}

// Parse validates the content of a rule file. path is used in messages.
func Parse(path string, data []byte, opts Options) (*RuleSet, error) {
	l := &loader{path: path, opts: opts, rs: &RuleSet{
		SourcePath: path, KeyRules: []KeyRule{}, RequiredTags: []RequiredTag{}, RemoveKeys: []RemoveKey{}, Warnings: []Warning{},
	}}
	text, err := l.decode(data)
	if err != nil {
		return nil, err
	}
	text = normalizeNewlines(text)
	l.lines = strings.Count(text, "\n") + 1
	if err := l.checkUTF8(text); err != nil {
		return nil, err
	}
	// The parser's cost grows with the square of the nesting depth, so deep
	// nesting is rejected before it runs.
	if err := l.checkNesting(text); err != nil {
		return nil, err
	}
	file, err := parser.ParseBytes([]byte(text), 0, parser.AllowDuplicateMapKey())
	if err != nil {
		return nil, l.syntaxError(err)
	}
	var docs []*ast.DocumentNode
	for _, d := range file.Docs {
		if d != nil && d.Body != nil {
			if _, isComment := d.Body.(*ast.CommentGroupNode); !isComment {
				docs = append(docs, d)
			}
		}
	}
	if len(docs) == 0 {
		return nil, l.fail(errkind.InvalidInput, 1, "", "the rule file is empty", `start the file with "version: 1"`)
	}
	if len(docs) > 1 {
		return nil, l.fail(errkind.InvalidInput, lineOf(docs[1].Body), "",
			"the rule file contains more than one YAML document", `remove the "---" separator and keep one document`)
	}
	if err := l.checkTree(docs[0].Body, 0); err != nil {
		return nil, err
	}
	if err := l.loadRoot(docs[0].Body); err != nil {
		return nil, err
	}
	if err := l.checkConsistency(); err != nil {
		return nil, err
	}
	l.rs.idx = newIndex(l.rs)
	return l.rs, nil
}

// normalizeNewlines turns CRLF and lone CR into LF, so files written on any
// OS read the same and line numbers do not depend on the newline (NFR9.1).
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

type loader struct {
	path string
	opts Options
	rs   *RuleSet
	// lines is the number of lines of the decoded text; 0 before decoding.
	lines int
	// aliasLines holds, for each entry of rs.KeyRules, the line of each alias.
	aliasLines []map[string]int
}

// fail returns a rule file error. A line past the end of the file (a syntax
// error at the end, for example) is reported as the last line, so the line
// always exists (fuzz test invariant).
func (l *loader) fail(kind errkind.Kind, line int, key, detail, hint string) error {
	if l.lines > 0 && line > l.lines {
		line = l.lines
	}
	return &errkind.Error{Kind: kind, Reason: errkind.ReasonRuleFile, Subject: l.path, Line: line, Key: key, Detail: detail, Hint: hint}
}

func (l *loader) warn(line int, key, message string) {
	l.rs.Warnings = append(l.rs.Warnings, Warning{Line: line, Key: key, Message: message})
}

func (l *loader) syntaxError(err error) error {
	line := 1
	detail := "the file is not valid YAML"
	var yerr yaml.Error
	if errors.As(err, &yerr) {
		if tok := yerr.GetToken(); tok != nil && tok.Position != nil && tok.Position.Line > 0 {
			line = tok.Position.Line
		}
		if msg := yerr.GetMessage(); msg != "" {
			detail += ": " + msg
		}
	}
	e := l.fail(errkind.InvalidInput, line, "", detail, "fix the YAML syntax at or just before this line").(*errkind.Error)
	e.Cause = err
	return e
}

func lineOf(n ast.Node) int {
	if n == nil {
		return 1
	}
	if tok := n.GetToken(); tok != nil && tok.Position != nil && tok.Position.Line > 0 {
		return tok.Position.Line
	}
	return 1
}

// maxDepth bounds the nesting of the YAML tree; the rule file schema needs
// fewer than ten levels.
const maxDepth = 32

// checkTree rejects YAML features the rule file never needs (anchors,
// aliases, merge keys, tags), duplicate keys and deep nesting (NFR8.4).
func (l *loader) checkTree(n ast.Node, depth int) error {
	if n == nil {
		return nil
	}
	if depth > maxDepth {
		return l.fail(errkind.InvalidInput, lineOf(n), "", "the YAML is nested too deeply", "a rule file needs only a few levels of nesting")
	}
	switch v := n.(type) {
	case *ast.AnchorNode:
		return l.fail(errkind.InvalidInput, lineOf(n), "", "YAML anchors (&) are not allowed in a rule file", "write the values out instead of using anchors")
	case *ast.AliasNode:
		return l.fail(errkind.InvalidInput, lineOf(n), "", "YAML aliases (*) are not allowed in a rule file", "write the values out instead of using aliases")
	case *ast.MergeKeyNode:
		return l.fail(errkind.InvalidInput, lineOf(n), "<<", "YAML merge keys (<<) are not allowed in a rule file", "write the keys out instead of merging")
	case *ast.TagNode:
		return l.fail(errkind.InvalidInput, lineOf(n), "", "YAML tags (!) are not allowed in a rule file", "remove the tag; use plain or quoted values")
	case *ast.MappingKeyNode:
		return l.fail(errkind.InvalidInput, lineOf(n), "", "complex mapping keys (?) are not allowed in a rule file", "use plain key: value pairs")
	case *ast.MappingNode:
		return l.checkMapping(v.Values, depth)
	case *ast.MappingValueNode:
		return l.checkMapping([]*ast.MappingValueNode{v}, depth)
	case *ast.SequenceNode:
		for _, item := range v.Values {
			if err := l.checkTree(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *loader) checkMapping(entries []*ast.MappingValueNode, depth int) error {
	seen := map[string]int{}
	for _, e := range entries {
		if err := l.checkTree(e.Key, depth+1); err != nil {
			return err
		}
		name := scalarText(e.Key)
		line := lineOf(e.Key)
		if first, dup := seen[name]; dup {
			return l.fail(errkind.InvalidInput, line, name,
				fmt.Sprintf("key %q appears twice in the same mapping (lines %d and %d)", name, first, line),
				"keep only one of them")
		}
		seen[name] = line
		if err := l.checkTree(e.Value, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// scalarText returns the text of a scalar node as written.
func scalarText(n ast.Node) string {
	if n == nil {
		return ""
	}
	if s, ok := n.(*ast.StringNode); ok {
		return s.Value
	}
	if tok := n.GetToken(); tok != nil {
		return tok.Value
	}
	return ""
}

func mappingEntries(n ast.Node) ([]*ast.MappingValueNode, bool) {
	switch v := n.(type) {
	case *ast.MappingNode:
		return v.Values, true
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{v}, true
	}
	return nil, false
}

func isNull(n ast.Node) bool {
	if n == nil {
		return true
	}
	_, ok := n.(*ast.NullNode)
	return ok
}
