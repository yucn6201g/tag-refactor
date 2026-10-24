// Command tag-refactor finds inconsistent AWS tag keys and plans how to unify
// them, showing which IAM policies reference the tags that would change.
//
// This package is the only place, together with internal/display, that may
// touch the process standard streams.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/yucn6201g/tag-refactor/internal/cli"
)

// panicMessage is the fixed line printed when the command panics. The panic
// value and the stack trace are never printed: they could contain data read
// from AWS or from the local AWS configuration.
const panicMessage = "Error: tag-refactor stopped because of an unexpected internal error"

// runFunc has the signature of cli.Run. It is a parameter of run so that tests
// can exercise the panic guard.
type runFunc func(ctx context.Context, args []string, stdout, stderr io.Writer, deps cli.Deps) int

func main() {
	ctx, stop := interruptContext(context.Background(), interruptSignals)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.Run)
	stop()
	os.Exit(code)
}

// interruptContext returns a context that Ctrl+C (and on Unix SIGTERM)
// cancels (U4 NFR design §3): the AWS calls stop, and the command cleans up
// its temporary files, puts the terminal back and says it was interrupted.
// The first signal also gives the signals back to Go's default, so a second
// Ctrl+C ends the process at once.
func interruptContext(parent context.Context, signals []os.Signal) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, signals...)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}

// run calls the command line and turns an unexpected panic into exit code 1
// with one fixed line on standard error.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, fn runFunc) (code int) {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintln(stderr, panicMessage)
			code = 1
		}
	}()
	return fn(ctx, args, stdout, stderr, cli.Deps{})
}
