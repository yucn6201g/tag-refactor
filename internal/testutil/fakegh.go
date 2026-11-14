package testutil

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The fakes of git and gh of the tests (v0.2.0 U1 and U2,
// unit-test-instructions.md): the test binary itself, copied under the name of
// the program into a folder put first on PATH. When it starts under that name
// with the variable of its folder set, it plays the program instead of running
// the tests and writes each call to a file of that folder.
//
// There are two fakes. The fake gh (InstallFakeGH) answers as gh would; the
// real git stays in use. The fake that writes down its arguments
// (InstallFakeArgs) can stand under any names, git and gh among them: it only
// keeps the name and the arguments of each call and ends with 0. The tests of
// the resume commands (U2) run those commands in a real shell and compare the
// arguments the fake received.

// fakeGHDirEnv names the folder of the fake gh: its calls and its settings.
const fakeGHDirEnv = "TAG_REFACTOR_FAKE_GH_DIR"

// fakeArgsDirEnv names the folder of the fake that writes down its arguments.
const fakeArgsDirEnv = "TAG_REFACTOR_FAKE_ARGS_DIR"

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
	// CreateHangs makes gh pr create wait without an answer, until it is
	// stopped (AC1.1.10).
	CreateHangs bool `json:"createHangs"`
	// CreateNoURL makes gh pr create end well without printing a URL.
	CreateNoURL bool `json:"createNoURL"`
}

// FakeGH is an installed fake gh.
type FakeGH struct {
	t        testing.TB
	dir      string
	settings fakeGHSettings
}

// InstallFakeGH puts a fake gh first on PATH for the rest of the test. The
// package that calls it must call RunFakeGHIfRequested first in its TestMain.
func InstallFakeGH(t testing.TB) *FakeGH {
	t.Helper()
	dir := t.TempDir()
	bin := installBinaries(t, dir, "gh")
	f := &FakeGH{t: t, dir: dir}
	f.write()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeGHDirEnv, dir)
	return f
}

// installBinaries copies the test binary into dir/bin under each name and
// returns that folder.
func installBinaries(t testing.TB, dir string, names ...string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find the test binary: %v", err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := linkOrCopy(self, filepath.Join(bin, name)); err != nil {
			t.Fatalf("copy the test binary as %s: %v", name, err)
		}
	}
	return bin
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

func (f *FakeGH) write() {
	f.t.Helper()
	data, err := json.Marshal(f.settings)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "settings.json"), data, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// FailAuth makes gh auth status fail, as when gh is not logged in.
func (f *FakeGH) FailAuth() { f.settings.AuthFails = true; f.write() }

// FailCreate makes gh pr create fail.
func (f *FakeGH) FailCreate() { f.settings.CreateFails = true; f.write() }

// HangCreate makes gh pr create wait without an answer until it is stopped.
func (f *FakeGH) HangCreate() { f.settings.CreateHangs = true; f.write() }

// CreateWithoutURL makes gh pr create end well without printing a URL.
func (f *FakeGH) CreateWithoutURL() { f.settings.CreateNoURL = true; f.write() }

// Calls returns the calls of the fake gh so far, in order.
func (f *FakeGH) Calls() []GHCall {
	f.t.Helper()
	return readCalls[GHCall](f.t, filepath.Join(f.dir, "calls.jsonl"))
}

// readCalls reads the calls a fake wrote, one JSON object a line.
func readCalls[T any](t testing.TB, path string) []T {
	t.Helper()
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var calls []T
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		var c T
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatal(err)
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
	return dir, dir != "" && programName() == "gh"
}

// programName is the name this binary was started under, without .exe.
func programName() string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
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
		return playCreate(s, stdout, stderr)
	}
	return 0
}

func playCreate(s fakeGHSettings, stdout, stderr io.Writer) int {
	switch {
	case s.CreateHangs:
		// The limit of the step stops this process long before.
		time.Sleep(time.Hour)
		return 0
	case s.CreateFails:
		_, _ = io.WriteString(stderr, "pull request create failed: GraphQL: something went wrong\n")
		return 1
	case s.CreateNoURL:
		_, _ = io.WriteString(stdout, "Creating pull request...\n")
		return 0
	}
	_, _ = io.WriteString(stdout, FakePullRequestURL+"\n")
	return 0
}

func appendCall(dir string, call any) error {
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

// ArgsCall is one call of the fake that writes down its arguments.
type ArgsCall struct {
	// Name is the name the fake was started under, without .exe.
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// FakeArgs is an installed fake that writes down its arguments.
type FakeArgs struct {
	t   testing.TB
	dir string
}

// InstallFakeArgs puts the fake under each of names first on PATH for the rest
// of the test. The package that calls it must call FakeArgsRequested first in
// its TestMain:
//
//	if dir, ok := testutil.FakeArgsRequested(); ok {
//		os.Exit(testutil.PlayFakeArgs(dir, os.Args[1:]))
//	}
func InstallFakeArgs(t testing.TB, names ...string) *FakeArgs {
	t.Helper()
	dir := t.TempDir()
	bin := installBinaries(t, dir, names...)
	if err := os.WriteFile(filepath.Join(dir, "names.txt"), []byte(strings.Join(names, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeArgsDirEnv, dir)
	return &FakeArgs{t: t, dir: dir}
}

// FakeArgsRequested reports whether this binary was started under one of the
// names of the fake that writes down its arguments, and the folder of the fake.
func FakeArgsRequested() (string, bool) {
	dir := os.Getenv(fakeArgsDirEnv)
	if dir == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, "names.txt"))
	if err != nil {
		return "", false
	}
	return dir, slices.Contains(strings.Split(string(data), "\n"), programName())
}

// PlayFakeArgs writes down one call and returns its exit code: 0, or 3 when
// the call could not be written.
func PlayFakeArgs(dir string, args []string) int {
	if args == nil {
		args = []string{}
	}
	if err := appendCall(dir, ArgsCall{Name: programName(), Args: args}); err != nil {
		return 3
	}
	return 0
}

// Calls returns the calls of the fake so far, in order.
func (f *FakeArgs) Calls() []ArgsCall {
	f.t.Helper()
	return readCalls[ArgsCall](f.t, filepath.Join(f.dir, "calls.jsonl"))
}

// Reset forgets the calls so far.
func (f *FakeArgs) Reset() {
	f.t.Helper()
	if err := os.Remove(filepath.Join(f.dir, "calls.jsonl")); err != nil && !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
}
