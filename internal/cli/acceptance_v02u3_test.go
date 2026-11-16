package cli_test

// Acceptance tests for v0.2.0 U3, CDK synth and mapping: every CDK app below
// --dir is synthesized in name order, Python apps too, --cdk-app picks apps,
// --synth-timeout bounds each synth, an app that fails does not stop the
// others, and the trace of synth finds the place that makes a resource when
// the sources alone do not. Like the earlier units, each test is an acceptance
// criterion written as Given/When/Then and run from the command entry point
// (cli.Run).
//
// These tests were written before the U3 implementation (team testing
// posture: acceptance criteria first).
//
// The tests marked "real synth" run cdk synth on copies of the test apps of
// internal/cdk/testdata/apps: they need Node.js and npm ci in ts-minimal (and
// the .venv of py-minimal for the Python app), fail without them in CI and are
// skipped elsewhere. The others stand in for cdk, node and python with
// programs (a procexec.Runner that answers as they would) or with the fakes of
// testutil.InstallFakeCDK, which are this test binary; git is the real one and
// gh the fake of testutil. AWS is never called: the plans are made in the
// tests.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The resource of the test app ts-trace as synth names it.
const (
	traceStack     = "tag-refactor-cdk-trace"
	traceLogicalID = "Logs6819BB44"
	traceARN       = "arn:aws:s3:::tag-refactor-cdk-trace-logs6819bb44-abcdefghijkl"
	traceTag       = "Tags.of(this).add('Owner', 'team-a');"
	traceTagAfter  = "Tags.of(this).add('Team', 'team-a');"
)

// renameKey is a change of a plan that renames from to to.
func renameKey(id, from, to, value string) map[string]any {
	return map[string]any{"changeId": id, "kind": "renameKey", "fromKey": from, "toKey": to, "fromValue": value,
		"mergesIntoExisting": false, "impacted": false}
}

// stackResource is a resource made by CloudFormation in a plan, with its tags
// and changes.
func stackResource(arn, stack, logicalID string, tags map[string]string, changes ...map[string]any) map[string]any {
	before := map[string]string{"aws:cloudformation:stack-name": stack, "aws:cloudformation:logical-id": logicalID}
	for k, v := range tags {
		before[k] = v
	}
	r := planResource(arn, before, changes)
	r["resourceType"] = "s3"
	return r
}

// traceResource is the bucket of ts-trace in a plan that renames Owner to Team.
func traceResource() map[string]any {
	return stackResource(traceARN, traceStack, traceLogicalID, map[string]string{"Owner": "team-a"},
		renameKey("c-000002", "Owner", "Team", "team-a"))
}

// strayResource is made by CloudFormation in a stack no app and no template
// has.
func strayResource() map[string]any {
	return stackResource("arn:aws:s3:::stray-bucket", "tag-refactor-stray", "Stray0001", map[string]string{"Env": "dev"},
		renameKey("c-000009", "Env", "Environment", "dev"))
}

// givenPyApp writes the files a Python CDK v2 app is told by, without a .venv.
func (w *iacWorld) givenPyApp(rel string) {
	w.givenFile(rel+"/cdk.json", `{"app": "python3 app.py"}`)
	w.givenFile(rel+"/requirements.txt", "aws-cdk-lib==2.276.0\nconstructs==10.8.1\n")
	w.givenFile(rel+"/app.py", "# the app\n")
}

// givenTestApp copies the test app name to rel; an app without node_modules
// gets the node_modules of ts-minimal (security design §10).
func (w *iacWorld) givenTestApp(name, rel string) string {
	w.t.Helper()
	src := testutil.CDKTestApp(name)
	dst := filepath.Join(w.dir, filepath.FromSlash(rel))
	testutil.CopyCDKApp(w.t, src, dst)
	if _, err := os.Stat(filepath.Join(src, "node_modules")); err != nil {
		testutil.LinkNodeModules(w.t, testutil.CDKTestApp("ts-minimal"), dst)
	}
	w.givenFile(".gitignore", "node_modules\ncdk.out\n.venv\n__pycache__\n")
	return dst
}

// givenLink makes link a link to the folder target: a symbolic link on Unix, a
// junction on Windows.
func givenLink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		return
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

// rel returns a folder of the world relative to it, with /.
func (w *iacWorld) rel(dir string) string {
	root, err := filepath.EvalSymlinks(w.dir)
	if err != nil {
		root = w.dir
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return dir
	}
	return filepath.ToSlash(rel)
}

// iacWith runs iac with the stand-in p (nil runs the programs of the
// computer), in the context ctx.
func (w *iacWorld) iacWith(ctx context.Context, p *programs, plan string, args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC), 10*time.Millisecond)
	deps := cli.Deps{Transport: w.fake, Now: clock.Now, RetryDelay: func(int) time.Duration { return 0 }}
	if p != nil {
		deps.Exec = p
	}
	code := cli.Run(ctx, append([]string{"iac", "--plan", plan, "--dir", w.dir}, args...), &stdout, &stderr, deps)
	if n := len(w.fake.Calls()); n != 0 {
		w.t.Fatalf("iac called AWS %d times: %v", n, w.fake.Ops())
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// programs stands in for cdk, node and python (procexec.Runner): it keeps
// every call and answers with answer, or as a working program would. git and
// gh run for real (gh is the fake of testutil when the test installed it).
type programs struct {
	mu    sync.Mutex
	calls []procexec.Command
	// missing are the programs LookPath does not find.
	missing map[string]bool
	// answer answers a call; errUnanswered leaves it to the working program.
	answer func(ctx context.Context, c procexec.Command) (procexec.Result, error)
}

// errUnanswered says that answer leaves a call to the working program.
var errUnanswered = errors.New("unanswered")

func realProgram(name string) bool {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".exe")
	return base == "git" || base == "gh"
}

func (p *programs) Run(ctx context.Context, c procexec.Command) (procexec.Result, error) {
	if realProgram(c.Name) {
		return procexec.System{}.Run(ctx, c)
	}
	p.mu.Lock()
	p.calls = append(p.calls, c)
	p.mu.Unlock()
	if p.answer != nil {
		if res, err := p.answer(ctx, c); !errors.Is(err, errUnanswered) {
			return res, err
		}
	}
	return workingProgram(c)
}

func (p *programs) LookPath(name string) (string, error) {
	if realProgram(name) {
		return procexec.System{}.LookPath(name)
	}
	if p.missing[name] {
		return "", exec.ErrNotFound
	}
	return filepath.Join("fake-bin", name), nil
}

func (p *programs) recorded() []procexec.Command {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.calls)
}

// synthDirs lists the folders of the synth calls, relative to the world.
func (p *programs) synthDirs(w *iacWorld) []string {
	var dirs []string
	for _, c := range p.recorded() {
		if isSynth(c) {
			dirs = append(dirs, w.rel(c.Dir))
		}
	}
	return dirs
}

func baseName(c procexec.Command) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(filepath.Base(c.Name)), ".exe"), ".cmd")
}

func isPythonVersion(c procexec.Command) bool {
	b := baseName(c)
	return (strings.HasPrefix(b, "python") || b == "py") && len(c.Args) == 1 && c.Args[0] == "--version"
}

func isHelper(c procexec.Command) bool {
	return baseName(c) == "node" && len(c.Args) == 1 && c.Args[0] != "--version"
}

func hasArg(c procexec.Command, arg string) bool { return slices.Contains(c.Args, arg) }

// outputOf returns the folder of --output of a synth call.
func outputOf(c procexec.Command) string {
	for i := 0; i+1 < len(c.Args); i++ {
		if c.Args[i] == "--output" {
			return c.Args[i+1]
		}
	}
	return ""
}

// emptyAnalysis is the answer of a helper that found nothing.
const emptyAnalysis = `{"version":1,"tagSites":[],"constructSites":[],"insertSites":[],"unsupported":[],"typescriptVersion":"5.6.3"}`

// workingProgram answers as cdk, node and python that work: synth writes a
// cloud assembly without stacks, the helper finds nothing.
func workingProgram(c procexec.Command) (procexec.Result, error) {
	switch {
	case isSynth(c):
		return procexec.Result{}, writeAssembly(outputOf(c), false)
	case isPythonVersion(c):
		return procexec.Result{Stdout: []byte("Python 3.12.0\n")}, nil
	case baseName(c) == "node" && hasArg(c, "--version"):
		return procexec.Result{Stdout: []byte("v22.0.0\n")}, nil
	case isHelper(c):
		return procexec.Result{Stdout: []byte(emptyAnalysis)}, nil
	}
	return procexec.Result{}, nil
}

// fakeStack is a stack of a cloud assembly the stand-in synth writes.
type fakeStack struct {
	name      string
	resources []fakeResource
	// trace gives, by construct path, the lines of aws:cdk:creationStack.
	trace map[string][]string
}

type fakeResource struct {
	logicalID, path string
	tags            map[string]string
}

// writeAssembly writes a cloud assembly into dir, as cdk synth does: the
// manifest, the templates and, with withTrace, the metadata files of the
// stacks with the trace of their constructs.
func writeAssembly(dir string, withTrace bool, stacks ...fakeStack) error {
	if dir == "" {
		return errors.New("synth got no --output")
	}
	artifacts := map[string]any{}
	for _, st := range stacks {
		resources := map[string]any{}
		for _, r := range st.resources {
			var tags []map[string]string
			for k, v := range r.tags {
				tags = append(tags, map[string]string{"Key": k, "Value": v})
			}
			resources[r.logicalID] = map[string]any{"Type": "AWS::S3::Bucket", "Properties": map[string]any{"Tags": tags},
				"Metadata": map[string]any{"aws:cdk:path": r.path}}
		}
		if err := writeJSON(filepath.Join(dir, st.name+".template.json"), map[string]any{"Resources": resources}); err != nil {
			return err
		}
		artifact := map[string]any{"type": "aws:cloudformation:stack", "environment": "aws://unknown-account/unknown-region",
			"properties": map[string]any{"templateFile": st.name + ".template.json", "stackName": st.name}}
		if withTrace {
			metadata := map[string]any{}
			for p, lines := range st.trace {
				metadata["/"+p] = []any{map[string]any{"type": "aws:cdk:creationStack", "data": lines}}
			}
			artifact["additionalMetadataFile"] = st.name + ".metadata.json"
			if err := writeJSON(filepath.Join(dir, st.name+".metadata.json"), metadata); err != nil {
				return err
			}
		}
		artifacts[st.name] = artifact
	}
	return writeJSON(filepath.Join(dir, "manifest.json"), map[string]any{"version": "36.0.0", "artifacts": artifacts})
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// AC2.1.1 (US2.1, BR1.1, BR1.4, NFR3.15): Given a TypeScript and a Python app
// below --dir and cdk.json files inside node_modules, .venv and venv, When iac
// runs, Then the two apps are synthesized one after the other in name order
// and nothing inside those folders is; and with --cdk-app naming one app, the
// other is neither synthesized nor shown.
func TestV02U3EveryAppInNameOrder(t *testing.T) {
	w := newIacWorld(t)
	w.givenAppSources("infra/ts")
	w.givenPyApp("infra/py")
	for _, rel := range []string{"node_modules/pkg", ".venv/pkg", "venv/pkg", "infra/ts/node_modules/x", "infra/py/.venv/y"} {
		w.givenAppSources(rel)
	}
	plan := w.givenPlan(strayResource())

	p := &programs{}
	r := w.iacWith(context.Background(), p, plan, "--lang", "en")

	r.requireExit(t, 2)
	if got := p.synthDirs(w); !slices.Equal(got, []string{"infra/py", "infra/ts"}) {
		t.Fatalf("synth ran in %v, want infra/py then infra/ts", got)
	}
	requireContains(t, "stderr", r.stderr, "CDK apps to synthesize: infra/py, infra/ts (waiting)\n")
	py := strings.Index(r.stderr, "infra/py: synthesizing (Python)\n")
	ts := strings.Index(r.stderr, "infra/ts: synthesizing (TypeScript)\n")
	if py < 0 || ts < py {
		t.Fatalf("the apps did not start in name order:\n%s", r.stderr)
	}
	for _, folder := range []string{"node_modules", ".venv", "venv/"} {
		requireNotContains(t, "stderr", r.stderr, folder)
	}

	only := &programs{}
	r = w.iacWith(context.Background(), only, plan, "--lang", "en", "--cdk-app", "infra/ts")

	r.requireExit(t, 2)
	if got := only.synthDirs(w); !slices.Equal(got, []string{"infra/ts"}) {
		t.Fatalf("with --cdk-app infra/ts synth ran in %v", got)
	}
	requireNotContains(t, "stderr", r.stderr, "infra/py")
}

// AC2.1.1, input errors (BR1.2, NFR2.7): Given --cdk-app pointing outside
// --dir, into folders discovery does not enter, through a link, at an
// absolute path or at a folder without an app, When iac runs, Then it stops
// with exit code 1 before any synth and changes no source; also when the plan
// has no resource made by CloudFormation. On Windows, Infra/TS is infra/ts.
func TestV02U3CdkAppInputErrors(t *testing.T) {
	w := newIacWorld(t)
	w.givenAppSources("infra/ts")
	w.givenAppSources("node_modules/x")
	w.givenAppSources(".venv/x")
	givenLink(t, filepath.Join(w.dir, "infra", "ts"), filepath.Join(w.dir, "link"))
	source := filepath.Join(w.dir, "infra", "ts", "bin", "app.ts")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	plan := w.givenPlan(strayResource())
	values := []string{"../other", "a/../../other", "node_modules/x", ".venv/x", "link", "/etc", `C:\x`, "C:", "infra/none", ""}
	if runtime.GOOS == "windows" {
		values = append(values, `..\other`)
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			p := &programs{}

			r := w.iacWith(context.Background(), p, plan, "--lang", "en", "--cdk-app", value)

			r.requireExit(t, 1)
			if calls := p.recorded(); len(calls) != 0 {
				t.Fatalf("programs ran: %v", calls)
			}
			// The value is sanitized: a backslash is written twice.
			requireContains(t, "stderr", r.stderr, "--cdk-app "+display.Sanitize(value))
			requireContains(t, "stderr", r.stderr, "give the folder of a cdk.json relative to --dir")
		})
	}
	t.Run("a plan without CloudFormation resources", func(t *testing.T) {
		other := w.givenPlan(planResource(arnOther, map[string]string{"env": "Prod"}, []map[string]any{
			{"changeId": "c-000001", "kind": "changeValue", "fromKey": "env", "fromValue": "Prod", "toValue": "prod", "impacted": false},
		}))
		p := &programs{}

		r := w.iacWith(context.Background(), p, other, "--lang", "en", "--cdk-app", "infra/none")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "--cdk-app infra/none is not a CDK app found below --dir")
		if calls := p.recorded(); len(calls) != 0 {
			t.Fatalf("programs ran: %v", calls)
		}
	})
	if after, err := os.ReadFile(source); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("the source changed: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Run("Infra/TS", func(t *testing.T) {
			p := &programs{}

			r := w.iacWith(context.Background(), p, plan, "--lang", "en", "--cdk-app", "Infra/TS")

			r.requireExit(t, 2)
			if got := p.synthDirs(w); !slices.Equal(got, []string{"infra/ts"}) {
				t.Fatalf("synth ran in %v, want infra/ts", got)
			}
		})
	}
}

// missingPython stands in for a computer without python3.
func missingPython() *programs { return &programs{missing: map[string]bool{"python3": true}} }

// AC2.1.2 (US2.1, BR5.2): Given two apps, one of which cannot be synthesized,
// When iac runs and standard error is not a terminal, Then each app gets a
// line when it waits, when it starts, and when it is done with the time or
// failed with the kind of reason, in name order, in English and in Japanese.
func TestV02U3ProgressLinesWhenNotATerminal(t *testing.T) {
	w := newIacWorld(t)
	w.givenPyApp("infra/py")
	w.givenAppSources("infra/ts")
	plan := w.givenPlan(strayResource())
	for _, tc := range []struct {
		lang  string
		lines []string
	}{
		{"en", []string{"CDK apps to synthesize: infra/py, infra/ts (waiting)", "infra/py: synthesizing (Python)",
			"infra/py: failed (0s): a program is missing", "infra/ts: synthesizing (TypeScript)", "infra/ts: done (0s)"}},
		{"ja", []string{"synth する CDK のアプリ: infra/py, infra/ts(待ち)", "infra/py: synth しています(Python)",
			"infra/py: 失敗(0 秒): プログラムが見つかりません", "infra/ts: synth しています(TypeScript)", "infra/ts: 済み(0 秒)"}},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			r := w.iacWith(context.Background(), missingPython(), plan, "--lang", tc.lang)

			r.requireExit(t, 2)
			at := 0
			for _, line := range tc.lines {
				i := strings.Index(r.stderr[at:], line+"\n")
				if i < 0 {
					t.Fatalf("no %q after the lines before it:\n%s", line, r.stderr)
				}
				at += i + len(line)
			}
		})
	}
}

// AC2.1.3 (US2.1, BR2.3, BR2.6, NFR8.4): Given AWS variables in the
// environment and two apps that need AWS to synth (a lookup with no
// cdk.context.json, and a profile in cdk.json), When iac runs, Then both fail
// as needing AWS with how to keep the looked up values in cdk.context.json,
// and the exit code is 2. (real synth)
func TestV02U3AppsThatNeedAWSFail(t *testing.T) {
	testApp(t)
	w := newIacWorld(t)
	w.givenTestApp("ts-lookup", "infra/lookup")
	w.givenTestApp("ts-profile", "infra/profile")
	t.Setenv("AWS_ACCESS_KEY_ID", testutil.FakeAccessKeyID)
	t.Setenv("AWS_PROFILE", "tag-refactor-test-profile")
	plan := w.givenPlan(strayResource())

	r := w.iacWith(context.Background(), nil, plan, "--lang", "en")

	r.requireExit(t, 2)
	for _, app := range []string{"infra/lookup", "infra/profile"} {
		requireMatch(t, "stderr", r.stderr, `(?m)^`+app+`: failed \(\d+s\): AWS credentials needed$`)
	}
	if n := strings.Count(r.stderr, "cdk.context.json"); n < 2 {
		t.Errorf("the way out is shown %d times, want 2:\n%s", n, r.stderr)
	}
	requireContains(t, "stdout", r.stdout, "the CDK app could not be synthesized or read")

	t.Run("synth gets no AWS variable", func(t *testing.T) {
		p := &programs{answer: func(_ context.Context, c procexec.Command) (procexec.Result, error) {
			if !isSynth(c) {
				return procexec.Result{}, errUnanswered
			}
			return procexec.Result{ExitCode: 1, Stderr: []byte("[Error at /tag-refactor-cdk-lookup] Need to perform AWS calls for account 000000000000, but no credentials have been configured\n")}, nil
		}}

		r := w.iacWith(context.Background(), p, plan, "--lang", "en")

		r.requireExit(t, 2)
		for _, c := range p.recorded() {
			for _, kv := range c.Env {
				name, _, _ := strings.Cut(kv, "=")
				upper := strings.ToUpper(name)
				if strings.HasPrefix(upper, "AWS_") && upper != "AWS_SHARED_CREDENTIALS_FILE" && upper != "AWS_CONFIG_FILE" &&
					upper != "AWS_EC2_METADATA_DISABLED" {
					t.Errorf("%s got %s", c.Name, name)
				}
			}
		}
	})
}

// AC2.1.4 (US2.1, NFR3.11, NFR3.16): Given a cdk that answers nothing and
// starts a grandchild, When iac runs with --synth-timeout 1s, Then synth stops
// after about one second, the app fails as not done in time, and no
// grandchild is left. A --synth-timeout that is not a duration of 1s or more
// stops iac with exit code 1 before any program runs.
func TestV02U3SynthTimeout(t *testing.T) {
	w := newIacWorld(t)
	w.givenAppSources("infra")
	plan := w.givenPlan(strayResource())
	fake := testutil.InstallFakeCDK(t, testutil.FakeCDKSettings{Hang: true})
	start := time.Now()

	r := w.iacWith(context.Background(), nil, plan, "--lang", "en", "--synth-timeout", "1s")

	if took := time.Since(start); took > 15*time.Second {
		t.Errorf("iac took %v with --synth-timeout 1s", took)
	}
	r.requireExit(t, 2)
	requireMatch(t, "stderr", r.stderr, `(?m)^infra: failed \(\d+s\): it did not end in time$`)
	pid, ok := fake.Grandchild(5 * time.Second)
	if !ok {
		t.Fatal("the grandchild of cdk did not start")
	}
	deadline := time.Now().Add(10 * time.Second)
	for testutil.ProcessAlive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("the grandchild %d of cdk is still running", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}

	for _, value := range []string{"0s", "999ms", "10", "abc", "-5s"} {
		t.Run(value, func(t *testing.T) {
			p := &programs{}

			r := w.iacWith(context.Background(), p, plan, "--lang", "en", "--synth-timeout="+value)

			r.requireExit(t, 1)
			requireContains(t, "stderr", r.stderr, "--synth-timeout "+value+" is not a duration of 1s or more")
			if calls := p.recorded(); len(calls) != 0 {
				t.Fatalf("programs ran: %v", calls)
			}
		})
	}
}

// givenTempDir points the temporary folder of the process at an empty folder
// of the test and returns it.
func givenTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tmp")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(name, dir)
	}
	return dir
}

func requireEmptyFolder(t *testing.T, dir string) {
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
		t.Fatalf("left in the temporary folder: %v", names)
	}
}

// AC2.1.5 (US2.1, BR2.5, NFR4.5): Given a marker in the cdk.out of the app,
// When synth succeeds, fails, runs out of time or is interrupted, Then cdk.out
// is unchanged and nothing is left in the temporary folder; an interruption
// rewrites no file and ends with exit code 1.
func TestV02U3NothingLeftBehind(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(cancel context.CancelFunc, ctx context.Context, c procexec.Command) (procexec.Result, error)
		code int
	}{
		{"success", nil, 2},
		{"failure", func(context.CancelFunc, context.Context, procexec.Command) (procexec.Result, error) {
			return procexec.Result{ExitCode: 1, Stderr: []byte("Error: boom\n")}, nil
		}, 2},
		{"timeout", func(context.CancelFunc, context.Context, procexec.Command) (procexec.Result, error) {
			return procexec.Result{}, procexec.ErrTimeout
		}, 2},
		{"interrupted", func(cancel context.CancelFunc, ctx context.Context, _ procexec.Command) (procexec.Result, error) {
			cancel()
			return procexec.Result{}, ctx.Err()
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newIacWorld(t)
			w.givenAppSources("infra")
			marker := w.givenFile("infra/cdk.out/marker.txt", "keep me\n")
			source := filepath.Join(w.dir, "infra", "bin", "app.ts")
			plan := w.givenPlan(strayResource())
			tmp := givenTempDir(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := &programs{answer: func(ctx context.Context, c procexec.Command) (procexec.Result, error) {
				if !isSynth(c) || tc.run == nil {
					return procexec.Result{}, errUnanswered
				}
				return tc.run(cancel, ctx, c)
			}}

			r := w.iacWith(ctx, p, plan, "--lang", "en")

			r.requireExit(t, tc.code)
			requireEmptyFolder(t, tmp)
			if data, err := os.ReadFile(marker); err != nil || string(data) != "keep me\n" {
				t.Fatalf("the marker in cdk.out changed: %q, %v", data, err)
			}
			if entries, _ := os.ReadDir(filepath.Dir(marker)); len(entries) != 1 {
				t.Fatalf("cdk.out holds %d entries", len(entries))
			}
			if data, _ := os.ReadFile(source); string(data) != "// the app\n" {
				t.Fatalf("the source changed: %q", data)
			}
			if tc.name == "interrupted" {
				requireContains(t, "stderr", r.stderr, "Exit code: 1 (interrupted)")
			}
		})
	}
}

// AC2.1.6 (US2.1, BR3.1): Given two apps, When the plan renames a key of a
// resource of each, Then both are reflected; and given two copies of one app
// with the same stack name and logical ID, the resource is not rewritten but
// listed to fix by hand as more than one place. (real synth)
func TestV02U3ResourcesOfEveryAppAreMatched(t *testing.T) {
	app := testApp(t)
	t.Run("two apps", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenApp(app, "infra/a")
		w.givenTestApp("ts-trace", "infra/b")
		plan := w.givenPlan(skeletonResource(), traceResource())

		r := w.iacWith(context.Background(), nil, plan, "--lang", "en")

		r.requireExit(t, 0)
		requireContains(t, "stdout", r.stdout, "infra/a/lib/stack.ts: 1 change can be reflected\n")
		requireContains(t, "stdout", r.stdout, "infra/b/lib/stack.ts: 1 change can be reflected\n")
	})
	t.Run("the same stack in two apps", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenApp(app, "infra/a")
		w.givenApp(app, "infra/b")
		plan := w.givenPlan(skeletonResource())

		r := w.iacWith(context.Background(), nil, plan, "--lang", "en")

		r.requireExit(t, 2)
		requireContains(t, "stdout", r.stdout, "more than one place of the source could be it")
		requireNotContains(t, "stdout", r.stdout, "+++ b/")
	})
}

// AC2.1.6, AC4.1.4 (BR4.1 to BR4.4, NFR3.13): Given an app that makes its
// bucket in a function the helper does not follow, When the plan renames the
// key the stack tags it with, Then iac finds the place in the trace of a
// second synth, writes the rename in the tag of the stack, and lists nothing
// to fix by hand. (real synth)
func TestV02U3TraceFindsTheMakerInAFunction(t *testing.T) {
	testApp(t)
	w := newIacWorld(t)
	w.givenTestApp("ts-trace", "infra")
	w.givenCommitted()
	source, err := os.ReadFile(filepath.Join(testutil.CDKTestApp("ts-trace"), "lib", "stack.ts"))
	if err != nil {
		t.Fatal(err)
	}
	plan := w.givenPlan(traceResource())

	r := w.iacWith(context.Background(), nil, plan, "--lang", "en", "--write", "--debug")

	r.requireExit(t, 0)
	w.requireFile("infra/lib/stack.ts", strings.Replace(string(source), traceTag, traceTagAfter, 1))
	requireContains(t, "stdout", r.stdout, "infra/lib/stack.ts: 1 change reflected\n")
	requireContains(t, "stderr", r.stderr, "debug: cdk: "+traceStack+"/"+traceLogicalID+" is made at infra/lib/stack.ts:8 ")
	for _, reason := range []string{"was not found in the source", "is not handled yet"} {
		requireNotContains(t, "stdout", r.stdout, reason)
	}
}

// AC2.1.7 (US2.1, NFR8.3): Given the .venv of the Python test app missing,
// When a Python synth test starts, Then it fails when CI is set and is skipped
// with the reason, which points at the section Development of README.md,
// otherwise; with everything there it goes on.
func TestV02U3MissingPythonToolsFailInCIAndSkipLocally(t *testing.T) {
	empty := t.TempDir()
	for _, tc := range []struct {
		name, ci string
		fails    bool
	}{
		{"in CI", "true", true},
		{"locally", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			look := testutil.ToolLookup{LookPath: func(string) (string, error) { return "node", nil }, Stat: os.Stat,
				Getenv: func(name string) string {
					if name == "CI" {
						return tc.ci
					}
					return ""
				}}
			rec := &recordingTB{}

			ok := testutil.RequirePythonToolsWith(rec, empty, look)

			if ok || (rec.fatal != "") != tc.fails || (rec.skip != "") == tc.fails {
				t.Fatalf("ok %v, fatal %q, skip %q", ok, rec.fatal, rec.skip)
			}
		})
	}
	t.Run("everything there", func(t *testing.T) {
		dir := t.TempDir()
		cdkName := "cdk"
		site := filepath.Join(dir, ".venv", "lib", "python3.13", "site-packages")
		if runtime.GOOS == "windows" {
			cdkName = "cdk.cmd"
			site = filepath.Join(dir, ".venv", "Lib", "site-packages")
		}
		for _, folder := range []string{filepath.Join(dir, "node_modules", ".bin"), filepath.Join(site, "aws_cdk_lib-2.276.0.dist-info"),
			filepath.Dir(testutil.VenvPython(dir))} {
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, file := range []string{filepath.Join(dir, "node_modules", ".bin", cdkName), testutil.VenvPython(dir)} {
			if err := os.WriteFile(file, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		look := testutil.ToolLookup{LookPath: func(string) (string, error) { return "node", nil }, Stat: os.Stat,
			Getenv: func(string) string { return "true" }}
		rec := &recordingTB{}

		if !testutil.RequirePythonToolsWith(rec, dir, look) || rec.fatal != "" || rec.skip != "" {
			t.Fatalf("fatal %q, skip %q", rec.fatal, rec.skip)
		}
	})
	t.Run("the reason", func(t *testing.T) {
		look := testutil.ToolLookup{LookPath: func(string) (string, error) { return "node", nil }, Stat: os.Stat,
			Getenv: func(string) string { return "" }}
		rec := &reasonTB{}

		testutil.RequirePythonToolsWith(rec, empty, look)

		requireContains(t, "reason", rec.text, "README.md")
		requireContains(t, "reason", rec.text, ".venv")
	})
}

// pathWithoutPython is a PATH with the folder of node and, outside Windows, a
// folder with only sh: npx (of the app command npx ts-node) starts its command
// with the sh of PATH there, and the folder of sh (/usr/bin) has python3.
func pathWithoutPython(t *testing.T, node string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return filepath.Dir(node)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("sh is not on PATH: %v", err)
	}
	bin := t.TempDir()
	if err := os.Symlink(sh, filepath.Join(bin, "sh")); err != nil {
		t.Fatalf("link sh: %v", err)
	}
	return filepath.Dir(node) + string(os.PathListSeparator) + bin
}

// reasonTB keeps the text of the skip or the failure.
type reasonTB struct{ text string }

func (r *reasonTB) Helper()                           {}
func (r *reasonTB) Fatalf(format string, args ...any) { r.text = fmt.Sprintf(format, args...) }
func (r *reasonTB) Skipf(format string, args ...any)  { r.text = fmt.Sprintf(format, args...) }

// venvPythonRel is where the Python of a .venv is, relative to the app, with /.
func venvPythonRel() string {
	if runtime.GOOS == "windows" {
		return ".venv/Scripts/python.exe"
	}
	return ".venv/bin/python"
}

// AC4.1.3 (US4.1, BR2.1, BR2.7, BR3.2): Given a Python app on a computer
// without Python and a TypeScript app, When iac runs, Then the Python app
// fails with the missing tool and how to install it, the change of the
// TypeScript app is reflected, a resource of no app and no template is listed
// as an app that failed with python, and the exit code is 2. A Python of the
// .venv that is too old fails the app with the version to install. (real
// synth)
func TestV02U3MissingAndOldPython(t *testing.T) {
	t.Run("no Python", func(t *testing.T) {
		app := testApp(t)
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node is not installed")
		}
		w := newIacWorld(t)
		w.givenPyApp("infra/py")
		// The app has its cdk, as py-minimal has: only Python is missing.
		testutil.LinkNodeModules(t, app, filepath.Join(w.dir, "infra", "py"))
		w.givenApp(app, "infra/ts")
		t.Setenv("PATH", pathWithoutPython(t, node))
		if _, err := exec.LookPath("python3"); err == nil {
			t.Skip("python3 is in the folder of node")
		}
		plan := w.givenPlan(skeletonResource(), strayResource())

		r := w.iacWith(context.Background(), nil, plan, "--lang", "en")

		r.requireExit(t, 2)
		requireContains(t, "stderr", r.stderr, "infra/py: failed (0s): a program is missing\n")
		requireContains(t, "stderr", r.stderr, "python was not found")
		requireContains(t, "stderr", r.stderr, "install Python 3")
		requireMatch(t, "stderr", r.stderr, `(?m)^infra/ts: done \(\d+s\)$`)
		requireContains(t, "stdout", r.stdout, "infra/ts/lib/stack.ts: 1 change can be reflected\n")
		requireContains(t, "stdout", r.stdout, "the CDK app could not be synthesized or read (python)")
	})
	t.Run("an old Python", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenPyApp("infra/py")
		w.givenFile("infra/py/"+venvPythonRel(), "")
		plan := w.givenPlan(strayResource())
		p := &programs{answer: func(_ context.Context, c procexec.Command) (procexec.Result, error) {
			if isPythonVersion(c) {
				return procexec.Result{Stdout: []byte("Python 3.8.18\n")}, nil
			}
			return procexec.Result{}, errUnanswered
		}}

		r := w.iacWith(context.Background(), p, plan, "--lang", "en")

		r.requireExit(t, 2)
		requireContains(t, "stderr", r.stderr, "infra/py: failed (0s): the program is too old\n")
		requireContains(t, "stderr", r.stderr, "install Python 3.10 or later")
		if dirs := p.synthDirs(w); len(dirs) != 0 {
			t.Fatalf("synth ran with an old Python: %v", dirs)
		}
		requireContains(t, "stdout", r.stdout, "the CDK app could not be synthesized or read (python)")
	})
}

// AC4.1.4 (US4.1, BR1.3, BR3.2, BR3.3): Given apps of CDK v1, of a version not
// known and of another language, When iac runs, Then none is synthesized,
// each is skipped, and a resource of no app is listed as CDK v1 (no app
// failed), or as another language when that is the only app; a change of a
// resource of a Python app is not rewritten but listed as not handled yet,
// and the app is done.
func TestV02U3SkippedAppsAndPythonChanges(t *testing.T) {
	t.Run("skipped apps", func(t *testing.T) {
		w := newIacWorld(t)
		for _, name := range []string{"ts-v1", "py-v1", "unknown-version", "other-lang"} {
			testutil.CopyCDKApp(t, testutil.CDKTestApp(name), filepath.Join(w.dir, "infra", name))
		}
		plan := w.givenPlan(strayResource())
		p := &programs{}

		r := w.iacWith(context.Background(), p, plan, "--lang", "en")

		r.requireExit(t, 2)
		if dirs := p.synthDirs(w); len(dirs) != 0 {
			t.Fatalf("synth ran in %v", dirs)
		}
		for _, line := range []string{"infra/ts-v1: skipped: CDK v1", "infra/py-v1: skipped: CDK v1",
			"infra/unknown-version: skipped: the version of CDK is not known", "infra/other-lang: skipped: neither TypeScript nor Python"} {
			requireContains(t, "stderr", r.stderr, line+"\n")
		}
		requireContains(t, "stdout", r.stdout, "the CDK app is CDK v1 or of a version not known")
	})
	t.Run("another language only", func(t *testing.T) {
		w := newIacWorld(t)
		testutil.CopyCDKApp(t, testutil.CDKTestApp("other-lang"), filepath.Join(w.dir, "infra"))
		plan := w.givenPlan(strayResource())

		r := w.iacWith(context.Background(), &programs{}, plan, "--lang", "en")

		r.requireExit(t, 2)
		requireContains(t, "stdout", r.stdout, "the CDK app is neither TypeScript nor Python")
	})
	t.Run("a change of a Python app", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenPyApp("infra/py")
		const stack = "tag-refactor-cdk-py-minimal"
		plan := w.givenPlan(stackResource("arn:aws:s3:::py-bucket", stack, "Bucket83908E77", map[string]string{"Env": "dev"},
			renameKey("c-000001", "Env", "Environment", "dev")))
		p := &programs{answer: func(_ context.Context, c procexec.Command) (procexec.Result, error) {
			if !isSynth(c) {
				return procexec.Result{}, errUnanswered
			}
			return procexec.Result{}, writeAssembly(outputOf(c), false, fakeStack{name: stack, resources: []fakeResource{
				{logicalID: "Bucket83908E77", path: stack + "/Bucket/Resource", tags: map[string]string{"Env": "dev"}}}})
		}}

		r := w.iacWith(context.Background(), p, plan, "--lang", "en")

		r.requireExit(t, 2)
		requireContains(t, "stderr", r.stderr, "infra/py: done (0s)\n")
		requireContains(t, "stdout", r.stdout, "this way of writing the tag, or this kind of change, is not handled yet")
		for _, c := range p.recorded() {
			if isHelper(c) {
				t.Fatalf("the helper read the Python app: %v", c.Args)
			}
		}
	})
}

// The secrets the stand-in cdk prints, made in the test (project rule: no
// token in the files of the repository).
var (
	u3Token   = "gh" + "p_" + strings.Repeat("a1", 18)
	u3ARN     = "arn:aws:s3:::secret-bucket"
	u3Account = "123456789012"
)

// homeOfAlice is a home folder of the form of the system the test runs on.
func homeOfAlice() string {
	if runtime.GOOS == "windows" {
		return `C:\Users\alice`
	}
	return "/home/alice"
}

// homeSource is the source of the app of the home test.
const homeSource = "import { Stack, Tags } from 'aws-cdk-lib';\nimport * as s3 from 'aws-cdk-lib/aws-s3';\n\n" +
	"export function make(stack: Stack) {\n  const bucket = new s3.Bucket(stack, 'Bucket');\n  Tags.of(bucket).add('Env', 'dev');\n}\n"

const homeStack = "tag-refactor-u3-home"

// homePrograms stands in for the programs of the home test: the app infra/ts
// is made in a function (the helper finds its place in the trace only), and
// the app infra/bad fails with secrets and home folders in its standard error.
func homePrograms(t *testing.T) *programs {
	keyAt := strings.Index(homeSource, "'Env'")
	valueAt := strings.Index(homeSource, "'dev'")
	site := func(key string) map[string]any {
		return map[string]any{"useFile": "lib/stack.ts", "useLine": 6, "op": "add", "scopeConstructPath": homeStack + "/Bucket",
			"keyEdit":    map[string]any{"file": "lib/stack.ts", "start": keyAt, "end": keyAt + len(key) + 2, "shape": "literal", "text": key, "sharedWith": []string{}},
			"valueEdit":  map[string]any{"file": "lib/stack.ts", "start": valueAt, "end": valueAt + 5, "shape": "literal", "text": "dev", "sharedWith": []string{}},
			"removeEdit": map[string]any{"file": "", "start": 0, "end": 0, "shape": "", "text": "", "sharedWith": []string{}}}
	}
	return &programs{answer: func(_ context.Context, c procexec.Command) (procexec.Result, error) {
		switch {
		case isSynth(c) && strings.HasSuffix(filepath.ToSlash(c.Dir), "/bad"):
			alice := homeOfAlice()
			return procexec.Result{ExitCode: 1, Stderr: []byte("Error: cannot read " + u3ARN + " of " + u3Account + " with " + u3Token + "\n" +
				"    at Object.<anonymous> (/home/alice/work/app/bin/app.ts:3:7)\n" +
				"    at Object.<anonymous> (" + filepath.Join(alice, "work", "app", "bin", "app.ts") + ":3:7)\n")}, nil
		case isSynth(c):
			return procexec.Result{}, writeAssembly(outputOf(c), hasArg(c, "--debug"), fakeStack{name: homeStack,
				resources: []fakeResource{{logicalID: "Bucket83908E77", path: homeStack + "/Bucket/Resource", tags: map[string]string{"Env": "dev"}}},
				trace: map[string][]string{homeStack + "/Bucket/Resource": {
					"...new Bucket2 in aws-cdk-lib...",
					"Object.<anonymous> (/home/alice/work/other/lib/x.ts:1:1)",
					"Object.<anonymous> (C:\\Users\\alice\\work\\other\\lib\\x.ts:2:2)",
					"make (" + filepath.Join(c.Dir, "lib", "stack.ts") + ":5:18)",
					"(no user code in 10 frames, " + u3ARN + " " + u3Account + ")",
				}}})
		case isHelper(c):
			var req struct {
				Mode       string `json:"mode"`
				TraceSites []struct {
					ConstructPath, File string
					Line                int
				} `json:"traceSites"`
			}
			if err := json.Unmarshal(c.Stdin, &req); err != nil {
				t.Errorf("the helper got %v", err)
			}
			analysis := map[string]any{"version": 1, "constructSites": []any{}, "insertSites": []any{}, "unsupported": []any{},
				"typescriptVersion": "5.6.3", "tagSites": []any{site("Env")}}
			if req.Mode == "recheck" {
				analysis["tagSites"] = []any{site("Environment")}
			}
			for _, ts := range req.TraceSites {
				if ts.ConstructPath == homeStack+"/Bucket" && ts.File == "lib/stack.ts" && ts.Line == 5 {
					analysis["constructSites"] = []any{map[string]any{"constructPath": homeStack + "/Bucket", "file": "lib/stack.ts",
						"line": 5, "constructId": "Bucket", "from": "trace"}}
				}
			}
			out, err := json.Marshal(analysis)
			return procexec.Result{Stdout: out}, err
		}
		return procexec.Result{}, errUnanswered
	}}
}

// givenHomeApps writes the two apps of the home test.
func (w *iacWorld) givenHomeApps() {
	w.givenFile("infra/ts/cdk.json", `{"app": "npx ts-node bin/app.ts"}`)
	w.givenFile("infra/ts/package.json", `{"dependencies": {"aws-cdk-lib": "2.275.0"}}`)
	w.givenFile("infra/ts/bin/app.ts", "// the app\n")
	w.givenFile("infra/ts/lib/stack.ts", homeSource)
	w.givenAppSources("infra/bad")
}

// NFR1.11, NFR1.12, NFR1.13: Given a home folder of alice, a trace that holds
// her paths, ARNs and an account ID, and a failing app whose standard error
// holds them with a GitHub token, When iac opens a pull request, Then neither
// alice, the ARN, the account ID nor the token is in the body, the commit
// message, standard error or standard output; and with --debug the home
// folder of the lines of standard error is ~.
func TestV02U3NoHomeOrSecretsLeak(t *testing.T) {
	resource := stackResource("arn:aws:s3:::tag-refactor-u3-home-bucket", homeStack, "Bucket83908E77", map[string]string{"Env": "dev"},
		renameKey("c-000001", "Env", "Environment", "dev"))
	t.Run("pull request", func(t *testing.T) {
		w := newPRWorld(t)
		w.givenHomeApps()
		w.givenPushed()
		plan := w.givenPlan(resource)
		p := homePrograms(t)

		r := w.iacWith(context.Background(), p, plan, "--lang", "en", "--pr")

		r.requireExit(t, 2)
		create, ok := w.gh.Find("pr", "create")
		if !ok {
			t.Fatalf("gh pr create was not called:\n%s", r.stderr)
		}
		requireContains(t, "body", create.Body, "`infra/ts/lib/stack.ts`")
		for what, text := range map[string]string{"body": create.Body, "commit message": w.out("log", "-1", "--format=%B"),
			"stderr": r.stderr, "stdout": r.stdout} {
			for _, secret := range []string{"alice", u3ARN, u3Account, u3Token} {
				requireNotContains(t, what, text, secret)
			}
		}
		if n := strings.Count(strings.Join(argsOf(p, isSynth), "\n"), "--debug"); n != 1 {
			t.Errorf("synth ran with --debug %d times, want 1 (the trace of infra/ts)", n)
		}
	})
	t.Run("debug", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenHomeApps()
		plan := w.givenPlan(resource)
		// The world has a home of its own for git; iac reads it when it starts.
		t.Setenv("HOME", homeOfAlice())
		t.Setenv("USERPROFILE", homeOfAlice())

		r := w.iacWith(context.Background(), homePrograms(t), plan, "--lang", "en", "--debug")

		r.requireExit(t, 2)
		// The lines are sanitized: a backslash is written twice.
		requireContains(t, "stderr", r.stderr, strings.ReplaceAll(filepath.Join("~", "work", "app", "bin", "app.ts"), `\`, `\\`)+":3:7)")
		requireNotContains(t, "stderr", r.stderr, display.Sanitize(homeOfAlice()))
		requireContains(t, "stderr", r.stderr, "trace: 1 construct paths")
		for _, secret := range []string{u3ARN, u3Account, u3Token} {
			requireNotContains(t, "stderr", r.stderr, secret)
		}
		requireContains(t, "stdout", r.stdout, "infra/ts/lib/stack.ts: 1 change can be reflected\n")
	})
}

// argsOf lists the arguments of the calls that match, one line each.
func argsOf(p *programs, match func(procexec.Command) bool) []string {
	var lines []string
	for _, c := range p.recorded() {
		if match(c) {
			lines = append(lines, strings.Join(c.Args, " "))
		}
	}
	return lines
}

// NFR10.5: Given an app without Python, an app with an old Python, a wrong
// --cdk-app, a wrong --synth-timeout and an app that needs AWS, When iac runs
// with --lang ja and --lang en, Then each says so in that language.
func TestV02U3MessagesInBothLanguages(t *testing.T) {
	needsAWS := func(_ context.Context, c procexec.Command) (procexec.Result, error) {
		if isSynth(c) {
			return procexec.Result{ExitCode: 1, Stderr: []byte("Need to perform AWS calls for account 000000000000, but no credentials have been configured\n")}, nil
		}
		return procexec.Result{}, errUnanswered
	}
	oldPython := func(_ context.Context, c procexec.Command) (procexec.Result, error) {
		if isPythonVersion(c) {
			return procexec.Result{Stdout: []byte("Python 3.8.18\n")}, nil
		}
		return procexec.Result{}, errUnanswered
	}
	for _, tc := range []struct {
		lang                                      string
		noPython, oldPython, cdkApp, timeout, aws string
	}{
		{"en", "install Python 3", "install Python 3.10 or later", "--cdk-app infra/none is not a CDK app found below --dir",
			"--synth-timeout abc is not a duration of 1s or more", "AWS credentials needed"},
		{"ja", "Python 3 を入れてください", "Python 3.10 以上を入れてください", "--cdk-app infra/none は --dir の下で見つかった CDK のアプリではありません",
			"--synth-timeout abc は 1 秒以上の時間ではありません", "AWS の認証情報が要る"},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			w := newIacWorld(t)
			w.givenPyApp("infra/py")
			plan := w.givenPlan(strayResource())

			r := w.iacWith(context.Background(), missingPython(), plan, "--lang", tc.lang)
			requireContains(t, "stderr", r.stderr, tc.noPython)

			old := newIacWorld(t)
			old.givenPyApp("infra/py")
			old.givenFile("infra/py/"+venvPythonRel(), "")
			r = old.iacWith(context.Background(), &programs{answer: oldPython}, plan, "--lang", tc.lang)
			requireContains(t, "stderr", r.stderr, tc.oldPython)

			r = w.iacWith(context.Background(), &programs{}, plan, "--lang", tc.lang, "--cdk-app", "infra/none")
			r.requireExit(t, 1)
			requireContains(t, "stderr", r.stderr, tc.cdkApp)

			r = w.iacWith(context.Background(), &programs{}, plan, "--lang", tc.lang, "--synth-timeout", "abc")
			r.requireExit(t, 1)
			requireContains(t, "stderr", r.stderr, tc.timeout)

			ts := newIacWorld(t)
			ts.givenAppSources("infra")
			r = ts.iacWith(context.Background(), &programs{answer: needsAWS}, plan, "--lang", tc.lang)
			requireContains(t, "stderr", r.stderr, tc.aws)
			requireContains(t, "stderr", r.stderr, "cdk.context.json")
		})
	}
}

// NFR5.3: Given a plan without resources made by CloudFormation and no
// --cdk-app, When iac runs, Then no app is looked for or synthesized and the
// output is the one of a folder without apps; and given one TypeScript app and
// neither --cdk-app nor --synth-timeout, the lines of standard error are those
// of U1.
func TestV02U3UnchangedWithoutCDKResources(t *testing.T) {
	plan := newIacWorld(t).givenPlan(planResource(arnOther, map[string]string{"env": "Prod"}, []map[string]any{
		{"changeId": "c-000001", "kind": "changeValue", "fromKey": "env", "fromValue": "Prod", "toValue": "prod", "impacted": false},
	}))
	with := newIacWorld(t)
	with.givenAppSources("infra")
	without := newIacWorld(t)
	p := &programs{}

	a := with.iacWith(context.Background(), p, plan, "--lang", "en")
	b := without.iacWith(context.Background(), &programs{}, plan, "--lang", "en")

	if calls := p.recorded(); len(calls) != 0 {
		t.Fatalf("programs ran: %v", calls)
	}
	if a.code != b.code || a.stdout != b.stdout || a.stderr != b.stderr {
		t.Fatalf("the output with an app differs:\n%d %s%s\nwithout:\n%d %s%s", a.code, a.stdout, a.stderr, b.code, b.stdout, b.stderr)
	}

	one := newIacWorld(t)
	one.givenAppSources("infra")
	r := one.iacWith(context.Background(), &programs{}, one.givenPlan(strayResource()), "--lang", "en")

	want := "CDK apps to synthesize: infra (waiting)\ninfra: synthesizing (TypeScript)\ninfra: done (0s)\n"
	if !strings.Contains(r.stderr, want) {
		t.Fatalf("the lines of the app are not those of U1:\n%s", r.stderr)
	}
	if lines := regexp.MustCompile(`(?m)^infra: `).FindAllString(r.stderr, -1); len(lines) != 2 {
		t.Fatalf("the app has %d lines, want 2:\n%s", len(lines), r.stderr)
	}
}
