// Package fileout writes the files tag-refactor saves for the user: the plan
// of plan --out and the draft of rules suggest --output (U4 security design
// §3). A file is first written to a temporary file in the same folder and
// only then given its name, so the name shows either the whole file or
// nothing; an existing file is never overwritten, and the temporary file is
// removed however the command ends.
//
// The package depends on errkind only. Its errors carry the path as Subject
// and a reason the command line turns into its two-line message.
package fileout

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The functions that give the temporary file its name. They are variables so
// that the tests can stand in for a file system without hard links and make
// the rename fail.
var (
	link   = os.Link
	rename = os.Rename
)

// bufferSize is the size of the write buffer in front of the temporary file.
const bufferSize = 64 << 10

// Prepare checks, before the command does anything else, that path does not
// exist and that a temporary file can be created in its folder: the
// temporary file is created and removed at once (security design §1, §3), so
// a wrong path does not cost a collection of several minutes. prefix starts
// the temporary file name, for example ".tag-refactor-plan-".
func Prepare(path, prefix string) error {
	if _, err := os.Lstat(path); err == nil {
		return Exists(path)
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputFolderMissing, Subject: path, Cause: err}
	}
	tmp, err := os.CreateTemp(dir, prefix+"*.tmp")
	if err != nil {
		return failed(path, err)
	}
	name := tmp.Name()
	closeErr := tmp.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return failed(path, closeErr)
	}
	if removeErr != nil {
		return failed(path, removeErr)
	}
	return nil
}

// Exists is the error of a path that already exists: tag-refactor never
// overwrites a file.
func Exists(path string) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputExists, Subject: path}
}

func failed(path string, err error) error {
	return &errkind.Error{Kind: errkind.StorageFailed, Reason: errkind.ReasonOutputFailed, Subject: path, Cause: err}
}

// TooLarge is the error of a file that would grow past its limit. Nothing is
// saved; the command line says how to make the file smaller.
func TooLarge(path string) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputTooLarge, Subject: path}
}

// Pending is a file being written: a temporary file in the folder of its
// path. Commit gives it the name; Discard, which every caller defers, closes
// and removes what is left.
type Pending struct {
	path  string
	limit int64
	file  *os.File
	// tmpName is the name of the temporary file.
	tmpName string
	buf     *bufio.Writer
	// written counts the bytes accepted so far.
	written int64
	// err is the first error met; once set, every write fails with it.
	err error
	// done is true once Commit gave the file its name.
	done bool
}

// Create creates the temporary file just before the file is written (the
// collection does not hold it). It refuses to start when path appeared in
// the meantime. limit is the largest size the file may have, in bytes.
// On Unix the temporary file is readable and writable by its owner only
// (os.CreateTemp creates it with 0600); on Windows it gets the folder's
// permissions (NFR8.17).
func Create(path, prefix string, limit int64) (*Pending, error) {
	if _, err := os.Lstat(path); err == nil {
		return nil, Exists(path)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), prefix+"*.tmp")
	if err != nil {
		return nil, failed(path, err)
	}
	return &Pending{path: path, limit: limit, file: tmp, tmpName: tmp.Name(), buf: bufio.NewWriterSize(tmp, bufferSize)}, nil
}

// Writer returns the writer of the file. It counts the bytes: a write that
// would take the file past its limit writes nothing and fails with the
// InvalidInput error of TooLarge, and so does every write after it.
func (p *Pending) Writer() io.Writer { return pendingWriter{p} }

type pendingWriter struct{ p *Pending }

func (w pendingWriter) Write(b []byte) (int, error) {
	p := w.p
	if p.err != nil {
		return 0, p.err
	}
	if p.done || p.file == nil {
		p.err = failed(p.path, os.ErrClosed)
		return 0, p.err
	}
	if p.written+int64(len(b)) > p.limit {
		p.err = TooLarge(p.path)
		return 0, p.err
	}
	n, err := p.buf.Write(b)
	p.written += int64(n)
	if err != nil {
		p.err = failed(p.path, err)
		return n, p.err
	}
	return n, nil
}

// Commit writes what is buffered, syncs and closes the temporary file, and
// gives it its name with a hard link, which fails when the name exists, so a
// file created meanwhile is not overwritten either. A file system without
// hard links gets a rename after one more check; the moment between the
// check and the rename is the limit NFR8.17 accepts (as U3 did).
func (p *Pending) Commit() error {
	if p.err != nil {
		return p.err
	}
	if p.done || p.file == nil {
		return failed(p.path, os.ErrClosed)
	}
	if err := p.buf.Flush(); err != nil {
		return p.fail(err)
	}
	if err := p.file.Sync(); err != nil {
		return p.fail(err)
	}
	f := p.file
	p.file = nil
	if err := f.Close(); err != nil {
		return p.fail(err)
	}
	tmpName := p.tmpName
	err := link(tmpName, p.path)
	switch {
	case err == nil:
		p.done = true
		_ = os.Remove(tmpName)
		return nil
	case errors.Is(err, fs.ErrExist):
		return p.keep(Exists(p.path))
	}
	if _, statErr := os.Lstat(p.path); statErr == nil {
		return p.keep(Exists(p.path))
	}
	if err := rename(tmpName, p.path); err != nil {
		return p.fail(err)
	}
	p.done = true
	return nil
}

func (p *Pending) fail(err error) error {
	return p.keep(failed(p.path, err))
}

func (p *Pending) keep(err error) error {
	if p.err == nil {
		p.err = err
	}
	return p.err
}

// Discard closes the temporary file when it is still open and removes it.
// On Windows an open file cannot be removed, so it is closed first. It does
// nothing to the file Commit named, and may be called more than once.
func (p *Pending) Discard() {
	if p == nil {
		return
	}
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
	if !p.done && p.tmpName != "" {
		// After a hard link the temporary name is already gone, and after a
		// rename it is the named file: only an unnamed one is removed.
		_ = os.Remove(p.tmpName)
	}
	if p.err == nil && !p.done {
		p.err = failed(p.path, os.ErrClosed)
	}
}
