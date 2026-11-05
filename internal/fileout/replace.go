package fileout

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RetryRename gives old the name new: it calls rename and, each time rename
// fails and retryable says so, waits the next of waits and calls it again, so
// there are at most len(waits)+1 calls. An error retryable refuses comes back
// at once, and so does the last error (U8 security design §7). The record of
// a run retries every error; a file of the user only the moments when another
// program holds it on Windows (TransientRename).
func RetryRename(old, new string, waits []time.Duration, retryable func(error) bool,
	rename func(string, string) error, sleep func(time.Duration)) error {
	err := rename(old, new)
	for _, wait := range waits {
		if err == nil || !retryable(err) {
			return err
		}
		sleep(wait)
		err = rename(old, new)
	}
	return err
}

// UserFileWaits are the waits of a rename over a file of the user, 4.7 seconds
// in all: an editor or a virus scanner can have the file open on Windows for a
// moment (U8 NFR6.5). A record of a run keeps its own, shorter waits.
var UserFileWaits = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond,
	800 * time.Millisecond, 1600 * time.Millisecond, 1600 * time.Millisecond}

// ReplaceOptions holds what Replace takes from outside. The zero value is the
// production setting.
type ReplaceOptions struct {
	// Waits are the waits of the rename; nil means UserFileWaits.
	Waits []time.Duration
	// Opened, when set, is called for each file Replace opens: the temporary
	// file when it is created and when it is read back (U8 NFR5.26).
	Opened func()
}

// The functions of Replace that the tests stand in for.
var (
	replaceRename = os.Rename
	replaceSleep  = time.Sleep
	replaceRead   = os.ReadFile
	randomHex     = func() (string, error) {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return hex.EncodeToString(b), nil
	}
)

// Replace puts content in place of the existing file at path so that the name
// shows either the old file or the whole new one (U8 NFR6.5, security design
// §7). The temporary file .<name>.tag-refactor-<random>.tmp, in the same
// folder and readable by its owner only, is written, synced and read back; it
// gets the permissions of the old file and only then the name. A failure
// removes the temporary file and leaves the old file as it was. The errors are
// those of Create, with the path as Subject and never the file's content.
func Replace(path string, content []byte, opts ReplaceOptions) (err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return failed(path, err)
	}
	suffix, err := randomHex()
	if err != nil {
		return failed(path, err)
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tag-refactor-"+suffix+".tmp")
	if err := writeTemp(tmp, content, opts.Opened); err != nil {
		return failed(path, err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if opts.Opened != nil {
		opts.Opened()
	}
	written, err := replaceRead(tmp)
	if err != nil {
		return failed(path, err)
	}
	if !bytes.Equal(written, content) {
		return failed(path, io.ErrShortWrite)
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		return failed(path, err)
	}
	waits := opts.Waits
	if waits == nil {
		waits = UserFileWaits
	}
	if err := RetryRename(tmp, path, waits, TransientRename, replaceRename, replaceSleep); err != nil {
		return failed(path, err)
	}
	return nil
}

// writeTemp creates the temporary file, which must not exist yet, and writes
// content to disk. A failure removes it.
func writeTemp(tmp string, content []byte, opened func()) (err error) {
	if opened != nil {
		opened()
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(content); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
