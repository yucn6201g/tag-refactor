//go:build !windows

package main

import (
	"os"
	"syscall"
)

// interruptSignals stop the command: Ctrl+C and SIGTERM.
var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
