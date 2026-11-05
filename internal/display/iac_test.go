package display

import (
	"errors"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/iac"
)

// Unit tests of the output of iac (U8 BR6.1, NFR8.27, NFR11.10), written after
// the implementation. The expected files are made like those of the other
// formats: go test -run TestGoldenIac ./internal/display/... -update

const iacDiff = "--- a/stacks/orders.yaml\n+++ b/stacks/orders.yaml\n@@ -3,4 +3,4 @@\n     Type: AWS::SQS::Queue\n-        - Key: env\n+        - Key: Environment\n\\ No newline at end of file"

// iacExample is the result of the example of the functional specification.
func iacExample() *iac.Result {
	return &iac.Result{
		Files: []iac.FileResult{
			{File: "main.tf", Tool: iac.ToolTerraform, Reflected: 4, Diff: "--- a/main.tf\n+++ b/main.tf\n@@ -1,1 +1,1 @@\n-  env = \"Prod\"\n+  Environment = \"prod\""},
			{File: "stacks/orders.yaml", Tool: iac.ToolCloudFormation, Reflected: 1, Diff: iacDiff},
		},
		Reflected: 5, AlreadyReflected: 3, Unmanaged: 12, Resources: 20,
		AssumedRemovals: []iac.ManualItem{{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:audit", ChangeID: "c-1", Key: "obsolete", File: "stacks/audit.yaml"}},
		Manual: []iac.ManualItem{
			{ARN: "arn:aws:s3:::logs-bucket", ChangeID: "c-2", Key: "env", Reason: iac.ReasonInModule, Detail: "module.logs.aws_s3_bucket.this", DetailKind: iac.DetailAddress},
			{ARN: "arn:aws:s3:::twice", ChangeID: "c-3", Key: "env", Reason: iac.ReasonAmbiguous, Detail: "a.b, c.d", DetailKind: iac.DetailAddresses},
			{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:billing", ChangeID: "c-4", Key: "Owner", File: "stacks/billing.yaml", Line: 20, Reason: iac.ReasonNotLiteral},
			{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:c", ChangeID: "c-5", Key: "k", Reason: iac.ReasonAmbiguous, Detail: "dev/a.yaml, prod/a.yaml", DetailKind: iac.DetailCandidates},
			{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:d", ChangeID: "c-6", Key: "k", Reason: iac.ReasonTemplateNotFound, Detail: "a.yaml", DetailKind: iac.DetailOtherType},
			{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:e", ChangeID: "c-7", Key: "k", Reason: iac.ReasonTemplateNotFound, Detail: "bad.yaml", DetailKind: iac.DetailSkipped},
			{ARN: "arn:aws:sqs:ap-northeast-1:111122223333:f", ChangeID: "c-8", Key: "k", File: "a.yaml", Line: 3, Reason: iac.ReasonKeyNotInSource},
		},
	}
}

func iacOutput(t *testing.T, lang Lang, res *iac.Result, debug bool) (string, string) {
	t.Helper()
	p, out, errOut := newPrinter()
	p.SetLang(lang)
	p.SetDebug(debug)
	p.IacResult(res)
	return out.String(), errOut.String()
}

func TestGoldenIacResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		lang Lang
	}{{"en", English}, {"ja", Japanese}} {
		out, errOut := iacOutput(t, tc.lang, iacExample(), false)
		checkGolden(t, "iac/result."+tc.name+".golden", []byte(out))
		checkGolden(t, "iac/guidance."+tc.name+".golden", []byte(errOut))
	}
}

func TestIacWithoutColorsTheMeaningStays(t *testing.T) {
	p, out, _ := newPrinter()
	p.colorOut = true
	p.IacResult(iacExample())
	colored := out.String()
	if !strings.Contains(colored, string(styleAdd)) || !strings.Contains(colored, string(styleRemove)) {
		t.Fatalf("no colors: %q", colored)
	}
	plain, _ := iacOutput(t, English, iacExample(), false)
	if StripColors(colored) != plain {
		t.Fatalf("colored output differs:\n%s\nfrom\n%s", StripColors(colored), plain)
	}
	for _, heading := range []string{"To fix by hand (7):", "Removals counted as reflected", "Not managed by IaC"} {
		if !strings.Contains(plain, heading) {
			t.Errorf("no heading %q", heading)
		}
	}
}

// The English the check of the checkpoint reads (cicd-pipeline.md); a change
// here is a change of scripts/verify.sh too.
func TestIacWordsTheVerificationReads(t *testing.T) {
	out, _ := iacOutput(t, English, iacExample(), false)
	if !strings.Contains(out, "already reflected: 3\n") {
		t.Errorf("no count of the reflected: %s", out)
	}
	if !strings.Contains(out, "the key is not in the source; it may come from a stack tag or default_tags") {
		t.Errorf("no reason of keyNotInSource: %s", out)
	}
}

func TestIacEveryReasonHasWords(t *testing.T) {
	for _, reason := range []iac.ManualReason{iac.ReasonTemplateNotFound, iac.ReasonBlockNotFound, iac.ReasonAmbiguous, iac.ReasonInModule,
		iac.ReasonMultipleInstances, iac.ReasonTagsNotLiteral, iac.ReasonUnsupportedTagShape, iac.ReasonNotLiteral, iac.ReasonKeyNotInSource,
		iac.ReasonValueDiffers, iac.ReasonKeyAlreadyExists, iac.ReasonNoTagSite, iac.ReasonUnreadableFile} {
		for _, lang := range []Lang{English, Japanese} {
			if got := T(lang, "reason."+string(reason)); got == "reason."+string(reason) || got == "" {
				t.Errorf("no words for %s", reason)
			}
		}
	}
}

func TestIacFileLinesFollowTheWriting(t *testing.T) {
	res := &iac.Result{WriteRequested: true, Files: []iac.FileResult{
		{File: "a.yaml", Reflected: 1, Tool: iac.ToolCloudFormation, Written: true, Diff: "--- a/a.yaml"},
		{File: "b.tf", Reflected: 2, Tool: iac.ToolTerraform, Diff: "--- a/b.tf"},
	}, WriteErr: errors.New("stop")}
	out, errOut := iacOutput(t, English, res, false)
	if !strings.Contains(out, "a.yaml: 1 change reflected\n") || !strings.Contains(out, "b.tf: 2 changes not written\n") {
		t.Fatalf("stdout:\n%s", out)
	}
	if !strings.Contains(errOut, "Review the difference") || !strings.Contains(errOut, "A stack update then gives the same tags") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	if strings.Contains(errOut, "Add --write") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	res.Interrupted, res.WriteErr = true, nil
	_, errOut = iacOutput(t, English, res, false)
	if !strings.Contains(errOut, "stopped by Ctrl+C") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	terraformOnly := &iac.Result{WriteRequested: true, Files: []iac.FileResult{{File: "b.tf", Reflected: 1, Tool: iac.ToolTerraform, Written: true, Diff: "x"}}}
	if _, errOut := iacOutput(t, Japanese, terraformOnly, false); strings.Contains(errOut, "スタックを更新") || !strings.Contains(errOut, "レビュー") {
		t.Fatalf("stderr:\n%s", errOut)
	}
}

func TestIacWarnsOfWhatItLeftOut(t *testing.T) {
	res := &iac.Result{
		SkippedFiles: []iac.SkippedFile{{File: "big.yaml", Reason: iac.SkipTooLarge}, {File: "a.json", Reason: iac.SkipNotUTF8},
			{File: "b.json", Reason: iac.SkipNotUTF8}, {File: "deep.yaml", Reason: iac.SkipTooDeep}, {File: "bad.yaml", Reason: iac.SkipParseError, Line: 2, Column: 5, Kind: "yaml"},
			{File: "bad.tf", Reason: iac.SkipParseError, Line: 3, Kind: "hcl"}, {File: "no.yaml", Reason: iac.SkipUnreadable}},
		SkippedFolders: []string{"very/deep"},
		Debug:          []string{"git ls-files: fatal: not a repo", "a.yaml: (a) the tags"},
	}
	_, errOut := iacOutput(t, English, res, false)
	for _, want := range []string{"warning: 1 file left out: larger than 10 MB", "warning: 2 files left out: not UTF-8", "warning: 1 file left out: nested too deep",
		"warning: 2 files left out: could not be parsed", "warning: 1 file left out: could not be read", "warning: 1 folders left out"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("no %q in:\n%s", want, errOut)
		}
	}
	if strings.Contains(errOut, "big.yaml") || strings.Contains(errOut, "git ls-files") {
		t.Fatalf("the names wait for --debug:\n%s", errOut)
	}
	_, errOut = iacOutput(t, English, res, true)
	for _, want := range []string{"debug: big.yaml: tooLarge", "debug: bad.yaml: parseError yaml line 2 column 5", "debug: bad.tf: parseError hcl line 3\n",
		"debug: very/deep", "debug: git ls-files: fatal: not a repo", "debug: a.yaml: (a) the tags"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("no %q in:\n%s", want, errOut)
		}
	}
	_, errOut = iacOutput(t, Japanese, res, false)
	if !strings.Contains(errOut, "警告: 2個のファイルを外しました: UTF-8 ではありません") {
		t.Fatalf("stderr:\n%s", errOut)
	}
}

func TestIacGuidanceOnTerraformAndNothingReflected(t *testing.T) {
	_, errOut := iacOutput(t, English, &iac.Result{Unmanaged: 2}, false)
	if !strings.Contains(errOut, "terraform show -json") || !strings.Contains(errOut, "Nothing was reflected in IaC") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	_, errOut = iacOutput(t, English, &iac.Result{Unmanaged: 2, HasTFState: true, AlreadyReflected: 1}, false)
	if strings.Contains(errOut, "terraform show -json") || strings.Contains(errOut, "Nothing was reflected") {
		t.Fatalf("stderr:\n%s", errOut)
	}
	_, errOut = iacOutput(t, English, &iac.Result{Manual: []iac.ManualItem{{Reason: iac.ReasonUnreadableFile}}}, false)
	if !strings.Contains(errOut, "fault of this tool") {
		t.Fatalf("stderr:\n%s", errOut)
	}
}

// NFR8.6: what a file holds cannot move the terminal, and a tab and a backslash
// show as they are.
func TestIacSanitizesTheDifference(t *testing.T) {
	res := &iac.Result{Files: []iac.FileResult{{File: "a\x1b[31m.yaml", Reflected: 1,
		Diff: "--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-\tkey: a\\b\n+\tkey: \x1b[31mred\xe2\x80\xae"}}}
	out, _ := iacOutput(t, English, res, false)
	if strings.Contains(out, "\x1b") || strings.Contains(out, "\xe2\x80\xae") {
		t.Fatalf("control characters reached the terminal: %q", out)
	}
	if !strings.Contains(out, "-\tkey: a\\b\n") || !strings.Contains(out, `\u{1B}[31mred\u{202E}`) {
		t.Fatalf("stdout: %q", out)
	}
}
