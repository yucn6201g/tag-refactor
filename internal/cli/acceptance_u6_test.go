package cli_test

// Acceptance tests for U6 (apply: the confirmation, --yes and
// --allow-impact, the checks before writing, the run record and the lock).
// Like the U1 to U5 tests, each one is an acceptance criterion written as
// Given/When/Then and run from the command entry point (cli.Run) against the
// HTTP-level fake AWS: an in-memory tag account that apply really changes,
// and an in-memory S3 bucket for --record-to s3://. The home folder is the
// isolated test folder, so the user's ~/.tag-refactor is never touched.
//
// These tests were written and committed before the U6 implementation (team
// testing posture: acceptance criteria first).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

const (
	arnQD = "arn:aws:sqs:us-east-1:123456789012:qd"
	arnQE = "arn:aws:sqs:us-east-1:123456789012:qe"
)

// renameRules renames env to Environment and writes Prod as prod: for
// {env: Prod} the plan has a rename and a value change (the first example of
// the expected tags in the U6 functional specification).
const renameRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      prod: [Prod]\n"

// applyWorld is the Given part of the U6 tests: the world of the acceptance
// tests with a tag account in us-east-1.
type applyWorld struct {
	*world
	account *testutil.TagAccount
}

func newApplyWorld(t *testing.T) *applyWorld {
	t.Helper()
	w := &applyWorld{world: newWorld(t), account: testutil.NewTagAccount()}
	w.fake.HandleTagAccount(w.account)
	return w
}

// givenQueues puts qa {env: Prod, aws:cloudformation:stack-name: s}, qb
// {env: dev} and qc {Environment: prod} (nothing to change) in us-east-1.
func (w *applyWorld) givenQueues() {
	w.account.Add("us-east-1", arnQA, map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s"})
	w.account.Add("us-east-1", arnQB, map[string]string{"env": "dev"})
	w.account.Add("us-east-1", arnQC, map[string]string{"Environment": "prod"})
}

// givenSavedPlan saves a plan of the rule file for us-east-1, as a user would
// with plan --out, and returns its path.
func (w *applyWorld) givenSavedPlan(rules string) string {
	w.t.Helper()
	path := filepath.Join(w.t.TempDir(), "plan.json")
	r := w.run("plan", "--rules", w.givenRuleFile(rules), "--region", "us-east-1", "--out", path)
	if r.code != 0 && r.code != 2 {
		w.t.Fatalf("plan --out failed with %d:\n%s", r.code, r.stderr)
	}
	return path
}

// apply runs the command line with stdin as the typed input and terminal
// saying whether standard input is a terminal.
func (w *applyWorld) apply(stdin string, terminal bool, args ...string) result {
	w.t.Helper()
	return w.applyContext(context.Background(), stdin, terminal, args...)
}

func (w *applyWorld) applyContext(ctx context.Context, stdin string, terminal bool, args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), 10*time.Millisecond)
	code := cli.Run(ctx, append([]string{"apply"}, args...), &stdout, &stderr, cli.Deps{
		Transport:       w.fake,
		Now:             clock.Now,
		RetryDelay:      func(int) time.Duration { return 0 },
		Stdin:           strings.NewReader(stdin),
		StdinIsTerminal: func() bool { return terminal },
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// runsDir is the default folder of the run records of the test account.
func (w *applyWorld) runsDir() string {
	return filepath.Join(w.env.Dir, ".tag-refactor", "runs", testutil.TestAccount)
}

// runRecord is the run record (contract C4) as the tests read it.
type runRecord struct {
	Version     int
	ToolVersion string
	RunID       string
	AccountID   string
	PlanID      string
	StartedAt   string
	Completed   *bool
	Entries     []recordEntry
}

type recordEntry struct {
	ARN              string
	Region           string
	TouchedKeys      []string
	BeforeTags       map[string]string
	PlannedAfterTags map[string]string
	AfterTags        map[string]string
	Status           string
	Reason           string
}

func (r runRecord) entry(t *testing.T, arn string) recordEntry {
	t.Helper()
	for _, e := range r.Entries {
		if e.ARN == arn {
			return e
		}
	}
	t.Fatalf("the record has no entry for %s: %+v", arn, r.Entries)
	return recordEntry{}
}

func parseRecord(t *testing.T, data []byte) runRecord {
	t.Helper()
	var rec runRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("the record is not valid JSON: %v\n%s", err, data)
	}
	return rec
}

// records reads every run record of a folder (files ending in .json).
func records(t *testing.T, dir string) []runRecord {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out []runRecord
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rec := parseRecord(t, data)
		if filepath.Base(p) != rec.RunID+".json" {
			t.Fatalf("the record %s holds the run %s", p, rec.RunID)
		}
		out = append(out, rec)
	}
	return out
}

// oneRecord requires exactly one run record in the default folder.
func (w *applyWorld) oneRecord() runRecord {
	w.t.Helper()
	recs := records(w.t, w.runsDir())
	if len(recs) != 1 {
		w.t.Fatalf("%d run records, want 1", len(recs))
	}
	return recs[0]
}

func (w *applyWorld) requireNoRecord() {
	w.t.Helper()
	if recs := records(w.t, w.runsDir()); len(recs) != 0 {
		w.t.Fatalf("no record may be made, found %+v", recs)
	}
}

func (w *applyWorld) requireNoLock() {
	w.t.Helper()
	if _, err := os.Stat(filepath.Join(w.runsDir(), ".lock")); !os.IsNotExist(err) {
		w.t.Fatalf("the lock is still there (err %v)", err)
	}
}

func (w *applyWorld) writeCount() int {
	return w.fake.Count(testutil.OpTagResources) + w.fake.Count(testutil.OpUntagResources)
}

func requireTags(t *testing.T, a *testutil.TagAccount, arn string, want map[string]string) {
	t.Helper()
	got := a.Tags(arn)
	if len(got) != len(want) {
		t.Fatalf("%s has %v, want %v", arn, got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s has %v, want %v", arn, got, want)
		}
	}
}

var rollbackCommand = regexp.MustCompile(`tag-refactor rollback --run (\d{8}T\d{6}Z-[0-9a-f]{4})`)

// AC4.1.1: Given a plan, When apply runs and something other than yes is
// typed, Then nothing is written, no record is made, and the exit code is 0.
func TestAcceptanceU6_AC4_1_1_AnythingButYesChangesNothing(t *testing.T) {
	for _, typed := range []string{"no\n", "YES\n", "y\n", ""} {
		t.Run(strings.TrimSpace(typed), func(t *testing.T) {
			w := newApplyWorld(t)
			w.givenQueues()
			plan := w.givenSavedPlan(renameRules)

			r := w.apply(typed, true, "--plan", plan)

			r.requireExit(t, 0)
			requireContains(t, "stderr", r.stderr, "Type yes to apply the changes")
			requireContains(t, "stderr", r.stderr, "Nothing was changed")
			if n := w.writeCount(); n != 0 {
				t.Fatalf("%d writes were called", n)
			}
			w.requireNoRecord()
			w.requireNoLock()
			requireTags(t, w.account, arnQA, map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s"})
		})
	}
}

// AC4.1.2: Given a saved plan, When apply runs with it and yes is typed, Then
// the tags become what the plan says, and the record is complete.
func TestAcceptanceU6_AC4_1_2_YesAppliesTheSavedPlan(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("  yes \n", true, "--plan", plan)

	r.requireExit(t, 0)
	requireTags(t, w.account, arnQA, map[string]string{"Environment": "prod", "aws:cloudformation:stack-name": "s"})
	requireTags(t, w.account, arnQB, map[string]string{"Environment": "dev"})
	requireTags(t, w.account, arnQC, map[string]string{"Environment": "prod"})
	requireContains(t, "stdout", r.stdout, "Applied: 2 resources")
	rec := w.oneRecord()
	if rec.Completed == nil || !*rec.Completed || len(rec.Entries) != 2 {
		t.Fatalf("record = %+v", rec)
	}
	for _, e := range rec.Entries {
		if e.Status != "applied" {
			t.Errorf("%s: status %q", e.ARN, e.Status)
		}
	}
	w.requireNoLock()
}

// AC4.1.3: Given a plan with no change that has impact, When apply runs with
// --yes, Then it writes without asking, and the summary is still shown on
// standard error.
func TestAcceptanceU6_AC4_1_3_YesWithoutImpactWritesWithoutAsking(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 0)
	requireNotContains(t, "stderr", r.stderr, "Type yes")
	requireContains(t, "stderr", r.stderr, "Resources to change: 2")
	requireTags(t, w.account, arnQB, map[string]string{"Environment": "dev"})
}

// givenImpactPolicies attaches a policy that uses aws:ResourceTag/env (the
// rename of env is affected, !) and one with a policy variable (undecided, ?).
func (w *applyWorld) givenImpactPolicies() {
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition, policyP2: policyVariableAndWildcard})
}

// AC4.1.4: Given a plan with changes that have impact, When apply runs, Then
// the confirmation shows them with ! and ? and words, and yes writes them.
func TestAcceptanceU6_AC4_1_4_ImpactIsShownWithSymbolsAndWords(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	w.givenImpactPolicies()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("yes\n", true, "--plan", plan)

	r.requireExit(t, 0)
	requireMatch(t, "stderr", r.stderr, `(?m)^  ! affected  `+regexp.QuoteMeta(policyP1))
	requireMatch(t, "stderr", r.stderr, `(?m)^  \? cannot decide  `+regexp.QuoteMeta(policyP2))
	requireContains(t, "stderr", r.stderr, "Type yes to apply the changes")
	requireTags(t, w.account, arnQB, map[string]string{"Environment": "dev"})
}

// AC4.1.5: Given a plan with changes that have impact, When apply runs with
// --yes only, Then nothing is written and the exit code is 2.
func TestAcceptanceU6_AC4_1_5_YesAloneStopsOnImpact(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	w.givenImpactPolicies()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 2)
	requireContains(t, "stderr", r.stderr, "--allow-impact")
	requireMatch(t, "stderr", r.stderr, `(?m)^  ! affected  `)
	if n := w.writeCount(); n != 0 {
		t.Fatalf("%d writes were called", n)
	}
	w.requireNoRecord()
	w.requireNoLock()
}

// AC4.1.6: Given a plan with changes that have impact, When apply runs with
// --yes and --allow-impact, Then it writes them.
func TestAcceptanceU6_AC4_1_6_YesWithAllowImpactWrites(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	w.givenImpactPolicies()
	plan := w.givenSavedPlan(renameRules)

	r := w.apply("", false, "--plan", plan, "--yes", "--allow-impact")

	r.requireExit(t, 0)
	requireTags(t, w.account, arnQA, map[string]string{"Environment": "prod", "aws:cloudformation:stack-name": "s"})
	requireMatch(t, "stderr", r.stderr, `(?m)^  ! affected  `)
}

// AC4.1.7: Given an apply that has ended, When its result is read, Then the
// rollback command with the run ID is shown, and the record U7 reads
// (contract C4) is where the command looks: --record-from repeats
// --record-to.
func TestAcceptanceU6_AC4_1_7_RollbackCommandAndRecord(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	var saved struct{ PlanID string }
	data, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(data, &saved)

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 0)
	m := rollbackCommand.FindStringSubmatch(r.stderr)
	if m == nil {
		t.Fatalf("no rollback command on stderr:\n%s", r.stderr)
	}
	rec := w.oneRecord()
	if rec.RunID != m[1] || rec.Version != 1 || rec.ToolVersion == "" || rec.AccountID != testutil.TestAccount ||
		rec.PlanID != saved.PlanID || rec.StartedAt == "" {
		t.Fatalf("record header = %+v (command run %s, plan %s)", rec, m[1], saved.PlanID)
	}
	if _, err := time.Parse(time.RFC3339, rec.StartedAt); err != nil {
		t.Fatalf("startedAt %q: %v", rec.StartedAt, err)
	}
	qa := rec.entry(t, arnQA)
	if strings.Join(qa.TouchedKeys, ",") != "Environment,env" || qa.Region != "us-east-1" ||
		qa.BeforeTags["env"] != "Prod" || qa.BeforeTags["aws:cloudformation:stack-name"] != "s" ||
		qa.PlannedAfterTags["Environment"] != "prod" || qa.AfterTags["Environment"] != "prod" {
		t.Fatalf("entry of qa = %+v", qa)
	}

	// With --record-to, the record goes to <folder>/<account>/ and the
	// command reads it from the same place.
	w.givenQueues()
	other := filepath.Join(t.TempDir(), "records")
	plan2 := w.givenSavedPlan(renameRules)
	r = w.apply("", false, "--plan", plan2, "--yes", "--record-to", other)
	r.requireExit(t, 0)
	m = rollbackCommand.FindStringSubmatch(r.stderr)
	if m == nil || !strings.Contains(r.stderr, "--record-from") {
		t.Fatalf("the rollback command must carry --record-from:\n%s", r.stderr)
	}
	if recs := records(t, filepath.Join(other, testutil.TestAccount)); len(recs) != 1 || recs[0].RunID != m[1] {
		t.Fatalf("records in --record-to: %+v", recs)
	}
}

// AC4.2.1: Given a resource whose tags were changed by hand after the plan
// was saved, When apply runs with the plan, Then that resource is skipped and
// listed at the end, the others are written, and the exit code is 2.
func TestAcceptanceU6_AC4_2_1_ResourceChangedAfterThePlanIsSkipped(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	w.account.SetTags(arnQA, map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s", "owner": "x"})

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, regexp.QuoteMeta(arnQA)+`\s+changed after the plan was made`)
	requireTags(t, w.account, arnQA, map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s", "owner": "x"})
	requireTags(t, w.account, arnQB, map[string]string{"Environment": "dev"})
	qa := w.oneRecord().entry(t, arnQA)
	if qa.Status != "skipped" || qa.Reason != "changedSincePlan" || len(qa.TouchedKeys) != 0 {
		t.Fatalf("entry of qa = %+v", qa)
	}
}

// AC4.2.2: Given a resource apply may not write, When apply runs, Then the
// others are written, the counts and the list are shown, and the details are
// in the log file, without tag values or AWS error texts.
func TestAcceptanceU6_AC4_2_2_DeniedResourceDoesNotStopTheOthers(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	w.account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		if arn == arnQB {
			return &testutil.ResourceFailure{StatusCode: http.StatusForbidden, Code: "AccessDeniedException",
				Message: "User arn:aws:iam::123456789012:user/tester is not authorized SECRETTEXT"}
		}
		return nil
	})

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 1)
	requireContains(t, "stdout", r.stdout, "Applied: 1 resource")
	requireContains(t, "stdout", r.stdout, "Failed: 1 resource")
	requireMatch(t, "stdout", r.stdout, regexp.QuoteMeta(arnQB)+`\s+AccessDenied`)
	requireTags(t, w.account, arnQA, map[string]string{"Environment": "prod", "aws:cloudformation:stack-name": "s"})
	rec := w.oneRecord()
	logPath := filepath.Join(w.runsDir(), rec.RunID+".log")
	requireContains(t, "stderr", r.stderr, filepath.Base(logPath))
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no log file: %v", err)
	}
	requireContains(t, "log", string(log), arnQB)
	requireContains(t, "log", string(log), "AccessDenied")
	for _, secret := range []string{"SECRETTEXT", "not authorized", "dev", "Prod"} {
		requireNotContains(t, "log", string(log), secret)
		requireNotContains(t, "stdout", r.stdout, "SECRETTEXT")
		requireNotContains(t, "stderr", r.stderr, "SECRETTEXT")
	}
	if qb := rec.entry(t, arnQB); qb.Status != "failed" || !strings.Contains(qb.Reason, "writeFailed") {
		t.Fatalf("entry of qb = %+v", qb)
	}
}

// AC4.2.3: Given apply runs, When the first tag is about to be written, Then
// the tags before writing are already in the record: by default in a local
// file, and with --record-to s3:// in the S3 bucket (no ACL, under the
// prefix and the account).
func TestAcceptanceU6_AC4_2_3_RecordIsSavedBeforeTheFirstWrite(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		w := newApplyWorld(t)
		w.givenQueues()
		plan := w.givenSavedPlan(renameRules)
		var before []runRecord
		var once sync.Once
		w.account.OnWrite(func(testutil.Call) { once.Do(func() { before = records(t, w.runsDir()) }) })

		w.apply("", false, "--plan", plan, "--yes").requireExit(t, 0)

		requirePendingRecord(t, before)
	})
	t.Run("s3", func(t *testing.T) {
		w := newApplyWorld(t)
		w.givenQueues()
		plan := w.givenSavedPlan(renameRules)
		bucket := testutil.NewS3Bucket("records-bucket", "eu-west-1")
		w.fake.HandleS3Buckets(bucket)
		prefix := "runs/" + testutil.TestAccount + "/"
		var before []runRecord
		var locks []string
		var once sync.Once
		w.account.OnWrite(func(testutil.Call) {
			once.Do(func() {
				for _, k := range bucket.Keys() {
					data, _ := bucket.Object(k)
					if strings.HasSuffix(k, ".json") {
						before = append(before, parseRecord(t, data))
					}
					if strings.HasSuffix(k, ".lock") {
						locks = append(locks, k)
					}
				}
				if _, err := os.Stat(filepath.Join(w.runsDir(), ".lock")); err != nil {
					t.Errorf("the local lock is missing while writing: %v", err)
				}
			})
		})

		r := w.apply("", false, "--plan", plan, "--yes", "--record-to", "s3://records-bucket/runs")

		r.requireExit(t, 0)
		requirePendingRecord(t, before)
		if len(locks) != 1 || locks[0] != prefix+".lock" {
			t.Fatalf("S3 locks while writing: %v", locks)
		}
		keys := bucket.Keys()
		if len(keys) != 1 || keys[0] != prefix+before[0].RunID+".json" {
			t.Fatalf("objects after the run: %v", keys)
		}
		data, _ := bucket.Object(keys[0])
		if rec := parseRecord(t, data); rec.Completed == nil || !*rec.Completed {
			t.Fatalf("the S3 record is not complete: %+v", rec)
		}
		for _, put := range bucket.Puts() {
			if testutil.HasACL(put) || !strings.HasPrefix(put.Key, prefix) {
				t.Errorf("PutObject %s sets an ACL or leaves the prefix", put.Key)
			}
		}
		w.requireNoLock()
		w.requireNoRecord()
		requireContains(t, "stderr", r.stderr, "--record-from s3://records-bucket/runs")
	})
}

func requirePendingRecord(t *testing.T, recs []runRecord) {
	t.Helper()
	if len(recs) != 1 {
		t.Fatalf("%d records before the first write, want 1", len(recs))
	}
	rec := recs[0]
	if rec.Completed == nil || *rec.Completed || len(rec.Entries) != 2 {
		t.Fatalf("record before the first write = %+v", rec)
	}
	for _, e := range rec.Entries {
		if e.Status != "pending" || len(e.TouchedKeys) == 0 || e.BeforeTags["env"] == "" || e.PlannedAfterTags["Environment"] == "" {
			t.Errorf("entry before the first write = %+v", e)
		}
	}
}

// AC4.2.4: Given a rule file that names keys starting with aws:, When apply
// runs with it, Then no tag starting with aws: is written or removed.
func TestAcceptanceU6_AC4_2_4_AWSTagsAreNeverChanged(t *testing.T) {
	w := newApplyWorld(t)
	w.account.Add("us-east-1", arnQA, map[string]string{"env": "dev", "aws:cloudformation:stack-name": "s", "aws:cloudformation:logical-id": "Q"})
	rules := w.givenRuleFile("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n" +
		"  \"aws:cloudformation:stack-name\":\n    aliases: [stack]\nremove: [\"aws:cloudformation:logical-id\"]\n")

	r := w.apply("", false, "--rules", rules, "--yes")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "aws:")
	requireTags(t, w.account, arnQA, map[string]string{"Environment": "dev", "aws:cloudformation:stack-name": "s", "aws:cloudformation:logical-id": "Q"})
	for _, c := range w.fake.Calls() {
		if c.Op != testutil.OpTagResources && c.Op != testutil.OpUntagResources {
			continue
		}
		body, _ := json.Marshal(c.JSON)
		if strings.Contains(strings.ToLower(string(body)), `"aws:`) {
			t.Fatalf("%s names a key starting with aws:: %s", c.Op, body)
		}
	}
}

// fiftyTags returns 50 tags without aws: keys: the given ones plus filler.
func fiftyTags(given map[string]string) map[string]string {
	tags := map[string]string{}
	for k, v := range given {
		tags[k] = v
	}
	for i := 0; len(tags) < testutil.MaxUserTags; i++ {
		tags["k"+strings.Repeat("0", 3-len(itoa(i)))+itoa(i)] = "v"
	}
	return tags
}

func itoa(i int) string {
	b := []byte{}
	for {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
		if i == 0 {
			return string(b)
		}
	}
}

// AC4.2.5: Given a resource that would have more than 50 tags, When apply
// runs, Then it is not written and is listed with the reason; a resource
// that stays at 50 is written (removing first, so it never has 51 on the
// way).
func TestAcceptanceU6_AC4_2_5_OverTheTagLimitIsSkipped(t *testing.T) {
	w := newApplyWorld(t)
	w.account.Add("us-east-1", arnQD, fiftyTags(map[string]string{"env": "Prod", "CostCenter": "7"}))
	w.account.Add("us-east-1", arnQE, fiftyTags(map[string]string{"Environment": "prod"}))
	plan := w.givenSavedPlan(renameRules + "required:\n  CostCenter:\n    default: \"100\"\n")

	r := w.apply("", false, "--plan", plan, "--yes")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, regexp.QuoteMeta(arnQE)+`\s+would have 51 tags, over the limit of 50`)
	if tags := w.account.Tags(arnQE); tags["CostCenter"] != "" || len(tags) != 50 {
		t.Fatalf("qe was changed: %v", tags)
	}
	qd := w.account.Tags(arnQD)
	if qd["Environment"] != "prod" || qd["env"] != "" || len(qd) != 50 {
		t.Fatalf("qd = %v, want Environment=prod without env, 50 tags", qd)
	}
	if e := w.oneRecord().entry(t, arnQE); e.Status != "skipped" || e.Reason != "tagLimitExceeded" {
		t.Fatalf("entry of qe = %+v", e)
	}
}

// AC4.2.6: Given a save that was stopped by force (a temporary file left
// half written by a killed run) and a run stopped with Ctrl+C while it
// writes, When the record files are read, Then every record is whole: the
// stopped run's record is valid JSON, not completed, with the resources it
// did not write still pending, and the rollback command is shown.
func TestAcceptanceU6_AC4_2_6_StoppedRunsLeaveWholeRecords(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	if err := os.MkdirAll(w.runsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	leftover := filepath.Join(w.runsDir(), ".20261020T070000Z-dead.tmp-12345")
	if err := os.WriteFile(leftover, []byte(`{"version":1,"entries":[{"arn":`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(leftover, old, old); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once sync.Once
	// Ctrl+C while the first write runs; AWS answers a moment later, as a
	// real call would, so the stop is seen before the next batch.
	w.account.OnWrite(func(testutil.Call) {
		once.Do(func() {
			cancel()
			time.Sleep(200 * time.Millisecond)
		})
	})

	r := w.applyContext(ctx, "", false, "--plan", plan, "--yes")

	r.requireExit(t, 1)
	if rollbackCommand.FindStringSubmatch(r.stderr) == nil {
		t.Fatalf("no rollback command after Ctrl+C:\n%s", r.stderr)
	}
	rec := w.oneRecord()
	statuses := map[string]int{}
	for _, e := range rec.Entries {
		statuses[e.Status]++
	}
	if rec.Completed == nil || *rec.Completed || statuses["applied"] != 1 || statuses["pending"] != 1 {
		t.Fatalf("record after Ctrl+C: completed %v, statuses %v\nstderr:\n%s", *rec.Completed, statuses, r.stderr)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Fatalf("the temporary file older than an hour was not cleaned up (err %v)", err)
	}
	w.requireNoLock()
}

// AC4.2.7: Given two applies of the same account at the same time, When both
// have ended, Then the later one stopped at the lock with exit code 1 and
// the contents of the lock, and the record of the first is whole.
func TestAcceptanceU6_AC4_2_7_ConcurrentApplyStopsAtTheLock(t *testing.T) {
	w := newApplyWorld(t)
	w.givenQueues()
	plan := w.givenSavedPlan(renameRules)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	w.account.OnWrite(func(testutil.Call) {
		once.Do(func() {
			close(started)
			<-release
		})
	})
	first := make(chan result, 1)
	go func() { first <- w.apply("", false, "--plan", plan, "--yes") }()
	select {
	case <-started:
	case r := <-first:
		t.Fatalf("the first apply ended before writing: exit %d\n%s", r.code, r.stderr)
	}

	second := w.apply("", false, "--plan", plan, "--yes")
	close(release)
	r := <-first

	second.requireExit(t, 1)
	requireContains(t, "stderr", second.stderr, "another apply or rollback is running for account "+testutil.TestAccount)
	requireContains(t, "stderr", second.stderr, "--force-unlock")
	r.requireExit(t, 0)
	rec := w.oneRecord()
	requireContains(t, "stderr", second.stderr, rec.RunID)
	if rec.Completed == nil || !*rec.Completed {
		t.Fatalf("the first record is not complete: %+v", rec)
	}
	w.requireNoLock()
}

// AC6.1.1 (apply): Given the tool, When the help of apply is shown, Then it
// lists its options, that one of --plan and --rules is needed, how --yes and
// --allow-impact go with impact, where the record goes, and the exit codes.
func TestAcceptanceU6_AC6_1_1_HelpOfApply(t *testing.T) {
	w := newApplyWorld(t)

	r := w.run("apply", "--help")

	r.requireExit(t, 0)
	for _, want := range []string{"Usage:", "--plan", "--rules", "--yes", "--allow-impact", "--record-to", "--force-unlock",
		"--profile", "--lang", "one of --plan and --rules", ".tag-refactor/runs", "Exit codes", "0 ", "1 ", "2 "} {
		requireContains(t, "apply --help", r.stdout, want)
	}
	if len(w.fake.Calls()) != 0 {
		t.Fatal("help called AWS")
	}
}
