//go:build !windows

package main

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// U4 NFR design §3: SIGTERM (like Ctrl+C) cancels the context of the
// command, and the first signal gives the signals back to Go's default.
// Only one signal is sent: after it, a second one would end the test
// process, which is the intended behaviour of a second Ctrl+C.
func TestASignalCancelsTheCommand(t *testing.T) {
	ctx, stop := interruptContext(context.Background(), interruptSignals)
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM did not cancel the context")
	}
	if context.Cause(ctx) == nil {
		t.Fatal("the context must say why it ended")
	}
}
