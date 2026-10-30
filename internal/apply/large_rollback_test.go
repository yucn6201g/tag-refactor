package apply

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The large tests of U7 (NFR5.19, NFR5.20, NFR5.22): the records of the two
// distributions of the measuring conditions (10,000 resources in 17
// regions, 2,000 in one), applied as in the U6 large tests, then rolled back
// through the real SDK against the HTTP-level fake AWS without delays. They
// count the calls of rollback against the formula of NFR5.20, worked out
// from the resources placed, check every call and the parallelism, and time
// the tool's own work. The race build divides the size by largeScale and
// stretches the time limit by largeTimeFactor.

// largeRollback is the world of one large rollback: the account, applied by
// one fake AWS, and rolled back through another, so the calls of rollback
// are counted alone.
type largeRollback struct {
	t         *testing.T
	resources []largeResource
	account   *testutil.TagAccount
	fake      *testutil.FakeAWS
	home      string
	runID     string
	// write is the write rollback makes for each ARN: "set|remove".
	write map[string]string
}

func newLargeRollback(t *testing.T, make func(region string, i int) largeResource) *largeRollback {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	l := &largeRollback{t: t, resources: placeLarge(make), account: testutil.NewTagAccount(), home: t.TempDir(), write: map[string]string{}}
	p := &planner.Plan{PlanID: "plan-large", AccountID: testutil.TestAccount, Regions: largeRegions, InventoryComplete: true, ImpactAnalysisComplete: true}
	for _, r := range l.resources {
		l.account.Add(r.region, r.arn, r.tags)
		p.Resources = append(p.Resources, planner.PlannedResource{ARN: r.arn, Region: r.region, ResourceType: "sqs", BeforeTags: r.tags, Changes: r.changes})
	}
	applyFake := l.newFake()
	res, err := New(Options{AWS: l.aws(applyFake), Journal: journal.New(journal.Options{Home: l.home}), Sink: &sink{}}).
		Run(context.Background(), Request{Plan: p, AccountID: testutil.TestAccount}, nil)
	if err != nil || res.Applied != len(l.resources) {
		t.Fatalf("apply: %v, applied %d", err, res.Applied)
	}
	l.runID = res.RunID
	for _, r := range l.resources {
		after := l.account.Tags(r.arn)
		var set, remove []string
		for _, k := range slices.Sorted(maps.Keys(r.tags)) {
			if after[k] != r.tags[k] {
				set = append(set, k+"="+r.tags[k])
			}
		}
		for _, k := range slices.Sorted(maps.Keys(after)) {
			if _, ok := r.tags[k]; !ok {
				remove = append(remove, k)
			}
		}
		l.write[r.arn] = strings.Join(set, ",") + "|" + strings.Join(remove, ",")
	}
	l.fake = l.newFake()
	l.fake.TrackConcurrency(func(c testutil.Call) []string { return []string{"region:" + c.Region} })
	return l
}

func (l *largeRollback) newFake() *testutil.FakeAWS {
	fake := testutil.NewFakeAWS(l.t)
	fake.HandleDefaults(largeRegions...)
	fake.HandleTagAccount(l.account)
	return fake
}

func (l *largeRollback) aws(fake *testutil.FakeAWS) AWS {
	session, err := awsaccess.NewSession(context.Background(), awsaccess.Options{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }})
	if err != nil {
		l.t.Fatal(err)
	}
	return struct {
		awsaccess.ARNTagReader
		awsaccess.TagWriter
	}{session, awsaccess.NewWriter(session)}
}

// units returns the unit of every ARN: region and index, the targets of a
// region in ARN order cut by 100 (NFR5.19); all resources are written back.
func (l *largeRollback) units() map[string]string {
	byRegion := map[string][]string{}
	for _, r := range l.resources {
		byRegion[r.region] = append(byRegion[r.region], r.arn)
	}
	unitOf := map[string]string{}
	for region, arns := range byRegion {
		sort.Strings(arns)
		for i, arn := range arns {
			unitOf[arn] = fmt.Sprintf("%s#%d", region, i/100)
		}
	}
	return unitOf
}

// bound is the number of calls NFR5.20 allows, from the resources placed:
// per region ⌈N/100⌉ first reads, 2 × R reads again and back, 3 × M reads
// after a late read back, and one call per batch and kind of write, the
// batches being the groups of the same write of a unit split by 20.
func (l *largeRollback) bound(late map[string]int) (reads, tags, untags int) {
	ceil := func(a, b int) int { return (a + b - 1) / b }
	perRegion := map[string]int{}
	groups := map[string]int{}
	for arn, unit := range l.units() {
		perRegion[unit[:strings.Index(unit, "#")]]++
		groups[unit+"|"+l.write[arn]]++
	}
	for region, n := range perRegion {
		reads += ceil(n, 100) + 2*ceil(n, 100) + 3*late[region]
	}
	for key, n := range groups {
		parts := strings.Split(key, "|")
		if parts[1] != "" {
			tags += ceil(n, 20)
		}
		if parts[2] != "" {
			untags += ceil(n, 20)
		}
	}
	return reads, tags, untags
}

// run rolls the record back and checks the result, the calls and the time.
func (l *largeRollback) run(late map[string]int) time.Duration {
	l.t.Helper()
	rb := NewRollbacker(Options{AWS: l.aws(l.fake), Journal: journal.New(journal.Options{Home: l.home}), Sink: &sink{}})
	start := time.Now()
	out, err := rb.Run(context.Background(), RollbackRequest{RunID: l.runID, AccountID: testutil.TestAccount}, nil)
	elapsed := time.Since(start)
	if err != nil {
		l.t.Fatal(err)
	}
	if out.Restored != len(l.resources) || out.Unconfirmed != 0 || !out.Completed || out.ExitCode() != 0 {
		l.t.Fatalf("restored %d of %d, unconfirmed %d, failed %d, skipped %d", out.Restored, len(l.resources), out.Unconfirmed,
			len(out.Failed), len(out.Skipped))
	}
	for _, r := range l.resources {
		if got := l.account.Tags(r.arn); !maps.Equal(got, r.tags) {
			l.t.Fatalf("%s has %v, want %v", r.arn, got, r.tags)
		}
	}
	reads, tags, untags := l.bound(late)
	gotReads, gotTags, gotUntags := l.fake.Count(testutil.OpGetResources), l.fake.Count(testutil.OpTagResources), l.fake.Count(testutil.OpUntagResources)
	l.t.Logf("%d resources rolled back in %v: GetResources %d (bound %d), TagResources %d (bound %d), UntagResources %d (bound %d)",
		len(l.resources), elapsed, gotReads, reads, gotTags, tags, gotUntags, untags)
	if gotReads > reads || gotTags > tags || gotUntags > untags {
		l.t.Errorf("calls over the bound of NFR5.20")
	}
	l.checkCalls()
	return elapsed
}

// checkCalls checks every call (NFR5.19): reads of at most 100 ARNs; writes
// of at most 20 resources that all make the same write and are of one unit;
// at most 4 regions at once and one call at a time in a region.
func (l *largeRollback) checkCalls() {
	l.t.Helper()
	unitOf := l.units()
	for _, c := range l.fake.Calls() {
		raw, _ := c.JSON["ResourceARNList"].([]any)
		var arns []string
		for _, a := range raw {
			s, _ := a.(string)
			arns = append(arns, s)
		}
		switch c.Op {
		case testutil.OpGetResources:
			if len(arns) > 100 {
				l.t.Fatalf("a read of %d ARNs", len(arns))
			}
			continue
		case testutil.OpTagResources, testutil.OpUntagResources:
		default:
			continue
		}
		if len(arns) > 20 {
			l.t.Fatalf("%s of %d resources", c.Op, len(arns))
		}
		for _, arn := range arns {
			if unitOf[arn] != unitOf[arns[0]] {
				l.t.Fatalf("%s mixes the units %s and %s", c.Op, unitOf[arns[0]], unitOf[arn])
			}
			if l.write[arn] != l.write[arns[0]] {
				l.t.Fatalf("%s mixes the writes %q and %q", c.Op, l.write[arns[0]], l.write[arn])
			}
		}
		set, remove, _ := strings.Cut(l.write[arns[0]], "|")
		if c.Op == testutil.OpTagResources {
			m, _ := c.JSON["Tags"].(map[string]any)
			var got []string
			for _, k := range slices.Sorted(maps.Keys(m)) {
				got = append(got, fmt.Sprintf("%s=%v", k, m[k]))
			}
			if strings.Join(got, ",") != set {
				l.t.Fatalf("TagResources sets %v, want %s", got, set)
			}
		} else {
			keys, _ := c.JSON["TagKeys"].([]any)
			var got []string
			for _, k := range keys {
				got = append(got, fmt.Sprint(k))
			}
			slices.Sort(got)
			if strings.Join(got, ",") != remove {
				l.t.Fatalf("UntagResources removes %v, want %s", got, remove)
			}
		}
	}
	if d := l.fake.MaxDistinct("region:"); d > 4 {
		l.t.Errorf("%d regions at once, want at most 4", d)
	}
	for _, region := range largeRegions {
		if n := l.fake.MaxConcurrent("region:" + region); n > 1 {
			l.t.Errorf("%d calls at once in %s", n, region)
		}
	}
}

func runLargeRollback(t *testing.T, make func(region string, i int) largeResource) {
	l := newLargeRollback(t, make)
	if elapsed, limit := l.run(nil), 30*time.Second*largeTimeFactor; elapsed > limit {
		t.Errorf("the rollback took %v, over %v (NFR5.22)", elapsed, limit)
	}
}

func TestLargeRollbackU7CommonChanges(t *testing.T)   { runLargeRollback(t, commonChanges) }
func TestLargeRollbackU7ScatteredValues(t *testing.T) { runLargeRollback(t, scatteredValues) }

// NFR5.20 with M > 0: the read back of the first resource of every unit of
// the skewed region and of one unit of another region is late once, so those
// units are read again; the calls stay within the bound with their M. The
// waits of the reads again are replaced, so the test does not wait.
func TestLargeRollbackU7LateReadBacks(t *testing.T) {
	fakeWaits(t, false, nil)
	l := newLargeRollback(t, commonChanges)
	late := map[string]int{}
	var lagged []string
	seen := map[string]bool{}
	units := l.units()
	for _, arn := range slices.Sorted(maps.Keys(units)) {
		unit := units[arn]
		region := unit[:strings.Index(unit, "#")]
		if seen[unit] || region != skewedRegion && region != "eu-west-1" || region == "eu-west-1" && late[region] > 0 {
			continue
		}
		seen[unit] = true
		late[region]++
		lagged = append(lagged, arn)
	}
	l.account.LagReads(1, lagged...)

	l.run(late)

	if n := l.account.LaggedReads(); n != len(lagged) || late[skewedRegion] != 2000/largeScale/100+min(1, 2000/largeScale%100) {
		t.Fatalf("%d late reads for %d resources, M of the skewed region %d", n, len(lagged), late[skewedRegion])
	}
}
