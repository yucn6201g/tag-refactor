package fileout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of RetryRename and Replace (U8 NFR6.5, NFR9.11, security design
// §7), written after the implementation.

type renameCalls struct {
	calls  int
	errs   []error
	sleeps []time.Duration
}

func (r *renameCalls) rename(string, string) error {
	r.calls++
	if r.calls <= len(r.errs) {
		return r.errs[r.calls-1]
	}
	return nil
}

func (r *renameCalls) sleep(d time.Duration) { r.sleeps = append(r.sleeps, d) }

func always(error) bool { return true }
func never(error) bool  { return false }

func TestRetryRenameSucceedsAtOnce(t *testing.T) {
	r := &renameCalls{}
	if err := RetryRename("a", "b", []time.Duration{time.Second}, always, r.rename, r.sleep); err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 || len(r.sleeps) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestRetryRenameWaitsTheListThenSucceeds(t *testing.T) {
	busy := errors.New("busy")
	r := &renameCalls{errs: []error{busy, busy}}
	waits := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}
	if err := RetryRename("a", "b", waits, always, r.rename, r.sleep); err != nil {
		t.Fatal(err)
	}
	if r.calls != 3 || len(r.sleeps) != 2 || r.sleeps[0] != time.Second || r.sleeps[1] != 2*time.Second {
		t.Fatalf("%+v", r)
	}
}

func TestRetryRenameGivesUpAfterTheList(t *testing.T) {
	busy := errors.New("busy")
	r := &renameCalls{errs: []error{busy, busy, busy, busy, busy, busy}}
	waits := make([]time.Duration, 4)
	if err := RetryRename("a", "b", waits, always, r.rename, r.sleep); !errors.Is(err, busy) {
		t.Fatalf("err = %v", err)
	}
	if r.calls != 5 {
		t.Fatalf("%d calls, want the 5 the record of a run makes", r.calls)
	}
}

func TestRetryRenameStopsAtAnErrorNotToRetry(t *testing.T) {
	bad := errors.New("bad")
	r := &renameCalls{errs: []error{bad}}
	if err := RetryRename("a", "b", []time.Duration{time.Second}, never, r.rename, r.sleep); !errors.Is(err, bad) {
		t.Fatalf("err = %v", err)
	}
	if r.calls != 1 || len(r.sleeps) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestRetryRenameGetsTheNames(t *testing.T) {
	var from, to string
	err := RetryRename("a", "b", nil, always, func(x, y string) error { from, to = x, y; return nil }, func(time.Duration) {})
	if err != nil || from != "a" || to != "b" {
		t.Fatalf("%v %s %s", err, from, to)
	}
}

func TestTransientRenameOnlyForTheMomentsOfWindows(t *testing.T) {
	wrap := func(n syscall.Errno) error { return &os.LinkError{Op: "rename", Old: "a", New: "b", Err: n} }
	for _, n := range []syscall.Errno{5, 32, 33} {
		want := runtime.GOOS == "windows"
		if got := TransientRename(wrap(n)); got != want {
			t.Errorf("errno %d: %v, want %v", n, got, want)
		}
	}
	for _, err := range []error{wrap(2), wrap(112), errors.New("other"), fs.ErrNotExist} {
		if TransientRename(err) {
			t.Errorf("%v must not be retried", err)
		}
	}
}

func TestUserFileWaitsAreAboutFiveSeconds(t *testing.T) {
	var total time.Duration
	for _, w := range UserFileWaits {
		total += w
	}
	if total != 4700*time.Millisecond || len(UserFileWaits) != 6 {
		t.Fatalf("%d waits, %v", len(UserFileWaits), total)
	}
}

func userFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.yaml")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s holds %q (err %v), want %q", path, got, err, want)
	}
}

func TestReplacePutsTheContentAndLeavesNoTemporaryFile(t *testing.T) {
	path := userFile(t, "old")
	opens := 0
	if err := Replace(path, []byte("new\r\n"), ReplaceOptions{Opened: func() { opens++ }}); err != nil {
		t.Fatal(err)
	}
	requireContent(t, path, "new\r\n")
	requireEntries(t, filepath.Dir(path), 1)
	if opens != 2 {
		t.Fatalf("%d opens: the temporary file is created and read back", opens)
	}
}

func TestReplaceKeepsThePermissions(t *testing.T) {
	path := userFile(t, "old")
	if err := Replace(path, []byte("new"), ReplaceOptions{}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v, err %v", info.Mode(), err)
	}
}

func TestReplaceWritesTheTemporaryFileForItsOwnerOnly(t *testing.T) {
	path := userFile(t, "old")
	var mode fs.FileMode
	var name string
	saved := replaceRead
	t.Cleanup(func() { replaceRead = saved })
	replaceRead = func(p string) ([]byte, error) {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		mode, name = info.Mode().Perm(), filepath.Base(p)
		return saved(p)
	}
	if err := Replace(path, []byte("new"), ReplaceOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, ".a.yaml.tag-refactor-") || !strings.HasSuffix(name, ".tmp") {
		t.Fatalf("temporary name %s", name)
	}
	if runtime.GOOS != "windows" && mode != 0o600 {
		t.Fatalf("temporary mode %v", mode)
	}
}

func TestReplaceStopsWhenTheTemporaryFileDiffers(t *testing.T) {
	path := userFile(t, "old")
	saved := replaceRead
	t.Cleanup(func() { replaceRead = saved })
	for name, read := range map[string]func(string) ([]byte, error){
		"other bytes": func(string) ([]byte, error) { return []byte("ne"), nil },
		"unreadable":  func(string) ([]byte, error) { return nil, errors.New("gone") },
	} {
		replaceRead = read
		err := Replace(path, []byte("new"), ReplaceOptions{})
		if errkind.KindOf(err) != errkind.StorageFailed {
			t.Fatalf("%s: err = %v", name, err)
		}
		requireContent(t, path, "old")
		requireEntries(t, filepath.Dir(path), 1)
	}
}

func TestReplaceFailedRenameLeavesTheFile(t *testing.T) {
	path := userFile(t, "old")
	savedRename, savedSleep := replaceRename, replaceSleep
	t.Cleanup(func() { replaceRename, replaceSleep = savedRename, savedSleep })
	calls := 0
	replaceRename = func(string, string) error {
		calls++
		return &os.LinkError{Op: "rename", Err: syscall.Errno(32)}
	}
	replaceSleep = func(time.Duration) {}
	err := Replace(path, []byte("new"), ReplaceOptions{Waits: []time.Duration{0, 0}})
	if e, ok := errkind.As(err); !ok || e.Reason != errkind.ReasonOutputFailed || e.Subject != path {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "new") {
		t.Fatalf("the content in the error: %v", err)
	}
	requireContent(t, path, "old")
	requireEntries(t, filepath.Dir(path), 1)
	want := 1
	if runtime.GOOS == "windows" {
		want = 3
	}
	if calls != want {
		t.Fatalf("%d renames, want %d", calls, want)
	}
}

func TestReplaceUsesTheLongWaitsForAMomentOfWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows retries")
	}
	path := userFile(t, "old")
	savedRename, savedSleep := replaceRename, replaceSleep
	t.Cleanup(func() { replaceRename, replaceSleep = savedRename, savedSleep })
	var waited time.Duration
	calls := 0
	replaceSleep = func(d time.Duration) { waited += d }
	replaceRename = func(a, b string) error {
		calls++
		if calls < 7 {
			return &os.LinkError{Op: "rename", Err: syscall.Errno(32)}
		}
		return savedRename(a, b)
	}
	if err := Replace(path, []byte("new"), ReplaceOptions{}); err != nil {
		t.Fatal(err)
	}
	if calls != 7 || waited != 4700*time.Millisecond {
		t.Fatalf("%d calls, %v", calls, waited)
	}
	requireContent(t, path, "new")
}

func TestReplaceOfAMissingFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.yaml")
	if err := Replace(path, []byte("x"), ReplaceOptions{}); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("err = %v", err)
	}
}

func TestReplaceFailsWhenTheRandomNameDoes(t *testing.T) {
	path := userFile(t, "old")
	saved := randomHex
	t.Cleanup(func() { randomHex = saved })
	randomHex = func() (string, error) { return "", errors.New("no random") }
	if err := Replace(path, []byte("x"), ReplaceOptions{}); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("err = %v", err)
	}
	requireContent(t, path, "old")
}

func TestReplaceDoesNotOverwriteATemporaryFileThatExists(t *testing.T) {
	path := userFile(t, "old")
	saved := randomHex
	t.Cleanup(func() { randomHex = saved })
	randomHex = func() (string, error) { return "same", nil }
	tmp := filepath.Join(filepath.Dir(path), ".a.yaml.tag-refactor-same.tmp")
	if err := os.WriteFile(tmp, []byte("someone's"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Replace(path, []byte("new"), ReplaceOptions{}); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("err = %v", err)
	}
	requireContent(t, tmp, "someone's")
	requireContent(t, path, "old")
}
