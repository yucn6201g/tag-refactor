//go:build windows

package display

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// enableVT adds ENABLE_VIRTUAL_TERMINAL_PROCESSING to the console mode of w,
// so the console understands the cursor sequences of the redrawn progress
// (NFR9.3). It reports false when w is not a console or the mode cannot be
// set, for example on consoles older than Windows 10.
func enableVT(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
