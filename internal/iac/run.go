package iac

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/fileout"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// softMemoryLimit makes the collector work harder near this much memory while
// iac runs (security design §3).
const softMemoryLimit = 1536 << 20

// maxWorkers bounds the goroutines that read and parse files.
const maxWorkers = 8

// The functions that open and replace files: the tests count the opens and make
// a write fail (NFR5.26, NFR6.6).
var (
	openFile    = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	replaceFile = fileout.Replace
)

// runner holds one run of Run.
type runner struct {
	req    Request
	root   string
	res    *Result
	budget *budget
}

// Run does workflow W1: the input, the targets, the places, the edits, the
// difference and, with --write, the checks and the writing. An error is a wrong
// input or a stop before anything was decided; once there is a result, the
// problems of --write are in Result.WriteErr (BR6.2).
func (w *Writer) Run(ctx context.Context, req Request) (*Result, error) {
	root, err := resolveRoot(req.Dir)
	if err != nil {
		return nil, err
	}
	plan, err := loadPlan(req.PlanPath)
	if err != nil {
		return nil, err
	}
	var state stateIndex
	if req.TFStatePath != "" {
		if state, err = readTFState(req.TFStatePath); err != nil {
			return nil, err
		}
	}
	defer setMemoryLimit()()
	targets, res := buildTargets(plan, state)
	res.HasTFState, res.WriteRequested = req.TFStatePath != "", req.Write
	r := &runner{req: req, root: root, res: res, budget: newBudget()}
	if len(targets) > 0 {
		if err := r.reflect(ctx, targets); err != nil {
			return nil, err
		}
	}
	sortItems(res.Manual)
	sortItems(res.AssumedRemovals)
	return res, nil
}

// setMemoryLimit lowers the soft memory limit of the process, unless it is lower
// already, and returns how to put it back.
func setMemoryLimit() func() {
	before := debug.SetMemoryLimit(-1)
	debug.SetMemoryLimit(min(before, softMemoryLimit))
	return func() { debug.SetMemoryLimit(before) }
}

func sortItems(items []ManualItem) {
	slices.SortFunc(items, func(a, b ManualItem) int {
		if c := strings.Compare(a.ARN, b.ARN); c != 0 {
			return c
		}
		return strings.Compare(a.ChangeID, b.ChangeID)
	})
}

// reflect does everything after the targets are known.
func (r *runner) reflect(ctx context.Context, targets []*target) error {
	wantCF, wantTF := false, false
	for _, t := range targets {
		wantCF = wantCF || t.tool == ToolCloudFormation
		wantTF = wantTF || t.tool == ToolTerraform
	}
	found, err := walkRoot(r.root, wantCF, wantTF)
	if err != nil {
		return err
	}
	r.res.SkippedFolders = found.folders
	r.res.SkippedFiles = found.skipped
	firsts, err := r.readAll(ctx, found.candidates)
	if err != nil {
		return err
	}
	tasks := r.placeTargets(targets, found.candidates, firsts)
	outs, err := r.processAll(ctx, tasks)
	if err != nil {
		return err
	}
	return r.finish(ctx, outs)
}

// forEach calls work for 0 to n-1 on a few goroutines and stops at the first
// error. A panic of a worker becomes an Internal error that stops the run: the
// other files are not worth trusting after it.
func forEach(ctx context.Context, n int, work func(context.Context, int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var next atomic.Int64
	var once sync.Once
	var firstErr error
	fail := func(err error) {
		once.Do(func() { firstErr = err; cancel() })
	}
	var wg sync.WaitGroup
	for range min(runtime.NumCPU(), maxWorkers, max(n, 1)) {
		wg.Go(func() {
			defer func() {
				if p := recover(); p != nil {
					fail(&errkind.Error{Kind: errkind.Internal, Op: "iac", Detail: "a worker stopped unexpectedly"})
				}
			}()
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				if err := work(ctx, i); err != nil {
					fail(err)
					return
				}
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// firstRead is what the first reading of a candidate leaves: the index, and the
// hash to see at the second reading that the file did not change. The parse
// itself is not kept (NFR5.26).
type firstRead struct {
	refs []resourceRef
	sum  [sha256.Size]byte
	skip *SkippedFile
}

// readFile reads a candidate, up to its limit.
func readFile(p string) ([]byte, error) {
	f, err := openFile(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, maxFileBytes+1))
}

func (r *runner) readAll(ctx context.Context, candidates []*candidate) ([]firstRead, error) {
	firsts := make([]firstRead, len(candidates))
	err := forEach(ctx, len(candidates), func(ctx context.Context, i int) error {
		var err error
		firsts[i], err = r.readOne(ctx, candidates[i])
		return err
	})
	return firsts, err
}

// readOne reads, checks and parses one candidate to make its index (security
// design §2, §3). Nothing but the budget stops it: a bad file is only left out.
func (r *runner) readOne(ctx context.Context, c *candidate) (firstRead, error) {
	skip := func(s SkippedFile) (firstRead, error) {
		s.File = c.rel
		return firstRead{skip: &s}, nil
	}
	raw, err := readFile(c.path)
	if err != nil {
		return skip(SkippedFile{Reason: SkipUnreadable})
	}
	if int64(len(raw)) > maxFileBytes {
		return skip(SkippedFile{Reason: SkipTooLarge})
	}
	first := firstRead{sum: sha256.Sum256(raw)}
	ct, reason, parse := prepare(c, raw)
	if !parse {
		return first, nil
	}
	if reason != "" {
		return skip(SkippedFile{Reason: reason})
	}
	release, err := r.budget.acquire(ctx, len(raw))
	if err != nil {
		return firstRead{}, err
	}
	defer release()
	parsed, failure := parseContent(ct)
	if failure != nil {
		return skip(SkippedFile{Reason: SkipParseError, Line: failure.line, Column: failure.column, Kind: failure.kind})
	}
	first.refs = parsed.refs()
	return first, nil
}

// placedTarget is a target with its one place.
type placedTarget struct {
	t  *target
	at place
}

// fileTask is the work on one file that holds targets.
type fileTask struct {
	c       *candidate
	sum     [sha256.Size]byte
	targets []placedTarget
}

// placeTargets makes the index of all files in name order, finds the place of
// each target and gathers the targets by file (BR3.2, BR3.3).
func (r *runner) placeTargets(targets []*target, candidates []*candidate, firsts []firstRead) []*fileTask {
	index := newPlaceIndex()
	byRel := make(map[string]int, len(candidates))
	for i, c := range candidates {
		byRel[c.rel] = i
		if s := firsts[i].skip; s != nil {
			r.res.SkippedFiles = append(r.res.SkippedFiles, *s)
			continue
		}
		index.add(c.tool(), c.rel, firsts[i].refs)
	}
	slices.SortFunc(r.res.SkippedFiles, func(a, b SkippedFile) int { return strings.Compare(a.File, b.File) })
	for _, s := range r.res.SkippedFiles {
		if strings.HasSuffix(strings.ToLower(s.File), ".tf") {
			index.skippedTF = append(index.skippedTF, s.File)
		} else {
			index.skippedCF = append(index.skippedCF, s.File)
		}
	}
	byFile := map[string]*fileTask{}
	var tasks []*fileTask
	for _, t := range targets {
		at, ok := index.find(r.res, t)
		if !ok {
			continue
		}
		task := byFile[at.file]
		if task == nil {
			i := byRel[at.file]
			task = &fileTask{c: candidates[i], sum: firsts[i].sum}
			byFile[at.file] = task
			tasks = append(tasks, task)
		}
		task.targets = append(task.targets, placedTarget{t, at})
	}
	slices.SortFunc(tasks, func(a, b *fileTask) int { return strings.Compare(a.c.rel, b.c.rel) })
	return tasks
}

// fileOutcome is the result of the work on one file.
type fileOutcome struct {
	c         *candidate
	reflected int
	already   int
	manual    []ManualItem
	assumed   []ManualItem
	debug     []string
	// updated, diff and blob are set for a file with edits.
	updated []byte
	diff    string
	blob    gitFile
	sum     [sha256.Size]byte
}

func (r *runner) processAll(ctx context.Context, tasks []*fileTask) ([]*fileOutcome, error) {
	outs := make([]*fileOutcome, len(tasks))
	err := forEach(ctx, len(tasks), func(ctx context.Context, i int) error {
		var err error
		outs[i], err = r.processFile(ctx, tasks[i])
		return err
	})
	return outs, err
}

// processFile reads a file with targets again and makes its edits.
func (r *runner) processFile(ctx context.Context, task *fileTask) (*fileOutcome, error) {
	out := &fileOutcome{c: task.c}
	raw, err := readFile(task.c.path)
	if err != nil || sha256.Sum256(raw) != task.sum {
		// The file changed between the two readings.
		out.markUnreadable(task.targets, "the file changed while it was read")
		return out, nil
	}
	ct, reason, parse := prepare(task.c, raw)
	if !parse || reason != "" {
		out.markUnreadable(task.targets, "the file changed while it was read")
		return out, nil
	}
	release, err := r.budget.acquire(ctx, len(raw))
	if err != nil {
		return nil, err
	}
	defer release()
	parsed, failure := parseContent(ct)
	if failure != nil {
		out.markUnreadable(task.targets, "the file does not parse on the second reading")
		return out, nil
	}
	r.editFile(out, task, ct, parsed)
	return out, nil
}

// editFile applies the changes of the targets and, when the edits pass the
// check, keeps the new content.
func (r *runner) editFile(out *fileOutcome, task *fileTask, ct *content, parsed parsedFile) {
	var edits []edit
	var expected []expectedTags
	var reflected []placedChange
	unreadable := false
	for _, pt := range task.targets {
		key := pt.t.logicalID
		if pt.t.tool == ToolTerraform {
			key = pt.t.address
		}
		site := parsed.site(key)
		sim := simulate(pt.t, site, pt.at, false)
		if allRemoved(site, sim) && !canReplaceWhole(ct.raw, ct.format, site) {
			sim = simulate(pt.t, site, pt.at, true)
		}
		unreadable = unreadable || site != nil && site.issue == ReasonUnreadableFile
		out.manual = append(out.manual, sim.manual...)
		out.assumed = append(out.assumed, sim.assumed...)
		out.already += sim.already
		for _, c := range sim.reflected {
			reflected = append(reflected, placedChange{pt, c})
		}
		if site == nil || site.issue != "" {
			continue
		}
		edits = append(edits, buildEdits(ct.raw, ct.format, site, sim.final)...)
		expected = append(expected, expectedTags{key: key, final: sim.final, others: site.others})
	}
	if len(reflected) == 0 {
		return
	}
	fail := func(why string) {
		for _, p := range reflected {
			out.manual = append(out.manual, ManualItem{ARN: p.pt.t.arn, ChangeID: p.c.ChangeID, Key: changeKey(p.c),
				File: p.pt.at.file, Line: p.pt.at.line, Reason: ReasonUnreadableFile})
		}
		out.debug = append(out.debug, task.c.rel+": "+why)
	}
	if unreadable {
		fail("a position in the file did not read back as the parser said")
		return
	}
	normal, err := normalize(edits)
	if err != nil {
		fail(err.Error())
		return
	}
	updated := applyEdits(ct.raw, normal)
	if why := checkEdits(ct, updated, normal, expected, parsed.allTags()); why != "" {
		fail(why)
		return
	}
	out.reflected = len(reflected)
	out.updated = updated
	out.diff = unifiedDiff(task.c.rel, ct.raw, updated, normal)
	out.blob = blobHashes(task.c.rel, ct.raw)
	out.sum = task.sum
}

// placedChange is a change the edits make, with its target.
type placedChange struct {
	pt placedTarget
	c  planner.Change
}

// allRemoved reports that the changes take every element out of the place.
func allRemoved(site *tagSite, sim *outcomeOfTarget) bool {
	if site == nil || site.issue != "" || len(site.entries) == 0 {
		return false
	}
	for i := range sim.final {
		if sim.final[i].orig >= 0 {
			return false
		}
	}
	return true
}

// markUnreadable puts every change of the targets on the list to fix by hand.
func (o *fileOutcome) markUnreadable(targets []placedTarget, why string) {
	for _, pt := range targets {
		for _, c := range pt.t.changes {
			o.manual = append(o.manual, ManualItem{ARN: pt.t.arn, ChangeID: c.ChangeID, Key: changeKey(c),
				File: pt.at.file, Line: pt.at.line, Reason: ReasonUnreadableFile})
		}
	}
	o.debug = append(o.debug, o.c.rel+": "+why)
}

// finish gathers the outcomes in file name order and, with --write, writes.
func (r *runner) finish(ctx context.Context, outs []*fileOutcome) error {
	res := r.res
	var toWrite []*fileOutcome
	for _, o := range outs {
		res.Manual = append(res.Manual, o.manual...)
		res.AssumedRemovals = append(res.AssumedRemovals, o.assumed...)
		res.AlreadyReflected += o.already
		res.Debug = append(res.Debug, o.debug...)
		if o.reflected > 0 {
			res.Reflected += o.reflected
			res.Files = append(res.Files, FileResult{File: o.c.rel, Tool: o.c.tool(), Reflected: o.reflected, Diff: o.diff})
			toWrite = append(toWrite, o)
		}
	}
	if !r.req.Write || len(toWrite) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	blobs := make([]gitFile, len(toWrite))
	for i, o := range toWrite {
		blobs[i] = o.blob
	}
	if err := gitCheck(ctx, r.root, blobs, &res.Debug); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res.WriteErr = err
		return nil
	}
	r.write(ctx, toWrite)
	return nil
}

// write checks every file again and then writes them one by one (BR5.3): a file
// changed or read-only stops everything before the first write, and a failure
// half way leaves the list of what was written.
func (r *runner) write(ctx context.Context, files []*fileOutcome) {
	res := r.res
	for _, o := range files {
		if err := checkBeforeWriting(o); err != nil {
			res.WriteErr = err
			return
		}
	}
	for i, o := range files {
		if ctx.Err() != nil {
			res.Interrupted = true
			return
		}
		err := replaceFile(o.c.path, o.updated, fileout.ReplaceOptions{Opened: countOpen})
		if err != nil {
			res.WriteErr = &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
				DetailID: "iac.writeFailed", DetailArgs: []any{o.c.rel}, HintID: "iac.writeFailedFix", Cause: err}
			return
		}
		res.Files[i].Written = true
	}
}

// countOpen is called for each file Replace opens; the tests count them.
var countOpen = func() {}

// checkBeforeWriting reads a file a last time (NFR6.6).
func checkBeforeWriting(o *fileOutcome) error {
	info, err := os.Lstat(o.c.path)
	if err == nil && !info.Mode().IsRegular() {
		return usageError("", "iac.changedSinceRead", "iac.changedSinceReadFix", o.c.rel)
	}
	if err == nil && info.Mode().Perm()&0o200 == 0 {
		return usageError("", "iac.readOnly", "iac.readOnlyFix", o.c.rel)
	}
	raw, err := readFile(o.c.path)
	if err != nil || sha256.Sum256(raw) != o.sum {
		return usageError("", "iac.changedSinceRead", "iac.changedSinceReadFix", o.c.rel)
	}
	return nil
}
