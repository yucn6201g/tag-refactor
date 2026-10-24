package cli_test

// Acceptance tests for U2 (scan of every region and discovery of never-tagged
// resources). Like the U1 tests, each one is an acceptance criterion written
// as Given/When/Then and run from the command entry point (cli.Run) against
// the HTTP-level fake AWS, with the AWS environment isolated.
//
// These tests were written and committed before the U2 implementation (team
// testing posture: acceptance criteria first).

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// allRegions are the 17 regions enabled in the U2 test account.
var allRegions = []string{
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-southeast-1", "ap-southeast-2",
	"ca-central-1", "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3",
	"sa-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2",
}

// apiOperations are the AWS API operations scan and plan may call (security
// design §1). The API Search of Resource Explorer is deliberately missing.
var apiOperations = []string{
	"sts:GetCallerIdentity", "ec2:DescribeRegions", "tag:GetResources", "iam:ListPolicies", "iam:GetPolicyVersion",
	"config:DescribeConfigurationRecorders", "config:DescribeConfigurationRecorderStatus", "config:SelectResourceConfig",
	"resource-explorer-2:ListIndexes", "resource-explorer-2:GetDefaultView", "resource-explorer-2:GetView",
	"resource-explorer-2:ListResources",
}

const requiredOwnerRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\nrequired:\n  Owner:\n    default: platform\n"

func arnIn(region, name string) string {
	return "arn:aws:sqs:" + region + ":123456789012:" + name
}

// newAccountWorld is an account with the 17 regions enabled, the credentials
// of the default profile in us-east-1, no tagged resources, and neither AWS
// Config nor Resource Explorer (discovery unavailable).
func newAccountWorld(t *testing.T) *world {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults(allRegions...)
	fake.Handle("tag:GetResources", testutil.TagPagesByRegion(nil))
	return &world{t: t, env: env, fake: fake}
}

func (w *world) givenTaggedByRegion(byRegion map[string][]testutil.TaggedResource) {
	w.fake.Handle("tag:GetResources", testutil.TagPagesByRegion(byRegion))
}

// givenConfigEverywhere makes AWS Config record every supported type in
// every region and answer the query with items (for the region they name).
func (w *world) givenConfigEverywhere(items ...testutil.ConfigItem) {
	w.fake.HandleConfigRecorders(testutil.Recorder{AllSupported: true, Strategy: testutil.StrategyAll, Recording: true})
	w.fake.Handle(testutil.OpSelectResourceConfig, func(c testutil.Call) testutil.Reply {
		var inRegion []testutil.ConfigItem
		for _, it := range items {
			if it.Region == c.Region {
				inRegion = append(inRegion, it)
			}
		}
		return testutil.ConfigPages(inRegion...)(c)
	})
}

// givenExplorerIn gives the regions a local Resource Explorer index and an
// unfiltered default view; ListResources answers with the resources of the
// region asked.
func (w *world) givenExplorerIn(regions []string, resources ...testutil.ExplorerResource) {
	var indexes []testutil.Index
	for _, r := range regions {
		indexes = append(indexes, testutil.Index{Region: r})
	}
	w.fake.Handle(testutil.OpListIndexes, testutil.Static(testutil.ExplorerIndexes(indexes...)))
	w.fake.HandleExplorerViews()
	w.fake.Handle(testutil.OpListResources, func(c testutil.Call) testutil.Reply {
		var inRegion []testutil.ExplorerResource
		for _, r := range resources {
			if r.Region == c.Region {
				inRegion = append(inRegion, r)
			}
		}
		return testutil.ExplorerPages(inRegion...)(c)
	})
}

func regionsCalled(f *testutil.FakeAWS, op string) []string {
	seen := map[string]bool{}
	for _, c := range f.Calls() {
		if c.Op == op {
			seen[c.Region] = true
		}
	}
	var regions []string
	for r := range seen {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	return regions
}

// AC1.1.1: Given 17 enabled regions, When scan runs, Then all 17 regions are
// read and the number of regions read is shown. --region narrows the regions
// (repeated values count once), and a region that is not enabled is an error
// before any resource is read.
func TestAcceptanceU2_AC1_1_1_ScanReadsEveryEnabledRegion(t *testing.T) {
	t.Run("all regions by default", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(map[string][]testutil.TaggedResource{
			"us-east-1":      {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "x"}}},
			"eu-west-1":      {{ARN: arnIn("eu-west-1", "b"), Tags: map[string]string{"Owner": "x"}}},
			"ap-northeast-1": {{ARN: arnIn("ap-northeast-1", "c"), Tags: map[string]string{"Owner": "x"}}},
		})

		r := w.run("scan")

		r.requireExit(t, 0)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 3 resources scanned in 17 of 17 regions · 0 variant groups$`)
		requireContains(t, "stderr", r.stderr, "Checked: tagged resources in 17 of 17 regions (tagging API)")
		if got := regionsCalled(w.fake, "tag:GetResources"); !slices.Equal(got, allRegions) {
			t.Fatalf("tag:GetResources was called in %v, want all 17 regions", got)
		}
	})
	t.Run("regions narrowed with --region", func(t *testing.T) {
		w := newAccountWorld(t)

		r := w.run("scan", "--region", "eu-west-1", "--region", "us-east-1", "--region", "eu-west-1")

		r.requireExit(t, 0)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 0 resources scanned in 2 of 17 regions · 0 variant groups$`)
		if got := regionsCalled(w.fake, "tag:GetResources"); !slices.Equal(got, []string{"eu-west-1", "us-east-1"}) {
			t.Fatalf("tag:GetResources was called in %v", got)
		}
	})
	t.Run("a region that is not enabled", func(t *testing.T) {
		w := newAccountWorld(t)

		r := w.run("scan", "--region", "us-east-1", "--region", "me-central-1")

		r.requireExit(t, 1)
		requireTwoLineError(t, r)
		requireContains(t, "stderr", r.stderr, "Error: the region me-central-1 is not available for this account")
		if n := w.fake.Count("tag:GetResources"); n != 0 {
			t.Fatalf("resources were read (%d calls) before the region was checked", n)
		}
	})
}

// AC1.1.2: Given never-tagged resources and AWS Config or Resource Explorer
// enabled, When scan runs, Then those resources are part of the result.
func TestAcceptanceU2_AC1_1_2_NeverTaggedResourcesAreFound(t *testing.T) {
	tagged := testutil.TaggedResource{ARN: arnIn("us-east-1", "tagged"), Tags: map[string]string{"Owner": "x"}}
	t.Run("through AWS Config", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {tagged}})
		w.givenConfigEverywhere(
			testutil.ConfigItem{ARN: arnIn("us-east-1", "never"), ResourceType: "AWS::SQS::Queue", ResourceID: "never", Region: "us-east-1"},
			testutil.ConfigItem{ARN: arnIn("us-east-1", "only-aws"), ResourceType: "AWS::SQS::Queue", ResourceID: "only-aws", Region: "us-east-1",
				Tags: map[string]string{"aws:cloudformation:stack-name": "s"}},
			testutil.ConfigItem{ARN: arnIn("us-east-1", "user-tag"), ResourceType: "AWS::SQS::Queue", ResourceID: "user-tag", Region: "us-east-1",
				Tags: map[string]string{"team": "a"}},
			testutil.ConfigItem{ARN: tagged.ARN, ResourceType: "AWS::SQS::Queue", ResourceID: "tagged", Region: "us-east-1", Tags: tagged.Tags},
		)

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 0)
		requireMatch(t, "stdout", r.stdout, `(?m)^Resources with no user tags: 2$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: complete$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 3 resources scanned in 1 of 17 regions`)
		requireContains(t, "stderr", r.stderr, "never-tagged resources via AWS Config in 1 region")
	})
	t.Run("through Resource Explorer", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"eu-west-1": {{ARN: arnIn("eu-west-1", "tagged"), Tags: map[string]string{"Owner": "x"}}}})
		w.givenExplorerIn([]string{"eu-west-1"},
			testutil.ExplorerResource{ARN: arnIn("eu-west-1", "never"), ResourceType: "sqs:queue", Region: "eu-west-1"},
			testutil.ExplorerResource{ARN: arnIn("eu-west-1", "tagged"), ResourceType: "sqs:queue", Region: "eu-west-1"},
			// Resource Explorer does not index IAM tags, so IAM resources are left out.
			testutil.ExplorerResource{ARN: "arn:aws:iam::123456789012:role/tagged-role", ResourceType: "iam:role", Region: "global"},
		)

		r := w.run("scan", "--region", "eu-west-1")

		r.requireExit(t, 0)
		requireMatch(t, "stdout", r.stdout, `(?m)^Resources with no user tags: 1$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: complete$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 2 resources scanned in 1 of 17 regions`)
		for _, c := range w.fake.Calls() {
			if c.Op != testutil.OpListResources {
				continue
			}
			filters, _ := c.JSON["Filters"].(map[string]any)
			if c.JSON["MaxResults"] != float64(500) || filters["FilterString"] != "tag:none" {
				t.Fatalf("ListResources asked with %v, want MaxResults 500 and tag:none", c.JSON)
			}
		}
	})
}

// AC1.1.3: Given neither AWS Config nor Resource Explorer, When scan runs,
// Then it says that never-tagged resources could not be found and why, the
// other results are shown, and that alone does not make the exit code 1.
func TestAcceptanceU2_AC1_1_3_NoDiscoveryIsReportedButNotAnError(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {
		{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}},
		{ARN: arnIn("us-east-1", "b"), Tags: map[string]string{"Env": "dev"}},
	}})

	r := w.run("scan")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: unavailable \(.*AWS Config.*Resource Explorer.*\)$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  environment\s+2 resources$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 2 resources scanned in 17 of 17 regions · 1 variant group$`)
	requireContains(t, "stderr", r.stderr, "Exit code: 2 (issues found)")
}

// AC1.1.4: Given scan is running, When regions are read, Then the state of
// each region (done, running, waiting) and the elapsed time are shown on
// standard error. Standard error is not a terminal here, so lines are
// appended.
func TestAcceptanceU2_AC1_1_4_ProgressIsAppendedWhenNotATerminal(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "x"}}}})

	r := w.run("scan", "--region", "us-east-1", "--region", "eu-west-1")

	r.requireExit(t, 0)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: waiting: eu-west-1, us-east-1$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: us-east-1 started$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: us-east-1 done \(1 resource, elapsed \d\d:\d\d\)$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: eu-west-1 done \(0 resources, elapsed \d\d:\d\d\)$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: discovering never-tagged resources$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^scan: us-east-1 never-tagged resources: unavailable$`)
	requireNotContains(t, "stderr", r.stderr, "\x1b")
	requireNotContains(t, "stdout", r.stdout, "scan: ")
}

// AC1.1.5: Given one region denies reading, When scan runs, Then that region
// and the reason are shown at the top of the result, which is marked
// incomplete (exit code 1). plan does the same (BR5.6).
func TestAcceptanceU2_AC1_1_5_ADeniedRegionMakesTheResultIncomplete(t *testing.T) {
	given := func(t *testing.T) *world {
		w := newAccountWorld(t)
		denied := testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "not authorized to perform: tag:GetResources")
		w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
			"eu-west-1": testutil.Static(denied),
			"us-east-1": testutil.TagPages([]testutil.TaggedResource{
				{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}},
				{ARN: arnIn("us-east-1", "b"), Tags: map[string]string{"Environment": "dev"}},
			}),
		}, testutil.TagPages(nil)))
		w.givenAttachedPolicies(map[string]string{})
		return w
	}
	t.Run("scan", func(t *testing.T) {
		w := given(t)

		r := w.run("scan")

		r.requireExit(t, 1)
		if !strings.HasPrefix(r.stdout, "! Results are incomplete\n    eu-west-1: AccessDenied (tag:GetResources)\n") {
			t.Fatalf("stdout must start with the incomplete marker and the region:\n%s", r.stdout)
		}
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 2 resources scanned in 16 of 17 regions`)
		requireContains(t, "stderr", r.stderr, "Exit code: 1 (results incomplete)")
		requireNotContains(t, "stderr", r.stderr, "Error:")
	})
	t.Run("plan", func(t *testing.T) {
		w := given(t)
		rules := w.givenRuleFile(aliasRules)

		r := w.run("plan", "--rules", rules)

		r.requireExit(t, 1)
		if !strings.HasPrefix(r.stdout, "! Results are incomplete\n    eu-west-1: AccessDenied (tag:GetResources)\n") {
			t.Fatalf("stdout must start with the incomplete marker and the region:\n%s", r.stdout)
		}
		requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+1 resource`)
		requireContains(t, "stderr", r.stderr, "Checked: tagged resources in 16 of 17 regions (tagging API)")
		requireContains(t, "stderr", r.stderr, "Exit code: 1")
		if w.fake.Count(testutil.OpListIndexes) != 0 || w.fake.Count(testutil.OpDescribeConfigurationRecorders) != 0 {
			t.Fatal("plan must not look for never-tagged resources")
		}
	})
}

// AC1.1.5 with BR5.4: Given one region denies reading and the regions that
// were read have no issues, When scan or plan runs, Then the result is marked
// incomplete (exit code 1) and does not say "No issues found" or "No changes
// needed", which are only said with exit code 0.
func TestAcceptanceU2_AC1_1_5_IncompleteResultsNeverSayNothingWasFound(t *testing.T) {
	given := func(t *testing.T) *world {
		w := newAccountWorld(t)
		denied := testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "not authorized to perform: tag:GetResources")
		w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
			"eu-west-1": testutil.Static(denied),
			"us-east-1": testutil.TagPages([]testutil.TaggedResource{
				{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "a"}},
			}),
		}, testutil.TagPages(nil)))
		w.givenConfigEverywhere()
		return w
	}
	t.Run("scan", func(t *testing.T) {
		w := given(t)

		r := w.run("scan")

		r.requireExit(t, 1)
		requireMatch(t, "stdout", r.stdout, `(?m)^! Results are incomplete$`)
		requireNotContains(t, "stdout", r.stdout, "No issues found")
		requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: complete$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 1 resource scanned in 16 of 17 regions · 0 variant groups$`)
		requireContains(t, "stderr", r.stderr, "Exit code: 1 (results incomplete)")
	})
	t.Run("plan", func(t *testing.T) {
		w := given(t)
		rules := w.givenRuleFile(aliasRules)

		r := w.run("plan", "--rules", rules)

		r.requireExit(t, 1)
		requireMatch(t, "stdout", r.stdout, `(?m)^! Results are incomplete$`)
		requireNotContains(t, "stdout", r.stdout, "No changes needed")
		// Since U4 the key no rule names (Owner) is a remove candidate (BR2.3).
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 1 resource scanned · 0 to change · 1 remove candidate$`)
		requireContains(t, "stderr", r.stderr, "Exit code: 1 (results incomplete)")
	})
}

// AC1.1.6: Given the keys env, Env and environment, When scan runs, Then the
// three keys are one group with the count of each. A group that only the
// abbreviation dictionary makes is marked and does not count as an issue.
func TestAcceptanceU2_AC1_1_6_AbbreviationsJoinTheVariantGroup(t *testing.T) {
	t.Run("env, Env and environment", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {
			{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}},
			{ARN: arnIn("us-east-1", "b"), Tags: map[string]string{"Env": "dev"}},
			{ARN: arnIn("us-east-1", "c"), Tags: map[string]string{"environment": "dev", "prod": "yes"}},
			{ARN: arnIn("us-east-1", "d"), Tags: map[string]string{"envname": "x"}},
		}})

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 2)
		requireMatch(t, "stdout", r.stdout, `(?m)^  environment\s+3 resources$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    env\s+1$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    Env\s+1$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    environment\s+1$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^  production\s+1 resource  \(abbreviation match\)$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    prod\s+1$`)
		requireNotContains(t, "stdout", r.stdout, "envname")
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: .* · 1 variant group$`)
	})
	t.Run("only the dictionary joins the keys", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {
			{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}},
			{ARN: arnIn("us-east-1", "b"), Tags: map[string]string{"environment": "dev"}},
		}})

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 0)
		requireMatch(t, "stdout", r.stdout, `(?m)^  environment\s+2 resources  \(abbreviation match\)$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: .* · 0 variant groups$`)
	})
}

// AC1.1.7: Given a rule file that requires Owner, When scan runs, Then the
// number of resources without Owner is shown and matches the test data.
// Resources of types that cannot be tagged are shown separately.
func TestAcceptanceU2_AC1_1_7_MissingRequiredTagsAreCounted(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {
		{ARN: arnIn("us-east-1", "has-owner"), Tags: map[string]string{"Owner": "a", "Environment": "dev"}},
		{ARN: arnIn("us-east-1", "lower-owner"), Tags: map[string]string{"owner": "a"}},
		{ARN: arnIn("us-east-1", "no-owner"), Tags: map[string]string{"Environment": "dev"}},
	}})
	w.givenConfigEverywhere(
		testutil.ConfigItem{ARN: arnIn("us-east-1", "never"), ResourceType: "AWS::SQS::Queue", ResourceID: "never", Region: "us-east-1"},
		testutil.ConfigItem{ResourceType: "AWS::SSM::ManagedInstanceInventory", ResourceID: "i-1", Region: "us-east-1"},
	)
	rules := w.givenRuleFile(requiredOwnerRules + "  aws:createdBy:\n    default: x\n")

	r := w.run("scan", "--region", "us-east-1", "--rules", rules)

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^Required tags missing \(from .*tagrules\.yaml\)$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  Owner\s+3 resources  \(1 more on resource types that cannot be tagged\)$`)
	requireNotContains(t, "stdout", r.stdout, "aws:createdBy")
	requireMatch(t, "stdout", r.stdout, `(?m)^Resources with no user tags: 2$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^Resource types not supported for tag changes: 1 type, 1 resource$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ssm:managed-instance-inventory\s+1$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 5 resources scanned in 1 of 17 regions · 1 variant group · 1 required tag missing$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^warning: .*aws:createdBy`)
	requireNotContains(t, "stderr", r.stderr, `"required" is not used`)
}

// AC1.1.8: Given only resources without problems, When scan runs, Then "no
// issues" and the number of resources are shown; when never-tagged
// resources could not be looked for everywhere, that is added.
func TestAcceptanceU2_AC1_1_8_NoIssuesSaysWhatWasChecked(t *testing.T) {
	ok := map[string][]testutil.TaggedResource{"us-east-1": {
		{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "a"}},
		{ARN: arnIn("us-east-1", "b"), Tags: map[string]string{"Owner": "b"}},
	}}
	t.Run("discovery complete", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(ok)
		w.givenConfigEverywhere()
		rules := w.givenRuleFile(requiredOwnerRules)

		r := w.run("scan", "--region", "us-east-1", "--rules", rules)

		r.requireExit(t, 0)
		if !strings.HasPrefix(r.stdout, "No issues found\n") {
			t.Fatalf("stdout must start with No issues found:\n%s", r.stdout)
		}
		requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 2 resources scanned in 1 of 17 regions · 0 variant groups · 0 required tags missing$`)
		requireNotContains(t, "stdout", r.stdout, "lower bound")
	})
	t.Run("discovery partial", func(t *testing.T) {
		w := newAccountWorld(t)
		w.givenTaggedByRegion(ok)
		w.fake.Handle(testutil.OpDescribeConfigurationRecorders, testutil.PerRegion(map[string]testutil.Handler{
			"us-east-1": testutil.Static(testutil.ConfigRecorders(testutil.Recorder{AllSupported: true, Recording: true})),
		}, testutil.Static(testutil.ConfigRecorders())))
		w.fake.Handle(testutil.OpDescribeConfigurationRecorderStatus, testutil.Static(testutil.ConfigRecorderStatus(testutil.Recorder{Recording: true})))
		w.fake.Handle(testutil.OpSelectResourceConfig, testutil.ConfigPages())
		rules := w.givenRuleFile(requiredOwnerRules)

		r := w.run("scan", "--region", "us-east-1", "--region", "eu-west-1", "--rules", rules)

		r.requireExit(t, 0)
		if !strings.HasPrefix(r.stdout, "No issues found (untagged-resource discovery: partial)\n") {
			t.Fatalf("stdout must start with No issues found and the discovery state:\n%s", r.stdout)
		}
		requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: partial \(not checked in eu-west-1: .+\)$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^  Required tag counts are a lower bound\.$`)
	})
}

// AC1.1.10: Given AWS answers with a call rate limit, When scan runs, Then it
// waits, retries and finishes.
func TestAcceptanceU2_AC1_1_10_RateLimitedCallsAreRetried(t *testing.T) {
	w := newAccountWorld(t)
	throttled := testutil.JSONError(http.StatusBadRequest, "ThrottlingException", "Rate exceeded")
	w.fake.Handle("tag:GetResources", testutil.Sequence(throttled, throttled, throttled,
		testutil.TagPage("", testutil.TaggedResource{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "a"}})))
	w.givenConfigEverywhere()
	w.fake.Handle(testutil.OpSelectResourceConfig, testutil.Sequence(throttled, throttled, throttled, testutil.ConfigResults("")))

	r := w.run("scan", "--debug", "--region", "us-east-1")

	r.requireExit(t, 0)
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 1 resource scanned in 1 of 17 regions`)
	requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: complete$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^debug: tag:GetResources us-east-1 \d+ms retries=3$`)
	requireMatch(t, "stderr", r.stderr, `(?m)^debug: config:SelectResourceConfig us-east-1 \d+ms retries=3$`)
}

// AC1.2.1: Given only the example read-only policy, When scan and plan run,
// Then they work, no write API is called, and neither is the API Search of
// Resource Explorer (only ListResources).
func TestAcceptanceU2_AC1_2_1_OnlyTheListedReadOperationsAreCalled(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{
		"us-east-1": {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"env": "dev"}}},
		"eu-west-1": {{ARN: arnIn("eu-west-1", "b"), Tags: map[string]string{"Env": "dev"}}},
	})
	w.fake.HandleConfigRecorders(testutil.Recorder{Strategy: testutil.StrategyInclusion, Types: []string{"AWS::SQS::Queue"}, Recording: true})
	w.fake.Handle(testutil.OpSelectResourceConfig, testutil.ConfigPages())
	w.givenExplorerIn(allRegions, testutil.ExplorerResource{ARN: arnIn("eu-west-1", "never"), ResourceType: "sqs:queue", Region: "eu-west-1"})
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(requiredOwnerRules)

	scan := w.run("scan", "--rules", rules)
	plan := w.run("plan", "--rules", rules)

	scan.requireExit(t, 2)
	plan.requireExit(t, 2)
	requireNotContains(t, "scan stderr", scan.stderr, "Error:")
	requireNotContains(t, "plan stderr", plan.stderr, "Error:")
	requireMatch(t, "scan stdout", scan.stdout, `(?m)^Untagged-resource discovery: complete$`)
	for _, op := range w.fake.Ops() {
		if !slices.Contains(apiOperations, op) || !testutil.IsReadOperation(op) {
			t.Errorf("operation outside the read-only table was called: %s", op)
		}
	}
	if w.fake.Count("resource-explorer-2:Search") != 0 {
		t.Fatal("the API Search of Resource Explorer was called")
	}
	for _, op := range []string{testutil.OpSelectResourceConfig, testutil.OpListResources, testutil.OpGetView, "iam:GetPolicyVersion"} {
		if w.fake.Count(op) == 0 {
			t.Errorf("%s was not exercised", op)
		}
	}
}

// AC1.2.2: Given part of the read permissions is missing, When scan runs, Then
// the IAM name of the missing operation is shown, including
// resource-explorer-2:Search for ListResources. A missing default view is
// told apart from a view that cannot be used.
func TestAcceptanceU2_AC1_2_2_MissingDiscoveryPermissionsNameTheIAMAction(t *testing.T) {
	w := newAccountWorld(t)
	w.givenTaggedByRegion(map[string][]testutil.TaggedResource{"us-east-1": {{ARN: arnIn("us-east-1", "a"), Tags: map[string]string{"Owner": "a"}}}})
	w.fake.HandleConfigRecorders(testutil.Recorder{AllSupported: true, Recording: true})
	w.fake.Handle(testutil.OpSelectResourceConfig, testutil.Static(testutil.JSONError(http.StatusBadRequest, "AccessDeniedException",
		"not authorized to perform: config:SelectResourceConfig")))
	w.fake.Handle(testutil.OpListIndexes, testutil.Static(testutil.ExplorerIndexes(
		testutil.Index{Region: "eu-west-1"}, testutil.Index{Region: "ap-northeast-1"})))
	w.fake.Handle(testutil.OpGetDefaultView, testutil.PerRegion(map[string]testutil.Handler{
		"eu-west-1": testutil.Static(testutil.DefaultView(testutil.ViewARN("eu-west-1"))),
	}, testutil.Static(testutil.DefaultView(""))))
	w.fake.Handle(testutil.OpGetView, testutil.Static(testutil.View(testutil.ViewARN("eu-west-1"), "")))
	w.fake.Handle(testutil.OpListResources, testutil.Static(testutil.RestJSONError(http.StatusUnauthorized, "UnauthorizedException", "Unauthorized")))

	r := w.run("scan", "--region", "us-east-1", "--region", "eu-west-1", "--region", "ap-northeast-1")

	r.requireExit(t, 0)
	line := regexp.MustCompile(`(?m)^Untagged-resource discovery: unavailable \(.*\)$`).FindString(r.stdout)
	if line == "" {
		t.Fatalf("no unavailable discovery line:\n%s", r.stdout)
	}
	for _, want := range []string{"AccessDenied config:SelectResourceConfig", "AccessDenied resource-explorer-2:Search", "no default Resource Explorer view"} {
		requireContains(t, "discovery line", line, want)
	}
	if w.fake.Count(testutil.OpListResources) != 1 {
		t.Fatalf("ListResources called %d times; it must not be called without a default view", w.fake.Count(testutil.OpListResources))
	}
	requireNotContains(t, "stdout", r.stdout, "not authorized")
}

// AC1.2.3 and AC1.2.5: Given the credentials expire during the collection,
// When scan or plan runs, Then the command stops with the two-line message,
// prints no partial result, exits 1, never prints the credentials, and leaves
// no request running (NFR5.8, NFR8.11).
func TestAcceptanceU2_AC1_2_3_ExpiredCredentialsStopEverything(t *testing.T) {
	expired := testutil.JSONError(http.StatusBadRequest, "ExpiredTokenException", "token "+testutil.FakeSessionToken+" expired")
	everywhere := map[string][]testutil.TaggedResource{}
	for _, region := range allRegions {
		everywhere[region] = []testutil.TaggedResource{{ARN: arnIn(region, "q"), Tags: map[string]string{"env": "dev"}}}
	}
	scenarios := map[string]struct {
		given func(w *world)
		args  func(rules string) []string
	}{
		"scan, tagging API": {
			given: func(w *world) {
				w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
					"eu-west-1": testutil.Static(expired),
				}, testutil.TagPagesByRegion(everywhere)))
			},
			args: func(string) []string { return []string{"scan", "--debug"} },
		},
		"scan, discovery": {
			given: func(w *world) {
				w.givenTaggedByRegion(everywhere)
				w.givenExplorerIn(allRegions)
				w.fake.Handle(testutil.OpListResources, testutil.PerRegion(map[string]testutil.Handler{
					"sa-east-1": testutil.Static(testutil.RestJSONError(http.StatusForbidden, "ExpiredTokenException", "expired")),
				}, testutil.ExplorerPages()))
			},
			args: func(string) []string { return []string{"scan"} },
		},
		"plan, IAM policies": {
			given: func(w *world) {
				w.givenTaggedByRegion(everywhere)
				w.fake.Handle("iam:ListPolicies", testutil.PolicyPages([]testutil.ManagedPolicy{{ARN: policyP1}, {ARN: policyP2}}))
				w.fake.Handle("iam:GetPolicyVersion", testutil.Static(testutil.QueryError(http.StatusForbidden, "ExpiredToken", "expired")))
			},
			args: func(rules string) []string { return []string{"plan", "--rules", rules} },
		},
	}
	for name, sc := range scenarios {
		t.Run(name, func(t *testing.T) {
			w := newAccountWorld(t)
			sc.given(w)
			rules := w.givenRuleFile(aliasRules)

			r := w.run(sc.args(rules)...)

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireMatch(t, "stderr", r.stderr, `(?m)^Error: the AWS credentials have expired`)
			for _, secret := range []string{testutil.FakeAccessKeyID, testutil.FakeSecretAccessKey, testutil.FakeSessionToken} {
				requireNotContains(t, "stderr", r.stderr, secret)
			}
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

// NFR8.9: control characters, bidirectional overrides and invalid UTF-8 in
// the ARNs and types read from AWS Config and Resource Explorer never reach
// the terminal as they are.
func TestAcceptanceU2_NFR8_9_DiscoveredStringsAreSanitized(t *testing.T) {
	w := newAccountWorld(t)
	evil := "\xc2\x9b31m\xe2\x80\xaeevil\nline\xff"
	w.fake.HandleConfigRecorders(testutil.Recorder{AllSupported: true, Recording: true})
	w.fake.Handle(testutil.OpSelectResourceConfig, testutil.ConfigPages(
		testutil.ConfigItem{ResourceType: "AWS::Evil::" + evil, ResourceID: "x", Region: "us-east-1"},
		testutil.ConfigItem{ARN: "arn:aws:sqs:us-east-1:123456789012:" + evil, ResourceType: "AWS::SQS::Queue", ResourceID: "y", Region: "us-east-1"},
	))

	r := w.run("scan", "--region", "us-east-1")

	r.requireExit(t, 0)
	for _, raw := range []string{"\xc2\x9b", "\xe2\x80\xae", "evil\nline", "\xff"} {
		requireNotContains(t, "stdout", r.stdout, raw)
		requireNotContains(t, "stderr", r.stderr, raw)
	}
	// The invalid byte 0xFF cannot survive JSON: the answer carries U+FFFD
	// instead, which is printable and kept. The raw byte is checked above.
	requireContains(t, "stdout", r.stdout, `\u{9B}31m\u{202E}evil\u{A}line`+"�")
}

// The count of regions and resources in the summary of a large account, as a
// quick sanity check of the totals used by the other tests.
func TestAcceptanceU2_SummaryCountsResourcesOfAllRegionsOnce(t *testing.T) {
	w := newAccountWorld(t)
	byRegion := map[string][]testutil.TaggedResource{}
	for i, region := range allRegions {
		for j := 0; j <= i; j++ {
			byRegion[region] = append(byRegion[region], testutil.TaggedResource{ARN: arnIn(region, fmt.Sprintf("q%d", j)), Tags: map[string]string{"Owner": "a"}})
		}
	}
	// A global resource (no region in its ARN) is returned by two regions.
	global := testutil.TaggedResource{ARN: "arn:aws:s3:::bucket-global", Tags: map[string]string{"Owner": "a"}}
	byRegion["us-east-1"] = append(byRegion["us-east-1"], global)
	byRegion["eu-west-1"] = append(byRegion["eu-west-1"], global)
	w.givenTaggedByRegion(byRegion)

	r := w.run("scan")

	r.requireExit(t, 0)
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: 154 resources scanned in 17 of 17 regions`)
}
