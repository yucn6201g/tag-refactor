package impact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The large tests of U5 (NFR1.16 to NFR1.18, NFR5.10, NFR5.11) run the
// analysis through the real SDK against the HTTP-level fake AWS, with the
// measuring conditions of logical-components.md: 17 regions, one of them
// skewed, 1,000 users, roles and groups with an inline policy each, 600
// customer managed policies (500 used), 2,000 resources with resource
// policies and 5,000 changes. Under the race detector the account is ten
// times smaller and the time limit ten times longer.

var largeRegions = []string{
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-southeast-1", "ap-southeast-2",
	"ca-central-1", "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3",
	"sa-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2",
}

const (
	// skewedRegion holds as many resource policies as all the others.
	skewedRegion = "ap-northeast-1"
	// emptyRegion holds no resources but two AWS managed keys.
	emptyRegion = "us-west-2"
)

// largeTemplates are the statements the policies take turns with: every
// kind of reference, many wildcards, and operators of every kind.
var largeTemplates = []string{
	condStatement("StringEquals", "aws:ResourceTag/env", `"dev"`),
	condStatement("StringLike", "aws:ResourceTag/en*", `"d*"`),
	condStatement("ForAnyValue:StringLike", "aws:TagKeys", `["t*","own?r"]`),
	condStatement("StringEquals", "aws:PrincipalTag/team", `"a"`),
	condStatement("StringEquals", "aws:ResourceTag/${aws:PrincipalTag/k}", `"x"`),
	`{"Sid":"R","Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::d/${aws:PrincipalTag/team}/*"}`,
	condStatement("Null", "aws:RequestTag/Owner", `"true"`),
	condStatement("ArnLike", "aws:ResourceTag/tmp", `"arn:*"`),
	condStatement("StringEqualsIfExists", "ec2:ResourceTag/cost-cent?r", `"x*"`),
}

func largeDoc(i int) string {
	return doc(largeTemplates[i%len(largeTemplates)], largeTemplates[(i+4)%len(largeTemplates)])
}

// largeAccount builds the measuring conditions divided by scale, and the
// changes: four per resource for the first resources with policies and for
// IAM roles.
func largeAccount(scale int) (*testutil.PolicyAccount, []planner.TagChangeInput) {
	n := func(v int) int { return max(1, v/scale) }
	a := &testutil.PolicyAccount{}
	for i := 0; i < n(600); i++ {
		m := testutil.ManagedPolicyDoc{Name: fmt.Sprintf("p%04d", i), Document: largeDoc(i)}
		switch {
		case i >= n(500):
			m.NotAttached = true
		case i >= n(450):
			m.NotAttached, m.Boundaries = true, 1
		}
		a.Managed = append(a.Managed, m)
	}
	principals := func(prefix string, count int) []testutil.Principal {
		out := make([]testutil.Principal, count)
		for i := range out {
			out[i] = testutil.Principal{Name: fmt.Sprintf("%s%04d", prefix, i), Inline: map[string]string{"inline": largeDoc(i + 3)}}
		}
		return out
	}
	a.Users, a.Roles, a.Groups = principals("u", n(300)), principals("r", n(400)), principals("g", n(300))

	others := make([]string, 0, len(largeRegions))
	for _, r := range largeRegions {
		if r != skewedRegion && r != emptyRegion {
			others = append(others, r)
		}
	}
	// Half of each kind in the skewed region, the other half spread over
	// the other regions but the empty one.
	place := func(total int, put func(region string, i int)) {
		for i := 0; i < n(total); i++ {
			region := skewedRegion
			if i >= n(total)/2 {
				region = others[i%len(others)]
			}
			put(region, i)
		}
	}
	place(200, func(region string, i int) {
		a.Buckets = append(a.Buckets, testutil.BucketPolicy{Name: fmt.Sprintf("b%04d", i), Region: region, Policy: largeDoc(i)})
	})
	place(300, func(region string, i int) {
		a.Keys = append(a.Keys, testutil.KeyPolicy{Region: region, ID: fmt.Sprintf("k%04d", i), Policy: largeDoc(i + 1)})
	})
	for i, region := range largeRegions {
		count := 2
		if region == skewedRegion {
			count = 4
		}
		for j := 0; j < count; j++ {
			a.Keys = append(a.Keys, testutil.KeyPolicy{Region: region, ID: fmt.Sprintf("aws%02d%d", i, j), AWSManaged: true})
		}
	}
	place(500, func(region string, i int) {
		a.Queues = append(a.Queues, testutil.RegionalPolicy{Region: region, Name: fmt.Sprintf("q%04d", i), Policy: largeDoc(i + 2)})
	})
	place(500, func(region string, i int) {
		a.Topics = append(a.Topics, testutil.RegionalPolicy{Region: region, Name: fmt.Sprintf("t%04d", i), Policy: largeDoc(i + 5)})
	})
	place(500, func(region string, i int) {
		a.Secrets = append(a.Secrets, testutil.RegionalPolicy{Region: region, Name: fmt.Sprintf("s%04d", i), Policy: largeDoc(i + 7)})
	})

	var arns []string
	for i := range a.Queues {
		arns = append(arns, testutil.QueueARN(a.Queues[i].Region, a.Queues[i].Name), testutil.TopicARN(a.Topics[i].Region, a.Topics[i].Name))
	}
	for _, b := range a.Buckets {
		arns = append(arns, testutil.BucketARN(b.Name))
	}
	arns = arns[:min(len(arns), n(1000))]
	for _, r := range a.Roles[:n(250)] {
		arns = append(arns, testutil.RoleARN(r.Name))
	}
	var changes []planner.TagChangeInput
	add := func(c planner.TagChangeInput) {
		c.ChangeID = fmt.Sprintf("c-%06d", len(changes)+1)
		changes = append(changes, c)
	}
	for _, arn := range arns {
		add(planner.TagChangeInput{ResourceARN: arn, Kind: planner.KindRenameKey, FromKey: "env", ToKey: "Environment", FromValue: "dev", ToValue: "dev"})
		add(planner.TagChangeInput{ResourceARN: arn, Kind: planner.KindChangeValue, FromKey: "team", ToKey: "team", FromValue: "a", ToValue: "b"})
		add(planner.TagChangeInput{ResourceARN: arn, Kind: planner.KindAddRequired, ToKey: "Owner", ToValue: "x"})
		add(planner.TagChangeInput{ResourceARN: arn, Kind: planner.KindRemoveKey, FromKey: "tmp", FromValue: "1"})
	}
	return a, changes
}

func ceilDiv(n, size int) int { return max(1, (n+size-1)/size) }

// callBounds is the call formula of logical-components.md per operation,
// worked out from what the account holds (no number written by hand).
func callBounds(a *testutil.PolicyAccount, regions []string) map[string]int {
	b := map[string]int{}
	used := 0
	for _, m := range a.Managed {
		if !m.NotAttached || m.Boundaries > 0 {
			used++
		}
	}
	b[testutil.OpListPolicies] = ceilDiv(len(a.Managed), 1000)
	b[testutil.OpGetPolicyVersion] = used
	for op, list := range map[string][]testutil.Principal{"User": a.Users, "Role": a.Roles, "Group": a.Groups} {
		b["iam:List"+op+"s"] = ceilDiv(len(list), 1000)
		for _, p := range list {
			b["iam:List"+op+"Policies"] += ceilDiv(len(p.Inline), 1000)
			b["iam:Get"+op+"Policy"] += len(p.Inline)
		}
	}
	b[testutil.OpListBuckets] = ceilDiv(len(a.Buckets), 10000)
	inPlan := map[string]bool{}
	for _, r := range regions {
		inPlan[r] = true
	}
	for _, bucket := range a.Buckets {
		if bucket.NoRegionInList {
			b[testutil.OpGetBucketLocation]++
		}
		if inPlan[bucket.Region] {
			b[testutil.OpGetBucketPolicy]++
		}
	}
	for _, r := range regions {
		var k, kc, q, t, s int
		for _, key := range a.Keys {
			if key.Region == r {
				k++
				if !key.AWSManaged {
					kc++
				}
			}
		}
		count := func(items []testutil.RegionalPolicy) int {
			c := 0
			for _, it := range items {
				if it.Region == r {
					c++
				}
			}
			return c
		}
		q, t, s = count(a.Queues), count(a.Topics), count(a.Secrets)
		b[testutil.OpListKeys] += ceilDiv(k, 1000)
		b[testutil.OpListQueues] += ceilDiv(q, 1000)
		b[testutil.OpListTopics] += ceilDiv(t, 100)
		b[testutil.OpListSecrets] += ceilDiv(s, 100)
		b[testutil.OpDescribeKey] += k
		b[testutil.OpGetKeyPolicy] += kc
		b[testutil.OpGetQueueAttributes] += q
		b[testutil.OpGetTopicAttributes] += t
		b[testutil.OpGetResourcePolicy] += s
	}
	return b
}

// largeWorld is the fake AWS of an account and a session that talks to it.
func largeWorld(t testing.TB, a *testutil.PolicyAccount) (*testutil.FakeAWS, *awsaccess.Session) {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandlePolicyAccount(a)
	s, err := awsaccess.NewSession(context.Background(), awsaccess.Options{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }})
	if err != nil {
		t.Fatal(err)
	}
	return fake, s
}

// concurrencyKeys sorts the calls into the stages of the parallel reads:
// IAM, the bucket list and locations, and the jobs of each region.
func concurrencyKeys(c testutil.Call) []string {
	switch {
	case c.Service == "iam":
		return []string{"iam"}
	case c.Op == testutil.OpListBuckets || c.Op == testutil.OpGetBucketLocation:
		return []string{"s3list"}
	}
	return []string{"region:" + c.Region}
}

// NFR1.16, NFR1.17: within the call formula, operation by operation, and
// within ten seconds of the tool's own work (the fake has no delay).
func TestLargeAnalysisU5CallsAndTime(t *testing.T) {
	a, changes := largeAccount(perfScale)
	fake, s := largeWorld(t, a)

	start := time.Now()
	res, err := New(s, largeRegions).Analyze(context.Background(), changes)
	elapsed := time.Since(start)

	if err != nil || !res.Complete {
		t.Fatalf("err %v, unread %+v", err, res.UnreadPolicies)
	}
	bounds := callBounds(a, largeRegions)
	total, bound := 0, 0
	for op, want := range bounds {
		bound += want
		if got := fake.Count(op); got > want {
			t.Errorf("%s: %d calls, bound %d", op, got, want)
		}
	}
	for _, op := range fake.Ops() {
		total += fake.Count(op)
		if _, ok := bounds[op]; !ok {
			t.Errorf("%s is outside the formula", op)
		}
	}
	if total > bound {
		t.Errorf("%d calls, bound %d", total, bound)
	}
	if fake.Count(testutil.OpListTopics) < 3 {
		t.Errorf("the skewed region's topics must take three pages")
	}
	if limit := time.Duration(timeLimitFactor) * 10 * time.Second; elapsed > limit {
		t.Errorf("the analysis took %v (limit %v)", elapsed, limit)
	}
	affected, undecided := 0, 0
	for _, f := range res.Findings {
		if f.Verdict == planner.VerdictAffected {
			affected++
		} else {
			undecided++
		}
	}
	if affected == 0 || undecided == 0 {
		t.Errorf("the measuring conditions must give both verdicts: %d affected, %d undecided", affected, undecided)
	}
	t.Logf("scale 1/%d: %d changes, %d calls (bound %d), %v, %d affected and %d undecided findings",
		perfScale, len(changes), total, bound, elapsed, affected, undecided)
}

// NFR1.18: at most 4 IAM calls, 4 calls of one region and 8 regions at
// once. A short delay lets the reads overlap.
func TestLargeAnalysisU5StaysWithinItsConcurrency(t *testing.T) {
	a, changes := largeAccount(perfScale * 4)
	fake, s := largeWorld(t, a)
	fake.SetDelay(func(testutil.Call) time.Duration { return time.Millisecond })
	fake.TrackConcurrency(concurrencyKeys)

	if _, err := New(s, largeRegions).Analyze(context.Background(), changes); err != nil {
		t.Fatal(err)
	}
	if got := fake.MaxConcurrent("iam"); got > 4 || got < 2 {
		t.Errorf("%d IAM calls at once, want 2 to 4", got)
	}
	if got := fake.MaxConcurrent("s3list"); got > 4 {
		t.Errorf("%d bucket location calls at once, limit 4", got)
	}
	for _, r := range largeRegions {
		if got := fake.MaxConcurrent("region:" + r); got > 4 {
			t.Errorf("%d calls at once in %s, limit 4", got, r)
		}
	}
	if got := fake.MaxDistinct("region:"); got > 8 || got < 2 {
		t.Errorf("%d regions at once, want 2 to 8", got)
	}
	t.Logf("at once: IAM %d, skewed region %d, regions %d", fake.MaxConcurrent("iam"), fake.MaxConcurrent("region:"+skewedRegion), fake.MaxDistinct("region:"))
}

// setWorkers replaces the limits for one test.
func setWorkers(t *testing.T, n int) {
	t.Helper()
	saved := [4]int{iamWorkers, locationWorkers, regionWorkers, regionJobWorkers}
	iamWorkers, locationWorkers, regionWorkers, regionJobWorkers = n, n, n, n
	t.Cleanup(func() {
		iamWorkers, locationWorkers, regionWorkers, regionJobWorkers = saved[0], saved[1], saved[2], saved[3]
	})
}

// NFR1.18: with every limit at 1 the analysis still finishes (no job waits
// for a slot its own dispatcher holds), one call at a time.
func TestLargeAnalysisU5FinishesWithOneSlotEach(t *testing.T) {
	setWorkers(t, 1)
	a, changes := largeAccount(perfScale * 10)
	for i := range a.Buckets {
		a.Buckets[i].NoRegionInList = i%3 == 0
	}
	fake, s := largeWorld(t, a)
	fake.TrackConcurrency(concurrencyKeys)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeLimitFactor)*30*time.Second)
	defer cancel()
	res, err := New(s, largeRegions).Analyze(ctx, changes)
	if err != nil || !res.Complete {
		t.Fatalf("err %v, unread %+v", err, res.UnreadPolicies)
	}
	if fake.MaxConcurrent("iam") != 1 || fake.MaxDistinct("region:") != 1 || fake.MaxConcurrent("s3list") != 1 {
		t.Fatalf("at once: IAM %d, regions %d, locations %d", fake.MaxConcurrent("iam"), fake.MaxDistinct("region:"), fake.MaxConcurrent("s3list"))
	}
	if fake.Count(testutil.OpGetBucketLocation) == 0 {
		t.Fatal("the buckets without a region were not located")
	}
}

// render writes the result the way plan shows and saves it.
func renderResult(t *testing.T, res planner.ImpactResult) string {
	t.Helper()
	plan := &planner.Plan{AccountID: "123456789012", InventoryComplete: true}
	plan.AttachImpact(res)
	var saved bytes.Buffer
	if err := planner.WritePlan(&saved, plan); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	p := display.New(&out, &errOut)
	p.PlanResult(plan, display.Output{Format: display.FormatTable})
	p.PlanResult(plan, display.Output{Format: display.FormatJSON})
	return saved.String() + out.String()
}

// NFR5.11: whatever order the kinds of policies and the regions finish in,
// the findings, the unread policies, the output and the saved plan are the
// same. Each seed gives each service and region another delay; the
// wildcards share the pattern cache.
func TestLargeAnalysisU5Deterministic(t *testing.T) {
	a, changes := largeAccount(perfScale * 10)
	a.Fail = map[string]testutil.Reply{
		testutil.OpListQueues + " eu-west-1":                                            testutil.SQSError(400, "AccessDenied", "AccessDenied", "denied"),
		testutil.OpGetKeyPolicy + " " + testutil.KeyARN(a.Keys[0].Region, a.Keys[0].ID): testutil.JSONError(400, "AccessDeniedException", "denied"),
	}
	var first planner.ImpactResult
	var firstText string
	for seed := int64(1); seed <= 10; seed++ {
		fake, s := largeWorld(t, a)
		fake.SetDelay(testutil.ServiceRegionDelays(seed, time.Millisecond))
		res, err := New(s, largeRegions).Analyze(context.Background(), changes)
		if err != nil {
			t.Fatal(err)
		}
		text := renderResult(t, res)
		if seed == 1 {
			first, firstText = res, text
			if res.Complete || len(res.UnreadPolicies) != 2 || len(res.Findings) == 0 {
				t.Fatalf("result = %d findings, unread %+v", len(res.Findings), res.UnreadPolicies)
			}
			continue
		}
		if !reflect.DeepEqual(res, first) || text != firstText {
			t.Fatalf("seed %d gives another result", seed)
		}
	}
}

// NFR5.10, BR5.4: a cancelled analysis returns the cancellation and no
// partial result, and leaves no goroutine or request behind.
func TestLargeAnalysisU5CancelledLeavesNothingRunning(t *testing.T) {
	a, changes := largeAccount(perfScale * 10)
	fake, s := largeWorld(t, a)
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	fake.SetDelay(func(c testutil.Call) time.Duration {
		if c.Op != testutil.OpGetUserPolicy {
			return 0
		}
		if c.N == 3 {
			cancel()
		}
		return time.Minute // a slow answer, stopped by the cancellation
	})

	res, err := New(s, largeRegions).Analyze(ctx, changes)

	if !errors.Is(err, context.Canceled) || res.Findings != nil || res.UnreadPolicies != nil {
		t.Fatalf("err %v, result %+v", err, res)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		var b strings.Builder
		_ = pprofGoroutines(&b)
		t.Fatalf("%d goroutines before, %d after:\n%s", before, after, b.String())
	}
	if fake.InFlight() != 0 {
		t.Fatalf("%d requests still running", fake.InFlight())
	}
}

// pprofGoroutines writes the stacks of the goroutines, to show a leak.
func pprofGoroutines(b *strings.Builder) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	b.Write(buf[:n])
	return nil
}
