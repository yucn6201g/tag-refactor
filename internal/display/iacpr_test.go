package display

// Unit tests of the lines of the CDK apps and of the pull request, and of the
// title and the body (v0.2.0 U1), written after the implementation.

import (
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
)

// BR1.7, NFR10.1: the states of an app are one line each on standard error,
// in words, with the time in seconds; synth that needs AWS says what to do.
func TestCDKProgressLines(t *testing.T) {
	p, out, errOut := newPrinter()

	p.AppsWaiting([]string{"infra"})
	p.AppsWaiting(nil)
	p.AppStarted("infra", "typescript")
	p.AppDone("infra", 42*time.Second+300*time.Millisecond)
	p.AppFailed("api", 12*time.Second, "needsAWS")
	p.AppFailed("web", time.Second, "helper")
	p.AppSkipped("py", "unsupportedLanguage")
	p.AppSkipped("second", iac.ReasonNotInU1)
	p.AppSkipped("odd", "somethingNew")

	want := "CDK apps to synthesize: infra (waiting)\n" +
		"infra: synthesizing (TypeScript)\n" +
		"infra: done (42s)\n" +
		"api: failed (12s): AWS credentials needed\n" +
		"  Fix: run cdk synth once where AWS credentials are set, so that cdk.context.json keeps the values it looked up, then run iac again\n" +
		"web: failed (1s): its sources could not be read; add --debug to see why\n" +
		"py: skipped: not TypeScript\n" +
		"second: skipped: this version takes the first CDK app only\n" +
		"odd: skipped: somethingNew\n"
	if errOut.String() != want || out.Len() != 0 {
		t.Fatalf("stderr:\n%s\nstdout:\n%s", errOut.String(), out.String())
	}
}

// NFR10.2: the same lines in Japanese.
func TestCDKProgressLinesInJapanese(t *testing.T) {
	p, _, errOut := newPrinter()
	p.SetLang(Japanese)

	p.AppsWaiting([]string{"infra"})
	p.AppStarted("infra", "typescript")
	p.AppDone("infra", 3*time.Second)
	p.AppFailed("infra", 3*time.Second, "timeout")

	for _, line := range []string{"synth する CDK のアプリ: infra(待ち)", "infra: synth しています(TypeScript)",
		"infra: 済み(3 秒)", "infra: 失敗(3 秒): 時間内に終わりませんでした"} {
		if !strings.Contains(errOut.String(), line+"\n") {
			t.Fatalf("no %q in:\n%s", line, errOut.String())
		}
	}
}

// Observability design §1, security design §7: the destination is one line
// before anything is written, the steps are one line each, the URL is the one
// line of standard output, and the debug lines show only with --debug.
func TestPullRequestLines(t *testing.T) {
	p, out, errOut := newPrinter()
	plan := pullrequest.Plan{Remote: "origin", Host: "github.com", RepoPath: "o/r", Base: "main", Branch: "tag-refactor/iac-1"}

	p.PullRequestPlan(plan)
	p.Step(pullrequest.StepChecked, pullrequest.StatusDone, plan.Branch)
	p.Step(pullrequest.StepBranch, pullrequest.StatusStarted, plan.Branch)
	p.Step(pullrequest.StepPush, pullrequest.StatusDone, plan.Branch)
	p.Step(pullrequest.StepCreate, pullrequest.StatusFailed, plan.Branch)
	p.DebugLine("not shown")
	p.SetDebug(true)
	p.DebugLine("git rev-parse --show-toplevel")
	p.PullRequestOpened("https://github.com/o/r/pull/7")

	want := "Pull request: origin (github.com/o/r), base main, branch tag-refactor/iac-1\n" +
		"Pull request: checks passed\n" +
		"Pull request: branch started\n" +
		"Pull request: push done\n" +
		"Pull request: create failed\n" +
		"debug: git rev-parse --show-toplevel\n"
	if errOut.String() != want {
		t.Fatalf("stderr:\n%s", errOut.String())
	}
	if out.String() != "https://github.com/o/r/pull/7\n" {
		t.Fatalf("stdout %q", out.String())
	}
}

// BR4.2: why no pull request was made, and what stays after a failed step.
func TestPullRequestNotOpenedAndStopped(t *testing.T) {
	p, _, errOut := newPrinter()

	p.PullRequestNotOpened(false)
	p.PullRequestNotOpened(true)
	p.PullRequestStopped(pullrequest.PublishOutcome{Branch: "b", CompletedSteps: []string{pullrequest.StepBranch, pullrequest.StepCommit},
		BodyFile: "/repo/.git/tag-refactor/pr-body-b.md"})
	p.PullRequestStopped(pullrequest.PublishOutcome{Branch: "b"})

	for _, line := range []string{"No file was written, so no pull request was opened.",
		"The pull request was not opened: the files were not all written.",
		"The working tree stays on the branch b; the steps done are not undone (done: branch, commit).",
		"The body of the pull request is in /repo/.git/tag-refactor/pr-body-b.md"} {
		if !strings.Contains(errOut.String(), line+"\n") {
			t.Fatalf("no %q in:\n%s", line, errOut.String())
		}
	}
	if strings.Count(errOut.String(), "stays on the branch") != 1 {
		t.Fatalf("a branch was reported with no step done:\n%s", errOut.String())
	}
}

// BR4.3, BR4.4, contract X2: the title holds the plan file name redacted; the
// body lists the written files with their kind, from the top of the working
// tree, and nothing that could hold an ARN or an account ID.
func TestPullRequestTitleAndBody(t *testing.T) {
	if got := PullRequestTitle(English, "plan-123456789012.json"); got != "Reflect tag changes from tag-refactor (plan-<redacted>.json)" {
		t.Fatalf("title %q", got)
	}
	if got := PullRequestTitle(Japanese, "plan.json"); got != "tag-refactor によるタグの変更の反映(plan.json)" {
		t.Fatalf("title %q", got)
	}
	res := &iac.Result{
		Files: []iac.FileResult{
			{File: "lib/stack.ts", Tool: iac.ToolCDK, Language: "typescript", Written: true},
			{File: "stacks/a.yaml", Tool: iac.ToolCloudFormation, Written: true},
			{File: "main.tf", Tool: iac.ToolTerraform, Written: true},
			{File: "not-written.yaml", Tool: iac.ToolCloudFormation},
			{File: "arn:aws:s3:::odd-`name`.yaml", Tool: iac.ToolCloudFormation, Written: true},
		},
		Manual: []iac.ManualItem{{ARN: "arn:aws:s3:::secret", Key: "env", Reason: iac.ReasonValueDiffers}},
	}

	body := PullRequestBody(English, res, "infra/")

	for _, want := range []string{"`tag-refactor iac --pr`", "## Changed files (4)", "- `infra/lib/stack.ts` (CDK, TypeScript)\n",
		"- `infra/stacks/a.yaml` (CloudFormation)\n", "- `infra/main.tf` (Terraform)\n", "- `infra/<redacted>'name'.yaml` (CloudFormation)\n"} {
		if !strings.Contains(body, want) {
			t.Fatalf("no %q in:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"not-written", "secret", "arn:aws"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("%q in:\n%s", unwanted, body)
		}
	}
	if ja := PullRequestBody(Japanese, res, ""); !strings.Contains(ja, "## 変更したファイル(4)") || !strings.Contains(ja, "(CDK、TypeScript)") {
		t.Fatalf("body:\n%s", ja)
	}
}

// Review 1, X1: a long plan file name is cut, with an ellipsis, so that the
// default title fits in 256 characters after the redaction, in both languages;
// a short name stays whole.
func TestPullRequestTitleOfALongPlanFile(t *testing.T) {
	long := "plan-" + strings.Repeat("あ", 300) + ".json"
	withSecrets := strings.Repeat("plan-123456789012-", 30) + ".json"
	for _, lang := range []Lang{English, Japanese} {
		for _, name := range []string{long, withSecrets} {
			title := PullRequestTitle(lang, name)

			if n := len([]rune(title)); n > 256 {
				t.Fatalf("%d characters: %s", n, title)
			}
			if !strings.Contains(title, "plan-") || !strings.Contains(title, "…") || strings.Contains(title, "123456789012") {
				t.Fatalf("title %q", title)
			}
		}
	}
	if got := PullRequestTitle(English, strings.Repeat("x", 200)+".json"); strings.Contains(got, "…") || len([]rune(got)) > 256 {
		t.Fatalf("a name that fits was cut: %q", got)
	}
}

// Contract X2: the reason of a failed app names the program only.
func TestManualLineOfAFailedApp(t *testing.T) {
	res := &iac.Result{Manual: []iac.ManualItem{{ARN: "arn:aws:s3:::b", ChangeID: "c-1", Key: "Env",
		Reason: iac.ReasonCDKAppFailed, Detail: "cdk", DetailKind: iac.DetailTool}},
		CDKApps: []iac.CDKApp{{Dir: "infra", Status: iac.CDKAppFailed, FailureReason: "needsAWS", Tool: "cdk"}}}

	stdout, _ := iacOutput(t, English, res, false)

	if !strings.Contains(stdout, "  Env  (no place)  the CDK app could not be synthesized or read (cdk)\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

// The written CDK source gets its own guidance.
func TestGuidanceOfAWrittenCDKSource(t *testing.T) {
	_, stderr := iacOutput(t, English, &iac.Result{WriteRequested: true, Reflected: 1,
		Files: []iac.FileResult{{File: "lib/stack.ts", Tool: iac.ToolCDK, Written: true, Reflected: 1}}}, false)

	if !strings.Contains(stderr, "A deploy of the CDK app then gives the same tags; see it first with cdk diff.") ||
		strings.Contains(stderr, "A stack update") {
		t.Fatalf("stderr:\n%s", stderr)
	}
}
