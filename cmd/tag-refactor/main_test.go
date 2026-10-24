package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
)

func TestRunReturnsTheExitCodeOfTheCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"scan"}, &stdout, &stderr,
		func(_ context.Context, args []string, out, _ io.Writer, _ cli.Deps) int {
			_, _ = io.WriteString(out, strings.Join(args, " "))
			return 2
		})
	if code != 2 || stdout.String() != "scan" {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
}

func TestRunTurnsAPanicIntoOneFixedLine(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), nil, &stdout, &stderr,
		func(context.Context, []string, io.Writer, io.Writer, cli.Deps) int {
			panic("secret AKIAFAKEFAKEFAKE0000 in a panic value")
		})
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stderr.String() != panicMessage+"\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "AKIA") || stdout.Len() != 0 {
		t.Fatal("the panic value or a stack trace was printed")
	}
}

// U4 NFR design §3: the context of the command ends on the signals, or when
// stop is called; Ctrl+C is always one of the signals.
func TestInterruptContextEndsWithStopOrItsParent(t *testing.T) {
	if !slices.Contains(interruptSignals, os.Interrupt) {
		t.Fatalf("signals = %v, want os.Interrupt among them", interruptSignals)
	}
	ctx, stop := interruptContext(context.Background(), interruptSignals)
	if ctx.Err() != nil {
		t.Fatal("the context must not start cancelled")
	}
	stop()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not end the context")
	}

	parent, cancel := context.WithCancel(context.Background())
	ctx, stop = interruptContext(parent, interruptSignals)
	defer stop()
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the parent did not end the context")
	}
}

func TestRunWithTheRealCLIPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--help"}, &stdout, &stderr, cli.Run)
	if code != 0 || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
