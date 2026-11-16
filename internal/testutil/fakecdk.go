package testutil

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The fake programs of the synth tests of v0.2.0 U3 (security design §8 and
// §10): the test binary, copied under the names cdk, node and python into a
// folder put first on PATH, or to a place the test names (the Python of a
// .venv, the cdk of node_modules/.bin). Started under one of those names with
// the variable of its folder set, it plays the program instead of running the
// tests and writes each call to a file of that folder. The variable reaches the
// programs synth starts: it is neither an AWS variable nor a token.
//
// cdk can hang after starting a grandchild that writes its process number to
// a file, so that the tests see the limit of synth and the stop of the whole
// tree of processes (NFR3.11, NFR3.16).

// fakeCDKDirEnv names the folder of the fake programs.
const fakeCDKDirEnv = "TAG_REFACTOR_FAKE_CDK_DIR"

// fakeCDKRoleEnv makes the fake the grandchild of a hanging cdk.
const fakeCDKRoleEnv = "TAG_REFACTOR_FAKE_CDK_ROLE"

// fakeCDKNames are the names the fake answers to.
var fakeCDKNames = []string{"cdk", "node", "python", "python3", "py"}

// FakeCDKSettings say how the fake programs answer.
type FakeCDKSettings struct {
	// Hang makes cdk start a grandchild and wait without an answer.
	Hang bool `json:"hang"`
	// CDKExit and CDKStderr are the exit code and the standard error of cdk.
	CDKExit   int    `json:"cdkExit"`
	CDKStderr string `json:"cdkStderr"`
	// PythonVersion is what python --version prints; empty is Python 3.12.0.
	PythonVersion string `json:"pythonVersion"`
	// PythonHang makes python start a grandchild and wait without an answer,
	// as Hang does for cdk (the limit of the version check, NFR3.14).
	PythonHang bool `json:"pythonHang"`
	// Started, when set, is a file cdk creates before it hangs.
	Started string `json:"started"`
}

// FakeCDK is an installed set of fake programs.
type FakeCDK struct {
	t   testing.TB
	dir string
	bin string
}

// InstallFakeCDK puts cdk, node and python first on PATH for the rest of the
// test. The package that calls it must call FakeCDKRequested first in its
// TestMain:
//
//	if dir, ok := testutil.FakeCDKRequested(); ok {
//		os.Exit(testutil.PlayFakeCDK(dir, os.Args[1:], os.Stdout, os.Stderr))
//	}
func InstallFakeCDK(t testing.TB, s FakeCDKSettings) *FakeCDK {
	t.Helper()
	f := NewFakeCDK(t, s)
	t.Setenv("PATH", f.bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return f
}

// NewFakeCDK makes the fake programs without putting them on PATH; the test
// places them with InstallAs and CDKIn.
func NewFakeCDK(t testing.TB, s FakeCDKSettings) *FakeCDK {
	t.Helper()
	dir := t.TempDir()
	f := &FakeCDK{t: t, dir: dir, bin: installBinaries(t, dir, fakeCDKNames...)}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeCDKDirEnv, dir)
	return f
}

// InstallAs copies the fake to path, which names the program it plays (for
// example .venv/bin/python, or .venv\Scripts\python.exe on Windows).
func (f *FakeCDK) InstallAs(path string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		f.t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := linkOrCopy(self, path); err != nil {
		f.t.Fatalf("copy the test binary to %s: %v", path, err)
	}
}

// CDKIn puts the fake cdk in node_modules/.bin of the app: a copy of the test
// binary on Unix, and on Windows a cdk.cmd that runs it, as npm makes it.
func (f *FakeCDK) CDKIn(appDir string) {
	f.t.Helper()
	bin := filepath.Join(appDir, "node_modules", ".bin")
	if runtime.GOOS != "windows" {
		f.InstallAs(filepath.Join(bin, "cdk"))
		return
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		f.t.Fatal(err)
	}
	script := "@\"" + filepath.Join(f.bin, "cdk.exe") + "\" %*\r\n"
	if err := os.WriteFile(filepath.Join(bin, "cdk.cmd"), []byte(script), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// Calls returns the calls of the fake programs so far, in order.
func (f *FakeCDK) Calls() []ArgsCall {
	f.t.Helper()
	return readCalls[ArgsCall](f.t, filepath.Join(f.dir, "calls.jsonl"))
}

// Grandchild waits up to limit for the process number the grandchild of a
// hanging cdk wrote.
func (f *FakeCDK) Grandchild(limit time.Duration) (int, bool) {
	f.t.Helper()
	deadline := time.Now().Add(limit)
	for {
		data, err := os.ReadFile(filepath.Join(f.dir, "grandchild.pid"))
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				return pid, true
			}
		}
		if time.Now().After(deadline) {
			return 0, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// FakeCDKRequested reports whether this binary was started as one of the fake
// programs, and the folder of the fake.
func FakeCDKRequested() (string, bool) {
	dir := os.Getenv(fakeCDKDirEnv)
	if dir == "" {
		return "", false
	}
	return dir, os.Getenv(fakeCDKRoleEnv) == "grandchild" || slices.Contains(fakeCDKNames, programName())
}

// PlayFakeCDK answers one call and returns its exit code. The writers come
// from the test file, as for PlayFakeGH.
func PlayFakeCDK(dir string, args []string, stdout, stderr io.Writer) int {
	if os.Getenv(fakeCDKRoleEnv) == "grandchild" {
		if err := os.WriteFile(filepath.Join(dir, "grandchild.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			return 3
		}
		time.Sleep(time.Hour)
		return 0
	}
	var s FakeCDKSettings
	if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	if args == nil {
		args = []string{}
	}
	name := programName()
	if err := appendCall(dir, ArgsCall{Name: name, Args: args}); err != nil {
		return 3
	}
	switch name {
	case "node":
		if len(args) == 1 && args[0] == "--version" {
			_, _ = io.WriteString(stdout, "v22.0.0\n")
		}
		return 0
	case "cdk":
		return playCDK(s, stderr)
	}
	if s.PythonHang {
		return hang(s)
	}
	version := s.PythonVersion
	if version == "" {
		version = "Python 3.12.0"
	}
	_, _ = io.WriteString(stdout, version+"\n")
	return 0
}

func playCDK(s FakeCDKSettings, stderr io.Writer) int {
	if !s.Hang {
		_, _ = io.WriteString(stderr, s.CDKStderr)
		return s.CDKExit
	}
	return hang(s)
}

// hang starts a grandchild that writes its process number, creates the file
// Started and waits.
func hang(s FakeCDKSettings) int {
	self, err := os.Executable()
	if err != nil {
		return 3
	}
	child := exec.Command(self)
	child.Env = append(os.Environ(), fakeCDKRoleEnv+"=grandchild")
	if err := child.Start(); err != nil {
		return 3
	}
	if s.Started != "" {
		_ = os.WriteFile(s.Started, nil, 0o600)
	}
	// The limit of synth stops this process, and the grandchild with it, long
	// before.
	time.Sleep(time.Hour)
	return 0
}
