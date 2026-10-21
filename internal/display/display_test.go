package display

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

func newPrinter() (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return New(&out, &errOut), &out, &errOut
}

func TestSanitizeMakesControlCharactersVisible(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain-ascii env=dev", "plain-ascii env=dev"},
		{"\x1b[31mred\x1b[0m", `\u{1B}[31mred\u{1B}[0m`},
		{"line\nbreak\ttab\rcr", `line\u{A}break\u{9}tab\u{D}cr`},
		{"del\x7f c1\xc2\x9b", `del\u{7F} c1\u{9B}`},
		{"bidi\xe2\x80\xae\xe2\x81\xa6x", `bidi\u{202E}\u{2066}x`},
		{"zero\xe2\x80\x8bwidth", `zero\u{200B}width`},
		{"ls\xe2\x80\xa8ps\xe2\x80\xa9", `ls\u{2028}ps\u{2029}`},
		{"bad\xff\xfebytes", `bad\x{FF}\x{FE}bytes`},
		{`back\slash`, `back\\slash`},
		{"\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e-key", "\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e-key"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := Sanitize(tc.in); got != tc.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScanResultListsVariantsOrNoIssues(t *testing.T) {
	p, out, errOut := newPrinter()
	snap := &inventory.Snapshot{ScannedRegions: []string{"us-east-1"}, Resources: make([]inventory.Resource, 412)}
	groups := []inventory.KeyVariantGroup{
		{NormalizedKey: "env", TotalResources: 208, IsVariant: true, Spellings: []inventory.Spelling{{Key: "env", ResourceCount: 150}, {Key: "Env", ResourceCount: 58}}},
		{NormalizedKey: "owner", TotalResources: 3, Spellings: []inventory.Spelling{{Key: "Owner", ResourceCount: 3}}},
		{NormalizedKey: "x", TotalResources: 1, IsVariant: true, Spellings: []inventory.Spelling{{Key: "x\x1b", ResourceCount: 1}, {Key: "X", ResourceCount: 1}}},
	}

	p.ScanResult(snap, groups)

	want := "Tag key variants (same key except for case and separators)\n" +
		"  env                   208 resources\n" +
		"    env                   150\n" +
		"    Env                    58\n" +
		"  x\\u{1B}" + strings.Repeat(" ", 17) + "1 resource\n" +
		"    x\\u{1B}" + strings.Repeat(" ", 17) + "1\n" +
		"    X                       1\n" +
		"\n" +
		"Summary: 412 resources scanned in 1 region (us-east-1) · 2 variant groups\n"
	if out.String() != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr: %q", errOut.String())
	}

	p2, out2, _ := newPrinter()
	p2.ScanResult(&inventory.Snapshot{ScannedRegions: []string{"eu-west-1"}, Resources: make([]inventory.Resource, 1)}, groups[1:2])
	if out2.String() != "No issues found\nSummary: 1 resource scanned in 1 region (eu-west-1) · 0 variant groups\n" {
		t.Fatalf("stdout: %q", out2.String())
	}
}

func samplePlan() *planner.Plan {
	return &planner.Plan{
		ScannedResources: 10,
		Changes: []planner.Change{
			{ChangeID: "c-000001", ResourceARN: "arn:a", FromKey: "env", ToKey: "Environment", Value: "dev"},
			{ChangeID: "c-000002", ResourceARN: "arn:b", FromKey: "env", ToKey: "Environment", Value: "dev"},
			{ChangeID: "c-000003", ResourceARN: "arn:b", FromKey: "Env", ToKey: "Environment", Value: "dev", MergesIntoExisting: true},
		},
		Conflicts: []planner.KeyConflict{{ResourceARN: "arn:c", CanonicalKey: "Environment",
			PresentTags: map[string]string{"env": "dev\x1b[2J", "Environment": `pr\od`}, KeyOrder: []string{"env", "Environment"}}},
		ImpactFindings: []planner.ImpactFinding{
			{Verdict: planner.VerdictAffected, PolicyRef: "arn:p1", StatementID: "AllowDev", ConditionOperator: "StringEquals",
				ConditionKey: "aws:ResourceTag/env", ConditionValue: "dev", ChangeIDs: []string{"c-000001", "c-000002", "c-000003"}},
			{Verdict: planner.VerdictUndecided, PolicyRef: "arn:p2", StatementID: "#0", ConditionOperator: "Null",
				ConditionKey: "aws:ResourceTag/${aws:PrincipalTag/team}", ChangeIDs: []string{"c-000001"}},
		},
		UnreadPolicies: []planner.UnreadPolicy{
			{SourceKind: planner.SourceIAMManaged, PolicyRef: "arn:p3", Reason: "access denied to iam:GetPolicyVersion"},
			{SourceKind: planner.SourceIAMManaged, Reason: "access denied to iam:ListPolicies"},
		},
		ImpactAnalysisComplete: false,
	}
}

func TestPlanResultShowsIncompleteChangesConflictsImpactAndSummary(t *testing.T) {
	p, out, _ := newPrinter()

	p.PlanResult(samplePlan())

	want := "! Impact analysis is incomplete\n" +
		"    IAM managed policy arn:p3: access denied to iam:GetPolicyVersion\n" +
		"    IAM managed policies: access denied to iam:ListPolicies\n" +
		"\n" +
		"Changes: 2 resources\n" +
		"  ~ rename key   env -> Environment                 2 resources\n" +
		"  ~ rename key   Env -> Environment                 1 resource  (merge into existing)\n" +
		"  (A resource with several changes is counted once in the total.)\n" +
		"\n" +
		"Conflicts (not changed; values differ): 1 resource\n" +
		"  arn:c   env=\"dev\\u{1B}[2J\"  Environment=\"pr\\\\od\"\n" +
		"\n" +
		"Impact (references that use the tags being changed)\n" +
		"  ! IAM policy  arn:p1\n" +
		"      Statement \"AllowDev\"  Condition StringEquals aws:ResourceTag/env = \"dev\"\n" +
		"      affected by: rename key env -> Environment, rename key Env -> Environment\n" +
		"  ? IAM policy  arn:p2\n" +
		"      Statement \"#0\"  Condition Null aws:ResourceTag/${aws:PrincipalTag/team}\n" +
		"      cannot decide (policy variable)\n" +
		"\n" +
		"Summary: 2 to change · 1 conflicts · 1 affected · 1 undecided\n"
	if out.String() != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestPlanResultWithNothingToDo(t *testing.T) {
	p, out, _ := newPrinter()

	p.PlanResult(&planner.Plan{ScannedResources: 3, ImpactAnalysisComplete: true})

	if out.String() != "No changes needed\nSummary: 3 resources scanned · 0 to change\n" {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestMessagesProgressAndDebugGoToStandardError(t *testing.T) {
	p, out, errOut := newPrinter()
	p.APICall(awsaccess.CallRecord{Op: "tag:GetResources", Region: "us-east-1"}) // debug is off: nothing
	p.SetDebug(true)

	p.Account(awsaccess.AccountContext{AccountID: "123456789012", Region: "us-east-1"})
	p.RegionStarted("us-east-1")
	p.RegionDone("us-east-1", 1)
	p.RegionFailed("us-east-1", "AccessDenied")
	p.Warn("careful\x1b")
	p.FileWarning("rules.yaml", 8, `"required" is not used in this version`)
	p.APICall(awsaccess.CallRecord{Op: "tag:GetResources", Region: "us-east-1", Duration: 132 * time.Millisecond, Retries: 0})
	p.APICall(awsaccess.CallRecord{Op: "iam:ListPolicies", Region: "us-east-1", Duration: time.Second, Retries: -1, ErrorKind: "Internal", CauseType: "*url.Error"})
	p.Error("access denied for tag:GetResources", "allow it")
	p.Fatal("stopped")
	p.Done(12*time.Second, 412)
	p.ExitCode(2, "issues found")

	want := "Account 123456789012 · region us-east-1\n" +
		"scan: us-east-1 started\n" +
		"scan: us-east-1 done (1 resource)\n" +
		"scan: us-east-1 failed (AccessDenied)\n" +
		"warning: careful\\u{1B}\n" +
		"warning: rules.yaml line 8: \"required\" is not used in this version\n" +
		"debug: tag:GetResources us-east-1 132ms retries=0\n" +
		"debug: iam:ListPolicies us-east-1 1000ms retries=n/a error=Internal cause=*url.Error\n" +
		"Error: access denied for tag:GetResources\n" +
		"  Fix: allow it\n" +
		"Error: stopped\n" +
		"Done in 00:12 · 412 resources\n" +
		"Exit code: 2 (issues found)\n"
	if errOut.String() != want {
		t.Fatalf("stderr:\n%s\nwant:\n%s", errOut.String(), want)
	}
	if out.Len() != 0 {
		t.Fatal("messages must not reach standard output")
	}
	if _, err := p.Stdout().Write([]byte("help\n")); err != nil || out.String() != "help\n" {
		t.Fatalf("Stdout() must write to standard output: %v %q", err, out.String())
	}
	if _, err := p.Stderr().Write([]byte("usage\n")); err != nil || !bytes.HasSuffix(errOut.Bytes(), []byte("usage\n")) {
		t.Fatalf("Stderr() must write to standard error: %v", err)
	}
}

// The scope of what was checked goes to standard error only, so standard
// output keeps just the results (BR6.1).
func TestScopeLinesGoToStandardError(t *testing.T) {
	p, out, errOut := newPrinter()

	p.ScanScope([]string{"us-east-1"})
	p.PlanScope([]string{"us-east-1"}, true)
	p.PlanScope([]string{"eu-west-1\x1b"}, false)

	want := "Checked: only the resources that the tagging API returns in us-east-1\n" +
		"Checked: only the resources that the tagging API returns in us-east-1, " +
		"and customer managed IAM policies attached to a user, role or group\n" +
		"Checked: only the resources that the tagging API returns in eu-west-1\\u{1B} (no IAM policies were read: nothing to change)\n"
	if errOut.String() != want {
		t.Fatalf("stderr:\n%s\nwant:\n%s", errOut.String(), want)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestFormatElapsed(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0: "00:00", 400 * time.Millisecond: "00:00", 12 * time.Second: "00:12", 61 * time.Second: "01:01",
		time.Hour + 2*time.Minute + 3*time.Second: "1:02:03", -time.Second: "00:00",
	} {
		if got := FormatElapsed(d); got != want {
			t.Errorf("FormatElapsed(%v) = %q, want %q", d, got, want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestTheFirstWriteErrorIsKept(t *testing.T) {
	p := New(failingWriter{}, failingWriter{})
	if p.WriteErr() != nil {
		t.Fatal("no error expected before writing")
	}
	if _, err := p.Stdout().Write([]byte("help")); err == nil || p.WriteErr() == nil {
		t.Fatal("a failed write through Stdout() must be kept")
	}
	p.Fatal("x")
	p.Fatal("y")
	if p.WriteErr() == nil || p.WriteErr().Error() != "closed" {
		t.Fatalf("WriteErr() = %v", p.WriteErr())
	}
}
