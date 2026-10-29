package journal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The saving while apply writes (NFR6.2, security design §3). The interval,
// the ticker and the limit are variables for the tests.
var (
	autosaveInterval = 5 * time.Second
	// newTicker starts the clock of the periodic save; stop ends it.
	newTicker = func(d time.Duration) (ticks <-chan time.Time, stop func()) {
		t := time.NewTicker(d)
		return t.C, t.Stop
	}
	// failLimit is the number of periodic saves that may fail in a row
	// before the record counts as unsavable.
	failLimit = 3
	// saveTimeout bounds one save to S3 of the periodic saver.
	saveTimeout = 30 * time.Second
)

// Run is the record of one apply while it runs (contract C10 Begin to
// Complete). Its methods may be called from several goroutines.
type Run struct {
	j     *Journal
	h     *LockHandle
	where string

	mu    sync.Mutex
	rec   RunRecord
	index map[string]int
	// version counts the changes of the record; saved is the version of the
	// last save that worked.
	version, saved uint64
	// first is true until the record has been saved once: an S3 record is
	// then written only if it does not exist.
	first  bool
	saving bool
	fails  int
	broken bool
	closed bool

	saves    sync.WaitGroup
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

// Begin starts the record of the run of h for planID (contract C10, BR3.7,
// BR5.4): it prepares the folder, removes the temporary files a stopped run
// left there more than an hour ago, and refuses a run ID that already has a
// record. Nothing is saved before RecordBefore.
func (j *Journal) Begin(ctx context.Context, h *LockHandle, planID string) (*Run, error) {
	r := &Run{j: j, h: h, first: true, index: map[string]int{},
		rec: RunRecord{Version: Version, ToolVersion: j.opts.ToolVersion, RunID: h.RunID, AccountID: h.AccountID,
			PlanID: planID, StartedAt: j.now(), Entries: []Entry{}}}
	if h.Location.Kind == LocationS3 {
		// The first save writes the record only if it does not exist, which
		// refuses a run ID that has one. Reading it here would need the 404
		// of a missing key, which S3 answers only with s3:ListBucket.
		r.where = h.Location.s3URI(h.AccountID, h.RunID+".json")
		return r, nil
	}
	dir := h.Location.localDir(j.opts.Home, h.AccountID)
	if err := j.prepareDir(dir); err != nil {
		return nil, err
	}
	removeOldTemps(dir)
	r.where = filepath.Join(dir, h.RunID+".json")
	if _, err := os.Lstat(r.where); !errors.Is(err, fs.ErrNotExist) {
		return nil, storageError(r.where, "journal.runExists", "a run record with this run ID already exists", err)
	}
	return r, nil
}

// Where is the path or the S3 URI of the record.
func (r *Run) Where() string { return r.where }

// RunID is the run ID of the record.
func (r *Run) RunID() string { return r.h.RunID }

// RecordBefore stores every entry of the run, in ARN order, and saves the
// record before anything is written (contract C10, BR3.7, NFR4.1). A failure
// is StorageFailed: the caller then writes nothing.
func (r *Run) RecordBefore(ctx context.Context, entries []Entry) error {
	r.mu.Lock()
	r.rec.Entries = make([]Entry, len(entries))
	for i, e := range entries {
		e.TouchedKeys = append([]string{}, e.TouchedKeys...)
		e.BeforeTags = cloneTags(e.BeforeTags)
		e.PlannedAfterTags = cloneTags(e.PlannedAfterTags)
		if e.AfterTags != nil {
			e.AfterTags = maps.Clone(e.AfterTags)
		}
		r.rec.Entries[i] = e
	}
	sort.Slice(r.rec.Entries, func(a, b int) bool { return r.rec.Entries[a].ARN < r.rec.Entries[b].ARN })
	r.index = make(map[string]int, len(entries))
	for i, e := range r.rec.Entries {
		r.index[e.ARN] = i
	}
	r.version++
	data, v, err := r.snapshotLocked()
	r.mu.Unlock()
	if err != nil {
		return err
	}
	return r.store(ctx, data, v)
}

func cloneTags(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return maps.Clone(m)
}

// RecordAfter sets the result of one resource (contract C10, BR4.4). It only
// changes the record in memory; the periodic save and the last save write
// it. It is StorageFailed once three periodic saves in a row have failed,
// and after the last save.
func (r *Run) RecordAfter(arn string, afterTags map[string]string, status Status, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.closed:
		return storageError(r.where, "journal.closed", "the run record is closed", nil)
	case r.broken:
		return storageError(r.where, "journal.saveFailed", "the run record could not be saved", nil)
	}
	i, ok := r.index[arn]
	if !ok {
		return &errkind.Error{Kind: errkind.Internal, Op: "record the result", Detail: "the resource is not in the run record"}
	}
	e := r.rec.Entries[i]
	if afterTags != nil {
		e.AfterTags = maps.Clone(afterTags)
	}
	e.Status, e.Reason = status, reason
	r.rec.Entries[i] = e
	r.version++
	return nil
}

// snapshotLocked encodes the record; the caller holds the lock.
func (r *Run) snapshotLocked() ([]byte, uint64, error) {
	rec := r.rec
	rec.Entries = slices.Clone(r.rec.Entries)
	data, err := marshal(rec)
	if err != nil {
		return nil, 0, errkind.Ensure(err, "encode the run record")
	}
	return data, r.version, nil
}

// store saves data, the encoding of version v, and notes the version saved.
func (r *Run) store(ctx context.Context, data []byte, v uint64) error {
	r.mu.Lock()
	first := r.first
	r.mu.Unlock()
	var err error
	switch {
	case r.h.Location.Kind != LocationS3:
		err = writeAtomic(filepath.Dir(r.where), r.h.RunID, data)
	case first:
		err = r.j.opts.Store.PutIfAbsent(ctx, r.where, data)
		if errkind.KindOf(err) == errkind.Conflict {
			// A retry of a write that reached S3 ends 412 on our own record.
			if held, gerr := r.j.opts.Store.Get(ctx, r.where); gerr == nil && bytes.Equal(held, data) {
				err = nil
			} else {
				err = storageError(r.where, "journal.runExists", "a run record with this run ID already exists", err)
			}
		}
	default:
		err = r.j.opts.Store.Put(ctx, r.where, data)
	}
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.first = false
	r.saved = max(r.saved, v)
	r.mu.Unlock()
	return nil
}

// StartAutosave starts saving the record every five seconds while apply
// writes (NFR6.2). A save runs beside the writes; a tick that finds a save
// still running, or nothing changed, is skipped. After failLimit failures in
// a row the record counts as unsavable and RecordAfter fails.
func (r *Run) StartAutosave() {
	r.stop, r.stopped = make(chan struct{}), make(chan struct{})
	ticks, stopTicker := newTicker(autosaveInterval)
	go func() {
		defer close(r.stopped)
		defer stopTicker()
		for {
			select {
			case <-r.stop:
				return
			case <-ticks:
				r.autosave()
			}
		}
	}()
}

// autosave starts one periodic save when one is due.
func (r *Run) autosave() {
	r.mu.Lock()
	if r.closed || r.saving || r.version == r.saved {
		r.mu.Unlock()
		return
	}
	data, v, err := r.snapshotLocked()
	if err != nil {
		r.noteFailureLocked()
		r.mu.Unlock()
		return
	}
	r.saving = true
	r.saves.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.saves.Done()
		ok := false
		defer func() {
			// A panic of the save counts as a failed save (security
			// design §3: no goroutine may take the process down).
			_ = recover()
			r.mu.Lock()
			r.saving = false
			if ok {
				r.fails = 0
			} else {
				r.noteFailureLocked()
			}
			r.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
		defer cancel()
		ok = r.store(ctx, data, v) == nil
	}()
}

func (r *Run) noteFailureLocked() {
	r.fails++
	if r.fails >= failLimit {
		r.broken = true
	}
}

// Broken reports whether the periodic saves failed failLimit times in a row.
func (r *Run) Broken() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.broken
}

// stopAutosave ends the periodic saves and waits for one that runs.
func (r *Run) stopAutosave() {
	if r.stop != nil {
		r.stopOnce.Do(func() { close(r.stop) })
		<-r.stopped
	}
	r.saves.Wait()
}

// Complete marks the record completed, saves it a last time and closes it
// (contract C10, BR4.6).
func (r *Run) Complete(ctx context.Context) error { return r.finish(ctx, true) }

// Close saves the record a last time without completing it and closes it:
// the run was stopped (BR4.5).
func (r *Run) Close(ctx context.Context) error { return r.finish(ctx, false) }

// finish stops the periodic saves, makes the last save and closes the
// record, so a goroutine that did not wait cannot change it afterwards
// (security design §3).
func (r *Run) finish(ctx context.Context, completed bool) error {
	r.stopAutosave()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return storageError(r.where, "journal.closed", "the run record is closed", nil)
	}
	r.rec.Completed = completed
	r.version++
	data, v, err := r.snapshotLocked()
	r.closed = true
	r.mu.Unlock()
	if err != nil {
		return err
	}
	return r.store(ctx, data, v)
}

// LogLine is one line of the log file of a run (BR5.5): never a tag value,
// an AWS error text or a credential (NFR8.23).
type LogLine struct {
	ARN, Region, Status, Reason, ErrorKind, Operation string
}

// WriteLog writes the log file <default folder>/<runId>.log, one
// tab-separated line per resource, and returns its path.
func (j *Journal) WriteLog(accountID, runID string, lines []LogLine) (string, error) {
	dir := j.defaultDir(accountID)
	if err := j.prepareDir(dir); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, l := range lines {
		fields := []string{l.ARN, l.Region, l.Status, l.Reason, l.ErrorKind, l.Operation}
		for i, f := range fields {
			fields[i] = cleanField(f)
		}
		b.WriteString(strings.Join(fields, "\t"))
		b.WriteByte('\n')
	}
	path := filepath.Join(dir, runID+".log")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", storageError(path, "journal.logFailed", "the log file could not be written", err)
	}
	return path, nil
}

// Load reads the record of runID at the location (contract C10). A missing
// record is NotFound and a record of another version UnsupportedVersion.
func (j *Journal) Load(ctx context.Context, accountID, runID string, loc Location) (*RunRecord, error) {
	var data []byte
	var where string
	var err error
	if loc.Kind == LocationS3 {
		where = loc.s3URI(accountID, runID+".json")
		data, err = j.opts.Store.Get(ctx, where)
	} else {
		where = filepath.Join(loc.localDir(j.opts.Home, accountID), runID+".json")
		data, err = os.ReadFile(where)
		if errors.Is(err, fs.ErrNotExist) {
			err = &errkind.Error{Kind: errkind.NotFound, Subject: where, Detail: "the run record does not exist", Cause: err}
		}
	}
	if err != nil {
		if errkind.KindOf(err) == errkind.Internal {
			err = storageError(where, "journal.readFailed", "the run record could not be read", err)
		}
		return nil, err
	}
	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, storageError(where, "journal.readFailed", "the run record could not be read", err)
	}
	if rec.Version != Version {
		return nil, &errkind.Error{Kind: errkind.UnsupportedVersion, Subject: where,
			Detail: fmt.Sprintf("run record version %d (written by tag-refactor %s) is not supported", rec.Version, rec.ToolVersion)}
	}
	return &rec, nil
}
