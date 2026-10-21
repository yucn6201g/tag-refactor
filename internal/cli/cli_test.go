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
		"rules suggest":            {[]string{"rules", "suggest"}, "rules is not available in this version"},
		"format on scan":           {[]string{"scan", "--format", "json"}, "--format is not available in this version"},
		"rules on scan":            {[]string{"scan", "--rules", "r.yaml"}, "--rules is not available in this version"},
		"out on plan":              {[]string{"plan", "--rules", "r.yaml", "--out", "p.json"}, "--out is not available in this version"},
		"lang on plan":             {[]string{"plan", "--lang", "ja"}, "--lang is not available in this version"},
		"plan without a rule file": {[]string{"plan"}, "plan needs a rule file"},
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
	for _, option := range []string{"--profile", "--region", "--debug"} {
		requireContains(t, "scan help", r.stdout, option)
	}
	requireNotContains(t, "scan help", r.stdout, "--format")
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
	if w.fake.Count("iam:ListPolicies") != 0 {
		t.Fatal("policies were read although nothing changes (BR4.4)")
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

func TestAWSFailuresHaveTwoLineMessages(t *testing.T) {
	cases := map[string]struct {
		reply  testutil.Reply
		change func(d *cli.Deps)
		what   string
	}{
		"call time limit": {testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.CallTimeout = 50 * time.Millisecond },
			"AWS did not answer tag:GetResources in time (region us-east-1)"},
		"attempt time limit": {testutil.Reply{Delay: time.Hour}, func(d *cli.Deps) { d.AttemptTimeout = 20 * time.Millisecond },
			"AWS kept failing on tag:GetResources after retries (region us-east-1)"},
		"throttling": {testutil.JSONError(400, "ThrottlingException", "slow down"), func(*cli.Deps) {},
			"AWS kept failing on tag:GetResources after retries (region us-east-1)"},
		"unexpected answer": {testutil.Reply{Body: "<<<", ContentType: "application/x-amz-json-1.1"}, func(*cli.Deps) {},
			"an unexpected error occurred in tag:GetResources (region us-east-1)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.fake.Handle("tag:GetResources", testutil.Static(tc.reply))

			r := w.runWith(tc.change, "scan", "--region", "us-east-1")

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireContains(t, "stderr", r.stderr, "Error: "+tc.what)
			requireContains(t, "stderr", r.stderr, "--debug")
		})
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
	requireContains(t, "scan stdout", scan.stdout, "Summary: 10000 resources scanned")
	requireContains(t, "plan stdout", plan.stdout, "Summary: 7500 to change")
	if elapsed > 10*time.Second {
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
	requireContains(t, "stdout", r.stdout, "Summary: 1000 to change · 0 conflicts · 500 affected · 500 undecided")
	if elapsed > 10*time.Second {
		t.Fatalf("impact analysis of 500 policies x 1,000 changes took %v (limit 10s)", elapsed)
	}
	t.Logf("impact analysis of 500 policies x 1,000 changes: %v", elapsed)
}
