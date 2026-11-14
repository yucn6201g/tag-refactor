package pullrequest

// Unit tests of pullrequest, written after the implementation (team testing
// posture). git and gh are stood in for: the answers of a repository on main,
// with origin on github.com and the branch on the remote.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
)

// tools stands in for git and gh.
type tools struct {
	mu      sync.Mutex
	calls   []procexec.Command
	missing map[string]bool
	// change replaces the answer to a command, by its words after -C <dir>.
	change map[string]procexec.Result
	errs   map[string]error
	gitDir string
	bodies []string
}

func newTools(t *testing.T) *tools {
	return &tools{change: map[string]procexec.Result{}, errs: map[string]error{}, gitDir: t.TempDir()}
}

func (f *tools) LookPath(name string) (string, error) {
	if f.missing[name] {
		return "", exec.ErrNotFound
	}
	return "/bin/" + name, nil
}

// words returns the words of a command after its options, as "git ..." or
// "gh ...".
func words(c procexec.Command) string {
	args := c.Args
	for len(args) > 1 && (args[0] == "-c" || args[0] == "-C") {
		args = args[2:]
	}
	if len(args) > 0 && args[0] == "--literal-pathspecs" {
		args = args[1:]
	}
	return filepath.Base(c.Name) + " " + strings.Join(args, " ")
}

func (f *tools) Run(_ context.Context, c procexec.Command) (procexec.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
	w := words(c)
	for prefix, err := range f.errs {
		if strings.HasPrefix(w, prefix) {
			return procexec.Result{}, err
		}
	}
	for prefix, res := range f.change {
		if strings.HasPrefix(w, prefix) {
			return res, nil
		}
	}
	out := func(s string) (procexec.Result, error) { return procexec.Result{Stdout: []byte(s + "\n")}, nil }
	switch {
	case w == "git rev-parse --show-toplevel":
		return out("/repo")
	case w == "git rev-parse --show-prefix":
		return out("infra/")
	case w == "git rev-parse --absolute-git-dir":
		return out(filepath.ToSlash(f.gitDir))
	case w == "git symbolic-ref --quiet --short HEAD":
		return out("main")
	case w == "git config --get branch.main.remote":
		return out("origin")
	case w == "git config --get remote.origin.url":
		return out("https://user:ghp_secret@github.com/o/r.git")
	case strings.HasPrefix(w, "git rev-parse --verify"):
		return procexec.Result{ExitCode: 1}, nil
	case strings.HasPrefix(w, "git ls-remote"):
		return out("0123abcd\trefs/heads/main")
	case strings.HasPrefix(w, "gh pr create"):
		body, _ := os.ReadFile(c.Args[len(c.Args)-1])
		f.bodies = append(f.bodies, string(body))
		return out("Creating pull request...\nhttps://github.com/o/r/pull/9")
	}
	return procexec.Result{}, nil
}

func (f *tools) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, words(c))
	}
	return out
}

func (f *tools) find(prefix string) (procexec.Command, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(words(c), prefix) {
			return c, true
		}
	}
	return procexec.Command{}, false
}

// steps keeps the steps a sink got.
type steps struct{ lines []string }

func (s *steps) Step(step, status, detail string) {
	s.lines = append(s.lines, step+" "+status+" "+detail)
}

var now = time.Date(2026, 11, 8, 18, 30, 5, 0, time.Local)

func publisher(f *tools) *Publisher {
	return &Publisher{Runner: f, Environ: func() []string {
		return []string{"PATH=/bin", "GIT_DIR=/elsewhere", "GH_TOKEN=ghp_user"}
	}, TermIn: strings.NewReader(""), TermErr: &bytes.Buffer{}}
}

func check(f *tools, title string) (Plan, error) {
	return publisher(f).Check(context.Background(), CheckRequest{Dir: "/repo/infra", Title: title, Now: now}, nil)
}

// BR4.1, contract C1: Check decides the remote, the branches and the places of
// the repository, says the checks passed, and runs git that reads without the
// GIT_ variables of the user and without asking.
func TestCheckDecidesThePlan(t *testing.T) {
	f := newTools(t)
	sink := &steps{}

	plan, err := publisher(f).Check(context.Background(), CheckRequest{Dir: "/repo/infra", Title: "T", Now: now}, sink)

	if err != nil {
		t.Fatal(err)
	}
	want := Plan{Remote: "origin", RemoteURL: "https://github.com/o/r.git", Host: "github.com", RepoPath: "o/r", Base: "main",
		Branch: "tag-refactor/iac-20261108-183005", Title: "T", RepoRoot: "/repo", Prefix: "infra/", GitDir: filepath.ToSlash(f.gitDir)}
	if plan != want {
		t.Fatalf("plan %+v\nwant %+v", plan, want)
	}
	if strings.Join(sink.lines, "|") != "checked done tag-refactor/iac-20261108-183005" {
		t.Fatalf("sink %v", sink.lines)
	}
	c, _ := f.find("git rev-parse --show-toplevel")
	if !slices.Contains(c.Env, "GIT_TERMINAL_PROMPT=0") || slices.Contains(c.Env, "GIT_DIR=/elsewhere") || c.Timeout != DefaultTimeouts.Read {
		t.Fatalf("read git: %+v", c)
	}
	gh, _ := f.find("gh auth status")
	if !slices.Contains(gh.Env, "GH_TOKEN=ghp_user") || gh.Timeout != DefaultTimeouts.GH {
		t.Fatalf("gh does not get the environment of the user: %+v", gh)
	}
	for _, w := range f.recorded() {
		if strings.HasPrefix(w, "git switch") || strings.HasPrefix(w, "git commit") || strings.HasPrefix(w, "git push") {
			t.Fatalf("Check changed the repository: %s", w)
		}
	}
}

// BR4.1: each check that does not pass stops Check, with the message of the
// catalog and nothing changed.
func TestCheckStopsAtTheFirstProblem(t *testing.T) {
	failed := procexec.Result{ExitCode: 1}
	cases := []struct {
		name  string
		title string
		set   func(*tools)
		want  string
	}{
		{"no git", "T", func(f *tools) { f.missing = map[string]bool{"git": true} }, "iac.prNoGit"},
		{"not a repository", "T", func(f *tools) { f.change["git rev-parse --show-toplevel"] = failed }, "iac.prNotRepo"},
		{"detached", "T", func(f *tools) { f.change["git symbolic-ref"] = failed }, "iac.prDetached"},
		{"merge", "T", func(f *tools) { writeFile(t, filepath.Join(f.gitDir, "MERGE_HEAD")) }, "iac.prInProgress"},
		{"rebase", "T", func(f *tools) { writeFile(t, filepath.Join(f.gitDir, "rebase-merge", "x")) }, "iac.prInProgress"},
		{"no remote", "T", func(f *tools) { f.change["git config --get remote.origin.url"] = failed }, "iac.prNoRemote"},
		{"not GitHub", "T", func(f *tools) {
			f.change["git config --get remote.origin.url"] = procexec.Result{Stdout: []byte("https://gitlab.com/o/r.git")}
			f.change["git remote get-url"] = procexec.Result{Stdout: []byte("https://gitlab.com/o/r.git")}
		}, "iac.prNotGitHub"},
		{"bad branch", "T", func(f *tools) { f.change["git check-ref-format"] = failed }, "iac.prBadBranch"},
		{"branch exists", "T", func(f *tools) { f.change["git rev-parse --verify"] = procexec.Result{} }, "iac.prBranchExists"},
		{"empty title", " ", func(*tools) {}, "iac.prBadTitle"},
		{"two lines", "a\nb", func(*tools) {}, "iac.prBadTitle"},
		{"long title", strings.Repeat("あ", 257), func(*tools) {}, "iac.prBadTitle"},
		{"no gh", "T", func(f *tools) { f.missing = map[string]bool{"gh": true} }, "iac.prNoGh"},
		{"not logged in", "T", func(f *tools) { f.change["gh auth status"] = failed }, "iac.prNotLoggedIn"},
		{"remote unreachable", "T", func(f *tools) { f.change["git ls-remote"] = failed }, "iac.prRemoteUnreachable"},
		{"base not on the remote", "T", func(f *tools) { f.change["git ls-remote"] = procexec.Result{} }, "iac.prBaseNotOnRemote"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newTools(t)
			tc.set(f)

			_, err := check(f, tc.title)

			e, ok := errkind.As(err)
			if !ok || e.Kind != errkind.InvalidInput || e.DetailID != tc.want || e.HintID != "iac.prCheckFix" {
				t.Fatalf("error %v, want %s", err, tc.want)
			}
		})
	}
	t.Run("a remote that git rewrites to GitHub", func(t *testing.T) {
		f := newTools(t)
		f.change["git config --get remote.origin.url"] = procexec.Result{Stdout: []byte("gh:o/r")}
		f.change["git remote get-url"] = procexec.Result{Stdout: []byte("git@github.com:o/r.git")}

		plan, err := check(f, "T")

		if err != nil || plan.RepoPath != "o/r" || plan.RemoteURL != "gh:o/r" {
			t.Fatalf("plan %+v, %v", plan, err)
		}
	})
}

func writeFile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubRepo(t *testing.T) {
	for u, want := range map[string]string{
		"https://github.com/o/r.git":          "o/r",
		"https://user:tok@github.com/o/r":     "o/r",
		"https://GitHub.com/o/r/":             "o/r",
		"ssh://git@github.com/o/r.git":        "o/r",
		"ssh://git@github.com:22/o/r.git":     "o/r",
		"git@github.com:o/r.git":              "o/r",
		"https://gitlab.com/o/r.git":          "",
		"https://github.com/o":                "",
		"https://github.com/o/r/x":            "",
		"C:/repos/origin.git":                 "",
		"/srv/git/origin.git":                 "",
		"git@github.example.com:o/r.git":      "",
		"https://github.com.evil.example/o/r": "",
	} {
		_, got, ok := GitHubRepo(u)
		if got != want || ok != (want != "") {
			t.Errorf("GitHubRepo(%q) = %q, %v; want %q", u, got, ok, want)
		}
	}
}

func publish(t *testing.T, f *tools, in PublishInput) (PublishOutcome, *steps) {
	t.Helper()
	plan, err := check(f, "Reflect tag changes from tag-refactor (plan.json)")
	if err != nil {
		t.Fatal(err)
	}
	sink := &steps{}
	return publisher(f).Publish(context.Background(), plan, in, sink), sink
}

// BR4.2, BR4.5, contract C1: Publish makes the branch, commits the written files
// only with the terminal, pushes, and opens the pull request with the body in a
// file of the folder of git, which goes afterwards.
func TestPublishMakesEveryStep(t *testing.T) {
	f := newTools(t)

	out, sink := publish(t, f, PublishInput{Files: []string{"lib/stack.ts"}, Title: "Reflect (plan.json)", Body: "body\n"})

	if out.FailedStep != "" || out.URL != "https://github.com/o/r/pull/9" || out.BodyFile != "" ||
		!slices.Equal(out.CompletedSteps, []string{StepBranch, StepCommit, StepPush, StepCreate}) ||
		!slices.Equal(out.CommittedFiles, []string{"infra/lib/stack.ts"}) {
		t.Fatalf("outcome %+v", out)
	}
	var steps []string
	for _, w := range f.recorded() {
		if !strings.HasPrefix(w, "git rev-parse") && !strings.HasPrefix(w, "git symbolic-ref") && !strings.HasPrefix(w, "git config") &&
			!strings.HasPrefix(w, "git check-ref-format") && !strings.HasPrefix(w, "git ls-remote") && !strings.HasPrefix(w, "gh auth") {
			steps = append(steps, w)
		}
	}
	branch := "tag-refactor/iac-20261108-183005"
	want := []string{"git switch -c " + branch, "git add -- infra/lib/stack.ts",
		"git commit --only -m Reflect (plan.json) -- infra/lib/stack.ts",
		"git push origin refs/heads/" + branch + ":refs/heads/" + branch}
	if len(steps) != 5 || !slices.Equal(steps[:4], want) || !strings.HasPrefix(steps[4],
		"gh pr create --repo o/r --base main --head "+branch+" --title Reflect (plan.json) --body-file ") {
		t.Fatalf("steps:\n%s", strings.Join(steps, "\n"))
	}
	for _, step := range []string{"git commit", "git push"} {
		c, _ := f.find(step)
		if !c.Interactive || c.TermIn == nil || c.TermErr == nil || slices.Contains(c.Env, "GIT_TERMINAL_PROMPT=0") ||
			slices.Contains(c.Env, "GIT_DIR=/elsewhere") || c.Timeout != DefaultTimeouts.Commit {
			t.Errorf("%s: %+v", step, c)
		}
	}
	create, _ := f.find("gh pr create")
	bodyFile := create.Args[len(create.Args)-1]
	if !strings.HasPrefix(bodyFile, filepath.Join(f.gitDir, "tag-refactor")) || f.bodies[0] != "body\n" {
		t.Fatalf("body file %s with %q", bodyFile, f.bodies)
	}
	if _, err := os.Stat(bodyFile); !os.IsNotExist(err) {
		t.Fatal("the body file was left")
	}
	if len(sink.lines) != 8 || sink.lines[7] != "create done https://github.com/o/r/pull/9" {
		t.Fatalf("sink %v", sink.lines)
	}
}

// BR4.2, NFR4.4: a step that fails stops Publish there; the steps done stay
// done and the outcome names them.
func TestPublishStopsAtAFailedStep(t *testing.T) {
	f := newTools(t)
	f.change["git push"] = procexec.Result{ExitCode: 1}

	out, sink := publish(t, f, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "b"})

	if out.FailedStep != StepPush || !slices.Equal(out.CompletedSteps, []string{StepBranch, StepCommit}) {
		t.Fatalf("outcome %+v", out)
	}
	if e, ok := errkind.As(out.Err); !ok || e.DetailID != "iac.prStepFailed" || e.DetailArgs[0] != StepPush {
		t.Fatalf("error %v", out.Err)
	}
	if _, created := f.find("gh pr create"); created {
		t.Fatal("gh pr create ran after a failed push")
	}
	if sink.lines[len(sink.lines)-1] != "push failed "+out.Branch {
		t.Fatalf("sink %v", sink.lines)
	}
}

// Contract C1 body_file: when gh fails, the body stays in its file and the
// outcome says where.
func TestPublishKeepsTheBodyWhenCreateFails(t *testing.T) {
	f := newTools(t)
	f.change["gh pr create"] = procexec.Result{ExitCode: 1, Stderr: []byte("GraphQL error")}

	out, _ := publish(t, f, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "the body"})

	if out.FailedStep != StepCreate || out.BodyFile == "" {
		t.Fatalf("outcome %+v", out)
	}
	if data, err := os.ReadFile(out.BodyFile); err != nil || string(data) != "the body" {
		t.Fatalf("body file: %q, %v", data, err)
	}
}

// NFR3.3: a step past its limit fails as a time limit; a stop by the user is the
// error of the context.
func TestPublishTimeoutsAndCancel(t *testing.T) {
	f := newTools(t)
	f.errs["git commit"] = procexec.ErrTimeout

	out, _ := publish(t, f, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "b"})

	if e, ok := errkind.As(out.Err); !ok || e.Kind != errkind.Transient || e.Reason != errkind.ReasonTimeout || out.FailedStep != StepCommit {
		t.Fatalf("outcome %+v", out)
	}
	g := newTools(t)
	plan, err := check(g, "T")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g.errs["git switch"] = context.Canceled

	stopped := publisher(g).Publish(ctx, plan, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "b"}, nil)

	if !errors.Is(stopped.Err, context.Canceled) || stopped.FailedStep != StepBranch {
		t.Fatalf("outcome %+v", stopped)
	}
}

// NFR1.2, BR4.3, BR4.4: the title, the commit message and the body are redacted.
func TestPublishRedactsWhatItBuilds(t *testing.T) {
	f := newTools(t)

	publish(t, f, PublishInput{Files: []string{"a.ts"}, Title: "Reflect (plan-123456789012.json)",
		Body: "see arn:aws:s3:::secret and AKIAABCDEFGHIJKLMNOP"})

	commit, _ := f.find("git commit")
	create, _ := f.find("gh pr create")
	for _, text := range []string{strings.Join(commit.Args, " "), strings.Join(create.Args, " "), f.bodies[0]} {
		for _, secret := range []string{"123456789012", "arn:aws", "AKIA"} {
			if strings.Contains(text, secret) {
				t.Errorf("%q holds %s", text, secret)
			}
		}
	}
}

// Review 1: git commit and git push keep the variables of the user's
// authentication; every git drops the variables that point it at another
// repository, and git that only reads keeps no GIT_ variable.
func TestGitEnvironments(t *testing.T) {
	parent := []string{"PATH=/bin", "SSH_ASKPASS=/bin/ask", "GIT_SSH_COMMAND=ssh -i key", "GIT_SSH=/bin/ssh",
		"GIT_SSH_VARIANT=ssh", "GIT_ASKPASS=/bin/askpass", "GIT_DIR=/elsewhere", "GIT_WORK_TREE=/w", "GIT_INDEX_FILE=/i",
		"GIT_OBJECT_DIRECTORY=/o", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/a", "GIT_CEILING_DIRECTORIES=/c", "GIT_NAMESPACE=n",
		"GIT_CONFIG_GLOBAL=/g", "git_dir=/lower"}
	kept := func(env []string) []string {
		var out []string
		for _, kv := range env {
			if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(strings.ToUpper(name), "GIT_") || name == "SSH_ASKPASS" {
				out = append(out, kv)
			}
		}
		return out
	}

	talk, read := gitEnv(parent, true, false), gitEnv(parent, true, true)

	want := []string{"SSH_ASKPASS=/bin/ask", "GIT_SSH_COMMAND=ssh -i key", "GIT_SSH=/bin/ssh", "GIT_SSH_VARIANT=ssh",
		"GIT_ASKPASS=/bin/askpass", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1"}
	if got := kept(talk); !slices.Equal(got, want) {
		t.Fatalf("commit and push get %v\nwant %v", got, want)
	}
	if got := kept(read); !slices.Equal(got, []string{"SSH_ASKPASS=/bin/ask", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1",
		"GIT_TERMINAL_PROMPT=0"}) {
		t.Fatalf("git that reads gets %v", got)
	}
	// On Unix a name in lower case is another variable, which git does not
	// read: it stays, as in the git check of iac.
	if got := kept(gitEnv([]string{"git_ssh_command=x", "GIT_SSH_COMMAND=y"}, false, false)); !slices.Equal(got,
		[]string{"git_ssh_command=x", "GIT_SSH_COMMAND=y", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1"}) {
		t.Fatalf("Unix: %v", got)
	}
}

func TestDefaultBranch(t *testing.T) {
	if got := DefaultBranch(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)); got != "tag-refactor/iac-20260102-030405" {
		t.Fatalf("DefaultBranch = %s", got)
	}
}
