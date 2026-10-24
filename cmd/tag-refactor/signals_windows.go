//go:build windows

package main

import "os"

// interruptSignals stop the command: Ctrl+C (and Ctrl+Break, which Go
// delivers as os.Interrupt too).
var interruptSignals = []os.Signal{os.Interrupt}
