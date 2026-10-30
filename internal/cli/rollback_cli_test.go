package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Unit tests of the rollback command (U7 step 10), written after the
// implementation: standard input, the record and its account, the lock
// against apply and another rollback, the operations called and what is
// never printed. They use the world of the U7 acceptance tests.

// BR4.3: without a terminal and without --yes, rollback stops before
// writing when something is to be written back, and leaves the record and
// the lock as they were.
func TestRollbackWithoutATerminalNeedsYes(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	_, data := readRollbackRecord(t, w.recordPath(runID))
	writes := w.writeCount()

	r := w.rollback("yes\n", false, "--run", runID)

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "rollback asks for a confirmation, but standard input is not a terminal")
	requireContains(t, "stderr", r.stderr, "--yes")
	if _, now := readRollbackRecord(t, w.recordPath(runID)); !bytes.Equal(now, data) || w.writeCount() != writes {
		t.Fatal("the record or the tags changed")
	}
	w.requireNoLock()
}

// BR1.2: a record that is not where rollback looks names the account and
// says to repeat --record-to as --record-from.
func TestRollbackOfAMissingRecordSaysWhereToLook(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules, "--record-to", filepath.Join(t.TempDir(), "records"))

	r := w.rollback("", false, "--run", runID, "--yes")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "the run record does not exist (account "+testutil.TestAccount+")")
	requireContains(t, "stderr", r.stderr, "pass the same value to --record-from")
	w.requireNoLock()
}

// BR1.3: a record of another account, given with --record-from, is refused
// with both accounts; nothing is read from AWS but the account.
func TestRollbackOfARecordOfAnotherAccount(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	data, _ := os.ReadFile(w.recordPath(runID))
	other := filepath.Join(t.TempDir(), "records")
	if err := os.MkdirAll(filepath.Join(other, testutil.TestAccount), 0o700); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), `"accountId": "`+testutil.TestAccount+`"`, `"accountId": "210987654321"`, 1)
	if err := os.WriteFile(filepath.Join(other, testutil.TestAccount, runID+".json"), []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := w.fake.Count(testutil.OpGetResources)

	r := w.rollback("", false, "--run", runID, "--yes", "--record-from", other)

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, "is for account 210987654321, but the credentials are for account "+testutil.TestAccount)
	if w.fake.Count(testutil.OpGetResources) != reads {
		t.Fatal("the tags were read for a record of another account")
	}
}

// NFR7.2: a rollback while an apply of the account runs, and a second
// rollback while one runs, stop at the lock without reading or changing the
// record.
func TestRollbackStopsAtTheLockOfAnotherRun(t *testing.T) {
	for _, first := range []string{"apply", "rollback"} {
		t.Run(first, func(t *testing.T) {
			w := newRollbackWorld(t)
			w.givenQueues()
			runID := w.givenApplied(renameRules)
			plan := ""
			if first == "apply" {
				// Another apply of the queues, as they were before.
				w.givenQueues()
				plan = w.givenSavedPlan(renameRules)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			w.account.OnWrite(func(testutil.Call) {
				once.Do(func() {
					close(started)
					<-release
				})
			})
			done := make(chan result, 1)
			go func() {
				if first == "apply" {
					done <- w.apply("", false, "--plan", plan, "--yes")
				} else {
					done <- w.rollback("", false, "--run", runID, "--yes")
				}
			}()
			select {
			case <-started:
			case r := <-done:
				t.Fatalf("the first %s ended before writing: exit %d\n%s", first, r.code, r.stderr)
			}
			_, data := readRollbackRecord(t, w.recordPath(runID))

			second := w.rollback("", false, "--run", runID, "--yes")

			close(release)
			<-done
			second.requireExit(t, 1)
			requireContains(t, "stderr", second.stderr, "another apply or rollback is running for account "+testutil.TestAccount)
			requireContains(t, "stderr", second.stderr, "("+first+")")
			if first == "apply" {
				if _, now := readRollbackRecord(t, w.recordPath(runID)); !bytes.Equal(now, data) {
					t.Fatal("the record changed")
				}
			}
			w.requireNoLock()
		})
	}
}

// NFR3.13: every operation rollback calls, with its record in S3, is
// allowed by docs/iam/apply-policy.json (the operations the fake AWS sees).
func TestRollbackCallsOnlyWhatTheApplyPolicyAllows(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	w.fake.HandleS3Buckets(testutil.NewS3Bucket("records-bucket", "eu-west-1"))
	runID := w.givenApplied(renameRules, "--record-to", "s3://records-bucket/runs")
	calls := len(w.fake.Calls())

	w.rollback("", false, "--run", runID, "--yes", "--record-from", "s3://records-bucket/runs").requireExit(t, 0)

	allowed := awsaccess.ApplyPermissions()
	seen := map[string]bool{}
	for _, c := range w.fake.Calls()[calls:] {
		seen[c.Op] = true
		if c.Op != awsaccess.OpGetCallerIdentity && !slices.Contains(allowed, awsaccess.IAMAction(c.Op)) {
			t.Errorf("rollback called %s, which the example policy does not allow", c.Op)
		}
	}
	for _, op := range []string{testutil.OpGetResources, testutil.OpTagResources, testutil.OpUntagResources, testutil.OpGetObject, testutil.OpPutObject, testutil.OpDeleteObject} {
		if !seen[op] {
			t.Errorf("rollback did not call %s", op)
		}
	}
}

// NFR8.26: the credentials and the AWS error texts appear nowhere: not on
// the screen, in the --debug lines, the record, the log or the lock; the log
// has no tag value.
func TestRollbackPrintsNoSecrets(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	w.account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		if arn == arnQB {
			return &testutil.ResourceFailure{StatusCode: http.StatusForbidden, Code: "AccessDeniedException",
				Message: "denied for " + testutil.FakeAccessKeyID + " SECRETTEXT"}
		}
		return nil
	})

	r := w.rollback("", false, "--run", runID, "--yes", "--debug")

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, "debug: tag:TagResources")
	files := []string{w.recordPath(runID)}
	logs, _ := filepath.Glob(filepath.Join(w.runsDir(), "*.log"))
	files = append(files, logs...)
	texts := []string{r.stdout, r.stderr}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, string(data))
	}
	for _, text := range texts {
		for _, secret := range []string{testutil.FakeAccessKeyID, testutil.FakeSecretAccessKey, testutil.FakeSessionToken, "SECRETTEXT", "denied for"} {
			requireNotContains(t, "output", text, secret)
		}
	}
	if len(logs) != 1 {
		t.Fatalf("%d log files", len(logs))
	}
	log, _ := os.ReadFile(logs[0])
	for _, value := range []string{"Prod", "dev", "prod"} {
		requireNotContains(t, "log", string(log), value)
	}
}

// U4 BR3.8: the confirmation and the result follow --lang.
func TestRollbackInJapanese(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	w.account.SetTags(arnQA, map[string]string{"Environment": "staging", "aws:cloudformation:stack-name": "s"})

	r := w.rollback("yes\n", true, "--run", runID, "--lang", "ja")

	r.requireExit(t, 2)
	for _, want := range []string{"アカウント " + testutil.TestAccount + " の apply " + runID + " を取り消します",
		"後から変えられた(飛ばす)", "--force で上書きして戻せます", "yes と入力すると戻します", "終了コード: 2"} {
		requireContains(t, "stderr", r.stderr, want)
	}
	for _, want := range []string{"戻した: 1 リソース", "飛ばした: 1 リソース", "apply のあとに変えられた"} {
		requireContains(t, "stdout", r.stdout, want)
	}
}

// BR1.4: a run whose apply wrote nothing has nothing to undo: exit 0, and
// the record stays as it was.
func TestRollbackOfARunThatWroteNothing(t *testing.T) {
	w := newRollbackWorld(t)
	w.givenQueues()
	runID := w.givenApplied(renameRules)
	path := w.recordPath(runID)
	var rec map[string]any
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &rec)
	for _, e := range rec["entries"].([]any) {
		e.(map[string]any)["status"], e.(map[string]any)["touchedKeys"] = "skipped", []any{}
	}
	data, _ = json.Marshal(rec)
	_ = os.WriteFile(path, data, 0o600)

	r := w.rollback("", false, "--run", runID)

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Nothing to undo: the apply wrote nothing.")
	requireContains(t, "stderr", r.stderr, "Exit code: 0 (nothing to undo)")
	if now, _ := os.ReadFile(path); !bytes.Equal(now, data) {
		t.Fatal("the record changed")
	}
}
