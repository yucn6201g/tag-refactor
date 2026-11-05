package iac

import (
	"bytes"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// The reader of Terraform files (security design §3): the tree of hclsyntax,
// whose ranges are already in bytes. hclwrite is not used: only the ranges of
// the keys and values change, so everything else stays as it is.

// hclFile is a parsed .tf file.
type hclFile struct {
	ct     *content
	blocks []*hclsyntax.Block
	// byKey finds the first block of an address.
	byKey map[string]*hclsyntax.Block
	lines lineTable
}

// parseHCL parses a .tf file.
func parseHCL(ct *content) (parsedFile, *parseFailure) {
	file, diags := hclsyntax.ParseConfig(ct.body, "main.tf", hcl.InitialPos)
	if diags.HasErrors() {
		failure := &parseFailure{kind: "hcl"}
		for _, d := range diags {
			if d.Severity == hcl.DiagError && d.Subject != nil {
				failure.line, failure.column = d.Subject.Start.Line, d.Subject.Start.Column
				break
			}
		}
		return nil, failure
	}
	f := &hclFile{ct: ct, byKey: map[string]*hclsyntax.Block{}}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return f, nil
	}
	for _, b := range body.Blocks {
		if b.Type == "resource" && len(b.Labels) == 2 {
			f.blocks = append(f.blocks, b)
			if _, seen := f.byKey[blockKey(b)]; !seen {
				f.byKey[blockKey(b)] = b
			}
		}
	}
	return f, nil
}

func blockKey(b *hclsyntax.Block) string { return b.Labels[0] + "." + b.Labels[1] }

func (f *hclFile) refs() []resourceRef {
	refs := make([]resourceRef, 0, len(f.blocks))
	for _, b := range f.blocks {
		refs = append(refs, resourceRef{key: blockKey(b), line: b.TypeRange.Start.Line})
	}
	return refs
}

func (f *hclFile) allTags() []resourceTags {
	out := make([]resourceTags, 0, len(f.blocks))
	for _, b := range f.blocks {
		out = append(out, resourceTags{key: blockKey(b), tags: f.blockSite(b).signature()})
	}
	return out
}

func (f *hclFile) site(key string) *tagSite {
	if b, ok := f.byKey[key]; ok {
		return f.blockSite(b)
	}
	return nil
}

func (f *hclFile) table() lineTable {
	if f.lines == nil {
		f.lines = newLineTable(f.ct.raw, f.ct.base, newlineBreak)
	}
	return f.lines
}

// blockSite finds the place of the tags of a block (BR4.1).
func (f *hclFile) blockSite(b *hclsyntax.Block) *tagSite {
	for _, inner := range b.Body.Blocks {
		if inner.Type == "tag" || inner.Type == "tags" || inner.Type == "dynamic" && len(inner.Labels) == 1 && inner.Labels[0] == "tag" {
			return &tagSite{issue: ReasonUnsupportedTagShape}
		}
	}
	attr, ok := b.Body.Attributes["tags"]
	if !ok {
		return nil
	}
	object, ok := attr.Expr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return &tagSite{issue: ReasonTagsNotLiteral}
	}
	return f.readObject(object, attr.NameRange.Start.Line)
}

// readObject reads the items of tags = { ... }.
func (f *hclFile) readObject(object *hclsyntax.ObjectConsExpr, line int) *tagSite {
	base := f.ct.base
	s := &tagSite{shape: shapeMap, line: line, value: span{object.SrcRange.Start.Byte + base, object.SrcRange.End.Byte + base}}
	for _, item := range object.Items {
		e := tagEntry{line: item.KeyExpr.Range().Start.Line, keyQuote: quotePlain, valueQuote: quoteDouble}
		keyOK := f.readKey(&e, item.KeyExpr)
		valueOK := f.readValue(&e, item.ValueExpr)
		e.literal = keyOK && valueOK
		e.elem = span{item.KeyExpr.Range().Start.Byte + base, item.ValueExpr.Range().End.Byte + base}
		s.entries = append(s.entries, e)
	}
	s.markDuplicates()
	f.finish(s)
	return s
}

// readKey reads a key written as a name or as a string (BR4.2).
func (f *hclFile) readKey(e *tagEntry, expr hclsyntax.Expression) bool {
	key, ok := expr.(*hclsyntax.ObjectConsKeyExpr)
	if !ok || key.ForceNonLiteral {
		return false
	}
	r := key.Wrapped.Range()
	s := span{r.Start.Byte + f.ct.base, r.End.Byte + f.ct.base}
	if name := hcl.ExprAsKeyword(key.Wrapped); name != "" {
		e.key, e.keySpan, e.keyQuote = name, s, quotePlain
		return true
	}
	if text, ok := f.stringOf(key.Wrapped); ok {
		e.key, e.keySpan, e.keyQuote = text, s, quoteDouble
		return true
	}
	return false
}

// readValue reads a value written as one string on one line, without
// interpolation (BR4.2).
func (f *hclFile) readValue(e *tagEntry, expr hclsyntax.Expression) bool {
	text, ok := f.stringOf(expr)
	if !ok {
		return false
	}
	r := expr.Range()
	e.value, e.valueSpan = text, span{r.Start.Byte + f.ct.base, r.End.Byte + f.ct.base}
	return true
}

// stringOf returns the text of a quoted string of literal parts on one line. A
// heredoc is several lines; ${ and %{ make other kinds of part.
func (f *hclFile) stringOf(expr hclsyntax.Expression) (string, bool) {
	tmpl, ok := expr.(*hclsyntax.TemplateExpr)
	r := expr.Range()
	if !ok || r.Start.Line != r.End.Line {
		return "", false
	}
	raw := f.ct.body
	if r.End.Byte-r.Start.Byte < 2 || raw[r.Start.Byte] != '"' || raw[r.End.Byte-1] != '"' {
		return "", false
	}
	var b strings.Builder
	for _, part := range tmpl.Parts {
		lit, ok := part.(*hclsyntax.LiteralValueExpr)
		if !ok || lit.Val.Type().FriendlyName() != "string" || lit.Val.IsNull() || !lit.Val.IsKnown() {
			return "", false
		}
		b.WriteString(lit.Val.AsString())
	}
	return b.String(), true
}

// finish says whether each item has lines of its own (BR4.4) and works out the
// ranges of the elements: to the end of the line, with a comma and a comment.
func (f *hclFile) finish(s *tagSite) {
	t := f.table()
	raw := f.ct.raw
	openLine, closeLine := t.lineOf(s.value.start), t.lineOf(s.value.end-1)
	s.multiline = openLine != closeLine
	lastLine := openLine
	for i := range s.entries {
		e := &s.entries[i]
		first := t.lineOf(e.elem.start)
		last := t.lineOf(e.elem.end - 1)
		tail := raw[e.elem.end:t.contentEnd(raw, last)]
		if first <= lastLine || last >= closeLine || !blankBefore(raw, t.start(first), e.elem.start) || !plainTail(tail) {
			s.multiline = false
			break
		}
		lastLine = last
		if e.literal {
			between := raw[min(e.keySpan.end, e.valueSpan.start):e.valueSpan.start]
			e.hasComment = hasHCLComment(tail) || hasHCLComment(between)
		}
		e.elem.end = t.contentEnd(raw, last)
	}
	if s.multiline {
		s.others = t.outsideLines(raw, openLine, closeLine, s.entries)
	}
}

// plainTail reports whether the text after a value holds at most a comma and a
// comment.
func plainTail(tail []byte) bool {
	rest := bytes.TrimLeft(tail, " \t\r")
	rest = bytes.TrimPrefix(rest, []byte(","))
	rest = bytes.TrimLeft(rest, " \t\r")
	return len(rest) == 0 || hasHCLComment(rest[:min(2, len(rest))])
}

func hasHCLComment(text []byte) bool {
	return bytes.IndexByte(text, '#') >= 0 || bytes.Contains(text, []byte("//")) || bytes.Contains(text, []byte("/*"))
}
