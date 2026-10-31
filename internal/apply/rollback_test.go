package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// Unit tests of rollback (U7 step 10), written after the implementation.
// They use the in-memory account of the apply tests (memAWS) and records
// written by the tests; rollback_scenario_test.go runs apply and rollback
// one after the other on the HTTP-level fake.

const rbRun = "20261019T211003Z-7f3a"

// rbWorld is one rollbacker with its in-memory account and its own home.
type rbWorld struct {
	t    *testing.T
	aws  *memAWS
	sink *sink
	home string
	j    *journal.Journal
	r    *Rollbacker
}

func newRBWorld(t *testing.T) *rbWorld {
	t.Helper()
	w := &rbWorld{t: t, aws: newMemAWS(), sink: &sink{}, home: t.TempDir()}
	w.j = journal.New(journal.Options{Home: w.home, ToolVersion: "v-test"})
	w.r = NewRollbacker(Options{AWS: w.aws, Journal: w.j, Sink: w.sink})
	return w
}

func sqsARN(name string) string { return "arn:aws:sqs:us-east-1:" + acct + ":" + name }

// entryOf is the record entry of a queue that apply changed from before to
// planned: its touched keys are the keys whose values differ.
func entryOf(name string, before, planned map[string]string, status journal.Status) journal.Entry {
	var touched []string
	for k := range before {
		if v, ok := planned[k]; !ok || v != before[k] {
			touched = append(touched, k)
		}
	}
	for k := range planned {
		if _, ok := before[k]; !ok {
			touched = append(touched, k)
		}
	}
	slices.Sort(touched)
	return journal.Entry{ARN: sqsARN(name), Region: "us-east-1", TouchedKeys: touched, BeforeTags: before,
		PlannedAfterTags: planned, Status: status}
}

// renamed is the entry of {env: Prod} renamed and changed to
// {Environment: prod}, the example of the functional specification.
func renamed(name string) journal.Entry {
	return entryOf(name, map[string]string{"env": "Prod"}, map[string]string{"Environment": "prod"}, journal.StatusApplied)
}

func record(entries ...journal.Entry) journal.RunRecord {
	return journal.RunRecord{Version: journal.Version, ToolVersion: "v-test", RunID: rbRun, AccountID: acct, PlanID: "plan-1",
		StartedAt: "2026-10-19T21:10:03Z", Completed: true, Entries: entries}
}

// recordPath is the default path of the record.
func (w *rbWorld) recordPath() string {
	return filepath.Join(journal.DefaultDir(w.home, acct), rbRun+".json")
}

// save writes the record to the default folder.
func (w *rbWorld) save(rec journal.RunRecord) {
	w.t.Helper()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		w.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(w.recordPath()), 0o700); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(w.recordPath(), data, 0o600); err != nil {
		w.t.Fatal(err)
	}
}

func (w *rbWorld) saved() journal.RunRecord {
	w.t.Helper()
	data, err := os.ReadFile(w.recordPath())
	if err != nil {
		w.t.Fatal(err)
	}
	var rec journal.RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		w.t.Fatal(err)
	}
	return rec
}

func (w *rbWorld) request() RollbackRequest { return RollbackRequest{RunID: rbRun, AccountID: acct} }

func (w *rbWorld) run(req RollbackRequest) *RollbackOutcome {
	w.t.Helper()
	out, err := w.r.Run(context.Background(), req, nil)
	if err != nil {
		w.t.Fatalf("rollback: %v", err)
	}
	return out
}

func (w *rbWorld) requireNoLock() {
	w.t.Helper()
	if _, err := os.Stat(filepath.Join(journal.DefaultDir(w.home, acct), ".lock")); !os.IsNotExist(err) {
		w.t.Fatalf("the lock is left (err %v)", err)
	}
}

func rbTarget(out *RollbackOutcome, name string) *RollbackTarget {
	for _, t := range out.Targets {
		if t.ARN == sqsARN(name) {
			return t
		}
	}
	return nil
}

func lastRun(rec journal.RunRecord) journal.RollbackRecord {
	if len(rec.Rollbacks) == 0 {
		return journal.RollbackRecord{}
	}
	return rec.Rollbacks[len(rec.Rollbacks)-1]
}

func resultOf(rb journal.RollbackRecord, name string) journal.RollbackResult {
	for _, r := range rb.Results {
		if r.ARN == sqsARN(name) {
			return r
		}
	}
	return journal.RollbackResult{}
}

// --- the checks of the record (NFR3.15, security design §2) ---

func TestTheChecksOfTheRecordStopBeforeAnyCall(t *testing.T) {
	good := func() journal.RunRecord { return record(renamed("a")) }
	long := strings.Repeat("é", 257)
	many := map[string]string{}
	for i := range 51 {
		many[fmt.Sprintf("k%02d", i)] = "v"
	}
	cases := map[string]struct {
		change func(r *journal.RunRecord)
		kind   errkind.Kind
		id     string
	}{
		"(a) another account":         {func(r *journal.RunRecord) { r.AccountID = "210987654321" }, errkind.AccountMismatch, "rollback.accountMismatch"},
		"(a) another run":             {func(r *journal.RunRecord) { r.RunID = "20261019T211003Z-0000" }, errkind.InvalidInput, "rollback.recordRunID"},
		"(b) not an ARN":              {func(r *journal.RunRecord) { r.Entries[0].ARN = "sqs:us-east-1:x" }, errkind.InvalidInput, "rollback.recordARN"},
		"(b) five parts":              {func(r *journal.RunRecord) { r.Entries[0].ARN = "arn:aws:sqs:us-east-1:" + acct }, errkind.InvalidInput, "rollback.recordARN"},
		"(b) empty service":           {func(r *journal.RunRecord) { r.Entries[0].ARN = "arn:aws::us-east-1:" + acct + ":q" }, errkind.InvalidInput, "rollback.recordARN"},
		"(b) empty resource":          {func(r *journal.RunRecord) { r.Entries[0].ARN = "arn:aws:sqs:us-east-1:" + acct + ":" }, errkind.InvalidInput, "rollback.recordARN"},
		"(b) another account's ARN":   {func(r *journal.RunRecord) { r.Entries[0].ARN = "arn:aws:sqs:us-east-1:210987654321:a" }, errkind.InvalidInput, "rollback.recordAccount"},
		"(b) the same ARN twice":      {func(r *journal.RunRecord) { r.Entries = append(r.Entries, r.Entries[0]) }, errkind.InvalidInput, "rollback.recordDuplicate"},
		"(c) not a region name":       {func(r *journal.RunRecord) { r.Entries[0].Region = "US-EAST-1" }, errkind.InvalidInput, "rollback.recordRegion"},
		"(c) region ends with letter": {func(r *journal.RunRecord) { r.Entries[0].Region = "us-east-x" }, errkind.InvalidInput, "rollback.recordRegion"},
		"(c) another region":          {func(r *journal.RunRecord) { r.Entries[0].Region = "eu-west-1" }, errkind.InvalidInput, "rollback.recordRegionMismatch"},
		"(d) an aws: key": {func(r *journal.RunRecord) {
			r.Entries[0].TouchedKeys = append(r.Entries[0].TouchedKeys, "AWS:cloudformation:stack-name")
			r.Entries[0].BeforeTags = map[string]string{"env": "Prod", "AWS:cloudformation:stack-name": "s"}
		}, errkind.InvalidInput, "rollback.recordAWSKey"},
		"(e) a key apply never saw": {func(r *journal.RunRecord) { r.Entries[0].TouchedKeys = []string{"owner"} }, errkind.InvalidInput, "rollback.recordTouched"},
		"(e) a key with one value": {func(r *journal.RunRecord) {
			r.Entries[0].BeforeTags["team"], r.Entries[0].PlannedAfterTags["team"] = "a", "a"
			r.Entries[0].TouchedKeys = append(r.Entries[0].TouchedKeys, "team")
		}, errkind.InvalidInput, "rollback.recordTouched"},
		"(f) a long value":   {func(r *journal.RunRecord) { r.Entries[0].BeforeTags["env"] = long }, errkind.InvalidInput, "rollback.recordLength"},
		"(f) a long key":     {func(r *journal.RunRecord) { r.Entries[0].PlannedAfterTags[strings.Repeat("k", 129)] = "v" }, errkind.InvalidInput, "rollback.recordLength"},
		"(f) over 50 before": {func(r *journal.RunRecord) { maps.Copy(r.Entries[0].BeforeTags, many) }, errkind.InvalidInput, "rollback.recordTooManyTags"},
		"(b) a bad skipped entry": {func(r *journal.RunRecord) {
			r.Entries = append(r.Entries, journal.Entry{ARN: "arn:aws:sqs:us-east-1:210987654321:other", Region: "us-east-1", Status: journal.StatusSkipped})
		}, errkind.InvalidInput, "rollback.recordAccount"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newRBWorld(t)
			rec := good()
			c.change(&rec)
			w.save(rec)
			before, _ := os.ReadFile(w.recordPath())

			_, err := w.r.Run(context.Background(), w.request(), nil)

			e, _ := errkind.As(err)
			if errkind.KindOf(err) != c.kind || e.DetailID != c.id || e.Subject != w.recordPath() {
				t.Fatalf("error %v (%s), want %s %s", err, e.DetailID, c.kind, c.id)
			}
			if w.aws.reads != 0 || len(w.aws.writes) != 0 {
				t.Fatalf("AWS was called: %d reads, %d writes", w.aws.reads, len(w.aws.writes))
			}
			if after, _ := os.ReadFile(w.recordPath()); string(after) != string(before) {
				t.Fatal("the record changed")
			}
			w.requireNoLock()
		})
	}
}

// A right record passes the checks: S3 buckets have no account or region,
// a global resource takes the region of its calls, values of 256 characters
// and 50 tags are within the limits, and keys apply removed or added count.
func TestARightRecordPassesTheChecks(t *testing.T) {
	fifty := map[string]string{"env": strings.Repeat("値", 256)}
	for i := 0; len(fifty) < 50; i++ {
		fifty[fmt.Sprintf("k%02d", i)] = "v"
	}
	fifty["aws:cloudformation:stack-name"] = "s"
	planned := maps.Clone(fifty)
	delete(planned, "env")
	planned["Environment"] = "prod"
	rec := record(
		journal.Entry{ARN: "arn:aws:s3:::bucket-b1", Region: "us-east-1", TouchedKeys: []string{"CostCenter"},
			BeforeTags: map[string]string{}, PlannedAfterTags: map[string]string{"CostCenter": "7"}, Status: journal.StatusApplied},
		journal.Entry{ARN: "arn:aws:iam::" + acct + ":role/r", Region: "us-east-1", TouchedKeys: []string{"obsolete"},
			BeforeTags: map[string]string{"obsolete": "x"}, PlannedAfterTags: map[string]string{}, Status: journal.StatusApplied},
		journal.Entry{ARN: sqsARN("full"), Region: "us-east-1", TouchedKeys: []string{"Environment", "env"},
			BeforeTags: fifty, PlannedAfterTags: planned, Status: journal.StatusApplied},
	)
	if err := checkRecord(&rec, acct, rbRun, "r.json"); err != nil {
		t.Fatalf("a right record was refused: %v", err)
	}
}

// --- the judgement and the tags to write (BR3.2 to BR3.6) ---

// The five rows of the example of the functional specification.
func TestTheJudgementOfTheFunctionalSpecification(t *testing.T) {
	for _, c := range []struct {
		current     map[string]string
		verdict     Verdict
		set, remove string
	}{
		{map[string]string{"Environment": "prod"}, VerdictRestore, "env=Prod", "Environment"},
		{map[string]string{"env": "Prod", "Environment": "prod"}, VerdictRestore, "", "Environment"},
		{map[string]string{"env": "Prod"}, VerdictAlreadyRestored, "", ""},
		{map[string]string{"Environment": "staging"}, VerdictChangedSinceApply, "env=Prod", "Environment"},
		{map[string]string{}, VerdictRestore, "env=Prod", ""},
	} {
		e := renamed("a")
		tg := &RollbackTarget{TouchedKeys: e.TouchedKeys, BeforeTags: e.BeforeTags, PlannedAfterTags: e.PlannedAfterTags}
		tg.Current = c.current
		if v := tg.judge(c.current); v != c.verdict {
			t.Errorf("%v: %s, want %s", c.current, v, c.verdict)
		}
		if c.verdict == VerdictAlreadyRestored {
			continue
		}
		tg.compute()
		var set []string
		for _, k := range slices.Sorted(maps.Keys(tg.TagsToSet)) {
			set = append(set, k+"="+tg.TagsToSet[k])
		}
		if strings.Join(set, ",") != c.set || strings.Join(tg.KeysToRemove, ",") != c.remove {
			t.Errorf("%v: set %v remove %v, want %s / %s", c.current, set, tg.KeysToRemove, c.set, c.remove)
		}
	}
}

// Only the touched keys count: other keys and aws: keys never change the
// verdict.
func TestOnlyTheTouchedKeysAreJudged(t *testing.T) {
	e := renamed("a")
	tg := &RollbackTarget{TouchedKeys: e.TouchedKeys, BeforeTags: e.BeforeTags, PlannedAfterTags: e.PlannedAfterTags}
	current := map[string]string{"Environment": "prod", "owner": "someone", "aws:cloudformation:stack-name": "new"}
	if v := tg.judge(current); v != VerdictRestore {
		t.Fatalf("verdict %s", v)
	}
	tg.Current = current
	tg.compute()
	if _, ok := tg.TagsToSet["owner"]; ok || slices.Contains(tg.KeysToRemove, "owner") || len(tg.TagsToSet) != 1 {
		t.Fatalf("a key apply did not touch is written: %+v", tg)
	}
}

// BR3.5: a resource that would have more than 50 tags once restored is
// skipped; one that has 50 all the way removes first.
func TestTheTagLimitOfRollback(t *testing.T) {
	fifty := map[string]string{"Environment": "prod"}
	for i := 0; len(fifty) < 50; i++ {
		fifty[fmt.Sprintf("k%02d", i)] = "v"
	}
	e := renamed("a")
	tg := &RollbackTarget{TouchedKeys: e.TouchedKeys, BeforeTags: e.BeforeTags, PlannedAfterTags: e.PlannedAfterTags, Current: fifty}
	tg.compute()
	if tg.Reason == ReasonTagLimitExceeded || !tg.RemoveFirst {
		t.Fatalf("fifty all the way: %+v", tg)
	}
	over := maps.Clone(fifty)
	over["k99"] = "v"
	delete(over, "k00")
	over["extra"] = "x"
	delete(over, "Environment")
	over["Environment"] = "prod"
	tg2 := &RollbackTarget{TouchedKeys: []string{"CostCenter"}, BeforeTags: map[string]string{"CostCenter": "1"},
		PlannedAfterTags: map[string]string{}, Current: over}
	tg2.compute()
	if tg2.Status != RollbackSkipped || tg2.Reason != ReasonTagLimitExceeded || tg2.TagCount != 52 {
		t.Fatalf("over the limit: %+v", tg2)
	}
}

// Security design §3: a resource the read leaves out has no tags when the
// record has no tag of its own before or after apply; otherwise it is gone.
func TestAResourceLeftOutOfTheReadHasNoTagsOrIsGone(t *testing.T) {
	for _, c := range []struct {
		name            string
		before, planned map[string]string
		ok              bool
	}{
		{"apply removed the last tag", map[string]string{"only": "1"}, map[string]string{}, true},
		{"apply tagged an untagged resource", map[string]string{"aws:x": "1"}, map[string]string{"aws:x": "1", "CostCenter": "7"}, true},
		{"tags before and after", map[string]string{"env": "Prod"}, map[string]string{"Environment": "prod"}, false},
	} {
		tg := &RollbackTarget{ARN: "arn:x", BeforeTags: c.before, PlannedAfterTags: c.planned}
		tags, ok := tg.currentTags(map[string]map[string]string{})
		if ok != c.ok || (ok && len(tags) != 0) {
			t.Errorf("%s: %v %v", c.name, tags, ok)
		}
		if found, ok := tg.currentTags(map[string]map[string]string{"arn:x": {"a": "b"}}); !ok || found["a"] != "b" {
			t.Errorf("%s: a resource in the answer has its tags", c.name)
		}
	}
}

// BR1.4: only the entries with touched keys that were not skipped are
// judged, whatever their status.
func TestTheTargetsOfARecord(t *testing.T) {
	failed := renamed("f")
	failed.Status = journal.StatusFailed
	pending := renamed("p")
	pending.Status = journal.StatusPending
	skipped := renamed("s")
	skipped.Status = journal.StatusSkipped
	nothing := journal.Entry{ARN: sqsARN("n"), Region: "us-east-1", Status: journal.StatusSkipped}
	rec := record(renamed("z"), failed, pending, skipped, nothing)
	var names []string
	for _, t := range newRollbackTargets(&rec) {
		names = append(names, t.ARN[strings.LastIndex(t.ARN, ":")+1:])
	}
	if strings.Join(names, ",") != "f,p,z" {
		t.Fatalf("targets %v", names)
	}
}

// --- units and batches (NFR5.19) ---

func TestUnitsHoldAHundredAndBatchesTwentyOfOneWrite(t *testing.T) {
	var ts []*RollbackTarget
	for i := range 250 {
		region := "us-east-1"
		if i%5 == 0 {
			region = "eu-west-1"
		}
		ts = append(ts, &RollbackTarget{ARN: fmt.Sprintf("arn:%03d", 999-i), Region: region,
			TagsToSet: map[string]string{"env": []string{"a", "b", "c"}[i%3]}, KeysToRemove: []string{"Environment"}})
	}
	regions, units := rollbackUnits(ts)
	if strings.Join(regions, ",") != "eu-west-1,us-east-1" || len(units["us-east-1"]) != 2 || len(units["eu-west-1"]) != 1 {
		t.Fatalf("regions %v, units %d and %d", regions, len(units["us-east-1"]), len(units["eu-west-1"]))
	}
	total := 0
	for _, region := range regions {
		last := ""
		for _, u := range units[region] {
			if len(u.targets) > 100 || u.targets[0].ARN <= last {
				t.Fatalf("unit of %d starting at %s after %s", len(u.targets), u.targets[0].ARN, last)
			}
			last = u.targets[len(u.targets)-1].ARN
			inUnit := map[*RollbackTarget]bool{}
			for _, tg := range u.targets {
				inUnit[tg] = true
			}
			for _, b := range rollbackBatches(u.targets) {
				total += len(b)
				key := batchKey(b[0].TagsToSet, b[0].KeysToRemove, b[0].RemoveFirst)
				for _, tg := range b {
					if len(b) > 20 || !inUnit[tg] || batchKey(tg.TagsToSet, tg.KeysToRemove, tg.RemoveFirst) != key {
						t.Fatal("a batch over 20, out of its unit, or with two writes")
					}
				}
			}
		}
	}
	if total != 250 {
		t.Fatalf("%d targets in batches", total)
	}
}

// --- the results of a write and its read back (security design §8) ---

func TestTheTableOfTheResults(t *testing.T) {
	denied := &errkind.Error{Kind: errkind.AccessDenied, Op: "sqs:TagQueue"}
	timeout := &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpTagResources}
	restoredTags := map[string]string{"env": "Prod"}
	writtenHalf := map[string]string{"env": "Prod", "Environment": "prod"}
	asBefore := map[string]string{"Environment": "prod"}
	for _, c := range []struct {
		name    string
		w       write
		back    map[string]string
		read    bool
		again   bool
		status  RollbackStatus
		reason  Reason
		confirm bool
	}{
		{"all written, restored", write{out: outApplied}, restoredTags, true, false, RollbackRestored, ReasonNone, true},
		{"all written, not yet", write{out: outApplied}, writtenHalf, true, true, RollbackPending, ReasonNone, false},
		{"all written, not read", write{out: outApplied}, nil, false, false, RollbackRestored, ReasonNone, false},
		{"first refused", write{out: outNothing, err: denied}, asBefore, true, false, RollbackFailed, ReasonWriteFailed, false},
		{"second refused", write{out: outPartial, err: denied}, writtenHalf, true, false, RollbackFailed, ReasonPartialWrite, false},
		{"gone", write{out: outGone, err: &errkind.Error{Kind: errkind.NotFound}}, nil, true, false, RollbackSkipped, ReasonResourceNotFound, false},
		{"no answer, restored", write{out: outDecide, err: timeout}, restoredTags, true, false, RollbackRestored, ReasonNone, true},
		{"no answer, as before", write{out: outDecide, err: timeout}, asBefore, true, false, RollbackFailed, ReasonWriteFailed, false},
		{"no answer, half", write{out: outDecide, err: timeout}, writtenHalf, true, false, RollbackFailed, ReasonResultUnknown, false},
		{"second no answer, as before", write{out: outDecidePartial, err: timeout}, asBefore, true, false, RollbackFailed, ReasonResultUnknown, false},
		{"no answer, not read", write{out: outDecide, err: timeout}, nil, false, false, RollbackFailed, ReasonResultUnknown, false},
		{"cancelled, restored", write{out: outCancelled, err: timeout}, restoredTags, true, false, RollbackFailed, ReasonResultUnknown, false},
	} {
		e := renamed("a")
		tg := &RollbackTarget{ARN: e.ARN, TouchedKeys: e.TouchedKeys, BeforeTags: e.BeforeTags, PlannedAfterTags: e.PlannedAfterTags,
			Current: asBefore, Status: RollbackPending}
		back := map[string]map[string]string{}
		if c.back != nil {
			back[e.ARN] = c.back
		}
		again := decideRollback(tg, c.w, back, c.read)
		if again != c.again || tg.Status != c.status || tg.Reason != c.reason || tg.Confirmed != c.confirm {
			t.Errorf("%s: again %v %s/%s confirmed %v", c.name, again, tg.Status, tg.Reason, tg.Confirmed)
		}
	}
	if r := (&RollbackTarget{ARN: "a", Status: RollbackRestored}).result(); r.Reason != "notConfirmed" {
		t.Errorf("unconfirmed result %+v", r)
	}
	if r := (&RollbackTarget{ARN: "a", Status: RollbackFailed, Reason: ReasonWriteFailed, ErrorKind: "AccessDenied"}).result(); r.Reason != "writeFailed:AccessDenied" {
		t.Errorf("failed result %+v", r)
	}
	if r := (&RollbackTarget{ARN: "a", Status: RollbackSkipped, Reason: ReasonResourceNotFound, ErrorKind: "NotFound"}).result(); r.Reason != "resourceNotFound" {
		t.Errorf("a resource gone at the write: %+v", r)
	}
	defer func() {
		if recover() == nil {
			t.Error("a write without an outcome must panic")
		}
	}()
	decideRollback(&RollbackTarget{}, write{}, nil, false)
}

// A write the second Ctrl+C cancelled, as the shared batch write reports
// it, stays unknown whatever the read back shows.
func TestACancelledWriteOfRollbackStaysUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &errkind.Error{Kind: errkind.Internal, Op: awsaccess.OpTagResources}
	tg := &RollbackTarget{ARN: "arn:a", TouchedKeys: []string{"k"}, BeforeTags: map[string]string{"k": "v"}, PlannedAfterTags: map[string]string{}}
	decideRollback(tg, write{out: writeOutcome(ctx, 0, e), err: e}, map[string]map[string]string{"arn:a": {"k": "v"}}, true)
	if tg.Status != RollbackFailed || tg.Reason != ReasonResultUnknown {
		t.Fatalf("%s/%s", tg.Status, tg.Reason)
	}
}

// --- the flows of Run ---

// rbPair saves a record of two renamed queues whose tags apply wrote, and
// puts their written tags in the account.
func (w *rbWorld) rbPair() {
	w.save(record(renamed("a"), renamed("b")))
	w.aws.put(sqsARN("a"), map[string]string{"Environment": "prod", "aws:x": "1"})
	w.aws.put(sqsARN("b"), map[string]string{"Environment": "prod"})
}

func TestARollbackRestoresAndRecords(t *testing.T) {
	w := newRBWorld(t)
	w.rbPair()
	var asked RollbackSummary
	req := w.request()
	req.Confirm = func(s RollbackSummary) (bool, error) { asked = s; return true, nil }

	out := w.run(req)

	if out.Restored != 2 || !out.Completed || out.ExitCode() != 0 || asked.ToRestore != 2 || !asked.ApplyCompleted {
		t.Fatalf("restored %d completed %v summary %+v", out.Restored, out.Completed, asked)
	}
	if tags := w.aws.tags[sqsARN("a")]; !maps.Equal(tags, map[string]string{"env": "Prod", "aws:x": "1"}) {
		t.Fatalf("a = %v", tags)
	}
	rb := lastRun(w.saved())
	if !rb.Completed || rb.RollbackID != out.RollbackID || resultOf(rb, "a").Status != "restored" || len(rb.Results) != 2 {
		t.Fatalf("rollback record %+v", rb)
	}
	if got := w.sink.done; len(got) == 0 || got[len(got)-1] != 2 {
		t.Fatalf("progress %v", got)
	}
	w.requireNoLock()
}

// BR1.4: a record without touched keys has nothing to undo: no read, no
// change. BR4.5: a record already restored is judged and recorded without a
// confirmation.
func TestNothingToUndoAndNothingToWrite(t *testing.T) {
	w := newRBWorld(t)
	w.save(record(journal.Entry{ARN: sqsARN("a"), Region: "us-east-1", Status: journal.StatusSkipped, Reason: "changedSincePlan"}))
	before, _ := os.ReadFile(w.recordPath())
	out := w.run(w.request())
	if after, _ := os.ReadFile(w.recordPath()); !out.NothingToUndo || out.ExitCode() != 0 || string(after) != string(before) || w.aws.reads != 0 {
		t.Fatalf("nothing to undo: %+v", out)
	}

	w2 := newRBWorld(t)
	w2.save(record(renamed("a")))
	w2.aws.put(sqsARN("a"), map[string]string{"env": "Prod"})
	req := w2.request()
	req.Confirm = func(RollbackSummary) (bool, error) { t.Fatal("asked although nothing is to write"); return false, nil }
	out = w2.run(req)
	if out.AlreadyRestored != 1 || !out.Completed || out.ExitCode() != 0 || len(w2.aws.writes) != 0 {
		t.Fatalf("already restored: %+v", out)
	}
	if rb := lastRun(w2.saved()); !rb.Completed || resultOf(rb, "a").Status != "alreadyRestored" {
		t.Fatalf("record %+v", rb)
	}
}

// BR4.2, BR4.3: a confirmation other than yes, or an error of the
// confirmation (no terminal), changes nothing.
func TestTheConfirmationCanStopTheRollback(t *testing.T) {
	for _, refuse := range []bool{true, false} {
		w := newRBWorld(t)
		w.rbPair()
		before, _ := os.ReadFile(w.recordPath())
		req := w.request()
		req.Confirm = func(RollbackSummary) (bool, error) {
			if refuse {
				return false, nil
			}
			return false, errkind.New(errkind.InvalidInput, "not a terminal")
		}
		out, err := w.r.Run(context.Background(), req, nil)
		if refuse && (err != nil || !out.Cancelled || out.ExitCode() != 0) || !refuse && errkind.KindOf(err) != errkind.InvalidInput {
			t.Fatalf("refuse %v: %+v %v", refuse, out, err)
		}
		if after, _ := os.ReadFile(w.recordPath()); string(after) != string(before) || len(w.aws.writes) != 0 {
			t.Fatal("the record or the tags changed")
		}
		w.requireNoLock()
	}
}

// BR3.3, NFR4.4: a resource changed after apply is skipped and listed; with
// --force it is written. A change made between the first judgement and the
// read again of its unit is seen too.
func TestResourcesChangedAfterApply(t *testing.T) {
	for _, force := range []bool{false, true} {
		w := newRBWorld(t)
		w.rbPair()
		w.aws.readErr = func(n int) error {
			if n == 2 { // the read again of the unit
				w.aws.tags[sqsARN("b")] = map[string]string{"Environment": "staging"}
			}
			return nil
		}
		req := w.request()
		req.Force = force
		out := w.run(req)
		b := rbTarget(out, "b")
		if !force && (b.Status != RollbackSkipped || b.Reason != ReasonChangedSinceApply || out.ExitCode() != 2 || out.Restored != 1) {
			t.Fatalf("without --force: %+v", b)
		}
		if force && (b.Status != RollbackRestored || w.aws.tags[sqsARN("b")]["env"] != "Prod" || out.ExitCode() != 0) {
			t.Fatalf("with --force: %+v %v", b, w.aws.tags[sqsARN("b")])
		}
	}
}

// BR3.1, BR3.6: a resource that is gone is skipped; a region that cannot be
// read fails its resources and the rest goes on.
func TestGoneAndUnreadableResources(t *testing.T) {
	w := newRBWorld(t)
	w.rbPair()
	delete(w.aws.tags, sqsARN("b"))
	out := w.run(w.request())
	if b := rbTarget(out, "b"); b.Status != RollbackSkipped || b.Reason != ReasonResourceNotFound || out.Summary.Gone != 1 {
		t.Fatalf("gone: %+v", b)
	}

	w2 := newRBWorld(t)
	w2.rbPair()
	w2.aws.readErr = func(n int) error {
		if n == 1 {
			return &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetResources}
		}
		return nil
	}
	out = w2.run(w2.request())
	a := rbTarget(out, "a")
	if a.Status != RollbackFailed || a.Reason != ReasonReadFailed || a.ErrorKind != "AccessDenied" || out.Summary.Unreadable != 2 ||
		out.ExitCode() != 1 || out.LogFile == "" || !strings.HasSuffix(out.LogFile, out.RollbackID+".log") {
		t.Fatalf("unreadable: %+v %+v", a, out.Summary)
	}
	if r := resultOf(lastRun(w2.saved()), "a"); r.Reason != "readFailed:AccessDenied" {
		t.Fatalf("record %+v", r)
	}
}

// BR5.6: a read again of a unit that fails fails the unit's resources
// without writing, and the next unit goes on.
func TestAFailedReadAgainFailsItsUnit(t *testing.T) {
	sizes(t, 1, 20, 1, 10)
	w := newRBWorld(t)
	w.rbPair()
	w.aws.readErr = func(n int) error {
		if n == 3 { // first reads 1 and 2, the read again of the unit of a
			return &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpGetResources}
		}
		return nil
	}
	out := w.run(w.request())
	if a, b := rbTarget(out, "a"), rbTarget(out, "b"); a.Reason != ReasonReadFailed || b.Status != RollbackRestored || out.Stop != StopNone {
		t.Fatalf("a %+v b %+v", a, b)
	}
}

// fakeWaits replaces the clock of the reads again: every wait is noted and
// ends at once, or never when hold is set; onWait is told of each.
func fakeWaits(t *testing.T, hold bool, onWait func()) func() []time.Duration {
	t.Helper()
	var mu sync.Mutex
	var waits []time.Duration
	old := newTimer
	newTimer = func(d time.Duration) (<-chan time.Time, func()) {
		mu.Lock()
		waits = append(waits, d)
		mu.Unlock()
		if onWait != nil {
			onWait()
		}
		ch := make(chan time.Time, 1)
		if !hold {
			ch <- time.Now()
		}
		return ch, func() {}
	}
	t.Cleanup(func() { newTimer = old })
	return func() []time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(waits)
	}
}

// lateAWS shows a written resource late for its next lag reads.
type lateAWS struct {
	*memAWS
	mu   sync.Mutex
	lag  int
	late map[string]map[string]string
}

func (l *lateAWS) TagResources(ctx context.Context, region string, arns []string, tags map[string]string) (awsaccess.WriteResult, error) {
	l.remember(arns)
	return l.memAWS.TagResources(ctx, region, arns, tags)
}

func (l *lateAWS) UntagResources(ctx context.Context, region string, arns []string, keys []string) (awsaccess.WriteResult, error) {
	l.remember(arns)
	return l.memAWS.UntagResources(ctx, region, arns, keys)
}

func (l *lateAWS) remember(arns []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.memAWS.mu.Lock()
	defer l.memAWS.mu.Unlock()
	for _, a := range arns {
		if _, ok := l.late[a]; !ok {
			l.late[a] = maps.Clone(l.tags[a])
		}
	}
}

func (l *lateAWS) GetTags(ctx context.Context, region string, arns []string) (map[string]map[string]string, error) {
	found, err := l.memAWS.GetTags(ctx, region, arns)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil || len(l.late) == 0 || l.lag == 0 {
		return found, err
	}
	for a, tags := range l.late {
		if _, ok := found[a]; ok {
			found[a] = maps.Clone(tags)
		}
	}
	l.lag--
	return found, nil
}

func newLateWorld(t *testing.T, lag int) (*rbWorld, *lateAWS) {
	w := newRBWorld(t)
	late := &lateAWS{memAWS: w.aws, lag: lag, late: map[string]map[string]string{}}
	w.r = NewRollbacker(Options{AWS: late, Journal: w.j, Sink: w.sink})
	w.rbPair()
	return w, late
}

// NFR5.24: tags read back late are read again after 1, 2 and 4 seconds; they
// are restored when a read again shows them, and not restored after three.
func TestTheReadsAgainAfterTheReadBack(t *testing.T) {
	for _, c := range []struct {
		lag    int
		waits  string
		status RollbackStatus
		reason Reason
	}{
		{1, "1s", RollbackRestored, ReasonNone},
		{3, "1s,2s,4s", RollbackRestored, ReasonNone},
		{4, "1s,2s,4s", RollbackFailed, ReasonNotRestored},
	} {
		waits := fakeWaits(t, false, nil)
		w, _ := newLateWorld(t, c.lag)
		out := w.run(w.request())
		var got []string
		for _, d := range waits() {
			got = append(got, d.String())
		}
		a := rbTarget(out, "a")
		if strings.Join(got, ",") != c.waits || a.Status != c.status || a.Reason != c.reason {
			t.Errorf("lag %d: waits %v, a %s/%s", c.lag, got, a.Status, a.Reason)
		}
		if c.reason == ReasonNotRestored && (out.ExitCode() != 1 || resultOf(lastRun(w.saved()), "a").Reason != "notRestored") {
			t.Errorf("notRestored: exit %d", out.ExitCode())
		}
	}
}

// NFR5.24: Ctrl+C during a wait ends it: what was not read as restored is
// failed (resultUnknown), the rollback is not completed.
func TestCtrlCDuringTheReadsAgain(t *testing.T) {
	stop := make(chan struct{})
	var once sync.Once
	fakeWaits(t, true, func() { once.Do(func() { close(stop) }) })
	w, _ := newLateWorld(t, 5)
	out, err := w.r.Run(context.Background(), w.request(), stop)
	if err != nil {
		t.Fatal(err)
	}
	a := rbTarget(out, "a")
	if a.Status != RollbackFailed || a.Reason != ReasonResultUnknown || out.Stop != StopSignal || out.Completed || out.ExitCode() != 1 {
		t.Fatalf("a %s/%s stop %q completed %v", a.Status, a.Reason, out.Stop, out.Completed)
	}
	if rb := lastRun(w.saved()); rb.Completed || resultOf(rb, "a").Reason != "resultUnknown" {
		t.Fatalf("record %+v", rb)
	}
}

// NFR7.2, BR2.3: a stop, expired credentials or a panic while the first
// pass judges ends the run before the confirmation: the record is
// unchanged and the lock removed.
func TestAStopWhileJudgingChangesNothing(t *testing.T) {
	cases := map[string]func(w *rbWorld, stop chan struct{}){
		"Ctrl+C": func(w *rbWorld, stop chan struct{}) {
			var once sync.Once
			w.aws.readErr = func(int) error { once.Do(func() { close(stop) }); return nil }
		},
		"credentials": func(w *rbWorld, _ chan struct{}) {
			w.aws.readErr = func(int) error { return &errkind.Error{Kind: errkind.CredentialsMissing} }
		},
		"panic": func(w *rbWorld, _ chan struct{}) {
			w.aws.readErr = func(int) error { panic("boom") }
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			sizes(t, 1, 20, 1, 10)
			w := newRBWorld(t)
			w.rbPair()
			before, _ := os.ReadFile(w.recordPath())
			stop := make(chan struct{})
			setup(w, stop)
			req := w.request()
			req.Confirm = func(RollbackSummary) (bool, error) { t.Error("asked"); return true, nil }

			out, err := w.r.Run(context.Background(), req, stop)

			switch name {
			case "Ctrl+C":
				if err != nil || out.Stop != StopSignal || out.Started || out.ExitCode() != 1 {
					t.Fatalf("out %+v err %v", out, err)
				}
			case "credentials":
				if errkind.KindOf(err) != errkind.CredentialsMissing {
					t.Fatalf("err %v", err)
				}
			default:
				if errkind.KindOf(err) != errkind.Internal {
					t.Fatalf("err %v", err)
				}
			}
			if after, _ := os.ReadFile(w.recordPath()); string(after) != string(before) || len(w.aws.writes) != 0 {
				t.Fatal("the record or the tags changed")
			}
			w.requireNoLock()
		})
	}
}

// BR5.5: the first Ctrl+C while writing lets the running unit finish and
// starts no new one; the record keeps the rollback not completed.
func TestTheFirstStopWhileWriting(t *testing.T) {
	sizes(t, 1, 20, 1, 10)
	w := newRBWorld(t)
	w.rbPair()
	stop := make(chan struct{})
	var once sync.Once
	w.aws.onWrite = func(string, []string) { once.Do(func() { close(stop) }) }
	out, err := w.r.Run(context.Background(), w.request(), stop)
	if err != nil {
		t.Fatal(err)
	}
	if out.Stop != StopSignal || out.Restored != 1 || out.NotWritten != 1 || out.Completed || out.ExitCode() != 1 {
		t.Fatalf("stop %q restored %d not written %d", out.Stop, out.Restored, out.NotWritten)
	}
	rb := lastRun(w.saved())
	if rb.Completed || len(rb.Results) != 1 {
		t.Fatalf("record %+v", rb)
	}
	w.requireNoLock()
}

// BR5.5, NFR4.2 (d): the second Ctrl+C cancels the running call: its
// resource is failed (resultUnknown), the record is saved once and nothing
// recorded after it fails, and a second rollback puts everything back.
func TestTheSecondStopAbandonsTheRollback(t *testing.T) {
	sizes(t, 1, 1, 100, 10)
	w := newRBWorld(t)
	w.rbPair()
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan struct{})
	var once sync.Once
	w.aws.onWrite = func(string, []string) {
		once.Do(func() {
			close(stop)
			cancel()
			time.Sleep(300 * time.Millisecond)
		})
	}
	out, err := w.r.Run(ctx, w.request(), stop)
	if err != nil {
		t.Fatal(err)
	}
	unknown := 0
	for _, tg := range out.Failed {
		if tg.Reason == ReasonResultUnknown {
			unknown++
		}
	}
	if unknown != 1 || out.Stop != StopSignal || out.SaveErr != nil || out.ReleaseErr != nil || out.ExitCode() != 1 {
		t.Fatalf("unknown %d stop %q save %v release %v", unknown, out.Stop, out.SaveErr, out.ReleaseErr)
	}
	if rb := lastRun(w.saved()); rb.Completed {
		t.Fatal("the rollback must stay not completed")
	}
	w.requireNoLock()

	w.aws.onWrite = nil
	again := w.run(w.request())
	if again.ExitCode() != 0 || len(w.saved().Rollbacks) != 2 {
		t.Fatalf("again: exit %d, %d rollbacks", again.ExitCode(), len(w.saved().Rollbacks))
	}
	for _, name := range []string{"a", "b"} {
		if tags := w.aws.tags[sqsARN(name)]; tags["env"] != "Prod" || tags["Environment"] != "" {
			t.Fatalf("%s = %v", name, tags)
		}
	}
}

// NFR5.23: nine write calls that end Transient do not stop the rollback; ten
// do. Expired credentials stop it at once.
func TestTheStopsOfRollbackWhileWriting(t *testing.T) {
	many := func(w *rbWorld, n int) {
		var entries []journal.Entry
		for i := range n {
			name := fmt.Sprintf("q%02d", i)
			entries = append(entries, entryOf(name, map[string]string{"env": fmt.Sprintf("v%02d", i)}, map[string]string{"Environment": "x"}, journal.StatusApplied))
			w.aws.put(sqsARN(name), map[string]string{"Environment": "x"})
		}
		w.save(record(entries...))
	}
	for _, failures := range []int{9, 10} {
		sizes(t, 1, 1, 1, 10)
		w := newRBWorld(t)
		many(w, 12)
		w.aws.callErr = func(_ string, n int) error {
			if n <= failures {
				return &errkind.Error{Kind: errkind.Transient}
			}
			return nil
		}
		out := w.run(w.request())
		if failures == 9 && (out.Stop != StopNone || out.Restored != 3) {
			t.Fatalf("9: stop %q restored %d", out.Stop, out.Restored)
		}
		if failures == 10 && (out.Stop != StopTransient || out.NotWritten != 2 || out.ExitCode() != 1) {
			t.Fatalf("10: stop %q not written %d", out.Stop, out.NotWritten)
		}
	}
	sizes(t, 1, 1, 1, 10)
	w := newRBWorld(t)
	many(w, 5)
	w.aws.callErr = func(_ string, n int) error {
		if n == 3 {
			return &errkind.Error{Kind: errkind.CredentialsMissing}
		}
		return nil
	}
	if out := w.run(w.request()); out.Stop != StopCredentials || out.Completed || out.ExitCode() != 1 {
		t.Fatalf("credentials: stop %q", out.Stop)
	}
}

// A panic of a region while writing ends the run as Internal; the record is
// saved and the lock removed.
func TestAPanicWhileWritingReleasesTheLock(t *testing.T) {
	w := newRBWorld(t)
	w.rbPair()
	w.aws.onWrite = func(string, []string) { panic("boom") }
	out := w.run(w.request())
	if out.Stop != StopInternal || out.Completed || out.ExitCode() != 1 {
		t.Fatalf("stop %q", out.Stop)
	}
	w.requireNoLock()
}

// BR2.1, BR2.2: a lock in the way is a Conflict before the record is read;
// --force-unlock removes it.
func TestTheLockOfRollback(t *testing.T) {
	w := newRBWorld(t)
	w.rbPair()
	if _, err := w.j.Lock(context.Background(), acct, journal.Location{}, journal.PurposeApply, "left-behind"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.r.Run(context.Background(), w.request(), nil); errkind.KindOf(err) != errkind.Conflict || w.aws.reads != 0 {
		t.Fatalf("held lock: %v, %d reads", err, w.aws.reads)
	}
	var removed []journal.HeldLock
	req := w.request()
	req.ForceUnlock = true
	req.OnLocksRemoved = func(l []journal.HeldLock) { removed = l }
	out := w.run(req)
	if len(removed) != 1 || removed[0].Info.RunID != "left-behind" || len(out.RemovedLocks) != 1 || out.Restored != 2 {
		t.Fatalf("force unlock: %+v %+v", removed, out)
	}
	w.requireNoLock()
}

// A record that does not exist is NotFound, with the lock removed.
func TestAMissingRecord(t *testing.T) {
	w := newRBWorld(t)
	_, err := w.r.Run(context.Background(), w.request(), nil)
	if errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("err %v", err)
	}
	w.requireNoLock()
}

// --- the exit codes (BR7.2) ---

func TestTheExitCodesOfRollback(t *testing.T) {
	failed := []*RollbackTarget{{}}
	for _, c := range []struct {
		out  RollbackOutcome
		want int
	}{
		{RollbackOutcome{}, 0},
		{RollbackOutcome{NothingToUndo: true}, 0},
		{RollbackOutcome{Cancelled: true, Failed: failed}, 0},
		{RollbackOutcome{Cancelled: true, ReleaseErr: errkind.New(errkind.StorageFailed, "x")}, 1},
		{RollbackOutcome{Skipped: failed}, 2},
		{RollbackOutcome{Skipped: failed, Failed: failed}, 1},
		{RollbackOutcome{NotWritten: 1}, 1},
		{RollbackOutcome{Stop: StopSignal}, 1},
		{RollbackOutcome{SaveErr: errkind.New(errkind.StorageFailed, "x")}, 1},
	} {
		if got := c.out.ExitCode(); got != c.want {
			t.Errorf("%+v: %d, want %d", c.out, got, c.want)
		}
	}
}

// NFR3.14: only the touched keys are written, never another key or an aws:
// key, even with --force.
func TestRollbackWritesTheTouchedKeysOnly(t *testing.T) {
	w := newRBWorld(t)
	w.save(record(renamed("a")))
	w.aws.put(sqsARN("a"), map[string]string{"Environment": "staging", "owner": "x", "aws:cloudformation:stack-name": "s"})
	req := w.request()
	req.Force = true
	w.run(req)
	for _, wr := range w.aws.writes {
		for k := range wr.set {
			if k != "env" {
				t.Errorf("%s sets %s", wr.op, k)
			}
		}
		if strings.Join(wr.keys, ",") != "" && strings.Join(wr.keys, ",") != "Environment" {
			t.Errorf("%s removes %v", wr.op, wr.keys)
		}
	}
	if !maps.Equal(w.aws.tags[sqsARN("a")], map[string]string{"env": "Prod", "owner": "x", "aws:cloudformation:stack-name": "s"}) {
		t.Fatalf("a = %v", w.aws.tags[sqsARN("a")])
	}
}

// BR4.1: the summary counts every verdict and says when the apply stopped
// half way and when the record was already undone.
func TestTheSummaryOfTheConfirmation(t *testing.T) {
	fifty := map[string]string{"Environment": "prod"}
	for i := 0; len(fifty) < 50; i++ {
		fifty[fmt.Sprintf("k%02d", i)] = "v"
	}
	w := newRBWorld(t)
	rec := record(renamed("restore"), renamed("changed"), renamed("already"), renamed("gone"),
		entryOf("over", map[string]string{"CostCenter": "1"}, map[string]string{}, journal.StatusApplied))
	rec.Completed = false
	rec.Rollbacks = []journal.RollbackRecord{{RollbackID: "r1", StartedAt: "2026-10-20T08:00:00Z", Completed: true}}
	w.save(rec)
	w.aws.put(sqsARN("restore"), map[string]string{"Environment": "prod"})
	w.aws.put(sqsARN("changed"), map[string]string{"Environment": "staging"})
	w.aws.put(sqsARN("already"), map[string]string{"env": "Prod"})
	w.aws.put(sqsARN("over"), fifty)
	var s RollbackSummary
	req := w.request()
	req.Confirm = func(got RollbackSummary) (bool, error) { s = got; return false, nil }
	w.run(req)
	want := RollbackSummary{AccountID: acct, RunID: rbRun, Record: w.recordPath(), UndoneAt: "2026-10-20T08:00:00Z",
		ToRestore: 1, ChangedSinceApply: 1, AlreadyRestored: 1, Gone: 1, OverLimit: 1}
	if s != want {
		t.Fatalf("summary %+v\nwant    %+v", s, want)
	}
}
