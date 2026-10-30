package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Unit tests of rollback inside the package (U7 step 10): the two stages of
// Ctrl+C, the confirmation while it waits, and the readers rollback gets.

var runIDOf = regexp.MustCompile(`tag-refactor rollback --run (\S+)`)

// applied applies the plan of the rig and returns its run ID.
func (r *applyRig) applied() string {
	r.t.Helper()
	res := r.run(context.Background(), Deps{}, "apply", "--plan", r.plan, "--yes")
	m := runIDOf.FindStringSubmatch(res.stderr)
	if res.code != ExitOK || m == nil {
		r.t.Fatalf("apply: %d\n%s", res.code, res.stderr)
	}
	return m[1]
}

func (r *applyRig) rollbackRecord(runID string) journal.RunRecord {
	r.t.Helper()
	j := journal.New(journal.Options{Home: r.env.Dir})
	rec, err := j.Load(context.Background(), testutil.TestAccount, runID, journal.Location{})
	if err != nil {
		r.t.Fatal(err)
	}
	return *rec
}

// BR5.5: two Ctrl+C while a write of rollback runs: the first stops new
// units, the second cancels the call; the record keeps the rollback not
// completed with the call's resource unknown, the lock is removed, and the
// command to run it again is shown.
func TestTwoCtrlCCancelTheRunningRollback(t *testing.T) {
	r := newApplyRig(t)
	runID := r.applied()
	signals, subscribe := signalPipe()
	var once sync.Once
	r.account.OnWrite(func(testutil.Call) {
		once.Do(func() {
			signals <- os.Interrupt
			signals <- os.Interrupt
		})
	})
	r.account.ReplyInstead(func(c testutil.Call) *testutil.Reply {
		if c.Op == testutil.OpGetResources {
			return nil
		}
		return &testutil.Reply{Delay: time.Minute}
	})

	res := r.run(context.Background(), Deps{interrupts: subscribe}, "rollback", "--run", runID, "--yes")

	if res.code != ExitError || !strings.Contains(res.stderr, "Stopped by Ctrl+C") ||
		!strings.Contains(res.stderr, "To undo the rest, run again: tag-refactor rollback --run "+runID) {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
	rec := r.rollbackRecord(runID)
	if len(rec.Rollbacks) != 1 || rec.Rollbacks[0].Completed {
		t.Fatalf("rollbacks %+v", rec.Rollbacks)
	}
	unknown := 0
	for _, res := range rec.Rollbacks[0].Results {
		if res.Status == "failed" && strings.HasPrefix(res.Reason, "resultUnknown") {
			unknown++
		}
	}
	if unknown == 0 {
		t.Fatalf("results %+v", rec.Rollbacks[0].Results)
	}
	if _, err := os.Stat(filepath.Join(r.env.Dir, ".tag-refactor", "runs", testutil.TestAccount, ".lock")); !os.IsNotExist(err) {
		t.Fatal("the lock was left")
	}
	if n := r.fake.InFlight(); n != 0 {
		t.Fatalf("%d calls still running", n)
	}
}

// BR4.2: Ctrl+C while the confirmation waits is not yes: nothing is
// written back, the record is unchanged, and the exit code is 0.
func TestCtrlCAtTheRollbackConfirmationUndoesNothing(t *testing.T) {
	r := newApplyRig(t)
	runID := r.applied()
	before := r.rollbackRecord(runID)
	writes := r.fake.Count(testutil.OpTagResources)
	ctx, cancel := context.WithCancel(context.Background())
	in := blockingReader{done: make(chan struct{})}
	defer close(in.done)
	_, subscribe := signalPipe()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	res := r.run(ctx, Deps{Stdin: in, StdinIsTerminal: func() bool { return true }, interrupts: subscribe}, "rollback", "--run", runID)

	if res.code != ExitOK || !strings.Contains(res.stderr, "Nothing was undone.") || !strings.Contains(res.stderr, "(nothing undone)") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
	if r.fake.Count(testutil.OpTagResources) != writes || len(r.rollbackRecord(runID).Rollbacks) != len(before.Rollbacks) {
		t.Fatal("something was written")
	}
}

// A line without its line break is not yes, and a missing home folder is an
// error before AWS is called.
func TestRollbackNeedsYesWithItsLineBreakAndAHome(t *testing.T) {
	r := newApplyRig(t)
	runID := r.applied()
	res := r.run(context.Background(), Deps{Stdin: strings.NewReader("yes"), StdinIsTerminal: func() bool { return true }}, "rollback", "--run", runID)
	if res.code != ExitOK || !strings.Contains(res.stderr, "Nothing was undone.") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", os.ErrNotExist }
	defer func() { userHomeDir = old }()
	calls := len(r.fake.Calls())
	res = r.run(context.Background(), Deps{}, "rollback", "--run", runID, "--yes")
	if res.code != ExitError || !strings.Contains(res.stderr, "the home folder could not be found") || len(r.fake.Calls()) != calls {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
}

// NFR3.13: the readers rollback connects with cannot write; only rollback
// itself builds the writer, from the session it keeps.
func TestTheReadersOfRollbackCannotWrite(t *testing.T) {
	if _, ok := any(rollbackReaders{}).(awsaccess.TagWriter); ok {
		t.Fatal("rollbackReaders is a TagWriter")
	}
	if _, ok := any(rollbackReaders{}).(awsaccess.ObjectStore); ok {
		t.Fatal("rollbackReaders is an ObjectStore")
	}
	if recordError(os.ErrNotExist, "1") != os.ErrNotExist {
		t.Fatal("an error without a kind passes as it is")
	}
}
