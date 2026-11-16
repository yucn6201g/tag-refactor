package cdk

// Unit tests of the real synth, the limits, the stops and the cleanup of
// v0.2.0 U3 (NFR2.1, NFR2.5, NFR2.6, NFR3.11, NFR3.14, NFR4.5, security design
// §2, §4 and §8), written after the implementation. The fake programs of
// testutil play cdk, node and python (TestMain); the real synth uses copies of
// the test apps, never the test apps themselves.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// recordingSystem runs the real programs and keeps the commands.
type recordingSystem struct {
	procexec.System
	mu    sync.Mutex
	calls []procexec.Command
}

func (r *recordingSystem) Run(ctx context.Context, c procexec.Command) (procexec.Result, error) {
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
	return r.System.Run(ctx, c)
}

// synthCall returns the command of cdk synth.
func (r *recordingSystem) synthCall(t *testing.T) procexec.Command {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if len(c.Args) > 0 && c.Args[0] == "synth" {
			return c
		}
	}
	t.Fatalf("no cdk synth in %+v", r.calls)
	return procexec.Command{}
}

// requireEmpty fails when dir holds anything.
func requireEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("%s holds %v", dir, names)
	}
}

// requireGone waits up to ten seconds for the process pid to end.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for testutil.ProcessAlive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("the grandchild %d is still running", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// realTemp is a temporary folder by its real path, as Discover's root is.
func realTemp(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// NFR3.11, NFR3.14, NFR4.5: the limit of the version check, the limit of synth
// and a stop end the whole tree of processes, and leave nothing in
// Config.TempDir nor in the app.
func TestCDKStopsLeaveNothingBehind(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings testutil.FakeCDKSettings
		python   bool
		cancel   bool
		reason   FailureReason
		tool     string
	}{
		{"the version check over its limit", testutil.FakeCDKSettings{PythonHang: true}, true, false, ReasonTimeout, "python"},
		{"synth over its limit", testutil.FakeCDKSettings{Hang: true}, false, false, ReasonTimeout, "cdk"},
		{"a stop during synth", testutil.FakeCDKSettings{Hang: true}, false, true, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := testutil.InstallFakeCDK(t, tc.settings)
			tmp := t.TempDir()
			app := App{Dir: "a", Abs: t.TempDir(), Language: TypeScript, CDKMajor: 2, Command: "node app.js"}
			if tc.python {
				app.Language, app.Command = Python, "python app.py"
			}
			writeFiles(t, app.Abs, map[string]string{"cdk.json": `{}`})
			s := NewSynthesizer(Config{TempDir: tmp})
			// The limits leave the fake, a copy of this large test binary, time
			// to start its grandchild while the other packages test in parallel.
			s.VersionTimeout = 8 * time.Second
			limit := 8 * time.Second
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				limit = time.Minute
				go func() {
					if _, ok := fake.Grandchild(30 * time.Second); ok {
						cancel()
					}
				}()
			}

			syn, err := s.Synthesize(ctx, app, Options{Timeout: limit}, nil)

			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error %v, failure %+v", err, syn.Failure)
				}
			} else if err != nil || syn.Failure == nil || syn.Failure.Reason != tc.reason || syn.Failure.Tool != tc.tool {
				t.Fatalf("error %v, failure %+v; want %s %s", err, syn.Failure, tc.reason, tc.tool)
			}
			pid, ok := fake.Grandchild(5 * time.Second)
			if !ok {
				t.Fatal("the fake started no grandchild")
			}
			requireGone(t, pid)
			requireEmpty(t, tmp)
			if entries, _ := os.ReadDir(app.Abs); len(entries) != 1 {
				t.Fatalf("the app holds %d entries", len(entries))
			}
		})
	}
}

// NFR4.5: a synth that fails leaves nothing in Config.TempDir; a standard
// error that says it needs AWS is needsAWS.
func TestCDKCleanupAfterAFailure(t *testing.T) {
	for _, tc := range []struct {
		settings testutil.FakeCDKSettings
		reason   FailureReason
	}{
		{testutil.FakeCDKSettings{CDKExit: 1, CDKStderr: "Unable to resolve AWS account to use.\n"}, ReasonNeedsAWS},
		{testutil.FakeCDKSettings{CDKExit: 1, CDKStderr: "Error: boom\n"}, ReasonSynthError},
		{testutil.FakeCDKSettings{}, ReasonSynthError},
	} {
		testutil.InstallFakeCDK(t, tc.settings)
		tmp := t.TempDir()
		app := App{Dir: "a", Abs: t.TempDir(), Language: TypeScript, CDKMajor: 2}

		syn, err := NewSynthesizer(Config{TempDir: tmp}).Synthesize(context.Background(), app, Options{}, nil)

		if err != nil || syn.Failure == nil || syn.Failure.Reason != tc.reason {
			t.Fatalf("%+v: error %v, failure %+v", tc.settings, err, syn.Failure)
		}
		requireEmpty(t, tmp)
		requireEmpty(t, app.Abs)
	}
}

// NFR4.5: a stop in the middle of Analyze or Recheck leaves nothing in
// Config.TempDir.
func TestCDKHelperStopLeavesNothing(t *testing.T) {
	for _, recheck := range []bool{false, true} {
		tmp := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		runner := helperRunner("v22.0.0", func() (procexec.Result, error) {
			cancel()
			return procexec.Result{}, context.Canceled
		})
		an := NewAnalyzer(Config{Runner: runner, TempDir: tmp})
		app := App{Dir: ".", Abs: t.TempDir()}

		var err error
		if recheck {
			_, err = an.Recheck(ctx, app, map[string][]byte{"lib/a.ts": []byte("x")}, []TraceSite{{ConstructPath: "s/B", File: "lib/a.ts", Line: 1}})
		} else {
			_, err = an.Analyze(ctx, app, AnalyzeRequest{})
		}

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("recheck %v: error %v", recheck, err)
		}
		requireEmpty(t, tmp)
		cancel()
	}
}

// toolSink is a ProgressSink that takes the failures with their program too.
type toolSink struct{ recordingSink }

func (s *toolSink) AppToolFailed(app string, elapsed time.Duration, reason, tool string) {
	s.add("tool failed " + app + " " + elapsed.String() + " " + reason + " " + tool)
}

// Security design §10, contract C6: a sink that is a ToolFailureSink gets the
// failure with its program instead of AppFailed; another gets AppFailed.
func TestCDKToolFailureSinkTakesTheProgram(t *testing.T) {
	app := App{Dir: "py", Abs: t.TempDir(), Language: Python, CDKMajor: 2, Command: "pipenv run python app.py"}
	runner := &fakeRunner{missing: map[string]bool{"pipenv": true}}
	s := NewSynthesizer(Config{Runner: runner, Now: fixedClock()})

	withTool := &toolSink{}
	if _, err := s.Synthesize(context.Background(), app, Options{}, withTool); err != nil {
		t.Fatal(err)
	}
	plain := &recordingSink{}
	if _, err := s.Synthesize(context.Background(), app, Options{}, plain); err != nil {
		t.Fatal(err)
	}

	if want := "started py python\ntool failed py 10s toolMissing pipenv"; withTool.got() != want {
		t.Fatalf("tool sink:\n%s\nwant:\n%s", withTool.got(), want)
	}
	if want := "started py python\nfailed py 10s toolMissing"; plain.got() != want {
		t.Fatalf("plain sink:\n%s\nwant:\n%s", plain.got(), want)
	}
	if len(runner.recorded()) != 0 {
		t.Fatalf("a program ran: %+v", runner.recorded())
	}
}

// venvFiles lists the files of the .venv of app, all but those below
// __pycache__, with their sizes, and the content of pyvenv.cfg.
func venvFiles(t *testing.T, app string) []string {
	t.Helper()
	root := filepath.Join(app, ".venv")
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files = append(files, rel+" "+strconv.FormatInt(info.Size(), 10))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "pyvenv.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	return append(files, string(cfg))
}

// pyMinimalCopy copies py-minimal below parent, at rel, and finds it with
// Discover. It ends the test when the Python test app is not ready.
func pyMinimalCopy(t *testing.T, parent, rel string) App {
	t.Helper()
	src := testutil.CDKTestApp("py-minimal")
	if !testutil.RequirePythonTools(t, src) {
		t.FailNow()
	}
	dst := filepath.Join(parent, filepath.FromSlash(rel))
	testutil.CopyCDKApp(t, src, dst)
	apps, err := NewSynthesizer(Config{}).Discover(context.Background(), parent, nil)
	if err != nil || len(apps) != 1 || apps[0].Language != Python || apps[0].CDKMajor != 2 {
		t.Fatalf("apps %+v, %v", apps, err)
	}
	return apps[0]
}

// BR2.2, NFR2.6, NFR4.5: py-minimal synthesizes with the Python of its .venv,
// which does not change; nothing is left in Config.TempDir or the app.
func TestCDKPythonAppRealSynth(t *testing.T) {
	src := testutil.CDKTestApp("py-minimal")
	if !testutil.RequirePythonTools(t, src) {
		t.FailNow()
	}
	before := venvFiles(t, src)
	app := pyMinimalCopy(t, realTemp(t), "py")
	tmp := t.TempDir()
	sink := &recordingSink{}

	syn, err := NewSynthesizer(Config{TempDir: tmp}).Synthesize(context.Background(), app, Options{}, sink)

	if err != nil || syn.Failure != nil {
		t.Fatalf("error %v, failure %+v\n%s", err, syn.Failure, strings.Join(syn.Debug, "\n"))
	}
	if len(syn.Stacks) != 1 || syn.Stacks[0].Name != "tag-refactor-cdk-py-minimal" || len(syn.Stacks[0].Resources) == 0 {
		t.Fatalf("stacks %+v", syn.Stacks)
	}
	if !strings.Contains(strings.Join(syn.Debug, "\n"), "(Python 3.") {
		t.Fatalf("debug:\n%s", strings.Join(syn.Debug, "\n"))
	}
	if !strings.HasPrefix(sink.got(), "started py python\ndone py ") {
		t.Fatalf("sink:\n%s", sink.got())
	}
	requireEmpty(t, tmp)
	if _, err := os.Stat(filepath.Join(app.Abs, "cdk.out")); !os.IsNotExist(err) {
		t.Fatalf("cdk.out was written: %v", err)
	}
	if after := venvFiles(t, src); !slices.Equal(before, after) {
		t.Fatal("the .venv of py-minimal changed")
	}
}

// Security design §4, NFR6.17: below a folder with a space, the Python of the
// .venv is given to --app in double quotes on Unix; on Windows, where the
// quotes do not reach cdk.cmd (windowsQuotedApp), --app is not given and the
// .venv is first on PATH alone, and an app whose cdk.json names
// .venv\Scripts\python.exe synthesizes (the way round of the README). Below a
// folder with $ on Unix, --app is not given either.
func TestCDKPythonAppUnderSpacedFolder(t *testing.T) {
	windows := runtime.GOOS == "windows"
	cases := []struct {
		name   string
		folder string
		quoted bool
	}{{"a space", "my app", !windows || windowsQuotedApp}}
	if !windows {
		cases = append(cases, struct {
			name   string
			folder string
			quoted bool
		}{"a dollar", "my$app", false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := pyMinimalCopy(t, realTemp(t), tc.folder+"/py")
			if windows {
				// python3 is not in the .venv of Windows, so it would be the
				// python of PATH (R-02); python is the .venv's, first on PATH.
				app.Command = "python app.py"
				writeFiles(t, app.Abs, map[string]string{"cdk.json": `{"app": "python app.py"}`})
			}
			runner := &recordingSystem{}

			syn, err := NewSynthesizer(Config{Runner: runner}).Synthesize(context.Background(), app, Options{}, nil)

			if err != nil {
				t.Fatal(err)
			}
			c := runner.synthCall(t)
			i := slices.Index(c.Args, "--app")
			debug := strings.Join(syn.Debug, "\n")
			if tc.quoted {
				if i < 0 || !strings.HasPrefix(c.Args[i+1], `"`) || !strings.HasSuffix(c.Args[i+1], `" app.py`) {
					t.Fatalf("args %q", c.Args)
				}
			} else if i >= 0 || !strings.Contains(debug, ".venv: ") {
				t.Fatalf("args %q, debug:\n%s", c.Args, debug)
			}
			if !windows && syn.Failure != nil {
				t.Fatalf("failure %+v\n%s", syn.Failure, debug)
			}
			if !windows {
				return
			}
			// The way round of the README: the app names the Python of the .venv.
			app.Command = `.venv\Scripts\python.exe app.py`
			writeFiles(t, app.Abs, map[string]string{"cdk.json": `{"app": ".venv\\Scripts\\python.exe app.py"}`})

			syn, err = NewSynthesizer(Config{}).Synthesize(context.Background(), app, Options{}, nil)

			if err != nil || syn.Failure != nil {
				t.Fatalf("with the Python of the .venv in cdk.json: %v, %+v\n%s", err, syn.Failure, strings.Join(syn.Debug, "\n"))
			}
		})
	}
}

// BR2.6, NFR2.1: ts-lookup and ts-profile need AWS; the AWS variables of the
// process never reach cdk synth.
func TestCDKNeedsAWSWithoutTheAWSVariables(t *testing.T) {
	modules := testutil.CDKTestApp("ts-minimal")
	if !testutil.RequireCDKTools(t, modules) {
		t.FailNow()
	}
	key := "AKIA" + strings.Repeat("Z", 16)
	t.Setenv("AWS_ACCESS_KEY_ID", key)
	t.Setenv("AWS_SECRET_ACCESS_KEY", strings.Repeat("s", 40))
	t.Setenv("AWS_PROFILE", "tag-refactor-test-profile")
	for _, name := range []string{"ts-lookup", "ts-profile"} {
		t.Run(name, func(t *testing.T) {
			root := realTemp(t)
			dst := filepath.Join(root, "app")
			testutil.CopyCDKApp(t, testutil.CDKTestApp(name), dst)
			testutil.LinkNodeModules(t, modules, dst)
			apps, err := NewSynthesizer(Config{}).Discover(context.Background(), root, nil)
			if err != nil || len(apps) != 1 {
				t.Fatalf("apps %+v, %v", apps, err)
			}
			runner := &recordingSystem{}

			syn, err := NewSynthesizer(Config{Runner: runner}).Synthesize(context.Background(), apps[0], Options{}, nil)

			if err != nil || syn.Failure == nil || syn.Failure.Reason != ReasonNeedsAWS {
				t.Fatalf("error %v, failure %+v\n%s", err, syn.Failure, strings.Join(syn.Debug, "\n"))
			}
			for _, kv := range runner.synthCall(t).Env {
				name, value, _ := strings.Cut(kv, "=")
				switch strings.ToUpper(name) {
				case "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_PROFILE":
					t.Errorf("%s reached cdk synth", name)
				}
				if strings.Contains(value, key) {
					t.Errorf("the key reached cdk synth in %s", name)
				}
			}
		})
	}
}
