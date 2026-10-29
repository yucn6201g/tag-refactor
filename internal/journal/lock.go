package journal

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// lockName is the name of the lock in the default folder and under the S3
// prefix.
const lockName = ".lock"

// LockHandle is a lock that is held.
type LockHandle struct {
	AccountID string
	RunID     string
	Location  Location
	// local is the path of the local lock; s3 the URI of the S3 lock, or
	// empty.
	local, s3 string
}

// HeldLock is a lock found where a run wanted to make its own: where it is
// and what it says. Info is empty when the lock could not be read.
type HeldLock struct {
	Where    string
	Info     LockInfo
	Readable bool
}

// HeldError is the cause of the Conflict of Lock: the lock in the way.
type HeldError struct{ Lock HeldLock }

func (e *HeldError) Error() string { return "the lock is held: " + e.Lock.Where }

// Held returns the lock in the way of a Conflict of Lock.
func Held(err error) (HeldLock, bool) {
	var h *HeldError
	if errors.As(err, &h) {
		return h.Lock, true
	}
	return HeldLock{}, false
}

func conflict(accountID string, held HeldLock) error {
	return &errkind.Error{Kind: errkind.Conflict, Subject: accountID,
		Detail: "another apply or rollback is running for account " + accountID, Cause: &HeldError{Lock: held}}
}

func parseLock(where string, data []byte) HeldLock {
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return HeldLock{Where: where}
	}
	return HeldLock{Where: where, Info: info, Readable: true}
}

// Lock takes the lock of the account for runID (contract C10, BR3.1): the
// local lock in the default folder, created only if it does not exist, and,
// for an S3 location, the S3 lock written only if it does not exist. A lock
// already there is a Conflict whose cause, HeldError, says what it holds.
// When the S3 lock cannot be taken, the local one is removed again.
func (j *Journal) Lock(ctx context.Context, accountID string, loc Location, purpose Purpose, runID string) (*LockHandle, error) {
	dir := j.defaultDir(accountID)
	if err := j.prepareDir(dir); err != nil {
		return nil, err
	}
	info := LockInfo{AccountID: accountID, Purpose: purpose, RunID: runID, CreatedAt: j.now(), Host: j.opts.Host,
		PID: j.opts.PID, ToolVersion: j.opts.ToolVersion}
	data, err := marshal(info)
	if err != nil {
		return nil, errkind.Ensure(err, "write the lock")
	}
	h := &LockHandle{AccountID: accountID, RunID: runID, Location: loc, local: filepath.Join(dir, lockName)}
	if err := createLocalLock(h.local, data); err != nil {
		return nil, err
	}
	if loc.Kind != LocationS3 {
		return h, nil
	}
	uri := loc.s3URI(accountID, lockName)
	if err := j.opts.Store.PutIfAbsent(ctx, uri, data); err != nil {
		_ = os.Remove(h.local)
		if errkind.KindOf(err) == errkind.Conflict {
			held := HeldLock{Where: uri}
			if existing, gerr := j.opts.Store.Get(ctx, uri); gerr == nil {
				held = parseLock(uri, existing)
			}
			return nil, conflict(accountID, held)
		}
		return nil, err
	}
	h.s3 = uri
	return h, nil
}

// createLocalLock creates the lock file with O_EXCL: of several processes,
// only one can create it (NFR7.1).
func createLocalLock(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		held := HeldLock{Where: path}
		if existing, rerr := os.ReadFile(path); rerr == nil {
			held = parseLock(path, existing)
		}
		var info LockInfo
		_ = json.Unmarshal(data, &info)
		return conflict(info.AccountID, held)
	}
	if err != nil {
		return storageError(path, "journal.lockFailed", "the lock could not be written", err)
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(path)
		return storageError(path, "journal.lockFailed", "the lock could not be written", errors.Join(werr, cerr))
	}
	return nil
}

// Release removes the locks of h, the S3 one first (contract C10, BR3.8).
// Before removing a lock it checks that the lock still holds h's run: a lock
// that --force-unlock gave to another run is kept, with a warning. A lock
// that cannot be removed is a StorageFailed naming where it is.
func (j *Journal) Release(ctx context.Context, h *LockHandle) error {
	var failed []string
	var causes []error
	if h.s3 != "" {
		if err := j.releaseS3(ctx, h); err != nil {
			failed, causes = append(failed, h.s3), append(causes, err)
		}
	}
	if err := j.releaseLocal(h); err != nil {
		failed, causes = append(failed, h.local), append(causes, err)
	}
	if len(failed) > 0 {
		e := storageError(strings.Join(failed, ", "), "journal.lockNotRemoved", "the lock could not be removed", errors.Join(causes...))
		e.Hint, e.HintID = "make sure no apply or rollback is running, then remove it with --force-unlock", "journal.lockNotRemovedFix"
		return e
	}
	return nil
}

func (j *Journal) releaseS3(ctx context.Context, h *LockHandle) error {
	data, err := j.opts.Store.Get(ctx, h.s3)
	if errkind.KindOf(err) == errkind.NotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if held := parseLock(h.s3, data); held.Info.RunID != h.RunID {
		j.opts.Warn(WarnLockReplaced, h.s3, held.Info.RunID)
		return nil
	}
	return j.opts.Store.Delete(ctx, h.s3)
}

func (j *Journal) releaseLocal(h *LockHandle) error {
	data, err := os.ReadFile(h.local)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if held := parseLock(h.local, data); held.Info.RunID != h.RunID {
		j.opts.Warn(WarnLockReplaced, h.local, held.Info.RunID)
		return nil
	}
	return os.Remove(h.local)
}

// ForceUnlock removes the locks of the account, local and, for an S3
// location, in S3, whatever run they hold, and returns what they held
// (BR3.2: only when a person asks with --force-unlock). Missing locks are
// not an error.
func (j *Journal) ForceUnlock(ctx context.Context, accountID string, loc Location) ([]HeldLock, error) {
	var removed []HeldLock
	local := filepath.Join(j.defaultDir(accountID), lockName)
	data, err := os.ReadFile(local)
	switch {
	case err == nil:
		if rerr := os.Remove(local); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			return removed, storageError(local, "journal.lockNotRemoved", "the lock could not be removed", rerr)
		}
		removed = append(removed, parseLock(local, data))
	case !errors.Is(err, fs.ErrNotExist):
		return removed, storageError(local, "journal.lockNotRemoved", "the lock could not be removed", err)
	}
	if loc.Kind != LocationS3 {
		return removed, nil
	}
	uri := loc.s3URI(accountID, lockName)
	data, err = j.opts.Store.Get(ctx, uri)
	switch {
	case errkind.KindOf(err) == errkind.NotFound:
		return removed, nil
	case err != nil:
		return removed, err
	}
	if err := j.opts.Store.Delete(ctx, uri); err != nil {
		return removed, err
	}
	return append(removed, parseLock(uri, data)), nil
}
