package cli_test

// Unit tests of the apply command (U6 step 10), written after the
// implementation, with the world of the U6 acceptance tests.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// BR1.2: a plan of another account writes nothing and names both accounts.
func TestApplyRefusesAPlanOfAnotherAccount(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	data, _ := os.ReadFile(plan)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	m["accountId"] = "111111111111"
	data, _ = json.Marshal(m)
	other := filepath.Join(t.TempDir(), "other.json")
	_ = os.WriteFile(other, data, 0o600)

	r := w.apply("yes\n", true, "--plan", other)

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "the plan is for account 111111111111, but the credentials are for account 123456789012")
	if w.writeCount() != 0 {
		t.Fatal("a plan of another account was written")
	}
	w.requireNoLock()
}

// BR2.3: without a terminal and without --yes, apply stops before asking,
// even when yes is piped.
func TestApplyWithoutATerminalNeedsYes(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("yes\n", false, "--plan", plan)

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, "standard input is not a terminal")
	requireContains(t, "stderr", r.stderr, "add --yes in scripts")
	w.requireNoLock()
	w.requireNoRecord()
}

// BR2.7: --allow-impact without --yes changes nothing but a warning; the
// confirmation decides.
func TestAllowImpactWithoutYesOnlyWarns(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	w.givenImpactPolicies()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("no\n", true, "--plan", plan, "--allow-impact")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "warning: --allow-impact works only with --yes")
	requireContains(t, "stderr", r.stderr, "Exit code: 0 (nothing changed)")
}

// BR1.4: a plan without changes asks nothing and makes no lock; conflicts
// give 2.
func TestAPlanWithoutChangesAppliesNothing(t *testing.T) {
	w := newApplyWorld(t)
	w.account.Add("us-east-1", arnQC, map[string]string{"Environment": "prod"})
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("", false, "--plan", plan)

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Nothing to apply")
	w.account.Add("us-east-1", arnQD, map[string]string{"env": "a", "Environment": "b"})
	conflict := w.givenSavedPlan(renameRules)
	r = w.apply("", false, "--plan", conflict)
	r.requireExit(t, 2)
	requireContains(t, "stderr", r.stderr, "Not changed: 1 conflict")
	if _, err := os.Stat(w.runsDir()); !os.IsNotExist(err) {
		t.Fatal("no folder of records may be made")
	}
}

// BR3.2: a lock left behind stops apply with its contents; --force-unlock
// removes it and goes on.
func TestALockLeftBehindNeedsForceUnlock(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	if err := os.MkdirAll(w.runsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(w.runsDir(), ".lock")
	_ = os.WriteFile(lock, []byte(`{"accountId":"123456789012","purpose":"apply","runId":"20261001T000000Z-dead","host":"old-pc","pid":7}`), 0o600)

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, "run 20261001T000000Z-dead (apply)")
	requireContains(t, "stderr", r.stderr, "host old-pc · process 7")
	r = w.apply("", false, "--plan", plan, "--yes", "--force-unlock")
	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Removed the lock")
	w.requireNoLock()
}

// BR1.3, BR6.3: --rules makes the plan now and says so; the rollback
// command repeats --record-to and --profile.
func TestApplyWithRulesAndTheRollbackCommand(t *testing.T) {
	w := newApplyWorld(t)
	w.env.SetProfile("dev", testutil.Profile{Region: "us-east-1", AccessKeyID: testutil.FakeAccessKeyID, SecretAccessKey: testutil.FakeSecretAccessKey})
	w.givenQueues()
	records := filepath.Join(t.TempDir(), "my records")

	r := w.apply("", false, "--rules", w.givenRuleFile(renameRules), "--yes", "--record-to", records, "--profile", "dev", "--lang", "ja")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "からその場で作成")
	// The path is printed sanitized: on Windows its backslashes are doubled.
	requireContains(t, "stderr", r.stderr, `--record-from "`+display.Sanitize(records)+`" --profile dev`)
	requireContains(t, "stdout", r.stdout, "書き換えた: 2 リソース")
	requireContains(t, "stderr", r.stderr, "終了コード: 0(書き換えました)")
}

// A wrong --record-to is a usage error before AWS is called.
func TestABadRecordLocationIsAUsageError(t *testing.T) {
	w := newApplyWorld(t)

	r := w.apply("", false, "--plan", "p.json", "--record-to", "s3://")

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, `--record-to "s3://" is not a folder or s3://<bucket>/<prefix>`)
	if len(w.fake.Calls()) != 0 {
		t.Fatal("AWS was called")
	}
	if !strings.Contains(r.stderr, "Fix: pass a folder") {
		t.Fatalf("stderr:\n%s", r.stderr)
	}
}
