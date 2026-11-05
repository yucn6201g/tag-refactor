package iac

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// The reader of CloudFormation templates in JSON (security design §3): the
// tokens of encoding/json, with InputOffset for the ranges of the text. Only
// Resources is kept as a tree; everything else of the file is read and dropped.

// jsonNode is a value of a template.
type jsonNode struct {
	kind jsonKind
	span span
	// str is the text of a string.
	str string
	// keys and vals are the pairs of an object; vals alone the items of an array.
	keys, vals []*jsonNode
}

type jsonKind int

const (
	jsonOther jsonKind = iota
	jsonString
	jsonObject
	jsonArray
)

// get returns the value of the key of an object, and how many times the key is
// written.
func (n *jsonNode) get(key string) (*jsonNode, int) {
	var found *jsonNode
	count := 0
	for i, k := range n.keys {
		if k.str == key {
			if found == nil {
				found = n.vals[i]
			}
			count++
		}
	}
	return found, count
}

// isIntrinsic reports the mapping of one key of a CloudFormation function.
func (n *jsonNode) isIntrinsic() bool {
	return n.kind == jsonObject && len(n.keys) == 1 && isIntrinsicKey(n.keys[0].str)
}

// jsonFile is a parsed template.
type jsonFile struct {
	ct        *content
	resources *jsonNode
	lines     lineTable
}

// jsonParser reads the tokens of the body.
type jsonParser struct {
	dec *json.Decoder
	ct  *content
	// prev is the position, in the body, where the last token ended.
	prev int
}

// parseJSON parses a template. A file without an object of Resources at the top
// is parsed all right and simply has no resources.
func parseJSON(ct *content) (parsedFile, *parseFailure) {
	dec := json.NewDecoder(bytes.NewReader(ct.body))
	dec.UseNumber()
	p := &jsonParser{dec: dec, ct: ct}
	f := &jsonFile{ct: ct}
	if err := p.top(f); err != nil {
		return nil, p.failure(err)
	}
	return f, nil
}

// failure keeps the place and the kind of an error of encoding/json.
func (p *jsonParser) failure(err error) *parseFailure {
	offset := p.dec.InputOffset()
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		offset = syntax.Offset
	}
	t := newLineTable(p.ct.body, 0, newlineBreak)
	pos := min(int(offset), len(p.ct.body))
	line := t.lineOf(pos)
	return &parseFailure{line: line, column: pos - t.start(line) + 1, kind: "json"}
}

// top reads the top level; it keeps only the value of Resources.
func (p *jsonParser) top(f *jsonFile) error {
	tok, err := p.dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); ok && d == '{' {
		for p.dec.More() {
			key, err := p.dec.Token()
			if err != nil {
				return err
			}
			if key == "Resources" {
				p.prev = int(p.dec.InputOffset())
				if f.resources, err = p.value(); err != nil {
					return err
				}
				continue
			}
			if err := p.skip(); err != nil {
				return err
			}
		}
	}
	if err := p.skipRest(tok); err != nil {
		return err
	}
	if _, err := p.dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("data after the value")
		}
		return err
	}
	return nil
}

// skipRest reads what is left of the top value, which top started with tok.
func (p *jsonParser) skipRest(tok json.Token) error {
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	depth := 1
	if d == '{' {
		// The fields were read; only the closing bracket is left.
		_, err := p.dec.Token()
		return err
	}
	for depth > 0 {
		next, err := p.dec.Token()
		if err != nil {
			return err
		}
		if nd, ok := next.(json.Delim); ok {
			if nd == '{' || nd == '[' {
				depth++
			} else {
				depth--
			}
		}
	}
	return nil
}

// skip reads one value without keeping it.
func (p *jsonParser) skip() error {
	depth := 0
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else {
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}

// next reads a token and its range in the raw file: from the first character
// after the spaces, commas and colons that come before it.
func (p *jsonParser) next() (json.Token, span, error) {
	tok, err := p.dec.Token()
	if err != nil {
		return nil, span{}, err
	}
	end := int(p.dec.InputOffset())
	start := p.prev
	for start < end && isJSONFiller(p.ct.body[start]) {
		start++
	}
	p.prev = end
	return tok, span{start + p.ct.base, end + p.ct.base}, nil
}

func isJSONFiller(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ':'
}

// value reads one value as a tree. The nesting check has bounded the depth.
func (p *jsonParser) value() (*jsonNode, error) {
	tok, s, err := p.next()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		n := &jsonNode{kind: jsonArray, span: s}
		if v == '{' {
			n.kind = jsonObject
		}
		if err := p.children(n); err != nil {
			return nil, err
		}
		_, closing, err := p.next()
		if err != nil {
			return nil, err
		}
		n.span.end = closing.end
		return n, nil
	case string:
		return &jsonNode{kind: jsonString, span: s, str: v}, nil
	}
	return &jsonNode{kind: jsonOther, span: s}, nil
}

func (p *jsonParser) children(n *jsonNode) error {
	for p.dec.More() {
		if n.kind == jsonObject {
			key, err := p.value()
			if err != nil {
				return err
			}
			n.keys = append(n.keys, key)
		}
		val, err := p.value()
		if err != nil {
			return err
		}
		n.vals = append(n.vals, val)
	}
	return nil
}

func (f *jsonFile) table() lineTable {
	if f.lines == nil {
		f.lines = newLineTable(f.ct.raw, f.ct.base, newlineBreak)
	}
	return f.lines
}

func (f *jsonFile) refs() []resourceRef {
	if f.resources == nil || f.resources.kind != jsonObject {
		return nil
	}
	refs := make([]resourceRef, 0, len(f.resources.keys))
	for i, key := range f.resources.keys {
		res := f.resources.vals[i]
		if res.kind != jsonObject {
			continue
		}
		if t, _ := res.get("Type"); t != nil && t.kind == jsonString {
			refs = append(refs, resourceRef{key: key.str, cloudFormationType: t.str, line: f.table().lineOf(key.span.start)})
		}
	}
	return refs
}

func (f *jsonFile) allTags() []resourceTags {
	refs := f.refs()
	out := make([]resourceTags, 0, len(refs))
	for _, r := range refs {
		out = append(out, resourceTags{key: r.key + "\x00" + r.cloudFormationType, tags: f.site(r.key).signature()})
	}
	return out
}

// site finds the place of the tags of a resource (BR4.1).
func (f *jsonFile) site(key string) *tagSite {
	if f.resources == nil || f.resources.kind != jsonObject {
		return nil
	}
	res, count := f.resources.get(key)
	notLiteral := &tagSite{issue: ReasonTagsNotLiteral}
	switch {
	case res == nil:
		return nil
	case count > 1:
		return notLiteral
	case res.kind != jsonObject:
		return &tagSite{issue: ReasonUnsupportedTagShape}
	}
	props, count := res.get("Properties")
	switch {
	case props == nil:
		return nil
	case count > 1 || props.isIntrinsic():
		return notLiteral
	case props.kind != jsonObject:
		return &tagSite{issue: ReasonUnsupportedTagShape}
	}
	tags, count := props.get("Tags")
	switch {
	case tags == nil:
		return nil
	case count > 1 || tags.isIntrinsic():
		return notLiteral
	case tags.kind == jsonArray:
		return f.readList(tags)
	case tags.kind == jsonObject:
		return f.readMap(tags)
	}
	if string(f.ct.raw[tags.span.start:tags.span.end]) == "null" {
		return nil
	}
	return &tagSite{issue: ReasonUnsupportedTagShape}
}

func (f *jsonFile) readList(tags *jsonNode) *tagSite {
	s := &tagSite{shape: shapeKeyValueList, value: tags.span, line: f.table().lineOf(tags.span.start)}
	for _, item := range tags.vals {
		if item.isIntrinsic() {
			return &tagSite{issue: ReasonTagsNotLiteral}
		}
		if item.kind != jsonObject {
			return &tagSite{issue: ReasonUnsupportedTagShape}
		}
		key, keys := item.get("Key")
		value, values := item.get("Value")
		if key == nil || value == nil {
			return &tagSite{issue: ReasonUnsupportedTagShape}
		}
		if len(item.keys) > 2 {
			s.extraFields = true
		}
		e := tagEntry{line: f.table().lineOf(key.span.start), elem: item.span}
		if !f.readEntry(&e, key, value, keys > 1 || values > 1) {
			return &tagSite{issue: ReasonUnreadableFile}
		}
		s.entries = append(s.entries, e)
	}
	return f.finish(s, tags)
}

func (f *jsonFile) readMap(tags *jsonNode) *tagSite {
	s := &tagSite{shape: shapeMap, value: tags.span, line: f.table().lineOf(tags.span.start)}
	for i, key := range tags.keys {
		value := tags.vals[i]
		e := tagEntry{line: f.table().lineOf(key.span.start), elem: span{key.span.start, value.span.end}}
		if !f.readEntry(&e, key, value, false) {
			return &tagSite{issue: ReasonUnreadableFile}
		}
		s.entries = append(s.entries, e)
	}
	return f.finish(s, tags)
}

// readEntry fills the key and value of an entry, reading the text of each range
// back as a JSON string. It reports false when one does not check out.
func (f *jsonFile) readEntry(e *tagEntry, key, value *jsonNode, repeated bool) bool {
	if key.kind == jsonString {
		if !f.check(key) {
			return false
		}
		e.key, e.keySpan, e.keyQuote = key.str, key.span, quoteDouble
	}
	if value.kind == jsonString {
		if !f.check(value) {
			return false
		}
		e.value, e.valueSpan, e.valueQuote = value.str, value.span, quoteDouble
	}
	e.literal = key.kind == jsonString && value.kind == jsonString && !repeated
	return true
}

func (f *jsonFile) check(n *jsonNode) bool {
	var text string
	return json.Unmarshal(f.ct.raw[n.span.start:n.span.end], &text) == nil && text == n.str
}

// finish says whether each element has lines of its own (BR4.4).
func (f *jsonFile) finish(s *tagSite, tags *jsonNode) *tagSite {
	s.markDuplicates()
	t := f.table()
	s.multiline = t.lineOf(tags.span.start) != t.lineOf(tags.span.end-1)
	for i := 1; i < len(s.entries) && s.multiline; i++ {
		if t.lineOf(s.entries[i].elem.start) == t.lineOf(s.entries[i-1].elem.end-1) {
			s.multiline = false
		}
	}
	return s
}
