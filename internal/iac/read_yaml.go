package iac

import (
	"bytes"
	"strings"
	"unicode/utf8"

	yaml "go.yaml.in/yaml/v3"
)

// The reader of CloudFormation templates in YAML (security design §3). Only the
// tree of nodes of yaml/v3 is used, never Go values, so aliases are not
// expanded. The nodes carry lines and columns (the column counts characters);
// the byte positions are worked out from the text, and the text in a range is
// read again and compared with the node before the range is trusted.

// yamlFile is a parsed template.
type yamlFile struct {
	ct        *content
	root      *yaml.Node
	resources *yaml.Node
	// resourcesAt is the index of the Resources key in the root.
	resourcesAt int
	// byKey finds a resource: the index of its key in resources.Content.
	byKey map[string]int
	lines lineTable
}

// parseYAML parses a template. A file without a mapping of Resources at the top
// is parsed all right and simply has no resources.
func parseYAML(ct *content) (parsedFile, *parseFailure) {
	var doc yaml.Node
	if err := yaml.Unmarshal(ct.body, &doc); err != nil {
		return nil, &parseFailure{line: lineFromError(err.Error()), kind: "yaml"}
	}
	f := &yamlFile{ct: ct, byKey: map[string]int{}}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return f, nil
	}
	f.root = doc.Content[0]
	at := findKey(f.root, "Resources")
	if at < 0 || f.root.Content[at+1].Kind != yaml.MappingNode {
		return f, nil
	}
	f.resourcesAt, f.resources = at, f.root.Content[at+1]
	for i := 0; i+1 < len(f.resources.Content); i += 2 {
		if key := f.resources.Content[i]; key.Kind == yaml.ScalarNode {
			if _, seen := f.byKey[key.Value]; !seen {
				f.byKey[key.Value] = i
			}
		}
	}
	return f, nil
}

// findKey returns the index of the key in the content of a mapping, or -1.
func findKey(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			return i
		}
	}
	return -1
}

// countKey counts how many times a key is written in a mapping.
func countKey(m *yaml.Node, key string) int {
	n := 0
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			n++
		}
	}
	return n
}

// hasMergeKey reports a merge key in a mapping.
func hasMergeKey(m *yaml.Node) bool { return findKey(m, "<<") >= 0 }

func (f *yamlFile) refs() []resourceRef {
	if f.resources == nil {
		return nil
	}
	refs := make([]resourceRef, 0, len(f.resources.Content)/2)
	for i := 0; i+1 < len(f.resources.Content); i += 2 {
		key, value := f.resources.Content[i], f.resources.Content[i+1]
		if value.Kind == yaml.AliasNode && value.Alias != nil {
			value = value.Alias
		}
		if key.Kind != yaml.ScalarNode || value.Kind != yaml.MappingNode {
			continue
		}
		if at := findKey(value, "Type"); at >= 0 && value.Content[at+1].Kind == yaml.ScalarNode {
			refs = append(refs, resourceRef{key: key.Value, cloudFormationType: value.Content[at+1].Value, line: key.Line})
		}
	}
	return refs
}

func (f *yamlFile) allTags() []resourceTags {
	refs := f.refs()
	out := make([]resourceTags, 0, len(refs))
	for _, r := range refs {
		out = append(out, resourceTags{key: r.key + "\x00" + r.cloudFormationType, tags: f.site(r.key).signature()})
	}
	return out
}

// lineTable is made when a resource is first looked at: most files of a run
// only give their index.
func (f *yamlFile) table() lineTable {
	if f.lines == nil {
		f.lines = newLineTable(f.ct.raw, f.ct.base, yamlBreak)
	}
	return f.lines
}

// yamlBreak is the line break of the scanner of yaml/v3, whose line numbers the
// table has to follow: CR LF, CR, LF, NEL, LS and PS.
func yamlBreak(raw []byte, i int) int {
	switch raw[i] {
	case '\n':
		return 1
	case '\r':
		if i+1 < len(raw) && raw[i+1] == '\n' {
			return 2
		}
		return 1
	case 0xc2:
		if i+1 < len(raw) && raw[i+1] == 0x85 {
			return 2
		}
	case 0xe2:
		if i+2 < len(raw) && raw[i+1] == 0x80 && (raw[i+2] == 0xa8 || raw[i+2] == 0xa9) {
			return 3
		}
	}
	return 0
}

// offset turns the line and column of a node into a byte position.
func (f *yamlFile) offset(n *yaml.Node) (int, bool) {
	t := f.table()
	if n.Line < 1 || n.Line > t.count() || n.Column < 1 {
		return 0, false
	}
	pos := t.start(n.Line)
	for c := 1; c < n.Column; c++ {
		if pos >= len(f.ct.raw) {
			return 0, false
		}
		_, size := utf8.DecodeRune(f.ct.raw[pos:])
		pos += size
	}
	return pos, true
}

// isAnchored reports an anchor or an alias on the node or below it.
func isAnchored(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return true
	}
	for _, c := range n.Content {
		if isAnchored(c) {
			return true
		}
	}
	return false
}

// isIntrinsicKey reports the key of the long form of a CloudFormation function,
// a mapping of one key such as Ref or Fn::If.
func isIntrinsicKey(key string) bool {
	return key == "Ref" || key == "Condition" || strings.HasPrefix(key, "Fn::")
}

func yamlIntrinsic(m *yaml.Node) bool {
	return m.Kind == yaml.MappingNode && len(m.Content) == 2 && m.Content[0].Kind == yaml.ScalarNode && isIntrinsicKey(m.Content[0].Value)
}

func isTagged(n *yaml.Node) bool { return n.Style&yaml.TaggedStyle != 0 }

func isNullNode(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null" && !isTagged(n)
}

// site finds the place of the tags of a resource (BR4.1).
func (f *yamlFile) site(key string) *tagSite {
	at, ok := f.byKey[key]
	if !ok {
		return nil
	}
	notLiteral := &tagSite{issue: ReasonTagsNotLiteral}
	if countKey(f.resources, key) > 1 {
		return notLiteral
	}
	keyNode, res := f.resources.Content[at], f.resources.Content[at+1]
	if keyNode.Anchor != "" || res.Kind != yaml.MappingNode || res.Anchor != "" || hasMergeKey(res) {
		return notLiteral
	}
	propsAt := findKey(res, "Properties")
	if propsAt < 0 {
		return nil
	}
	props := res.Content[propsAt+1]
	switch {
	case res.Content[propsAt].Anchor != "" || props.Kind == yaml.AliasNode || props.Anchor != "" || isTagged(props) || countKey(res, "Properties") > 1:
		return notLiteral
	case isNullNode(props):
		return nil
	case props.Kind != yaml.MappingNode:
		return &tagSite{issue: ReasonUnsupportedTagShape}
	case hasMergeKey(props) || yamlIntrinsic(props):
		return notLiteral
	}
	tagsAt := findKey(props, "Tags")
	if tagsAt < 0 {
		return nil
	}
	tagsKey, tags := props.Content[tagsAt], props.Content[tagsAt+1]
	if tagsKey.Anchor != "" || countKey(props, "Tags") > 1 {
		return notLiteral
	}
	bound := f.boundAfter(at, res, propsAt, props, tagsAt)
	return f.readTags(tagsKey, tags, bound)
}

// boundAfter returns the line of the first node after the Tags, or one past the
// last line. The elements of the place end before it.
func (f *yamlFile) boundAfter(at int, res *yaml.Node, propsAt int, props *yaml.Node, tagsAt int) int {
	next := func(m *yaml.Node, i int) int {
		if i+2 < len(m.Content) {
			return m.Content[i+2].Line
		}
		return 0
	}
	for _, line := range []int{next(props, tagsAt), next(res, propsAt), next(f.resources, at), next(f.root, f.resourcesAt)} {
		if line > 0 {
			return line
		}
	}
	return f.table().count() + 1
}

// readTags reads the value of Tags.
func (f *yamlFile) readTags(tagsKey, tags *yaml.Node, bound int) *tagSite {
	notLiteral := &tagSite{issue: ReasonTagsNotLiteral}
	switch {
	case tags.Kind == yaml.AliasNode || tags.Anchor != "" || isTagged(tags):
		return notLiteral
	case isNullNode(tags):
		return nil
	case tags.Kind == yaml.SequenceNode:
		return f.readList(tagsKey, tags, bound)
	case tags.Kind != yaml.MappingNode:
		return &tagSite{issue: ReasonUnsupportedTagShape}
	case hasMergeKey(tags) || yamlIntrinsic(tags):
		return notLiteral
	}
	return f.readMap(tagsKey, tags, bound)
}

// readList reads the Key and Value mappings of cfnKeyValueList.
func (f *yamlFile) readList(tagsKey, tags *yaml.Node, bound int) *tagSite {
	s := &tagSite{shape: shapeKeyValueList, line: tagsKey.Line, multiline: tags.Style&yaml.FlowStyle == 0}
	for _, item := range tags.Content {
		switch {
		case isAnchored(item) || isTagged(item) || yamlIntrinsic(item):
			return &tagSite{issue: ReasonTagsNotLiteral}
		case item.Kind != yaml.MappingNode:
			return &tagSite{issue: ReasonUnsupportedTagShape}
		case hasMergeKey(item):
			return &tagSite{issue: ReasonTagsNotLiteral}
		}
		keyAt, valueAt := findKey(item, "Key"), findKey(item, "Value")
		if keyAt < 0 || valueAt < 0 {
			return &tagSite{issue: ReasonUnsupportedTagShape}
		}
		if len(item.Content) > 4 {
			s.extraFields = true
		}
		e := tagEntry{line: item.Content[keyAt].Line}
		if !f.readEntry(&e, item.Content[keyAt+1], item.Content[valueAt+1], countKey(item, "Key") > 1 || countKey(item, "Value") > 1) {
			return &tagSite{issue: ReasonUnreadableFile}
		}
		s.entries = append(s.entries, e)
	}
	return f.finish(s, tagsKey, tags.Content, bound, true)
}

// readMap reads the keys and values of cfnMap.
func (f *yamlFile) readMap(tagsKey, tags *yaml.Node, bound int) *tagSite {
	s := &tagSite{shape: shapeMap, line: tagsKey.Line, multiline: tags.Style&yaml.FlowStyle == 0}
	var first []*yaml.Node
	for i := 0; i+1 < len(tags.Content); i += 2 {
		key, value := tags.Content[i], tags.Content[i+1]
		if isAnchored(key) || isAnchored(value) {
			return &tagSite{issue: ReasonTagsNotLiteral}
		}
		e := tagEntry{line: key.Line}
		if !f.readEntry(&e, key, value, false) {
			return &tagSite{issue: ReasonUnreadableFile}
		}
		s.entries = append(s.entries, e)
		first = append(first, key)
	}
	return f.finish(s, tagsKey, first, bound, false)
}

// readEntry fills the key and the value of an entry. It reports false when a
// position did not check out.
func (f *yamlFile) readEntry(e *tagEntry, key, value *yaml.Node, repeated bool) bool {
	keySpan, keyQuote, keyState := f.readScalar(key)
	valueSpan, valueQuote, valueState := f.readScalar(value)
	if keyState == scalarMismatch || valueState == scalarMismatch {
		return false
	}
	if keyState == scalarLiteral {
		e.key, e.keySpan, e.keyQuote = key.Value, keySpan, keyQuote
	}
	if valueState == scalarLiteral {
		e.value, e.valueSpan, e.valueQuote = value.Value, valueSpan, valueQuote
	}
	e.literal = keyState == scalarLiteral && valueState == scalarLiteral && !repeated
	return true
}

// finish works out the ranges of the elements. nodes are the elements of a list
// or the keys of a map, whose position starts the element.
func (f *yamlFile) finish(s *tagSite, tagsKey *yaml.Node, nodes []*yaml.Node, bound int, list bool) *tagSite {
	s.markDuplicates()
	t := f.table()
	raw := f.ct.raw
	if len(s.entries) == 0 {
		s.others = t.outsideLines(raw, tagsKey.Line, bound, nil)
		return s
	}
	if !s.multiline {
		return s
	}
	starts := make([]int, len(nodes))
	for i, n := range nodes {
		pos, ok := f.offset(n)
		if !ok {
			return &tagSite{issue: ReasonUnreadableFile}
		}
		if list {
			if pos, ok = dashBefore(raw, f.ct.base, pos); !ok {
				s.multiline = false
				return s
			}
		}
		if !blankBefore(raw, t.start(t.lineOf(pos)), pos) {
			s.multiline = false
			return s
		}
		starts[i] = pos
	}
	for i := range s.entries {
		e := &s.entries[i]
		stop := bound
		if i+1 < len(starts) {
			stop = t.lineOf(starts[i+1])
		}
		e.elem = span{starts[i], t.contentEnd(raw, lastContentLine(raw, t, t.lineOf(starts[i]), stop))}
		if e.literal {
			e.hasComment = commentOutside(raw, e)
		}
	}
	s.others = t.outsideLines(raw, tagsKey.Line, bound, s.entries)
	keySpan, _, state := f.readScalar(tagsKey)
	if state != scalarLiteral {
		s.multiline = false
		return s
	}
	colon := bytes.IndexByte(raw[keySpan.end:], ':')
	if colon < 0 {
		s.multiline = false
		return s
	}
	s.value = span{keySpan.end + colon + 1, s.entries[len(s.entries)-1].elem.end}
	return s
}

// dashBefore finds the "-" of a block list item before the item's first key.
func dashBefore(raw []byte, base, pos int) (int, bool) {
	i := pos - 1
	for i >= base && (raw[i] == ' ' || raw[i] == '\t' || isBreakByte(raw[i])) {
		i--
	}
	if i >= base && raw[i] == '-' {
		return i, true
	}
	return 0, false
}

// blankBefore reports whether only spaces come before pos on its line.
func blankBefore(raw []byte, lineStart, pos int) bool {
	return len(bytes.TrimLeft(raw[lineStart:pos], " \t")) == 0
}

// lastContentLine walks up from the line before stop to the last line of an
// element: comments, blank lines and document markers below it are not its own.
func lastContentLine(raw []byte, t lineTable, first, stop int) int {
	for line := min(stop-1, t.count()); line > first; line-- {
		text := bytes.TrimSpace(raw[t.start(line):t.contentEnd(raw, line)])
		if len(text) == 0 || text[0] == '#' || string(text) == "---" || string(text) == "..." {
			continue
		}
		return line
	}
	return first
}

// commentOutside reports a # in the element that is not in its key or value.
func commentOutside(raw []byte, e *tagEntry) bool {
	a, b := e.keySpan, e.valueSpan
	if a.start > b.start {
		a, b = b, a
	}
	outside := [][]byte{raw[e.elem.start:a.start], raw[a.end:b.start], raw[b.end:e.elem.end]}
	for _, text := range outside {
		if bytes.IndexByte(text, '#') >= 0 {
			return true
		}
	}
	return false
}

// scalarState is what readScalar finds.
type scalarState int

const (
	// scalarLiteral is a string on one line, with its range checked.
	scalarLiteral scalarState = iota
	// scalarNotLiteral is anything else that is fine to leave alone.
	scalarNotLiteral
	// scalarMismatch is a position that does not hold what the node says.
	scalarMismatch
)

// readScalar finds the range of a scalar. Tags, block scalars, scalars over
// several lines and empty values are not literal (BR4.2).
func (f *yamlFile) readScalar(n *yaml.Node) (span, quoteStyle, scalarState) {
	if n.Kind != yaml.ScalarNode || isTagged(n) || n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return span{}, quotePlain, scalarNotLiteral
	}
	pos, ok := f.offset(n)
	if !ok {
		return span{}, quotePlain, scalarMismatch
	}
	raw := f.ct.raw
	lineStop := f.table().contentEnd(raw, n.Line)
	switch {
	case n.Style&yaml.SingleQuotedStyle != 0:
		return f.singleQuoted(n, pos, lineStop)
	case n.Style&yaml.DoubleQuotedStyle != 0:
		return f.doubleQuoted(n, pos, lineStop)
	}
	return plainScalar(n, raw, pos, lineStop)
}

// plainScalar checks a scalar without quotes: the value is the same characters
// as the text. A value that only starts the same goes on over lines.
func plainScalar(n *yaml.Node, raw []byte, pos, lineStop int) (span, quoteStyle, scalarState) {
	if n.Value == "" {
		return span{}, quotePlain, scalarNotLiteral
	}
	end := pos + len(n.Value)
	if end <= lineStop && string(raw[pos:end]) == n.Value {
		return span{pos, end}, quotePlain, scalarLiteral
	}
	word, _, _ := strings.Cut(strings.ReplaceAll(n.Value, "\n", " "), " ")
	if wordEnd := pos + len(word); wordEnd <= lineStop && string(raw[pos:wordEnd]) == word {
		return span{}, quotePlain, scalarNotLiteral
	}
	return span{}, quotePlain, scalarMismatch
}

// singleQuoted finds the closing quote on the line and reads the text back.
func (f *yamlFile) singleQuoted(n *yaml.Node, pos, lineStop int) (span, quoteStyle, scalarState) {
	raw := f.ct.raw
	if pos >= lineStop || raw[pos] != '\'' {
		return span{}, quoteSingle, scalarMismatch
	}
	for i := pos + 1; i < lineStop; i++ {
		if raw[i] != '\'' {
			continue
		}
		if i+1 < lineStop && raw[i+1] == '\'' {
			i++
			continue
		}
		text := strings.ReplaceAll(string(raw[pos+1:i]), "''", "'")
		if text != n.Value {
			return span{}, quoteSingle, scalarMismatch
		}
		return span{pos, i + 1}, quoteSingle, scalarLiteral
	}
	return span{}, quoteSingle, scalarNotLiteral
}

// doubleQuoted finds the closing quote on the line and reads the text back,
// with the parser when it holds an escape.
func (f *yamlFile) doubleQuoted(n *yaml.Node, pos, lineStop int) (span, quoteStyle, scalarState) {
	raw := f.ct.raw
	if pos >= lineStop || raw[pos] != '"' {
		return span{}, quoteDouble, scalarMismatch
	}
	for i := pos + 1; i < lineStop; i++ {
		switch raw[i] {
		case '\\':
			i++
		case '"':
			text, ok := readDoubleQuoted(raw[pos : i+1])
			if !ok || text != n.Value {
				return span{}, quoteDouble, scalarMismatch
			}
			return span{pos, i + 1}, quoteDouble, scalarLiteral
		}
	}
	return span{}, quoteDouble, scalarNotLiteral
}

func readDoubleQuoted(fragment []byte) (string, bool) {
	inner := fragment[1 : len(fragment)-1]
	if bytes.IndexByte(inner, '\\') < 0 {
		return string(inner), true
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(fragment, &doc); err != nil || len(doc.Content) != 1 {
		return "", false
	}
	return doc.Content[0].Value, true
}
