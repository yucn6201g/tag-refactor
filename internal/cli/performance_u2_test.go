package cli_test

// Performance, determinism and cancellation tests of U2 (NFR1.6, NFR1.7,
// NFR5.7, NFR5.8, NFR10.3), run from the command entry point against the
// HTTP-level fake AWS.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// regionPath is how the large fake account lets a region be searched for
// never-tagged resources.
type regionPath int

const (
	pathNone     regionPath = iota // neither AWS Config nor Resource Explorer
	pathConfig                     // AWS Config records every type
	pathExplorer                   // Resource Explorer only
	pathBoth                       // AWS Config records some types, plus Resource Explorer
)

type regionLoad struct {
	path     regionPath
	tagged   []testutil.TaggedResource
	untagged []string // ARNs of never-tagged queues
}

// largeAccount builds the fake account of NFR1.6: 17 regions, 10,000 tagged
// resources, 1,000 never-tagged ones and 100 attached policies (divided by
// perfScale). The first region is empty; the others take turns among the
// AWS Config, Resource Explorer and both paths.
func largeAccount(t *testing.T, w *world) map[string]*regionLoad {
	t.Helper()
	taggedTotal, untaggedTotal, policies := 10000/perfScale, 1000/perfScale, 100/perfScale
	loads := map[string]*regionLoad{}
	busy := allRegions[1:]
	for i, region := range allRegions {
		loads[region] = &regionLoad{path: pathNone}
		if i > 0 {
			loads[region].path = []regionPath{pathConfig, pathExplorer, pathBoth}[i%3]
		}
	}
	keys := []string{"env", "Env", "Environment", "Owner"}
	for n := range taggedTotal {
		region := busy[n%len(busy)]
		loads[region].tagged = append(loads[region].tagged, testutil.TaggedResource{
			ARN: arnIn(region, fmt.Sprintf("tagged-%05d", n)), Tags: map[string]string{keys[n%len(keys)]: "dev"}})
	}
	for n := range untaggedTotal {
		region := busy[n%len(busy)]
		loads[region].untagged = append(loads[region].untagged, arnIn(region, fmt.Sprintf("never-%05d", n)))
	}

	tagged := map[string][]testutil.TaggedResource{}
	recorders, configItems, explorerItems := map[string]testutil.Handler{}, map[string]testutil.Handler{}, map[string]testutil.Handler{}
	var indexes []testutil.Index
	for region, l := range loads {
		tagged[region] = l.tagged
		var cfg []testutil.ConfigItem
		for _, r := range l.tagged {
			cfg = append(cfg, testutil.ConfigItem{ARN: r.ARN, ResourceType: "AWS::SQS::Queue", Region: region, Tags: r.Tags})
		}
		var re []testutil.ExplorerResource
		for _, arn := range l.untagged {
			cfg = append(cfg, testutil.ConfigItem{ARN: arn, ResourceType: "AWS::SQS::Queue", Region: region})
			re = append(re, testutil.ExplorerResource{ARN: arn, ResourceType: "sqs:queue", Region: region})
		}
		switch l.path {
		case pathConfig:
			recorders[region] = testutil.Static(testutil.ConfigRecorders(testutil.Recorder{AllSupported: true, Recording: true}))
		case pathBoth:
			recorders[region] = testutil.Static(testutil.ConfigRecorders(testutil.Recorder{Strategy: testutil.StrategyInclusion,
				Types: []string{"AWS::SQS::Queue"}, Recording: true}))
		}
		if l.path == pathConfig || l.path == pathBoth {
			configItems[region] = testutil.ConfigPages(cfg...)
		}
		if l.path == pathExplorer || l.path == pathBoth {
			indexes = append(indexes, testutil.Index{Region: region})
			explorerItems[region] = testutil.ExplorerPages(re...)
		}
	}
	w.givenTaggedByRegion(tagged)
	w.fake.Handle(testutil.OpDescribeConfigurationRecorders, testutil.PerRegion(recorders, testutil.Static(testutil.ConfigRecorders())))
	w.fake.Handle(testutil.OpDescribeConfigurationRecorderStatus, testutil.Static(testutil.ConfigRecorderStatus(testutil.Recorder{Recording: true})))
	w.fake.Handle(testutil.OpSelectResourceConfig, testutil.PerRegion(configItems, testutil.ConfigPages()))
	w.fake.Handle(testutil.OpListIndexes, testutil.Static(testutil.ExplorerIndexes(indexes...)))
	w.fake.HandleExplorerViews()
	w.fake.Handle(testutil.OpListResources, testutil.PerRegion(explorerItems, testutil.ExplorerPages()))
	docs := map[string]string{}
	for i := range policies {
		docs[fmt.Sprintf("arn:aws:iam::123456789012:policy/P%03d", i)] = policyEnvCondition
	}
	w.givenAttachedPolicies(docs)
	return loads
}

func pages(n, size int) int { return max(1, (n+size-1)/size) }

// callBound is the upper bound of performance design §3 for scan (with the
// discovery) or plan (with the IAM reads). The two calls that identify the
// account (sts:GetCallerIdentity, ec2:DescribeRegions) come on top. Since U5
// plan reads every IAM policy with iam:GetAccountAuthorizationDetails, one
// page per 1,000 users, roles, groups and customer managed policies; this
// account has only the customer managed policies and no resources with
// policies, so the U5 call formula (U5 logical-components.md) also adds one
// page of ListBuckets and of the four lists of every region.
func callBound(loads map[string]*regionLoad, scan bool, policies int) int {
	bound := 2
	for _, l := range loads {
		bound += pages(len(l.tagged), 100)
		if !scan {
			continue
		}
		bound += 4 // the recorders, their status, the default view and the view
		configRecords := len(l.tagged) + len(l.untagged)
		switch l.path {
		case pathConfig:
			bound += pages(configRecords, 100)
		case pathExplorer:
			bound += pages(len(l.untagged), 500)
		case pathBoth:
			bound += pages(configRecords, 100) + pages(len(l.untagged), 500)
		}
	}
	if scan {
		return bound + 1 // ListIndexes, one page for up to 100 indexes
	}
	return bound + pages(policies, 1000) + pages(0, 10000) + 4*len(loads)
}

// NFR1.6, NFR1.7: with 10,000 tagged and 1,000 never-tagged resources in 17
// regions and 100 policies, scan and plan stay within the call bound and
// finish within 30 seconds of the tool's own work (the fake has no delay).
func TestPerformanceU2CallsAndTimeFor10000Resources(t *testing.T) {
	w := newAccountWorld(t)
	loads := largeAccount(t, w)
	rules := w.givenRuleFile(requiredOwnerRules)

	start := time.Now()
	scan := w.run("scan", "--rules", rules)
	scanCalls := len(w.fake.Calls())
	plan := w.run("plan", "--rules", rules)
	planCalls := len(w.fake.Calls()) - scanCalls
	elapsed := time.Since(start)

	scan.requireExit(t, 2)
	plan.requireExit(t, 2)
	tagged, untagged := 10000/perfScale, 1000/perfScale
	// U4 BR3.6: counts have a comma every three digits.
	requireContains(t, "scan stdout", scan.stdout, fmt.Sprintf("Summary: %s resources scanned in 17 of 17 regions", display.FormatCount(tagged+untagged)))
	requireMatch(t, "scan stdout", scan.stdout, fmt.Sprintf(`(?m)^Resources with no user tags: %s$`, display.FormatCount(untagged)))
	requireMatch(t, "scan stdout", scan.stdout, `(?m)^Untagged-resource discovery: partial \(not checked in ap-northeast-1: `)
	if bound := callBound(loads, true, 0); scanCalls > bound {
		t.Errorf("scan made %d calls, bound %d", scanCalls, bound)
	}
	if bound := callBound(loads, false, 100/perfScale); planCalls > bound {
		t.Errorf("plan made %d calls, bound %d", planCalls, bound)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("scan and plan took %v (limit 30s)", elapsed)
	}
	t.Logf("scale 1/%d: scan %d calls, plan %d calls, %v", perfScale, scanCalls, planCalls, elapsed)
}

// NFR5.7: whatever order the regions finish in, standard output is the same.
// Each seed gives every region another delay.
func TestDeterministicOutputWhateverOrderRegionsFinish(t *testing.T) {
	global := testutil.TaggedResource{ARN: "arn:aws:s3:::bucket-shared", Tags: map[string]string{"env": "dev"}}
	byRegion := map[string][]testutil.TaggedResource{}
	var explorer []testutil.ExplorerResource
	var config []testutil.ConfigItem
	for i, region := range allRegions {
		byRegion[region] = []testutil.TaggedResource{global,
			{ARN: arnIn(region, "q"), Tags: map[string]string{[]string{"env", "Env", "environment", "prod"}[i%4]: "x"}}}
		never := arnIn(region, "never")
		explorer = append(explorer, testutil.ExplorerResource{ARN: never, ResourceType: "sqs:queue", Region: region})
		config = append(config, testutil.ConfigItem{ARN: never, ResourceType: "AWS::SQS::Queue", Region: region})
	}
	var first string
	for seed := int64(1); seed <= 10; seed++ {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(byRegion)
		w.fake.HandleConfigRecorders(testutil.Recorder{Strategy: testutil.StrategyInclusion, Types: []string{"AWS::SQS::Queue"}, Recording: true})
		w.fake.Handle(testutil.OpSelectResourceConfig, func(c testutil.Call) testutil.Reply {
			var mine []testutil.ConfigItem
			for _, it := range config {
				if it.Region == c.Region {
					mine = append(mine, it)
				}
			}
			return testutil.ConfigPages(mine...)(c)
		})
		w.givenExplorerIn(allRegions, explorer...)
		w.fake.SetDelay(testutil.RegionDelays(seed, 3*time.Millisecond))

		r := w.run("scan")

		r.requireExit(t, 2)
		if seed == 1 {
			first = r.stdout
			requireContains(t, "stdout", r.stdout, "Summary: 35 resources scanned in 17 of 17 regions")
			continue
		}
		if r.stdout != first {
			t.Fatalf("seed %d changed standard output:\n%s\nfirst:\n%s", seed, r.stdout, first)
		}
	}
}

// NFR5.8, NFR8.11: every expired-credential name, in each of the three
// stages, stops the run with nothing on standard output, and no request is
// left running or started afterwards.
func TestCancelOnExpiredCredentialsInEveryStage(t *testing.T) {
	everywhere := map[string][]testutil.TaggedResource{}
	for _, region := range allRegions {
		everywhere[region] = []testutil.TaggedResource{{ARN: arnIn(region, "q"), Tags: map[string]string{"env": "dev"}}}
	}
	for _, code := range []string{"ExpiredToken", "ExpiredTokenException", "RequestExpired", "TokenRefreshRequired"} {
		stages := map[string]struct {
			given func(w *world)
			args  func(rules string) []string
		}{
			"tagging API": {
				given: func(w *world) {
					w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
						"eu-north-1": testutil.Static(testutil.JSONError(http.StatusBadRequest, code, "expired")),
					}, testutil.TagPagesByRegion(everywhere)))
				},
				args: func(string) []string { return []string{"scan"} },
			},
			"discovery": {
				given: func(w *world) {
					w.givenTaggedByRegion(everywhere)
					w.fake.HandleConfigRecorders(testutil.Recorder{AllSupported: true, Recording: true})
					w.fake.Handle(testutil.OpSelectResourceConfig, testutil.PerRegion(map[string]testutil.Handler{
						"ca-central-1": testutil.Static(testutil.JSONError(http.StatusBadRequest, code, "expired")),
					}, testutil.ConfigPages()))
				},
				args: func(string) []string { return []string{"scan"} },
			},
			"plan IAM reads": {
				given: func(w *world) {
					w.givenTaggedByRegion(everywhere)
					w.fake.Handle("iam:GetAccountAuthorizationDetails", testutil.Static(testutil.QueryError(http.StatusForbidden, code, "expired")))
				},
				args: func(rules string) []string { return []string{"plan", "--rules", rules} },
			},
		}
		for stage, sc := range stages {
			t.Run(code+" in "+stage, func(t *testing.T) {
				w := newAccountWorld(t)
				sc.given(w)
				w.fake.SetDelay(testutil.RegionDelays(7, 2*time.Millisecond))
				rules := w.givenRuleFile(aliasRules)

				r := w.run(sc.args(rules)...)

				r.requireExit(t, 1)
				if r.stdout != "" {
					t.Fatalf("stdout must be empty:\n%s", r.stdout)
				}
				requireMatch(t, "stderr", r.stderr, `(?m)^Error: the AWS credentials have expired`)
				if n := w.fake.InFlight(); n != 0 {
					t.Fatalf("%d requests still running after the command returned", n)
				}
				before := len(w.fake.Calls())
				time.Sleep(200 * time.Millisecond)
				if after := len(w.fake.Calls()); after != before {
					t.Fatalf("%d requests arrived after the command returned", after-before)
				}
			})
		}
	}
}
