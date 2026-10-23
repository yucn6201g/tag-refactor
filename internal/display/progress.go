package display

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Progress of the collection on standard error (BR6.1, NFR11.5). In append
// mode every change of state adds one line. In redraw mode, used only when
// standard error is a terminal that accepts cursor sequences, a table with
// one line per region is rewritten in place, at most every 0.2 seconds.
// The cursor sequences are constants of the tool; every string read from
// AWS is sanitized before it is put in a line.

// redrawInterval is the shortest time between two redraws.
const redrawInterval = 200 * time.Millisecond

// Region states of the progress table.
const (
	stateWaiting = "waiting"
	stateRunning = "running"
	stateDone    = "done"
	stateFailed  = "failed"
)

type regionProgress struct {
	state  string
	count  int
	reason string
}

// progress is the state of the progress display. It is only used with the
// printer's lock held.
type progress struct {
	start   time.Time
	regions []string
	byName  map[string]*regionProgress
	// discovery is true during the search for never-tagged resources.
	discovery     bool
	discoveryDone int
	// drawn is the number of lines of the table on the screen.
	drawn    int
	lastDraw time.Time
}

func (p *Printer) elapsed() string {
	if p.progress == nil {
		return FormatElapsed(0)
	}
	return FormatElapsed(p.now().Sub(p.progress.start))
}

// RegionsWaiting implements inventory.ProgressSink: it starts the progress
// display with every region waiting.
func (p *Printer) RegionsWaiting(regions []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endProgress()
	pr := &progress{start: p.now(), regions: append([]string(nil), regions...), byName: map[string]*regionProgress{}}
	for _, r := range regions {
		pr.byName[r] = &regionProgress{state: stateWaiting}
	}
	p.progress = pr
	if p.redraw {
		p.drawTable(true)
		return
	}
	p.progressLine("scan: waiting: %s", strings.Join(regions, ", "))
}

// RegionStarted implements inventory.ProgressSink.
func (p *Printer) RegionStarted(region string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.setRegion(region, regionProgress{state: stateRunning}) {
		p.progressLine("scan: %s started", region)
	}
}

// RegionDone implements inventory.ProgressSink.
func (p *Printer) RegionDone(region string, count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.setRegion(region, regionProgress{state: stateDone, count: count}) {
		p.progressLine("scan: %s done (%s, elapsed %s)", region, plural(count, "resource"), p.elapsed())
	}
}

// RegionFailed implements inventory.ProgressSink.
func (p *Printer) RegionFailed(region, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.setRegion(region, regionProgress{state: stateFailed, reason: reason}) {
		p.progressLine("scan: %s failed (%s, elapsed %s)", region, reason, p.elapsed())
	}
}

// DiscoveryStarted implements inventory.ProgressSink. The table of the
// regions stays on the screen in its final state.
func (p *Printer) DiscoveryStarted(regions []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.redraw && p.progress != nil {
		p.drawTable(true)
		p.progress.drawn = 0
		p.progress.discovery = true
		p.progress.regions = append([]string(nil), regions...)
		p.drawTable(true)
		return
	}
	p.progressLine("scan: discovering never-tagged resources")
}

// DiscoveryRegionDone implements inventory.ProgressSink.
func (p *Printer) DiscoveryRegionDone(region, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.redraw && p.progress != nil {
		p.progress.discoveryDone++
		p.drawTable(false)
		return
	}
	p.progressLine("scan: %s never-tagged resources: %s", region, status)
}

// setRegion changes the state of a region. In redraw mode it redraws the
// table and returns true; in append mode it returns false and the caller
// appends a line.
func (p *Printer) setRegion(region string, st regionProgress) bool {
	if !p.redraw || p.progress == nil {
		return false
	}
	if rp, ok := p.progress.byName[region]; ok {
		*rp = st
	}
	p.drawTable(false)
	return true
}

// progressLine appends one progress line.
func (p *Printer) progressLine(format string, args ...any) {
	p.line(p.errOut, format, args...)
}

// endProgress leaves the table on the screen in its final state, so the
// lines that follow start below it. It is called before results, errors and
// the final lines.
func (p *Printer) endProgress() {
	if p.progress == nil {
		return
	}
	if p.redraw && p.progress.drawn > 0 {
		p.drawTable(true)
	}
	p.progress = nil
}

// tableLines renders the progress table.
func (p *Printer) tableLines() []string {
	pr := p.progress
	if pr.discovery {
		return []string{fmt.Sprintf("  discovering never-tagged resources: %d/%d regions · elapsed %s",
			pr.discoveryDone, len(pr.regions), p.elapsed())}
	}
	lines := []string{Sanitize(fmt.Sprintf("Account %s · %s", p.account, plural(len(pr.regions), "region")))}
	finished := 0
	for _, name := range pr.regions {
		rp := pr.byName[name]
		row := fmt.Sprintf("  %-10s %-16s", "["+rp.state+"]", Sanitize(name))
		switch rp.state {
		case stateDone:
			finished++
			row += fmt.Sprintf(" %6d %s", rp.count, pluralWord(rp.count, "resource"))
		case stateFailed:
			finished++
			row += " " + Sanitize(rp.reason)
		}
		lines = append(lines, strings.TrimRight(row, " "))
	}
	lines = append(lines, fmt.Sprintf("  %d/%d regions · elapsed %s", finished, len(pr.regions), p.elapsed()))
	return lines
}

// drawTable redraws the table in place: it moves the cursor up over the lines
// drawn before, clears them and writes the new ones. Unless force is set, it
// skips the redraw when the last one is less than redrawInterval ago.
func (p *Printer) drawTable(force bool) {
	pr := p.progress
	now := p.now()
	if !force && pr.drawn > 0 && now.Sub(pr.lastDraw) < redrawInterval {
		return
	}
	var b strings.Builder
	p.eraseTable(&b)
	lines := p.tableLines()
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	p.writeRaw(b.String())
	pr.drawn = len(lines)
	pr.lastDraw = now
}

// eraseTable writes the sequences that move the cursor to the first line of
// the table and clear the screen below it.
func (p *Printer) eraseTable(b *strings.Builder) {
	if p.progress == nil || p.progress.drawn == 0 {
		return
	}
	fmt.Fprintf(b, "\x1b[%dA\r\x1b[J", p.progress.drawn)
	p.progress.drawn = 0
}

// writeRaw writes text that the tool built from sanitized parts.
func (p *Printer) writeRaw(s string) {
	if _, err := io.WriteString(p.errOut, s); err != nil && p.werr == nil {
		p.werr = err
	}
}
