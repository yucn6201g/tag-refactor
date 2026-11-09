// Package pullrequest opens a pull request of the files iac wrote (v0.2.0 U1,
// contract C1). Check runs before anything is written: it decides the
// repository, the remote, the branch the request goes to and the new branch,
// and checks that git and gh can do the work, the checks that need no network
// first. Publish runs after the writing: it makes the branch, commits the
// written files only, pushes, and asks gh to open the pull request. A step that
// fails stops it there; nothing done before is undone (BR4.2).
//
// git and gh get the environment of the user: their credentials are theirs,
// and this package never reads or prints a token (NFR1.1, contract C5). git
// gets it without the GIT_ variables, as the git check of iac does, except that
// commit and push keep the ones of the user's authentication (authVariables);
// commit and push talk with the terminal, for the hooks and the signing of the
// user.
package pullrequest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// The steps of a pull request, as ProgressSink receives them (contract C6).
const (
	StepChecked = "checked"
	StepBranch  = "branch"
	StepCommit  = "commit"
	StepPush    = "push"
	StepCreate  = "create"
)

// The states of a step.
const (
	StatusStarted = "started"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// ProgressSink receives the steps (contract C6). The detail is the branch or
// the URL, never a token or the body. display.Printer implements it.
type ProgressSink interface {
	Step(step, status, detail string)
}

// nopSink shows nothing.
type nopSink struct{}

func (nopSink) Step(string, string, string) {}

// Timeouts are the limits of the steps (NFR3.3, contract C5).
type Timeouts struct {
	// Read is for git that only reads, git switch -c and git add.
	Read time.Duration
	// Commit is for git commit and git push, which run the user's hooks.
	Commit time.Duration
	// GH is for gh.
	GH time.Duration
}

// DefaultTimeouts are the limits of contract C5.
var DefaultTimeouts = Timeouts{Read: 30 * time.Second, Commit: 10 * time.Minute, GH: 2 * time.Minute}

// maxTitle is the longest title, in characters (X1).
const maxTitle = 256

// CheckRequest is what Check is asked (contract C1).
type CheckRequest struct {
	// Dir is --dir.
	Dir string
	// BranchName is the new branch; empty means the default name of Now.
	BranchName string
	// Title is the title of the pull request and the commit message.
	Title string
	// BodyFile is --pr-body-file (U2); empty in U1.
	BodyFile string
	Now      time.Time
}

// Plan is what Check decided (contract C1).
type Plan struct {
	Remote string
	// RemoteURL is the URL of the remote without its user information.
	RemoteURL string
	// Host is github.com; RepoPath is owner/name.
	Host, RepoPath string
	// Base is the branch iac ran on, which the pull request goes to.
	Base   string
	Branch string
	Title  string
	// UserBody is the content of --pr-body-file (U2).
	UserBody string
	// RepoRoot is the top of the working tree, Prefix the place of --dir in
	// it (empty or ending with /), GitDir the folder of git.
	RepoRoot, Prefix, GitDir string
}

// PublishInput is what Publish commits and opens (contract C1).
type PublishInput struct {
	// Files are the files iac wrote, relative to --dir with /.
	Files []string
	Title string
	Body  string
}

// PublishOutcome is what Publish did (contract C1).
type PublishOutcome struct {
	Remote, Base, Branch string
	// CommittedFiles are relative to the top of the working tree.
	CommittedFiles []string
	CompletedSteps []string
	// FailedStep is the step that failed; empty when every step was done.
	FailedStep string
	URL        string
	// BodyFile is where the body stays when the request was not opened.
	BodyFile string
	// ResumeCommands are for U2.
	ResumeCommands []string
	// Err is the cause, with a kind; it holds no token.
	Err error
}

// Publisher checks and opens pull requests (PullRequestPublisher).
type Publisher struct {
	Runner procexec.Runner
	// Environ is the environment of the user, given to git and gh.
	Environ func() []string
	// TermIn and TermErr are the terminal of git commit and git push.
	TermIn  io.Reader
	TermErr io.Writer
	// Timeouts are the limits; the zero value means DefaultTimeouts.
	Timeouts Timeouts
	// Debug receives the lines for --debug; nil drops them.
	Debug func(string)
}

func (p *Publisher) runner() procexec.Runner {
	if p.Runner == nil {
		return procexec.System{}
	}
	return p.Runner
}

func (p *Publisher) environ() []string {
	if p.Environ == nil {
		return os.Environ()
	}
	return p.Environ()
}

func (p *Publisher) timeouts() Timeouts {
	if p.Timeouts == (Timeouts{}) {
		return DefaultTimeouts
	}
	return p.Timeouts
}

func (p *Publisher) debug(line string) {
	if p.Debug != nil {
		p.Debug(redact.String(line))
	}
}

// stop is the error of a check that did not pass: nothing was changed.
func stop(detailID string, args ...any) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--pr",
		DetailID: detailID, DetailArgs: args, HintID: "iac.prCheckFix"}
}

// DefaultBranch is the name of the new branch at now (contract X1).
func DefaultBranch(now time.Time) string {
	return "tag-refactor/iac-" + now.Format("20060102-150405")
}

// gitCaller runs git for one repository.
type gitCaller struct {
	p    *Publisher
	git  string
	dir  string
	read []string
	talk []string
}

func (p *Publisher) newGit(dir string) (*gitCaller, error) {
	git, err := p.runner().LookPath("git")
	if err != nil {
		return nil, stop("iac.prNoGit")
	}
	windows := runtime.GOOS == "windows"
	env := p.environ()
	return &gitCaller{p: p, git: git, dir: dir, read: gitEnv(env, windows, true), talk: gitEnv(env, windows, false)}, nil
}

// out runs git that only reads and returns what it printed, trimmed.
func (g *gitCaller) out(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"-c", "core.fsmonitor=false", "-C", g.dir}, args...)
	g.p.debug("git " + strings.Join(args, " "))
	res, err := g.p.runner().Run(ctx, procexec.Command{Name: g.git, Args: full, Env: g.read, Timeout: g.p.timeouts().Read})
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git %s ended with %d", args[0], res.ExitCode)
	}
	if err != nil {
		g.p.debug("git " + args[0] + ": " + strings.TrimSpace(string(res.Stderr)))
		return "", err
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

// authVariables are the GIT_ variables git commit and git push keep: they say
// how the user authenticates (ssh and the asking of a password), and point git
// at no other repository. Every other GIT_ variable goes (GIT_DIR,
// GIT_WORK_TREE, GIT_INDEX_FILE, GIT_OBJECT_DIRECTORY, GIT_NAMESPACE and the
// others could point git elsewhere).
var authVariables = []string{"GIT_SSH", "GIT_SSH_COMMAND", "GIT_SSH_VARIANT", "GIT_ASKPASS"}

// gitEnv is the environment of git: without the GIT_ variables, which could
// point it at another repository (on Windows names have no case), and with
// what keeps it from writing the index on its own and from reading paths as
// patterns. git that only reads does not ask on the terminal and keeps no GIT_
// variable; git that talks with the terminal (noPrompt false: commit and push)
// keeps the variables of authVariables.
func gitEnv(environ []string, windows, noPrompt bool) []string {
	env := make([]string, 0, len(environ)+3)
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if windows {
			name = strings.ToUpper(name)
		}
		if !strings.HasPrefix(name, "GIT_") || !noPrompt && slices.Contains(authVariables, name) {
			env = append(env, kv)
		}
	}
	env = append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1")
	if noPrompt {
		env = append(env, "GIT_TERMINAL_PROMPT=0")
	}
	return env
}

// Check decides the plan of the pull request and checks it before anything is
// written (BR4.1, contract C1). The first check that does not pass stops it.
func (p *Publisher) Check(ctx context.Context, req CheckRequest, sink ProgressSink) (Plan, error) {
	if sink == nil {
		sink = nopSink{}
	}
	plan, err := p.checkLocal(ctx, req)
	if err != nil {
		return Plan{}, err
	}
	if err := p.checkNetwork(ctx, plan); err != nil {
		return Plan{}, err
	}
	sink.Step(StepChecked, StatusDone, plan.Branch)
	return plan, nil
}

// checkLocal does the checks that need no network.
func (p *Publisher) checkLocal(ctx context.Context, req CheckRequest) (Plan, error) {
	if err := checkTitle(req.Title); err != nil {
		return Plan{}, err
	}
	g, err := p.newGit(req.Dir)
	if err != nil {
		return Plan{}, err
	}
	var plan Plan
	if plan.RepoRoot, err = g.out(ctx, "rev-parse", "--show-toplevel"); err != nil {
		return Plan{}, ctxOr(ctx, stop("iac.prNotRepo", req.Dir))
	}
	if plan.Prefix, err = g.out(ctx, "rev-parse", "--show-prefix"); err != nil {
		return Plan{}, ctxOr(ctx, stop("iac.prNotRepo", req.Dir))
	}
	if plan.GitDir, err = g.out(ctx, "rev-parse", "--absolute-git-dir"); err != nil {
		return Plan{}, ctxOr(ctx, stop("iac.prNotRepo", req.Dir))
	}
	if plan.Base, err = g.out(ctx, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil || plan.Base == "" {
		return Plan{}, ctxOr(ctx, stop("iac.prDetached"))
	}
	if what := inProgress(plan.GitDir); what != "" {
		return Plan{}, stop("iac.prInProgress", what)
	}
	if err := p.decideRemote(ctx, g, &plan); err != nil {
		return Plan{}, err
	}
	plan.Branch = req.BranchName
	if plan.Branch == "" {
		plan.Branch = DefaultBranch(req.Now)
	}
	if _, err := g.out(ctx, "check-ref-format", "--branch", plan.Branch); err != nil {
		return Plan{}, ctxOr(ctx, stop("iac.prBadBranch", plan.Branch))
	}
	if _, err := g.out(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+plan.Branch); err == nil {
		return Plan{}, stop("iac.prBranchExists", plan.Branch)
	}
	plan.Title = req.Title
	return plan, ctx.Err()
}

func ctxOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func checkTitle(title string) error {
	if strings.TrimSpace(title) == "" || strings.ContainsAny(title, "\r\n") || utf8.RuneCountInString(title) > maxTitle {
		return stop("iac.prBadTitle")
	}
	return nil
}

// inProgress names an operation of git that is not finished, or nothing.
func inProgress(gitDir string) string {
	for _, m := range []struct{ file, what string }{
		{"MERGE_HEAD", "merge"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"},
		{"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"},
	} {
		if _, err := os.Lstat(filepath.Join(filepath.FromSlash(gitDir), m.file)); err == nil {
			return m.what
		}
	}
	return ""
}

// decideRemote takes the remote the branch follows, else origin, and checks
// that it is on GitHub: by the URL as configured, or as git rewrites it.
func (p *Publisher) decideRemote(ctx context.Context, g *gitCaller, plan *Plan) error {
	remote, _ := g.out(ctx, "config", "--get", "branch."+plan.Base+".remote")
	if remote == "" || remote == "." {
		remote = "origin"
	}
	raw, err := g.out(ctx, "config", "--get", "remote."+remote+".url")
	if err != nil || raw == "" {
		return ctxOr(ctx, stop("iac.prNoRemote", remote))
	}
	plan.Remote, plan.RemoteURL = remote, redact.URL(raw)
	host, repoPath, ok := GitHubRepo(raw)
	if !ok {
		if expanded, err := g.out(ctx, "remote", "get-url", remote); err == nil {
			host, repoPath, ok = GitHubRepo(expanded)
		}
	}
	if !ok {
		return ctxOr(ctx, stop("iac.prNotGitHub", remote, plan.RemoteURL))
	}
	plan.Host, plan.RepoPath = host, repoPath
	return nil
}

var (
	scpLike = regexp.MustCompile(`^(?:[^@/]+@)?([A-Za-z0-9.-]+):([^/].*)$`)
	urlLike = regexp.MustCompile(`^(?:https?|ssh|git)://(?:[^@/]+@)?([A-Za-z0-9.-]+)(?::\d+)?/(.+)$`)
)

// GitHubRepo reads owner/name from a URL of github.com: https, ssh or the
// scp-like form of ssh.
func GitHubRepo(u string) (host, repoPath string, ok bool) {
	var m []string
	switch {
	case urlLike.MatchString(u):
		m = urlLike.FindStringSubmatch(u)
	case scpLike.MatchString(u) && strings.Contains(u, "@"):
		m = scpLike.FindStringSubmatch(u)
	default:
		return "", "", false
	}
	if !strings.EqualFold(m[1], "github.com") {
		return "", "", false
	}
	p := strings.TrimSuffix(strings.TrimSuffix(m[2], "/"), ".git")
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return "github.com", p, true
}

// checkNetwork asks gh whether it is logged in, and the remote whether it has
// the branch the pull request goes to.
func (p *Publisher) checkNetwork(ctx context.Context, plan Plan) error {
	gh, err := p.runner().LookPath("gh")
	if err != nil {
		return stop("iac.prNoGh")
	}
	p.debug("gh auth status --hostname " + plan.Host)
	res, err := p.runner().Run(ctx, procexec.Command{Name: gh, Args: []string{"auth", "status", "--hostname", plan.Host},
		Dir: plan.RepoRoot, Env: p.environ(), Timeout: p.timeouts().GH})
	if err != nil || res.ExitCode != 0 {
		p.debug("gh auth status: " + strings.TrimSpace(string(res.Stderr)))
		return ctxOr(ctx, stop("iac.prNotLoggedIn", plan.Host))
	}
	g, err := p.newGit(plan.RepoRoot)
	if err != nil {
		return err
	}
	heads, err := g.out(ctx, "ls-remote", "--heads", plan.Remote, "refs/heads/"+plan.Base)
	if err != nil {
		return ctxOr(ctx, stop("iac.prRemoteUnreachable", plan.Remote))
	}
	if heads == "" {
		return stop("iac.prBaseNotOnRemote", plan.Base, plan.Remote)
	}
	return nil
}

// Publish makes the branch, the commit, the push and the pull request, in this
// order, and stops at the first step that fails (BR4.2, contract C1). It is
// called only with files written.
func (p *Publisher) Publish(ctx context.Context, plan Plan, in PublishInput, sink ProgressSink) PublishOutcome {
	if sink == nil {
		sink = nopSink{}
	}
	out := PublishOutcome{Remote: plan.Remote, Base: plan.Base, Branch: plan.Branch}
	for _, f := range in.Files {
		out.CommittedFiles = append(out.CommittedFiles, path.Join(plan.Prefix, f))
	}
	g, err := p.newGit(plan.RepoRoot)
	if err != nil {
		out.FailedStep, out.Err = StepBranch, err
		return out
	}
	title, body := redact.String(in.Title), redact.String(in.Body)
	steps := []struct {
		name string
		run  func() error
	}{
		{StepBranch, func() error { return p.branch(ctx, g, plan) }},
		{StepCommit, func() error { return p.commit(ctx, g, out.CommittedFiles, title) }},
		{StepPush, func() error { return p.push(ctx, g, plan) }},
		{StepCreate, func() error { return p.create(ctx, plan, title, body, &out) }},
	}
	for _, s := range steps {
		sink.Step(s.name, StatusStarted, plan.Branch)
		if err := s.run(); err != nil {
			out.FailedStep, out.Err = s.name, stepError(ctx, s.name, err)
			sink.Step(s.name, StatusFailed, plan.Branch)
			return out
		}
		out.CompletedSteps = append(out.CompletedSteps, s.name)
		detail := plan.Branch
		if s.name == StepCreate {
			detail = out.URL
		}
		sink.Step(s.name, StatusDone, detail)
	}
	return out
}

// stepError gives the failure of a step its kind: a time limit, a stop by the
// user, or a failure.
func stepError(ctx context.Context, step string, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, procexec.ErrTimeout):
		return &errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout, Op: step, Subject: "--pr",
			DetailID: "iac.prStepTimeout", DetailArgs: []any{step}, HintID: "iac.prStepFix", Cause: err}
	}
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Op: step, Subject: "--pr",
		DetailID: "iac.prStepFailed", DetailArgs: []any{step}, HintID: "iac.prStepFix", Cause: err}
}

func (p *Publisher) branch(ctx context.Context, g *gitCaller, plan Plan) error {
	_, err := g.out(ctx, "switch", "-c", plan.Branch)
	return err
}

// commit stages the files and commits them alone: with --only, what the user
// staged before stays staged and out of the commit (FR1.8).
func (p *Publisher) commit(ctx context.Context, g *gitCaller, files []string, title string) error {
	add := append([]string{"--literal-pathspecs", "add", "--"}, files...)
	if _, err := g.out(ctx, add...); err != nil {
		return err
	}
	args := append([]string{"-C", g.dir, "--literal-pathspecs", "commit", "--only", "-m", title, "--"}, files...)
	return p.talk(ctx, g, args)
}

func (p *Publisher) push(ctx context.Context, g *gitCaller, plan Plan) error {
	return p.talk(ctx, g, []string{"-C", g.dir, "push", plan.Remote, "refs/heads/" + plan.Branch + ":refs/heads/" + plan.Branch})
}

// talk runs git with the terminal, for the hooks and the signing of the user.
func (p *Publisher) talk(ctx context.Context, g *gitCaller, args []string) error {
	p.debug("git " + strings.Join(args[2:], " "))
	res, err := p.runner().Run(ctx, procexec.Command{Name: g.git, Args: args, Env: g.talk, Timeout: p.timeouts().Commit,
		Interactive: true, TermIn: p.TermIn, TermErr: p.TermErr})
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git ended with %d", res.ExitCode)
	}
	return err
}

// create writes the body to a file in the folder of git, which no commit takes,
// and asks gh to open the pull request with it (contract C1 body_file). The
// file goes once the request is open, and stays to be used again otherwise.
func (p *Publisher) create(ctx context.Context, plan Plan, title, body string, out *PublishOutcome) error {
	gh, err := p.runner().LookPath("gh")
	if err != nil {
		return err
	}
	bodyFile, err := writeBody(plan, body)
	if err != nil {
		return err
	}
	out.BodyFile = bodyFile
	args := []string{"pr", "create", "--repo", plan.RepoPath, "--base", plan.Base, "--head", plan.Branch,
		"--title", title, "--body-file", bodyFile}
	p.debug("gh pr create --repo " + plan.RepoPath + " --base " + plan.Base + " --head " + plan.Branch)
	res, err := p.runner().Run(ctx, procexec.Command{Name: gh, Args: args, Dir: plan.RepoRoot, Env: p.environ(),
		Timeout: p.timeouts().GH})
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("gh ended with %d", res.ExitCode)
	}
	if err != nil {
		p.debug("gh pr create: " + strings.TrimSpace(string(res.Stderr)))
		return err
	}
	for _, line := range strings.Split(string(res.Stdout), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "https://") {
			out.URL = line
		}
	}
	if err := os.Remove(bodyFile); err == nil {
		out.BodyFile = ""
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func writeBody(plan Plan, body string) (string, error) {
	dir := filepath.Join(filepath.FromSlash(plan.GitDir), "tag-refactor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file := filepath.Join(dir, "pr-body-"+unsafeName.ReplaceAllString(plan.Branch, "-")+".md")
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		return "", err
	}
	return file, nil
}
