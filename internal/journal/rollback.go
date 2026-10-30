package journal

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The record of a rollback (U7 BR6.1 to BR6.4, contract C10, security design
// §4, §5). rollback reads the record of the apply once, after it took its
// lock, and writes its result into the rollbacks of that same record: the
// entries, completed, the other rollbacks and every field this tool does not
// know stay as they were read, as raw JSON.

// rollbacksKey is the key of the rollbacks in the record.
const rollbacksKey = "rollbacks"

// RollbackRun is the record of the apply a rollback undoes, while the
// rollback runs. Its methods may be called from several goroutines.
type RollbackRun struct {
	j     *Journal
	h     *LockHandle
	where string
	// runID is the run ID of the record's name, <runID>.json.
	runID string
	// rec is the record as this tool reads it, for the checks and the
	// judgement of rollback.
	rec RunRecord

	mu sync.Mutex
	// top is every field of the record as read; others the rollbacks it
	// already had. Only the rollbacks are written again, with own last.
	top    map[string]json.RawMessage
	others []json.RawMessage
	own    *RollbackRecord
	index  map[string]int
	sv     saver
}

// OpenForRollback reads the record of runID at the location of h, once (U7
// BR1.2, contract C10). h must be a lock of rollback, which keeps apply and
// other rollbacks of the account from changing the record from then on; any
// other lock is Internal. A missing record is NotFound, a record of another
// version UnsupportedVersion, and one that is not a run record InvalidInput.
func (j *Journal) OpenForRollback(ctx context.Context, h *LockHandle, runID string) (*RollbackRun, error) {
	if h == nil || h.Purpose != PurposeRollback {
		return nil, &errkind.Error{Kind: errkind.Internal, Op: "read the run record",
			Detail: "the record of a rollback is read only under the lock of a rollback"}
	}
	data, where, err := j.readRecord(ctx, h.AccountID, runID, h.Location)
	if err != nil {
		return nil, err
	}
	r := &RollbackRun{j: j, h: h, where: where, runID: runID, index: map[string]int{}}
	if err := r.parse(data); err != nil {
		return nil, err
	}
	r.sv = saver{mu: &r.mu, snapshot: r.snapshotLocked, save: r.store}
	return r, nil
}

// parse reads data as a run record twice over: typed, and as raw JSON whose
// fields are written back as they are.
func (r *RollbackRun) parse(data []byte) error {
	var head struct {
		Version     int    `json:"version"`
		ToolVersion string `json:"toolVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return r.notARecord(err)
	}
	if head.Version != Version {
		return unsupportedVersion(r.where, head.Version, head.ToolVersion)
	}
	if err := json.Unmarshal(data, &r.rec); err != nil {
		return r.notARecord(err)
	}
	if err := json.Unmarshal(data, &r.top); err != nil {
		return r.notARecord(err)
	}
	if raw, ok := r.top[rollbacksKey]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &r.others); err != nil {
			return r.notARecord(err)
		}
	}
	return nil
}

func (r *RollbackRun) notARecord(cause error) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Subject: r.where, Detail: "the file is not a run record of tag-refactor",
		DetailID: "journal.notARecord", Hint: "pass the run ID that apply showed, and the same --record-from as its --record-to",
		HintID: "journal.notARecordFix", Cause: cause}
}

// Record is the record as read (contract C4), with the rollbacks it had
// before this one. The caller must not change it.
func (r *RollbackRun) Record() *RunRecord { return &r.rec }

// Where is the path or the S3 URI of the record.
func (r *RollbackRun) Where() string { return r.where }

// BeginRollback adds this rollback, not completed and without results, and
// saves the record at once; once that worked, the record is saved every
// five seconds (U7 BR5.4, contract C10). A failure is StorageFailed: the
// caller then writes nothing.
func (r *RollbackRun) BeginRollback(ctx context.Context, rollbackID string, startedAt time.Time) error {
	r.mu.Lock()
	if r.own != nil || r.sv.closed {
		r.mu.Unlock()
		return &errkind.Error{Kind: errkind.Internal, Op: "record the rollback", Detail: "the rollback was already begun"}
	}
	r.own = &RollbackRecord{RollbackID: rollbackID, StartedAt: startedAt.UTC().Format(timeFormat), Results: []RollbackResult{}}
	r.sv.changed()
	data, v, err := r.snapshotLocked()
	r.mu.Unlock()
	if err != nil {
		return err
	}
	if err := r.store(ctx, data, v); err != nil {
		return err
	}
	r.sv.start()
	return nil
}

// RecordRollbackResults adds the results of resources, or replaces the
// earlier result of the same ARN (contract C10). It only changes the record
// in memory: the periodic save and the last save write it. After the record
// was closed (the second Ctrl+C abandons a run) it does nothing; once three
// periodic saves in a row have failed it is StorageFailed.
func (r *RollbackRun) RecordRollbackResults(results []RollbackResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.sv.closed:
		return nil
	case r.own == nil:
		return &errkind.Error{Kind: errkind.Internal, Op: "record the rollback", Detail: "the rollback was not begun"}
	case r.sv.broken:
		return storageError(r.where, "journal.saveFailed", "the run record could not be saved", nil)
	}
	for _, res := range results {
		if i, ok := r.index[res.ARN]; ok {
			r.own.Results[i] = res
			continue
		}
		r.index[res.ARN] = len(r.own.Results)
		r.own.Results = append(r.own.Results, res)
	}
	r.sv.changed()
	return nil
}

// Broken reports whether the periodic saves failed failLimit times in a row.
func (r *RollbackRun) Broken() bool { return r.sv.isBroken() }

// CompleteRollback marks this rollback completed, saves the record a last
// time and closes it (U7 BR6.2).
func (r *RollbackRun) CompleteRollback(ctx context.Context) error { return r.finish(ctx, true) }

// Close saves the record a last time with this rollback not completed, and
// closes it: the rollback was stopped (U7 BR5.5).
func (r *RollbackRun) Close(ctx context.Context) error { return r.finish(ctx, false) }

func (r *RollbackRun) finish(ctx context.Context, completed bool) error {
	r.sv.halt()
	r.mu.Lock()
	switch {
	case r.sv.closed:
		r.mu.Unlock()
		return storageError(r.where, "journal.closed", "the run record is closed", nil)
	case r.own == nil:
		r.mu.Unlock()
		return &errkind.Error{Kind: errkind.Internal, Op: "record the rollback", Detail: "the rollback was not begun"}
	}
	r.own.Completed = completed
	r.sv.changed()
	data, v, err := r.snapshotLocked()
	r.sv.closed = true
	r.mu.Unlock()
	if err != nil {
		return err
	}
	return r.store(ctx, data, v)
}

// snapshotLocked encodes the record: every field as read, with the
// rollbacks as read and this one after them. The caller holds the lock.
func (r *RollbackRun) snapshotLocked() ([]byte, uint64, error) {
	own := *r.own
	own.Results = slices.Clone(r.own.Results)
	ownData, err := json.Marshal(own)
	if err != nil {
		return nil, 0, errkind.Ensure(err, "encode the run record")
	}
	rollbacks, err := json.Marshal(append(slices.Clone(r.others), ownData))
	if err != nil {
		return nil, 0, errkind.Ensure(err, "encode the run record")
	}
	top := make(map[string]json.RawMessage, len(r.top)+1)
	for k, v := range r.top {
		top[k] = v
	}
	top[rollbacksKey] = rollbacks
	data, err := marshal(top)
	if err != nil {
		return nil, 0, errkind.Ensure(err, "encode the run record")
	}
	return data, r.sv.version, nil
}

// store saves data, the encoding of version v, in place of the record:
// locally through a temporary file, in S3 by one PutObject. No conditional
// write: while rollback holds its lock nobody else writes the record (U7
// security design §4).
func (r *RollbackRun) store(ctx context.Context, data []byte, v uint64) error {
	var err error
	if r.h.Location.Kind == LocationS3 {
		err = r.j.opts.Store.Put(ctx, r.where, data)
	} else {
		err = writeAtomic(filepath.Dir(r.where), r.runID, data)
	}
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.sv.stored(v)
	r.mu.Unlock()
	return nil
}
