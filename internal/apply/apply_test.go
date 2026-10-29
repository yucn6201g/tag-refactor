package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Unit tests of the apply workflow (U6 step 10), written after the
// implementation. The AWS of these tests is in memory (memAWS); the HTTP
// fake runs the large tests and the acceptance tests.

const acct = "123456789012"

// memAWS is an in-memory account for the applier: it keeps tags, answers
// reads by ARN and writes, and fails as a test asks.
type memAWS struct {
	mu     sync.Mutex
	tags   map[string]map[string]string
	writes []memWrite
	reads  int
	calls  int
	// fail returns the per-resource failure of a write, or nil.
	fail func(op, arn string) *errkind.Error
	// callErr returns the error of a whole write call n (from 1), or nil.
	callErr func(op string, n int) error
	// readErr returns the error of read n (from 1), or nil.
	readErr func(n int) error
	// onWrite runs at the start of every write call.
	onWrite func(op string, arns []string)
	// writeOnError makes a whole-call error still write (a timeout after
	// AWS wrote).
	writeOnError bool
}

type memWrite struct {
	op, region string
	arns       []string
	set        map[string]string
	keys       []string
}

func newMemAWS() *memAWS { return &memAWS{tags: map[string]map[string]string{}} }

func (m *memAWS) put(arn string, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tags[arn] = maps.Clone(tags)
}

func (m *memAWS) GetTags(_ context.Context, _ string, arns []string) (map[string]map[string]string, error) {
	m.mu.Lock()
	m.reads++
	n, readErr := m.reads, m.readErr
	m.mu.Unlock()
	if readErr != nil {
		if err := readErr(n); err != nil {
			return nil, err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]map[string]string{}
	for _, a := range arns {
		if t, ok := m.tags[a]; ok {
			out[a] = maps.Clone(t)
		}
	}
	return out, nil
}

func (m *memAWS) write(ctx context.Context, w memWrite) (awsaccess.WriteResult, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.writes = append(m.writes, w)
	hook, callErr := m.onWrite, m.callErr
	m.mu.Unlock()
	if hook != nil {
		hook(w.op, w.arns)
	}
	var err error
	if callErr != nil {
		err = callErr(w.op, n)
	}
	if err == nil && ctx.Err() != nil {
		err = &errkind.Error{Kind: errkind.Internal, Op: w.op, Cause: ctx.Err()}
	}
	if err != nil && !m.writeOnError {
		return awsaccess.WriteResult{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	res := awsaccess.WriteResult{Failed: map[string]*errkind.Error{}}
	for _, arn := range w.arns {
		if m.fail != nil {
			if e := m.fail(w.op, arn); e != nil {
				res.Failed[arn] = e
				continue
			}
		}
		t, ok := m.tags[arn]
		if !ok {
			res.Failed[arn] = &errkind.Error{Kind: errkind.NotFound, Op: w.op}
			continue
		}
		maps.Copy(t, w.set)
		for _, k := range w.keys {
			delete(t, k)
		}
		if len(userTags(t)) > MaxTags {
			panic("the applier went over 50 tags on the way")
		}
	}
	return res, err
}

func (m *memAWS) TagResources(ctx context.Context, region string, arns []string, tags map[string]string) (awsaccess.WriteResult, error) {
	return m.write(ctx, memWrite{op: awsaccess.OpTagResources, region: region, arns: arns, set: tags})
}

func (m *memAWS) UntagResources(ctx context.Context, region string, arns []string, keys []string) (awsaccess.WriteResult, error) {
	return m.write(ctx, memWrite{op: awsaccess.OpUntagResources, region: region, arns: arns, keys: keys})
}

// sink records the progress and the notices.
type sink struct {
	mu      sync.Mutex
	done    []int
	notices []string
}

func (s *sink) ItemsProcessed(done, _ int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = append(s.done, done)
}

func (s *sink) Notice(id string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notices = append(s.notices, fmt.Sprintf(id+" %v", args))
}

// world is one applier with its in-memory account and its own home.
type world struct {
	t    *testing.T
	aws  *memAWS
	sink *sink
	home string
	j    *journal.Journal
	a    *Applier
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, aws: newMemAWS(), sink: &sink{}, home: t.TempDir()}
	w.j = journal.New(journal.Options{Home: w.home, ToolVersion: "v-test"})
	w.a = New(Options{AWS: w.aws, Journal: w.j, Sink: w.sink})
	return w
}

func rename(from, to string, merges bool) planner.Change {
	return planner.Change{Kind: planner.KindRenameKey, FromKey: from, ToKey: to, MergesIntoExisting: merges}
}

func value(key, from, to string) planner.Change {
	return planner.Change{Kind: planner.KindChangeValue, FromKey: key, FromValue: from, ToValue: to}
}

func add(key, v string) planner.Change {
	return planner.Change{Kind: planner.KindAddRequired, ToKey: key, ToValue: v}
}

// resource is a planned resource of us-east-1 whose tags are also put in the
// account.
func (w *world) resource(name string, tags map[string]string, changes ...planner.Change) planner.PlannedResource {
	arn := "arn:aws:sqs:us-east-1:" + acct + ":" + name
	w.aws.put(arn, tags)
	return planner.PlannedResource{ARN: arn, Region: "us-east-1", ResourceType: "sqs", BeforeTags: maps.Clone(tags), Changes: changes}
}

func plan(resources ...planner.PlannedResource) *planner.Plan {
	return &planner.Plan{PlanID: "plan-1", AccountID: acct, Regions: []string{"us-east-1"}, Resources: resources,
		InventoryComplete: true, ImpactAnalysisComplete: true}
}

func (w *world) run(p *planner.Plan) *Result {
	w.t.Helper()
	res, err := w.a.Run(context.Background(), Request{Plan: p, AccountID: acct}, nil)
	if err != nil {
		w.t.Fatalf("run: %v", err)
	}
	return res
}

func (w *world) record(res *Result) journal.RunRecord {
	w.t.Helper()
	data, err := os.ReadFile(res.Record)
	if err != nil {
		w.t.Fatal(err)
	}
	var rec journal.RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		w.t.Fatal(err)
	}
	return rec
}

func (w *world) requireNoLock() {
	w.t.Helper()
	if _, err := os.Stat(filepath.Join(journal.DefaultDir(w.home, acct), ".lock")); !os.IsNotExist(err) {
		w.t.Fatalf("the lock is left (err %v)", err)
	}
}

func target(res *Result, name string) *Target {
	for _, t := range res.Targets {
		if strings.HasSuffix(t.ARN, ":"+name) {
			return t
		}
	}
	return nil
}

// sizes changes the sizes of the work for one test.
func sizes(t *testing.T, workers, batch, unit, limit int) {
	t.Helper()
	w, b, u, l := regionWorkers, batchSize, unitSize, transientLimit
	regionWorkers, batchSize, unitSize, transientLimit = workers, batch, unit, limit
	t.Cleanup(func() { regionWorkers, batchSize, unitSize, transientLimit = w, b, u, l })
}

// --- the expected tags and the limit (BR3.4 to BR3.6) ---

// The four examples of the U6 functional specification.
func TestExpectedTagsOfTheFunctionalSpecification(t *testing.T) {
	fifty := map[string]string{"env": "Prod"}
	for i := 0; len(fifty) < 50; i++ {
		fifty[fmt.Sprintf("k%02d", i)] = "v"
	}
	for _, c := range []struct {
		name        string
		current     map[string]string
		changes     []planner.Change
		set         string
		remove      string
		removeFirst bool
	}{
		{"rename then value", map[string]string{"env": "Prod"}, []planner.Change{rename("env", "Environment", false), value("Environment", "Prod", "prod")},
			"Environment=prod", "env", false},
		{"merge into an existing key", map[string]string{"env": "Prod", "Environment": "prod"}, []planner.Change{rename("env", "Environment", true)},
			"", "env", false},
		{"aws: keys stay", map[string]string{"Team": "a", "aws:cloudformation:stack-name": "x"}, []planner.Change{add("CostCenter", "100")},
			"CostCenter=100", "", false},
		{"fifty all the way", fifty, []planner.Change{rename("env", "Environment", false)}, "Environment=Prod", "env", true},
	} {
		tg := &Target{Current: c.current, Changes: c.changes}
		tg.compute(func(string, string) { t.Errorf("%s: unexpected aws: warning", c.name) })
		var set []string
		for _, k := range slices.Sorted(maps.Keys(tg.TagsToSet)) {
			set = append(set, k+"="+tg.TagsToSet[k])
		}
		if strings.Join(set, ",") != c.set || strings.Join(tg.KeysToRemove, ",") != c.remove || tg.RemoveFirst != c.removeFirst {
			t.Errorf("%s: set %v remove %v removeFirst %v", c.name, set, tg.KeysToRemove, tg.RemoveFirst)
		}
		if c.name == "aws: keys stay" && tg.PlannedAfter["aws:cloudformation:stack-name"] != "x" {
			t.Errorf("planned after %v", tg.PlannedAfter)
		}
	}
}

func TestOverFiftyTagsIsSkippedAndNothingToWriteIsNotRecorded(t *testing.T) {
	fifty := map[string]string{}
	for i := 0; len(fifty) < 50; i++ {
		fifty[fmt.Sprintf("k%02d", i)] = "v"
	}
	tg := &Target{Current: fifty, Changes: []planner.Change{add("Owner", "x")}}
	tg.compute(func(string, string) {})
	if tg.Status != journal.StatusSkipped || tg.Reason != ReasonTagLimitExceeded || tg.TagCount != 51 || len(tg.TouchedKeys) != 0 {
		t.Fatalf("51 tags: %+v", tg)
	}
	same := &Target{Current: map[string]string{"Environment": "prod"}, Changes: []planner.Change{value("Environment", "prod", "prod")}}
	same.compute(func(string, string) {})
	if same.Reason != ReasonNothingToWrite {
		t.Fatalf("nothing to write: %+v", same)
	}
}

// BR3.5: a change of a key starting with aws: is warned about and never
// written, even if a plan names one.
func TestChangesOfAWSKeysAreNeverWritten(t *testing.T) {
	var warned []string
	tg := &Target{ARN: "arn:x", Current: map[string]string{"aws:x": "1", "Env": "a"},
		Changes: []planner.Change{{Kind: planner.KindRemoveKey, FromKey: "aws:x"}, add("AWS:y", "2"), add("Owner", "o")}}
	tg.compute(func(arn, key string) { warned = append(warned, key) })
	if strings.Join(warned, ",") != "aws:x,AWS:y" || len(tg.TagsToSet) != 1 || tg.TagsToSet["Owner"] != "o" || len(tg.KeysToRemove) != 0 ||
		tg.PlannedAfter["aws:x"] != "1" || tg.PlannedAfter["AWS:y"] != "" {
		t.Fatalf("warned %v, target %+v", warned, tg)
	}
}

// --- batches and units (NFR5.13) ---

// Keys and values with =, commas, line breaks, spaces and quotes never make
// two different writes share a batch key.
func TestBatchKeysNeverCollide(t *testing.T) {
	writes := []*Target{
		{TagsToSet: map[string]string{"a": "b,c=d"}},
		{TagsToSet: map[string]string{"a": "b", "c": "d"}},
		{TagsToSet: map[string]string{"a=b": "c"}},
		{TagsToSet: map[string]string{"a": "b=c"}},
		{TagsToSet: map[string]string{"a": "b\nc"}},
		{TagsToSet: map[string]string{"a": `b","c`}},
		{TagsToSet: map[string]string{"a": "b "}},
		{KeysToRemove: []string{"a,b"}},
		{KeysToRemove: []string{"a", "b"}},
		{TagsToSet: map[string]string{"a": "b"}, KeysToRemove: []string{"c"}},
		{TagsToSet: map[string]string{"a": "b"}, KeysToRemove: []string{"c"}, RemoveFirst: true},
	}
	seen := map[string]int{}
	for i, w := range writes {
		k := groupKey(w)
		if j, dup := seen[k]; dup {
			t.Errorf("writes %d and %d share the key %s", j, i, k)
		}
		seen[k] = i
	}
}

func TestBatchesHoldTwentyAndUnitsAHundred(t *testing.T) {
	var targets []*Target
	for i := range 130 {
		v := "a"
		if i%3 == 0 {
			v = "b"
		}
		region := "us-east-1"
		if i >= 120 {
			region = "eu-west-1"
		}
		targets = append(targets, &Target{ARN: fmt.Sprintf("arn:%03d", 999-i), Region: region, TagsToSet: map[string]string{"k": v}})
	}
	regions, units := planUnits(targets)
	if strings.Join(regions, ",") != "eu-west-1,us-east-1" {
		t.Fatalf("regions %v", regions)
	}
	total := 0
	for _, region := range regions {
		for _, u := range units[region] {
			if n := len(u.targets()); n > 100 {
				t.Errorf("a unit of %d", n)
			}
			for _, b := range u.batches {
				total += len(b.targets)
				if len(b.targets) > 20 || !sort.SliceIsSorted(b.targets, func(i, j int) bool { return b.targets[i].ARN < b.targets[j].ARN }) {
					t.Errorf("batch of %d, sorted %v", len(b.targets), b.targets)
				}
				for _, tg := range b.targets {
					if groupKey(tg) != b.key || tg.Region != region {
						t.Error("a batch mixes keys or regions")
					}
				}
			}
		}
	}
	if total != 130 || len(units["us-east-1"]) != 2 {
		t.Fatalf("total %d, units of us-east-1 %d", total, len(units["us-east-1"]))
	}
	if got := chunks([]string{"a", "b", "c"}, 2); len(got) != 2 || len(got[1]) != 1 {
		t.Fatalf("chunks %v", got)
	}
}

// --- the checks before writing (BR3.3, BR4.7, never-tagged resources) ---

func TestTheChecksBeforeWriting(t *testing.T) {
	w := newWorld(t)
	changed := w.resource("changed", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	gone := w.resource("gone", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	never := w.resource("never", map[string]string{}, add("Owner", "x"))
	awsOnly := w.resource("awsonly", map[string]string{"env": "dev", "aws:x": "1"}, rename("env", "Environment", false))
	w.aws.put(changed.ARN, map[string]string{"env": "prod"})
	w.aws.put(awsOnly.ARN, map[string]string{"env": "dev", "aws:x": "2"})
	w.aws.mu.Lock()
	delete(w.aws.tags, gone.ARN)
	delete(w.aws.tags, never.ARN) // never tagged: not returned
	w.aws.tags[never.ARN+"-shadow"] = nil
	w.aws.mu.Unlock()

	res := w.run(plan(changed, gone, never, awsOnly))

	if r := target(res, "changed"); r.Status != journal.StatusSkipped || r.Reason != ReasonChangedSincePlan {
		t.Errorf("changed: %+v", r)
	}
	if r := target(res, "gone"); r.Status != journal.StatusSkipped || r.Reason != ReasonResourceNotFound {
		t.Errorf("gone: %+v", r)
	}
	// A resource the plan saw without tags of its own is written as empty;
	// it is missing here, so the write finds it gone.
	if r := target(res, "never"); r.Status != journal.StatusSkipped || r.Reason != ReasonResourceNotFound || len(w.aws.writes) == 0 {
		t.Errorf("never tagged: %+v", r)
	}
	if r := target(res, "awsonly"); r.Status != journal.StatusApplied {
		t.Errorf("a change of an aws: tag only is not a change: %+v", r)
	}
	rec := w.record(res)
	for _, e := range rec.Entries {
		if strings.HasSuffix(e.ARN, ":changed") && (len(e.TouchedKeys) != 0 || e.BeforeTags["env"] != "prod" || e.PlannedAfterTags["env"] != "prod") {
			t.Errorf("record of changed: %+v", e)
		}
		if strings.HasSuffix(e.ARN, ":gone") && (e.BeforeTags["env"] != "dev" || e.Status != journal.StatusSkipped) {
			t.Errorf("record of gone: %+v", e)
		}
	}
	if res.ExitCode(plan()) != 2 {
		t.Errorf("exit code %d", res.ExitCode(plan()))
	}
}

// BR4.7: a resource changed between the first read and its batch is skipped
// and its touched keys stay in the record.
func TestAResourceChangedBeforeItsBatchIsSkipped(t *testing.T) {
	w := newWorld(t)
	a := w.resource("a", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	w.aws.readErr = func(n int) error {
		if n == 2 { // the read again of the unit
			w.aws.tags[a.ARN] = map[string]string{"env": "test"}
		}
		return nil
	}

	res := w.run(plan(a))

	if r := target(res, "a"); r.Status != journal.StatusSkipped || r.Reason != ReasonChangedSincePlan || len(w.aws.writes) != 0 {
		t.Fatalf("a: %+v, writes %d", r, len(w.aws.writes))
	}
	if e := w.record(res).Entries[0]; len(e.TouchedKeys) == 0 || e.Status != journal.StatusSkipped {
		t.Fatalf("record %+v", e)
	}
}

func TestAFailedFirstReadFailsItsResources(t *testing.T) {
	w := newWorld(t)
	a := w.resource("a", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	w.aws.readErr = func(int) error { return &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetResources} }

	res := w.run(plan(a))

	r := target(res, "a")
	if r.Status != journal.StatusFailed || r.Reason != ReasonReadFailed || r.ErrorKind != "AccessDenied" || res.ExitCode(plan()) != 1 {
		t.Fatalf("a: %+v", r)
	}
	if e := w.record(res).Entries[0]; e.Reason != "readFailed:AccessDenied" || len(e.TouchedKeys) != 0 {
		t.Fatalf("record %+v", e)
	}
	if res.LogFile == "" {
		t.Fatal("a failure writes the log")
	}
	w.aws.readErr = func(int) error { return &errkind.Error{Kind: errkind.CredentialsMissing} }
	if _, err := w.a.Run(context.Background(), Request{Plan: plan(a), AccountID: acct}, nil); errkind.KindOf(err) != errkind.CredentialsMissing {
		t.Fatalf("expired credentials before writing: %v", err)
	}
	w.requireNoLock()
}

// --- the three kinds of failure and the read back (BR4.2, BR4.3) ---

func TestTheKindsOfWriteFailure(t *testing.T) {
	denied := &errkind.Error{Kind: errkind.AccessDenied, Op: "sqs:TagQueue"}
	for _, c := range []struct {
		name   string
		setup  func(w *world, arn string)
		status journal.Status
		reason Reason
		after  bool
	}{
		{"refused first", func(w *world, arn string) {
			w.aws.fail = func(op, a string) *errkind.Error {
				if op == awsaccess.OpTagResources {
					return denied
				}
				return nil
			}
		}, journal.StatusFailed, ReasonWriteFailed, false},
		{"refused second", func(w *world, arn string) {
			w.aws.fail = func(op, a string) *errkind.Error {
				if op == awsaccess.OpUntagResources {
					return denied
				}
				return nil
			}
		}, journal.StatusFailed, ReasonPartialWrite, true},
		{"gone", func(w *world, arn string) {
			w.aws.fail = func(string, string) *errkind.Error { return &errkind.Error{Kind: errkind.NotFound} }
		}, journal.StatusSkipped, ReasonResourceNotFound, false},
		{"transient, nothing written", func(w *world, arn string) {
			w.aws.callErr = func(string, int) error { return &errkind.Error{Kind: errkind.Transient} }
		}, journal.StatusFailed, ReasonWriteFailed, false},
		// The first call timed out after AWS wrote it: the second is never
		// sent, so the tags are half way and the result is unknown.
		{"timeout after the first call wrote", func(w *world, arn string) {
			w.aws.writeOnError = true
			w.aws.callErr = func(string, int) error { return &errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout} }
		}, journal.StatusPending, ReasonResultUnknown, true},
		// A one-call write that timed out after AWS wrote it: the read back
		// shows the planned tags, so it is applied.
		{"timeout after the only call wrote", func(w *world, arn string) {
			w.aws.writeOnError = true
			w.aws.callErr = func(string, int) error { return &errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout} }
		}, journal.StatusApplied, ReasonNone, true},
		{"timeout of the second call only", func(w *world, arn string) {
			w.aws.callErr = func(op string, _ int) error {
				if op == awsaccess.OpUntagResources {
					return &errkind.Error{Kind: errkind.Transient}
				}
				return nil
			}
		}, journal.StatusPending, ReasonResultUnknown, true},
		{"per-resource transient, read back fails", func(w *world, arn string) {
			w.aws.fail = func(string, string) *errkind.Error { return &errkind.Error{Kind: errkind.Transient} }
			w.aws.readErr = func(n int) error {
				if n == 3 {
					return &errkind.Error{Kind: errkind.Transient}
				}
				return nil
			}
		}, journal.StatusPending, ReasonResultUnknown, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			changes, touched := []planner.Change{rename("env", "Environment", false)}, "Environment,env"
			if strings.Contains(c.name, "only call") {
				changes, touched = []planner.Change{add("Owner", "x")}, "Owner"
			}
			a := w.resource("a", map[string]string{"env": "dev"}, changes...)
			c.setup(w, a.ARN)

			res := w.run(plan(a))

			r := target(res, "a")
			if r.Status != c.status || r.Reason != c.reason || (r.AfterTags != nil) != c.after {
				t.Fatalf("a: status %s reason %s after %v", r.Status, r.Reason, r.AfterTags)
			}
			e := w.record(res).Entries[0]
			if e.Status != c.status || strings.Join(e.TouchedKeys, ",") != touched {
				t.Fatalf("record %+v", e)
			}
			if c.status != journal.StatusApplied && res.ExitCode(plan()) == 0 {
				t.Fatal("exit code 0 for a failure or a skip")
			}
			w.requireNoLock()
		})
	}
}

// BR4.3: tags read back that differ from the plan leave the resource
// applied, but not confirmed.
func TestUnconfirmedTagsAreCounted(t *testing.T) {
	w := newWorld(t)
	a := w.resource("a", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	w.aws.readErr = func(n int) error {
		if n == 3 {
			w.aws.tags[a.ARN] = map[string]string{"env": "dev"} // late
		}
		return nil
	}
	res := w.run(plan(a))
	if res.Applied != 1 || res.Unconfirmed != 1 || res.ExitCode(plan()) != 0 || !res.Completed {
		t.Fatalf("applied %d unconfirmed %d", res.Applied, res.Unconfirmed)
	}
}

// --- the counter, the stops and the end (NFR5.17, BR4.5, BR3.8) ---

func manyResources(w *world, n int) *planner.Plan {
	var rs []planner.PlannedResource
	for i := range n {
		rs = append(rs, w.resource(fmt.Sprintf("q%02d", i), map[string]string{"env": fmt.Sprintf("v%02d", i)}, rename("env", "Environment", false)))
	}
	return plan(rs...)
}

func TestNineTransientCallsDoNotStopTenDo(t *testing.T) {
	for _, failures := range []int{9, 10} {
		t.Run(fmt.Sprint(failures), func(t *testing.T) {
			sizes(t, 1, 1, 1, 10)
			w := newWorld(t)
			p := manyResources(w, 12)
			w.aws.callErr = func(op string, n int) error {
				if n <= failures {
					return &errkind.Error{Kind: errkind.Transient}
				}
				return nil
			}

			res := w.run(p)

			if failures == 9 && (res.Stop != StopNone || res.Applied != 3) {
				t.Fatalf("9: stop %q applied %d", res.Stop, res.Applied)
			}
			if failures == 10 && (res.Stop != StopTransient || len(w.aws.writes) != 10 || res.NotWritten != 2 || res.ExitCode(p) != 1) {
				t.Fatalf("10: stop %q, %d writes, not written %d", res.Stop, len(w.aws.writes), res.NotWritten)
			}
			if !res.Completed && failures == 9 {
				t.Fatal("the record must be complete")
			}
		})
	}
	// One call that worked, even with per-resource failures, starts the
	// count again.
	s := newStopper()
	for range 9 {
		s.callEnded(&errkind.Error{Kind: errkind.Transient})
	}
	s.callEnded(nil)
	for range 9 {
		s.callEnded(&errkind.Error{Kind: errkind.Transient})
	}
	s.callEnded(&errkind.Error{Kind: errkind.AccessDenied})
	if s.stopped() {
		t.Fatal("a successful call must reset the count")
	}
	s.callEnded(&errkind.Error{Kind: errkind.CredentialsMissing})
	if r, _ := s.why(); r != StopCredentials {
		t.Fatal("expired credentials stop at once")
	}
}

func TestExpiredCredentialsStopNewWrites(t *testing.T) {
	sizes(t, 1, 1, 1, 10)
	w := newWorld(t)
	p := manyResources(w, 5)
	w.aws.callErr = func(op string, n int) error {
		if n == 3 {
			return &errkind.Error{Kind: errkind.CredentialsMissing}
		}
		return nil
	}
	res := w.run(p)
	if res.Stop != StopCredentials || len(w.aws.writes) != 3 || res.Completed || res.ExitCode(p) != 1 {
		t.Fatalf("stop %q, writes %d, completed %v", res.Stop, len(w.aws.writes), res.Completed)
	}
}

// The first Ctrl+C lets the running call and its batch finish and starts no
// new batch; the record stays open with the rest pending.
func TestTheFirstStopFinishesTheRunningBatch(t *testing.T) {
	sizes(t, 1, 1, 100, 10)
	w := newWorld(t)
	p := manyResources(w, 4)
	stop := make(chan struct{})
	var once sync.Once
	w.aws.onWrite = func(string, []string) {
		once.Do(func() {
			close(stop)
			time.Sleep(50 * time.Millisecond)
		})
	}
	res, err := w.a.Run(context.Background(), Request{Plan: p, AccountID: acct}, stop)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopSignal || res.Applied != 1 || res.NotWritten != 3 || res.Completed || res.SaveErr != nil {
		t.Fatalf("stop %q applied %d not written %d", res.Stop, res.Applied, res.NotWritten)
	}
	rec := w.record(res)
	pending := 0
	for _, e := range rec.Entries {
		if e.Status == journal.StatusPending {
			pending++
		}
	}
	if rec.Completed || pending != 3 {
		t.Fatalf("record completed %v, %d pending", rec.Completed, pending)
	}
	w.requireNoLock()
}

// The second Ctrl+C cancels the running call: its resource stays pending
// with resultUnknown, the record is still saved and the lock removed.
func TestTheSecondStopCancelsTheRunningCall(t *testing.T) {
	sizes(t, 1, 1, 100, 10)
	w := newWorld(t)
	p := manyResources(w, 2)
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan struct{})
	var once sync.Once
	// The call does not end at once when it is cancelled, as a stuck call
	// would not: the applier saves the record without waiting for it.
	w.aws.onWrite = func(string, []string) {
		once.Do(func() {
			close(stop)
			cancel()
			time.Sleep(300 * time.Millisecond)
		})
	}
	res, err := w.a.Run(ctx, Request{Plan: p, AccountID: acct}, stop)
	if err != nil {
		t.Fatal(err)
	}
	unknown := 0
	for _, tg := range res.Failed {
		if tg.Reason == ReasonResultUnknown {
			unknown++
		}
	}
	if unknown != 1 || res.SaveErr != nil || res.ReleaseErr != nil || res.ExitCode(p) != 1 {
		t.Fatalf("unknown %d, save %v, release %v", unknown, res.SaveErr, res.ReleaseErr)
	}
	if rec := w.record(res); rec.Completed {
		t.Fatal("the record must stay open")
	}
	w.requireNoLock()
}

// A panic in a region or in the conductor ends the run as Internal, saves
// the record and removes the lock (security design §3).
func TestPanicsReleaseTheLock(t *testing.T) {
	w := newWorld(t)
	p := manyResources(w, 2)
	w.aws.onWrite = func(string, []string) { panic("boom") }
	res := w.run(p)
	if res.Stop != StopInternal || res.Completed || res.ExitCode(p) != 1 {
		t.Fatalf("region panic: stop %q", res.Stop)
	}
	w.requireNoLock()

	w2 := newWorld(t)
	p2 := manyResources(w2, 1)
	_, err := w2.a.Run(context.Background(), Request{Plan: p2, AccountID: acct, Confirm: func() bool { panic("boom") }}, nil)
	if errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("conductor panic before the record: %v", err)
	}
	w2.requireNoLock()
}

func TestAConfirmationOtherThanYesChangesNothing(t *testing.T) {
	w := newWorld(t)
	p := manyResources(w, 1)
	res, err := w.a.Run(context.Background(), Request{Plan: p, AccountID: acct, Confirm: func() bool { return false }}, nil)
	if err != nil || !res.Cancelled || res.ExitCode(p) != 0 || len(w.aws.writes) != 0 || res.Record != "" {
		t.Fatalf("res %+v err %v", res, err)
	}
	w.requireNoLock()
}

// --force-unlock removes a lock in the way and takes it again; without it
// the lock is a Conflict.
func TestForceUnlock(t *testing.T) {
	w := newWorld(t)
	p := manyResources(w, 1)
	if _, err := w.j.Lock(context.Background(), acct, journal.Location{}, journal.PurposeApply, "left-behind"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.a.Run(context.Background(), Request{Plan: p, AccountID: acct}, nil); errkind.KindOf(err) != errkind.Conflict {
		t.Fatalf("held lock: %v", err)
	}
	var removed []journal.HeldLock
	res, err := w.a.Run(context.Background(), Request{Plan: p, AccountID: acct, ForceUnlock: true,
		OnLocksRemoved: func(l []journal.HeldLock) { removed = l }}, nil)
	if err != nil || len(removed) != 1 || removed[0].Info.RunID != "left-behind" || res.Applied != 1 {
		t.Fatalf("force unlock: %v %+v", err, removed)
	}
	w.requireNoLock()
}

// --- the exit codes (BR6.2) ---

func TestExitCodes(t *testing.T) {
	p := plan()
	incomplete := plan()
	incomplete.InventoryComplete = false
	conflicts := plan()
	conflicts.Conflicts = []planner.KeyConflict{{}}
	for _, c := range []struct {
		res  Result
		p    *planner.Plan
		want int
	}{
		{Result{}, p, 0},
		{Result{Cancelled: true, Failed: []*Target{{}}}, p, 0},
		{Result{Skipped: []*Target{{}}}, p, 2},
		{Result{}, conflicts, 2},
		{Result{Failed: []*Target{{}}, Skipped: []*Target{{}}}, p, 1},
		{Result{NotWritten: 1}, p, 1},
		{Result{Stop: StopSignal}, p, 1},
		{Result{SaveErr: errkind.New(errkind.StorageFailed, "x")}, p, 1},
		{Result{ReleaseErr: errkind.New(errkind.StorageFailed, "x")}, p, 1},
		{Result{}, incomplete, 1},
	} {
		if got := c.res.ExitCode(c.p); got != c.want {
			t.Errorf("%+v: exit %d, want %d", c.res, got, c.want)
		}
	}
	if TransientLimit() != 10 {
		t.Fatal("TransientLimit")
	}
}

// The progress reaches the number of resources to write, and a resource
// that removes first is written untag then tag.
func TestProgressAndOrder(t *testing.T) {
	w := newWorld(t)
	tags := map[string]string{"env": "Prod"}
	for i := 0; len(tags) < 50; i++ {
		tags[fmt.Sprintf("k%02d", i)] = "v"
	}
	a := w.resource("a", tags, rename("env", "Environment", false))
	b := w.resource("b", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	res := w.run(plan(a, b))
	if res.Applied != 2 || w.sink.done[len(w.sink.done)-1] != 2 {
		t.Fatalf("applied %d progress %v", res.Applied, w.sink.done)
	}
	var order []string
	for _, wr := range w.aws.writes {
		if slices.Contains(wr.arns, a.ARN) {
			order = append(order, wr.op)
		}
	}
	if strings.Join(order, ",") != awsaccess.OpUntagResources+","+awsaccess.OpTagResources {
		t.Fatalf("order of a: %v", order)
	}
}
