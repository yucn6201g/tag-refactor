package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

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

func TestRunWithTheRealCLIPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--help"}, &stdout, &stderr, cli.Run)
	if code != 0 || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
