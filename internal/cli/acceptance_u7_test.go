package cli_test

// Acceptance tests for U7 (rollback: the undo of an apply from its run
// record, the confirmation, --force, --yes, --record-from and
// --force-unlock). Like the U6 tests, each one is an acceptance criterion
// written as Given/When/Then and run from the command entry point (cli.Run)
// against the HTTP-level fake AWS: apply really changes the in-memory tag
// account, and rollback changes it back. The home folder is the isolated
// test folder, so the user's ~/.tag-refactor is never touched.
//
// These tests were written and committed before the U7 implementation (team
// testing posture: acceptance criteria first).

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// fourKindsRules makes the four kinds of change on {env: Prod, obsolete: x}:
// the rename of env, the value Prod to prod, the required CostCenter and the
// removal of obsolete (U7 NFR4.2 (a)).
const fourKindsRules = renameRules + "required:\n  CostCenter:\n    default: \"100\"\nremove: [obsolete]\n"

// rollbackWorld is the Given part of the U7 tests: the world of the U6 tests,
// where apply writes the tag account that rollback then reads and writes.
type rollbackWorld struct {
	*applyWorld
}

func newRollbackWorld(t *testing.T) *rollbackWorld {
	t.Helper()
	return &rollbackWorld{applyWorld: newApplyWorld(t)}
}

// snapshot returns a copy of the tags of the resources, aws: keys included.
func (w *rollbackWorld) snapshot(arns ...string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, arn := range arns {
		out[arn] = w.account.Tags(arn)
	}
	return out
}

// requireSnapshot requires the tags of every resource of the snapshot to be
// exactly those of the snapshot, aws: keys included (U7 NFR4.2: no
// difference).
func (w *rollbackWorld) requireSnapshot(want map[string]map[string]string) {
	w.t.Helper()
	for arn, tags := range want {
		if got := w.account.Tags(arn); !maps.Equal(got, tags) {
			w.t.Fatalf("%s has %v, want the tags before apply %v", arn, got, tags)
		}
	}
}

// givenApplied applies the rule file with --yes and the extra arguments, and
// returns the run ID the result shows.
func (w *rollbackWorld) givenApplied(rules string, args ...string) string {
	w.t.Helper()
	r := w.apply("", false, append([]string{"--rules", w.givenRuleFile(rules), "--yes"}, args...)...)
	if r.code != 0 && r.code != 2 {
		w.t.Fatalf("apply ended with %d:\n%s", r.code, r.stderr)
	}
	m := rollbackCommand.FindStringSubmatch(r.stderr)
	if m == nil {
		w.t.Fatalf("apply showed no rollback command:\n%s", r.stderr)
	}
	return m[1]
}

// rollback runs the rollback command with stdin as the typed input and
// terminal saying whether standard input is a terminal.
func (w *rollbackWorld) rollback(stdin string, terminal bool, args ...string) result {
	w.t.Helper()
	return w.rollbackContext(context.Background(), stdin, terminal, args...)
}

func (w *rollbackWorld) rollbackContext(ctx context.Context, stdin string, terminal bool, args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC), 10*time.Millisecond)
	code := cli.Run(ctx, append([]string{"rollback"}, args...), &stdout, &stderr, cli.Deps{
		Transport:       w.fake,
		Now:             clock.Now,
		RetryDelay:      func(int) time.Duration { return 0 },
		Stdin:           strings.NewReader(stdin),
		StdinIsTerminal: func() bool { return terminal },
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// rollbackRecord is the run record with its rollbacks (contract C4) as the
// tests read it; Entries stays as JSON to compare it before and after.
type rollbackRecord struct {
	RunID     string
	Completed bool
	Entries   json.RawMessage
	Rollbacks []rollbackRun
}

type rollbackRun struct {
	RollbackID string
	StartedAt  string
	Completed  bool
	Results    []struct{ ARN, Status, Reason string }
}

// result returns the result of arn in the rollback run.
func (r rollbackRun) result(t *testing.T, arn string) (status, reason string) {
	t.Helper()
	for _, res := range r.Results {
		if res.ARN == arn {
			return res.Status, res.Reason
		}
	}
	t.Fatalf("the rollback %s has no result for %s: %+v", r.RollbackID, arn, r.Results)
	return "", ""
}

// readRollbackRecord reads the record file at path, with its raw bytes.
func readRollbackRecord(t *testing.T, path string) (rollbackRecord, []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec rollbackRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("the record is not valid JSON: %v\n%s", err, data)
	}
	return rec, data
}

// recordPath is the default path of the record of runID.
func (w *rollbackWorld) recordPath(runID string) string {
	return filepath.Join(w.runsDir(), runID+".json")
}

// requireSameJSON requires two JSON texts to mean the same.
func requireSameJSON(t *testing.T, what string, a, b []byte) {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("%s changed:\nbefore %s\nafter  %s", what, a, b)
	}
}

// AC5.1.1: Given an apply with the four kinds of change and no other change
// since, When rollback runs with its run ID and yes is typed, Then every tag,
// aws: keys included, is what it was before apply, the result is added to
// the rollbacks of the record, and the entries of the apply are unchanged.
func TestAcceptanceU7_AC5_1_1_RollbackRestoresEveryTag(t *testing.T) {
	w := newRollbackWorld(t)
	w.account.Add("us-east-1", arnQA, map[string]string{"env": "Prod", "obsolete": "x", "aws:cloudformation:stack-name": "s"})
	w.account.Add("us-east-1", arnQB, map[string]string{"env": "dev"})
	w.account.Add("us-east-1", arnQC, map[string]string{"Environment": "prod", "CostCenter": "100"})
	before := w.snapshot(arnQA, arnQB, arnQC)
	runID := w.givenApplied(fourKindsRules)
	if tags := w.account.Tags(arnQA); tags["Environment"] != "prod" || tags["CostCenter"] != "100" || tags["obsolete"] != "" {
		t.Fatalf("apply did not make the four changes: %v", tags)
	}
	applied, appliedData := readRollbackRecord(t, w.recordPath(runID))

	r := w.rollback("yes\n", true, "--run", runID)

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Type yes to undo the changes")
	requireContains(t, "stdout", r.stdout, "Restored: 2 resources")
	w.requireSnapshot(before)
	rec, data := readRollbackRecord(t, w.recordPath(runID))
	if len(rec.Rollbacks) != 1 || !rec.Rollbacks[0].Completed || rec.Rollbacks[0].RollbackID == "" || rec.Rollbacks[0].RollbackID == runID {
		t.Fatalf("rollbacks = %+v", rec.Rollbacks)
	}
	for _, arn := range []string{arnQA, arnQB} {
		if status, _ := rec.Rollbacks[0].result(t, arn); status != "restored" {
			t.Errorf("%s: %s, want restored", arn, status)
		}
	}
	requireSameJSON(t, "the entries of the apply", applied.Entries, rec.Entries)
	if rec.Completed != applied.Completed || !bytes.Contains(data, []byte(`"rollbacks"`)) || bytes.Contains(appliedData, []byte(`"rollbacks"`)) {
		t.Fatalf("completed %v (was %v)", rec.Completed, applied.Completed)
	}
	w.requireNoLock()
}

// AC5.1.2: Given an applied run, When rollback runs and something other than
// yes is typed, Then nothing is written back, the record is unchanged, and
// the exit code is 0.
func TestAcceptanceU7_AC5_1_2_AnythingButYesUndoesNothing(t *testing.T) {
	for _, typed := range []string{"no\n", "YES\n", ""} {
		t.Run(strings.TrimSpace(typed), func(t *testing.T) {
			w := newRollbackWorld(t)
			w.givenQueues()
			runID := w.givenApplied(renameRules)
			after := w.snapshot(arnQA, arnQB)
			writes := w.writeCount()
			_, data := readRollbackRecord(t, w.recordPath(runID))

			r := w.rollback(typed, true, "--run", runID)

			r.requireExit(t, 0)
			requireContains(t, "stderr", r.stderr, "Nothing was undone")
			if n := w.writeCount(); n != writes {
				t.Fatalf("%d writes were called", n-writes)
			}
			w.requireSnapshot(after)
			if _, now := readRollbackRecord(t, w.recordPath(runID)); !bytes.Equal(now, data) {
				t.Fatalf("the record changed:\n%s", now)
			}
			w.requireNoLock()
		})
	}
}

// AC5.1.3: Given a resource whose tags were changed by hand after apply, When
// rollback runs, Then that resource is skipped and listed as changed after
// the apply, the command with --force is shown, the others are restored, and
// the exit code is 2.
func TestAcceptanceU7_AC5_1_3_ResourceChangedAfterApplyIsSkipped(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	before := w.snapshot(arnQB)
	runID := w.givenApplied(renameRules)
	w.account.SetTags(arnQA, map[string]string{"Environment": "staging", "aws:cloudformation:stack-name": "s"})

	r := w.rollback("", false, "--run", runID, "--yes")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, regexp.QuoteMeta(arnQA)+`\s+changed after the apply`)
	requireContains(t, "stderr", r.stderr, "tag-refactor rollback --run "+runID+" --force")
	requireTags(t, w.account, arnQA, map[string]string{"Environment": "staging", "aws:cloudformation:stack-name": "s"})
	w.requireSnapshot(before)
	rec, _ := readRollbackRecord(t, w.recordPath(runID))
	if status, reason := rec.Rollbacks[0].result(t, arnQA); status != "skipped" || reason != "changedSinceApply" {
		t.Fatalf("qa: %s %s", status, reason)
	}
}

// AC5.1.4: Given the resource changed by hand of AC5.1.3, When rollback runs
// with --force, Then it is restored too: its touched keys are what they were
// before apply, and a key apply did not touch stays as it is.
func TestAcceptanceU7_AC5_1_4_ForceRestoresTheTouchedKeysOnly(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	w.account.SetTags(arnQA, map[string]string{"Environment": "staging", "owner": "x", "aws:cloudformation:stack-name": "s"})

	r := w.rollback("", false, "--run", runID, "--yes", "--force")

	r.requireExit(t, 0)
	requireTags(t, w.account, arnQA, map[string]string{"env": "Prod", "owner": "x", "aws:cloudformation:stack-name": "s"})
	requireTags(t, w.account, arnQB, map[string]string{"env": "dev"})
	for _, c := range w.fake.Calls() {
		if c.Op != testutil.OpTagResources && c.Op != testutil.OpUntagResources {
			continue
		}
		body, _ := json.Marshal(c.JSON)
		if strings.Contains(string(body), `"owner"`) || strings.Contains(strings.ToLower(string(body)), `"aws:`) {
			t.Fatalf("%s writes a key apply did not touch: %s", c.Op, body)
		}
	}
}

// AC5.1.5: Given a rollback with a resource restored, one changed after the
// apply and one that may not be written, When it ends, Then the restored,
// skipped and failed counts are shown with the list, the details are in the
// log file without tag values or AWS error texts, and the exit code is 1.
func TestAcceptanceU7_AC5_1_5_CountsOfTheResult(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	w.account.Add("us-east-1", arnQD, map[string]string{"env": "test"})
	runID := w.givenApplied(renameRules)
	w.account.SetTags(arnQA, map[string]string{"Environment": "staging", "aws:cloudformation:stack-name": "s"})
	w.account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		if arn == arnQB {
			return &testutil.ResourceFailure{StatusCode: http.StatusForbidden, Code: "AccessDeniedException",
				Message: "User arn:aws:iam::123456789012:user/tester is not authorized SECRETTEXT"}
		}
		return nil
	})

	r := w.rollback("", false, "--run", runID, "--yes")

	r.requireExit(t, 1)
	requireContains(t, "stdout", r.stdout, "Restored: 1 resource")
	requireContains(t, "stdout", r.stdout, "Skipped: 1 resource")
	requireContains(t, "stdout", r.stdout, "Failed: 1 resource")
	requireMatch(t, "stdout", r.stdout, regexp.QuoteMeta(arnQB)+`\s+AccessDenied`)
	requireTags(t, w.account, arnQD, map[string]string{"env": "test"})
	rec, _ := readRollbackRecord(t, w.recordPath(runID))
	logPath := filepath.Join(w.runsDir(), rec.Rollbacks[0].RollbackID+".log")
	requireContains(t, "stderr", r.stderr, filepath.Base(logPath))
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no log file: %v", err)
	}
	requireContains(t, "log", string(log), arnQB)
	requireContains(t, "log", string(log), arnQA)
	for _, secret := range []string{"SECRETTEXT", "not authorized", "staging", "Prod", testutil.FakeSecretAccessKey} {
		requireNotContains(t, "log", string(log), secret)
		requireNotContains(t, "stdout", r.stdout, "SECRETTEXT")
		requireNotContains(t, "stderr", r.stderr, "SECRETTEXT")
	}
	if status, reason := rec.Rollbacks[0].result(t, arnQB); status != "failed" || !strings.HasPrefix(reason, "writeFailed") {
		t.Fatalf("qb: %s %s", status, reason)
	}
}

// AC5.1.6: Given an apply that stopped half way (Ctrl+C after its first
// write, then a tag call that reached AWS for the next resource) and the lock
// a killed run would leave, When rollback runs, Then it stops at the lock and
// shows it; and When it runs again with --force-unlock, Then only what apply
// wrote is undone and every tag is what it was before apply.
func TestAcceptanceU7_AC5_1_6_StoppedApplyIsUndoneAfterForceUnlock(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	before := w.snapshot(arnQA, arnQB, arnQC)
	plan := w.givenSavedPlan(renameRules)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once sync.Once
	w.account.OnWrite(func(testutil.Call) {
		once.Do(func() {
			cancel()
			time.Sleep(200 * time.Millisecond)
		})
	})
	stopped := w.applyContext(ctx, "", false, "--plan", plan, "--yes")
	stopped.requireExit(t, 1)
	w.account.OnWrite(nil)
	runID := rollbackCommand.FindStringSubmatch(stopped.stderr)[1]
	rec, _ := readRollbackRecord(t, w.recordPath(runID))
	if rec.Completed {
		t.Fatal("the stopped apply must leave a record that is not completed")
	}
	// The resource apply had not written yet got its first tag call before
	// the process was killed: it is half way, and its record entry pending.
	if w.account.Tags(arnQB)["Environment"] == "" {
		w.account.SetTags(arnQB, map[string]string{"env": "dev", "Environment": "dev"})
	} else {
		w.account.SetTags(arnQA, map[string]string{"env": "Prod", "Environment": "prod", "aws:cloudformation:stack-name": "s"})
	}
	lock := filepath.Join(w.runsDir(), ".lock")
	if err := os.WriteFile(lock, []byte(`{"accountId":"`+testutil.TestAccount+`","purpose":"apply","runId":"`+runID+
		`","createdAt":"2026-10-20T09:00:00Z","host":"pc1","pid":4242,"toolVersion":"dev"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	writes := w.writeCount()

	held := w.rollback("", false, "--run", runID, "--yes")

	held.requireExit(t, 1)
	requireContains(t, "stderr", held.stderr, "another apply or rollback is running for account "+testutil.TestAccount)
	requireContains(t, "stderr", held.stderr, "--force-unlock")
	requireContains(t, "stderr", held.stderr, "4242")
	if w.writeCount() != writes {
		t.Fatal("rollback wrote while the lock was held")
	}

	r := w.rollback("yes\n", true, "--run", runID, "--force-unlock")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Removed the lock")
	requireContains(t, "stderr", r.stderr, "! The apply of this record stopped before the end")
	w.requireSnapshot(before)
	rec, _ = readRollbackRecord(t, w.recordPath(runID))
	if len(rec.Rollbacks) != 1 || !rec.Rollbacks[0].Completed {
		t.Fatalf("rollbacks = %+v", rec.Rollbacks)
	}
	w.requireNoLock()

	// A second rollback of the same run finds everything restored, asks
	// nothing even without a terminal, and adds a second result.
	again := w.rollback("", false, "--run", runID)

	again.requireExit(t, 0)
	requireContains(t, "stdout", again.stdout, "Already restored: 2 resources")
	rec, _ = readRollbackRecord(t, w.recordPath(runID))
	if len(rec.Rollbacks) != 2 || !rec.Rollbacks[1].Completed {
		t.Fatalf("rollbacks after the second run = %+v", rec.Rollbacks)
	}
	for _, arn := range []string{arnQA, arnQB} {
		if status, _ := rec.Rollbacks[1].result(t, arn); status != "alreadyRestored" {
			t.Errorf("%s: %s, want alreadyRestored", arn, status)
		}
	}
	w.requireSnapshot(before)
}

// BR7.4: Given applies with --record-to (a folder, then S3) and --profile,
// When the rollback command each one shows is run as it is (with --yes),
// Then it finds the record, signs with the profile and restores the tags.
func TestAcceptanceU7_BR7_4_TheCommandApplyShowsUndoesIt(t *testing.T) {
	for _, place := range []string{"folder", "s3"} {
		t.Run(place, func(t *testing.T) {
			w := newRollbackWorld(t)
			w.env.SetProfile("ops", testutil.Profile{Region: "us-east-1", AccessKeyID: "AKIAOPSPROFILE000000", SecretAccessKey: "ops-secret"})
			w.givenQueues()
			before := w.snapshot(arnQA, arnQB)
			recordTo := filepath.Join(t.TempDir(), "my records")
			if place == "s3" {
				w.fake.HandleS3Buckets(testutil.NewS3Bucket("records-bucket", "eu-west-1"))
				recordTo = "s3://records-bucket/runs"
			}
			applied := w.apply("", false, "--rules", w.givenRuleFile(renameRules), "--yes", "--record-to", recordTo, "--profile", "ops")
			applied.requireExit(t, 0)
			m := regexp.MustCompile(`To undo: tag-refactor (rollback .*)`).FindStringSubmatch(applied.stderr)
			if m == nil || !strings.Contains(m[1], "--record-from") || !strings.Contains(m[1], "--profile ops") {
				t.Fatalf("the rollback command of apply:\n%s", applied.stderr)
			}
			args := shellWords(m[1])[1:]
			calls := len(w.fake.Calls())

			r := w.rollback("", false, append(args, "--yes")...)

			r.requireExit(t, 0)
			w.requireSnapshot(before)
			for _, c := range w.fake.Calls()[calls:] {
				if c.AccessKeyID != "" && c.AccessKeyID != "AKIAOPSPROFILE000000" {
					t.Fatalf("%s was signed with %s, not the ops profile", c.Op, c.AccessKeyID)
				}
			}
			for _, out := range []string{r.stdout, r.stderr} {
				requireNotContains(t, "output", out, "ops-secret")
			}
		})
	}
}

// shellWords splits a command as the result shows it: words separated by
// spaces, with "..." holding spaces.
func shellWords(s string) []string {
	var words []string
	var cur strings.Builder
	quoted, inWord := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && quoted && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted, inWord = !quoted, true
		case c == ' ' && !quoted:
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

// AC6.1.1 (rollback): Given the tool, When the help of rollback is shown,
// Then it lists its options, where the record is read, what --force and
// --yes do, the lock and --force-unlock, and the exit codes.
func TestAcceptanceU7_AC6_1_1_HelpOfRollback(t *testing.T) {
	w := newRollbackWorld(t)

	r := w.run("rollback", "--help")

	r.requireExit(t, 0)
	for _, want := range []string{"Usage:", "--run", "--force", "--record-from", "--yes", "--force-unlock", "--profile", "--lang",
		".tag-refactor/runs", "--record-to", "changed after the apply", "Exit codes", "0 ", "1 ", "2 "} {
		requireContains(t, "rollback --help", r.stdout, want)
	}
	if len(w.fake.Calls()) != 0 {
		t.Fatal("help called AWS")
	}
}
