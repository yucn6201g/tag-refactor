package procexec

// Unit tests of procexec, written after the implementation (team testing
// posture). The programs it runs are this test binary itself, started with
// childEnv set to the part it plays (unit-test-instructions.md).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const childEnv = "TAG_REFACTOR_PROCEXEC_CHILD"

func TestMain(m *testing.M) {
	if part := os.Getenv(childEnv); part != "" {
		os.Exit(playChild(part))
	}
	os.Exit(m.Run())
}

// playChild is what the test binary does as a child.
func playChild(part string) int {
	switch part {
	case "echo":
		in, _ := io.ReadAll(os.Stdin)
		_, _ = os.Stdout.Write(in)
		_, _ = os.Stderr.WriteString("to stderr\n")
		code, _ := strconv.Atoi(os.Getenv("CHILD_EXIT"))
		return code
	case "big":
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 100<<10))
		return 0
	case "sleep":
		// Start a grandchild that writes its process ID, then sleep.
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), childEnv+"=grandchild")
		if err := cmd.Start(); err != nil {
			return 3
		}
		time.Sleep(time.Hour)
		return 0
	case "grandchild":
		_ = os.WriteFile(os.Getenv("CHILD_PID_FILE"), []byte(strconv.Itoa(os.Getpid())), 0o600)
		time.Sleep(time.Hour)
		return 0
	case "group":
		_, _ = fmt.Fprintf(os.Stderr, "%d %d\n", processGroup(os.Getpid()), processGroup(os.Getppid()))
		return 0
	}
	return 2
}

func child(t *testing.T, part string, extra ...string) Command {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Command{Name: self, Env: append(append(os.Environ(), childEnv+"="+part), extra...)}
}

func TestRunKeepsTheOutputsAndTheExitCode(t *testing.T) {
	c := child(t, "echo", "CHILD_EXIT=3")
	c.Stdin = []byte("request")

	res, err := System{}.Run(context.Background(), c)

	if err != nil {
		t.Fatal(err)
	}
	if string(res.Stdout) != "request" || string(res.Stderr) != "to stderr\n" || res.ExitCode != 3 {
		t.Fatalf("got %q, %q, %d", res.Stdout, res.Stderr, res.ExitCode)
	}
}

func TestRunKeepsOutputUpToItsLimit(t *testing.T) {
	c := child(t, "big")
	c.StdoutLimit = 1 << 10

	res, err := System{}.Run(context.Background(), c)

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stdout) != 1<<10 || res.StdoutDropped != 99<<10 {
		t.Fatalf("kept %d, dropped %d", len(res.Stdout), res.StdoutDropped)
	}
}

func TestDefaultLimitIs64KiB(t *testing.T) {
	res, err := System{}.Run(context.Background(), child(t, "big"))

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stdout) != DefaultLimit || res.StdoutDropped != 100<<10-DefaultLimit {
		t.Fatalf("kept %d, dropped %d", len(res.Stdout), res.StdoutDropped)
	}
}

// NFR3.1, NFR3.10: a program that does not end is stopped at its limit with
// its child, and Run comes back within the limit and WaitDelay.
func TestTimeoutStopsTheProgramAndItsChildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	c := child(t, "sleep", "CHILD_PID_FILE="+pidFile)
	c.Timeout = 2 * time.Second
	start := time.Now()

	_, err := System{}.Run(context.Background(), c)

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("error %v, want ErrTimeout", err)
	}
	if took := time.Since(start); took > c.Timeout+10*time.Second {
		t.Fatalf("Run came back after %v", took)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the grandchild did not start: %v", err)
	}
	pid, _ := strconv.Atoi(string(data))
	deadline := time.Now().Add(5 * time.Second)
	for alive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("the grandchild %d is still running", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestCancelledContextStopsTheProgram(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	_, err := System{}.Run(ctx, child(t, "sleep", "CHILD_PID_FILE="+filepath.Join(t.TempDir(), "pid")))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
}

func TestAProgramThatCannotStartIsAStartError(t *testing.T) {
	_, err := System{}.Run(context.Background(), Command{Name: filepath.Join(t.TempDir(), "no-such-program")})

	var start *StartError
	if !errors.As(err, &start) || !strings.Contains(start.Error(), "could not start") {
		t.Fatalf("error %v, want a StartError", err)
	}
}

// Interactive: the terminal streams are used, standard output is thrown away,
// and nothing is kept.
func TestInteractiveUsesTheTerminal(t *testing.T) {
	c := child(t, "echo")
	var termErr bytes.Buffer
	c.Interactive, c.TermIn, c.TermErr = true, strings.NewReader("typed"), &termErr

	res, err := System{}.Run(context.Background(), c)

	if err != nil {
		t.Fatal(err)
	}
	if termErr.String() != "to stderr\n" || len(res.Stdout) != 0 || len(res.Stderr) != 0 {
		t.Fatalf("terminal %q, kept %q and %q", termErr.String(), res.Stdout, res.Stderr)
	}
}

// Reliability design §1: on Unix a program that is not interactive gets a
// group of its own; an interactive one stays in the group of tag-refactor, so
// that reading the terminal does not stop it.
func TestProcessGroups(t *testing.T) {
	if !hasProcessGroups() {
		t.Skip("process groups are a Unix matter; Windows uses a job object in both ways")
	}
	for _, interactive := range []bool{false, true} {
		c := child(t, "group")
		var termErr bytes.Buffer
		c.Interactive, c.TermErr = interactive, &termErr

		res, err := System{}.Run(context.Background(), c)

		if err != nil {
			t.Fatal(err)
		}
		out := string(res.Stderr)
		if interactive {
			out = termErr.String()
		}
		var own, parent int
		if _, err := fmt.Sscan(out, &own, &parent); err != nil {
			t.Fatalf("read %q: %v", out, err)
		}
		if (own == parent) != interactive {
			t.Errorf("interactive %v: group %d, group of tag-refactor %d", interactive, own, parent)
		}
	}
}

func TestLookPath(t *testing.T) {
	if _, err := (System{}).LookPath(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("a missing program was found")
	}
}
