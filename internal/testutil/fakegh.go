package testutil

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The fake gh of the tests (v0.2.0 U1, unit-test-instructions.md): the test
// binary itself, copied as gh into a folder put first on PATH. When it starts
// as gh with fakeGHDirEnv set, RunFakeGHIfRequested plays gh instead of
// running the tests: it writes each call to a file of that folder and answers
// as gh would. The real git stays in use; only gh is stood in for.

// fakeGHDirEnv names the folder of the fake gh: its calls and its settings.
const fakeGHDirEnv = "TAG_REFACTOR_FAKE_GH_DIR"

// FakePullRequestURL is what the fake gh prints for gh pr create.
const FakePullRequestURL = "https://github.com/o/r/pull/7"

// GHCall is one call of the fake gh.
type GHCall struct {
	Args []string `json:"args"`
	// Body is the content of the file of --body-file, read when gh ran.
	Body string `json:"body,omitempty"`
	// TokenNames are the names of the token variables gh received, never their
	// values.
	TokenNames []string `json:"tokenNames,omitempty"`
}

// fakeGHSettings say how the fake gh answers.
type fakeGHSettings struct {
	AuthFails   bool `json:"authFails"`
	CreateFails bool `json:"createFails"`
}

// FakeGH is an installed fake gh.
type FakeGH struct {
	t   testing.TB
	dir string
}

// InstallFakeGH puts a fake gh first on PATH for the rest of the test. The
// package that calls it must call RunFakeGHIfRequested first in its TestMain.
func InstallFakeGH(t testing.TB) *FakeGH {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find the test binary: %v", err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.exe"
	}
	if err := linkOrCopy(self, filepath.Join(bin, name)); err != nil {
		t.Fatalf("copy the test binary as gh: %v", err)
	}
	f := &FakeGH{t: t, dir: dir}
	f.write(fakeGHSettings{})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeGHDirEnv, dir)
	return f
}

// linkOrCopy makes dst the same file as src: a hard link when it can, else a
// copy. On Windows it always copies: a hard link of the running test binary
// cannot be deleted while the binary runs, so the cleanup of t.TempDir would
// fail with "Access is denied".
func linkOrCopy(src, dst string) error {
	if runtime.GOOS != "windows" {
		if err := os.Link(src, dst); err == nil {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (f *FakeGH) write(s fakeGHSettings) {
	f.t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "settings.json"), data, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// FailAuth makes gh auth status fail, as when gh is not logged in.
func (f *FakeGH) FailAuth() { f.write(fakeGHSettings{AuthFails: true}) }

// FailCreate makes gh pr create fail.
func (f *FakeGH) FailCreate() { f.write(fakeGHSettings{CreateFails: true}) }

// Calls returns the calls of the fake gh so far, in order.
func (f *FakeGH) Calls() []GHCall {
	f.t.Helper()
	file, err := os.Open(filepath.Join(f.dir, "calls.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var calls []GHCall
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		var c GHCall
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			f.t.Fatal(err)
		}
		calls = append(calls, c)
	}
	return calls
}

// Find returns the first call that starts with the given arguments.
func (f *FakeGH) Find(prefix ...string) (GHCall, bool) {
	for _, c := range f.Calls() {
		if len(c.Args) >= len(prefix) && strings.Join(c.Args[:len(prefix)], "\x00") == strings.Join(prefix, "\x00") {
			return c, true
		}
	}
	return GHCall{}, false
}

// Flag returns the value after flag in the arguments of a call.
func (c GHCall) Flag(flag string) string {
	for i := 0; i+1 < len(c.Args); i++ {
		if c.Args[i] == flag {
			return c.Args[i+1]
		}
	}
	return ""
}

// FakeGHRequested reports whether this binary was started as the fake gh, and
// the folder of the fake. TestMain calls it first and, when it is true, ends
// with PlayFakeGH instead of running the tests:
//
//	if dir, ok := testutil.FakeGHRequested(); ok {
//		os.Exit(testutil.PlayFakeGH(dir, os.Args[1:], os.Stdout, os.Stderr))
//	}
//
// The writers come from the test file: only internal/display and the command
// may name the process output (the output guard test).
func FakeGHRequested() (string, bool) {
	dir := os.Getenv(fakeGHDirEnv)
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	return dir, dir != "" && base == "gh"
}

// PlayFakeGH answers one call of gh and returns its exit code.
func PlayFakeGH(dir string, args []string, stdout, stderr io.Writer) int {
	var s fakeGHSettings
	if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	call := GHCall{Args: args}
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"} {
		if _, ok := os.LookupEnv(name); ok {
			call.TokenNames = append(call.TokenNames, name)
		}
	}
	if body := call.Flag("--body-file"); body != "" {
		if data, err := os.ReadFile(body); err == nil {
			call.Body = string(data)
		}
	}
	if err := appendCall(dir, call); err != nil {
		return 3
	}
	switch {
	case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
		if s.AuthFails {
			_, _ = io.WriteString(stderr, "You are not logged into any GitHub hosts. To log in, run: gh auth login\n")
			return 1
		}
		return 0
	case len(args) >= 2 && args[0] == "pr" && args[1] == "create":
		if s.CreateFails {
			_, _ = io.WriteString(stderr, "pull request create failed: GraphQL: something went wrong\n")
			return 1
		}
		_, _ = io.WriteString(stdout, FakePullRequestURL+"\n")
		return 0
	}
	return 0
}

func appendCall(dir string, call GHCall) error {
	data, err := json.Marshal(call)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
