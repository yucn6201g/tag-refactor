package display

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Whether progress can be redrawn in place is decided from standard error
// only, never from standard output (NFR11.5, observability design §1). The
// checks are variables so that tests can stand in for a terminal.
var (
	// isTerminal reports whether w is a terminal.
	isTerminal = func(w io.Writer) bool {
		f, ok := w.(*os.File)
		return ok && term.IsTerminal(int(f.Fd()))
	}
	// enableVirtualTerminal turns on the handling of the cursor sequences
	// on w and reports whether it worked, with a function that puts the
	// previous setting back (nil when nothing was changed). Only Windows
	// needs to do anything (NFR9.3).
	enableVirtualTerminal = enableVT
	// getenv reads the environment.
	getenv = os.Getenv
)

// canRedraw reports whether progress on w may be redrawn in place: w is a
// terminal, TERM is not "dumb", and on Windows the console accepted virtual
// terminal processing. In every other case, including a terminal that cannot
// be recognized (Git Bash's mintty), lines are appended, the safe side. The
// function it returns, when not nil, puts the terminal setting back.
func canRedraw(w io.Writer) (bool, func()) {
	if getenv("TERM") == "dumb" {
		return false, nil
	}
	if !isTerminal(w) {
		return false, nil
	}
	return enableVirtualTerminal(w)
}
