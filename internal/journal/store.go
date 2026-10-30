package journal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The saving of local files (NFR6.1, NFR9.9, security design §3). The file
// operations, the waits and the clock of the files are variables so that the
// tests can make a save fail half way.

// tempFile is the temporary file a save writes.
type tempFile interface {
	Write(b []byte) (int, error)
	Sync() error
	Close() error
	Name() string
}

var (
	// createTemp creates the temporary file in the folder of the record.
	createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }
	// rename puts the temporary file in place of the record.
	rename = os.Rename
	// renameAttempts is the number of tries of the rename: on Windows it
	// fails while another process has the record open (NFR9.9).
	renameAttempts = 5
	// renameWait is the wait between two tries.
	renameWait = 100 * time.Millisecond
	// sleep waits.
	sleep = time.Sleep
	// syncFolder writes the rename of a folder to disk.
	syncFolder = syncDir
	// fileNow is the clock of the files on disk, to find old temporary
	// files.
	fileNow = time.Now
)

// tempMarker is in the name of every temporary file: .<runId>.tmp-<random>,
// which never ends with .json, so readers of records never see one.
const tempMarker = ".tmp-"

// tempMaxAge is the age after which a temporary file left by a stopped
// process is removed.
const tempMaxAge = time.Hour

// writeAtomic saves data as <dir>/<name>.json: written to a temporary file
// of the folder, synced to disk, closed and renamed over the record, up to
// renameAttempts times. A failure removes the temporary file and leaves the
// previous record as it was. On Linux and macOS the folder is synced too.
func writeAtomic(dir, name string, data []byte) (err error) {
	target := filepath.Join(dir, name+".json")
	f, err := createTemp(dir, "."+name+tempMarker+"*")
	if err != nil {
		return storageError(target, "journal.saveFailed", "the run record could not be saved", err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return storageError(target, "journal.saveFailed", "the run record could not be saved", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return storageError(target, "journal.saveFailed", "the run record could not be saved", err)
	}
	if err := f.Close(); err != nil {
		return storageError(target, "journal.saveFailed", "the run record could not be saved", err)
	}
	var renameErr error
	for attempt := 1; attempt <= renameAttempts; attempt++ {
		if renameErr = rename(tmp, target); renameErr == nil {
			break
		}
		if attempt < renameAttempts {
			sleep(renameWait)
		}
	}
	if renameErr != nil {
		return storageError(target, "journal.saveFailed", "the run record could not be saved", renameErr)
	}
	// The record is in place; a folder that cannot be synced only risks the
	// rename after a power cut, which leaves the previous version.
	_ = syncFolder(dir)
	return nil
}

// syncDir writes the entries of a folder to disk. Windows cannot open a
// folder for that, so there it does nothing (NFR9.9).
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

// prepareDir creates the folder for the user only (0700), or warns when an
// existing one can be read or written by other users; its permissions are
// not changed (NFR8.24). Windows gives the folder the permissions of its
// parent, and has no such bits to check.
func (j *Journal) prepareDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return storageError(dir, "journal.folderFailed", "the folder of the run records could not be created", nil)
	case err == nil:
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			j.opts.Warn(WarnFolderReadable, dir)
		}
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return storageError(dir, "journal.folderFailed", "the folder of the run records could not be created", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return storageError(dir, "journal.folderFailed", "the folder of the run records could not be created", err)
	}
	return nil
}

// removeOldTemps removes the temporary files older than tempMaxAge that a
// stopped process left in the folder. Failures are ignored: such files are
// never read.
func removeOldTemps(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	limit := fileNow().Add(-tempMaxAge)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, ".") || !strings.Contains(name, tempMarker) || e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(limit) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
