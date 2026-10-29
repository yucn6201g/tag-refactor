package journal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the run record and the lock (U6 step 10), written after the
// implementation. Every test works in its own temporary home folder.

const acct = "123456789012"

// memStore is an in-memory ObjectStore with failures and a gate.
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    atomic.Int32
	// fail, when set, decides the error of a Put or PutIfAbsent.
	fail func(uri string) error
	// gate, when set, is waited on by every Put.
	gate      chan struct{}
	ifAbsent  error
	deleteErr error
}

func newMemStore() *memStore { return &memStore{objects: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, uri string, data []byte) error {
	m.puts.Add(1)
	if m.gate != nil {
		<-m.gate
	}
	if m.fail != nil {
		if err := m.fail(uri); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[uri] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Get(_ context.Context, uri string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[uri]
	if !ok {
		return nil, &errkind.Error{Kind: errkind.NotFound, Subject: uri}
	}
	return data, nil
}

func (m *memStore) PutIfAbsent(ctx context.Context, uri string, data []byte) error {
	if m.ifAbsent != nil {
		return m.ifAbsent
	}
	m.mu.Lock()
	_, exists := m.objects[uri]
	m.mu.Unlock()
	if exists {
		return &errkind.Error{Kind: errkind.Conflict, Subject: uri}
	}
	return m.Put(ctx, uri, data)
}

func (m *memStore) Delete(_ context.Context, uri string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, uri)
	return nil
}

type warnings struct {
	mu  sync.Mutex
	ids []string
}

func (w *warnings) warn(id string, _ ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ids = append(w.ids, id)
}

func newJournal(t *testing.T, store *memStore, w *warnings) *Journal {
	t.Helper()
	opts := Options{Home: t.TempDir(), ToolVersion: "v-test", Now: func() time.Time { return time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC) }}
	if store != nil {
		opts.Store = store
	}
	if w != nil {
		opts.Warn = w.warn
	}
	return New(opts)
}

var s3Loc = Location{Kind: LocationS3, Bucket: "records", Prefix: "runs", Value: "s3://records/runs"}

// restore puts the file operations back after a test changed them.
func restore(t *testing.T) {
	t.Helper()
	ct, rn, att, wait, sl, sf, now := createTemp, rename, renameAttempts, renameWait, sleep, syncFolder, fileNow
	ticker, interval, limit := newTicker, autosaveInterval, failLimit
	t.Cleanup(func() {
		createTemp, rename, renameAttempts, renameWait, sleep, syncFolder, fileNow = ct, rn, att, wait, sl, sf, now
		newTicker, autosaveInterval, failLimit = ticker, interval, limit
	})
}

// --- the save that never leaves a broken record (NFR6.1, NFR9.9) ---

func TestWriteAtomicReplacesTheRecordAndLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeAtomic(dir, "run", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(dir, "run", []byte("new")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "run.json"))
	entries, _ := os.ReadDir(dir)
	if string(data) != "new" || len(entries) != 1 {
		t.Fatalf("record %q, folder %v", data, entries)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(dir, "run.json")); info.Mode().Perm() != 0o600 {
			t.Fatalf("record permissions %v, want 0600", info.Mode().Perm())
		}
	}
}

// failingTemp is a temporary file whose write, sync or close fails.
type failingTemp struct {
	*os.File
	failWrite, failSync bool
}

func (f failingTemp) Write(b []byte) (int, error) {
	if f.failWrite {
		half := len(b) / 2
		_, _ = f.File.Write(b[:half])
		return half, errors.New("disk full")
	}
	return f.File.Write(b)
}

func (f failingTemp) Sync() error {
	if f.failSync {
		return errors.New("sync failed")
	}
	return f.File.Sync()
}

// AC4.2.6, NFR6.1: a save stopped half way, at any step, leaves the
// previous record whole and no temporary file.
func TestASaveStoppedHalfWayLeavesThePreviousRecord(t *testing.T) {
	for name, setup := range map[string]func(){
		"create": func() {
			createTemp = func(string, string) (tempFile, error) { return nil, errors.New("no space") }
		},
		"write": func() {
			createTemp = func(dir, p string) (tempFile, error) {
				f, err := os.CreateTemp(dir, p)
				return failingTemp{File: f, failWrite: true}, err
			}
		},
		"sync": func() {
			createTemp = func(dir, p string) (tempFile, error) {
				f, err := os.CreateTemp(dir, p)
				return failingTemp{File: f, failSync: true}, err
			}
		},
		"rename": func() { rename = func(string, string) error { return errors.New("in use") } },
	} {
		t.Run(name, func(t *testing.T) {
			restore(t)
			sleep = func(time.Duration) {}
			dir := t.TempDir()
			if err := writeAtomic(dir, "run", []byte(`{"version":1}`)); err != nil {
				t.Fatal(err)
			}
			setup()

			err := writeAtomic(dir, "run", []byte(`{"version":1,"entries":[]}`))

			if errkind.KindOf(err) != errkind.StorageFailed {
				t.Fatalf("error %v, want StorageFailed", err)
			}
			data, _ := os.ReadFile(filepath.Join(dir, "run.json"))
			entries, _ := os.ReadDir(dir)
			if string(data) != `{"version":1}` || len(entries) != 1 {
				t.Fatalf("record %q, folder %v", data, entries)
			}
		})
	}
}

// NFR9.9: the rename is tried five times, 100 ms apart.
func TestTheRenameIsTriedFiveTimes(t *testing.T) {
	for _, failures := range []int{4, 5} {
		restore(t)
		var waits []time.Duration
		sleep = func(d time.Duration) { waits = append(waits, d) }
		calls := 0
		rename = func(a, b string) error {
			calls++
			if calls <= failures {
				return errors.New("sharing violation")
			}
			return os.Rename(a, b)
		}
		err := writeAtomic(t.TempDir(), "run", []byte("x"))
		if failures == 4 && (err != nil || calls != 5 || len(waits) != 4 || waits[0] != 100*time.Millisecond) {
			t.Errorf("4 failures: err %v, %d calls, waits %v", err, calls, waits)
		}
		if failures == 5 && (errkind.KindOf(err) != errkind.StorageFailed || calls != 5 || len(waits) != 4) {
			t.Errorf("5 failures: err %v, %d calls, waits %v", err, calls, waits)
		}
	}
}

func TestOldTemporaryFilesAreRemoved(t *testing.T) {
	restore(t)
	dir := t.TempDir()
	for _, name := range []string{".a.tmp-1", ".b.tmp-2", "c.json", ".lock"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600)
	}
	old := time.Now().Add(-2 * time.Hour)
	for _, name := range []string{".a.tmp-1", "c.json", ".lock"} {
		_ = os.Chtimes(filepath.Join(dir, name), old, old)
	}
	removeOldTemps(dir)
	var left []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, ",") != ".b.tmp-2,.lock,c.json" {
		t.Fatalf("left %v", left)
	}
}

// --- the record and its periodic save (NFR6.2) ---

// fakeTicker lets a test send the ticks of the periodic save.
func fakeTicker(t *testing.T) chan time.Time {
	t.Helper()
	ticks := make(chan time.Time)
	newTicker = func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} }
	return ticks
}

func beginS3(t *testing.T, j *Journal) *Run {
	t.Helper()
	h, err := j.Lock(context.Background(), acct, s3Loc, PurposeApply, "20261020T090000Z-abcd")
	if err != nil {
		t.Fatal(err)
	}
	r, err := j.Begin(context.Background(), h, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{ARN: "arn:b", Region: "us-east-1", TouchedKeys: []string{"k"}, BeforeTags: map[string]string{"k": "1"}, PlannedAfterTags: map[string]string{"k": "2"}, Status: StatusPending},
		{ARN: "arn:a", Region: "us-east-1", Status: StatusSkipped, Reason: "changedSincePlan"},
	}
	if err := r.RecordBefore(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	return r
}

func savedRecord(t *testing.T, m *memStore, uri string) RunRecord {
	t.Helper()
	data, err := m.Get(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

// waitFor waits until cond holds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func idle(r *Run) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.saving
}

func TestRecordBeforeSavesEveryEntryInARNOrder(t *testing.T) {
	m := newMemStore()
	r := beginS3(t, newJournal(t, m, nil))
	rec := savedRecord(t, m, r.Where())
	if rec.Entries[0].ARN != "arn:a" || rec.Entries[1].Status != StatusPending || rec.Completed || rec.PlanID != "plan-1" ||
		rec.RunID != "20261020T090000Z-abcd" || rec.StartedAt != "2026-10-20T09:00:00Z" || rec.Entries[0].BeforeTags == nil {
		t.Fatalf("record = %+v", rec)
	}
	if !strings.HasSuffix(r.Where(), "runs/"+acct+"/20261020T090000Z-abcd.json") || r.RunID() != "20261020T090000Z-abcd" {
		t.Fatalf("where %s", r.Where())
	}
}

func TestThePeriodicSaveSavesOnlyChanges(t *testing.T) {
	restore(t)
	ticks := fakeTicker(t)
	m := newMemStore()
	r := beginS3(t, newJournal(t, m, nil))
	r.StartAutosave()
	before := m.puts.Load()

	ticks <- time.Now() // nothing changed since RecordBefore
	if err := r.RecordAfter("arn:b", map[string]string{"k": "2"}, StatusApplied, ""); err != nil {
		t.Fatal(err)
	}
	ticks <- time.Now()
	waitFor(t, func() bool { return m.puts.Load() == before+1 && idle(r) })
	if rec := savedRecord(t, m, r.Where()); rec.Entries[1].Status != StatusApplied || rec.Entries[1].AfterTags["k"] != "2" {
		t.Fatalf("saved %+v", rec.Entries)
	}
	if err := r.Complete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := savedRecord(t, m, r.Where()); !rec.Completed {
		t.Fatal("the last save must complete the record")
	}
}

// A tick that finds a save still running is skipped: saves never overlap.
func TestPeriodicSavesNeverOverlap(t *testing.T) {
	restore(t)
	ticks := fakeTicker(t)
	m := newMemStore()
	r := beginS3(t, newJournal(t, m, nil))
	m.gate = make(chan struct{})
	r.StartAutosave()
	start := m.puts.Load()
	_ = r.RecordAfter("arn:b", nil, StatusApplied, "")
	ticks <- time.Now()
	waitFor(t, func() bool { return m.puts.Load() == start+1 })
	_ = r.RecordAfter("arn:b", nil, StatusFailed, "x")
	ticks <- time.Now()
	ticks <- time.Now()
	if n := m.puts.Load(); n != start+1 {
		t.Fatalf("%d saves started while one ran", n-start)
	}
	close(m.gate)
	waitFor(t, func() bool { return idle(r) })
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := savedRecord(t, m, r.Where()); rec.Completed || rec.Entries[1].Status != StatusFailed {
		t.Fatalf("closed record %+v", rec)
	}
}

// Two failed periodic saves then one that works keep the record usable;
// three in a row make RecordAfter fail (NFR6.2).
func TestThreeFailedSavesInARowStopTheRecord(t *testing.T) {
	for _, failures := range []int{2, 3} {
		restore(t)
		ticks := fakeTicker(t)
		m := newMemStore()
		r := beginS3(t, newJournal(t, m, nil))
		var n atomic.Int32
		m.fail = func(string) error {
			if n.Add(1) <= int32(failures) {
				return errors.New("throttled")
			}
			return nil
		}
		r.StartAutosave()
		for i := range failures + 1 {
			_ = r.RecordAfter("arn:b", nil, StatusApplied, string(rune('a'+i)))
			ticks <- time.Now()
			waitFor(t, func() bool { return idle(r) && n.Load() == int32(i+1) })
		}
		err := r.RecordAfter("arn:b", nil, StatusApplied, "")
		if failures == 2 && (err != nil || r.Broken()) {
			t.Errorf("after 2 failures and a save: %v, broken %v", err, r.Broken())
		}
		if failures == 3 && (errkind.KindOf(err) != errkind.StorageFailed || !r.Broken()) {
			t.Errorf("after 3 failures: %v", err)
		}
		_ = r.Close(context.Background())
	}
}

func TestAClosedRecordRefusesChanges(t *testing.T) {
	m := newMemStore()
	r := beginS3(t, newJournal(t, m, nil))
	if err := r.Complete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordAfter("arn:b", nil, StatusApplied, ""); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("after Complete: %v", err)
	}
	if err := r.Close(context.Background()); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("second close: %v", err)
	}
	r2 := beginS3(t, newJournal(t, newMemStore(), nil))
	if err := r2.RecordAfter("arn:none", nil, StatusApplied, ""); errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("unknown resource: %v", err)
	}
}

func TestBeginRefusesARunIDThatHasARecord(t *testing.T) {
	m := newMemStore()
	j := newJournal(t, m, nil)
	r := beginS3(t, j)
	_ = r.Complete(context.Background())
	h := &LockHandle{AccountID: acct, RunID: r.RunID(), Location: s3Loc}
	if _, err := j.Begin(context.Background(), h, "plan-2"); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("S3: %v", err)
	}
	local := newJournal(t, nil, nil)
	lh := &LockHandle{AccountID: acct, RunID: "20261020T090000Z-0001"}
	lr, err := local.Begin(context.Background(), lh, "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := lr.RecordBefore(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := local.Begin(context.Background(), lh, "p"); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("local: %v", err)
	}
	rec, err := local.Load(context.Background(), acct, lh.RunID, Location{Kind: LocationLocal})
	if err != nil || rec.RunID != lh.RunID || rec.Entries == nil {
		t.Fatalf("load: %+v %v", rec, err)
	}
}

// --- the lock (NFR7.1, BR3.1, BR3.2, BR3.8) ---

func TestASecondLockIsAConflictThatShowsTheFirst(t *testing.T) {
	j := newJournal(t, nil, nil)
	if _, err := j.Lock(context.Background(), acct, Location{}, PurposeApply, "run-1"); err != nil {
		t.Fatal(err)
	}
	_, err := j.Lock(context.Background(), acct, Location{}, PurposeRollback, "run-2")
	held, ok := Held(err)
	if errkind.KindOf(err) != errkind.Conflict || !ok || !held.Readable || held.Info.RunID != "run-1" ||
		held.Info.Purpose != PurposeApply || held.Info.PID == 0 || held.Info.ToolVersion != "v-test" {
		t.Fatalf("err %v held %+v", err, held)
	}
	if _, ok := Held(errors.New("other")); ok {
		t.Fatal("Held of another error")
	}
}

// NFR7.1, NFR9.8: of many runs that take the lock at once, one gets it.
func TestOnlyOneOfManyRacingRunsGetsTheLock(t *testing.T) {
	j := newJournal(t, nil, nil)
	if err := j.prepareDir(j.defaultDir(acct)); err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := j.Lock(context.Background(), acct, Location{}, PurposeApply, string(rune('a'+i))); err == nil {
				wins.Add(1)
			} else if errkind.KindOf(err) != errkind.Conflict {
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d runs got the lock", wins.Load())
	}
}

func TestTheS3LockIsTakenAfterTheLocalOneAndUndoneOnFailure(t *testing.T) {
	m := newMemStore()
	j := newJournal(t, m, nil)
	_ = m.Put(context.Background(), s3Loc.s3URI(acct, lockName), []byte(`{"runId":"other","host":"pc2"}`))

	_, err := j.Lock(context.Background(), acct, s3Loc, PurposeApply, "mine")
	held, _ := Held(err)
	if errkind.KindOf(err) != errkind.Conflict || held.Info.RunID != "other" || held.Info.Host != "pc2" {
		t.Fatalf("err %v held %+v", err, held)
	}
	if _, err := os.Stat(filepath.Join(j.defaultDir(acct), lockName)); !os.IsNotExist(err) {
		t.Fatal("the local lock must be removed when the S3 lock is held")
	}
	m2 := newMemStore()
	m2.ifAbsent = &errkind.Error{Kind: errkind.StorageFailed}
	j2 := newJournal(t, m2, nil)
	if _, err := j2.Lock(context.Background(), acct, s3Loc, PurposeApply, "mine"); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("501: %v", err)
	}
	if _, err := os.Stat(filepath.Join(j2.defaultDir(acct), lockName)); !os.IsNotExist(err) {
		t.Fatal("the local lock must be removed when S3 cannot take a lock")
	}
}

// Release removes only locks of its own run: one that --force-unlock gave to
// another run is kept with a warning.
func TestReleaseKeepsALockOfAnotherRun(t *testing.T) {
	m := newMemStore()
	w := &warnings{}
	j := newJournal(t, m, w)
	h, err := j.Lock(context.Background(), acct, s3Loc, PurposeApply, "mine")
	if err != nil {
		t.Fatal(err)
	}
	removed, err := j.ForceUnlock(context.Background(), acct, s3Loc)
	if err != nil || len(removed) != 2 || removed[0].Info.RunID != "mine" {
		t.Fatalf("force unlock: %+v %v", removed, err)
	}
	if _, err := j.Lock(context.Background(), acct, s3Loc, PurposeApply, "theirs"); err != nil {
		t.Fatal(err)
	}

	if err := j.Release(context.Background(), h); err != nil {
		t.Fatal(err)
	}

	if len(w.ids) != 2 || w.ids[0] != WarnLockReplaced {
		t.Fatalf("warnings %v", w.ids)
	}
	if _, err := m.Get(context.Background(), s3Loc.s3URI(acct, lockName)); err != nil {
		t.Fatal("the S3 lock of the other run was removed")
	}
	if _, err := os.Stat(filepath.Join(j.defaultDir(acct), lockName)); err != nil {
		t.Fatal("the local lock of the other run was removed")
	}
}

func TestReleaseReportsALockItCannotRemove(t *testing.T) {
	m := newMemStore()
	j := newJournal(t, m, nil)
	h, err := j.Lock(context.Background(), acct, s3Loc, PurposeApply, "mine")
	if err != nil {
		t.Fatal(err)
	}
	m.deleteErr = &errkind.Error{Kind: errkind.AccessDenied}
	err = j.Release(context.Background(), h)
	if e, _ := errkind.As(err); errkind.KindOf(err) != errkind.StorageFailed || !strings.Contains(e.Subject, "s3://records/runs/") {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(j.defaultDir(acct), lockName)); !os.IsNotExist(err) {
		t.Fatal("the local lock must still be removed")
	}
	if removed, err := j.ForceUnlock(context.Background(), acct, Location{}); err != nil || len(removed) != 0 {
		t.Fatalf("nothing to unlock: %v %v", removed, err)
	}
}

// --- permissions and the log (NFR8.23, NFR8.24) ---

func TestFoldersAreForTheUserOnly(t *testing.T) {
	w := &warnings{}
	j := newJournal(t, nil, w)
	dir := filepath.Join(j.opts.Home, "new")
	if err := j.prepareDir(dir); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return // Windows gives the folder the permissions of its parent.
	}
	if info, _ := os.Stat(dir); info.Mode().Perm() != 0o700 {
		t.Fatalf("permissions %v", info.Mode().Perm())
	}
	open := filepath.Join(j.opts.Home, "open")
	_ = os.Mkdir(open, 0o755)
	_ = os.Chmod(open, 0o755)
	_ = j.prepareDir(open)
	if len(w.ids) != 1 || w.ids[0] != WarnFolderReadable {
		t.Fatalf("warnings %v", w.ids)
	}
	if info, _ := os.Stat(open); info.Mode().Perm() != 0o755 {
		t.Fatal("the permissions of an existing folder must not change")
	}
}

func TestTheLogHasNoTagValuesAndOneLinePerResource(t *testing.T) {
	j := newJournal(t, nil, nil)
	path, err := j.WriteLog(acct, "run", []LogLine{
		{ARN: "arn:a", Region: "us-east-1", Status: "failed", Reason: "writeFailed", ErrorKind: "AccessDenied", Operation: "sqs:TagQueue"},
		{ARN: "arn:b\nfake line", Region: "us-east-1", Status: "skipped", Reason: "changedSincePlan"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := "arn:a\tus-east-1\tfailed\twriteFailed\tAccessDenied\tsqs:TagQueue\narn:b fake line\tus-east-1\tskipped\tchangedSincePlan\t\t\n"
	if string(data) != want {
		t.Fatalf("log %q", data)
	}
	if prepared := j.prepareDir(path); errkind.KindOf(prepared) != errkind.StorageFailed {
		t.Fatal("a file in place of the folder is StorageFailed")
	}
}

func TestLoadAndLocations(t *testing.T) {
	m := newMemStore()
	j := newJournal(t, m, nil)
	if _, err := j.Load(context.Background(), acct, "none", Location{}); errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("missing local record: %v", err)
	}
	if _, err := j.Load(context.Background(), acct, "none", s3Loc); errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("missing S3 record: %v", err)
	}
	_ = m.Put(context.Background(), s3Loc.s3URI(acct, "v2.json"), []byte(`{"version":2,"toolVersion":"v9"}`))
	if _, err := j.Load(context.Background(), acct, "v2", s3Loc); errkind.KindOf(err) != errkind.UnsupportedVersion {
		t.Fatalf("version 2: %v", err)
	}
	_ = m.Put(context.Background(), s3Loc.s3URI(acct, "bad.json"), []byte(`{`))
	if _, err := j.Load(context.Background(), acct, "bad", s3Loc); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("broken JSON: %v", err)
	}
	for value, ok := range map[string]bool{
		"": true, "s3://my-bucket": true, "s3://my-bucket/a/b/": true, "./records": true,
		"s3://": false, "s3://B/x": false, "s3://ab/x": false, "s3://a..b/x": false, "s3://my-bucket/a//b": false,
	} {
		if _, err := ParseLocation(value); (err == nil) != ok {
			t.Errorf("ParseLocation(%q) = %v", value, err)
		}
	}
	l, _ := ParseLocation("s3://my-bucket/a/b/")
	if l.Prefix != "a/b" || l.RecordDir("h", acct) != "s3://my-bucket/a/b/"+acct+"/" {
		t.Fatalf("location %+v %s", l, l.RecordDir("h", acct))
	}
	noPrefix, _ := ParseLocation("s3://my-bucket")
	if noPrefix.s3URI(acct, ".lock") != "s3://my-bucket/"+acct+"/.lock" {
		t.Fatal(noPrefix.s3URI(acct, ".lock"))
	}
	dir, _ := ParseLocation("records")
	if !strings.HasSuffix(dir.RecordDir("h", acct), filepath.Join("records", acct)+string(filepath.Separator)) ||
		!strings.HasSuffix(Location{}.RecordDir("h", acct), filepath.Join("h", ".tag-refactor", "runs", acct)+string(filepath.Separator)) {
		t.Fatal("RecordDir")
	}
	id := NewRunID(time.Date(2026, 10, 19, 21, 10, 3, 0, time.UTC), bytes.NewReader([]byte{0x7f, 0x3a}))
	if id != "20261019T211003Z-7f3a" || !ValidRunID(id) || ValidRunID("x") || !ValidRunID(NewRunID(time.Now(), nil)) {
		t.Fatalf("run ID %q", id)
	}
	if NewRunID(time.Now(), bytes.NewReader(nil)) == "" {
		t.Fatal("a short random source still gives an ID")
	}
}
