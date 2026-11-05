package iac

import (
	"context"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sync/semaphore"

	"github.com/yucn6201g/tag-refactor/internal/restype"
)

// The readers of the three formats have one shape (logical-components.md):
// parsing a file gives the index of its resources, the place of the tags of one
// resource, and every resource's tags for the check after the edits. Applying
// changes, making edits and the difference know nothing else of a format.

// span is a range of bytes of the file as it is on disk.
type span struct{ start, end int }

// quoteStyle is how a key or a value is written.
type quoteStyle int

const (
	quotePlain quoteStyle = iota
	quoteSingle
	quoteDouble
)

// tagShape is the shape of the place of the tags (TagSite.shape).
type tagShape int

const (
	shapeKeyValueList tagShape = iota
	shapeMap
)

// tagEntry is one tag of a place (TagEntry).
type tagEntry struct {
	// key is empty when the key itself is not a plain string.
	key, value string
	// literal is true when the key and the value are strings written directly
	// on one line (BR4.2).
	literal bool
	line    int
	// keySpan and valueSpan include the quotes; only literal entries have them.
	keySpan, valueSpan   span
	keyQuote, valueQuote quoteStyle
	// elem is the whole element, with the comment at the end of its line in
	// YAML and Terraform.
	elem span
	// hasComment is true when elem holds a comment, so it is not copied.
	hasComment bool
}

// tagSite is the place of the tags of one resource (TagSite).
type tagSite struct {
	shape   tagShape
	entries []tagEntry
	// issue, when set, stops every change of the resource: tagsNotLiteral,
	// unsupportedTagShape, or unreadableFile when a position did not check out.
	issue ManualReason
	// multiline is true when each element has its own lines, so elements can be
	// removed and added (BR4.4).
	multiline bool
	// extraFields is true when an element has more than Key and Value (BR4.5).
	extraFields bool
	// value is the range of the value of the place: in YAML from after "Tags:"
	// to the end of the last element, otherwise from the bracket to the bracket.
	value span
	line  int
	// others holds the lines of the place that belong to no element, comments
	// and blank lines; edits must not change them (security design §4 (d)).
	others string
}

// signature writes the tags of the place for comparing two parses.
func (s *tagSite) signature() string {
	if s == nil {
		return "-"
	}
	if s.issue != "" {
		return string(s.issue)
	}
	var b strings.Builder
	b.WriteString(strconv.Itoa(int(s.shape)))
	for i := range s.entries {
		e := &s.entries[i]
		b.WriteByte('\x00')
		b.WriteString(e.key)
		b.WriteByte('\x00')
		b.WriteString(e.value)
		if !e.literal {
			b.WriteString("\x00~")
		}
	}
	return b.String()
}

// markDuplicates makes every entry of a key written twice not literal: which of
// them counts is not for this tool to say.
func (s *tagSite) markDuplicates() {
	seen := make(map[string]int, len(s.entries))
	for i := range s.entries {
		seen[s.entries[i].key]++
	}
	for i := range s.entries {
		if e := &s.entries[i]; e.key != "" && seen[e.key] > 1 {
			e.literal = false
		}
	}
}

// resourceRef is one resource of the index of a file: its logical ID and Type,
// or its Terraform address.
type resourceRef struct {
	key, cloudFormationType string
	line                    int
}

// resourceTags is one resource with the signature of its tags.
type resourceTags struct {
	key, tags string
}

// parsedFile is a file after its parser.
type parsedFile interface {
	refs() []resourceRef
	// site returns the place of the tags of the resource; nil when it has none.
	site(key string) *tagSite
	// allTags returns every resource with its tags, in file order.
	allTags() []resourceTags
}

// parseFailure is what is kept of a parser's error (NFR8.27).
type parseFailure struct {
	line, column int
	kind         string
}

// parseContent runs the parser of the format.
func parseContent(ct *content) (parsedFile, *parseFailure) {
	switch ct.format {
	case formatJSON:
		return parseJSON(ct)
	case formatTerraform:
		return parseHCL(ct)
	}
	return parseYAML(ct)
}

// lineTable finds lines from byte positions of the raw file.
type lineTable []int

// newLineTable lists where the lines start. breaks returns the length of the
// line break at position i, or 0.
func newLineTable(raw []byte, base int, breaks func([]byte, int) int) lineTable {
	starts := lineTable{base}
	for i := base; i < len(raw); {
		if n := breaks(raw, i); n > 0 {
			i += n
			starts = append(starts, i)
			continue
		}
		i++
	}
	return starts
}

// newlineBreak is the line break of JSON and Terraform.
func newlineBreak(raw []byte, i int) int {
	if raw[i] == '\n' {
		return 1
	}
	return 0
}

// lineOf returns the line (from 1) of position pos.
func (t lineTable) lineOf(pos int) int {
	return sort.Search(len(t), func(i int) bool { return t[i] > pos })
}

// start returns where line (from 1) starts.
func (t lineTable) start(line int) int { return t[line-1] }

// count returns the number of lines; text after the last break makes a line.
func (t lineTable) count() int { return len(t) }

// contentEnd returns the end of line (from 1) without its line break.
func (t lineTable) contentEnd(raw []byte, line int) int {
	end := len(raw)
	if line < len(t) {
		end = t[line]
	}
	for end > t[line-1] && isBreakByte(raw[end-1]) {
		end--
	}
	return end
}

func isBreakByte(c byte) bool { return c == '\n' || c == '\r' }

// outsideLines writes the lines between from and to, both excluded, that no
// element touches (security design §4 (d)).
func (t lineTable) outsideLines(raw []byte, from, to int, entries []tagEntry) string {
	var b strings.Builder
	next := 0
	for line := from + 1; line < to && line <= t.count(); line++ {
		start := t.start(line)
		for next < len(entries) && t.lineOf(entries[next].elem.end) < line {
			next++
		}
		if next < len(entries) && t.lineOf(entries[next].elem.start) <= line {
			continue
		}
		b.Write(raw[start:t.contentEnd(raw, line)])
		b.WriteByte('\n')
	}
	return b.String()
}

// The memory budget of the parsers (security design §3): at most this many
// bytes of files are being parsed at once. A file takes its size while it is
// parsed and gives it back when its index is made.
var parseBudgetBytes int64 = 32 << 20

// budget hands out the bytes of parseBudgetBytes.
type budget struct {
	sem *semaphore.Weighted
}

func newBudget() *budget { return &budget{sem: semaphore.NewWeighted(parseBudgetBytes)} }

// acquire waits for size bytes and returns how to give them back.
func (b *budget) acquire(ctx context.Context, size int) (func(), error) {
	n := min(int64(size), parseBudgetBytes)
	if err := b.sem.Acquire(ctx, n); err != nil {
		return nil, err
	}
	return func() { b.sem.Release(n) }, nil
}

// place is the one place of a target, once it is found.
type place struct {
	file string
	line int
}

// fileRef is a resource of the index, with its file.
type fileRef struct {
	file string
	ref  resourceRef
}

// placeIndex holds the resources of every file, in file name order.
type placeIndex struct {
	byKey map[string][]fileRef
	// skippedCF and skippedTF name the candidates left out, for the detail of
	// a place not found (BR3.1).
	skippedCF, skippedTF []string
}

func newPlaceIndex() *placeIndex { return &placeIndex{byKey: map[string][]fileRef{}} }

// add puts a file's resources in the index. Files must come in name order.
func (x *placeIndex) add(tool Tool, file string, refs []resourceRef) {
	for _, r := range refs {
		k := indexKey(tool, r.key)
		x.byKey[k] = append(x.byKey[k], fileRef{file: file, ref: r})
	}
}

// indexKey keeps logical IDs and addresses apart.
func indexKey(tool Tool, key string) string {
	if tool == ToolTerraform {
		return "t\x00" + key
	}
	return "c\x00" + key
}

// listNames writes file names for a detail: the first five, and how many more.
func listNames(names []string) string {
	const shown = 5
	if len(names) <= shown {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:shown], ", ") + ", +" + strconv.Itoa(len(names)-shown)
}

// find looks for the one place of the target (BR3.2, BR3.3). When there is no
// such place it puts the changes on the list to fix by hand and returns false.
func (x *placeIndex) find(res *Result, t *target) (place, bool) {
	if t.tool == ToolTerraform {
		return x.findBlock(res, t)
	}
	return x.findTemplate(res, t)
}

func (x *placeIndex) findBlock(res *Result, t *target) (place, bool) {
	refs := x.byKey[indexKey(ToolTerraform, t.address)]
	switch len(refs) {
	case 0:
		kind, detail := DetailNone, ""
		if len(x.skippedTF) > 0 {
			kind, detail = DetailSkipped, listNames(x.skippedTF)
		}
		manualAll(res, t, ReasonBlockNotFound, kind, detail)
		return place{}, false
	case 1:
		return place{file: refs[0].file, line: refs[0].ref.line}, true
	}
	manualAll(res, t, ReasonAmbiguous, DetailCandidates, listNames(uniqueFiles(refs)))
	return place{}, false
}

func (x *placeIndex) findTemplate(res *Result, t *target) (place, bool) {
	var match, other []fileRef
	for _, r := range x.byKey[indexKey(ToolCloudFormation, t.logicalID)] {
		if rt, ok := restype.ResourceTypeOf(r.ref.cloudFormationType); ok && rt == t.resourceType {
			match = append(match, r)
		} else {
			other = append(other, r)
		}
	}
	switch len(match) {
	case 0:
		kind, detail := DetailNone, ""
		switch {
		case len(other) > 0:
			kind, detail = DetailOtherType, listNames(uniqueFiles(other))
		case len(x.skippedCF) > 0:
			kind, detail = DetailSkipped, listNames(x.skippedCF)
		}
		manualAll(res, t, ReasonTemplateNotFound, kind, detail)
		return place{}, false
	case 1:
		return place{file: match[0].file, line: match[0].ref.line}, true
	}
	var named []fileRef
	for _, r := range match {
		base := path.Base(r.file)
		if strings.TrimSuffix(base, path.Ext(base)) == t.stackName {
			named = append(named, r)
		}
	}
	if len(named) == 1 {
		return place{file: named[0].file, line: named[0].ref.line}, true
	}
	manualAll(res, t, ReasonAmbiguous, DetailCandidates, listNames(uniqueFiles(match)))
	return place{}, false
}

func uniqueFiles(refs []fileRef) []string {
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		names = append(names, r.file)
	}
	return slices.Compact(names)
}

// parseErrorLine takes the line from the text of a parser's error, the only
// thing of that text that is kept: the text can quote the source (NFR8.27).
var parseErrorLine = regexp.MustCompile(`line (\d+)`)

func lineFromError(text string) int {
	if m := parseErrorLine.FindStringSubmatch(text); m != nil {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			return n
		}
	}
	return 0
}
