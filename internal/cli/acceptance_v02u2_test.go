package cli_test

// Acceptance tests for v0.2.0 U2, the rest of the pull request: the options
// --pr-branch, --pr-title and --pr-body-file and their checks, the shapes that
// stop a body or are told about, the redaction of the title, the remote and the
// branch on it, the body with the list to fix by hand, nothing to reflect, and
// the resume commands after a failed or stopped step. Like the earlier units,
// each test is an acceptance criterion written as Given/When/Then and run from
// the command entry point (cli.Run).
//
// These tests were written before the U2 implementation (team testing
// posture: acceptance criteria first).
//
// git is the real one, with bare repositories in temporary folders as the
// remotes; gh is the fake of testutil. They use CloudFormation templates, so
// that no synth is needed. AWS is never called: the plans are made here. The
// shapes of tokens are put together in the tests and are no real tokens.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// u2Clock is the time of the runs of these tests, in a zone nine hours ahead of
// UTC: the default branch takes the time in UTC.
var u2Clock = time.Date(2026, 11, 8, 18, 30, 5, 0, time.FixedZone("UTC+9", 9*60*60))

// u2Branch is the default branch of a run at u2Clock.
const u2Branch = "tag-refactor/iac-20261108-093005Z"

// runU2 runs the command line with ctx, deps and the clock at u2Clock.
func (w *prWorld) runU2(ctx context.Context, deps cli.Deps, args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(u2Clock, 0)
	deps.Transport, deps.Now, deps.RetryDelay = w.fake, clock.Now, func(int) time.Duration { return 0 }
	code := cli.Run(ctx, args, &stdout, &stderr, deps)
	if n := len(w.fake.Calls()); n != 0 {
		w.t.Fatalf("iac called AWS %d times: %v", n, w.fake.Ops())
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// iacPR runs iac --pr in English with the given options.
func (w *prWorld) iacPR(plan string, args ...string) result {
	w.t.Helper()
	return w.runU2(context.Background(), cli.Deps{},
		append([]string{"iac", "--lang", "en", "--plan", plan, "--dir", w.dir, "--pr"}, args...)...)
}

// ordersWorld is a repository with the template of the orders queue pushed to
// origin, and a plan of the four changes of the example.
func ordersWorld(t *testing.T) (*prWorld, string) {
	t.Helper()
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenPushed()
	return w, w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))
}

// givenBodyFile writes a body file outside the repository.
func (w *prWorld) givenBodyFile(content string) string {
	w.t.Helper()
	path := filepath.Join(w.t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

// givenHook writes a hook of git in the folder of the hooks of the world.
func (w *prWorld) givenHook(name, script string) {
	w.t.Helper()
	hooks := strings.TrimSpace(w.out("config", "core.hooksPath"))
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(hooks), name), []byte(script), 0o755); err != nil {
		w.t.Fatal(err)
	}
}

// requireNothingDone checks that the sources, the branch, the commits and the
// remote are as before, and that gh was not called.
func (w *prWorld) requireNothingDone(source string) {
	w.t.Helper()
	w.requireFile("stacks/orders.yaml", source)
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		w.t.Fatalf("the working tree is on %s", branch)
	}
	if subject := strings.TrimSpace(w.out("log", "-1", "--format=%s")); subject != "sources" {
		w.t.Fatalf("a commit was made: %s", subject)
	}
	if heads := strings.TrimSpace(gitAt(w.t, "", "--git-dir", w.bare, "for-each-ref", "--format=%(refname)", "refs/heads")); heads != "refs/heads/main" {
		w.t.Fatalf("origin holds %s", heads)
	}
	if calls := w.gh.Calls(); len(calls) != 0 {
		w.t.Fatalf("gh was called: %v", calls)
	}
}

// Shapes put together here: they are no real values.
func fakeGitHubToken() string { return "gh" + "p_" + strings.Repeat("a1B2", 5) }
func fakeAccessKey() string   { return "AK" + "IA" + strings.Repeat("Q7", 8) }
func fakeURLWithPassword() string {
	return "https://" + "someone" + ":" + "pa55word" + "@example.invalid/x"
}

// resumeHeader is the line before the resume commands, for the shell that a
// run with MSYSTEM set to msystem prints them for.
func resumeHeader(msystem string) string {
	if runtime.GOOS == "windows" && msystem == "" {
		return "To finish by hand, run these lines in PowerShell, one at a time from the top:"
	}
	return "To finish by hand, run these lines in sh, one at a time from the top:"
}

// resumeLines returns the lines of the resume commands in stderr: those after
// the header, up to the first line that does not start with git or gh.
func resumeLines(t *testing.T, stderr, header string) []string {
	t.Helper()
	_, rest, ok := strings.Cut(stderr, header+"\n")
	if !ok {
		t.Fatalf("no %q in:\n%s", header, stderr)
	}
	var lines []string
	for _, line := range strings.Split(rest, "\n") {
		if !strings.HasPrefix(line, "  git ") && !strings.HasPrefix(line, "  gh ") {
			break
		}
		lines = append(lines, strings.TrimPrefix(line, "  "))
	}
	return lines
}

// AC1.1.2 (US1.1): Given --pr-branch, --pr-title and --pr-body-file, When iac
// --pr runs, Then the branch, the title, the commit message and the body are
// theirs, with the user's body first and the changed files after it; Given
// none of them, Then the branch is named by the time in UTC with a Z.
func TestV02U2PullRequestOptions(t *testing.T) {
	t.Run("given", func(t *testing.T) {
		w, plan := ordersWorld(t)
		body := w.givenBodyFile("Please review the tags.\n")

		r := w.iacPR(plan, "--pr-branch", "feature/tags", "--pr-title", "Unify the tags of orders", "--pr-body-file", body)

		r.requireExit(t, 0)
		if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "feature/tags" {
			t.Fatalf("the working tree is on %s", branch)
		}
		create, ok := w.gh.Find("pr", "create")
		if !ok {
			t.Fatalf("gh pr create was not called:\n%s", r.stderr)
		}
		if create.Flag("--head") != "feature/tags" || create.Flag("--title") != "Unify the tags of orders" {
			t.Fatalf("gh pr create %v", create.Args)
		}
		if !strings.HasPrefix(create.Body, "Please review the tags.\n\n## Changed files (1)\n") {
			t.Fatalf("body:\n%s", create.Body)
		}
		requireNotContains(t, "body", create.Body, "was opened by")
		if subject := strings.TrimSpace(w.out("log", "-1", "--format=%s")); subject != "Unify the tags of orders" {
			t.Fatalf("commit message %q", subject)
		}
	})
	t.Run("not given", func(t *testing.T) {
		w, plan := ordersWorld(t)

		r := w.iacPR(plan)

		r.requireExit(t, 0)
		if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != u2Branch {
			t.Fatalf("the working tree is on %s, want %s", branch, u2Branch)
		}
		create, _ := w.gh.Find("pr", "create")
		requireContains(t, "body", create.Body, "`tag-refactor iac --pr`")
	})
}

// AC1.1.2 (US1.1, BR1.1, NFR6.12): Given wrong options or a body file that
// cannot be taken, When iac runs, Then it stops with 1 before anything is
// written: the source is the same, git writes nothing and gh is not called.
func TestV02U2InputErrorsChangeNothing(t *testing.T) {
	cases := []struct {
		name string
		args func(w *prWorld) []string
		want string
	}{
		{"--pr-title without --pr", func(*prWorld) []string { return []string{"--pr-title", "x"} }, "--pr-title needs --pr"},
		{"@{-1}", func(*prWorld) []string { return []string{"--pr", "--pr-branch", "@{-1}"} }, "@{-1} is not a valid branch name"},
		{"-x", func(*prWorld) []string { return []string{"--pr", "--pr-branch=-x"} }, "-x is not a valid branch name"},
		{"--help", func(*prWorld) []string { return []string{"--pr", "--pr-branch=--help"} }, "--help is not a valid branch name"},
		{"a..b", func(*prWorld) []string { return []string{"--pr", "--pr-branch", "a..b"} }, "a..b is not a valid branch name"},
		{"title of two lines", func(*prWorld) []string { return []string{"--pr", "--pr-title", "a\nb"} }, "one line of 1 to 256 characters"},
		// Review N4: an empty value is an error for each of the three options.
		{"empty title", func(*prWorld) []string { return []string{"--pr", "--pr-title", ""} }, "one line of 1 to 256 characters"},
		{"empty branch", func(*prWorld) []string { return []string{"--pr", "--pr-branch", ""} }, "--pr-branch needs a value that is not empty"},
		{"empty body file", func(*prWorld) []string { return []string{"--pr", "--pr-body-file="} }, "--pr-body-file needs a value that is not empty"},
		{"body over 1 MiB", func(w *prWorld) []string {
			return []string{"--pr", "--pr-body-file", w.givenBodyFile(strings.Repeat("a", 1<<20+1))}
		}, "larger than 1 MiB"},
		{"body of 60,001 characters", func(w *prWorld) []string {
			return []string{"--pr", "--pr-body-file", w.givenBodyFile(strings.Repeat("あ", 60001))}
		}, "more than 60,000 characters"},
		{"body not UTF-8", func(w *prWorld) []string {
			return []string{"--pr", "--pr-body-file", w.givenBodyFile("a\xffb")}
		}, "not UTF-8"},
		{"named pipe", func(w *prWorld) []string {
			if runtime.GOOS == "windows" {
				w.t.Skip("no named pipes in the file system on Windows")
			}
			pipe := filepath.Join(w.t.TempDir(), "pipe")
			if err := exec.Command("mkfifo", pipe).Run(); err != nil {
				w.t.Skipf("mkfifo: %v", err)
			}
			return []string{"--pr", "--pr-body-file", pipe}
		}, "not a regular file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, plan := ordersWorld(t)
			args := tc.args(w)

			r := w.runU2(context.Background(), cli.Deps{},
				append([]string{"iac", "--lang", "en", "--plan", plan, "--dir", w.dir}, args...)...)

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireContains(t, "stderr", r.stderr, tc.want)
			w.requireNothingDone(ordersYAML)
		})
	}
}

// NFR1.6: Given a body file with a GitHub token, an AWS access key ID and a URL
// with a password, When iac --pr runs with --debug, Then it stops with 1, no
// file changes, and standard error, --debug included, names the shapes and
// their lines but none of the values.
func TestV02U2SecretsInTheBodyStop(t *testing.T) {
	w, plan := ordersWorld(t)
	body := w.givenBodyFile("Context:\ntoken " + fakeGitHubToken() + "\nkey " + fakeAccessKey() + "\nsee " + fakeURLWithPassword() + "\n")

	r := w.iacPR(plan, "--pr-body-file", body, "--debug")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	for _, want := range []string{"GitHub token on line 2", "AWS access key ID on line 3", "URL with a password on line 4"} {
		requireContains(t, "stderr", r.stderr, want)
	}
	for _, secret := range []string{fakeGitHubToken(), fakeAccessKey(), "pa55word", "a1B2a1B2"} {
		requireNotContains(t, "stderr", r.stderr, secret)
	}
	w.requireNothingDone(ordersYAML)
}

// BR1.1 (unit test, written after the implementation): a relative path of
// --pr-body-file is read from the current folder, not from --dir.
func TestIacPRBodyFileRelativeToTheCurrentFolder(t *testing.T) {
	w, plan := ordersWorld(t)
	body := w.givenBodyFile("Relative body.\n")
	t.Chdir(filepath.Dir(body))

	r := w.iacPR(plan, "--pr-body-file", filepath.Base(body))

	r.requireExit(t, 0)
	create, ok := w.gh.Find("pr", "create")
	if !ok || !strings.HasPrefix(create.Body, "Relative body.\n\n") {
		t.Fatalf("gh pr create %v, body:\n%s\nstderr:\n%s", ok, create.Body, r.stderr)
	}
}

// NFR1.7: Given a body file with an ARN, When iac --pr runs, Then one line
// tells about it before anything is written, the body of the pull request keeps
// the ARN as written, and the exit code is 0.
func TestV02U2NoticeOfAnARNInTheBody(t *testing.T) {
	w, plan := ordersWorld(t)
	arn := "arn:aws:sqs:ap-northeast-1:111122223333:orders"
	body := w.givenBodyFile("Queue: " + arn + "\n")

	r := w.iacPR(plan, "--pr-body-file", body)

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Note: the file of --pr-body-file holds an ARN on line 1; it goes into the pull request as it is\n")
	requireContains(t, "stderr", r.stderr, "Note: the file of --pr-body-file holds a 12-digit number on line 1; it goes into the pull request as it is\n")
	if strings.Index(r.stderr, "holds an ARN") > strings.Index(r.stderr, "Pull request: branch started") {
		t.Fatalf("the notice is not before the steps:\n%s", r.stderr)
	}
	create, _ := w.gh.Find("pr", "create")
	requireContains(t, "body", create.Body, "Queue: "+arn+"\n")
}

// NFR1.8: Given an ARN in --pr-title, When iac --pr runs, Then the title of gh
// and the commit message show <redacted> in its place, and one line says so.
func TestV02U2TitleIsRedacted(t *testing.T) {
	w, plan := ordersWorld(t)

	r := w.iacPR(plan, "--pr-title", "Tags of arn:aws:sqs:ap-northeast-1:111122223333:orders")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Some values of --pr-title were replaced with <redacted>")
	create, _ := w.gh.Find("pr", "create")
	if got := create.Flag("--title"); got != "Tags of <redacted>" {
		t.Fatalf("title %q", got)
	}
	if subject := strings.TrimSpace(w.out("log", "-1", "--format=%s")); subject != "Tags of <redacted>" {
		t.Fatalf("commit message %q", subject)
	}
}

// AC1.1.3 (US1.1): Given the branch iac runs on follows upstream, When iac
// --pr runs, Then it pushes to upstream and opens the pull request there;
// Given it follows nothing, Then origin.
func TestV02U2RemoteIsTheOneFollowed(t *testing.T) {
	t.Run("upstream", func(t *testing.T) {
		w, plan := ordersWorld(t)
		up := filepath.Join(t.TempDir(), "upstream.git")
		gitAt(t, "", "init", "-q", "--bare", up)
		w.git("config", "remote.upstream.url", "https://github.com/up/r.git")
		w.git("config", "url."+filepath.ToSlash(up)+".insteadOf", "https://github.com/up/r.git")
		w.git("push", "-q", "upstream", "main")
		w.git("config", "branch.main.remote", "upstream")

		r := w.iacPR(plan)

		r.requireExit(t, 0)
		create, _ := w.gh.Find("pr", "create")
		if create.Flag("--repo") != "up/r" {
			t.Fatalf("gh pr create %v", create.Args)
		}
		gitAt(t, "", "--git-dir", up, "rev-parse", "--verify", "-q", "refs/heads/"+u2Branch)
		if heads := strings.TrimSpace(gitAt(t, "", "--git-dir", w.bare, "for-each-ref", "--format=%(refname)", "refs/heads")); heads != "refs/heads/main" {
			t.Fatalf("origin holds %s", heads)
		}
	})
	t.Run("nothing followed", func(t *testing.T) {
		w, plan := ordersWorld(t)
		w.git("config", "--unset", "branch.main.remote")

		r := w.iacPR(plan)

		r.requireExit(t, 0)
		create, _ := w.gh.Find("pr", "create")
		if create.Flag("--repo") != "o/r" {
			t.Fatalf("gh pr create %v", create.Args)
		}
		gitAt(t, "", "--git-dir", w.bare, "rev-parse", "--verify", "-q", "refs/heads/"+u2Branch)
	})
}

// AC1.1.4 (US1.1, BR2.1): Given a branch of the same name on the remote, When
// iac --pr runs, Then no file changes, the exit code is 1, and it says to give
// another name with --pr-branch.
func TestV02U2BranchOnTheRemoteStops(t *testing.T) {
	w, plan := ordersWorld(t)
	w.git("push", "-q", "origin", "main:refs/heads/feature/tags")

	r := w.runU2(context.Background(), cli.Deps{}, "iac", "--lang", "en", "--plan", plan, "--dir", w.dir, "--pr",
		"--pr-branch", "feature/tags")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "the branch feature/tags is already on the remote origin")
	requireContains(t, "stderr", r.stderr, "--pr-branch")
	w.requireFile("stacks/orders.yaml", ordersYAML)
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		t.Fatalf("the working tree is on %s", branch)
	}
	if _, created := w.gh.Find("pr", "create"); created {
		t.Fatal("gh pr create was called")
	}
}

// AC1.1.5 (US1.1, BR2.4): Given a pre-commit hook and another file staged,
// When iac --pr runs, Then the hook runs, the commit holds the written file
// only, and the other file stays staged.
func TestV02U2HooksRunAndOtherStagedFilesStay(t *testing.T) {
	w, plan := ordersWorld(t)
	marker := filepath.ToSlash(filepath.Join(t.TempDir(), "hook-ran"))
	w.givenHook("pre-commit", "#!/bin/sh\necho ran > '"+marker+"'\n")
	w.givenFile("notes.txt", "staged by the user\n")
	w.git("add", "notes.txt")

	r := w.iacPR(plan)

	r.requireExit(t, 0)
	if _, err := os.Stat(filepath.FromSlash(marker)); err != nil {
		t.Fatalf("the pre-commit hook did not run: %v", err)
	}
	shown := strings.Split(strings.TrimSpace(w.out("show", "--name-only", "--format=%s", "HEAD")), "\n")
	if len(shown) != 3 || shown[2] != "stacks/orders.yaml" {
		t.Fatalf("the commit holds %q", shown)
	}
	requireContains(t, "staged files", w.out("diff", "--cached", "--name-only"), "notes.txt")
}

// manualWorld is a repository whose template has Owner=team-a, and a plan that
// changes Owner from team-x: a value that differs, to fix by hand.
func manualWorld(t *testing.T, withOthers bool) (*prWorld, string) {
	t.Helper()
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenPushed()
	changes := []map[string]any{{"changeId": "c-000005", "kind": "changeValue", "fromKey": "Owner", "fromValue": "team-x",
		"toValue": "team-b", "impacted": false}}
	if withOthers {
		changes = append(changesOfTheExample(), changes...)
	}
	return w, w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changes))
}

// AC1.1.6 (US1.1, BR3.1, BR4.2): Given a plan with one change to fix by hand,
// When iac --lang ja --pr runs, Then the title and the body are Japanese, the
// body lists the changed file with its kind and the change to fix by hand with
// the logical ID of the resource (review N3), and
// the exit code is 2.
func TestV02U2BodyInJapaneseWithTheListToFixByHand(t *testing.T) {
	w, plan := manualWorld(t, true)

	r := w.runU2(context.Background(), cli.Deps{}, "iac", "--lang", "ja", "--plan", plan, "--dir", w.dir, "--pr")

	r.requireExit(t, 2)
	create, ok := w.gh.Find("pr", "create")
	if !ok {
		t.Fatalf("gh pr create was not called:\n%s", r.stderr)
	}
	if got := create.Flag("--title"); got != "tag-refactor によるタグの変更の反映(plan.json)" {
		t.Fatalf("title %q", got)
	}
	for _, want := range []string{"このプルリクエストは `tag-refactor iac --pr` が作りました。", "## 変更したファイル(1)\n",
		"- `stacks/orders.yaml` (CloudFormation)\n", "## 手で直すもの(1)\n", "- `stacks/orders.yaml:",
		" OrdersQueue, Owner: ソースの値が変更の前の値とも後の値とも違います。どちらが正しいかを決めてください\n"} {
		requireContains(t, "body", create.Body, want)
	}
	requireContains(t, "stdout", r.stdout, testutil.FakePullRequestURL+"\n")
}

// AC1.1.7, AC1.1.8 (US1.1, BR4.1): Given the sources show the plan already,
// When iac --pr runs, Then no branch and no pull request are made, it says
// there is nothing to reflect, and the exit code is 0; Given only a change to
// fix by hand, Then the list is shown and the exit code is 2.
func TestV02U2NothingToReflect(t *testing.T) {
	t.Run("nothing at all", func(t *testing.T) {
		w := newPRWorld(t)
		w.givenFile("stacks/orders.yaml", ordersYAMLAfter)
		w.givenPushed()
		plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

		r := w.iacPR(plan)

		r.requireExit(t, 0)
		requireContains(t, "stderr", r.stderr, "Nothing to reflect, so no pull request was opened.")
		w.requireNoBranchNorPullRequest()
	})
	t.Run("only to fix by hand", func(t *testing.T) {
		w, plan := manualWorld(t, false)

		r := w.iacPR(plan)

		r.requireExit(t, 2)
		requireContains(t, "stdout", r.stdout, "To fix by hand (1):\n")
		requireContains(t, "stderr", r.stderr, "No file was written, so no pull request was opened.")
		w.requireNoBranchNorPullRequest()
	})
}

func (w *prWorld) requireNoBranchNorPullRequest() {
	w.t.Helper()
	if _, created := w.gh.Find("pr", "create"); created {
		w.t.Fatal("gh pr create was called")
	}
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		w.t.Fatalf("the working tree is on %s", branch)
	}
}

// cancelAt runs the programs of the computer, and stops the run (as Ctrl+C
// does) instead of running the command whose arguments start with words.
type cancelAt struct {
	procexec.System
	words  string
	cancel context.CancelFunc
}

func (c *cancelAt) Run(ctx context.Context, cmd procexec.Command) (procexec.Result, error) {
	if strings.HasPrefix(strings.Join(cmd.Args, " "), c.words) {
		c.cancel()
		return procexec.Result{}, ctx.Err()
	}
	return c.System.Run(ctx, cmd)
}

// AC1.1.9 (US1.1, BR4.3, NFR1.9, NFR1.10): Given a step that fails or is
// stopped, When iac --pr ends, Then standard error shows the steps done, the
// resume commands for the shell (one command a line, the commit as add and
// commit, the body by --body-file and never in the commands) and where the
// body file is, the exit code is 1, and the body file stays.
func TestV02U2ResumeCommandsAfterAStop(t *testing.T) {
	for _, msystem := range []string{"", "MINGW64"} {
		t.Run("create fails, MSYSTEM="+msystem, func(t *testing.T) {
			t.Setenv("MSYSTEM", msystem)
			w, plan := ordersWorld(t)
			w.gh.FailCreate()

			r := w.iacPR(plan)

			r.requireExit(t, 1)
			requireContains(t, "stderr", r.stderr, "Error: the create step of the pull request failed")
			requireContains(t, "stderr", r.stderr, "(done: branch, commit, push)")
			lines := resumeLines(t, r.stderr, resumeHeader(msystem))
			if len(lines) != 1 || !strings.HasPrefix(lines[0], "gh pr create --repo ") || !strings.Contains(lines[0], " --body-file ") {
				t.Fatalf("resume commands %q", lines)
			}
			requireNotContains(t, "stderr", r.stderr, "## Changed files")
			w.requireBodyFileKept(r.stderr)
		})
	}
	t.Run("commit fails", func(t *testing.T) {
		t.Setenv("MSYSTEM", "MINGW64")
		w, plan := ordersWorld(t)
		w.givenHook("pre-commit", "#!/bin/sh\nexit 1\n")

		r := w.iacPR(plan)

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Error: the commit step of the pull request failed")
		lines := resumeLines(t, r.stderr, resumeHeader("MINGW64"))
		if len(lines) != 4 || !strings.Contains(lines[0], " --literal-pathspecs add -- ") ||
			!strings.Contains(lines[1], " --literal-pathspecs commit --only -m ") || !strings.Contains(lines[2], " push ") ||
			!strings.HasPrefix(lines[3], "gh pr create ") {
			t.Fatalf("resume commands %q", lines)
		}
		for _, line := range lines {
			if strings.Contains(line, "&&") || strings.Contains(line, ";") {
				t.Fatalf("more than one command on a line: %q", line)
			}
		}
		w.requireBodyFileKept(r.stderr)
	})
	t.Run("stopped", func(t *testing.T) {
		w, plan := ordersWorld(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := w.runU2(ctx, cli.Deps{Exec: &cancelAt{words: "pr create", cancel: cancel}},
			"iac", "--lang", "en", "--plan", plan, "--dir", w.dir, "--pr")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Exit code: 1 (interrupted)")
		requireNotContains(t, "stderr", r.stderr, "Error: ")
		lines := resumeLines(t, r.stderr, resumeHeader(os.Getenv("MSYSTEM")))
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "gh pr create ") {
			t.Fatalf("resume commands %q", lines)
		}
		w.requireBodyFileKept(r.stderr)
	})
	t.Run("the body file cannot be written", func(t *testing.T) {
		w, plan := ordersWorld(t)
		w.givenFile(".git/tag-refactor", "a file where the folder of the bodies goes\n")

		r := w.iacPR(plan)

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "The written files stay as they are; no branch was made.")
		requireContains(t, "stderr", r.stderr, "run gh pr create without --body-file")
		if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
			t.Fatalf("the working tree is on %s", branch)
		}
		w.requireFile("stacks/orders.yaml", ordersYAMLAfter)
	})
}

// requireBodyFileKept checks that standard error names the body file, which
// is still there with the body.
func (w *prWorld) requireBodyFileKept(stderr string) {
	w.t.Helper()
	bodies, err := filepath.Glob(filepath.Join(w.dir, ".git", "tag-refactor", "pr-body-*.md"))
	if err != nil || len(bodies) != 1 {
		w.t.Fatalf("body files %v, %v", bodies, err)
	}
	requireContains(w.t, "stderr", stderr, "The body of the pull request is in "+bodies[0])
	if body, _ := os.ReadFile(bodies[0]); !strings.Contains(string(body), "`stacks/orders.yaml` (CloudFormation)") {
		w.t.Fatalf("body:\n%s", body)
	}
}

// AC1.1.10 (US1.1, BR4.4): Given a gh that does not answer and a short limit,
// When iac --pr runs, Then it stops at the limit with 1, and neither output
// holds the GitHub token.
func TestV02U2GhThatDoesNotAnswerStops(t *testing.T) {
	w, plan := ordersWorld(t)
	w.gh.HangCreate()
	token := fakeGitHubToken()
	t.Setenv("GH_TOKEN", token)
	deps := cli.Deps{}
	cli.SetPullRequestTimeoutsForTest(&deps, pullrequest.Timeouts{Read: 30 * time.Second, Commit: 2 * time.Minute, GH: 5 * time.Second})
	start := time.Now()

	r := w.runU2(context.Background(), deps, "iac", "--lang", "en", "--plan", plan, "--dir", w.dir, "--pr", "--debug")

	r.requireExit(t, 1)
	if took := time.Since(start); took > time.Minute {
		t.Fatalf("it took %v", took)
	}
	requireContains(t, "stderr", r.stderr, "the create step of the pull request did not end in time")
	requireNotContains(t, "stderr", r.stderr, token)
	requireNotContains(t, "stdout", r.stdout, token)
}

// BR4.3 (review N2, written after the implementation): Given gh that ends well
// without a URL and a branch name with characters of the shell, When iac --pr
// runs, Then the line that looks for the pull request quotes the name for the
// shell, and the exit code is 0.
func TestV02U2NoURLGivesALineQuotedForTheShell(t *testing.T) {
	branch := "feature/it's-$x;y"
	cases := map[string]string{"MINGW64": `'feature/it'\''s-$x;y'`}
	if runtime.GOOS == "windows" {
		cases[""] = `'feature/it''s-$x;y'`
	}
	for msystem, quoted := range cases {
		t.Run("MSYSTEM="+msystem, func(t *testing.T) {
			t.Setenv("MSYSTEM", msystem)
			w, plan := ordersWorld(t)
			w.gh.CreateWithoutURL()

			r := w.iacPR(plan, "--pr-branch", branch)

			r.requireExit(t, 0)
			shell := "sh"
			if msystem == "" {
				shell = "PowerShell"
			}
			requireContains(t, "stderr", r.stderr, "check it with this line in "+shell+":\n  gh pr list --repo ")
			requireContains(t, "stderr", r.stderr, " --head "+quoted+"\n")
		})
	}
}

// BR3.3, NFR1.8 (review N5, written after the implementation): Given a body
// file of 60,000 characters and sixty templates with long names, so that the
// body is over 65,536 characters even without the list to fix by hand, When
// iac --pr runs, Then it stops with 1 after the files are written: they stay
// written, no branch and no commit are made, nothing is pushed and gh pr
// create is not called.
func TestV02U2BodyTooLongEvenWithoutTheList(t *testing.T) {
	w := newPRWorld(t)
	var resources []map[string]any
	var files []string
	for i := 0; i < 60; i++ {
		name := fmt.Sprintf("stack-%02d-%s", i, strings.Repeat("x", 60))
		logical := fmt.Sprintf("Q%02d", i)
		file := "stacks/" + name + ".yaml"
		files = append(files, file)
		w.givenFile(file, strings.Replace(ordersYAML, "OrdersQueue", logical, 1))
		changes := []map[string]any{{"changeId": fmt.Sprintf("c-%06d", 2*i+1), "kind": "renameKey", "fromKey": "env",
			"toKey": "Environment", "fromValue": "Prod", "mergesIntoExisting": false, "impacted": false}}
		if i == 0 {
			// One change to fix by hand, so that the list is there to be left out.
			changes = append(changes, map[string]any{"changeId": "c-000000", "kind": "changeValue", "fromKey": "Owner",
				"fromValue": "team-x", "toValue": "team-b", "impacted": false})
		}
		resources = append(resources, planResource(fmt.Sprintf("arn:aws:sqs:ap-northeast-1:111122223333:q%02d", i),
			beforeOfTheExample(cloudFormationTags(name, logical)), changes))
	}
	w.givenPushed()
	plan := w.givenPlan(resources...)
	body := w.givenBodyFile(strings.Repeat("あ", 60000))

	r := w.iacPR(plan, "--pr-body-file", body)

	r.requireExit(t, 1)
	// The difference and the list are on standard output, as with --write.
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: .+
  Fix: .+$`)
	requireContains(t, "stderr", r.stderr, "longer than 65,536 characters even without the list to fix by hand")
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(w.dir, filepath.FromSlash(file)))
		if err != nil || !strings.Contains(string(data), "Key: Environment") {
			t.Fatalf("%s was not written: %v\n%s", file, err, data)
		}
	}
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		t.Fatalf("the working tree is on %s", branch)
	}
	if branches := strings.TrimSpace(w.out("for-each-ref", "--format=%(refname)", "refs/heads")); branches != "refs/heads/main" {
		t.Fatalf("branches %s", branches)
	}
	if subject := strings.TrimSpace(w.out("log", "-1", "--format=%s")); subject != "sources" {
		t.Fatalf("a commit was made: %s", subject)
	}
	if heads := strings.TrimSpace(gitAt(t, "", "--git-dir", w.bare, "for-each-ref", "--format=%(refname)", "refs/heads")); heads != "refs/heads/main" {
		t.Fatalf("origin holds %s", heads)
	}
	if _, created := w.gh.Find("pr", "create"); created {
		t.Fatal("gh pr create was called")
	}
}
