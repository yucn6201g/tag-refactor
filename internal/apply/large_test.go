package apply

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The large tests of U6 (NFR5.13, NFR5.14, NFR5.16): the two distributions
// of the measuring conditions, 10,000 resources in 17 regions with 2,000 in
// one, applied through the real SDK against the HTTP-level fake AWS without
// delays. They count the calls against the formula of NFR5.14, worked out
// from the resources placed, check the sizes of every call and the
// parallelism, and time the tool's own work. The race build divides the
// size by largeScale and stretches the time limit by largeTimeFactor.

var largeRegions = []string{
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-southeast-1", "ap-southeast-2",
	"ca-central-1", "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3",
	"sa-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2",
}

// skewedRegion holds 2,000 resources; the 16 others 500 each.
const skewedRegion = "us-east-1"

// largeResource is one placed resource: its region, tags and changes.
type largeResource struct {
	region  string
	arn     string
	tags    map[string]string
	changes []planner.Change
	// set and remove describe its write, for the formula.
	set, remove string
}

// commonChanges is the usual distribution: env -> Environment with the
// values dev, stg and Prod (Prod -> prod), and a required CostCenter with
// five values: 15 combinations, so most batches hold 20.
func commonChanges(region string, i int) largeResource {
	values := []string{"dev", "stg", "Prod"}
	v := values[i%3]
	cc := fmt.Sprintf("cc%d", (i/3)%5)
	changes := []planner.Change{rename("env", "Environment", false)}
	after := v
	if v == "Prod" {
		changes = append(changes, value("Environment", "Prod", "prod"))
		after = "prod"
	}
	changes = append(changes, add("CostCenter", cc))
	return largeResource{region: region, tags: map[string]string{"env": v}, changes: changes,
		set: "Environment=" + after + ",CostCenter=" + cc, remove: "env"}
}

// scatteredValues renames Owner to owner with a value of its own on every
// resource: a batch holds about one resource (close to the worst case).
func scatteredValues(region string, i int) largeResource {
	v := fmt.Sprintf("owner-%s-%05d", region, i)
	return largeResource{region: region, tags: map[string]string{"Owner": v}, changes: []planner.Change{rename("Owner", "owner", false)},
		set: "owner=" + v, remove: "Owner"}
}

// placeLarge builds the distribution with a fixed seed.
func placeLarge(make func(region string, i int) largeResource) []largeResource {
	rng := rand.New(rand.NewPCG(6, 6))
	var out []largeResource
	n := 0
	for _, region := range largeRegions {
		count := 500 / largeScale
		if region == skewedRegion {
			count = 2000 / largeScale
		}
		for range count {
			r := make(region, n)
			r.arn = fmt.Sprintf("arn:aws:sqs:%s:%s:q-%05d-%04x", region, acct, n, rng.Uint32()&0xffff)
			out = append(out, r)
			n++
		}
	}
	return out
}

// bound is the number of calls NFR5.14 allows: per region ⌈N/100⌉ first
// reads, 2 × R reads again and back with R ≤ ⌈N/81⌉, and one call per batch
// and kind of write, the batches being the groups of the same write split
// by 20.
func bound(resources []largeResource) (reads, tags, untags int) {
	type key struct{ region, set, remove string }
	byRegion := map[string]int{}
	groups := map[key]int{}
	for _, r := range resources {
		byRegion[r.region]++
		groups[key{r.region, r.set, r.remove}]++
	}
	ceil := func(a, b int) int { return (a + b - 1) / b }
	for _, n := range byRegion {
		reads += ceil(n, 100) + 2*ceil(n, 81)
	}
	for k, n := range groups {
		batches := ceil(n, 20)
		if k.set != "" {
			tags += batches
		}
		if k.remove != "" {
			untags += batches
		}
	}
	return reads, tags, untags
}

func runLarge(t *testing.T, make func(region string, i int) largeResource) {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults(largeRegions...)
	account := testutil.NewTagAccount()
	fake.HandleTagAccount(account)
	fake.TrackConcurrency(func(c testutil.Call) []string { return []string{"region:" + c.Region} })
	resources := placeLarge(make)
	p := &planner.Plan{PlanID: "plan-large", AccountID: testutil.TestAccount, Regions: largeRegions, InventoryComplete: true, ImpactAnalysisComplete: true}
	for _, r := range resources {
		account.Add(r.region, r.arn, r.tags)
		p.Resources = append(p.Resources, planner.PlannedResource{ARN: r.arn, Region: r.region, ResourceType: "sqs", BeforeTags: r.tags, Changes: r.changes})
	}
	session, err := awsaccess.NewSession(context.Background(), awsaccess.Options{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }})
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{AWS: struct {
		awsaccess.ARNTagReader
		awsaccess.TagWriter
	}{session, awsaccess.NewWriter(session)}, Journal: journal.New(journal.Options{Home: t.TempDir()}), Sink: &sink{}})

	start := time.Now()
	res, err := a.Run(context.Background(), Request{Plan: p, AccountID: testutil.TestAccount}, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != len(resources) || !res.Completed || res.ExitCode(p) != 0 {
		t.Fatalf("applied %d of %d, completed %v, failed %d, skipped %d", res.Applied, len(resources), res.Completed, len(res.Failed), len(res.Skipped))
	}
	reads, tags, untags := bound(resources)
	gotReads, gotTags, gotUntags := fake.Count(testutil.OpGetResources), fake.Count(testutil.OpTagResources), fake.Count(testutil.OpUntagResources)
	t.Logf("%d resources in %v: GetResources %d (bound %d), TagResources %d (bound %d), UntagResources %d (bound %d)",
		len(resources), elapsed, gotReads, reads, gotTags, tags, gotUntags, untags)
	if gotReads > reads || gotTags > tags || gotUntags > untags {
		t.Errorf("calls over the bound of NFR5.14")
	}
	for _, c := range fake.Calls() {
		n := len(c.JSON["ResourceARNList"].([]any))
		if (c.Op == testutil.OpGetResources && n > 100) || (c.Op != testutil.OpGetResources && n > 20) {
			t.Fatalf("%s with %d ARNs", c.Op, n)
		}
	}
	if d := fake.MaxDistinct("region:"); d > 4 {
		t.Errorf("%d regions at once, want at most 4", d)
	}
	for _, region := range largeRegions {
		if n := fake.MaxConcurrent("region:" + region); n > 1 {
			t.Errorf("%d calls at once in %s", n, region)
		}
	}
	if limit := 30 * time.Second * largeTimeFactor; elapsed > limit {
		t.Errorf("the run took %v, over %v (NFR5.16)", elapsed, limit)
	}
}

func TestLargeApplyU6CommonChanges(t *testing.T)   { runLarge(t, commonChanges) }
func TestLargeApplyU6ScatteredValues(t *testing.T) { runLarge(t, scatteredValues) }

// The bound of the common distribution matches the estimate of the U6
// measuring conditions for the skewed region (20 first reads, about 50
// reads again and back, about 105 batches of each kind of write).
func TestLargeApplyU6BoundOfTheSkewedRegion(t *testing.T) {
	var skewed []largeResource
	for _, r := range placeLarge(commonChanges) {
		if r.region == skewedRegion {
			skewed = append(skewed, r)
		}
	}
	reads, tags, untags := bound(skewed)
	if largeScale == 1 && (reads != 20+2*25 || tags < 100 || tags > 110 || untags != tags) {
		t.Fatalf("bound of the skewed region: reads %d, tags %d, untags %d", reads, tags, untags)
	}
}
