package pullrequest

// Unit tests of the resume commands (v0.2.0 U2, security design §4, BR4.3,
// NFR1.9, NFR6.15), written after the implementation. The last test pastes the
// lines into the real shells and checks the arguments git and gh receive.

import (
	"encoding/base64"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

func resumePlan() Plan {
	return Plan{RepoRoot: "/w/repo", Branch: "feature/tags", Remote: "origin", RepoPath: "o/r", Base: "main"}
}

func commandsOf(lines []ResumeLine) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Step+": "+l.Command)
	}
	return out
}

// BR4.3: the lines start at the step that failed and go up to the creation;
// the commit is two lines, add and commit, with the committed files only.
func TestResumeCommandsFromTheStepThatFailed(t *testing.T) {
	out := PublishOutcome{FailedStep: StepBranch, CommittedFiles: []string{"a.ts", "b c.ts"}, BodyFile: "/w/repo/.git/tag-refactor/pr-body-1.md"}

	got := commandsOf(ResumeCommands(resumePlan(), "T", out, ShellSh))

	want := []string{
		"branch: git -C '/w/repo' switch -c 'feature/tags'",
		"commit: git -C '/w/repo' --literal-pathspecs add -- 'a.ts' 'b c.ts'",
		"commit: git -C '/w/repo' --literal-pathspecs commit --only -m 'T' -- 'a.ts' 'b c.ts'",
		"push: git -C '/w/repo' push 'origin' 'refs/heads/feature/tags:refs/heads/feature/tags'",
		"create: gh pr create --repo 'o/r' --base 'main' --head 'feature/tags' --title 'T' --body-file '/w/repo/.git/tag-refactor/pr-body-1.md'",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	out.FailedStep = StepPush
	if got := commandsOf(ResumeCommands(resumePlan(), "T", out, ShellSh)); !slices.Equal(got, want[3:]) {
		t.Fatalf("from push: %v", got)
	}
	for _, step := range []string{"", "unknown"} {
		out.FailedStep = step
		if got := ResumeCommands(resumePlan(), "T", out, ShellSh); got != nil {
			t.Fatalf("step %q gives %v", step, got)
		}
	}
}

// NFR1.9, NFR6.15: each value is quoted for the shell, or turned away when it
// cannot be quoted safely; the fixed words are never quoted.
func TestQuoteForEachShell(t *testing.T) {
	cases := []struct {
		value, sh, ps string // empty: the value is turned away
	}{
		{"it's", `'it'\''s'`, `'it''s'`},
		{`say "x"`, `'say "x"'`, ""},
		{"$x $(echo x) `x`", "'$x $(echo x) `x`'", "'$x $(echo x) `x`'"},
		{"‘a’ ‚b‛", "'‘a’ ‚b‛'", "'‘‘a’’ ‚‚b‛‛'"},
		{"a b", "'a b'", "'a b'"},
		{`C:\a b\`, `'C:\a b\'`, ""},
		{`C:\ab\`, `'C:\ab\'`, `'C:\ab\'`},
		{"line\nnext", "", ""},
		{"a\u202eb", "", ""},
		{"a\xffb", "", ""},
		{"-x *.ts [a] :x あ", "'-x *.ts [a] :x あ'", "'-x *.ts [a] :x あ'"},
	}
	for _, tc := range cases {
		for _, sh := range []struct {
			name  string
			quote func(string) (string, bool)
			want  string
		}{{"sh", quoteSh, tc.sh}, {"PowerShell", quotePowerShell, tc.ps}} {
			got, ok := sh.quote(tc.value)
			if ok != (sh.want != "") || got != sh.want {
				t.Errorf("%s: %q gives %q %v, want %q", sh.name, tc.value, got, ok, sh.want)
			}
		}
	}
}

// NFR1.9: a line with a value that cannot be quoted names its values for the
// user to type; the other lines keep their commands.
func TestResumeLineByHand(t *testing.T) {
	out := PublishOutcome{FailedStep: StepCommit, CommittedFiles: []string{"a.ts"}, BodyFile: `C:\w\pr-body-1.md`}

	lines := ResumeCommands(resumePlan(), `fix "tags"`, out, ShellPowerShell)

	if len(lines) != 4 || lines[0].Command == "" || lines[2].Command == "" {
		t.Fatalf("lines %+v", lines)
	}
	wantCommit := []ManualValue{{"root", "/w/repo"}, {"title", `fix "tags"`}, {"file", "a.ts"}}
	if lines[1].Command != "" || !reflect.DeepEqual(lines[1].Manual, wantCommit) {
		t.Fatalf("commit line %+v", lines[1])
	}
	if lines[3].Command != "" || lines[3].Manual[len(lines[3].Manual)-1] != (ManualValue{"bodyFile", `C:\w\pr-body-1.md`}) {
		t.Fatalf("create line %+v", lines[3])
	}
}

// BR4.3: without a body file, the creation is done by hand (gh would ask for
// the body in an editor), even with values that can be quoted.
func TestResumeWithoutABodyFile(t *testing.T) {
	out := PublishOutcome{FailedStep: StepCreate}

	lines := ResumeCommands(resumePlan(), "T", out, ShellSh)

	want := []ManualValue{{"repo", "o/r"}, {"base", "main"}, {"branch", "feature/tags"}, {"title", "T"}}
	if len(lines) != 1 || lines[0].Command != "" || !reflect.DeepEqual(lines[0].Manual, want) {
		t.Fatalf("lines %+v", lines)
	}
}

// BR4.3, review N2: the line that looks for the pull request quotes the
// branch for the shell, or names its values when it cannot.
func TestListCommand(t *testing.T) {
	plan := resumePlan()
	plan.Branch = "feature/it's-$x;y"

	sh := ListCommand(plan, ShellSh)
	ps := ListCommand(plan, ShellPowerShell)

	if sh.Step != StepList || sh.Command != `gh pr list --repo 'o/r' --head 'feature/it'\''s-$x;y'` {
		t.Fatalf("sh %+v", sh)
	}
	if ps.Command != `gh pr list --repo 'o/r' --head 'feature/it''s-$x;y'` {
		t.Fatalf("PowerShell %+v", ps)
	}
	plan.Branch = `feature/"x"`
	byHand := ListCommand(plan, ShellPowerShell)
	if byHand.Command != "" || !reflect.DeepEqual(byHand.Manual, []ManualValue{{"repo", "o/r"}, {"branch", `feature/"x"`}}) {
		t.Fatalf("by hand %+v", byHand)
	}
}

// NFR6.15: PowerShell on Windows unless Git Bash or MSYS2 set MSYSTEM; sh
// elsewhere.
func TestDefaultShell(t *testing.T) {
	env := func(msystem string) func(string) string {
		return func(name string) string {
			if name == "MSYSTEM" {
				return msystem
			}
			return ""
		}
	}
	wantPlain := ShellSh
	if runtime.GOOS == "windows" {
		wantPlain = ShellPowerShell
	}
	if got := DefaultShell(env("")); got != wantPlain {
		t.Fatalf("without MSYSTEM: %v", got)
	}
	if got := DefaultShell(env("MINGW64")); got != ShellSh {
		t.Fatalf("with MSYSTEM: %v", got)
	}
}

// shellRun runs one line in a shell.
type shellRun struct {
	name  string
	sh    Shell
	path  string
	start func(path, line string) *exec.Cmd
}

func powerShellCmd(path, line string) *exec.Cmd {
	u := utf16.Encode([]rune("$ErrorActionPreference = 'Stop'; " + line))
	b := make([]byte, 0, 2*len(u))
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	return exec.Command(path, "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
}

// shells finds the shells to paste the lines into. In CI (CI is set), a shell
// of the platform that is missing fails the test; on a computer of a
// developer, it is skipped.
func shells(t *testing.T) []shellRun {
	t.Helper()
	inCI := os.Getenv("CI") != ""
	var found []shellRun
	look := func(name string, sh Shell, required bool, start func(string, string) *exec.Cmd) {
		path, err := exec.LookPath(name)
		switch {
		case err == nil:
			found = append(found, shellRun{name, sh, path, start})
		case required && inCI:
			t.Fatalf("%s is needed in CI: %v", name, err)
		default:
			t.Logf("%s is not found; its lines are not run: %v", name, err)
		}
	}
	shCmd := func(path, line string) *exec.Cmd { return exec.Command(path, "-c", line) }
	windows := runtime.GOOS == "windows"
	look("sh", ShellSh, !windows, shCmd)
	look("powershell", ShellPowerShell, windows, powerShellCmd)
	look("pwsh", ShellPowerShell, windows, powerShellCmd)
	return found
}

// NFR1.9, NFR6.15: pasted into the real shell, each command gives git and gh
// the values as they are: quotes, $, backquotes, curly quotes, spaces, a
// leading -, wildcards and Japanese.
func TestResumeCommandsInTheShell(t *testing.T) {
	runs := shells(t)
	if len(runs) == 0 {
		t.Skip("no shell is found")
	}
	fake := testutil.InstallFakeArgs(t, "git", "gh")
	plan := Plan{RepoRoot: `C:\w\my repo`, Branch: "feature/タグ", Remote: "origin", RepoPath: "o/r", Base: "main"}
	if runtime.GOOS != "windows" {
		plan.RepoRoot = "/w/my repo"
	}
	title := "It's $x $(echo x) `x` ‘q’ ‛r‚ あ"
	files := []string{"a b.ts", "-x.ts", "*.ts", "[a].ts", ":x.ts"}
	out := PublishOutcome{FailedStep: StepBranch, CommittedFiles: files, BodyFile: plan.RepoRoot + "/.git/tag-refactor/pr-body-x.md"}
	ref := "refs/heads/" + plan.Branch
	want := []testutil.ArgsCall{
		{Name: "git", Args: []string{"-C", plan.RepoRoot, "switch", "-c", plan.Branch}},
		{Name: "git", Args: append([]string{"-C", plan.RepoRoot, "--literal-pathspecs", "add", "--"}, files...)},
		{Name: "git", Args: append([]string{"-C", plan.RepoRoot, "--literal-pathspecs", "commit", "--only", "-m", title, "--"}, files...)},
		{Name: "git", Args: []string{"-C", plan.RepoRoot, "push", "origin", ref + ":" + ref}},
		{Name: "gh", Args: []string{"pr", "create", "--repo", "o/r", "--base", "main", "--head", plan.Branch, "--title", title,
			"--body-file", out.BodyFile}},
	}
	for _, run := range runs {
		t.Run(run.name, func(t *testing.T) {
			fake.Reset()
			for _, l := range ResumeCommands(plan, title, out, run.sh) {
				if l.Command == "" {
					t.Fatalf("a line by hand: %+v", l)
				}
				if output, err := run.start(run.path, l.Command).CombinedOutput(); err != nil {
					t.Fatalf("%s: %v\n%s", l.Command, err, output)
				}
			}
			if got := fake.Calls(); !reflect.DeepEqual(got, want) {
				t.Fatalf("calls\n%q\nwant\n%q", got, want)
			}
		})
	}
}
