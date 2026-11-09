package cdk

// Helpers of the unit tests of cdk, which are written after the
// implementation (team testing posture).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// fakeRunner stands in for the programs: missing names are not found, and run
// answers each call.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []procexec.Command
	missing map[string]bool
	run     func(c procexec.Command) (procexec.Result, error)
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.missing[name] {
		return "", exec.ErrNotFound
	}
	return filepath.Join("fake-bin", name), nil
}

func (f *fakeRunner) Run(_ context.Context, c procexec.Command) (procexec.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	if f.run == nil {
		return procexec.Result{}, nil
	}
	return f.run(c)
}

func (f *fakeRunner) recorded() []procexec.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]procexec.Command(nil), f.calls...)
}

// fixedClock moves ten seconds at each call.
func fixedClock() func() time.Time {
	t := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t = t.Add(10 * time.Second)
		return t
	}
}

// writeFiles writes files below dir: name to content.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// recordingSink keeps the lines of the states of the apps.
type recordingSink struct {
	mu    sync.Mutex
	lines []string
}

func (r *recordingSink) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
}

func (r *recordingSink) AppsWaiting(apps []string) { r.add("waiting " + strings.Join(apps, ",")) }
func (r *recordingSink) AppStarted(app, language string) {
	r.add("started " + app + " " + language)
}
func (r *recordingSink) AppDone(app string, elapsed time.Duration) {
	r.add("done " + app + " " + elapsed.String())
}
func (r *recordingSink) AppFailed(app string, elapsed time.Duration, reason string) {
	r.add("failed " + app + " " + elapsed.String() + " " + reason)
}
func (r *recordingSink) AppSkipped(app, reason string) { r.add("skipped " + app + " " + reason) }

func (r *recordingSink) got() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.lines, "\n")
}

// nodeApp makes an app in a temporary folder from the given sources, with the
// node_modules of the test app, for the tests that run the helper with node.
// It ends the test when Node.js or the node_modules are missing (BR5.1).
func nodeApp(t *testing.T, files map[string]string) App {
	t.Helper()
	src := testutil.CDKTestApp("ts-minimal")
	if !testutil.RequireCDKTools(t, src) {
		t.FailNow()
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, files)
	testutil.LinkNodeModules(t, src, dir)
	return App{Dir: ".", Abs: dir, Language: TypeScript, CDKMajor: 2}
}

// testAppCopy copies the test app for the helper.
func testAppCopy(t *testing.T) App {
	t.Helper()
	src := testutil.CDKTestApp("ts-minimal")
	if !testutil.RequireCDKTools(t, src) {
		t.FailNow()
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testutil.CopyCDKApp(t, src, dir)
	return App{Dir: ".", Abs: dir, Language: TypeScript, CDKMajor: 2}
}
