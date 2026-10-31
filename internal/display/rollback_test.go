package display

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/apply"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the output of rollback (U7 BR4.1, BR7.1, NFR11.9), written
// after the implementation: the confirmation and the result, in English and
// Japanese, with and without colors.

func rollbackSummary() apply.RollbackSummary {
	return apply.RollbackSummary{AccountID: "123456789012", RunID: "20261019T211003Z-7f3a",
		Record:    "/home/sato/.tag-refactor/runs/123456789012/20261019T211003Z-7f3a.json",
		ToRestore: 118, ChangedSinceApply: 2, AlreadyRestored: 3, Gone: 1}
}

func rollbackSummaryEnglish() string {
	row := func(label, count, note string) string { return fmt.Sprintf("  %-33s  %3s%s\n", label, count, note) }
	return "Account 123456789012: undo apply 20261019T211003Z-7f3a\n" +
		"Record: /home/sato/.tag-refactor/runs/123456789012/20261019T211003Z-7f3a.json\n" +
		"! The apply of this record stopped before the end: only what it wrote is undone\n" +
		"\n" +
		row("To restore", "118", "") +
		row("Changed after the apply (skipped)", "2", "   --force restores them too") +
		row("Already restored", "3", "") +
		row("Gone (skipped)", "1", "") +
		"\n"
}

// The confirmation of the example of the functional specification, in
// English: the stopped apply is marked with ! and words.
func TestTheConfirmationOfRollbackInEnglish(t *testing.T) {
	p, out, errOut := newPrinter()

	p.RollbackSummary(rollbackSummary())
	p.RollbackPrompt()

	if got := errOut.String(); got != rollbackSummaryEnglish()+"Type yes to undo the changes: " {
		t.Fatalf("stderr:\n%s", got)
	}
	if out.Len() != 0 {
		t.Fatalf("the confirmation goes to standard error only: %q", out.String())
	}
}

// The same in Japanese, with a record already undone, --force and the rows
// that only show when they count something; the counts stay in one column.
func TestTheConfirmationOfRollbackInJapanese(t *testing.T) {
	p, _, errOut := newPrinter()
	p.SetLang(Japanese)
	s := rollbackSummary()
	s.ApplyCompleted, s.UndoneAt, s.Force, s.OverLimit, s.Unreadable = true, "2026-10-20T08:00:00Z", true, 4, 5

	p.RollbackSummary(s)

	got := errOut.String()
	for _, want := range []string{
		"アカウント 123456789012 の apply 20261019T211003Z-7f3a を取り消します\n",
		"記録: /home/sato/.tag-refactor/runs/123456789012/20261019T211003Z-7f3a.json\n",
		"! この記録はすでに取り消し済みです(2026-10-20T08:00:00Z)。そのあとに同じ変更を apply していると、それも戻します\n",
		"後から変えられた(上書きして戻す)", "戻すと 50 を超える(飛ばす)", "読めなかった(失敗)", "118 件",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "途中で止まった") || strings.Contains(got, "--force で") {
		t.Errorf("a completed apply and --force have no such line:\n%s", got)
	}
	var widths []int
	for _, line := range strings.Split(got, "\n") {
		if strings.HasSuffix(line, " 件") {
			widths = append(widths, Width(line))
		}
	}
	if len(widths) != 6 {
		t.Fatalf("%d rows:\n%s", len(widths), got)
	}
	for _, w := range widths {
		if w != widths[0] {
			t.Fatalf("the counts are not in one column: %v\n%s", widths, got)
		}
	}
}

// NFR11.9: with colors the confirmation and the result mean the same once
// the colors are taken out; with NO_COLOR there are none.
func TestRollbackReadsTheSameWithoutColors(t *testing.T) {
	var out, errOut bytes.Buffer
	standIn(t, []io.Writer{&out, &errOut}, map[string]string{}, true)
	colored := New(&out, &errOut)
	colored.RollbackSummary(rollbackSummary())
	colored.RollbackResult(rollbackOutcome(), RollbackCommands{})
	var out2, errOut2 bytes.Buffer
	standIn(t, []io.Writer{&out2, &errOut2}, map[string]string{"NO_COLOR": "1"}, true)
	plain := New(&out2, &errOut2)
	plain.RollbackSummary(rollbackSummary())
	plain.RollbackResult(rollbackOutcome(), RollbackCommands{})
	if !strings.Contains(errOut.String(), "\x1b[") || StripColors(errOut.String()) != errOut2.String() ||
		StripColors(out.String()) != out2.String() || strings.Contains(errOut2.String()+out2.String(), "\x1b[") {
		t.Fatalf("colored:\n%q\nplain:\n%q", errOut.String(), errOut2.String())
	}
	if !strings.HasPrefix(errOut2.String(), rollbackSummaryEnglish()) {
		t.Fatalf("NO_COLOR stderr:\n%s", errOut2.String())
	}
}

func rollbackOutcome() *apply.RollbackOutcome {
	return &apply.RollbackOutcome{RunID: "20261019T211003Z-7f3a", RollbackID: "20261021T090000Z-beef", Started: true,
		Record: "/r/20261019T211003Z-7f3a.json", LogFile: "/r/20261021T090000Z-beef.log",
		Restored: 3, Unconfirmed: 1, AlreadyRestored: 2, NotWritten: 1, Stop: apply.StopSignal,
		Skipped: []*apply.RollbackTarget{
			{ARN: "arn:s1", Reason: apply.ReasonChangedSinceApply},
			{ARN: "arn:s2", Reason: apply.ReasonResourceNotFound, ErrorKind: "NotFound", Operation: "tag:TagResources"},
			{ARN: "arn:s3", Reason: apply.ReasonTagLimitExceeded, TagCount: 51},
		},
		Failed: []*apply.RollbackTarget{
			{ARN: "arn:f1", Reason: apply.ReasonReadFailed, ErrorKind: "AccessDenied", Operation: "tag:GetResources"},
			{ARN: "arn:f2", Reason: apply.ReasonWriteFailed, ErrorKind: "AccessDenied", Operation: "sqs:TagQueue"},
			{ARN: "arn:f3", Reason: apply.ReasonPartialWrite, ErrorKind: "AccessDenied", Operation: "sqs:UntagQueue"},
			{ARN: "arn:f4", Reason: apply.ReasonResultUnknown, ErrorKind: "Transient", Operation: "tag:TagResources"},
			{ARN: "arn:f5", Reason: apply.ReasonResultUnknown},
			{ARN: "arn:f6", Reason: apply.ReasonNotRestored},
		}}
}

func TestTheResultOfRollbackListsEveryReasonInWords(t *testing.T) {
	p, out, errOut := newPrinter()

	p.RollbackResult(rollbackOutcome(), RollbackCommands{
		Force: "tag-refactor rollback --run 20261019T211003Z-7f3a --force",
		Again: "tag-refactor rollback --run 20261019T211003Z-7f3a"})

	want := `Restored: 3 resources
Not confirmed after writing: 1 resource (the tagging API may show new tags late)
Already restored: 2 resources
Skipped: 3 resources
  arn:s1  changed after the apply
  arn:s2  no longer exists
  arn:s3  would have 51 tags, over the limit of 50
Failed: 6 resources
  arn:f1  its tags could not be read: AccessDenied (tag:GetResources)
  arn:f2  AccessDenied (sqs:TagQueue); nothing was written
  arn:f3  AccessDenied (sqs:UntagQueue); partly written
  arn:f4  Transient (tag:TagResources); the result is unknown
  arn:f5  stopped while writing; the result is unknown
  arn:f6  not restored when read back; run rollback again
Not undone (stopped): 1 resource
`
	if out.String() != want {
		t.Fatalf("stdout:\n%s", out.String())
	}
	wantErr := "Stopped by Ctrl+C: the calls that were running finished and were recorded.\n" +
		"Log: /r/20261021T090000Z-beef.log\n" +
		"Record: /r/20261019T211003Z-7f3a.json (this rollback is not completed: it stopped)\n" +
		"To restore the changed resources too: tag-refactor rollback --run 20261019T211003Z-7f3a --force\n" +
		"To undo the rest, run again: tag-refactor rollback --run 20261019T211003Z-7f3a\n"
	if errOut.String() != wantErr {
		t.Fatalf("stderr:\n%s", errOut.String())
	}
}

func TestTheResultOfRollbackInJapaneseAndItsFailures(t *testing.T) {
	p, out, errOut := newPrinter()
	p.SetLang(Japanese)
	o := &apply.RollbackOutcome{Started: true, Completed: true, Record: "/r/x.json", Restored: 2,
		Skipped:    []*apply.RollbackTarget{{ARN: "arn:s1", Reason: apply.ReasonChangedSinceApply}},
		SaveErr:    &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/x.json"},
		ReleaseErr: &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/.lock"},
		LogErr:     &errkind.Error{Kind: errkind.StorageFailed, Subject: "/r/x.log"}, Stop: apply.StopTransient}

	p.RollbackResult(o, RollbackCommands{Force: "tag-refactor rollback --run r --force"})

	if out.String() != "戻した: 2 リソース\nすでに戻っていた: 0 リソース\n飛ばした: 1 リソース\n  arn:s1  apply のあとに変えられた\n失敗: 0 リソース\n" {
		t.Fatalf("stdout %q", out.String())
	}
	for _, line := range []string{"止めました: AWS の書き込みの呼び出しが 10 回続けて失敗しました。", "ログファイルを書けませんでした: /r/x.log",
		"記録: /r/x.json\n", "最後に記録を保存できませんでした: /r/x.json", "印を消せませんでした: /r/.lock",
		"後から変えられたリソースも戻すには: tag-refactor rollback --run r --force"} {
		if !strings.Contains(errOut.String(), line) {
			t.Errorf("stderr does not contain %q:\n%s", line, errOut.String())
		}
	}
	for _, stop := range []apply.StopReason{apply.StopCredentials, apply.StopStorage, apply.StopInternal} {
		p.RollbackResult(&apply.RollbackOutcome{Started: true, Stop: stop}, RollbackCommands{})
	}
	if strings.Count(errOut.String(), "止めました") != 4 {
		t.Fatalf("stops:\n%s", errOut.String())
	}
}

// A stop before anything was undone shows no counts and says the record was
// not changed; the other messages of rollback.
func TestTheOtherMessagesOfRollback(t *testing.T) {
	p, out, errOut := newPrinter()

	p.RollbackResult(&apply.RollbackOutcome{Stop: apply.StopSignal}, RollbackCommands{})
	p.RollbackPrompt()
	p.RollbackCancelled()
	p.RollbackNothing()
	for i := 1; i <= 20; i++ {
		p.RollbackProgress(i, 20)
	}
	p.RollbackProgress(0, 0)

	want := "Stopped by Ctrl+C before anything was undone: the record was not changed.\n" +
		"Type yes to undo the changes: \nNothing was undone.\n" +
		"Nothing to undo: the apply wrote nothing.\n"
	if !strings.HasPrefix(errOut.String(), want) || out.Len() != 0 {
		t.Fatalf("stderr:\n%s\nstdout %q", errOut.String(), out.String())
	}
	if n := strings.Count(errOut.String(), "resources done"); n != 10 || !strings.Contains(errOut.String(), "rollback: 20 of 20 resources done") {
		t.Fatalf("%d progress lines:\n%s", n, errOut.String())
	}
}

// Every reason rollback can give has words in both languages, and the exit
// meanings of rollback are in the catalog.
func TestTheReasonsOfRollbackAreInTheCatalog(t *testing.T) {
	for _, r := range []apply.Reason{apply.ReasonChangedSinceApply, apply.ReasonResourceNotFound, apply.ReasonTagLimitExceeded,
		apply.ReasonReadFailed, apply.ReasonWriteFailed, apply.ReasonPartialWrite, apply.ReasonResultUnknown, apply.ReasonNotRestored} {
		for _, lang := range []Lang{English, Japanese} {
			if text := rollbackReason(lang, &apply.RollbackTarget{Reason: r, ErrorKind: "AccessDenied"}); text == string(r) || text == "" {
				t.Errorf("%s has no words in %v", r, lang)
			}
		}
	}
	if rollbackReason(English, &apply.RollbackTarget{Reason: "other"}) != "other" {
		t.Error("an unknown reason is shown as it is")
	}
	for _, id := range []string{ExitMeaningRestored, ExitMeaningNotUndone, ExitMeaningNothingToUndo, ExitMeaningRollbackSkipped, ExitMeaningRollbackFailed} {
		if _, ok := catalog[id]; !ok {
			t.Errorf("no message for %s", id)
		}
	}
}
