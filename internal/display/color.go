package display

import (
	"io"
	"strings"
)

// Colors only help (BR3.7, NFR11.7): every meaning is also written with a
// symbol and words, and taking the color sequences out of colored output
// gives exactly the output without colors. The sequences are constants of
// the tool; they wrap whole lines, outside the padding.

// style is one color of the display rules (design-system-mapping.md).
type style string

const (
	styleNone      style = ""
	styleAdd       style = "\x1b[32m"   // green: +
	styleChange    style = "\x1b[33m"   // yellow: ~
	styleRemove    style = "\x1b[31m"   // red: -, x
	styleImpact    style = "\x1b[1;31m" // bold red: !, incomplete
	styleUndecided style = "\x1b[1;33m" // bold yellow: ?
	styleNote      style = "\x1b[2m"    // dim: notes
	styleReset           = "\x1b[0m"
)

// paint wraps text in s when on is true.
func paint(on bool, s style, text string) string {
	if !on || s == styleNone || text == "" {
		return text
	}
	return string(s) + text + styleReset
}

// StripColors removes the color sequences paint writes; the tests compare
// colored output with plain output through it (NFR11.7).
func StripColors(s string) string {
	for _, seq := range []string{string(styleAdd), string(styleChange), string(styleRemove), string(styleImpact),
		string(styleUndecided), string(styleNote), styleReset} {
		s = strings.ReplaceAll(s, seq, "")
	}
	return s
}

// canColor reports whether colors may be written to w (NFR11.7): w is a
// terminal, NO_COLOR is not set or empty, TERM is not "dumb", and on Windows
// the console of w accepted virtual terminal processing. Each stream is
// decided apart. The function it returns, when not nil, puts the console
// setting back; Close calls it.
func canColor(w io.Writer) (bool, func()) {
	if getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" || !isTerminal(w) {
		return false, nil
	}
	return enableVirtualTerminal(w)
}
