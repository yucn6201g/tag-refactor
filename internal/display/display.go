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
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Printer writes results to out and everything else to errOut. It implements
// inventory.ProgressSink and awsaccess.DebugSink.
type Printer struct {
	out    io.Writer
	errOut io.Writer
	debug  bool
	werr   error
}

var (
	_ inventory.ProgressSink = (*Printer)(nil)
	_ awsaccess.DebugSink    = (*Printer)(nil)
)

// New returns a printer for the given standard output and standard error.
func New(stdout, stderr io.Writer) *Printer {
	return &Printer{out: stdout, errOut: stderr}
}

// SetDebug turns the --debug lines on or off.
func (p *Printer) SetDebug(on bool) { p.debug = on }

// Stdout returns the result writer, for the help text of the command parser.
func (p *Printer) Stdout() io.Writer { return p.out }

// Stderr returns the message writer, for the command parser.
func (p *Printer) Stderr() io.Writer { return p.errOut }

// WriteErr returns the first error met while writing, if any.
func (p *Printer) WriteErr() error { return p.werr }

// line writes one line. String arguments are sanitized; the format itself is
// a constant written by the tool.
func (p *Printer) line(w io.Writer, format string, args ...any) {
	for i, a := range args {
		if s, ok := a.(string); ok {
			args[i] = Sanitize(s)
		}
	}
	if _, err := fmt.Fprintf(w, format+"\n", args...); err != nil && p.werr == nil {
		p.werr = err
	}
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

// RegionStarted implements inventory.ProgressSink.
func (p *Printer) RegionStarted(region string) {
	p.line(p.errOut, "scan: %s started", region)
}

// RegionDone implements inventory.ProgressSink.
func (p *Printer) RegionDone(region string, count int) {
	p.line(p.errOut, "scan: %s done (%s)", region, plural(count, "resource"))
}

// RegionFailed implements inventory.ProgressSink.
func (p *Printer) RegionFailed(region, reason string) {
	p.line(p.errOut, "scan: %s failed (%s)", region, reason)
}

// Warn implements inventory.ProgressSink.
func (p *Printer) Warn(message string) {
	p.line(p.errOut, "warning: %s", message)
}

// FileWarning prints a warning about a line of a file.
func (p *Printer) FileWarning(path string, line int, message string) {
	p.line(p.errOut, "warning: %s line %d: %s", path, line, message)
}

// APICall implements awsaccess.DebugSink. It prints one line per call when
// --debug is on, with the operation, region, duration and retries only
// (NFR8.2, NFR11.3).
func (p *Printer) APICall(rec awsaccess.CallRecord) {
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

// Account prints the account and region line.
func (p *Printer) Account(acc awsaccess.AccountContext) {
	p.line(p.errOut, "Account %s · region %s", acc.AccountID, acc.Region)
}

// Error prints the two-line error message (NFR11.4).
func (p *Printer) Error(what, fix string) {
	p.line(p.errOut, "Error: %s", what)
	p.line(p.errOut, "  Fix: %s", fix)
}

// Fatal prints a one-line message for failures that have no fix to suggest.
func (p *Printer) Fatal(message string) {
	p.line(p.errOut, "Error: %s", message)
}

// Done prints the elapsed time and the number of scanned resources (NFR1.1).
func (p *Printer) Done(elapsed time.Duration, resources int) {
	p.line(p.errOut, "Done in %s · %s", FormatElapsed(elapsed), plural(resources, "resource"))
}

// ExitCode prints the exit code line.
func (p *Printer) ExitCode(code int, meaning string) {
	p.line(p.errOut, "Exit code: %d (%s)", code, meaning)
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
