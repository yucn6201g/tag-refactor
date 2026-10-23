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
	snap := &inventory.Snapshot{EnabledRegions: make([]string, 17), ScannedRegions: make([]string, 17),
		FailedRegions: []inventory.FailedRegion{{Region: "me-south-1", Kind: "AccessDenied", Operation: "tag:GetResources"}},
		Resources:     make([]inventory.Resource, 412),
		Discovery: &inventory.Discovery{Status: inventory.StatusPartial, Regions: []inventory.DiscoveryRegion{
			{Region: "a-1", Status: inventory.StatusComplete},
			{Region: "me-south-1", Status: inventory.StatusUnavailable, Reason: "AccessDenied resource-explorer-2:Search"},
			{Region: "z-1\x1b", Status: inventory.StatusFailed, Reason: "AccessDenied resource-explorer-2:Search"},
		}}}
	report := &inventory.ScanReport{Snapshot: snap, RulesPath: "tagrules.yaml", NeverTagged: 3,
		Groups: []inventory.KeyVariantGroup{
			{NormalizedKey: "environment", TotalResources: 208, IsVariant: true, Spellings: []inventory.Spelling{{Key: "env", ResourceCount: 150}, {Key: "Env", ResourceCount: 58}}},
			{NormalizedKey: "owner", TotalResources: 3, Spellings: []inventory.Spelling{{Key: "Owner", ResourceCount: 3}}},
			{NormalizedKey: "production", TotalResources: 1, AbbreviationOnly: true, Spellings: []inventory.Spelling{{Key: "prod\x1b", ResourceCount: 1}}},
		},
		Missing: []inventory.MissingRequiredTag{
			{Key: "Owner", ResourceCount: 409, NotTaggableCount: 2},
			{Key: "Team", ResourceCount: 0},
		},
		Unsupported: []inventory.TypeCount{{ResourceType: "ssm:patch-compliance", Count: 2}},
	}

	p.ScanResult(report)

	want := "! Results are incomplete\n" +
		"    me-south-1: AccessDenied (tag:GetResources)\n" +
		"\n" +
		"Tag key variants (candidates; confirm them in your rule file)\n" +
		"  environment           208 resources\n" +
		"    env                   150\n" +
		"    Env                    58\n" +
		"  production              1 resource  (abbreviation match)\n" +
		"    prod\\u{1B}" + strings.Repeat(" ", 14) + "1\n" +
		"\n" +
		"Required tags missing (from tagrules.yaml)\n" +
		"  Owner         409 resources  (2 more on resource types that cannot be tagged)\n" +
		"\n" +
		"Resources with no user tags: 3\n" +
		"Untagged-resource discovery: partial (not checked in me-south-1, z-1\\u{1B}: AccessDenied resource-explorer-2:Search)\n" +
		"  Required tag counts are a lower bound.\n" +
		"Resource types not supported for tag changes: 1 type, 2 resources\n" +
		"  ssm:patch-compliance    2\n" +
		"\n" +
		"Summary: 412 resources scanned in 16 of 17 regions · 1 variant group · 1 required tag missing\n"
	if out.String() != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr: %q", errOut.String())
	}

	p2, out2, _ := newPrinter()
	p2.ScanResult(&inventory.ScanReport{Snapshot: &inventory.Snapshot{EnabledRegions: []string{"eu-west-1"}, ScannedRegions: []string{"eu-west-1"},
		Resources: make([]inventory.Resource, 1), Discovery: &inventory.Discovery{Status: inventory.StatusUnavailable,
			Regions: []inventory.DiscoveryRegion{{Region: "eu-west-1", Status: inventory.StatusUnavailable, Reason: "no Resource Explorer index"}}}},
		Groups: report.Groups[1:2], Missing: []inventory.MissingRequiredTag{}, RulesPath: "r.yaml"})
	want2 := "No issues found (untagged-resource discovery: unavailable)\n" +
		"Required tags missing (from r.yaml): none\n" +
		"\n" +
		"Resources with no user tags: 0\n" +
		"Untagged-resource discovery: unavailable (eu-west-1: no Resource Explorer index)\n" +
		"  Required tag counts are a lower bound.\n" +
		"\n" +
		"Summary: 1 resource scanned in 1 of 1 regions · 0 variant groups · 0 required tags missing\n"
	if out2.String() != want2 {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out2.String(), want2)
	}
	p3, out3, _ := newPrinter()
	p3.ScanResult(&inventory.ScanReport{Snapshot: &inventory.Snapshot{EnabledRegions: []string{"eu-west-1"}, ScannedRegions: []string{"eu-west-1"},
		Discovery: &inventory.Discovery{Status: inventory.StatusComplete}}})
	if !strings.HasPrefix(out3.String(), "No issues found\nRequired tags missing: not checked (no rule file)\n") ||
		!strings.Contains(out3.String(), "Untagged-resource discovery: complete\n\nSummary: 0 resources scanned in 1 of 1 regions · 0 variant groups\n") {
		t.Fatalf("stdout: %q", out3.String())
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

	p.PlanResult(&planner.Plan{ScannedResources: 3, ImpactAnalysisComplete: true, InventoryComplete: true})

	if out.String() != "No changes needed\nSummary: 3 resources scanned · 0 to change\n" {
		t.Fatalf("stdout: %q", out.String())
	}
}

// BR5.4: "No changes needed" is only said with exit code 0, so not when the
// tagging API could not read a region.
func TestPlanResultWithNothingToDoButAnIncompleteInventory(t *testing.T) {
	p, out, _ := newPrinter()

	p.PlanResult(&planner.Plan{ScannedResources: 3, ImpactAnalysisComplete: true, InventoryComplete: false})

	if out.String() != "Summary: 3 resources scanned · 0 to change\n" {
		t.Fatalf("stdout: %q", out.String())
	}
}

// BR5.4: "No issues found" is only said with exit code 0, so not when the
// tagging API could not read a region, whatever the discovery found.
func TestScanResultWithNoIssuesButFailedRegions(t *testing.T) {
	for _, d := range []*inventory.Discovery{nil, {Status: inventory.StatusComplete}, {Status: inventory.StatusPartial}} {
		p, out, _ := newPrinter()
		p.ScanResult(&inventory.ScanReport{Snapshot: &inventory.Snapshot{EnabledRegions: []string{"eu-west-1", "us-east-1"},
			ScannedRegions: []string{"eu-west-1", "us-east-1"}, Discovery: d,
			FailedRegions: []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}}}})
		if !strings.HasPrefix(out.String(), "! Results are incomplete\n    eu-west-1: AccessDenied (tag:GetResources)\n\nRequired tags missing:") ||
			strings.Contains(out.String(), "No issues found") {
			t.Fatalf("stdout: %q", out.String())
		}
	}
}

func TestMessagesProgressAndDebugGoToStandardError(t *testing.T) {
	p, out, errOut := newPrinter()
	p.APICall(awsaccess.CallRecord{Op: "tag:GetResources", Region: "us-east-1"}) // debug is off: nothing
	p.SetDebug(true)

	p.Account(awsaccess.AccountContext{AccountID: "123456789012", Region: "us-east-1"})
	p.RegionsWaiting([]string{"eu-west-1", "us-east-1"})
	p.RegionStarted("us-east-1")
	p.RegionDone("us-east-1", 1)
	p.RegionFailed("eu-west-1", "AccessDenied (tag:GetResources)")
	p.DiscoveryStarted([]string{"eu-west-1", "us-east-1"})
	p.DiscoveryRegionDone("us-east-1\x1b", "complete")
	p.Warn("careful\x1b")
	p.FileWarning("rules.yaml", 8, `"required" is not used in this version`)
	p.APICall(awsaccess.CallRecord{Op: "tag:GetResources", Region: "us-east-1", Duration: 132 * time.Millisecond, Retries: 0})
	p.APICall(awsaccess.CallRecord{Op: "iam:ListPolicies", Region: "us-east-1", Duration: time.Second, Retries: -1, ErrorKind: "Internal", CauseType: "*url.Error"})
	p.Error("access denied for tag:GetResources", "allow it")
	p.Fatal("stopped")
	p.Done(12*time.Second, 412)
	p.ExitCode(2, "issues found")

	want := "Account 123456789012 · region us-east-1\n" +
		"scan: waiting: eu-west-1, us-east-1\n" +
		"scan: us-east-1 started\n" +
		"scan: us-east-1 done (1 resource, elapsed 00:00)\n" +
		"scan: eu-west-1 failed (AccessDenied (tag:GetResources), elapsed 00:00)\n" +
		"scan: discovering never-tagged resources\n" +
		"scan: us-east-1\\u{1B} never-tagged resources: complete\n" +
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

	snap := &inventory.Snapshot{EnabledRegions: make([]string, 17), ScannedRegions: make([]string, 3),
		FailedRegions: []inventory.FailedRegion{{Region: "eu-west-1"}}, Discovery: &inventory.Discovery{Regions: []inventory.DiscoveryRegion{
			{Region: "a-1", Status: inventory.StatusComplete, Methods: []string{inventory.MethodConfig, inventory.MethodExplorer}},
			{Region: "b-1", Status: inventory.StatusComplete, Methods: []string{inventory.MethodAggregator}},
			{Region: "c-1", Status: inventory.StatusUnavailable},
		}}}
	p.ScanScope(snap)
	p.PlanScope(snap, true)
	p.PlanScope(snap, false)
	snap.Discovery = nil
	p.ScanScope(snap)

	want := "Checked: tagged resources in 2 of 17 regions (tagging API); never-tagged resources via AWS Config in 1 region " +
		"(without the global IAM types), Resource Explorer in 1 region, the Resource Explorer aggregator index for 1 region; not checked in 1 region\n" +
		"Checked: tagged resources in 2 of 17 regions (tagging API), and customer managed IAM policies attached to a user, role or group\n" +
		"Checked: tagged resources in 2 of 17 regions (tagging API); no IAM policies were read: nothing to change\n" +
		"Checked: tagged resources in 2 of 17 regions (tagging API); never-tagged resources were not looked for\n"
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
