package rules

import (
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// maxHiddenBrackets bounds the opening brackets that the nesting scan does
// not count as nesting: those in quoted values, comments, block scalars and
// plain text. A rule file needs none of them (tag keys and values cannot
// contain brackets), and the bound means that a quote the scan misjudges can
// hide at most this many levels from it, which the parser handles quickly.
const maxHiddenBrackets = 1024

// checkNesting bounds the nesting of the rule file before the YAML parser
// reads it (NFR8.4). The parser uses time and memory that grow with the square
// of the nesting depth, so a small file of brackets could stop the tool or
// exhaust its memory before checkTree's depth check runs on the parsed tree.
//
// It is one pass over the text that follows the YAML rules closely enough for
// this purpose: it counts flow collections ("[" and "{") outside quoted
// scalars, comments and block scalars, and the levels of block indentation
// including compact entries such as "- - -". Where it may misjudge the text,
// it errs towards rejecting: the brackets it does not count are limited by
// maxHiddenBrackets. checkTree still checks the parsed tree exactly.
func (l *loader) checkNesting(text string) error {
	s := nestingScan{blockParent: noBlockIndent, blockIndent: noBlockIndent}
	for i, ln := range strings.Split(text, "\n") {
		if problem := s.scanLine(ln); problem != "" {
			return l.fail(errkind.InvalidInput, i+1, "", problem, nestingHint(problem))
		}
	}
	return nil
}

const (
	tooDeep          = "the YAML is nested too deeply"
	tooManyBrackets  = "the rule file has too many brackets inside quoted values, comments or text"
	nestingFixHint   = "a rule file needs only a few levels of nesting"
	bracketsFixHint  = "tag keys and values never contain brackets; remove them"
	noBlockIndent    = -1
	quoteNone        = 0
	singleQuote      = '\''
	doubleQuote      = '"'
	escapeCharacter  = '\\'
	commentCharacter = '#'
)

func nestingHint(problem string) string {
	if problem == tooManyBrackets {
		return bracketsFixHint
	}
	return nestingFixHint
}

// nestingScan is the state of checkNesting between lines.
type nestingScan struct {
	// flow is the depth of open flow collections.
	flow int
	// indents holds the columns of the open block levels.
	indents []int
	// quote is the quote character of an open quoted scalar, or quoteNone.
	quote byte
	// nodeStart is true where a quote would start a quoted scalar (YAML only
	// allows that at the start of a node).
	nodeStart bool
	// jsonNode is true just after a quoted scalar or a flow collection ends;
	// in a flow collection a ":" right after it is a value indicator.
	jsonNode bool
	// blockParent is the indentation of the line holding a block scalar
	// indicator ("|" or ">"), or noBlockIndent outside a block scalar;
	// blockIndent is the indentation of the block scalar's content once known.
	blockParent int
	blockIndent int
	// hidden counts the opening brackets not counted as nesting.
	hidden int
}

// scanLine scans one line and returns a problem, or "" when the line is fine.
func (s *nestingScan) scanLine(ln string) string {
	indent := len(ln) - len(strings.TrimLeft(ln, " "))
	if s.inBlockScalar(ln, indent) {
		return s.hide(strings.Count(ln, "[") + strings.Count(ln, "{"))
	}
	pos := 0
	if s.quote == quoteNone && s.flow == 0 {
		trimmed := strings.TrimLeft(ln, " \t")
		if trimmed == "" || trimmed[0] == commentCharacter || trimmed[0] == '%' {
			return s.hide(strings.Count(ln, "[") + strings.Count(ln, "{"))
		}
		var problem string
		if pos, problem = s.blockLine(ln, indent); problem != "" {
			return problem
		}
	}
	return s.scanText(ln, pos)
}

// inBlockScalar reports whether the line is content of an open block scalar,
// ending the block scalar at the first line that is not.
func (s *nestingScan) inBlockScalar(ln string, indent int) bool {
	if s.blockParent == noBlockIndent || s.quote != quoteNone {
		return false
	}
	if strings.TrimSpace(ln) == "" {
		return true
	}
	if s.blockIndent == noBlockIndent && indent > s.blockParent {
		s.blockIndent = indent
	}
	if s.blockIndent != noBlockIndent && indent >= s.blockIndent {
		return true
	}
	s.blockParent, s.blockIndent = noBlockIndent, noBlockIndent
	return false
}

// blockLine handles the start of a line in block context: a document marker,
// the indentation, and the "- " and "? " indicators of compact entries. It
// returns where the rest of the line starts.
func (s *nestingScan) blockLine(ln string, indent int) (int, string) {
	s.nodeStart, s.jsonNode = true, false
	pos := indent
	if indent == 0 && (strings.HasPrefix(ln, "---") || strings.HasPrefix(ln, "...")) && isBlankAt(ln, 3) {
		s.indents = s.indents[:0]
		return 3, ""
	}
	for {
		s.pushIndent(pos)
		if len(s.indents) > maxDepth {
			return pos, tooDeep
		}
		if pos >= len(ln) || (ln[pos] != '-' && ln[pos] != '?') || !isBlankAt(ln, pos+1) {
			return pos, ""
		}
		pos++
		for pos < len(ln) && (ln[pos] == ' ' || ln[pos] == '\t') {
			pos++
		}
		if pos >= len(ln) || ln[pos] == commentCharacter {
			return pos, ""
		}
	}
}

// pushIndent opens a block level at column col, closing the levels at the
// same or a deeper column.
func (s *nestingScan) pushIndent(col int) {
	for len(s.indents) > 0 && s.indents[len(s.indents)-1] >= col {
		s.indents = s.indents[:len(s.indents)-1]
	}
	s.indents = append(s.indents, col)
}

// scanText scans the line from pos for quotes, comments and flow brackets.
func (s *nestingScan) scanText(ln string, pos int) string {
	afterBlank := true
	for i := pos; i < len(ln); i++ {
		c := ln[i]
		if s.quote != quoteNone {
			var problem string
			if i, problem = s.quoted(ln, i); problem != "" {
				return problem
			}
			afterBlank = false
			continue
		}
		if c == ' ' || c == '\t' {
			afterBlank = true
			continue
		}
		if c == commentCharacter && afterBlank {
			return s.hide(strings.Count(ln[i:], "[") + strings.Count(ln[i:], "{"))
		}
		var problem string
		if i, problem = s.token(ln, i); problem != "" {
			return problem
		}
		afterBlank = false
	}
	return ""
}

// token handles one character outside quotes and comments and returns the
// index of the last character it consumed.
func (s *nestingScan) token(ln string, i int) (int, string) {
	c := ln[i]
	wasJSONNode := s.jsonNode
	s.jsonNode = false
	switch {
	case (c == singleQuote || c == doubleQuote) && s.nodeStart:
		s.quote, s.nodeStart = c, false
	case (c == '[' || c == '{') && (s.flow > 0 || s.nodeStart):
		s.flow++
		s.nodeStart = true
		if s.flow > maxDepth {
			return i, tooDeep
		}
	case (c == ']' || c == '}') && s.flow > 0:
		s.flow--
		s.nodeStart, s.jsonNode = false, true
	case c == ',' && s.flow > 0:
		s.nodeStart = true
	case c == ':' && (isBlankAt(ln, i+1) || (s.flow > 0 && (wasJSONNode || strings.IndexByte(",[]{}", ln[i+1]) >= 0))):
		s.nodeStart = true
	case (c == '-' || c == '?') && s.nodeStart && isBlankAt(ln, i+1):
		// An indicator; a node may still start after it.
	case (c == '&' || c == '!') && s.nodeStart:
		// An anchor or a tag comes before the node it belongs to.
		for i+1 < len(ln) && ln[i+1] != ' ' && ln[i+1] != '\t' {
			i++
		}
	case (c == '|' || c == '>') && s.nodeStart && s.flow == 0:
		s.blockParent = len(ln) - len(strings.TrimLeft(ln, " "))
		s.blockIndent = noBlockIndent
		return len(ln), s.hide(strings.Count(ln[i:], "[") + strings.Count(ln[i:], "{"))
	default:
		if c == '[' || c == '{' {
			// A bracket inside plain text, for example "a[b" outside a flow collection.
			if problem := s.hide(1); problem != "" {
				return i, problem
			}
		}
		s.nodeStart = false
	}
	return i, ""
}

// quoted handles one character inside a quoted scalar and returns the index
// of the last character it consumed.
func (s *nestingScan) quoted(ln string, i int) (int, string) {
	switch c := ln[i]; {
	case c == escapeCharacter && s.quote == doubleQuote:
		return i + 1, ""
	case c == singleQuote && s.quote == singleQuote && i+1 < len(ln) && ln[i+1] == singleQuote:
		return i + 1, ""
	case c == s.quote:
		s.quote = quoteNone
		s.jsonNode = true
	case c == '[' || c == '{':
		return i, s.hide(1)
	}
	return i, ""
}

// hide counts n opening brackets that are not nesting.
func (s *nestingScan) hide(n int) string {
	s.hidden += n
	if s.hidden > maxHiddenBrackets {
		return tooManyBrackets
	}
	return ""
}

// isBlankAt reports whether position i of ln is a space, a tab or the end of
// the line.
func isBlankAt(ln string, i int) bool {
	return i >= len(ln) || ln[i] == ' ' || ln[i] == '\t'
}
