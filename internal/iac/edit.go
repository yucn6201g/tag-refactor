package iac

import (
	"bytes"
	"errors"
	"slices"
	"strings"
)

// edit replaces the bytes of a range of the file with text (Edit). With an
// empty range it inserts; with empty text it removes.
type edit struct {
	start, end int
	text       string
}

// editBuilder makes the edits of one target (security design §4).
type editBuilder struct {
	src    []byte
	format fileFormat
	site   *tagSite
	// final is the list after the changes.
	final []simEntry
	// kept says, by index of the site, where an original entry went; nil when it
	// was removed. last is the last kept index, or -1.
	kept []*simEntry
	last int
	// added are the entries new to the place.
	added []simEntry
	nl    string
}

// buildEdits compares the list after the changes with the place as it is and
// returns the edits, in no particular order.
func buildEdits(src []byte, format fileFormat, site *tagSite, final []simEntry) []edit {
	if site == nil {
		return nil
	}
	b := &editBuilder{src: src, format: format, site: site, final: final, last: -1, nl: fileLineBreak(src)}
	b.kept = make([]*simEntry, len(site.entries))
	for i := range final {
		e := &final[i]
		if e.orig < 0 {
			b.added = append(b.added, *e)
			continue
		}
		b.kept[e.orig] = e
		b.last = max(b.last, e.orig)
	}
	edits := b.replacements()
	if b.last < 0 && len(site.entries) > 0 {
		return append(edits, b.wholePlace())
	}
	switch format {
	case formatJSON:
		edits = append(edits, b.removeJSON()...)
	default:
		edits = append(edits, b.removeLines()...)
	}
	if len(b.added) > 0 {
		at := site.entries[b.last].elem.end
		var text strings.Builder
		sep := b.separator(&site.entries[b.last])
		for _, e := range b.added {
			text.WriteString(sep)
			text.WriteString(b.copied(e))
		}
		edits = append(edits, edit{start: at, end: at, text: text.String()})
	}
	return edits
}

// replacements are the edits of the keys and values of the kept entries.
func (b *editBuilder) replacements() []edit {
	var edits []edit
	for i, e := range b.kept {
		if e == nil {
			continue
		}
		o := &b.site.entries[i]
		if e.key != o.key {
			edits = append(edits, edit{o.keySpan.start, o.keySpan.end, writeText(b.format, o.keyQuote, e.key, true)})
		}
		if e.value != o.value {
			edits = append(edits, edit{o.valueSpan.start, o.valueSpan.end, writeText(b.format, o.valueQuote, e.value, false)})
		}
	}
	return edits
}

// copied writes a new tag in the form of the copy source (BR4.5).
func (b *editBuilder) copied(e simEntry) string {
	source := b.site.copySource()
	src := b.src
	type part struct {
		s    span
		text string
	}
	parts := []part{
		{source.keySpan, writeText(b.format, source.keyQuote, e.key, true)},
		{source.valueSpan, writeText(b.format, source.valueQuote, e.value, false)},
	}
	if parts[0].s.start > parts[1].s.start {
		parts[0], parts[1] = parts[1], parts[0]
	}
	var out strings.Builder
	out.Write(src[source.elem.start:parts[0].s.start])
	out.WriteString(parts[0].text)
	out.Write(src[parts[0].s.end:parts[1].s.start])
	out.WriteString(parts[1].text)
	out.Write(src[parts[1].s.end:source.elem.end])
	return out.String()
}

// removeLines removes the lines of the removed elements, in YAML and Terraform.
// When the last of them is the end of a file without a final line break, the
// line break before them goes instead, so the file still ends without one.
func (b *editBuilder) removeLines() []edit {
	var edits []edit
	for i, e := range b.kept {
		if e != nil {
			continue
		}
		o := b.site.entries[i].elem
		start, end := lineHead(b.src, o.start), lineTail(b.src, o.end)
		if n := len(edits); n > 0 && edits[n-1].end == start {
			edits[n-1].end = end
			continue
		}
		edits = append(edits, edit{start: start, end: end})
	}
	if n := len(edits); n > 0 && edits[n-1].end == len(b.src) && !isBreakByte(b.src[len(b.src)-1]) {
		edits[n-1].start = breakBefore(b.src, edits[n-1].start)
	}
	return edits
}

// lineHead returns where the line of pos starts.
func lineHead(src []byte, pos int) int {
	for pos > 0 && !isBreakByte(src[pos-1]) {
		pos--
	}
	return pos
}

// lineTail returns where the line after pos starts.
func lineTail(src []byte, pos int) int {
	for pos < len(src) && !isBreakByte(src[pos]) {
		pos++
	}
	switch {
	case pos+1 < len(src) && src[pos] == '\r' && src[pos+1] == '\n':
		return pos + 2
	case pos < len(src):
		return pos + 1
	}
	return pos
}

// breakBefore returns where the line break before pos starts.
func breakBefore(src []byte, pos int) int {
	if pos >= 2 && src[pos-2] == '\r' && src[pos-1] == '\n' {
		return pos - 2
	}
	if pos >= 1 {
		return pos - 1
	}
	return pos
}

// removeJSON removes the removed elements with their commas (security design
// §4): an element with a kept one after it goes up to the start of the next
// element, and the elements after the last kept one go together, from the end
// of the last kept one, so the array stays valid.
func (b *editBuilder) removeJSON() []edit {
	var edits []edit
	entries := b.site.entries
	for i, e := range b.kept {
		if e == nil && i < b.last {
			edits = append(edits, edit{start: entries[i].elem.start, end: entries[i+1].elem.start})
		}
	}
	if b.last < len(entries)-1 {
		edits = append(edits, edit{start: entries[b.last].elem.end, end: entries[len(entries)-1].elem.end})
	}
	return edits
}

// wholePlace replaces the value of a place whose elements are all removed with
// its empty form, or with the new elements alone.
func (b *editBuilder) wholePlace() edit {
	value := b.site.value
	if len(b.added) == 0 {
		return edit{value.start, value.end, b.emptyForm()}
	}
	entries := b.site.entries
	var text strings.Builder
	text.Write(b.src[value.start:entries[0].elem.start])
	sep := b.separator(&entries[0])
	for i, e := range b.added {
		if i > 0 {
			text.WriteString(sep)
		}
		text.WriteString(b.copied(e))
	}
	text.Write(b.src[entries[len(entries)-1].elem.end:value.end])
	return edit{value.start, value.end, text.String()}
}

func (b *editBuilder) emptyForm() string {
	list, spaced := "[]", ""
	if b.site.shape == shapeMap {
		list = "{}"
	}
	if b.format == formatYAML {
		spaced = " "
	}
	return spaced + list
}

// separator returns what goes before a new element: the text between two
// original elements, when one is only spaces (and the comma of JSON), from its
// last line break; otherwise a line break and the indent of the element.
func (b *editBuilder) separator(anchor *tagEntry) string {
	entries := b.site.entries
	for i := 0; i+1 < len(entries); i++ {
		if s, ok := b.cleanGap(b.src[entries[i].elem.end:entries[i+1].elem.start]); ok {
			return s
		}
	}
	head := lineHead(b.src, anchor.elem.start)
	indent := indentOf(b.src[head:anchor.elem.start])
	if b.format == formatJSON {
		if head <= b.site.value.start {
			return ", "
		}
		return "," + b.nl + indent
	}
	return b.nl + indent
}

// indentOf returns the spaces at the start of text.
func indentOf(text []byte) string {
	return string(text[:len(text)-len(bytes.TrimLeft(text, " \t"))])
}

// cleanGap reduces the text between two elements to what a new element needs.
func (b *editBuilder) cleanGap(gap []byte) (string, bool) {
	comma := b.format == formatJSON
	rest := gap
	if comma {
		rest = bytes.Replace(gap, []byte(","), nil, 1)
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return "", false
	}
	at := bytes.LastIndexByte(gap, '\n')
	if at < 0 {
		return "", false
	}
	if at > 0 && gap[at-1] == '\r' {
		at--
	}
	if comma {
		return "," + string(gap[at:]), true
	}
	return string(gap[at:]), true
}

// fileLineBreak is the first line break of the file.
func fileLineBreak(src []byte) string {
	at := bytes.IndexByte(src, '\n')
	if at > 0 && src[at-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// canReplaceWhole reports whether the value of a place can be replaced when all
// its elements go: comments and blank lines in it would go with them (security
// design §4).
func canReplaceWhole(src []byte, format fileFormat, site *tagSite) bool {
	entries := site.entries
	gaps := [][]byte{src[site.value.start:entries[0].elem.start], src[entries[len(entries)-1].elem.end:site.value.end]}
	for i := 0; i+1 < len(entries); i++ {
		gaps = append(gaps, src[entries[i].elem.end:entries[i+1].elem.start])
	}
	for _, gap := range gaps {
		text := gap
		if format == formatJSON {
			text = bytes.ReplaceAll(gap, []byte(","), nil)
		}
		if len(bytes.TrimSpace(text)) != 0 || bytes.Count(text, []byte("\n")) > 1 {
			return false
		}
	}
	return true
}

var errEditsOverlap = errors.New("the edits overlap")

// normalize sorts the edits and joins a removal and an insertion that start at
// the same place into one (security design §4). Edits that still overlap are a
// fault of this tool.
func normalize(edits []edit) ([]edit, error) {
	slices.SortStableFunc(edits, func(a, b edit) int {
		if a.start != b.start {
			return a.start - b.start
		}
		// The insertion comes first, so its text goes before the removal.
		return a.end - b.end
	})
	out := edits[:0:0]
	for _, e := range edits {
		if n := len(out); n > 0 {
			prev := &out[n-1]
			if prev.start == e.start && prev.start == prev.end {
				prev.end, prev.text = e.end, prev.text+e.text
				continue
			}
			if prev.end > e.start {
				return nil, errEditsOverlap
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// applyEdits makes the new content from normalized edits.
func applyEdits(src []byte, edits []edit) []byte {
	size := len(src)
	for _, e := range edits {
		size += len(e.text) - (e.end - e.start)
	}
	out := make([]byte, 0, size)
	at := 0
	for _, e := range edits {
		out = append(out, src[at:e.start]...)
		out = append(out, e.text...)
		at = e.end
	}
	return append(out, src[at:]...)
}

// expectedTags is what a target's place must hold after the edits.
type expectedTags struct {
	key    string
	final  []simEntry
	others string
}

// checkEdits reads the new content again and checks it (BR4.6, security design
// §4): (a) the targets have the tags of the list, (b) the resources are the same
// and the others have the same tags, (c) the bytes outside the edits are the
// same, (d) the lines of a place that are no element are the same. It returns
// what failed, for --debug, or nothing.
func checkEdits(ct *content, updated []byte, edits []edit, targets []expectedTags, before []resourceTags) string {
	if !outsideUnchanged(ct.raw, updated, edits) {
		return "(c) bytes outside the edits changed"
	}
	next := &content{raw: updated, body: updated[ct.base:], base: ct.base, format: ct.format}
	parsed, failure := parseContent(next)
	if failure != nil {
		return "the file does not parse after the edits"
	}
	isTarget := make(map[string]bool, len(targets))
	for _, t := range targets {
		isTarget[t.key] = true
		site := parsed.site(t.key)
		if !sameTags(site, t.final) {
			return "(a) the tags of a target are not the expected ones"
		}
		if site != nil && site.others != t.others {
			return "(d) the lines between the elements of a target changed"
		}
	}
	after := parsed.allTags()
	if len(after) != len(before) {
		return "(b) the number of resources changed"
	}
	for i := range after {
		if after[i].key != before[i].key {
			return "(b) the resources changed"
		}
		if !isTarget[resourceKeyOf(after[i].key)] && after[i].tags != before[i].tags {
			return "(b) the tags of another resource changed"
		}
	}
	return ""
}

// resourceKeyOf takes the logical ID or the address from the key of allTags.
func resourceKeyOf(key string) string {
	id, _, _ := strings.Cut(key, "\x00")
	return id
}

// sameTags compares a place with the list of a target.
func sameTags(site *tagSite, final []simEntry) bool {
	if site == nil {
		return len(final) == 0
	}
	if len(site.entries) != len(final) {
		return false
	}
	for i := range final {
		e, f := &site.entries[i], &final[i]
		if e.key != f.key || e.value != f.value || e.literal != f.literal {
			return false
		}
	}
	return true
}

// outsideUnchanged compares the bytes outside the edits, from the positions.
func outsideUnchanged(src, updated []byte, edits []edit) bool {
	at, shift := 0, 0
	for _, e := range edits {
		if !bytes.Equal(src[at:e.start], updated[at+shift:e.start+shift]) {
			return false
		}
		shift += len(e.text) - (e.end - e.start)
		at = e.end
	}
	return bytes.Equal(src[at:], updated[at+shift:])
}
