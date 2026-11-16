package cdk

// Unit tests of the Python of a Python app (v0.2.0 U3, security design §2 to
// §4), written after the implementation.

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// NFR2.5: the folder of the programs of the .venv goes first on the one PATH
// variable, VIRTUAL_ENV is replaced, and the vars given do not change.
func TestPythonWithVenv(t *testing.T) {
	root := t.TempDir()
	venv := filepath.Join(root, ".venv")
	sep := string(os.PathListSeparator)
	unixBin, winBin := filepath.Join(venv, "bin"), filepath.Join(venv, "Scripts")
	for _, tc := range []struct {
		name    string
		vars    []string
		windows bool
		want    []string
	}{
		{"unix form", []string{"PATH=/usr/bin", "HOME=/h", "VIRTUAL_ENV=/old"}, false,
			[]string{"HOME=/h", "PATH=" + unixBin + sep + "/usr/bin", "VIRTUAL_ENV=" + venv}},
		{"windows form keeps the name Path", []string{"Path=C:\\Windows", "virtual_env=C:\\old"}, true,
			[]string{"Path=" + winBin + sep + "C:\\Windows", "VIRTUAL_ENV=" + venv}},
		{"Path is another variable on Unix", []string{"Path=x"}, false,
			[]string{"Path=x", "PATH=" + unixBin, "VIRTUAL_ENV=" + venv}},
		{"no PATH", []string{"HOME=/h"}, false, []string{"HOME=/h", "PATH=" + unixBin, "VIRTUAL_ENV=" + venv}},
		{"the .venv is first already", []string{"PATH=" + unixBin + sep + "/usr/bin"}, false,
			[]string{"PATH=" + unixBin + sep + "/usr/bin", "VIRTUAL_ENV=" + venv}},
		{"one PATH only", []string{"PATH=/a", "PATH=/b"}, false, []string{"PATH=" + unixBin + sep + "/a", "VIRTUAL_ENV=" + venv}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			given := slices.Clone(tc.vars)

			got := withVenv(tc.vars, venv, tc.windows)

			if !slices.Equal(got, tc.want) {
				t.Fatalf("withVenv = %q, want %q", got, tc.want)
			}
			if !slices.Equal(tc.vars, given) {
				t.Fatalf("the vars changed: %q", tc.vars)
			}
		})
	}
}

// venvPythonIn makes the Python of the .venv of dir, as the OS names it.
func venvPythonIn(t *testing.T, dir string) string {
	t.Helper()
	p := testutil.VenvPython(dir)
	writeFiles(t, filepath.Dir(p), map[string]string{filepath.Base(p): ""})
	return p
}

// BR2.2, NFR2.6: the Python of the .venv right in the app folder comes first;
// else the first word of the app command is looked for on PATH.
func TestPythonFor(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		missing map[string]bool
		setup   func(t *testing.T, app string) string
		path    string
		tool    string
	}{
		{"python3 on PATH", "python3 app.py", nil, nil, filepath.Join("fake-bin", "python3"), ""},
		{"python3 missing", "python3 app.py", map[string]bool{"python3": true}, nil, "", "python"},
		{"pipenv on PATH is not checked", "pipenv run python app.py", nil, nil, "", ""},
		{"pipenv missing", "pipenv run python app.py", map[string]bool{"pipenv": true}, nil, "", "pipenv"},
		{"pipenv by an absolute path", "/home/alice/bin/pipenv run python app.py", map[string]bool{"/home/alice/bin/pipenv": true}, nil, "", "pipenv"},
		{"empty command", "", nil, nil, "", "python"},
		{"a .venv in the parent only", "python3 app.py", nil, func(t *testing.T, app string) string {
			venvPythonIn(t, filepath.Dir(app))
			return ""
		}, filepath.Join("fake-bin", "python3"), ""},
		{"a .venv python that is a folder", "python3 app.py", nil, func(t *testing.T, app string) string {
			if err := os.MkdirAll(testutil.VenvPython(app), 0o700); err != nil {
				t.Fatal(err)
			}
			return ""
		}, filepath.Join("fake-bin", "python3"), ""},
		{"the .venv of the app", "python3 app.py", map[string]bool{"python3": true}, func(t *testing.T, app string) string {
			return venvPythonIn(t, app)
		}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := filepath.Join(t.TempDir(), "app")
			if err := os.MkdirAll(app, 0o700); err != nil {
				t.Fatal(err)
			}
			want := tc.path
			if tc.setup != nil {
				if p := tc.setup(t, app); p != "" {
					want = p
				}
			}
			s := NewSynthesizer(Config{Runner: &fakeRunner{missing: tc.missing}})

			in, failure := s.pythonFor(App{Dir: "app", Abs: app, Language: Python, CDKMajor: 2, Command: tc.command})

			switch {
			case tc.tool != "":
				if failure == nil || failure.Reason != ReasonToolMissing || failure.Tool != tc.tool {
					t.Fatalf("failure %+v, want toolMissing %s", failure, tc.tool)
				}
			case failure != nil:
				t.Fatalf("failure %+v", failure)
			case in.path != want:
				t.Fatalf("python %q, want %q", in.path, want)
			}
			if tc.setup != nil && want != tc.path {
				arg, _ := appCommand(tc.command, want, runtime.GOOS == "windows")
				if in.venv != filepath.Join(app, ".venv") || in.appArg != arg {
					t.Fatalf("interpreter %+v", in)
				}
			}
		})
	}
}

// lookRunner records the names LookPath is asked and finds none of those in
// missing.
type lookRunner struct {
	*fakeRunner
	looked  []string
	missing map[string]bool
}

func (l *lookRunner) LookPath(name string) (string, error) {
	l.looked = append(l.looked, name)
	if l.missing[name] {
		return "", errors.New("not found")
	}
	return name, nil
}

// R-01: without a .venv, a first word with a path that is not absolute is the
// app's, as cdk synth runs in the app folder, not the folder tag-refactor was
// started in; a word with no separator is looked for on PATH as it is.
func TestPythonForRelativePath(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		rel           string
	}{
		{"a slash", "venv/bin/python app.py", filepath.Join("venv", "bin", "python")},
		{"a backslash", `venv\Scripts\python.exe app.py`, filepath.Join("venv", "Scripts", "python.exe")},
		{"dot slash", "./tools/run app.py", filepath.Join("tools", "run")},
		{"no separator", "python3 app.py", ""},
	} {
		for _, missing := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				app := App{Dir: "app", Abs: filepath.Join(t.TempDir(), "app"), Language: Python, CDKMajor: 2, Command: tc.command}
				want := firstWordOf(tc.command)
				if tc.rel != "" {
					want = filepath.Join(app.Abs, tc.rel)
				}
				l := &lookRunner{fakeRunner: &fakeRunner{}, missing: map[string]bool{want: missing}}

				in, failure := NewSynthesizer(Config{Runner: l}).pythonFor(app)

				if len(l.looked) != 1 || l.looked[0] != want {
					t.Fatalf("LookPath asked %q, want %q", l.looked, want)
				}
				if missing != (failure != nil && failure.Reason == ReasonToolMissing) {
					t.Fatalf("missing %v, failure %+v, interpreter %+v", missing, failure, in)
				}
			})
		}
	}
}

func firstWordOf(command string) string {
	w, _ := firstWord(command)
	return w
}

// R-02: when --app is not replaced, the Python that is checked is the one the
// first word reaches: the one in the folder of the programs of the .venv, else
// the one on PATH, and the debug lines say --app was not replaced.
func TestPythonForNotReplaced(t *testing.T) {
	// A folder name the Python of the .venv cannot be quoted for.
	folder := "a$b"
	if runtime.GOOS == "windows" {
		folder = "a b"
	}
	for _, tc := range []struct {
		name    string
		command string
		own     bool // the .venv has a program of the first word
		missing bool // PATH has none
		want    string
		tool    string
	}{
		{"on PATH", "python3 app.py", false, false, filepath.Join("fake-bin", "python3"), ""},
		{"in the .venv", "python3 app.py", true, false, "", ""},
		{"nowhere", "python3 app.py", false, true, "", "python"},
		{"another command is not checked", "pipenv run python app.py", false, false, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := filepath.Join(t.TempDir(), folder)
			python := venvPythonIn(t, app)
			own := filepath.Join(filepath.Dir(python), "python3")
			if runtime.GOOS == "windows" {
				own += ".exe"
			}
			if tc.own {
				writeFiles(t, filepath.Dir(own), map[string]string{filepath.Base(own): ""})
				tc.want = own
			}
			missing := map[string]bool{}
			if tc.missing {
				missing["python3"] = true
			}
			s := NewSynthesizer(Config{Runner: &fakeRunner{missing: missing}})

			in, failure := s.pythonFor(App{Dir: "app", Abs: app, Language: Python, CDKMajor: 2, Command: tc.command})

			switch {
			case tc.tool != "":
				if failure == nil || failure.Reason != ReasonToolMissing || failure.Tool != tc.tool {
					t.Fatalf("failure %+v, want toolMissing %s", failure, tc.tool)
				}
			case failure != nil:
				t.Fatalf("failure %+v", failure)
			case in.appArg != "" || in.venv == "":
				t.Fatalf("interpreter %+v", in)
			case tc.command == "pipenv run python app.py":
				if in.notReplaced || in.path != python {
					t.Fatalf("interpreter %+v", in)
				}
			case !in.notReplaced || in.path != tc.want:
				t.Fatalf("interpreter %+v, want path %q", in, tc.want)
			}
		})
	}
}

// R-02: the synth says in the debug lines that --app was not replaced.
func TestPythonNotReplacedIsSaidInDebug(t *testing.T) {
	folder := "a$b"
	if runtime.GOOS == "windows" {
		folder = "a b"
	}
	app := App{Dir: "py", Abs: filepath.Join(t.TempDir(), folder), Language: Python, CDKMajor: 2, Command: "python3 app.py"}
	venvPythonIn(t, app.Abs)
	runner := versionRunner(t, func(procexec.Command) (procexec.Result, error) {
		return procexec.Result{Stdout: []byte("Python 3.12.1\n")}, nil
	})

	syn, err := NewSynthesizer(Config{Runner: runner}).Synthesize(context.Background(), app, Options{}, nil)

	if err != nil || syn.Failure != nil {
		t.Fatalf("%v, %+v", err, syn.Failure)
	}
	if !strings.Contains(strings.Join(syn.Debug, "\n"), "--app was not replaced") {
		t.Fatalf("debug:\n%s", strings.Join(syn.Debug, "\n"))
	}
}

// pythonApp is a Python app with no .venv; the runner answers the version
// check with answer and writes an empty assembly for cdk.
func pythonApp(t *testing.T) App {
	t.Helper()
	return App{Dir: "py", Abs: t.TempDir(), Language: Python, CDKMajor: 2, Command: "python3 app.py"}
}

func versionRunner(t *testing.T, answer func(c procexec.Command) (procexec.Result, error)) *fakeRunner {
	return &fakeRunner{run: func(c procexec.Command) (procexec.Result, error) {
		if slices.Equal(c.Args, []string{"--version"}) {
			return answer(c)
		}
		writeAssembly(t, c, map[string]string{"manifest.json": `{"artifacts": {}}`})
		return procexec.Result{}, nil
	}}
}

// NFR6.16, NFR3.14: Python older than 3.10 is refused, the version is read from
// standard output or standard error, and a failed check never runs cdk.
func TestPythonCheckTheVersion(t *testing.T) {
	answer := func(stdout, stderr string, exit int, err error) func(procexec.Command) (procexec.Result, error) {
		return func(procexec.Command) (procexec.Result, error) {
			return procexec.Result{Stdout: []byte(stdout), Stderr: []byte(stderr), ExitCode: exit}, err
		}
	}
	for _, tc := range []struct {
		name   string
		answer func(procexec.Command) (procexec.Result, error)
		reason FailureReason
		debug  string
	}{
		{"3.8.18", answer("Python 3.8.18\n", "", 0, nil), ReasonToolTooOld, "is older than 3.10"},
		{"3.9.0", answer("Python 3.9.0\n", "", 0, nil), ReasonToolTooOld, "is older than 3.10"},
		{"3.9.0 on standard error", answer("", "Python 3.9.0\n", 0, nil), ReasonToolTooOld, "is older than 3.10"},
		{"3.10.0", answer("Python 3.10.0\n", "", 0, nil), "", "(Python 3.10)"},
		{"3.14.5", answer("Python 3.14.5\r\n", "", 0, nil), "", "(Python 3.14)"},
		{"4.0", answer("Python 4.0.1\n", "", 0, nil), "", "(Python 4.0)"},
		{"an answer that is not read", answer("hello from a shim\nmore\n", "", 0, nil), ReasonToolMissing, "answered hello from a shim"},
		{"exit 1", answer("Python 3.12.0\n", "", 1, nil), ReasonToolMissing, ""},
		{"cannot start", answer("", "", 0, &procexec.StartError{Name: "python3", Err: errors.New("x")}), ReasonToolMissing, ""},
		{"timeout", answer("", "", 0, procexec.ErrTimeout), ReasonTimeout, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := versionRunner(t, tc.answer)
			s := NewSynthesizer(Config{Runner: runner})

			syn, err := s.Synthesize(context.Background(), pythonApp(t), Options{}, nil)

			if err != nil {
				t.Fatal(err)
			}
			calls := runner.recorded()
			if tc.reason == "" {
				if syn.Failure != nil || len(calls) != 2 {
					t.Fatalf("failure %+v, %d calls", syn.Failure, len(calls))
				}
			} else {
				if syn.Failure == nil || syn.Failure.Reason != tc.reason || syn.Failure.Tool != "python" {
					t.Fatalf("failure %+v, want %s python", syn.Failure, tc.reason)
				}
				if len(calls) != 1 {
					t.Fatalf("cdk ran after a failed version check: %+v", calls)
				}
			}
			if calls[0].Timeout != pythonVersionLimit || calls[0].Name != filepath.Join("fake-bin", "python3") {
				t.Fatalf("version check %+v", calls[0])
			}
			if tc.debug != "" && !strings.Contains(strings.Join(syn.Debug, "\n"), tc.debug) {
				t.Fatalf("debug:\n%s", strings.Join(syn.Debug, "\n"))
			}
			if strings.Contains(strings.Join(syn.Debug, "\n"), "\nmore") || strings.Contains(strings.Join(syn.Debug, "\n"), "shim more") {
				t.Fatalf("the second line of the answer is in the debug lines:\n%s", strings.Join(syn.Debug, "\n"))
			}
		})
	}
	t.Run("a stop is the error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runner := versionRunner(t, func(procexec.Command) (procexec.Result, error) {
			cancel()
			return procexec.Result{}, context.Canceled
		})

		_, err := NewSynthesizer(Config{Runner: runner}).Synthesize(ctx, pythonApp(t), Options{}, nil)

		if !errors.Is(err, context.Canceled) || len(runner.recorded()) != 1 {
			t.Fatalf("error %v, %d calls", err, len(runner.recorded()))
		}
	})
	t.Run("the limit of the check can be set", func(t *testing.T) {
		runner := versionRunner(t, answer("Python 3.12.0\n", "", 0, nil))
		s := NewSynthesizer(Config{Runner: runner})
		s.VersionTimeout = 5 * time.Second

		if _, err := s.Synthesize(context.Background(), pythonApp(t), Options{}, nil); err != nil {
			t.Fatal(err)
		}
		if c := runner.recorded()[0]; c.Timeout != 5*time.Second {
			t.Fatalf("limit %v", c.Timeout)
		}
	})
}

// Security design §4, NFR6.17: the first word python, python3 or py of the app
// command is replaced by the Python of the .venv, quoted when a shell would
// read it otherwise, and left alone when quoting is not safe.
func TestPythonAppCommand(t *testing.T) {
	const unixPy = "/home/alice/app/.venv/bin/python"
	const winPy = `C:\Users\alice\app\.venv\Scripts\python.exe`
	for _, tc := range []struct {
		command, python string
		windows         bool
		want            string
		ok              bool
	}{
		{"python3 app.py", unixPy, false, unixPy + " app.py", true},
		{"  python3   app.py", unixPy, false, unixPy + "   app.py", true},
		{"python\tapp.py", unixPy, false, unixPy + "\tapp.py", true},
		{"PYTHON3 app.py", unixPy, false, unixPy + " app.py", true},
		{"python.exe app.py", winPy, true, winPy + " app.py", true},
		{"py app.py", winPy, true, winPy + " app.py", true},
		{"python3", unixPy, false, unixPy, true},
		{"python3 app.py", "/home/a b/.venv/bin/python", false, `"/home/a b/.venv/bin/python" app.py`, true},
		{"python3 app.py", "/home/a&b/.venv/bin/python", false, `"/home/a&b/.venv/bin/python" app.py`, true},
		{"python3 app.py", "/home/a(b)/.venv/bin/python", false, `"/home/a(b)/.venv/bin/python" app.py`, true},
		{"python3 app.py", "/home/a$b/.venv/bin/python", false, "", false},
		{"python3 app.py", `/home/a"b/.venv/bin/python`, false, "", false},
		{"python3 app.py", `/home/a\b/.venv/bin/python`, false, "", false},
		{"python app.py", `C:\Users\a b\.venv\Scripts\python.exe`, true, "", false},
		{"python app.py", `C:\Users\a&b\.venv\Scripts\python.exe`, true, "", false},
		{"python app.py", `C:\Users\a%b\.venv\Scripts\python.exe`, true, "", false},
		{".venv/bin/python app.py", unixPy, false, "", false},
		{"pipenv run python app.py", unixPy, false, "", false},
		{"", unixPy, false, "", false},
	} {
		got, ok := appCommand(tc.command, tc.python, tc.windows)
		if got != tc.want || ok != tc.ok {
			t.Errorf("appCommand(%q, %q, %v) = %q, %v; want %q, %v", tc.command, tc.python, tc.windows, got, ok, tc.want, tc.ok)
		}
	}
}

// NFR2.5, NFR2.1: the version check, the normal synth and the trace synth get
// the same environment: the .venv first on PATH, VIRTUAL_ENV, no AWS variable
// and no token; --app carries the Python of the .venv.
func TestPythonEnvironmentOfEachRun(t *testing.T) {
	app := pythonApp(t)
	python := venvPythonIn(t, app.Abs)
	venv := filepath.Join(app.Abs, ".venv")
	bin := filepath.Dir(python)
	parent := []string{"PATH=" + filepath.Join("usr", "bin"), "AWS_PROFILE=sandbox", "AWS_ACCESS_KEY_ID=x", "GH_TOKEN=y", "VIRTUAL_ENV=old"}
	if runtime.GOOS == "windows" {
		parent[0] = "Path=" + `C:\Windows`
	}
	runner := versionRunner(t, func(procexec.Command) (procexec.Result, error) {
		return procexec.Result{Stdout: []byte("Python 3.12.1\n")}, nil
	})
	s := NewSynthesizer(Config{Runner: runner, Environ: func() []string { return parent }})

	for _, trace := range []bool{false, true} {
		syn, err := s.Synthesize(context.Background(), app, Options{WithTrace: trace}, nil)
		if err != nil || syn.Failure != nil {
			t.Fatalf("trace %v: %v, %+v", trace, err, syn.Failure)
		}
	}

	calls := runner.recorded()
	if len(calls) != 4 || calls[0].Name != python || calls[2].Name != python {
		t.Fatalf("calls %+v", calls)
	}
	arg, ok := appCommand(app.Command, python, runtime.GOOS == "windows")
	for i, c := range calls {
		vars := map[string]string{}
		for _, kv := range c.Env {
			name, value, _ := strings.Cut(kv, "=")
			vars[strings.ToUpper(name)] = value
		}
		first, _, _ := strings.Cut(vars["PATH"], string(os.PathListSeparator))
		if first != bin || vars["VIRTUAL_ENV"] != venv {
			t.Errorf("call %d: PATH %q, VIRTUAL_ENV %q", i, vars["PATH"], vars["VIRTUAL_ENV"])
		}
		for _, name := range []string{"AWS_PROFILE", "AWS_ACCESS_KEY_ID", "GH_TOKEN"} {
			if _, found := vars[name]; found {
				t.Errorf("call %d has %s", i, name)
			}
		}
		if i%2 == 1 {
			hasApp := slices.Contains(c.Args, "--app")
			if hasApp != ok || ok && c.Args[slices.Index(c.Args, "--app")+1] != arg {
				t.Errorf("call %d: args %q, want --app %q (%v)", i, c.Args, arg, ok)
			}
			if trace := slices.Contains(c.Args, "--debug"); trace != (i == 3) {
				t.Errorf("call %d: args %q", i, c.Args)
			}
		}
	}
	if !slices.Equal(envNames(calls[0].Env), envNames(calls[1].Env)) || !slices.Equal(envNames(calls[1].Env), envNames(calls[3].Env)) {
		t.Fatalf("the environments differ:\n%v\n%v\n%v", envNames(calls[0].Env), envNames(calls[1].Env), envNames(calls[3].Env))
	}
}

func envNames(vars []string) []string {
	var names []string
	for _, kv := range vars {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	return names
}

// NFR6.16: the oldest Python is the lower bound of Requires-Python of the
// aws-cdk-lib installed in the .venv of py-minimal.
func TestMinPythonMatchesTheTestApp(t *testing.T) {
	app := testutil.CDKTestApp("py-minimal")
	if !testutil.RequirePythonTools(t, app) {
		t.FailNow()
	}
	var found []string
	for _, pattern := range []string{
		filepath.Join(app, ".venv", "lib", "python*", "site-packages", "aws_cdk_lib-*.dist-info", "METADATA"),
		filepath.Join(app, ".venv", "Lib", "site-packages", "aws_cdk_lib-*.dist-info", "METADATA"),
	} {
		m, _ := filepath.Glob(pattern)
		found = append(found, m...)
	}
	if len(found) != 1 {
		t.Fatalf("METADATA of aws-cdk-lib: %v", found)
	}
	f, err := os.Open(found[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	requires := regexp.MustCompile(`^Requires-Python:\s*>=\s*(\d+)\.(\d+)`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := requires.FindStringSubmatch(scanner.Text()); m != nil {
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
			if [2]int{major, minor} != minPython || MinPythonVersion() != m[1]+"."+m[2] {
				t.Fatalf("aws-cdk-lib needs Python %s.%s, minPython is %s", m[1], m[2], MinPythonVersion())
			}
			return
		}
	}
	t.Fatalf("no Requires-Python in %s", found[0])
}

// NFR6.18: every requirement of py-minimal is pinned with ==.
func TestPyMinimalPins(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.CDKTestApp("py-minimal"), "requirements.txt"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		n++
		if !strings.Contains(line, "==") || strings.ContainsAny(line, "<>~*") {
			t.Errorf("not pinned: %q", line)
		}
	}
	if n == 0 {
		t.Fatal("no requirement")
	}
}
