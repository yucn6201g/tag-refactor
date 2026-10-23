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
// set, for example on consoles older than Windows 10. When it changed the
// mode, it also returns a function that puts the original mode back: the
// console is shared with the shell that started the tool.
func enableVT(w io.Writer) (bool, func()) {
	f, ok := w.(*os.File)
	if !ok {
		return false, nil
	}
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false, nil
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true, nil
	}
	if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false, nil
	}
	return true, func() {
		// The tool is ending: when the console refuses, there is nothing
		// left to do, and the setting only affects cursor sequences.
		_ = windows.SetConsoleMode(handle, mode)
	}
}
