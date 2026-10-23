//go:build windows

package display

import (
	"bytes"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// NFR9.3: enableVT turns virtual terminal processing on and gives a function
// that puts the previous console mode back. It needs a console; without one
// (as on most CI runners) the test is skipped.
func TestEnableVTRestoresTheConsoleMode(t *testing.T) {
	if ok, restore := enableVT(&bytes.Buffer{}); ok || restore != nil {
		t.Fatal("a buffer is not a console")
	}
	console, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no console: %v", err)
	}
	defer func() { _ = console.Close() }()
	handle := windows.Handle(console.Fd())
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		t.Skipf("no console mode: %v", err)
	}
	t.Cleanup(func() { _ = windows.SetConsoleMode(handle, original) })
	without := original &^ windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(handle, without); err != nil {
		t.Skipf("console mode cannot be set: %v", err)
	}

	ok, restore := enableVT(console)
	if !ok || restore == nil {
		t.Skip("this console does not accept virtual terminal processing")
	}
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil || mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		t.Fatalf("mode %#x after enableVT, err %v", mode, err)
	}
	if ok, again := enableVT(console); !ok || again != nil {
		t.Fatal("when the mode is already on, there is nothing to restore")
	}
	restore()
	if err := windows.GetConsoleMode(handle, &mode); err != nil || mode != without {
		t.Fatalf("mode %#x after restore, want %#x (err %v)", mode, without, err)
	}
}
