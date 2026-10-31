package journal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the record of rollback (U7 step 10), written after the
// implementation: OpenForRollback and its lock, the three calls, the fields
// kept as they were read, the periodic save and the S3 record.

const rbRunID = "20261019T211003Z-7f3a"

// recordJSON is a record with fields this tool does not know, at the top and
// in an entry, an earlier rollback with a field of its own, and numbers,
// Unicode and nesting that a decode and encode would change.
const recordJSON = `{"version":1,"toolVersion":"v-old","runId":"` + rbRunID + `","accountId":"` + acct + `","planId":"p",` +
	`"startedAt":"2026-10-19T21:10:03Z","completed":false,` +
	`"future":{"precise":1.00000000000000011102230246251565,"big":123456789012345678901234567890,"list":[1,{"a":"<&>"}],"text":"日本語 é"},` +
	`"entries":[{"arn":"arn:aws:sqs:us-east-1:` + acct + `:a","region":"us-east-1","touchedKeys":["env"],` +
	`"beforeTags":{"env":"Prod"},"plannedAfterTags":{},"status":"applied","extra":{"n":1e400}}],` +
	`"rollbacks":[{"rollbackId":"20261020T080000Z-0001","startedAt":"2026-10-20T08:00:00Z","completed":true,"results":[],"note":"kept"}]}`

func rollbackLock(t *testing.T, j *Journal, loc Location) *LockHandle {
	t.Helper()
	h, err := j.Lock(context.Background(), acct, loc, PurposeRollback, "20261021T090000Z-beef")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// localRecord writes data as the local record of rbRunID and returns its path.
func localRecord(t *testing.T, j *Journal, data string) string {
	t.Helper()
	dir := j.defaultDir(acct)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, rbRunID+".json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func generic(t *testing.T, data []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	return m
}

// U7 security design §4: only a lock of rollback opens a record for
// rollback; a lock keeps its purpose.
func TestOpenForRollbackNeedsTheLockOfARollback(t *testing.T) {
	j := newJournal(t, nil, nil)
	localRecord(t, j, recordJSON)
	h, err := j.Lock(context.Background(), acct, Location{}, PurposeApply, "run")
	if err != nil || h.Purpose != PurposeApply {
		t.Fatalf("lock %+v %v", h, err)
	}
	if _, err := j.OpenForRollback(context.Background(), h, rbRunID); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("a lock of apply: %v", err)
	}
	if _, err := j.OpenForRollback(context.Background(), nil, rbRunID); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("no lock: %v", err)
	}
	_ = j.Release(context.Background(), h)
	r, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, Location{}), rbRunID)
	if err != nil {
		t.Fatal(err)
	}
	rec := r.Record()
	if rec.RunID != rbRunID || len(rec.Entries) != 1 || len(rec.Rollbacks) != 1 || !rec.Rollbacks[0].Completed ||
		!strings.HasSuffix(r.Where(), rbRunID+".json") {
		t.Fatalf("record %+v at %s", rec, r.Where())
	}
}

// BR1.2: a missing record is NotFound, another version UnsupportedVersion,
// and a file that is not a record InvalidInput naming it.
func TestOpenForRollbackRefusesWhatIsNotARecord(t *testing.T) {
	for _, c := range []struct {
		data string
		kind errkind.Kind
	}{
		{"", errkind.NotFound},
		{`{"version":2,"toolVersion":"v9"}`, errkind.UnsupportedVersion},
		{`{"version":1,`, errkind.InvalidInput},
		{`[1,2]`, errkind.InvalidInput},
		{`{"version":1,"entries":"no"}`, errkind.InvalidInput},
		{`{"version":1,"entries":[],"rollbacks":{"x":1}}`, errkind.InvalidInput},
	} {
		j := newJournal(t, nil, nil)
		if c.data != "" {
			localRecord(t, j, c.data)
		}
		_, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, Location{}), rbRunID)
		if errkind.KindOf(err) != c.kind {
			t.Errorf("%q: %v, want %s", c.data, err, c.kind)
		}
		if e, _ := errkind.As(err); c.kind == errkind.InvalidInput && (e.DetailID != "journal.notARecord" || e.DetailArgs[0] == "") {
			t.Errorf("%q: %+v", c.data, e)
		}
	}
	m := newMemStore()
	j := newJournal(t, m, nil)
	if _, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, s3Loc), rbRunID); errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("missing S3 record: %v", err)
	}
}

// NFR6.3: the fields this tool does not know, the entries, completed and the
// earlier rollbacks are written back as they were read; only this rollback
// is added.
func TestRollbackKeepsWhatItDoesNotChange(t *testing.T) {
	j := newJournal(t, nil, nil)
	path := localRecord(t, j, recordJSON)
	r, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, Location{}), rbRunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.BeginRollback(context.Background(), "20261021T090000Z-beef", time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordRollbackResults([]RollbackResult{{ARN: "arn:aws:sqs:us-east-1:" + acct + ":a", Status: "restored"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.CompleteRollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	before, after := generic(t, []byte(recordJSON)), generic(t, data)
	for _, k := range []string{"version", "toolVersion", "runId", "accountId", "planId", "startedAt", "completed", "future", "entries"} {
		if !reflect.DeepEqual(before[k], after[k]) {
			t.Errorf("%s changed: %v -> %v", k, before[k], after[k])
		}
	}
	rolls := after["rollbacks"].([]any)
	if len(rolls) != 2 || !reflect.DeepEqual(rolls[0], before["rollbacks"].([]any)[0]) {
		t.Fatalf("rollbacks %v", rolls)
	}
	own := rolls[1].(map[string]any)
	if own["rollbackId"] != "20261021T090000Z-beef" || own["startedAt"] != "2026-10-21T09:00:00Z" || own["completed"] != true ||
		len(own["results"].([]any)) != 1 {
		t.Fatalf("this rollback %v", own)
	}
	for _, kept := range []string{"1.00000000000000011102230246251565", "123456789012345678901234567890", "1e400", "日本語"} {
		if !strings.Contains(string(data), kept) {
			t.Errorf("%s was not kept as it was:\n%s", kept, data)
		}
	}
}

// Contract C10: BeginRollback saves at once, RecordRollbackResults adds or
// replaces by ARN, Close keeps the rollback not completed; the calls out of
// order are Internal, and a call after the record was closed does nothing.
func TestTheThreeCallsOfRollback(t *testing.T) {
	j := newJournal(t, nil, nil)
	path := localRecord(t, j, recordJSON)
	r, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, Location{}), rbRunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordRollbackResults(nil); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("before BeginRollback: %v", err)
	}
	if err := r.Close(context.Background()); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("Close before BeginRollback: %v", err)
	}
	r, _ = j.OpenForRollback(context.Background(), &LockHandle{AccountID: acct, Purpose: PurposeRollback}, rbRunID)
	if err := r.BeginRollback(context.Background(), "rb", time.Now()); err != nil {
		t.Fatal(err)
	}
	var saved RunRecord
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &saved)
	if len(saved.Rollbacks) != 2 || saved.Rollbacks[1].Completed || saved.Rollbacks[1].Results == nil {
		t.Fatalf("after BeginRollback: %+v", saved.Rollbacks)
	}
	if err := r.BeginRollback(context.Background(), "rb", time.Now()); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("a second BeginRollback: %v", err)
	}
	_ = r.RecordRollbackResults([]RollbackResult{{ARN: "a", Status: "failed", Reason: "readFailed"}, {ARN: "b", Status: "skipped"}})
	_ = r.RecordRollbackResults([]RollbackResult{{ARN: "a", Status: "restored"}})
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	saved = RunRecord{}
	_ = json.Unmarshal(data, &saved)
	own := saved.Rollbacks[1]
	if own.Completed || len(own.Results) != 2 || own.Results[0] != (RollbackResult{ARN: "a", Status: "restored"}) {
		t.Fatalf("after Close: %+v", own)
	}
	if err := r.RecordRollbackResults([]RollbackResult{{ARN: "c"}}); err != nil {
		t.Fatalf("after Close a result does nothing: %v", err)
	}
	if err := r.CompleteRollback(context.Background()); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("a second end: %v", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(data) {
		t.Fatal("the record changed after it was closed")
	}
}

// NFR6.4: while rollback writes, the record is saved every five seconds when
// it changed, never twice at once, and three failures in a row stop it.
func TestThePeriodicSaveOfRollback(t *testing.T) {
	for _, failures := range []int{2, 3} {
		restore(t)
		ticks := fakeTicker(t)
		m := newMemStore()
		j := newJournal(t, m, nil)
		uri := s3Loc.s3URI(acct, rbRunID+".json")
		_ = m.Put(context.Background(), uri, []byte(recordJSON))
		r, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, s3Loc), rbRunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.BeginRollback(context.Background(), "rb", time.Now()); err != nil {
			t.Fatal(err)
		}
		start := m.puts.Load()
		ticks <- time.Now() // nothing changed since BeginRollback
		var n atomic.Int32
		m.fail = func(string) error {
			if n.Add(1) <= int32(failures) {
				return errors.New("throttled")
			}
			return nil
		}
		for i := range failures + 1 {
			_ = r.RecordRollbackResults([]RollbackResult{{ARN: fmt.Sprint(i), Status: "restored"}})
			ticks <- time.Now()
			waitFor(t, func() bool {
				r.mu.Lock()
				defer r.mu.Unlock()
				return !r.sv.saving && n.Load() == int32(i+1)
			})
		}
		err = r.RecordRollbackResults([]RollbackResult{{ARN: "x", Status: "restored"}})
		if failures == 2 && (err != nil || r.Broken() || m.puts.Load() != start+3) {
			t.Errorf("2 failures: %v broken %v puts %d", err, r.Broken(), m.puts.Load()-start)
		}
		if failures == 3 && (errkind.KindOf(err) != errkind.StorageFailed || !r.Broken()) {
			t.Errorf("3 failures: %v", err)
		}
		_ = r.Close(context.Background())
	}
}

// NFR6.3, NFR7.2: in S3 the record of rollback is read once under the
// prefix and written by plain PutObject, the first save too (no conditional
// write); a save that fails is StorageFailed.
func TestTheS3RecordOfRollback(t *testing.T) {
	m := newMemStore()
	m.ifAbsent = errors.New("a conditional write must not be used")
	j := newJournal(t, m, nil)
	uri := s3Loc.s3URI(acct, rbRunID+".json")
	_ = m.Put(context.Background(), uri, []byte(recordJSON))
	h := &LockHandle{AccountID: acct, Location: s3Loc, Purpose: PurposeRollback}
	r, err := j.OpenForRollback(context.Background(), h, rbRunID)
	if err != nil || r.Where() != uri || !strings.HasPrefix(uri, "s3://records/runs/"+acct+"/") {
		t.Fatalf("open: %v %s", err, r.Where())
	}
	if err := r.BeginRollback(context.Background(), "rb", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.CompleteRollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := savedRecord(t, m, uri); len(rec.Rollbacks) != 2 || !rec.Rollbacks[1].Completed {
		t.Fatalf("S3 record %+v", rec.Rollbacks)
	}
	m.fail = func(string) error { return &errkind.Error{Kind: errkind.StorageFailed, Subject: uri} }
	r2, _ := j.OpenForRollback(context.Background(), h, rbRunID)
	if err := r2.BeginRollback(context.Background(), "rb2", time.Now()); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("a failed first save: %v", err)
	}
}

// NFR6.4: results recorded while the periodic saves run never break the
// record (run with -race in CI).
func TestResultsAndPeriodicSavesAtOnce(t *testing.T) {
	restore(t)
	ticks := fakeTicker(t)
	j := newJournal(t, nil, nil)
	path := localRecord(t, j, recordJSON)
	r, err := j.OpenForRollback(context.Background(), rollbackLock(t, j, Location{}), rbRunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.BeginRollback(context.Background(), "rb", time.Now()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				_ = r.RecordRollbackResults([]RollbackResult{{ARN: fmt.Sprintf("arn:%d:%d", g, i), Status: "restored"}})
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case ticks <- time.Now():
			}
		}
	}()
	wg.Wait()
	close(done)
	if err := r.CompleteRollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rec RunRecord
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &rec); err != nil || len(rec.Rollbacks[1].Results) != 200 {
		t.Fatalf("record %v: %d results", err, len(rec.Rollbacks[1].Results))
	}
}
