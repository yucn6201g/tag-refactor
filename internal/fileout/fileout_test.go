package fileout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of internal/fileout (NFR8.17, security design §3), written
// after the implementation. The draft tests of U3's writeDraftFileWith moved
// here with the function.

const prefix = ".tag-refactor-test-"

func requireReason(t *testing.T, err error, kind errkind.Kind, reason errkind.Reason) {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok || e.Kind != kind || e.Reason != reason || e.Subject == "" {
		t.Fatalf("err = %v, want %s %s with the path", err, kind, reason)
	}
}

func requireEntries(t *testing.T, dir string, want int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != want {
		t.Fatalf("the folder holds %v (err %v), want %d entries", entries, err, want)
	}
}

// write saves data at path through Create, Writer and Commit.
func write(path string, limit int64, data string) error {
	p, err := Create(path, prefix, limit)
	if err != nil {
		return err
	}
	defer p.Discard()
	if _, err := p.Writer().Write([]byte(data)); err != nil {
		return err
	}
	return p.Commit()
}

func TestPrepareChecksThePathAndLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.json")
	if err := os.WriteFile(existing, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Prepare(filepath.Join(dir, "new.json"), prefix); err != nil {
		t.Fatal(err)
	}
	requireEntries(t, dir, 1)
	requireReason(t, Prepare(existing, prefix), errkind.InvalidInput, errkind.ReasonOutputExists)
	requireReason(t, Prepare(filepath.Join(dir, "missing", "new.json"), prefix), errkind.InvalidInput, errkind.ReasonOutputFolderMissing)
	requireReason(t, Prepare(filepath.Join(existing, "new.json"), prefix), errkind.InvalidInput, errkind.ReasonOutputFolderMissing)
	requireEntries(t, dir, 1)
}

func TestCommitWritesOnceAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")

	if err := write(path, 1<<20, "first"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "first" {
		t.Fatalf("file = %q, err = %v", got, err)
	}
	requireEntries(t, dir, 1)

	requireReason(t, write(path, 1<<20, "second"), errkind.InvalidInput, errkind.ReasonOutputExists)

	// A file that appears between Create and Commit is not overwritten
	// either: the hard link refuses an existing name.
	late := filepath.Join(dir, "late.json")
	p, err := Create(late, prefix, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Writer().Write([]byte("mine?")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(late, []byte("theirs"), 0o600); err != nil {
		t.Fatal(err)
	}
	requireReason(t, p.Commit(), errkind.InvalidInput, errkind.ReasonOutputExists)
	p.Discard()
	if got, _ := os.ReadFile(late); string(got) != "theirs" {
		t.Fatal("the file created meanwhile was overwritten")
	}
	if got, _ := os.ReadFile(path); string(got) != "first" {
		t.Fatal("the existing file was overwritten")
	}
	requireEntries(t, dir, 2)
	requireReason(t, write(filepath.Join(dir, "missing", "x.json"), 1<<20, "x"), errkind.StorageFailed, errkind.ReasonOutputFailed)
}

func TestTheLimitWritesNothingAndSavesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	p, err := Create(path, prefix, 10)
	if err != nil {
		t.Fatal(err)
	}
	w := p.Writer()
	if n, err := w.Write([]byte("0123456")); n != 7 || err != nil {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	n, err := w.Write([]byte("7890"))
	if n != 0 {
		t.Fatalf("a write over the limit wrote %d bytes", n)
	}
	requireReason(t, err, errkind.InvalidInput, errkind.ReasonOutputTooLarge)
	if _, err := w.Write([]byte("x")); err == nil {
		t.Fatal("writes after the limit must keep failing")
	}
	requireReason(t, p.Commit(), errkind.InvalidInput, errkind.ReasonOutputTooLarge)
	p.Discard()
	requireEntries(t, dir, 0)

	exact := filepath.Join(dir, "exact.json")
	if err := write(exact, 4, "1234"); err != nil {
		t.Fatalf("a file of exactly the limit must be saved: %v", err)
	}
}

func TestAFailureOnTheWayLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")

	// The command stops before Commit: Discard removes the temporary file.
	p, err := Create(path, prefix, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Writer().Write([]byte("half a pl")); err != nil {
		t.Fatal(err)
	}
	requireEntries(t, dir, 1)
	p.Discard()
	p.Discard() // a second call does nothing
	requireEntries(t, dir, 0)
	if _, err := p.Writer().Write([]byte("x")); err == nil {
		t.Fatal("a discarded file must refuse writes")
	}
	if err := p.Commit(); err == nil {
		t.Fatal("a discarded file must not be committed")
	}
	(*Pending)(nil).Discard()

	// Commit after Commit fails and keeps the file.
	if err := write(path, 1<<20, "done"); err != nil {
		t.Fatal(err)
	}
	requireEntries(t, dir, 1)
}

func TestWithoutHardLinksTheFileIsRenamed(t *testing.T) {
	oldLink, oldRename := link, rename
	t.Cleanup(func() { link, rename = oldLink, oldRename })
	link = func(string, string) error { return &os.LinkError{Op: "link", Err: errors.ErrUnsupported} }

	t.Run("renames when the name is free", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tagrules.yaml")
		if err := write(path, 1<<20, "version: 1\n"); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != "version: 1\n" {
			t.Fatalf("file = %q, err = %v", got, err)
		}
		requireEntries(t, dir, 1)
	})
	t.Run("refuses an existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tagrules.yaml")
		p, err := Create(path, prefix, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Discard()
		if err := os.WriteFile(path, []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
		requireReason(t, p.Commit(), errkind.InvalidInput, errkind.ReasonOutputExists)
		p.Discard()
		if got, _ := os.ReadFile(path); string(got) != "mine" {
			t.Fatal("the existing file was overwritten")
		}
		requireEntries(t, dir, 1)
	})
	t.Run("leaves nothing when the name cannot be given", func(t *testing.T) {
		rename = func(string, string) error { return &os.LinkError{Op: "rename", Err: fs.ErrPermission} }
		defer func() { rename = oldRename }()
		dir := t.TempDir()
		path := filepath.Join(dir, "tagrules.yaml")
		requireReason(t, write(path, 1<<20, "x"), errkind.StorageFailed, errkind.ReasonOutputFailed)
		requireEntries(t, dir, 0)
	})
}

// NFR8.17: the saved file is readable and writable by its owner only on
// Unix; on Windows it gets the folder's permissions, which are not checked.
func TestTheFileIsTheOwnersOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps the permissions of the folder (NFR8.17)")
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := write(path, 1<<20, "{}"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, err = %v, want 0600", info.Mode(), err)
	}
}

func TestTemporaryFilesUseThePrefix(t *testing.T) {
	dir := t.TempDir()
	p, err := Create(filepath.Join(dir, "plan.json"), ".tag-refactor-plan-", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Discard()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), ".tag-refactor-plan-") || !strings.HasSuffix(entries[0].Name(), ".tmp") {
		t.Fatalf("temporary file = %v", entries)
	}
}
