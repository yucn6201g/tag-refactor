// Package rules reads and validates the rule file (contracts C1 and C8). In U1
// only the key aliases are used; the other items of the schema are checked
// for shape and reported as not used.
//
// The file is read defensively: only regular files, at most 64 KiB, its
// nesting is bounded before the YAML parser reads it, and the YAML syntax tree
// is checked before it is mapped, so anchors, aliases, merge keys, tags and
// duplicate keys are rejected by the tool itself rather than by library
// settings (NFR8.4).
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

// MaxFileSize is the largest rule file that is read (64 KiB). A rule file
// lists tag keys and needs far less; a small limit also bounds the work of the
// YAML parser.
const MaxFileSize = 64 << 10

// SupportedVersion is the rule file version this tool reads.
const SupportedVersion = 1

// utf8BOM is the byte order mark some editors put at the start of a file.
const utf8BOM = "\xef\xbb\xbf"

// KeyRule is a canonical tag key and its aliases.
type KeyRule struct {
	CanonicalKey string
	// Aliases are in file order; aws: aliases are removed (BR3.3).
	Aliases []string
	// Line is the line of the canonical key in the rule file.
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
	KeyRules   []KeyRule
	Warnings   []Warning
}

// Load reads and validates the rule file at path (BR3.1 to BR3.3).
func Load(path string) (*RuleSet, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
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
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, storageError(path, err)
	}
	if len(data) > MaxFileSize {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRuleFile, Subject: path,
			Detail: "the rule file is larger than 64 KiB", Hint: "a rule file only lists tag keys; reduce it below 64 KiB"}
	}
	return data, nil
}

func storageError(path string, err error) error {
	return &errkind.Error{Kind: errkind.StorageFailed, Reason: errkind.ReasonRuleFile, Subject: path,
		Detail: "the rule file could not be read", Hint: "check that the file is readable", Cause: err}
}

// Parse validates the content of a rule file. path is used in messages.
func Parse(path string, data []byte) (*RuleSet, error) {
	text := normalizeNewlines(string(data))
	text = strings.TrimPrefix(text, utf8BOM)
	l := &loader{path: path, rs: &RuleSet{SourcePath: path, KeyRules: []KeyRule{}, Warnings: []Warning{}}}
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
	if err := l.checkAliases(); err != nil {
		return nil, err
	}
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
	rs   *RuleSet
	// aliasLines holds, for each entry of rs.KeyRules, the line of each alias.
	aliasLines []map[string]int
}

func (l *loader) fail(kind errkind.Kind, line int, key, detail, hint string) error {
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
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRuleFile, Subject: l.path, Line: line,
		Detail: detail, Hint: "fix the YAML syntax at or just before this line", Cause: err}
}

func lineOf(n ast.Node) int {
	if n == nil {
		return 1
	}
	if tok := n.GetToken(); tok != nil && tok.Position != nil {
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
