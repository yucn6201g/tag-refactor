package cli_test

// Acceptance tests for v0.2.0 U1, the walking skeleton: iac reflects the
// rename of one key in a TypeScript CDK app, found through cdk synth, and with
// --pr opens a pull request of the files it wrote. Like the earlier units,
// each test is an acceptance criterion written as Given/When/Then and run from
// the command entry point (cli.Run).
//
// These tests were written before the U1 implementation (team testing
// posture: acceptance criteria first).
//
// The tests marked "real synth" run cdk synth and the helper on a copy of the
// test app internal/cdk/testdata/apps/ts-minimal: they need Node.js and npm ci
// in that folder, fail without them in CI and are skipped elsewhere. git is
// the real one, with a bare repository in a temporary folder as origin; gh is
// the fake of testutil. AWS is never called: the plans are made in the tests.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The resource of the test app as synth names it.
const (
	skeletonStack     = "tag-refactor-cdk-skeleton"
	skeletonLogicalID = "Bucket83908E77"
	skeletonARN       = "arn:aws:s3:::tag-refactor-cdk-skeleton-bucket83908e77-abcdefghijkl"
	skeletonTag       = "Tags.of(bucket).add('Env', 'dev');"
	skeletonTagAfter  = "Tags.of(bucket).add('Environment', 'dev');"
)

// skeletonResource is the bucket of the test app in a plan that renames Env to
// Environment, with the tags CloudFormation puts on what it makes.
func skeletonResource() map[string]any {
	r := planResource(skeletonARN, map[string]string{"Env": "dev",
		"aws:cloudformation:stack-name": skeletonStack, "aws:cloudformation:logical-id": skeletonLogicalID},
		[]map[string]any{{"changeId": "c-000001", "kind": "renameKey", "fromKey": "Env", "toKey": "Environment",
			"fromValue": "dev", "mergesIntoExisting": false, "impacted": false}})
	r["resourceType"] = "s3"
	return r
}

// testApp returns the test app, or ends the test when its tools are missing
// (BR5.1): a failure in CI, a skip elsewhere.
func testApp(t *testing.T) string {
	t.Helper()
	app := testutil.CDKTestApp("ts-minimal")
	if !testutil.RequireCDKTools(t, app) {
		t.FailNow()
	}
	return app
}

// givenApp copies the test app into the folder of the world, at rel.
func (w *iacWorld) givenApp(app, rel string) {
	w.t.Helper()
	testutil.CopyCDKApp(w.t, app, filepath.Join(w.dir, filepath.FromSlash(rel)))
	w.givenFile(".gitignore", "node_modules\ncdk.out\n")
}

// runWith runs the command line with deps on top of the settings of the world.
func (w *iacWorld) runWith(deps cli.Deps, args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC), 10*time.Millisecond)
	deps.Transport, deps.Now, deps.RetryDelay = w.fake, clock.Now, func(int) time.Duration { return 0 }
	code := cli.Run(context.Background(), args, &stdout, &stderr, deps)
	if n := len(w.fake.Calls()); n != 0 {
		w.t.Fatalf("iac called AWS %d times: %v", n, w.fake.Ops())
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// planNamed saves a plan under a file name of the test's choosing.
func (w *iacWorld) planNamed(name string, resources ...map[string]any) string {
	w.t.Helper()
	saved := w.givenPlan(resources...)
	path := filepath.Join(w.t.TempDir(), name)
	if err := os.Rename(saved, path); err != nil {
		w.t.Fatal(err)
	}
	return path
}

// prWorld is a repository whose origin is a GitHub URL for gh and, through
// url.<bare>.insteadOf, a bare repository in a temporary folder for git.
type prWorld struct {
	*iacWorld
	bare string
	gh   *testutil.FakeGH
}

const fakeRemote = "https://github.com/o/r.git"

func newPRWorld(t *testing.T) *prWorld {
	t.Helper()
	w := &prWorld{iacWorld: newIacWorld(t)}
	w.bare = filepath.Join(t.TempDir(), "origin.git")
	gitAt(t, "", "init", "-q", "--bare", w.bare)
	w.git("init", "-q")
	w.git("symbolic-ref", "HEAD", "refs/heads/main")
	hooks := t.TempDir()
	for _, kv := range [][2]string{
		{"user.name", "t"}, {"user.email", "t@example.invalid"}, {"commit.gpgsign", "false"},
		{"core.autocrlf", "false"}, {"core.hooksPath", filepath.ToSlash(hooks)},
		{"remote.origin.url", fakeRemote}, {"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"},
		{"url." + filepath.ToSlash(w.bare) + ".insteadOf", fakeRemote},
	} {
		w.git("config", kv[0], kv[1])
	}
	w.gh = testutil.InstallFakeGH(t)
	return w
}

// givenPushed commits everything and pushes main to the bare repository.
func (w *prWorld) givenPushed() {
	w.t.Helper()
	w.git("add", "-A")
	w.git("commit", "-q", "-m", "sources")
	w.git("push", "-q", "-u", "origin", "main")
}

func (w *prWorld) out(args ...string) string { return gitAt(w.t, w.dir, args...) }

// gitAt runs git in dir without the user's settings and returns what it
// printed.
func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// AC1.1.1 (US1.1): Given a committed CDK app in a repository whose origin is
// on GitHub, a file the user staged, and a plan that renames a key of the
// app's bucket, When iac runs with --pr, Then the key is renamed in the
// source, a new branch holds one commit of that file only, the branch is
// pushed, gh opens the pull request against the branch iac ran on, the URL is
// one line of standard output, and nothing was asked. (real synth)
func TestV02U1PullRequestFromACDKApp(t *testing.T) {
	app := testApp(t)
	w := newPRWorld(t)
	w.givenApp(app, "infra")
	w.givenPushed()
	w.givenFile("notes.txt", "staged by the user\n")
	w.git("add", "notes.txt")
	source, err := os.ReadFile(filepath.Join(app, "lib", "stack.ts"))
	if err != nil {
		t.Fatal(err)
	}
	plan := w.givenPlan(skeletonResource())

	r := w.iac(plan, "--pr")

	r.requireExit(t, 0)
	w.requireFile("infra/lib/stack.ts", strings.Replace(string(source), skeletonTag, skeletonTagAfter, 1))
	urls := 0
	for _, line := range strings.Split(r.stdout, "\n") {
		if line == testutil.FakePullRequestURL {
			urls++
		}
	}
	if urls != 1 {
		t.Fatalf("the URL is on %d lines of stdout, want 1:\n%s", urls, r.stdout)
	}
	branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD"))
	requireMatch(t, "branch", branch, `^tag-refactor/iac-\d{8}-\d{6}$`)
	title := "Reflect tag changes from tag-refactor (plan.json)"
	shown := strings.Split(strings.TrimSpace(w.out("show", "--name-only", "--format=%s", "HEAD")), "\n")
	if len(shown) != 3 || shown[0] != title || shown[2] != "infra/lib/stack.ts" {
		t.Fatalf("the commit is %q, want the title and infra/lib/stack.ts only", shown)
	}
	requireContains(t, "staged files", w.out("diff", "--cached", "--name-only"), "notes.txt")
	gitAt(t, "", "--git-dir", w.bare, "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	create, ok := w.gh.Find("pr", "create")
	if !ok {
		t.Fatalf("gh pr create was not called: %v", w.gh.Calls())
	}
	for flag, want := range map[string]string{"--base": "main", "--head": branch, "--title": title} {
		if got := create.Flag(flag); got != want {
			t.Errorf("gh pr create %s %q, want %q", flag, got, want)
		}
	}
	if create.Flag("--body-file") == "" {
		t.Error("gh pr create got no --body-file")
	}
	requireContains(t, "body", create.Body, "`infra/lib/stack.ts` (CDK, TypeScript)")
	requireContains(t, "stderr", r.stderr, "Pull request: origin (github.com/o/r), base main, branch "+branch)
	for _, question := range []string{"[y/N]", "yes/no", "Type yes"} {
		requireNotContains(t, "stderr", r.stderr, question)
	}
}

// AC2.1.2 (US2.1): Given a CDK app, When iac runs, Then standard error shows
// the app waiting, being synthesized, and done with the time it took. (real
// synth)
func TestV02U1SynthProgressOnStandardError(t *testing.T) {
	app := testApp(t)
	w := newIacWorld(t)
	w.givenApp(app, "infra")
	plan := w.givenPlan(skeletonResource())

	r := w.iac(plan)

	r.requireExit(t, 0)
	waiting := strings.Index(r.stderr, "CDK apps to synthesize: infra (waiting)\n")
	running := strings.Index(r.stderr, "infra: synthesizing (TypeScript)\n")
	done := regexp.MustCompile(`(?m)^infra: done \(\d+s\)$`).FindStringIndex(r.stderr)
	if waiting < 0 || running < waiting || done == nil || done[0] < running {
		t.Fatalf("the lines of the app are not waiting, synthesizing and done in order:\n%s", r.stderr)
	}
	requireContains(t, "stdout", r.stdout, "infra/lib/stack.ts: 1 change can be reflected\n")
}

// recordingRunner stands in for the outside programs (procexec.Runner): it
// keeps every call and answers with handle.
type recordingRunner struct {
	mu     sync.Mutex
	calls  []procexec.Command
	handle func(procexec.Command) procexec.Result
}

func (r *recordingRunner) Run(_ context.Context, c procexec.Command) (procexec.Result, error) {
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
	if r.handle == nil {
		return procexec.Result{}, nil
	}
	return r.handle(c), nil
}

func (r *recordingRunner) LookPath(name string) (string, error) {
	return filepath.Join("fake-bin", name), nil
}

func (r *recordingRunner) recorded() []procexec.Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]procexec.Command(nil), r.calls...)
}

func isSynth(c procexec.Command) bool {
	return strings.Contains(filepath.Base(c.Name), "cdk") && len(c.Args) > 0 && c.Args[0] == "synth"
}

// envValue returns the value of name in env, comparing names as Windows does.
func envValue(env []string, name string) (string, bool) {
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// givenAppSources writes the files a TypeScript CDK v2 app is told by, without
// installing it: enough for a synth that is stood in for.
func (w *iacWorld) givenAppSources(rel string) {
	w.givenFile(rel+"/cdk.json", `{"app": "npx ts-node --prefer-ts-exts bin/app.ts"}`)
	w.givenFile(rel+"/package.json", `{"dependencies": {"aws-cdk-lib": "2.275.0", "constructs": "10.8.1"}}`)
	w.givenFile(rel+"/bin/app.ts", "// the app\n")
}

// AC2.1.3 (US2.1): Given AWS variables (one in lower case), GitHub tokens and
// AWS credentials in the files of the environment, When iac runs cdk synth,
// Then synth gets none of them, but empty credential files and the variables
// that stop its calls; and when synth answers that it needs AWS, iac says why
// and what to do, puts the resource on the list to fix by hand, and calls no
// AWS and no helper.
func TestV02U1SynthGetsNoCredentials(t *testing.T) {
	w := newIacWorld(t)
	w.givenAppSources("infra")
	t.Setenv("AWS_ACCESS_KEY_ID", testutil.FakeAccessKeyID)
	t.Setenv("aws_session_token", testutil.FakeSessionToken)
	t.Setenv("AWS_PROFILE", "sandbox")
	t.Setenv("GH_TOKEN", "ghp_faketokenfaketokenfaketoken000000")
	t.Setenv("GITHUB_TOKEN", "ghs_faketokenfaketokenfaketoken000000")
	t.Setenv("TAG_REFACTOR_TEST_REPO_TOKEN", "github_pat_faketokenfaketoken")
	emptyFiles := map[string]bool{}
	runner := &recordingRunner{handle: func(c procexec.Command) procexec.Result {
		if !isSynth(c) {
			return procexec.Result{}
		}
		for _, name := range []string{"AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
			path, _ := envValue(c.Env, name)
			info, err := os.Stat(path)
			emptyFiles[name] = err == nil && info.Size() == 0
		}
		return procexec.Result{ExitCode: 1, Stderr: []byte("Unable to resolve AWS account to use. It must be either configured when you define your CDK Stack, or through the environment\n")}
	}}
	plan := w.givenPlan(skeletonResource())

	r := w.runWith(cli.Deps{Exec: runner}, "iac", "--lang", "en", "--plan", plan, "--dir", w.dir)

	r.requireExit(t, 2)
	var synth *procexec.Command
	for _, c := range runner.recorded() {
		switch {
		case isSynth(c):
			synth = &c
		case strings.Contains(strings.Join(c.Args, " "), "helper"):
			t.Errorf("the helper ran after a failed synth: %v", c.Args)
		}
	}
	if synth == nil {
		t.Fatalf("cdk synth did not run: %v", runner.recorded())
	}
	for _, kv := range synth.Env {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		allowed := upper == "AWS_SHARED_CREDENTIALS_FILE" || upper == "AWS_CONFIG_FILE" || upper == "AWS_EC2_METADATA_DISABLED"
		if strings.HasPrefix(upper, "AWS_") && !allowed || upper == "GH_TOKEN" || upper == "GITHUB_TOKEN" || upper == "TAG_REFACTOR_TEST_REPO_TOKEN" {
			t.Errorf("synth got %s", name)
		}
	}
	for name, want := range map[string]string{"AWS_EC2_METADATA_DISABLED": "true", "CDK_DISABLE_VERSION_CHECK": "1", "CDK_DISABLE_CLI_TELEMETRY": "true"} {
		if got, _ := envValue(synth.Env, name); got != want {
			t.Errorf("synth got %s=%q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
		if !emptyFiles[name] {
			t.Errorf("%s of synth was not an empty file", name)
		}
	}
	requireContains(t, "stderr", r.stderr, "infra: failed (")
	requireContains(t, "stderr", r.stderr, "AWS credentials needed")
	requireContains(t, "stderr", r.stderr, "cdk.context.json")
	requireContains(t, "stdout", r.stdout, "To fix by hand (1):\n")
	requireContains(t, "stdout", r.stdout, "the CDK app could not be synthesized or read (cdk)")
}

// AC2.1.6 (US2.1): Given the stack-name and logical-id tags of a resource of
// the plan, When iac runs with --debug, Then it shows the source file and the
// line that makes the resource, and the difference of that file. (real synth)
func TestV02U1ResourceLeadsToItsSourceLine(t *testing.T) {
	app := testApp(t)
	w := newIacWorld(t)
	w.givenApp(app, "infra")
	plan := w.givenPlan(skeletonResource())

	r := w.iac(plan, "--debug")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr,
		"debug: cdk: tag-refactor-cdk-skeleton/Bucket83908E77 is made at infra/lib/stack.ts:9 (tag-refactor-cdk-skeleton/Bucket)")
	requireNotContains(t, "stderr", r.stderr, skeletonARN)
	requireContains(t, "stdout", r.stdout, "--- a/infra/lib/stack.ts\n+++ b/infra/lib/stack.ts\n")
	requireContains(t, "stdout", r.stdout, "\n-    "+skeletonTag+"\n+    "+skeletonTagAfter+"\n")
}

// recordingTB is a ToolTB that keeps what the check asked for.
type recordingTB struct {
	fatal, skip string
}

func (r *recordingTB) Helper()                           {}
func (r *recordingTB) Fatalf(format string, args ...any) { r.fatal = format }
func (r *recordingTB) Skipf(format string, args ...any)  { r.skip = format }

// AC2.1.7 (US2.1): Given Node.js, the CDK CLI or the node_modules of the test
// app missing, When a synth test starts, Then it fails when CI is set and is
// skipped with the reason otherwise; with everything there it goes on.
func TestV02U1MissingToolsFailInCIAndSkipLocally(t *testing.T) {
	empty := t.TempDir()
	noNode := testutil.ToolLookup{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }, Stat: os.Stat}
	for _, tc := range []struct {
		name, ci string
		fails    bool
	}{
		{"in CI", "true", true},
		{"locally", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			look := noNode
			look.Getenv = func(name string) string {
				if name == "CI" {
					return tc.ci
				}
				return ""
			}
			rec := &recordingTB{}

			ok := testutil.RequireCDKToolsWith(rec, empty, look)

			if ok || (rec.fatal != "") != tc.fails || (rec.skip != "") == tc.fails {
				t.Fatalf("ok %v, fatal %q, skip %q", ok, rec.fatal, rec.skip)
			}
		})
	}
	t.Run("everything there", func(t *testing.T) {
		dir := t.TempDir()
		cdkName := "cdk"
		if runtime.GOOS == "windows" {
			cdkName = "cdk.cmd"
		}
		bin := filepath.Join(dir, "node_modules", ".bin")
		if err := os.MkdirAll(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, cdkName), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		look := testutil.ToolLookup{LookPath: func(string) (string, error) { return "node", nil }, Stat: os.Stat,
			Getenv: func(string) string { return "true" }}
		rec := &recordingTB{}

		if !testutil.RequireCDKToolsWith(rec, dir, look) || rec.fatal != "" || rec.skip != "" {
			t.Fatalf("fatal %q, skip %q", rec.fatal, rec.skip)
		}
	})
}

// AC3.1.1, part (US3.1, BR3.5): Given a copy of the test app committed in a
// repository, When iac renames Env to Environment with --write, Then only the
// range of the key changes, and the copy synthesized again gives the bucket
// Environment=dev and no Env. (real synth)
func TestV02U1RenamedKeySynthesizesToTheNewKey(t *testing.T) {
	app := testApp(t)
	w := newIacWorld(t)
	w.givenApp(app, "infra")
	w.givenCommitted()
	source, err := os.ReadFile(filepath.Join(app, "lib", "stack.ts"))
	if err != nil {
		t.Fatal(err)
	}
	plan := w.givenPlan(skeletonResource())

	r := w.iac(plan, "--write")

	r.requireExit(t, 0)
	w.requireFile("infra/lib/stack.ts", strings.Replace(string(source), skeletonTag, skeletonTagAfter, 1))
	requireContains(t, "stdout", r.stdout, "infra/lib/stack.ts: 1 change reflected\n")
	s := cdk.NewSynthesizer(cdk.Config{})
	ctx := context.Background()
	apps, err := s.Discover(ctx, filepath.Join(w.dir, "infra"), nil)
	if err != nil || len(apps) != 1 {
		t.Fatalf("Discover: %v, %v", apps, err)
	}
	syn, err := s.Synthesize(ctx, apps[0], cdk.Options{}, nil)
	if err != nil || syn.Failure != nil {
		t.Fatalf("synth again: %v, %+v", err, syn.Failure)
	}
	found := false
	for _, stack := range syn.Stacks {
		for _, res := range stack.Resources {
			if res.LogicalID != skeletonLogicalID {
				continue
			}
			found = true
			if res.Tags["Environment"] != "dev" {
				t.Errorf("the bucket has Environment=%q, want dev", res.Tags["Environment"])
			}
			if _, ok := res.Tags["Env"]; ok {
				t.Error("the bucket still has Env")
			}
		}
	}
	if !found {
		t.Fatalf("no %s in the synth again: %+v", skeletonLogicalID, syn.Stacks)
	}
}

// BR4.2, NFR4.2: Given --pr and a write that fails at the second file, When iac
// ends, Then no branch, commit, push or pull request is made, the files
// written and not written are shown, and the exit code is 1. The result of the
// writer is given: no file write can be made to fail half way on every system.
func TestV02U1NoPullRequestAfterAFailedWrite(t *testing.T) {
	w := newPRWorld(t)
	w.givenFile("a.yaml", "Resources: {}\n")
	w.givenFile("b.yaml", "Resources: {}\n")
	w.givenPushed()
	plan := w.givenPlan(skeletonResource())
	deps := cli.Deps{}
	cli.SetIacForTest(&deps, func(_ context.Context, req iac.Request) (*iac.Result, error) {
		if !req.Write {
			t.Error("--pr did not ask for --write")
		}
		return &iac.Result{
			Files: []iac.FileResult{
				{File: "a.yaml", Tool: iac.ToolCloudFormation, Reflected: 1, Written: true},
				{File: "b.yaml", Tool: iac.ToolCloudFormation, Reflected: 1},
			},
			WrittenFiles: []string{"a.yaml"}, Reflected: 2, Resources: 2, WriteRequested: true,
			WriteErr: &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
				DetailID: "iac.writeFailed", DetailArgs: []any{"b.yaml"}, HintID: "iac.writeFailedFix"},
		}, nil
	})

	r := w.runWith(deps, "iac", "--lang", "en", "--plan", plan, "--dir", w.dir, "--pr")

	r.requireExit(t, 1)
	requireContains(t, "stdout", r.stdout, "a.yaml: 1 change reflected\n")
	requireContains(t, "stdout", r.stdout, "b.yaml: 1 change not written\n")
	requireContains(t, "stderr", r.stderr, "b.yaml could not be written")
	if _, created := w.gh.Find("pr", "create"); created {
		t.Error("gh pr create was called")
	}
	if branches := strings.TrimSpace(w.out("branch", "--list", "tag-refactor/*")); branches != "" {
		t.Errorf("a branch was made: %s", branches)
	}
	if heads := strings.TrimSpace(gitAt(t, "", "--git-dir", w.bare, "for-each-ref", "--format=%(refname)", "refs/heads")); heads != "refs/heads/main" {
		t.Errorf("origin holds %s", heads)
	}
	if subject := strings.TrimSpace(w.out("log", "-1", "--format=%s")); subject != "sources" {
		t.Errorf("a commit was made: %s", subject)
	}
}

// NFR5, NFR3.6: Given a CDK app in --dir and a plan without any resource made
// by CloudFormation, When iac runs, Then neither cdk synth nor the helper
// starts, and the output is the one of before.
func TestV02U1NoSynthWithoutCloudFormationResources(t *testing.T) {
	w := newIacWorld(t)
	w.givenAppSources("infra")
	runner := &recordingRunner{}
	plan := w.givenPlan(planResource(arnOther, map[string]string{"env": "Prod"}, []map[string]any{
		{"changeId": "c-000001", "kind": "changeValue", "fromKey": "env", "fromValue": "Prod", "toValue": "prod", "impacted": false},
	}))

	r := w.runWith(cli.Deps{Exec: runner}, "iac", "--lang", "en", "--plan", plan, "--dir", w.dir)

	r.requireExit(t, 0)
	if calls := runner.recorded(); len(calls) != 0 {
		t.Fatalf("outside programs ran: %v", calls)
	}
	requireContains(t, "stdout", r.stdout, "Not managed by IaC: 1 resource")
	requireNotContains(t, "stderr", r.stderr, "CDK")
}

// NFR1.2, BR4.3, BR4.4: Given a plan whose file name holds an account ID and
// (where file names allow it) an ARN, When iac opens a pull request, Then the
// title, the body and the commit message show neither.
func TestV02U1PullRequestTextHidesAccountsAndARNs(t *testing.T) {
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenPushed()
	name := "plan-123456789012.json"
	if runtime.GOOS != "windows" {
		name = "plan-123456789012-arn:aws:s3:::secret-bucket.json"
	}
	plan := w.planNamed(name, planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

	r := w.iac(plan, "--pr")

	r.requireExit(t, 0)
	create, ok := w.gh.Find("pr", "create")
	if !ok {
		t.Fatalf("gh pr create was not called:\n%s", r.stderr)
	}
	title := create.Flag("--title")
	requireContains(t, "title", title, "<redacted>")
	for what, text := range map[string]string{"title": title, "body": create.Body,
		"commit message": w.out("log", "-1", "--format=%B")} {
		for _, secret := range []string{"123456789012", "arn:aws", "secret-bucket"} {
			requireNotContains(t, what, text, secret)
		}
	}
	requireContains(t, "body", create.Body, "`stacks/orders.yaml` (CloudFormation)")
}
