package iac

import "bytes"

// The check before a parser sees a file (NFR3.19 (f), security design §2): a
// parser that recurses on deep nesting can use up the stack of Go, which cannot
// be recovered from. Each scan is one pass in time proportional to the file;
// it knows just enough of the format to skip strings, comments, block scalars
// and heredocs, where brackets mean nothing.

// The limits of the scans. A level costs a parser a few kilobytes of stack, so
// these stay far from the limit of one gigabyte.
const (
	maxNesting      = 256
	maxUnaryRun     = 256
	maxConditionals = 10000
)

// tooDeep reports whether the content of a file nests past the limits.
func tooDeep(format fileFormat, data []byte) bool {
	switch format {
	case formatJSON:
		return jsonTooDeep(data)
	case formatTerraform:
		return hclTooDeep(data)
	}
	return yamlTooDeep(data)
}

// jsonTooDeep counts the depth of [ and {, outside strings.
func jsonTooDeep(data []byte) bool {
	depth := 0
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case inString:
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == '[' || c == '{':
			depth++
			if depth > maxNesting {
				return true
			}
		case c == ']' || c == '}':
			depth--
		}
	}
	return false
}

// yamlScan is the state of the scan of a YAML file.
type yamlScan struct {
	data []byte
	// indents is the stack of the indents of the block structure.
	indents []int
	// flow is the depth of [ and {.
	flow int
	// inSingle and inDouble are true inside a quoted scalar, which can go on
	// over lines.
	inSingle, inDouble bool
	// blockIndent is the indent a line must pass to belong to the block scalar
	// being read, or -1 outside one.
	blockIndent int
	// afterDash is where the value after the "- " of this line starts, or -1.
	afterDash int
}

// yamlTooDeep counts (a) the depth of [ and {, (b) the steps of the indent and
// (c) the "- " and "? " that start a line, added to (b).
func yamlTooDeep(data []byte) bool {
	s := &yamlScan{data: data, blockIndent: -1, afterDash: -1}
	for pos := 0; pos < len(data); {
		end := bytes.IndexByte(data[pos:], '\n')
		if end < 0 {
			end = len(data)
		} else {
			end += pos
		}
		if s.line(pos, end) {
			return true
		}
		pos = end + 1
	}
	return false
}

// line scans the line data[start:end] and reports whether it passes a limit.
func (s *yamlScan) line(start, end int) bool {
	i := start
	if s.inSingle || s.inDouble {
		i = s.skipQuoted(i, end)
		if s.inSingle || s.inDouble {
			return false
		}
		return s.rest(i, end, false)
	}
	indent := 0
	for i < end && s.data[i] == ' ' {
		i++
		indent++
	}
	blank := i >= end || s.data[i] == '\r' || s.data[i] == '#'
	if s.blockIndent >= 0 {
		if blank && (i >= end || s.data[i] == '\r') || indent > s.blockIndent {
			return false
		}
		s.blockIndent = -1
	}
	if blank {
		return false
	}
	if s.flow == 0 {
		for len(s.indents) > 0 && s.indents[len(s.indents)-1] >= indent {
			s.indents = s.indents[:len(s.indents)-1]
		}
		s.indents = append(s.indents, indent)
		if len(s.indents) > maxNesting {
			return true
		}
	}
	// A run of "- " and "? " starts one level each.
	run := 0
	for i+1 < end && (s.data[i] == '-' || s.data[i] == '?') && (s.data[i+1] == ' ' || s.data[i+1] == '\t') {
		run++
		i += 2
		for i < end && (s.data[i] == ' ' || s.data[i] == '\t') {
			i++
		}
	}
	if s.flow == 0 && len(s.indents)+run > maxNesting {
		return true
	}
	s.afterDash = -1
	if s.flow == 0 && run > 0 {
		// The structure of the line sits deeper than its first character.
		s.indents = append(s.indents, i-start)
		s.afterDash = i
	}
	return s.rest(i, end, true)
}

// rest scans the line after its indent. tokenStart says that a new value can
// start at i, so a quote or a bracket there means something.
func (s *yamlScan) rest(i, end int, tokenStart bool) bool {
	inPlain := false
	for i < end {
		c := s.data[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			if i+1 < end && s.data[i+1] == '#' {
				return false
			}
			i++
			continue
		case c == '#' && tokenStart:
			return false
		case c == ':' && (i+1 >= end || s.data[i+1] == ' ' || s.data[i+1] == '\t' || s.data[i+1] == '\r'):
			tokenStart, inPlain = true, false
		case s.flow > 0 && (c == ',' || c == '[' || c == '{'):
			if c == ',' {
				tokenStart, inPlain = true, false
				break
			}
			s.flow++
			if s.flow > maxNesting {
				return true
			}
			tokenStart, inPlain = true, false
		case s.flow > 0 && (c == ']' || c == '}'):
			s.flow--
			tokenStart, inPlain = false, false
		case inPlain:
		case !tokenStart:
			inPlain = true
		case c == '[' || c == '{':
			s.flow++
			if s.flow > maxNesting {
				return true
			}
		case c == '"':
			s.inDouble = true
			i = s.skipQuoted(i+1, end)
			if s.inDouble {
				return false
			}
			tokenStart = false
			continue
		case c == '\'':
			s.inSingle = true
			i = s.skipQuoted(i+1, end)
			if s.inSingle {
				return false
			}
			tokenStart = false
			continue
		case c == '!' || c == '&' || c == '*':
			// A tag, an anchor or an alias: a value can still follow it.
			for i < end && s.data[i] != ' ' && s.data[i] != '\t' {
				i++
			}
			continue
		case (c == '|' || c == '>') && s.flow == 0 && blockHeader(s.data[i+1:end]):
			s.blockIndent = s.blockParent(i)
			return false
		default:
			tokenStart, inPlain = false, true
		}
		i++
	}
	return false
}

// blockParent returns the indent of the node that holds the block scalar whose
// header is at position i: the content of the scalar is indented past it. A key
// is the structure last counted; for "- |" it is the dash, two characters back.
func (s *yamlScan) blockParent(i int) int {
	top := 0
	if len(s.indents) > 0 {
		top = s.indents[len(s.indents)-1]
	}
	if i == s.afterDash {
		return max(top-2, 0)
	}
	return top
}

// blockHeader reports whether the text after | or > is the rest of a block
// scalar's header: indicators, and then only a comment.
func blockHeader(rest []byte) bool {
	i := 0
	for i < len(rest) && (rest[i] == '+' || rest[i] == '-' || rest[i] >= '0' && rest[i] <= '9') {
		i++
	}
	for i < len(rest) {
		switch rest[i] {
		case ' ', '\t', '\r':
			i++
		case '#':
			return i > 0
		default:
			return false
		}
	}
	return true
}

// skipQuoted goes on inside a quoted scalar and returns where it ends or end.
func (s *yamlScan) skipQuoted(i, end int) int {
	for i < end {
		c := s.data[i]
		switch {
		case s.inDouble && c == '\\':
			i += 2
			continue
		case s.inDouble && c == '"':
			s.inDouble = false
			return i + 1
		case s.inSingle && c == '\'':
			if i+1 < end && s.data[i+1] == '\'' {
				i += 2
				continue
			}
			s.inSingle = false
			return i + 1
		}
		i++
	}
	return end
}

// hclFrame is one level of the scan of a Terraform file: code, a quoted string
// or a heredoc.
type hclFrame struct {
	kind hclKind
	// braces counts the { opened in this code, to find the } that closes ${.
	braces int
	// marker is the end of a heredoc.
	marker []byte
}

type hclKind int

const (
	hclCode hclKind = iota
	hclString
	hclHeredoc
)

// hclScan is the state of the scan of a Terraform file.
type hclScan struct {
	data         []byte
	stack        []hclFrame
	depth        int
	unaryRun     int
	conditionals int
}

// hclTooDeep counts (a) the depth of [, {, (, ${ and %{, (b) the length of a
// run of the unary operators ! and -, and (c) the ? of the conditional
// expressions in the whole file, whose chains the parser recurses on. A heredoc
// skips the brackets of its text, but not its ${ and %{, which the parser reads
// as code.
func hclTooDeep(data []byte) bool {
	s := &hclScan{data: data, stack: []hclFrame{{kind: hclCode}}}
	for i := 0; i < len(data); {
		next, deep := s.step(i)
		if deep {
			return true
		}
		i = next
	}
	return false
}

// step scans one element from position i and returns where the next one starts.
func (s *hclScan) step(i int) (int, bool) {
	switch s.stack[len(s.stack)-1].kind {
	case hclString:
		return s.stringStep(i)
	case hclHeredoc:
		return s.heredocStep(i)
	}
	return s.codeStep(i)
}

func (s *hclScan) open() bool {
	s.depth++
	return s.depth > maxNesting
}

func (s *hclScan) codeStep(i int) (int, bool) {
	data := s.data
	c := data[i]
	if c != ' ' && c != '\t' && c != '\r' && c != '\n' && c != '!' && c != '-' {
		s.unaryRun = 0
	}
	top := &s.stack[len(s.stack)-1]
	switch c {
	case '"':
		s.stack = append(s.stack, hclFrame{kind: hclString})
	case '#':
		return lineEnd(data, i), false
	case '/':
		if i+1 < len(data) && data[i+1] == '/' {
			return lineEnd(data, i), false
		}
		if i+1 < len(data) && data[i+1] == '*' {
			if end := bytes.Index(data[i+2:], []byte("*/")); end >= 0 {
				return i + 2 + end + 2, false
			}
			return len(data), false
		}
	case '<':
		if marker, next := heredocStart(data, i); marker != nil {
			s.stack = append(s.stack, hclFrame{kind: hclHeredoc, marker: marker})
			return next, false
		}
	case '[', '(':
		return i + 1, s.open()
	case '{':
		top.braces++
		return i + 1, s.open()
	case ']', ')':
		s.depth--
	case '}':
		s.depth--
		if top.braces > 0 {
			top.braces--
		} else if len(s.stack) > 1 {
			s.stack = s.stack[:len(s.stack)-1]
		}
	case '!', '-':
		s.unaryRun++
		return i + 1, s.unaryRun > maxUnaryRun
	case '?':
		s.conditionals++
		return i + 1, s.conditionals > maxConditionals
	}
	return i + 1, false
}

func (s *hclScan) stringStep(i int) (int, bool) {
	data := s.data
	switch data[i] {
	case '\\':
		return i + 2, false
	case '"', '\n':
		s.stack = s.stack[:len(s.stack)-1]
		return i + 1, false
	}
	return s.template(i)
}

// template handles the ${ and %{ of a string or a heredoc: doubled ones are
// text, and others start code.
func (s *hclScan) template(i int) (int, bool) {
	data := s.data
	if i+2 < len(data) && (data[i] == '$' || data[i] == '%') && data[i+1] == data[i] && data[i+2] == '{' {
		return i + 3, false
	}
	if i+1 < len(data) && (data[i] == '$' || data[i] == '%') && data[i+1] == '{' {
		s.stack = append(s.stack, hclFrame{kind: hclCode})
		return i + 2, s.open()
	}
	return i + 1, false
}

func (s *hclScan) heredocStep(i int) (int, bool) {
	data := s.data
	top := s.stack[len(s.stack)-1]
	if i == 0 || data[i-1] == '\n' {
		end := lineEnd(data, i)
		if bytes.Equal(bytes.TrimSpace(data[i:end]), top.marker) {
			s.stack = s.stack[:len(s.stack)-1]
			return end, false
		}
	}
	return s.template(i)
}

// heredocStart reads <<EOT or <<-EOT at position i, up to the end of the line:
// it returns the marker and the start of the next line.
func heredocStart(data []byte, i int) ([]byte, int) {
	j := i + 1
	if j >= len(data) || data[j] != '<' {
		return nil, 0
	}
	j++
	if j < len(data) && data[j] == '-' {
		j++
	}
	end := lineEnd(data, j)
	marker := bytes.TrimSpace(data[j:end])
	if len(marker) == 0 || bytes.ContainsAny(marker, " \t") {
		return nil, 0
	}
	return marker, min(end+1, len(data))
}

// lineEnd returns the position of the newline that ends the line of i, or the
// end of the data.
func lineEnd(data []byte, i int) int {
	if end := bytes.IndexByte(data[i:], '\n'); end >= 0 {
		return i + end
	}
	return len(data)
}
