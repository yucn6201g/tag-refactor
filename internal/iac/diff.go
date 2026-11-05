package iac

import (
	"bytes"
	"strconv"
	"strings"
)

// The difference is worked out from the edits, not by comparing files
// (logical-components.md): the edits say which lines change.

// diffContext is the number of lines of context on each side of a change.
const diffContext = 3

// diffChange is a run of changed lines: old lines a to b and new lines c to d,
// from 1 and both included; b < a or d < c when that side has no line.
type diffChange struct {
	a, b, c, d int
}

// textLines splits a file into lines, each with its line break.
type textLines struct {
	raw   []byte
	table lineTable
}

func newTextLines(raw []byte) *textLines {
	return &textLines{raw: raw, table: newLineTable(raw, 0, newlineBreak)}
}

// count returns the number of lines: the empty text after a last line break is
// not one.
func (l *textLines) count() int {
	n := l.table.count()
	if l.table.start(n) == len(l.raw) {
		n--
	}
	return n
}

// line returns line n (from 1) with its line break.
func (l *textLines) line(n int) []byte {
	end := len(l.raw)
	if n < l.table.count() {
		end = l.table.start(n + 1)
	}
	return l.raw[l.table.start(n):end]
}

func (l *textLines) atLineStart(pos int) bool {
	return pos == len(l.raw) && len(l.raw) > 0 && l.raw[pos-1] == '\n' || pos < len(l.raw) && l.table.start(l.table.lineOf(pos)) == pos
}

// unifiedDiff writes the difference of a file from its normalized edits.
func unifiedDiff(rel string, before, after []byte, edits []edit) string {
	oldLines, newLines := newTextLines(before), newTextLines(after)
	changes := changedLines(oldLines, newLines, edits)
	var b strings.Builder
	b.WriteString("--- a/" + rel + "\n+++ b/" + rel + "\n")
	for start := 0; start < len(changes); {
		end := start + 1
		for end < len(changes) && changes[end].a-changes[end-1].b-1 <= 2*diffContext {
			end++
		}
		writeHunk(&b, oldLines, newLines, changes[start:end])
		start = end
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// changedLines turns the edits into runs of lines. Edits that touch the same
// line make one run; the lines at the start and the end of a run that are the
// same on both sides are context, so a whole place replaced shows only what
// changes.
func changedLines(oldLines, newLines *textLines, edits []edit) []diffChange {
	var runs []diffChange
	shift := 0
	for _, e := range edits {
		start := e.start + shift
		end := start + len(e.text)
		shift += len(e.text) - (e.end - e.start)
		run := diffChange{a: oldLines.table.lineOf(e.start), c: newLines.table.lineOf(start)}
		run.b, run.d = run.a, run.c
		if e.end > e.start {
			run.b = oldLines.table.lineOf(e.end - 1)
		}
		if end > start {
			run.d = newLines.table.lineOf(end - 1)
		}
		if e.text == "" && oldLines.atLineStart(e.start) && oldLines.atLineStart(e.end) {
			run.d = run.c - 1
		}
		if n := len(runs); n > 0 && run.a <= runs[n-1].b {
			runs[n-1].b, runs[n-1].d = max(runs[n-1].b, run.b), max(runs[n-1].d, run.d)
			continue
		}
		runs = append(runs, run)
	}
	for i := range runs {
		r := &runs[i]
		for r.a <= r.b && r.c <= r.d && bytes.Equal(oldLines.line(r.a), newLines.line(r.c)) {
			r.a++
			r.c++
		}
		for r.a <= r.b && r.c <= r.d && bytes.Equal(oldLines.line(r.b), newLines.line(r.d)) {
			r.b--
			r.d--
		}
	}
	return joinTouching(runs)
}

// joinTouching makes one run of runs with no line between them, so a change
// shows its old lines and then its new lines, as differences usually do.
func joinTouching(runs []diffChange) []diffChange {
	joined := runs[:0]
	for _, r := range runs {
		if n := len(joined); n > 0 && r.a == joined[n-1].b+1 && r.c == joined[n-1].d+1 {
			joined[n-1].b, joined[n-1].d = r.b, r.d
			continue
		}
		joined = append(joined, r)
	}
	return joined
}

// writeHunk writes the runs of one hunk with their context.
func writeHunk(b *strings.Builder, oldLines, newLines *textLines, runs []diffChange) {
	first, last := runs[0], runs[len(runs)-1]
	oldStart := max(first.a-diffContext, 1)
	oldEnd := min(last.b+diffContext, oldLines.count())
	newStart := first.c - (first.a - oldStart)
	newEnd := last.d + (oldEnd - last.b)
	writeHunkHeader(b, oldStart, oldEnd-oldStart+1, newStart, newEnd-newStart+1)
	at := oldStart
	for _, r := range runs {
		writeLines(b, ' ', oldLines, at, r.a-1)
		writeLines(b, '-', oldLines, r.a, r.b)
		writeLines(b, '+', newLines, r.c, r.d)
		at = r.b + 1
	}
	writeLines(b, ' ', oldLines, at, oldEnd)
}

// writeHunkHeader writes the @@ line; a side without lines names the line before
// it, as unified differences do.
func writeHunkHeader(b *strings.Builder, oldStart, oldCount, newStart, newCount int) {
	if oldCount == 0 {
		oldStart--
	}
	if newCount == 0 {
		newStart--
	}
	b.WriteString("@@ -" + strconv.Itoa(oldStart) + "," + strconv.Itoa(oldCount) + " +" + strconv.Itoa(newStart) + "," + strconv.Itoa(newCount) + " @@\n")
}

// writeLines writes lines from to to with a mark. The line break is left out, so
// a CRLF file shows as any other, and a last line without one says so.
func writeLines(b *strings.Builder, mark byte, lines *textLines, from, to int) {
	for n := from; n <= to; n++ {
		text := lines.line(n)
		b.WriteByte(mark)
		b.Write(bytes.TrimSuffix(bytes.TrimSuffix(text, []byte("\n")), []byte("\r")))
		b.WriteByte('\n')
		if !bytes.HasSuffix(text, []byte("\n")) {
			b.WriteString("\\ No newline at end of file\n")
		}
	}
}
