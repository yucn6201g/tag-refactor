package display

import (
	"fmt"
	"strings"
)

// Markdown output (BR3.5, NFR8.16). Every string that comes from data is
// written as a code span through mdSpan or mdCell, where Markdown and HTML
// mean nothing: a tag value cannot become a heading, a link, raw HTML or a
// table column. Inside a code span <, > and & are kept as they are (an
// entity would change the value people copy). The functions that put text
// into a line accept mdArg only, which a plain string is not, so data
// written outside a code span does not compile.

// mdArg is a piece of a Markdown line: data in a code span (mdCode) or text
// of the tool (mdTool).
type mdArg interface{ mdText() string }

// mdCode is data written as a code span. Only mdSpan and mdCell make one.
type mdCode string

func (c mdCode) mdText() string { return string(c) }

// mdTool is text of the tool: words of the catalog and numbers.
type mdTool string

func (t mdTool) mdText() string { return string(t) }

// mdSpan writes data as a code span: sanitized like every line of the table
// (newlines and control characters become their visible form), between
// backtick fences one longer than the longest run of backticks in it, with
// a space inside each fence.
func mdSpan(s string) mdCode {
	v := Sanitize(s)
	longest, run := 0, 0
	for i := 0; i < len(v); i++ {
		if v[i] == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	return mdCode(fence + " " + v + " " + fence)
}

// mdCell is mdSpan for a table cell: | is escaped as \|, which a table keeps
// inside the code span.
func mdCell(s string) mdCode {
	return mdCode(strings.ReplaceAll(string(mdSpan(s)), "|", `\|`))
}

// mdCount is a number of the tool.
func mdCount(n int) mdTool { return mdTool(FormatCount(n)) }

// mdJoin joins pieces with a separator of the tool.
func mdJoin(sep string, parts []mdArg) mdTool {
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		texts = append(texts, p.mdText())
	}
	return mdTool(strings.Join(texts, sep))
}

// mdLine writes a line from a message of the catalog with Markdown pieces.
func (r *results) mdLine(id string, args ...mdArg) {
	values := make([]any, 0, len(args))
	for _, a := range args {
		values = append(values, a.mdText())
	}
	_, _ = fmt.Fprintf(r.w, r.tr(id), values...)
	_ = r.w.WriteByte('\n')
}

// mdHeading writes a heading with a count: "## Changes (4 resources)". The
// heading holds words and numbers of the tool only.
func (r *results) mdHeading(id, unit string, n int) {
	r.mdLine(id, mdTool(r.count(unit, n)))
}

// mdTable writes the header line of a table from the catalog and the line
// under it.
func (r *results) mdTable(id string) {
	head := r.tr(id)
	_, _ = r.w.WriteString(head + "\n")
	cols := strings.Count(head, "|") - 1
	_, _ = r.w.WriteString("|" + strings.Repeat("---|", cols) + "\n")
}

// mdRow writes one table row.
func (r *results) mdRow(cells ...mdArg) {
	_, _ = r.w.WriteString("|")
	for _, c := range cells {
		_, _ = r.w.WriteString(" " + c.mdText() + " |")
	}
	_ = r.w.WriteByte('\n')
}
