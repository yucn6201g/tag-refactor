package iac

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The large tests of U8 (NFR3.19, NFR5.25, NFR5.26): the measuring conditions,
// with the files on disk and git stood in for, unless TAG_REFACTOR_LARGE_REAL_GIT
// is 1. They time the run from its start to its end (making the files is not
// counted), watch the largest heap every 100 milliseconds and count the files
// opened against the formula. The race build divides the size by largeScale and
// stretches the time limit by largeTimeFactor; the coverage job stretches it by
// three, its instrumented code being slower.

// largeLimit is the time limit of the large run.
func largeLimit() time.Duration {
	limit := 60 * time.Second * largeTimeFactor
	if testing.CoverMode() != "" {
		limit *= 3
	}
	return limit
}

const largeHeapLimit = 2 << 30

// watchHeap records the largest HeapInuse until the function it returns is
// called, which gives that largest value.
func watchHeap() func() uint64 {
	var largest atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.HeapInuse > largest.Load() {
			largest.Store(m.HeapInuse)
		}
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				sample()
				return
			case <-ticker.C:
				sample()
			}
		}
	}()
	return func() uint64 {
		close(stop)
		<-done
		return largest.Load()
	}
}

// countOpens counts the files iac opens for reading and the temporary files.
func countOpens(t *testing.T) *atomic.Int64 {
	t.Helper()
	var opens atomic.Int64
	savedOpen, savedCount := openFile, countOpen
	t.Cleanup(func() { openFile, countOpen = savedOpen, savedCount })
	openFile = func(p string) (io.ReadCloser, error) {
		opens.Add(1)
		return os.Open(p)
	}
	countOpen = func() { opens.Add(1) }
	return &opens
}

func TestLargeIacU8(t *testing.T) {
	l, err := testutil.WriteLargeIac(t.TempDir(), largeScale)
	if err != nil {
		t.Fatal(err)
	}
	realGit := os.Getenv("TAG_REFACTOR_LARGE_REAL_GIT") == "1"
	if realGit {
		f := &fixture{t: t, dir: l.Dir}
		f.commit()
	} else {
		noGit(t)
	}
	opens := countOpens(t)
	finished := watchHeap()
	start := time.Now()

	res, err := New().Run(context.Background(), Request{PlanPath: l.PlanPath, Dir: l.Dir, TFStatePath: l.StatePath, Write: true})

	elapsed := time.Since(start)
	largest := finished()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d resources, %d changes, %d candidates: %v, the largest heap %d MB, %d files opened, real git: %v, reflected %d, to fix by hand %d",
		l.Resources, l.Changes, l.Candidates, elapsed.Round(time.Millisecond), largest>>20, opens.Load(), realGit, res.Reflected, len(res.Manual))
	if res.WriteErr != nil {
		t.Fatalf("write stopped: %v", res.WriteErr)
	}
	if limit := largeLimit(); elapsed > limit {
		t.Errorf("the run took %v, the limit is %v", elapsed, limit)
	}
	if largest > largeHeapLimit {
		t.Errorf("the largest heap was %d MB", largest>>20)
	}
	written := 0
	for i := range res.Files {
		if res.Files[i].Written {
			written++
		}
	}
	bound := int64(l.Candidates + l.FilesWithTargets + written*3)
	if opens.Load() > bound {
		t.Errorf("%d files opened, the bound is %d", opens.Load(), bound)
	}
	if written != l.FilesWithTargets || len(res.SkippedFiles) != 0 || res.Unmanaged != 0 {
		t.Errorf("written %d of %d, left out %d, unmanaged %d", written, l.FilesWithTargets, len(res.SkippedFiles), res.Unmanaged)
	}
	if res.Reflected+len(res.Manual) != l.Changes || res.AlreadyReflected != 0 {
		t.Errorf("reflected %d, to fix by hand %d, already %d, of %d changes", res.Reflected, len(res.Manual), res.AlreadyReflected, l.Changes)
	}
	share := float64(l.ManualResources) / float64(l.Resources)
	if share < 0.05 || share > 0.15 {
		t.Errorf("%.1f%% of the resources need fixing by hand", share*100)
	}
	if res.ExitCode() != 2 {
		t.Errorf("exit code = %d", res.ExitCode())
	}

	// The second run finds everything but the manual ones reflected and writes
	// nothing.
	opens.Store(0)
	start = time.Now()
	again, err := New().Run(context.Background(), Request{PlanPath: l.PlanPath, Dir: l.Dir, TFStatePath: l.StatePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("the second run: %v", time.Since(start).Round(time.Millisecond))
	if len(again.Files) != 0 || again.Reflected != 0 || again.AlreadyReflected == 0 {
		t.Errorf("second run: %d files, reflected %d, already %d", len(again.Files), again.Reflected, again.AlreadyReflected)
	}
}

// Without --write the same folder takes no git and no temporary file.
func TestLargeIacU8DifferenceOnly(t *testing.T) {
	l, err := testutil.WriteLargeIac(t.TempDir(), 10*largeScale)
	if err != nil {
		t.Fatal(err)
	}
	savedLook := lookGit
	t.Cleanup(func() { lookGit = savedLook })
	lookGit = func(string) (string, error) {
		t.Error("git was looked for")
		return "", exec.ErrNotFound
	}
	opens := countOpens(t)

	res, err := New().Run(context.Background(), Request{PlanPath: l.PlanPath, Dir: l.Dir, TFStatePath: l.StatePath})

	if err != nil {
		t.Fatal(err)
	}
	if bound := int64(l.Candidates + l.FilesWithTargets); opens.Load() > bound {
		t.Errorf("%d files opened, the bound is %d", opens.Load(), bound)
	}
	if len(res.Files) != l.FilesWithTargets || res.Files[0].Written {
		t.Fatalf("%d files of %d", len(res.Files), l.FilesWithTargets)
	}
}
