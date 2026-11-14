package pullrequest

// Unit tests of the checks and the body file of v0.2.0 U2, written after the
// implementation: the redaction of the title, the body of --pr-body-file, the
// name of --pr-branch, the default branch in UTC, the branch on the remote and
// the body file. The shapes of tokens are put together at run time.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

func checkWith(f *tools, req CheckRequest) (Plan, error) {
	req.Dir, req.Now = "/repo/infra", now
	if req.Title == "" {
		req.Title = "T"
	}
	return publisher(f).Check(context.Background(), req, nil)
}

func detailOf(t *testing.T, err error) string {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok || e.Kind != errkind.InvalidInput {
		t.Fatalf("error %v is not an input error", err)
	}
	return e.DetailID
}

// NFR1.8, BR1.1: the title is redacted first, and the redacted title is the one
// checked against the 256 characters and kept in the plan.
func TestCheckRedactsTheTitle(t *testing.T) {
	arn := " arn:aws:s3:::" + strings.Repeat("b", 40)
	plan, err := checkWith(newTools(t), CheckRequest{Title: strings.Repeat("あ", 245) + arn})
	if err != nil || !plan.TitleRedacted || plan.Title != strings.Repeat("あ", 245)+" <redacted>" {
		t.Fatalf("plan %q %v, %v", plan.Title, plan.TitleRedacted, err)
	}
	if _, err := checkWith(newTools(t), CheckRequest{Title: strings.Repeat("あ", 246) + arn}); detailOf(t, err) != "iac.prBadTitle" {
		t.Fatalf("a title of 257 characters after the redaction: %v", err)
	}
	if plan, err := checkWith(newTools(t), CheckRequest{Title: "plain"}); err != nil || plan.TitleRedacted || plan.Title != "plain" {
		t.Fatalf("plan %+v, %v", plan, err)
	}
}

func bodyFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// BR1.1, NFR6.14: the body file must be a regular file of UTF-8, up to 1 MiB
// and 60,000 characters; git is not called when it is not.
func TestCheckReadsTheBodyFile(t *testing.T) {
	cases := []struct {
		name, file, want string
	}{
		{"missing", filepath.Join(t.TempDir(), "none.md"), "iac.prBodyUnreadable"},
		{"a folder", t.TempDir(), "iac.prBodyNotRegular"},
		{"over 1 MiB", bodyFile(t, strings.Repeat("a", 1<<20+1)), "iac.prBodyTooLarge"},
		{"1 MiB, too many characters", bodyFile(t, strings.Repeat("a", 1<<20)), "iac.prBodyTooLong"},
		{"not UTF-8", bodyFile(t, "a\xffb"), "iac.prBodyNotUTF8"},
		{"60,001 characters", bodyFile(t, strings.Repeat("あ", 60001)), "iac.prBodyTooLong"},
		{"60,000 characters", bodyFile(t, strings.Repeat("あ", 60000)), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newTools(t)

			plan, err := checkWith(f, CheckRequest{BodyFile: tc.file})

			if tc.want == "" {
				if err != nil || len([]rune(plan.UserBody)) != 60000 {
					t.Fatalf("%d characters, %v", len([]rune(plan.UserBody)), err)
				}
				return
			}
			if got := detailOf(t, err); got != tc.want {
				t.Fatalf("detail %s, want %s", got, tc.want)
			}
			if calls := f.recorded(); len(calls) != 0 {
				t.Fatalf("git ran: %v", calls)
			}
		})
	}
}

// NFR1.6, NFR1.7: a shape that stops the run is an error that holds the kinds
// and the lines; the other shapes are notices of the plan, and the body stays
// as written.
func TestCheckLooksForShapesInTheBody(t *testing.T) {
	secret := "gh" + "p_" + strings.Repeat("Ab1", 8)
	_, err := checkWith(newTools(t), CheckRequest{BodyFile: bodyFile(t, "x\n"+secret+"\n")})

	var shapes *BodyShapesError
	if detailOf(t, err) != "iac.prBodySecret" || !errors.As(err, &shapes) ||
		len(shapes.Findings) != 1 || shapes.Findings[0].Kind != redact.KindGitHubToken || !slices.Equal(shapes.Findings[0].Lines, []int{2}) {
		t.Fatalf("error %v, %+v", err, shapes)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("the error holds the token: %v", err)
	}
	body := "queue arn:aws:sqs:ap-northeast-1:111122223333:q\n"
	plan, err := checkWith(newTools(t), CheckRequest{BodyFile: bodyFile(t, body)})
	if err != nil || plan.UserBody != body || len(plan.BodyNotices) != 2 || plan.BodyNotices[0].Kind != redact.KindARN {
		t.Fatalf("plan %+v, %v", plan, err)
	}
}

// NFR6.12: a name that starts with - is turned away without git; git must take
// the name and give it back unchanged.
func TestCheckTheNameOfTheBranch(t *testing.T) {
	f := newTools(t)
	if _, err := checkWith(f, CheckRequest{BranchName: "-x"}); detailOf(t, err) != "iac.prBadBranch" || len(f.recorded()) != 0 {
		t.Fatalf("-x: %v, calls %v", err, f.recorded())
	}
	for name, answer := range map[string]procexec.Result{
		"@{-1}":   {Stdout: []byte("main\n")},
		"a..b":    {ExitCode: 128},
		"feature": {Stdout: []byte("other\n")},
	} {
		f := newTools(t)
		f.change["git check-ref-format --branch"] = answer
		if _, err := checkWith(f, CheckRequest{BranchName: name}); detailOf(t, err) != "iac.prBadBranch" {
			t.Errorf("%s: %v", name, err)
		}
	}
	f = newTools(t)
	f.change["git check-ref-format --branch"] = procexec.Result{Stdout: []byte("feature/tags\n")}
	plan, err := checkWith(f, CheckRequest{BranchName: "feature/tags"})
	if err != nil || plan.Branch != "feature/tags" {
		t.Fatalf("plan %+v, %v", plan, err)
	}
	if _, asked := f.find("git ls-remote --heads origin refs/heads/feature/tags"); !asked {
		t.Fatalf("the remote was not asked for the branch: %v", f.recorded())
	}
}

// BR1.2, NFR6.13: the default branch is the time in UTC with a Z, whatever the
// zone of the computer.
func TestDefaultBranchIsInUTC(t *testing.T) {
	local := time.Local
	t.Cleanup(func() { time.Local = local })
	time.Local = time.FixedZone("UTC+9", 9*60*60)

	at := time.Date(2026, 11, 9, 8, 0, 0, 0, time.Local)

	if got := DefaultBranch(at); got != "tag-refactor/iac-20261108-230000Z" {
		t.Fatalf("DefaultBranch = %s", got)
	}
	if DefaultBranch(at) != DefaultBranch(at.UTC()) {
		t.Fatal("the zone changes the name")
	}
}

// BR2.1: a branch of the same name on the remote stops Check with the way out.
func TestCheckStopsWhenTheBranchIsOnTheRemote(t *testing.T) {
	f := newTools(t)
	f.change["git ls-remote --heads origin refs/heads/"+defaultBranch] = procexec.Result{Stdout: []byte("abc\trefs/heads/x\n")}

	_, err := checkWith(f, CheckRequest{})

	e, _ := errkind.As(err)
	if detailOf(t, err) != "iac.prBranchOnRemote" || e.HintID != "iac.prBranchOnRemoteFix" {
		t.Fatalf("error %+v", e)
	}
}

// NFR1.10: the body file is new for each run, under 100 characters of name,
// only for the user on Unix, and the other files are left alone.
func TestBodyFile(t *testing.T) {
	plan := Plan{GitDir: filepath.ToSlash(t.TempDir()), Branch: strings.Repeat("ブ", 200)}

	first, err := writeBody(plan, "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeBody(plan, "two")
	if err != nil {
		t.Fatal(err)
	}

	if first == second || len(filepath.Base(first)) > 100 || !strings.HasPrefix(filepath.Base(first), "pr-body-") {
		t.Fatalf("files %s and %s", first, second)
	}
	if data, _ := os.ReadFile(first); string(data) != "one" {
		t.Fatalf("first holds %q", data)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(first); info.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", info.Mode())
		}
		if info, _ := os.Stat(filepath.Dir(first)); info.Mode().Perm() != 0o700 {
			t.Fatalf("folder mode %v", info.Mode())
		}
	}
}

// NFR1.10, security design §5: when the body file cannot be written, Publish
// fails at the branch step before git runs, and names no body file.
func TestPublishStopsWhenTheBodyFileCannotBeWritten(t *testing.T) {
	f := newTools(t)
	plan, err := check(f, "T")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.gitDir, "tag-refactor"))
	sink := &steps{}

	out := publisher(f).Publish(context.Background(), plan, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "b"}, sink)

	if out.FailedStep != StepBranch || out.BodyFile != "" || len(out.CompletedSteps) != 0 || detailOf(t, out.Err) != "iac.prBodyFileFailed" {
		t.Fatalf("outcome %+v", out)
	}
	if _, switched := f.find("git switch"); switched {
		t.Fatal("a branch was made")
	}
	if !slices.Equal(sink.lines, []string{"branch started " + plan.Branch, "branch failed " + plan.Branch}) {
		t.Fatalf("sink %v", sink.lines)
	}
}

// BR4.3: gh that ends well without a URL is a success with no URL; the body
// file goes all the same.
func TestPublishWithoutAURL(t *testing.T) {
	f := newTools(t)
	f.change["gh pr create"] = procexec.Result{Stdout: []byte("Creating pull request...\n")}

	out, _ := publish(t, f, PublishInput{Files: []string{"a.ts"}, Title: "T", Body: "b"})

	if out.FailedStep != "" || out.URL != "" || out.BodyFile != "" {
		t.Fatalf("outcome %+v", out)
	}
	if left, _ := filepath.Glob(filepath.Join(f.gitDir, "tag-refactor", "*")); len(left) != 0 {
		t.Fatalf("files left: %v", left)
	}
}
