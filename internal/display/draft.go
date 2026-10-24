package display

import "github.com/yucn6201g/tag-refactor/internal/inventory"

// Output of rules suggest (U3 workflow W2).

// WriteDraft writes the draft rule file to standard output as it is. It is
// the one result that is not sanitized here: the draft must stay valid YAML,
// and rules.Suggest has already written every key and value as an escaped
// YAML string, so nothing in it can rewrite the terminal (security design
// §3). Only text built by rules.Suggest may be passed.
func (p *Printer) WriteDraft(draft []byte) {
	p.locked(func() {
		p.endProgress()
		if _, err := p.out.Write(draft); err != nil && p.werr == nil {
			p.werr = err
		}
	})
}

// DraftIncomplete prints to standard error the regions the tagging API could
// not read (BR6.6). Standard output holds the draft only, so the marker goes
// to standard error as well as into the draft's first comments.
func (p *Printer) DraftIncomplete(failed []inventory.FailedRegion) {
	p.locked(func() {
		p.endProgress()
		if len(failed) == 0 {
			return
		}
		p.line(p.errOut, "! Results are incomplete")
		for _, f := range failed {
			p.line(p.errOut, "    %s: %s (%s)", f.Region, f.Kind, f.Operation)
		}
	})
}

// DraftWritten says on standard error where --output wrote the draft.
func (p *Printer) DraftWritten(path string, keys int) {
	p.locked(func() {
		p.line(p.errOut, "Draft written to %s (%s)", path, rawString(plural(keys, "key")))
	})
}

// SuggestScope prints to standard error what rules suggest read: the tagged
// resources only (never-tagged resources are not looked for).
func (p *Printer) SuggestScope(snap *inventory.Snapshot) {
	p.locked(func() {
		p.line(p.errOut, "Checked: %s", rawString(taggedScope(snap)))
	})
}
