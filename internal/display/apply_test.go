package display

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/apply"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Unit tests of the output of apply (U6 BR2.1, BR6.1, NFR11.8), written
// after the implementation: the confirmation and the result, in English and
// Japanese, with and without colors.

// applyPlan has two changes with impact (! and ?), a resource managed by
// IaC, a conflict and a required tag without a value.
func applyPlan() *planner.Plan {
	p := &planner.Plan{PlanID: "plan-20261019T120400Z-1a2b", AccountID: "123456789012",
		CreatedAt: time.Date(2026, 10, 19, 12, 4, 0, 0, time.UTC), Regions: []string{"us-east-1", "eu-west-1"},
		InventoryComplete: true, ImpactAnalysisComplete: true,
		Resources: []planner.PlannedResource{
			{ARN: "arn:aws:sqs:us-east-1:123456789012:qa", IaCManagedBy: "cloudformation:logging", Changes: []planner.Change{
				{ChangeID: "c-000001", Kind: planner.KindRenameKey, FromKey: "env", ToKey: "Environment"},
				{ChangeID: "c-000002", Kind: planner.KindAddRequired, ToKey: "Owner", ToValue: "x"}}},
		},
		Conflicts:         []planner.KeyConflict{{ResourceARN: "arn:c"}},
		UnsetRequiredTags: []planner.UnsetRequiredTag{{Key: "CostCenter", ARNs: []string{"arn:u"}}},
		ImpactFindings: []planner.ImpactFinding{
			{Verdict: planner.VerdictAffected, PolicyRef: "arn:aws:iam::123456789012:policy/DevAccess", StatementID: "AllowDev",
				ConditionKey: "aws:ResourceTag/env", ChangeIDs: []string{"c-000001"}},
			{Verdict: planner.VerdictUndecided, PolicyRef: "arn:aws:s3:::logs-bucket", StatementID: "DenyOthers",
				ConditionKey: "aws:ResourceTag/team"},
		},
	}
	return p
}

const applySummaryEnglish = `Account 123456789012 · 2 regions · plan plan-20261019T120400Z-1a2b (created 2026-10-19T12:04:00Z)

Resources to change: 1
  rename key       1
  value            0
  add              1
  remove           0

! Changes with impact (2)
  ! affected  arn:aws:iam::123456789012:policy/DevAccess  AllowDev  aws:ResourceTag/env  (rename key env -> Environment)
  ? cannot decide  arn:aws:s3:::logs-bucket  DenyOthers  aws:ResourceTag/team
⚠ IaC-managed resources: 1 (they will differ from their IaC definitions; the next deploy may change them back)
Not changed: 1 conflict · 1 required tag values not set
Record: /home/u/.tag-refactor/runs/123456789012/
`

func TestTheConfirmationScreenInEnglish(t *testing.T) {
	p, out, errOut := newPrinter()

	p.ApplySummary(applyPlan(), ApplyView{Record: "/home/u/.tag-refactor/runs/123456789012/"})
	p.ApplyPrompt()

	if got := errOut.String(); got != applySummaryEnglish+"Type yes to apply the changes: " {
		t.Fatalf("stderr:\n%s", got)
	}
	if out.Len() != 0 {
		t.Fatalf("the confirmation goes to standard error only: %q", out.String())
	}
}

func TestTheConfirmationScreenInJapaneseWithIncompleteMarks(t *testing.T) {
	p, _, errOut := newPrinter()
	p.SetLang(Japanese)
	plan := applyPlan()
	plan.InventoryComplete, plan.ImpactAnalysisComplete = false, false

	p.ApplySummary(plan, ApplyView{RuleFile: "tagrules.yaml", Record: "s3://records/runs/123456789012/"})

	for _, want := range []string{
		"アカウント 123456789012 · 2 リージョン · 変更案 plan-20261019T120400Z-1a2b(tagrules.yaml からその場で作成)\n",
		"変更するリソース: 1\n",
		"! 影響あり(2)\n",
		"  ! 影響あり  arn:aws:iam::123456789012:policy/DevAccess  AllowDev  aws:ResourceTag/env  (キー名の変更 env -> Environment)\n",
		"  ? 判定できない  arn:aws:s3:::logs-bucket  DenyOthers  aws:ResourceTag/team\n",
		"! 変更案の収集が不完全です", "! 変更案の影響分析が不完全です",
		"⚠ IaC で管理されているリソース: 1", "記録: s3://records/runs/123456789012/\n",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr does not contain %q:\n%s", want, errOut.String())
		}
	}
}

// NFR11.8: with colors the meaning is the same once the colors are taken
// out; without a terminal or with NO_COLOR there are none.
func TestTheConfirmationAndTheResultReadTheSameWithoutColors(t *testing.T) {
	var out, errOut bytes.Buffer
	standIn(t, []io.Writer{&out, &errOut}, map[string]string{}, true)
	colored := New(&out, &errOut)
	colored.ApplySummary(applyPlan(), ApplyView{Record: "/home/u/.tag-refactor/runs/123456789012/"})
	if !strings.Contains(errOut.String(), "\x1b[") || StripColors(errOut.String()) != applySummaryEnglish {
		t.Fatalf("colored stderr:\n%q", errOut.String())
	}
	var out2, errOut2 bytes.Buffer
	standIn(t, []io.Writer{&out2, &errOut2}, map[string]string{"NO_COLOR": "1"}, true)
	plain := New(&out2, &errOut2)
	plain.ApplySummary(applyPlan(), ApplyView{Record: "/home/u/.tag-refactor/runs/123456789012/"})
	if errOut2.String() != applySummaryEnglish {
		t.Fatalf("NO_COLOR stderr:\n%q", errOut2.String())
	}
}

func sampleResult() *apply.Result {
	return &apply.Result{RunID: "20261019T211003Z-7f3a", Record: "/r/20261019T211003Z-7f3a.json", Applied: 3, Unconfirmed: 1,
		LogFile: "/r/20261019T211003Z-7f3a.log", NotWritten: 2, Stop: apply.StopSignal,
		Skipped: []*apply.Target{
			{ARN: "arn:s1", Reason: apply.ReasonChangedSincePlan},
			{ARN: "arn:s2", Reason: apply.ReasonResourceNotFound},
			{ARN: "arn:s3", Reason: apply.ReasonTagLimitExceeded, TagCount: 51},
		},
		Failed: []*apply.Target{
			{ARN: "arn:f1", Reason: apply.ReasonWriteFailed, ErrorKind: "AccessDenied", Operation: "sqs:TagQueue"},
			{ARN: "arn:f2", Reason: apply.ReasonPartialWrite, ErrorKind: "AccessDenied", Operation: "sqs:UntagQueue"},
			{ARN: "arn:f3", Reason: apply.ReasonResultUnknown, ErrorKind: "Transient", Operation: "tag:TagResources"},
			{ARN: "arn:f4", Reason: apply.ReasonResultUnknown},
			{ARN: "arn:f5", Reason: apply.ReasonReadFailed, ErrorKind: "AccessDenied", Operation: "tag:GetResources"},
		}}
}

func TestTheResultListsEveryReasonInWords(t *testing.T) {
	p, out, errOut := newPrinter()

	p.ApplyResult(sampleResult(), "tag-refactor rollback --run 20261019T211003Z-7f3a")

	want := `Applied: 3 resources
Not confirmed after writing: 1 resource (the tagging API may show new tags late)
Skipped: 3 resources
  arn:s1  changed after the plan was made
  arn:s2  no longer exists
  arn:s3  would have 51 tags, over the limit of 50
Failed: 5 resources
  arn:f1  AccessDenied (sqs:TagQueue); nothing was written
  arn:f2  AccessDenied (sqs:UntagQueue); partly written
  arn:f3  Transient (tag:TagResources); the result is unknown
  arn:f4  stopped while writing; the result is unknown
  arn:f5  its tags could not be read: AccessDenied (tag:GetResources)
Not written (stopped): 2 resources
`
	if out.String() != want {
		t.Fatalf("stdout:\n%s", out.String())
	}
	for _, line := range []string{"Stopped by Ctrl+C", "Log: /r/20261019T211003Z-7f3a.log",
		"Record: /r/20261019T211003Z-7f3a.json (not completed: the run stopped)", "To undo: tag-refactor rollback --run 20261019T211003Z-7f3a"} {
		if !strings.Contains(errOut.String(), line) {
			t.Errorf("stderr does not contain %q:\n%s", line, errOut.String())
		}
	}
}

func TestTheResultInJapaneseAndItsFailures(t *testing.T) {
	p, out, errOut := newPrinter()
	p.SetLang(Japanese)
	res := &apply.Result{Applied: 1, Completed: true, Record: "/r/x.json",
		SaveErr:    &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/x.json"},
		ReleaseErr: &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/.lock"},
		LogErr:     &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/x.log"},
		Stop:       apply.StopTransient}

	p.ApplyResult(res, "")

	if out.String() != "書き換えた: 1 リソース\n" {
		t.Fatalf("stdout %q", out.String())
	}
	for _, line := range []string{"止めました: AWS の書き込みの呼び出しが 10 回続けて失敗しました。", "ログファイルを書けませんでした: /r/x.log",
		"記録: /r/x.json\n", "最後に記録を保存できませんでした: /r/x.json", "印を消せませんでした: /r/.lock"} {
		if !strings.Contains(errOut.String(), line) {
			t.Errorf("stderr does not contain %q:\n%s", line, errOut.String())
		}
	}
	for _, stop := range []apply.StopReason{apply.StopCredentials, apply.StopStorage, apply.StopInternal} {
		p.ApplyResult(&apply.Result{Stop: stop}, "")
	}
	if strings.Count(errOut.String(), "止めました") != 4 {
		t.Fatalf("stops:\n%s", errOut.String())
	}
}

func TestTheOtherMessagesOfApply(t *testing.T) {
	p, out, errOut := newPrinter()
	plan := applyPlan()

	p.ApplyCancelled()
	p.ApplyNothing(plan)
	p.ApplyImpactStop()
	p.AllowImpactIgnored()
	p.LocksHeld([]journal.HeldLock{
		{Where: "/h/.lock", Readable: true, Info: journal.LockInfo{RunID: "r1", Purpose: journal.PurposeApply, CreatedAt: "2026-10-19T21:10:03Z", Host: "pc", PID: 42}},
		{Where: "s3://b/.lock"},
	}, true)
	p.Notice(apply.NoticeAWSKey, "arn:x", "aws:y")
	for i := 1; i <= 20; i++ {
		p.ItemsProcessed(i, 20)
	}
	p.ItemsProcessed(0, 0)
	p.LockNotReleased(errkind.New(errkind.StorageFailed, "x"))

	want := "\nNothing was changed.\n" +
		"Nothing to apply: the plan has no change.\n" +
		"Not changed: 1 conflict · 1 required tag values not set\n" +
		"Stopped: the plan has changes with impact (! or ?), so nothing was changed.\n" +
		"  To apply them anyway, add --allow-impact to --yes; without --yes, check them on the confirmation screen.\n" +
		"warning: --allow-impact works only with --yes; decide on the confirmation screen\n" +
		"Lock /h/.lock: run r1 (apply) · created 2026-10-19T21:10:03Z · host pc · process 42\n" +
		"Removed the lock /h/.lock\n" +
		"Lock s3://b/.lock: its content could not be read\n" +
		"Removed the lock s3://b/.lock\n" +
		"warning: arn:x: the change of aws:y is not written; keys starting with aws: are never changed\n"
	if !strings.HasPrefix(errOut.String(), want) {
		t.Fatalf("stderr:\n%s", errOut.String())
	}
	if n := strings.Count(errOut.String(), "resources done"); n != 10 || !strings.Contains(errOut.String(), "apply: 20 of 20 resources done") {
		t.Fatalf("%d progress lines:\n%s", n, errOut.String())
	}
	if !strings.Contains(errOut.String(), "The lock could not be removed: -") || out.Len() != 0 {
		t.Fatalf("stdout %q", out.String())
	}
}

// Every warning of the journal and the applier has a message.
func TestTheWarningsOfApplyAreInTheCatalog(t *testing.T) {
	for _, id := range append(append([]string{}, journal.WarningIDs...), apply.NoticeAWSKey) {
		if _, ok := catalog[id]; !ok {
			t.Errorf("no message for %s", id)
		}
	}
}
