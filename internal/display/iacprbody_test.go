package display

// Unit tests of the body of the pull request and of what iac --pr shows
// around it (v0.2.0 U2), written after the implementation.

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

func bodyResult(manual int) *iac.Result {
	res := &iac.Result{Files: []iac.FileResult{{File: "app.py", Tool: iac.ToolCDK, Language: "python", Written: true}}}
	for i := 0; i < manual; i++ {
		res.Manual = append(res.Manual, iac.ManualItem{File: "app.py", Line: i + 1,
			ARN: fmt.Sprintf("arn:aws:sqs:ap-northeast-1:111122223333:q%d", i), Key: "Owner", Reason: iac.ReasonValueDiffers})
	}
	return res
}

// BR3.1, NFR1.7: the body of the user goes first as it is, ARN and all; the
// parts tag-refactor makes are redacted each on its own.
func TestPullRequestBodyKeepsTheBodyOfTheUser(t *testing.T) {
	user := "Queue arn:aws:sqs:ap-northeast-1:111122223333:q0 gets an owner."

	body, ok := PullRequestBody(English, bodyResult(1), "infra/", user)

	want := user + "\n\n## Changed files (1)\n\n- `infra/app.py` (CDK, Python)\n\n## To fix by hand (1)\n\n" +
		"- `infra/app.py:1` q0, Owner: "
	if !ok || !strings.HasPrefix(body, want) {
		t.Fatalf("body:\n%s", body)
	}
	if strings.Count(body, "111122223333") != 1 || strings.Contains(body, "tag-refactor iac --pr") {
		t.Fatalf("body:\n%s", body)
	}
	if ja, _ := PullRequestBody(Japanese, bodyResult(2), "", "本文\n"); !strings.HasPrefix(ja, "本文\n\n## 変更したファイル(1)\n\n- `app.py` (CDK、Python)\n\n## 手で直すもの(2)\n") {
		t.Fatalf("body:\n%s", ja)
	}
}

// Review N3 (entities.md ManualLine.resourceName): a line of the list to fix
// by hand names the logical ID, or the address of Terraform; only when neither
// is known, what follows the last colon or slash of the ARN.
func TestResourceNameOfTheListToFixByHand(t *testing.T) {
	alias := "arn:aws:lambda:ap-northeast-1:111122223333:function:fn:live"
	alb := "arn:aws:elasticloadbalancing:ap-northeast-1:111122223333:loadbalancer/app/my-alb/50dc6c495c0c9188"
	cases := []struct {
		item iac.ManualItem
		want string
	}{
		{iac.ManualItem{ARN: alias, LogicalID: "FnAlias1A2B3C4D", Address: "x"}, "FnAlias1A2B3C4D"},
		{iac.ManualItem{ARN: alb, LogicalID: "MyAlb"}, "MyAlb"},
		{iac.ManualItem{ARN: alias, Address: "aws_lambda_alias.live"}, "aws_lambda_alias.live"},
		{iac.ManualItem{ARN: alias}, "live"},
		{iac.ManualItem{ARN: alb}, "50dc6c495c0c9188"},
		{iac.ManualItem{ARN: "odd"}, "odd"},
	}
	for _, tc := range cases {
		tc.item.Key, tc.item.Reason = "Owner", iac.ReasonValueDiffers
		if got := manualLine(English, &tc.item, ""); !strings.HasPrefix(got, "- "+tc.want+", Owner: ") {
			t.Errorf("%+v gives %q, want the name %q", tc.item, got, tc.want)
		}
	}
}

// BR3.3: a body of 65,536 characters is kept whole; one more character takes
// lines off the end of the list to fix by hand, with a line that says how many;
// a body that is too long without the list is not ok.
func TestPullRequestBodyAtItsLimit(t *testing.T) {
	res := bodyResult(3)
	withX, _ := PullRequestBody(English, res, "", "x")
	room := MaxPullRequestBody - (utf8.RuneCountInString(withX) - 1)

	whole, ok := PullRequestBody(English, res, "", strings.Repeat("あ", room))
	if !ok || utf8.RuneCountInString(whole) != MaxPullRequestBody || !strings.Contains(whole, ":3` q2, Owner") {
		t.Fatalf("%d characters, ok %v", utf8.RuneCountInString(whole), ok)
	}

	cut, ok := PullRequestBody(English, res, "", strings.Repeat("あ", room+1))
	if !ok || utf8.RuneCountInString(cut) > MaxPullRequestBody || strings.Contains(cut, ":3` q2") ||
		!strings.Contains(cut, "- and 1 more (see the output of this command for all)\n") {
		t.Fatalf("%d characters, ok %v, end %q", utf8.RuneCountInString(cut), ok, cut[len(cut)-200:])
	}

	if _, ok := PullRequestBody(English, res, "", strings.Repeat("あ", MaxPullRequestBody)); ok {
		t.Fatal("a body too long without the list is ok")
	}
	if _, ok := PullRequestBody(English, bodyResult(0), "", strings.Repeat("あ", MaxPullRequestBody)); ok {
		t.Fatal("a body too long with no list is ok")
	}
}

// NFR1.6, NFR1.7: the shapes are named with their lines, ten at most, and
// never with their values; in Japanese too.
func TestBodyShapesAndNotices(t *testing.T) {
	p, out, errOut := newPrinter()
	twelve := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	p.BodyShapes([]redact.Finding{{Kind: redact.KindGitHubToken, Stop: true, Lines: twelve}, {Kind: redact.KindURLPassword, Stop: true, Lines: []int{4, 9}}})
	p.BodyNotices([]redact.Finding{{Kind: redact.KindAccountID, Lines: []int{3}}})

	for _, line := range []string{"--pr-body-file: GitHub token on lines 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 and 2 more\n",
		"--pr-body-file: URL with a password on lines 4, 9\n",
		"Note: the file of --pr-body-file holds a 12-digit number on line 3; it goes into the pull request as it is\n"} {
		if !strings.Contains(errOut.String(), line) {
			t.Fatalf("no %q in:\n%s", line, errOut.String())
		}
	}
	if out.Len() != 0 {
		t.Fatalf("stdout: %s", out.String())
	}

	ja, _, jaErr := newPrinter()
	ja.SetLang(Japanese)
	ja.BodyShapes([]redact.Finding{{Kind: redact.KindAccessKeyID, Stop: true, Lines: []int{2, 5}}})
	ja.BodyNotices([]redact.Finding{{Kind: redact.KindARN, Lines: []int{1}}})
	for _, line := range []string{"--pr-body-file: AWS のアクセスキーの ID(2、5 行目)\n",
		"注意: --pr-body-file のファイルに ARN があります(1 行目)。そのままプルリクエストに入ります\n"} {
		if !strings.Contains(jaErr.String(), line) {
			t.Fatalf("no %q in:\n%s", line, jaErr.String())
		}
	}
}

// BR4.3, NFR1.9: the commands are shown as they are, backslashes and all; the
// values of a line by hand are quoted so that every character shows; the
// body file is named, and a path that does not show is quoted.
func TestPullRequestStoppedWithTheResumeLines(t *testing.T) {
	p, _, errOut := newPrinter()
	out := pullrequest.PublishOutcome{Branch: "b", CompletedSteps: []string{pullrequest.StepBranch},
		FailedStep: pullrequest.StepCommit, BodyFile: `C:\w\.git\tag-refactor\pr-body-b.md`}
	lines := []pullrequest.ResumeLine{
		{Step: pullrequest.StepCommit, Command: `git -C 'C:\w' --literal-pathspecs add -- 'a.ts'`},
		{Step: pullrequest.StepCommit, Manual: []pullrequest.ManualValue{{Name: "title", Value: "a\nb"}, {Name: "file", Value: `C:\x "y"`}}},
		{Step: pullrequest.StepCreate, Command: "gh pr create"},
	}

	p.PullRequestStopped(out, lines, pullrequest.ShellPowerShell)

	want := "The working tree stays on the branch b; the steps done are not undone (done: branch).\n" +
		"To finish by hand, run these lines in PowerShell, one at a time from the top:\n" +
		"  git -C 'C:\\w' --literal-pathspecs add -- 'a.ts'\n" +
		"  commit by hand: a value cannot be written safely for this shell; type the command with these values:\n" +
		"    title: \"a\\nb\"\n" +
		"    file: \"C:\\\\x \\\"y\\\"\"\n" +
		"  gh pr create\n" +
		"The body of the pull request is in C:\\w\\.git\\tag-refactor\\pr-body-b.md\n" +
		"  Delete it when you no longer need it.\n"
	if !strings.Contains(errOut.String(), want) {
		t.Fatalf("stderr:\n%s\nwant\n%s", errOut.String(), want)
	}

	q, _, qErr := newPrinter()
	q.PullRequestStopped(pullrequest.PublishOutcome{FailedStep: pullrequest.StepCreate, BodyFile: "/w/a\nb.md"},
		[]pullrequest.ResumeLine{{Step: pullrequest.StepCreate, Command: "gh pr create"}}, pullrequest.ShellSh)
	for _, line := range []string{"The written files stay as they are; no branch was made.\n",
		"run these lines in sh,", `The body of the pull request is in "/w/a\nb.md"` + "\n"} {
		if !strings.Contains(qErr.String(), line) {
			t.Fatalf("no %q in:\n%s", line, qErr.String())
		}
	}
}

// BR4.3: without a body file, the creation by hand says to write the body in
// an editor, and no body file is named.
func TestPullRequestStoppedWithoutABodyFile(t *testing.T) {
	p, _, errOut := newPrinter()
	p.SetLang(Japanese)

	p.PullRequestStopped(pullrequest.PublishOutcome{FailedStep: pullrequest.StepBranch},
		[]pullrequest.ResumeLine{{Step: pullrequest.StepCreate, Manual: []pullrequest.ManualValue{{Name: "repo", Value: "o/r"}}}}, pullrequest.ShellSh)

	for _, line := range []string{"書き換えたファイルはそのまま残っています。ブランチは作っていません。\n",
		"gh pr create を --body-file なしで動かすと", "    リポジトリ: \"o/r\"\n"} {
		if !strings.Contains(errOut.String(), line) {
			t.Fatalf("no %q in:\n%s", line, errOut.String())
		}
	}
	if strings.Contains(errOut.String(), "本文は") {
		t.Fatalf("a body file is named:\n%s", errOut.String())
	}
}

// The other lines of U2: the redacted title, nothing to reflect and a pull
// request without a URL.
func TestOtherLinesOfThePullRequest(t *testing.T) {
	p, _, errOut := newPrinter()

	p.TitleRedacted()
	p.NothingToReflect()
	p.PullRequestWithoutURL(pullrequest.ResumeLine{Step: pullrequest.StepList, Command: `gh pr list --repo 'o/r' --head 'C:\b'`}, pullrequest.ShellPowerShell)
	p.PullRequestWithoutURL(pullrequest.ResumeLine{Step: pullrequest.StepList,
		Manual: []pullrequest.ManualValue{{Name: "repo", Value: "o/r"}, {Name: "branch", Value: "b\x1b[2J"}}}, pullrequest.ShellSh)

	for _, line := range []string{"Some values of --pr-title were replaced with <redacted> in the title and the commit message\n",
		"Nothing to reflect, so no pull request was opened.\n",
		"check it with this line in PowerShell:\n  gh pr list --repo 'o/r' --head 'C:\\b'\n",
		"check it with gh pr list --repo and --head, with these values (they cannot be written safely for this shell):\n" +
			"    repository: \"o/r\"\n    branch: \"b\\x1b[2J\"\n"} {
		if !strings.Contains(errOut.String(), line) {
			t.Fatalf("no %q in:\n%s", line, errOut.String())
		}
	}
}

// Every shape of redact.Scan and every value of a resume line has its name in
// both languages.
func TestNamesOfShapesAndValues(t *testing.T) {
	kinds := []string{redact.KindGitHubToken, redact.KindAccessKeyID, redact.KindURLPassword, redact.KindARN, redact.KindAccountID}
	values := []string{"root", "branch", "file", "title", "remote", "repo", "base", "bodyFile"}
	for _, ids := range []struct {
		names map[string]string
		keys  []string
	}{{shapeNames, kinds}, {valueNames, values}} {
		for _, k := range ids.keys {
			m, ok := catalog[ids.names[k]]
			if !ok || m.en == "" || m.ja == "" {
				t.Errorf("%s has no message (%q)", k, ids.names[k])
			}
		}
	}
}
