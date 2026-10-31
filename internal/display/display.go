// Package display writes everything tag-refactor prints. Results go to
// standard output; progress, warnings, errors and debug lines go to standard
// error (BR3.10). Results come in four formats (table, diff, JSON and
// Markdown, U4 BR3.1 to BR3.5), in English or Japanese (BR3.8, BR3.9), with
// colors on a terminal that accepts them (BR3.7, NFR11.7); the meaning is
// always written with symbols and words too. Every string built at run time
// is sanitized before it reaches the terminal (NFR8.6); JSON goes through
// planner.SafeJSONWriter and Markdown through mdCode (NFR8.15, NFR8.16).
//
// Together with cmd/tag-refactor, this is the only package allowed to write
// to the process output; a test checks the sources for that.
package display

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Printer writes results to out and everything else to errOut. It implements
// inventory.ProgressSink and awsaccess.DebugSink. It is the only writer of
// both streams; one lock keeps the lines of the goroutines that read regions
// from mixing (observability design §1).
type Printer struct {
	mu     sync.Mutex
	out    io.Writer
	errOut io.Writer
	debug  bool
	werr   error
	now    func() time.Time
	lang   Lang
	// colorOut and colorErr say whether each stream gets colors (NFR11.7).
	colorOut, colorErr bool
	// redraw is true when progress is redrawn in place on errOut.
	redraw   bool
	progress *progress
	// restores put the terminal settings New changed back, in reverse order.
	restores []func()
	// account is the account ID shown above the progress table.
	account string
}

var (
	_ inventory.ProgressSink = (*Printer)(nil)
	_ awsaccess.DebugSink    = (*Printer)(nil)
)

// New returns a printer for the given standard output and standard error, in
// English until SetLang. Progress is redrawn in place only when standard
// error is a terminal that accepts it; standard output plays no part in that
// choice. Colors are decided for each stream on its own (NFR11.7). Close puts
// back what New changed in the terminal.
func New(stdout, stderr io.Writer) *Printer {
	p := &Printer{out: stdout, errOut: stderr, now: time.Now, lang: English}
	// Standard error is asked once: the same console setting serves the
	// redrawn progress and the colors.
	var restore func()
	p.redraw, restore = canRedraw(stderr)
	p.keep(restore)
	p.colorErr = p.redraw && getenv("NO_COLOR") == ""
	p.colorOut, restore = canColor(stdout)
	p.keep(restore)
	return p
}

func (p *Printer) keep(restore func()) {
	if restore != nil {
		p.restores = append(p.restores, restore)
	}
}

// Close ends the progress and puts the terminal settings back as they were
// before New: on Windows, the virtual terminal processing New turned on for
// standard error and standard output (NFR9.3, NFR11.7). Progress and colors
// stop after it. Calling it again does nothing.
func (p *Printer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endProgress()
	p.redraw = false
	p.colorOut, p.colorErr = false, false
	for i := len(p.restores) - 1; i >= 0; i-- {
		p.restores[i]()
	}
	p.restores = nil
}

// SetLang sets the language of everything printed from now on (BR3.8).
func (p *Printer) SetLang(l Lang) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lang = l
}

// Lang returns the language of the printer.
func (p *Printer) Lang() Lang {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lang
}

// SetDebug turns the --debug lines on or off.
func (p *Printer) SetDebug(on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.debug = on
}

// SetClock replaces the clock of the elapsed times (tests).
func (p *Printer) SetClock(now func() time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.now = now
}

// Stdout returns the result writer, for the help text of the command parser.
// Write errors through it are kept like those of the printer itself.
func (p *Printer) Stdout() io.Writer { return recordingWriter{p: p, w: p.out} }

// Stderr returns the message writer, for the command parser.
func (p *Printer) Stderr() io.Writer { return recordingWriter{p: p, w: p.errOut} }

// recordingWriter passes writes through and keeps the first write error.
type recordingWriter struct {
	p *Printer
	w io.Writer
}

func (r recordingWriter) Write(b []byte) (int, error) {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	n, err := r.w.Write(b)
	if err != nil && r.p.werr == nil {
		r.p.werr = err
	}
	return n, err
}

// WriteErr returns the first error met while writing, if any.
func (p *Printer) WriteErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.werr
}

// tr returns the message id in the printer's language.
func (p *Printer) tr(id string, args ...any) string { return T(p.lang, id, args...) }

// count returns n with its unit in the printer's language.
func (p *Printer) count(unit string, n int) string { return Count(p.lang, unit, n) }

// sanitizeArgs sanitizes the string arguments of a line; a rawString was
// sanitized piece by piece already.
func sanitizeArgs(args []any) []any {
	for i, a := range args {
		if s, ok := a.(string); ok {
			args[i] = Sanitize(s)
		}
	}
	return args
}

// say writes one line of standard error with the message id. String
// arguments are sanitized; the message itself is a constant of the tool. A
// line written while the progress table is on the screen goes in its place,
// and the table is drawn again below it. The caller holds the lock.
func (p *Printer) say(st style, id string, args ...any) {
	p.sayText(st, fmt.Sprintf(p.tr(id), sanitizeArgs(args)...))
}

// sayText writes a line of standard error built from sanitized parts.
func (p *Printer) sayText(st style, text string) {
	text = paint(p.colorErr, st, text) + "\n"
	if p.redraw && p.progress != nil && p.progress.drawn > 0 {
		var b strings.Builder
		p.eraseTable(&b)
		b.WriteString(text)
		p.writeRaw(b.String())
		p.drawTable(true)
		return
	}
	p.writeRaw(text)
}

// locked runs f with the lock held.
func (p *Printer) locked(f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f()
}

// Sanitize makes a string safe to print on a terminal (NFR8.6): control
// characters (Cc), format characters such as bidirectional overrides (Cf),
// line and paragraph separators (Zl, Zp) become \u{XXXX}, invalid UTF-8 bytes
// become \x{XX}, and a backslash becomes \\ so the result stays unambiguous.
// Printable characters, including non-Latin scripts, are kept.
func Sanitize(s string) string {
	clean := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c == 0x7f || c == '\\' || c >= utf8.RuneSelf {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x{%02X}`, s[i])
		case r == '\\':
			b.WriteString(`\\`)
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp):
			fmt.Fprintf(&b, `\u{%X}`, r)
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// Warn implements inventory.ProgressSink.
func (p *Printer) Warn(message string) {
	p.locked(func() { p.say(styleChange, "stderr.warning", message) })
}

// WarnText prints a warning of the tool from the catalog.
func (p *Printer) WarnText(id string) {
	p.locked(func() { p.say(styleChange, "stderr.warning", rawString(p.tr(id))) })
}

// FileWarning prints a warning about a line of a file.
func (p *Printer) FileWarning(path string, line int, message string) {
	p.locked(func() { p.say(styleChange, "stderr.fileWarning", path, line, message) })
}

// APICall implements awsaccess.DebugSink. It prints one line per call when
// --debug is on, with the operation (the API name), region, duration and
// retries only (NFR8.2, NFR11.3, NFR11.6). Calls of several regions arrive at
// the same time; the lock keeps their lines whole. The line is for
// developers and has one form in every language.
func (p *Printer) APICall(rec awsaccess.CallRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.debug {
		return
	}
	retries := "n/a"
	if rec.Retries >= 0 {
		retries = fmt.Sprint(rec.Retries)
	}
	suffix := ""
	if rec.ErrorKind != "" {
		suffix = " error=" + rec.ErrorKind
		if rec.CauseType != "" {
			suffix += " cause=" + rec.CauseType
		}
	}
	p.sayText(styleNone, fmt.Sprintf("debug: %s %s %dms retries=%s%s", Sanitize(rec.Op), Sanitize(rec.Region),
		rec.Duration.Milliseconds(), retries, Sanitize(suffix)))
}

// Account prints the account and the region of the session.
func (p *Printer) Account(acc awsaccess.AccountContext) {
	p.locked(func() {
		p.account = acc.AccountID
		p.say(styleNone, "stderr.account", acc.AccountID, acc.Region)
	})
}

// Error prints the two-line error message (NFR11.4).
func (p *Printer) Error(what, fix string) {
	p.locked(func() {
		p.endProgress()
		p.say(styleImpact, "stderr.error", what)
		p.say(styleNone, "stderr.fix", fix)
	})
}

// Fatal prints a one-line message for failures that have no fix to suggest.
func (p *Printer) Fatal(message string) {
	p.locked(func() {
		p.endProgress()
		p.say(styleImpact, "stderr.error", message)
	})
}

// Interrupted says that Ctrl+C stopped the command and nothing was saved
// (U4 NFR design §3).
func (p *Printer) Interrupted() {
	p.locked(func() {
		p.endProgress()
		p.say(styleImpact, "stderr.interrupted")
	})
}

// Done prints the elapsed time and the number of scanned resources (NFR1.1).
func (p *Printer) Done(elapsed time.Duration, resources int) {
	p.locked(func() {
		p.endProgress()
		p.say(styleNone, "stderr.done", FormatElapsed(elapsed), rawString(p.count("unit.resource", resources)))
	})
}

// The meanings of the exit code line; cli chooses one of them.
const (
	ExitMeaningError             = "exit.error"
	ExitMeaningInterrupted       = "exit.interrupted"
	ExitMeaningResultsIncomplete = "exit.resultsIncomplete"
	ExitMeaningImpactIncomplete  = "exit.impactIncomplete"
	ExitMeaningDraftTooLarge     = "exit.draftTooLarge"
	ExitMeaningDraftReady        = "exit.draftReady"
	ExitMeaningIssuesFound       = "exit.issuesFound"
	ExitMeaningReviewNeeded      = "exit.reviewNeeded"
	ExitMeaningNoIssues          = "exit.noIssues"
	ExitMeaningNoChangesNeeded   = "exit.noChangesNeeded"
	// The meanings of apply (U6 BR6.2).
	ExitMeaningApplied      = "exit.applied"
	ExitMeaningCancelled    = "exit.cancelled"
	ExitMeaningApplySkipped = "exit.applySkipped"
	ExitMeaningApplyFailed  = "exit.applyFailed"
	ExitMeaningImpactStop   = "exit.impactStop"
	// The meanings of rollback (U7 BR7.2).
	ExitMeaningRestored        = "exit.restored"
	ExitMeaningNotUndone       = "exit.notUndone"
	ExitMeaningNothingToUndo   = "exit.nothingToUndo"
	ExitMeaningRollbackSkipped = "exit.rollbackSkipped"
	ExitMeaningRollbackFailed  = "exit.rollbackFailed"
)

// ExitCode prints the exit code line with its meaning, one of the
// ExitMeaning identifiers.
func (p *Printer) ExitCode(code int, meaning string) {
	p.locked(func() { p.say(styleNone, "stderr.exitCode", code, rawString(p.tr(meaning))) })
}

// FormatElapsed renders a duration as mm:ss, or h:mm:ss from one hour.
func FormatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second) / time.Second)
	h, m, s := total/3600, (total/60)%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// rawString marks a string that has already been sanitized piece by piece,
// so it is not sanitized again (which would double the backslashes).
type rawString string

func (r rawString) String() string { return string(r) }

// results is the writer of one result on standard output: it collects the
// lines in a buffer and writes them at once (U4 NFR design, performance).
type results struct {
	w     *bufio.Writer
	lang  Lang
	color bool
}

// newResults starts a result on standard output. The caller holds the lock
// and ends it with flush.
func (p *Printer) newResults() *results {
	return &results{w: bufio.NewWriterSize(p.out, 64<<10), lang: p.lang, color: p.colorOut}
}

// flush writes the result and keeps the first write error.
func (p *Printer) flush(r *results) {
	if err := r.w.Flush(); err != nil && p.werr == nil {
		p.werr = err
	}
}

func (r *results) tr(id string, args ...any) string { return T(r.lang, id, args...) }
func (r *results) count(unit string, n int) string  { return Count(r.lang, unit, n) }

// line writes one line from a message of the catalog, with its string
// arguments sanitized.
func (r *results) line(st style, id string, args ...any) {
	r.text(st, fmt.Sprintf(r.tr(id), sanitizeArgs(args)...))
}

// text writes a line built from sanitized parts.
func (r *results) text(st style, s string) {
	_, _ = r.w.WriteString(paint(r.color, st, s))
	_ = r.w.WriteByte('\n')
}

func (r *results) blank() { _ = r.w.WriteByte('\n') }
