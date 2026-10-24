package cli_test

// Unit tests of the command line, written after the implementation. They use
// the same fake AWS and isolated environment as the acceptance tests.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// runWith runs the command line like world.run, with changed dependencies.
func (w *world) runWith(change func(d *cli.Deps), args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	deps := cli.Deps{Transport: w.fake, RetryDelay: func(int) time.Duration { return 0 }}
	change(&deps)
	code := cli.Run(context.Background(), args, &stdout, &stderr, deps)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestUsageErrorsAreTwoLinesAndExitOne(t *testing.T) {
	cases := map[string]struct {
		args []string
		what string
	}{
		"unknown flag":             {[]string{"scan", "--bogus"}, "unknown flag: --bogus"},
		"positional argument":      {[]string{"scan", "extra"}, `unknown command "extra"`},
		"unknown command":          {[]string{"nope"}, `unknown command "nope"`},
		"later command":            {[]string{"apply", "--plan", "p.json"}, "apply is not available in this version"},
		"rules suggest with rules": {[]string{"rules", "suggest", "--rules", "r.yaml"}, "unknown flag: --rules"},
		"unknown rules command":    {[]string{"rules", "check"}, `unknown command "check"`},
		"comma-separated regions":  {[]string{"scan", "--region", "us-east-1,eu-west-1"}, `"us-east-1,eu-west-1" is not an AWS region name`},
		"broken rule file on scan": {[]string{"scan", "--rules", "missing-rules.yaml"}, "missing-rules.yaml: the rule file does not exist"},
		"plan without a rule file": {[]string{"plan"}, "plan needs a rule file"},
		// U4 made --format, --out and --lang available (contract C5); their
		// wrong values are the usage errors now, still before any AWS call.
		"diff on scan":            {[]string{"scan", "--format", "diff"}, `--format "diff" is not one of table|json|markdown`},
		"unknown format on plan":  {[]string{"plan", "--rules", "r.yaml", "--format", "yaml"}, `--format "yaml" is not one of table|diff|json|markdown`},
		"unknown lang on suggest": {[]string{"rules", "suggest", "--lang", "fr"}, `--lang "fr" is not en or ja`},
		"unknown lang on plan":    {[]string{"plan", "--lang", "EN"}, `--lang "EN" is not en or ja`},
		"out in a missing folder": {[]string{"plan", "--rules", "r.yaml", "--out", "no-such-folder/p.json"}, "the folder of no-such-folder"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)

			r := w.run(tc.args...)

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireContains(t, "stderr", r.stderr, "Error: "+tc.what)
			requireContains(t, "stderr", r.stderr, "Exit code: 1 (error)")
			if len(w.fake.Calls()) != 0 {
				t.Fatalf("AWS was called: %v", w.fake.Ops())
			}
		})
	}
}

func TestHelpAndNoArgumentsExitZero(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {}, {"scan", "--help"}, {"plan", "-h"}} {
		w := newWorld(t)

		r := w.run(args...)

		r.requireExit(t, 0)
		requireContains(t, "stdout", r.stdout, "Usage:")
		requireNotContains(t, "stderr", r.stderr, "Exit code")
	}
	r := newWorld(t).run("scan", "--help")
	// U4 lists --format and --lang, which earlier versions hid.
	for _, option := range []string{"--profile", "--region", "--debug", "--format", "--lang"} {
		requireContains(t, "scan help", r.stdout, option)
	}
}

func TestRuleFileErrorsNameTheFileLineKeyAndFix(t *testing.T) {
	cases := map[string]struct {
		content string
		what    string
		fix     string
	}{
		"unknown key": {"version: 1\nkeys:\n  Environment:\n    aliasses: [env]\n",
			`line 4: unknown key "aliasses"`, `did you mean "aliases"?`},
		"unsupported version": {"version: 2\n", "line 1: rule file version 2 is not supported", "this version of tag-refactor reads rule file version 1"},
		"broken YAML":         {"version: 1\nkeys: [a, b\n", "line 2: the file is not valid YAML", "fix the YAML syntax"},
		"alias used twice": {"version: 1\nkeys:\n  A:\n    aliases: [x]\n  B:\n    aliases: [x]\n",
			`line 6: the alias "x" is listed under "A" (line 4) and "B" (line 6)`, "keep the alias under one canonical key only"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			path := w.givenRuleFile(tc.content)

			r := w.run("plan", "--rules", path, "--region", "us-east-1")

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			// The path is printed sanitized: on Windows its backslashes are doubled.
			requireContains(t, "stderr", r.stderr, "Error: "+display.Sanitize(path)+" "+tc.what)
			requireContains(t, "stderr", r.stderr, "  Fix: "+tc.fix)
			if len(w.fake.Calls()) != 0 {
				t.Fatal("AWS must not be called when the rule file is wrong")
			}
		})
	}
	w := newWorld(t)
	r := w.run("plan", "--rules", w.t.TempDir()+"/missing.yaml")
	r.requireExit(t, 1)
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: .*missing\.yaml: the rule file does not exist$`)
}

func TestDebugShowsEachCallButNoTagValues(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "VALUE-ONE-123", "Env": "VALUE-TWO-456"}})

	r := w.run("scan", "--debug", "--region", "us-east-1")

	r.requireExit(t, 2)
	for _, op := range []string{"sts:GetCallerIdentity", "ec2:DescribeRegions", "tag:GetResources"} {
		requireMatch(t, "stderr", r.stderr, `(?m)^debug: `+op+` us-east-1 \d+ms retries=0$`)
	}
	requireNotContains(t, "stderr", r.stderr, "VALUE-")
	requireNotContains(t, "stdout", r.stdout, "VALUE-")

	quiet := w.run("scan", "--region", "us-east-1")
	requireNotContains(t, "stderr", quiet.stderr, "debug:")
}

func TestCredentialProblemsSayHowToFixThem(t *testing.T) {
	t.Run("expired SSO session", func(t *testing.T) {
		w := newWorld(t)
		w.env.WriteConfig("[profile dev]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n" +
			"sso_account_id = 123456789012\nsso_role_name = ReadOnly\nregion = us-east-1\n")
		w.env.WriteSSOToken("https://example.awsapps.com/start", time.Now().Add(-time.Hour))

		r := w.run("scan", "--profile", "dev")

		r.requireExit(t, 1)
		requireTwoLineError(t, r)
		requireContains(t, "stderr", r.stderr, "Error: the AWS SSO session has expired or is not signed in")
		requireContains(t, "stderr", r.stderr, `  Fix: run "aws sso login --profile dev" and try again`)
	})
	t.Run("expired session token", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Handle("sts:GetCallerIdentity", testutil.Static(testutil.QueryError(http.StatusForbidden, "ExpiredToken", "expired")))

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Error: the AWS credentials have expired (sts:GetCallerIdentity)")
	})
	t.Run("rejected access key", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Handle("sts:GetCallerIdentity", testutil.Static(testutil.QueryError(http.StatusForbidden, "InvalidClientTokenId", "bad")))

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Error: AWS rejected the credentials (sts:GetCallerIdentity)")
	})
}

func TestRegionProblemsSayHowToChooseARegion(t *testing.T) {
	t.Run("no region anywhere", func(t *testing.T) {
		w := newWorld(t)
		w.env.UseFakeCredentials("")

		r := w.run("scan")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Error: no AWS region is set")
		requireContains(t, "stderr", r.stderr, "--region")
	})
	t.Run("region not enabled", func(t *testing.T) {
		w := newWorld(t)

		r := w.run("scan", "--region", "me-central-1")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Error: the region me-central-1 is not available for this account")
	})
	t.Run("disabled opt-in region rejects the credentials", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Handle("sts:GetCallerIdentity", func(c testutil.Call) testutil.Reply {
			if c.Region == "me-south-1" {
				return testutil.QueryError(http.StatusForbidden, "InvalidClientTokenId", "The security token included in the request is invalid.")
			}
			return testutil.CallerIdentity(testutil.TestAccount, testutil.TestCallerARN)
		})

		r := w.run("scan", "--region", "me-south-1")

		r.requireExit(t, 1)
		requireTwoLineError(t, r)
		requireContains(t, "stderr", r.stderr, "Error: the region me-south-1 is not available for this account")
		requireNotContains(t, "stderr", r.stderr, "rejected the credentials")
	})
	t.Run("environment region", func(t *testing.T) {
		w := newWorld(t)
		t.Setenv("AWS_REGION", "eu-west-1")
		w.givenResources()

		r := w.run("scan")

		r.requireExit(t, 0)
		requireContains(t, "stderr", r.stderr, "region eu-west-1")
	})
}

func TestPlanWithoutChangesReadsNoPolicies(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 0)
	if r.stdout != "No changes needed\nSummary: 1 resource scanned · 0 to change\n" {
		t.Fatalf("stdout = %q", r.stdout)
	}
	requireContains(t, "stderr", r.stderr, "Exit code: 0 (no changes needed)")
	requireContains(t, "stderr", r.stderr, "Checked: tagged resources in 1 of 3 regions (tagging API); no IAM policies were read: nothing to change\n")
	if w.fake.Count("iam:ListPolicies") != 0 {
		t.Fatal("policies were read although nothing changes (BR4.4)")
	}
}

// Review R-07: a rule file without key rules still plans nothing, and says so
// on standard error.
func TestPlanWarnsAboutARuleFileWithoutKeyRules(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	rules := w.givenRuleFile("version: 1\n")

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 0)
	// Since U4 a key no rule names is a remove candidate (BR2.3), listed under
	// "No changes needed" without changing the exit code (Q4).
	if r.stdout != "No changes needed\n\nRemove candidates (keys not in your rules; not changed): 1 key\n  env (1)\n\n"+
		"Summary: 1 resource scanned · 0 to change · 1 remove candidates\n" {
		t.Fatalf("stdout = %q", r.stdout)
	}
	requireContains(t, "stderr", r.stderr, "warning: "+display.Sanitize(rules)+` line 1: no key rules: "keys" is missing or empty, so no tag key is renamed`)
}

// Review R-04: the scope of the check is stated on standard error, and
// standard output keeps only the results (BR6.1).
func TestScanAndPlanStateWhatTheyChecked(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(aliasRules)

	scan := w.run("scan", "--region", "us-east-1")
	plan := w.run("plan", "--rules", rules, "--region", "us-east-1")

	scan.requireExit(t, 0)
	// The U2 wording (BR5.7): regions read out of the enabled ones, and how
	// never-tagged resources were looked for.
	requireContains(t, "scan stderr", scan.stderr, "\nChecked: tagged resources in 1 of 3 regions (tagging API); never-tagged resources not checked\nDone in ")
	requireNotContains(t, "scan stdout", scan.stdout, "Checked")
	plan.requireExit(t, 2)
	requireContains(t, "plan stderr", plan.stderr, "\nChecked: tagged resources in 1 of 3 regions (tagging API), "+
		"and customer managed IAM policies attached to a user, role or group\nDone in ")
	requireNotContains(t, "plan stdout", plan.stdout, "Checked")
	// U4 adds the required values not set, the remove candidates and the
	// IaC-managed resources to the summary (BR3.2).
	if !strings.HasSuffix(plan.stdout, "Summary: 1 to change · 0 conflicts · 0 affected · 0 undecided · 0 value not set · 0 remove candidates · 0 IaC-managed\n") {
		t.Fatalf("plan stdout = %q", plan.stdout)
	}
}

func TestPlanWithOnlyConflictsNeedsReviewWithoutImpactAnalysis(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQC, Tags: map[string]string{"env": "dev", "Environment": "prod"}})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireContains(t, "stdout", r.stdout, "Summary: 0 to change · 1 conflicts · 0 affected · 0 undecided")
	if w.fake.Count("iam:ListPolicies") != 0 {
		t.Fatal("no change, so no impact analysis")
	}
}

// Failures of the calls that every command needs (the account, or an answer
// that cannot be understood) stop the command with a two-line message. Since
// U2, a region whose tagging API keeps failing is part of the result instead
// (see TestARegionThatKeepsFailingIsReportedInTheResult).
func TestAWSFailuresHaveTwoLineMessages(t *testing.T) {
	cases := map[string]struct {
		op     string
		reply  testutil.Reply
		change func(d *cli.Deps)
		what   string
	}{
		"call time limit": {"sts:GetCallerIdentity", testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.CallTimeout = 50 * time.Millisecond },
			"AWS did not answer sts:GetCallerIdentity in time (region us-east-1)"},
		"attempt time limit": {"sts:GetCallerIdentity", testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.AttemptTimeout = 20 * time.Millisecond },
			"AWS kept failing on sts:GetCallerIdentity after retries (region us-east-1)"},
		"throttling": {"sts:GetCallerIdentity", testutil.QueryError(400, "Throttling", "slow down"), func(*cli.Deps) {},
			"AWS kept failing on sts:GetCallerIdentity after retries (region us-east-1)"},
		"unexpected answer": {"tag:GetResources", testutil.Reply{Body: "<<<", ContentType: "application/x-amz-json-1.1"}, func(*cli.Deps) {},
			"an unexpected error occurred in tag:GetResources (region us-east-1)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.givenResources()
			w.fake.Handle(tc.op, testutil.Static(tc.reply))

			r := w.runWith(tc.change, "scan", "--region", "us-east-1")

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireContains(t, "stderr", r.stderr, "Error: "+tc.what)
			requireContains(t, "stderr", r.stderr, "--debug")
		})
	}
}

// BR4.1, NFR5.5: a region whose tagging API keeps failing (throttling after
// ten attempts, or the time limits) is listed at the top of the result with
// the kind of failure; the other regions are still read.
func TestARegionThatKeepsFailingIsReportedInTheResult(t *testing.T) {
	cases := map[string]struct {
		reply  testutil.Reply
		change func(d *cli.Deps)
	}{
		"call time limit":    {testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.CallTimeout = 50 * time.Millisecond }},
		"attempt time limit": {testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.AttemptTimeout = 20 * time.Millisecond }},
		"throttling":         {testutil.JSONError(400, "ThrottlingException", "slow down"), func(*cli.Deps) {}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.fake.Handle("tag:GetResources", testutil.PerRegion(map[string]testutil.Handler{
				"eu-west-1": testutil.Static(tc.reply),
			}, testutil.TagPages([]testutil.TaggedResource{{ARN: arnQA, Tags: map[string]string{"Owner": "a"}}})))

			r := w.runWith(tc.change, "scan", "--region", "us-east-1", "--region", "eu-west-1")

			r.requireExit(t, 1)
			if !strings.HasPrefix(r.stdout, "! Results are incomplete\n    eu-west-1: Transient (tag:GetResources)\n") {
				t.Fatalf("stdout must start with the failed region:\n%s", r.stdout)
			}
			requireContains(t, "stdout", r.stdout, "Summary: 1 resource scanned in 1 of 3 regions")
		})
	}
	w := newWorld(t)
	w.fake.Handle("tag:GetResources", testutil.Static(testutil.JSONError(400, "ThrottlingException", "slow down")))
	w.runWith(func(*cli.Deps) {}, "scan", "--region", "us-east-1")
	if n := w.fake.Count("tag:GetResources"); n != 10 {
		t.Fatalf("%d attempts, want 10 (NFR5.4)", n)
	}
}

// BR5.2: scan counts the required keys, so it does not say they are unused.
// U3 plan read them without applying them and said so; since U4 plan adds
// them (BR1.4), so the line is gone and the plan has a change to review.
func TestScanUsesRequiredKeysWhilePlanWarnsAboutThem(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}})
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(aliasRules + "required:\n  Owner:\n    default: x\n")

	scan := w.run("scan", "--region", "us-east-1", "--rules", rules)
	plan := w.run("plan", "--region", "us-east-1", "--rules", rules)

	scan.requireExit(t, 2)
	requireMatch(t, "scan stdout", scan.stdout, `(?m)^  Owner\s+1 resource$`)
	requireNotContains(t, "scan stderr", scan.stderr, `"required" is not used`)
	requireNotContains(t, "scan stderr", scan.stderr, "not applied to the plan yet")
	plan.requireExit(t, 2)
	requireMatch(t, "plan stdout", plan.stdout, `(?m)^  \+ add\s+Owner = x\s+1 resource$`)
	requireNotContains(t, "plan stderr", plan.stderr, "not applied to the plan yet")
}

// BR1.1: the first --region is where the account is identified; every
// --region is read.
func TestTheFirstRegionIsTheSessionRegion(t *testing.T) {
	w := newWorld(t)
	w.givenResources()

	r := w.run("scan", "--region", "eu-west-1", "--region", "ap-northeast-1")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "Account 123456789012 · region eu-west-1")
	for _, c := range w.fake.Calls() {
		if c.Op == "sts:GetCallerIdentity" && c.Region != "eu-west-1" {
			t.Fatalf("the account was identified in %s", c.Region)
		}
	}
	if got := regionsCalled(w.fake, "tag:GetResources"); strings.Join(got, ",") != "ap-northeast-1,eu-west-1" {
		t.Fatalf("regions read = %v", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }

func TestAFailedWriteOfTheResultExitsOne(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}})
	var stderr bytes.Buffer

	code := cli.Run(context.Background(), []string{"scan", "--region", "us-east-1"}, failingWriter{}, &stderr,
		cli.Deps{Transport: w.fake})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 when standard output cannot be written", code)
	}
	if code := cli.Run(context.Background(), []string{"--help"}, failingWriter{}, &stderr, cli.Deps{}); code != 1 {
		t.Fatalf("help with a broken stdout: exit code = %d", code)
	}
}

// Performance (NFR1.4): 10,000 resources through the HTTP-level fake for both
// scan and plan. The limit is 10 seconds on every CI OS; locally this runs in
// well under half of that.
func TestPerformanceScanAndPlanWith10000Resources(t *testing.T) {
	w := newWorld(t)
	var pages [][]testutil.TaggedResource
	for p := 0; p < 100; p++ {
		var page []testutil.TaggedResource
		for i := 0; i < 100; i++ {
			n := p*100 + i
			key := []string{"env", "Env", "Environment", "env"}[n%4]
			page = append(page, testutil.TaggedResource{
				ARN:  fmt.Sprintf("arn:aws:ec2:us-east-1:123456789012:instance/i-%08d", n),
				Tags: map[string]string{key: "dev", "Owner": "team", "cost-center": "1"},
			})
		}
		pages = append(pages, page)
	}
	w.fake.Handle("tag:GetResources", testutil.TagPages(pages...))
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(aliasRules)

	start := time.Now()
	scan := w.run("scan", "--region", "us-east-1")
	plan := w.run("plan", "--rules", rules, "--region", "us-east-1")
	elapsed := time.Since(start)

	scan.requireExit(t, 2)
	plan.requireExit(t, 2)
	// U4 BR3.6: counts have a comma every three digits.
	requireContains(t, "scan stdout", scan.stdout, "Summary: 10,000 resources scanned")
	requireContains(t, "plan stdout", plan.stdout, "Summary: 7,500 to change")
	if elapsed > timeLimitFactor*10*time.Second {
		t.Fatalf("scan and plan of 10,000 resources took %v (limit 10s)", elapsed)
	}
	t.Logf("scan and plan of 10,000 resources: %v", elapsed)
}

// Performance (NFR1.5): 500 attached policies against 1,000 changes, through
// the HTTP-level fake, including pathological wildcard patterns.
func TestPerformanceImpactWith500PoliciesAnd1000Changes(t *testing.T) {
	w := newWorld(t)
	var resources []testutil.TaggedResource
	for i := 0; i < 1000; i++ {
		resources = append(resources, testutil.TaggedResource{
			ARN:  fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:q-%04d", i),
			Tags: map[string]string{fmt.Sprintf("old-key-%04d", i): "v"},
		})
	}
	w.givenResources(resources...)
	var rules strings.Builder
	rules.WriteString("version: 1\nkeys:\n")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&rules, "  NewKey%04d:\n    aliases: [old-key-%04d]\n", i, i)
	}
	docs := map[string]string{}
	for i := 0; i < 500; i++ {
		docs[fmt.Sprintf("arn:aws:iam::123456789012:policy/P%03d", i)] = fmt.Sprintf(`{"Version":"2012-10-17","Statement":[`+
			`{"Sid":"Exact","Effect":"Allow","Action":"sqs:*","Resource":"*","Condition":{"StringEquals":{"aws:ResourceTag/old-key-%04d":"v","ec2:ResourceTag/other":"x"}}},`+
			`{"Sid":"Wild","Effect":"Allow","Action":"sqs:*","Resource":"*","Condition":{"StringLike":{"aws:ResourceTag/o*l*d*-*k*e*y*-*9*9*z":"v"}}},`+
			`{"Sid":"Var","Effect":"Allow","Action":"sqs:*","Resource":"*","Condition":{"StringEquals":{"aws:ResourceTag/${aws:PrincipalTag/k}":"v"}}}]}`, i*2)
	}
	w.givenAttachedPolicies(docs)
	path := w.givenRuleFile(rules.String())

	start := time.Now()
	r := w.run("plan", "--rules", path, "--region", "us-east-1")
	elapsed := time.Since(start)

	r.requireExit(t, 2)
	// U4 BR3.6: counts have a comma every three digits.
	requireContains(t, "stdout", r.stdout, "Summary: 1,000 to change · 0 conflicts · 500 affected · 500 undecided")
	if elapsed > timeLimitFactor*10*time.Second {
		t.Fatalf("impact analysis of 500 policies x 1,000 changes took %v (limit 10s)", elapsed)
	}
	t.Logf("impact analysis of 500 policies x 1,000 changes: %v", elapsed)
}
