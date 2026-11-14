// Package procexec runs the outside programs of tag-refactor (cdk, node, git
// and gh) with a time limit, and stops the whole tree of processes when the
// limit passes or the context is cancelled (v0.2.0 U1, reliability design §1,
// NFR3.1 to NFR3.3, NFR3.10). It never goes through a shell.
//
// There are two ways to run a program. Without Interactive, the program reads
// the given bytes, its standard output and standard error are kept up to their
// limits, and it runs in a group of its own (a process group on Unix, a job
// object on Windows) that is stopped as one. With Interactive, the program
// talks with the terminal through TermIn and TermErr, its standard output is
// thrown away, and on Unix it stays in the foreground group of tag-refactor so
// that reading the terminal does not stop it; only that process is stopped
// then (its children can stay, an accepted risk). On Windows a job object
// holds it in both ways.
//
// Callers reach the programs through Runner, which tests stand in for.
package procexec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

// DefaultLimit is how much of standard output and of standard error is kept
// when a Command does not say (64 KiB).
const DefaultLimit = 64 << 10

// WaitDelay is how long Run waits for the output after the program was
// stopped (reliability design §1).
const WaitDelay = 5 * time.Second

// Command is one call of an outside program.
type Command struct {
	// Name is the program: a path, or what LookPath returned.
	Name string
	// Args are the arguments after the program.
	Args []string
	// Dir is the working folder; empty means the one of tag-refactor.
	Dir string
	// Env is the whole environment of the program. Nil gives it none.
	Env []string
	// Stdin is what the program reads when it is not interactive.
	Stdin []byte
	// Timeout stops the program after this long; zero means no limit but the
	// context.
	Timeout time.Duration
	// StdoutLimit and StderrLimit bound what is kept; zero means DefaultLimit.
	StdoutLimit, StderrLimit int
	// Interactive connects the program to the terminal: TermIn and TermErr.
	Interactive bool
	TermIn      io.Reader
	TermErr     io.Writer
}

// Result is how a program ended.
type Result struct {
	// Stdout and Stderr are what the program wrote, up to the limits; both are
	// empty for an interactive program, whose standard error went to TermErr.
	Stdout, Stderr []byte
	// StdoutDropped and StderrDropped count the bytes past the limits.
	StdoutDropped, StderrDropped int64
	// ExitCode is the code the program ended with.
	ExitCode int
}

// ErrTimeout is the error of a program stopped by its Timeout.
var ErrTimeout = errors.New("the program did not end in time")

// StartError is the error of a program that could not be started.
type StartError struct {
	Name string
	Err  error
}

func (e *StartError) Error() string { return "could not start " + e.Name + ": " + e.Err.Error() }
func (e *StartError) Unwrap() error { return e.Err }

// Runner runs outside programs. The error of Run is nil when the program ran
// and ended, whatever its exit code; otherwise it is a *StartError, ErrTimeout
// or the error of the context.
type Runner interface {
	Run(ctx context.Context, c Command) (Result, error)
	LookPath(name string) (string, error)
}

// System runs the programs of the computer.
type System struct{}

var _ Runner = System{}

// LookPath finds a program on PATH (and, on Windows, with the extensions of
// PATHEXT).
func (System) LookPath(name string) (string, error) { return exec.LookPath(name) }

// Run runs c and waits for it to end.
func (System) Run(ctx context.Context, c Command) (Result, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = c.Dir, c.Env, WaitDelay
	if cmd.Env == nil {
		cmd.Env = []string{}
	}
	stdout := &capped{limit: limitOr(c.StdoutLimit)}
	stderr := &capped{limit: limitOr(c.StderrLimit)}
	if c.Interactive {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = c.TermIn, io.Discard, c.TermErr
	} else {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(c.Stdin), stdout, stderr
	}
	tree := newProcessTree(cmd, c.Interactive)
	defer tree.release()
	if err := tree.start(); err != nil {
		return Result{}, &StartError{Name: c.Name, Err: err}
	}
	err := cmd.Wait()
	res := Result{Stdout: stdout.buf.Bytes(), Stderr: stderr.buf.Bytes(),
		StdoutDropped: stdout.dropped, StderrDropped: stderr.dropped, ExitCode: -1}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	switch ctxErr := ctx.Err(); {
	case errors.Is(ctxErr, context.DeadlineExceeded) && c.Timeout > 0:
		return res, ErrTimeout
	case ctxErr != nil:
		return res, ctxErr
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) && !errors.Is(err, exec.ErrWaitDelay) {
		return res, err
	}
	return res, nil
}

func limitOr(n int) int {
	if n <= 0 {
		return DefaultLimit
	}
	return n
}

// capped keeps the first limit bytes written to it and counts the rest.
type capped struct {
	buf     bytes.Buffer
	limit   int
	dropped int64
}

func (c *capped) Write(p []byte) (int, error) {
	room := c.limit - c.buf.Len()
	switch {
	case room <= 0:
		c.dropped += int64(len(p))
	case len(p) > room:
		c.buf.Write(p[:room])
		c.dropped += int64(len(p) - room)
	default:
		c.buf.Write(p)
	}
	return len(p), nil
}
