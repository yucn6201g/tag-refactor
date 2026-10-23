// Package display writes everything tag-refactor prints. Results go to
// standard output; progress, warnings, errors and debug lines go to standard
// error (BR6.1). Output is a plain English list with symbols and no colors
// (BR6.2). Every string built at run time is sanitized before it reaches the
// terminal (NFR8.6).
//
// Together with cmd/tag-refactor, this is the only package allowed to write
// to the process output; a test checks the sources for that.
package display

import (
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
	// redraw is true when progress is redrawn in place on errOut.
	redraw   bool
	progress *progress
	// restore puts the terminal setting of errOut back; nil when New
	// changed nothing.
	restore func()
	// account is the account ID shown above the progress table.
	account string
}

var (
	_ inventory.ProgressSink = (*Printer)(nil)
	_ awsaccess.DebugSink    = (*Printer)(nil)
)

// New returns a printer for the given standard output and standard error.
// Progress is redrawn in place only when standard error is a terminal that
// accepts it; standard output plays no part in that choice. Close puts back
// what New changed in the terminal.
func New(stdout, stderr io.Writer) *Printer {
	redraw, restore := canRedraw(stderr)
	return &Printer{out: stdout, errOut: stderr, now: time.Now, redraw: redraw, restore: restore}
}

// Close ends the progress and puts the terminal setting of standard error
// back as it was before New: on Windows, the virtual terminal processing New
// turned on (NFR9.3). Progress is no longer redrawn after it. Calling it again
// does nothing.
func (p *Printer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endProgress()
	p.redraw = false
	if p.restore != nil {
		p.restore()
		p.restore = nil
	}
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

// line writes one line. String arguments are sanitized; the format itself is
// a constant written by the tool. The caller holds the lock. A line for
// standard error while the progress table is on the screen is written in its
// place, and the table is drawn again below it.
func (p *Printer) line(w io.Writer, format string, args ...any) {
	for i, a := range args {
		if s, ok := a.(string); ok {
			args[i] = Sanitize(s)
		}
	}
	text := fmt.Sprintf(format+"\n", args...)
	if w == p.errOut && p.redraw && p.progress != nil && p.progress.drawn > 0 {
		var b strings.Builder
		p.eraseTable(&b)
		b.WriteString(text)
		p.writeRaw(b.String())
		p.drawTable(true)
		return
	}
	if _, err := io.WriteString(w, text); err != nil && p.werr == nil {
		p.werr = err
	}
}

// locked runs f with the lock held.
func (p *Printer) locked(f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f()
}

func (p *Printer) blank(w io.Writer) { p.line(w, "") }

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

// plural returns "1 resource" or "n resources".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Warn implements inventory.ProgressSink.
func (p *Printer) Warn(message string) {
	p.locked(func() { p.line(p.errOut, "warning: %s", message) })
}

// FileWarning prints a warning about a line of a file.
func (p *Printer) FileWarning(path string, line int, message string) {
	p.locked(func() { p.line(p.errOut, "warning: %s line %d: %s", path, line, message) })
}

// APICall implements awsaccess.DebugSink. It prints one line per call when
// --debug is on, with the operation (the API name), region, duration and
// retries only (NFR8.2, NFR11.3, NFR11.6). Calls of several regions arrive at
// the same time; the lock keeps their lines whole.
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
	p.line(p.errOut, "debug: %s %s %dms retries=%s%s", rec.Op, rec.Region, rec.Duration.Milliseconds(), retries, suffix)
}

// Account prints the account and the region of the session.
func (p *Printer) Account(acc awsaccess.AccountContext) {
	p.locked(func() {
		p.account = acc.AccountID
		p.line(p.errOut, "Account %s · region %s", acc.AccountID, acc.Region)
	})
}

// Error prints the two-line error message (NFR11.4).
func (p *Printer) Error(what, fix string) {
	p.locked(func() {
		p.endProgress()
		p.line(p.errOut, "Error: %s", what)
		p.line(p.errOut, "  Fix: %s", fix)
	})
}

// Fatal prints a one-line message for failures that have no fix to suggest.
func (p *Printer) Fatal(message string) {
	p.locked(func() {
		p.endProgress()
		p.line(p.errOut, "Error: %s", message)
	})
}

// Done prints the elapsed time and the number of scanned resources (NFR1.1).
func (p *Printer) Done(elapsed time.Duration, resources int) {
	p.locked(func() {
		p.endProgress()
		p.line(p.errOut, "Done in %s · %s", FormatElapsed(elapsed), plural(resources, "resource"))
	})
}

// ExitCode prints the exit code line.
func (p *Printer) ExitCode(code int, meaning string) {
	p.locked(func() { p.line(p.errOut, "Exit code: %d (%s)", code, meaning) })
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
