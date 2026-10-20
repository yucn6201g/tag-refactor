package cli_test

// Acceptance tests for U1 (walking skeleton). Each test is one acceptance
// criterion of the user stories, written as Given/When/Then and run from the
// command entry point (cli.Run) against the HTTP-level fake AWS. The real AWS
// SDK configuration, signing, retries, pagination and error mapping run; only
// the HTTP round trip is faked, and the AWS environment is isolated.
//
// These tests were written and committed before the implementation (team
// testing posture: acceptance criteria first).

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

const (
	arnQA    = "arn:aws:sqs:us-east-1:123456789012:qa"
	arnQB    = "arn:aws:sqs:us-east-1:123456789012:qb"
	arnQC    = "arn:aws:sqs:us-east-1:123456789012:qc"
	arnB1    = "arn:aws:s3:::bucket-b1"
	policyP1 = "arn:aws:iam::123456789012:policy/DevAccess"
	policyP2 = "arn:aws:iam::123456789012:policy/TeamAccess"
)

const aliasRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n"

// world is the Given part shared by the acceptance tests: an isolated AWS
// environment with fake credentials and a fake AWS.
type world struct {
	t    *testing.T
	env  *testutil.AWSEnv
	fake *testutil.FakeAWS
}

type result struct {
	code   int
	stdout string
	stderr string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1", "eu-west-1", "ap-northeast-1")
	return &world{t: t, env: env, fake: fake}
}

func (w *world) givenResources(resources ...testutil.TaggedResource) {
	w.fake.Handle("tag:GetResources", testutil.TagPages(resources))
}

// givenAttachedPolicies makes the given customer managed policies (ARN to
// JSON document) attached and readable.
func (w *world) givenAttachedPolicies(docs map[string]string) {
	arns := make([]string, 0, len(docs))
	for arn := range docs {
		arns = append(arns, arn)
	}
	sort.Strings(arns)
	page := make([]testutil.ManagedPolicy, 0, len(arns))
	for _, arn := range arns {
		page = append(page, testutil.ManagedPolicy{ARN: arn})
	}
	w.fake.Handle("iam:ListPolicies", testutil.PolicyPages(page))
	w.fake.Handle("iam:GetPolicyVersion", testutil.PolicyDocuments(docs))
}

func (w *world) givenRuleFile(content string) string {
	w.t.Helper()
	path := filepath.Join(w.t.TempDir(), "tagrules.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

func (w *world) run(args ...string) result {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	clock := testutil.NewFakeClock(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), 10*time.Millisecond)
	code := cli.Run(context.Background(), args, &stdout, &stderr, cli.Deps{
		Transport:  w.fake,
		Now:        clock.Now,
		RetryDelay: func(int) time.Duration { return 0 },
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func (r result) requireExit(t *testing.T, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", r.code, want, r.stdout, r.stderr)
	}
}

func requireContains(t *testing.T, stream, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%s does not contain %q:\n%s", stream, want, got)
	}
}

func requireMatch(t *testing.T, stream, got, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(got) {
		t.Fatalf("%s does not match %q:\n%s", stream, pattern, got)
	}
}

func requireNotContains(t *testing.T, stream, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("%s contains %q:\n%s", stream, unwanted, got)
	}
}

// requireTwoLineError checks the error format: "Error: <what>" then
// "  Fix: <how>" on standard error, and nothing on standard output.
func requireTwoLineError(t *testing.T, r result) {
	t.Helper()
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: .+\n  Fix: .+$`)
	if r.stdout != "" {
		t.Fatalf("stdout must be empty on errors, got:\n%s", r.stdout)
	}
}

// sandboxResources mirrors the fixture resources of the sandbox account
// (QA, QB, QC and B1 of infrastructure-specification.md).
func sandboxResources() []testutil.TaggedResource {
	return []testutil.TaggedResource{
		{ARN: arnQA, Tags: map[string]string{"env": "dev", "aws:cloudformation:stack-name": "tag-refactor-u1-fixtures"}},
		{ARN: arnQB, Tags: map[string]string{"Env": "dev"}},
		{ARN: arnQC, Tags: map[string]string{"env": "dev", "Environment": "prod"}},
		{ARN: arnB1, Tags: map[string]string{"env": "dev", "cost-center": "a", "cost_center": "b"}},
	}
}

const policyEnvCondition = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowDevEC2","Effect":"Allow","Action":"ec2:StartInstances","Resource":"*",` +
	`"Condition":{"StringEquals":{"aws:ResourceTag/env":"dev"}}}]}`

const policyVariableAndWildcard = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Action":"ec2:StartInstances","Resource":"*","Condition":{"StringEquals":{"aws:ResourceTag/${aws:PrincipalTag/team}":"yes"}}},` +
	`{"Sid":"Wildcard","Effect":"Allow","Action":"ec2:StopInstances","Resource":"*","Condition":{"StringLike":{"aws:ResourceTag/en*":"dev"}}}]}`

// AC1.1.8: Given only resources without problems, When scan runs, Then
// "no issues" and the number of scanned resources are shown.
func TestAcceptance_AC1_1_8_ScanWithoutVariantsReportsNoIssues(t *testing.T) {
	w := newWorld(t)
	w.givenResources(
		testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "prod"}},
		testutil.TaggedResource{ARN: arnQB, Tags: map[string]string{"Environment": "dev", "Owner": "team-a"}},
		testutil.TaggedResource{ARN: arnB1, Tags: map[string]string{}},
	)

	r := w.run("scan", "--region", "us-east-1")

	r.requireExit(t, 0)
	requireContains(t, "stdout", r.stdout, "No issues found")
	requireContains(t, "stdout", r.stdout, "3 resources scanned")
	requireContains(t, "stderr", r.stderr, "Exit code: 0")
}

// AC1.2.1: Given only the read-only permissions, When scan and plan run, Then
// both work without permission errors and no write API is called.
func TestAcceptance_AC1_2_1_ScanAndPlanUseOnlyReadOperations(t *testing.T) {
	w := newWorld(t)
	w.givenResources(sandboxResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(aliasRules)

	scan := w.run("scan", "--region", "us-east-1")
	plan := w.run("plan", "--rules", rules, "--region", "us-east-1")

	scan.requireExit(t, 2)
	plan.requireExit(t, 2)
	requireNotContains(t, "scan stderr", scan.stderr, "Error:")
	requireNotContains(t, "plan stderr", plan.stderr, "Error:")
	for _, op := range w.fake.Ops() {
		if !testutil.IsReadOperation(op) {
			t.Errorf("non-read operation called: %s", op)
		}
	}
	want := []string{"ec2:DescribeRegions", "iam:GetPolicyVersion", "iam:ListPolicies", "sts:GetCallerIdentity", "tag:GetResources"}
	if got := w.fake.Ops(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}

// AC1.2.2: Given a missing read permission, When scan runs, Then the name of
// the missing operation (API) is shown.
func TestAcceptance_AC1_2_2_MissingPermissionNamesTheOperation(t *testing.T) {
	w := newWorld(t)
	w.fake.Handle("tag:GetResources", testutil.Static(testutil.JSONError(http.StatusBadRequest, "AccessDeniedException",
		"User: arn:aws:iam::123456789012:user/tester is not authorized to perform: tag:GetResources")))

	r := w.run("scan", "--region", "us-east-1")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireMatch(t, "stderr", r.stderr, `(?m)^Error: .*tag:GetResources`)
}

// AC1.2.3: Given no AWS credentials, When any subcommand runs, Then what is
// missing and how to set it up are shown, and the exit code is 1.
func TestAcceptance_AC1_2_3_NoCredentialsExplainsSetup(t *testing.T) {
	w := newWorld(t)
	w.env.WriteConfig("")
	w.env.WriteCredentials("")
	rules := w.givenRuleFile(aliasRules)

	for _, args := range [][]string{
		{"scan", "--region", "us-east-1"},
		{"plan", "--rules", rules, "--region", "us-east-1"},
	} {
		r := w.run(args...)

		r.requireExit(t, 1)
		requireTwoLineError(t, r)
		requireMatch(t, "stderr", r.stderr, `(?mi)^Error: .*credentials`)
		requireMatch(t, "stderr", r.stderr, `(?m)^  Fix: .*(aws configure|aws sso login|--profile)`)
	}
	if calls := w.fake.Calls(); len(calls) != 0 {
		t.Fatalf("AWS was called without credentials: %+v", calls)
	}
}

// AC1.2.4: Given --profile names a profile, When scan runs, Then the
// credentials and the region of that profile are used.
func TestAcceptance_AC1_2_4_ProfileSelectsCredentialsAndRegion(t *testing.T) {
	w := newWorld(t)
	w.env.SetProfile("default", testutil.Profile{Region: "eu-west-1", AccessKeyID: "AKIADEFAULTPROFILE00", SecretAccessKey: "default-secret"})
	w.env.SetProfile("dev", testutil.Profile{Region: "ap-northeast-1", AccessKeyID: "AKIADEVPROFILE000000", SecretAccessKey: "dev-secret"})
	w.givenResources(testutil.TaggedResource{ARN: "arn:aws:sqs:ap-northeast-1:123456789012:q", Tags: map[string]string{"Environment": "dev"}})

	r := w.run("scan", "--profile", "dev")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "region ap-northeast-1")
	calls := w.fake.Calls()
	if len(calls) == 0 {
		t.Fatal("no AWS call was made")
	}
	for _, c := range calls {
		if c.AccessKeyID != "AKIADEVPROFILE000000" {
			t.Errorf("%s was signed with %s, want the dev profile key", c.Op, c.AccessKeyID)
		}
		if c.Op == "tag:GetResources" && c.Region != "ap-northeast-1" {
			t.Errorf("tag:GetResources went to %s, want ap-northeast-1", c.Region)
		}
	}
}

// AC1.2.5: Given any subcommand ran, When standard output, standard error and
// the debug log are checked, Then no credentials appear anywhere.
func TestAcceptance_AC1_2_5_CredentialsNeverAppearInOutput(t *testing.T) {
	secretsInMessage := "key " + testutil.FakeAccessKeyID + " secret " + testutil.FakeSecretAccessKey + " token " + testutil.FakeSessionToken
	scenarios := map[string]struct {
		given    func(w *world)
		args     func(rules string) []string
		wantExit int
	}{
		"access denied echoing credentials": {
			given: func(w *world) {
				w.fake.Handle("tag:GetResources", testutil.Static(testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", secretsInMessage)))
			},
			args:     func(string) []string { return []string{"scan", "--debug", "--region", "us-east-1"} },
			wantExit: 1,
		},
		"transient failure after retries": {
			given: func(w *world) {
				w.fake.Handle("sts:GetCallerIdentity", testutil.Static(testutil.QueryError(http.StatusServiceUnavailable, "ServiceUnavailable", secretsInMessage)))
			},
			args:     func(string) []string { return []string{"scan", "--debug", "--region", "us-east-1"} },
			wantExit: 1,
		},
		"unexpected response": {
			given: func(w *world) {
				w.fake.Handle("tag:GetResources", testutil.Static(testutil.Reply{Body: "{not json " + secretsInMessage, ContentType: "application/x-amz-json-1.1"}))
			},
			args:     func(string) []string { return []string{"scan", "--debug", "--region", "us-east-1"} },
			wantExit: 1,
		},
		"successful plan with debug": {
			given: func(w *world) {
				w.givenResources(sandboxResources()...)
				w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
			},
			args: func(rules string) []string {
				return []string{"plan", "--debug", "--rules", rules, "--region", "us-east-1"}
			},
			wantExit: 2,
		},
	}
	for name, sc := range scenarios {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			sc.given(w)
			rules := w.givenRuleFile(aliasRules)

			r := w.run(sc.args(rules)...)

			r.requireExit(t, sc.wantExit)
			for _, secret := range []string{testutil.FakeAccessKeyID, testutil.FakeSecretAccessKey, testutil.FakeSessionToken} {
				requireNotContains(t, "stdout", r.stdout, secret)
				requireNotContains(t, "stderr", r.stderr, secret)
			}
			requireContains(t, "stderr", r.stderr, "debug: ")
		})
	}
}

// AC2.1.2: Given the rule file makes env an alias of Environment, When plan
// runs, Then env is treated as a key rename to Environment.
func TestAcceptance_AC2_1_2_AliasBecomesKeyRename(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.givenAttachedPolicies(map[string]string{})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+1 resources?$`)
	requireContains(t, "stdout", r.stdout, "Summary: 1 to change")
}

// AC3.1.8: Given the plan result is saved from standard output, When the
// saved output is checked, Then no progress or guidance messages are mixed in.
func TestAcceptance_AC3_1_8_PlanStdoutHoldsOnlyResults(t *testing.T) {
	w := newWorld(t)
	w.givenResources(sandboxResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(aliasRules + "required:\n  Owner:\n    default: platform\n")

	r := w.run("plan", "--debug", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	for _, line := range []string{"Account 123456789012", "scan: us-east-1 started", "scan: us-east-1 done", "warning: ", "debug: ", "Done in ", "Exit code: 2"} {
		requireNotContains(t, "stdout", r.stdout, line)
		requireContains(t, "stderr", r.stderr, line)
	}
	requireMatch(t, "stdout", r.stdout, `(?m)^Summary: `)
	requireNotContains(t, "stderr", r.stderr, "Summary: ")
}

// AC3.2.1: Given a policy attached to an IAM role uses the env tag in a
// condition, When a plan renames env to Environment, Then the policy is shown
// as affected (!) with its name, statement and condition.
func TestAcceptance_AC3_2_1_AffectedPolicyIsShown(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! IAM policy\s+`+regexp.QuoteMeta(policyP1)+`$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+Statement "AllowDevEC2"\s+Condition .*aws:ResourceTag/env = "dev"$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+affected by: rename key env -> Environment$`)
	requireContains(t, "stdout", r.stdout, "1 affected")
}

// AC3.2.4: Given policy conditions with a wildcard or a policy variable, When
// plan runs, Then the policy variable is shown as "cannot decide" (?) and
// neither is shown as having no impact.
func TestAcceptance_AC3_2_4_PolicyVariableIsUndecidedAndWildcardIsMatched(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.givenAttachedPolicies(map[string]string{policyP2: policyVariableAndWildcard})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireMatch(t, "stdout", r.stdout, `(?m)^  \? IAM policy\s+`+regexp.QuoteMeta(policyP2)+`$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+Statement "#0"\s+Condition .*aws:ResourceTag/\$\{aws:PrincipalTag/team\}`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+cannot decide \(policy variable\)$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^\s+Statement "Wildcard"\s+Condition .*aws:ResourceTag/en\*`)
	requireContains(t, "stdout", r.stdout, "1 affected · 1 undecided")
}

// AC3.2.5: Given no permission to read policies, When plan runs, Then the
// kind of policy that could not be read and the reason are shown, and the
// impact analysis is reported as incomplete.
func TestAcceptance_AC3_2_5_UnreadablePoliciesMakeImpactIncomplete(t *testing.T) {
	w := newWorld(t)
	w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"env": "dev"}})
	w.fake.Handle("iam:ListPolicies", testutil.Static(testutil.QueryError(http.StatusForbidden, "AccessDenied",
		"User: arn:aws:iam::123456789012:user/tester is not authorized to perform: iam:ListPolicies")))
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 1)
	if !strings.HasPrefix(r.stdout, "! Impact analysis is incomplete\n") {
		t.Fatalf("stdout must start with the incomplete marker:\n%s", r.stdout)
	}
	requireMatch(t, "stdout", r.stdout, `(?m)IAM managed polic.*iam:ListPolicies`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key\s+env -> Environment`)
	requireContains(t, "stderr", r.stderr, "Exit code: 1")
}

// AC6.1.3: Given NO_COLOR is set, When plan runs, Then no colors are printed
// and the change kinds and impacts are readable from symbols and words.
func TestAcceptance_AC6_1_3_SymbolsWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := newWorld(t)
	w.givenResources(sandboxResources()...)
	w.givenAttachedPolicies(map[string]string{policyP1: policyEnvCondition, policyP2: policyVariableAndWildcard})
	rules := w.givenRuleFile(aliasRules)

	r := w.run("plan", "--rules", rules, "--region", "us-east-1")

	r.requireExit(t, 2)
	requireNotContains(t, "stdout", r.stdout, "\x1b")
	requireNotContains(t, "stderr", r.stderr, "\x1b")
	requireMatch(t, "stdout", r.stdout, `(?m)^  ~ rename key `)
	requireMatch(t, "stdout", r.stdout, `(?m)^  ! IAM policy `)
	requireMatch(t, "stdout", r.stdout, `(?m)^  \? IAM policy `)
	requireMatch(t, "stdout", r.stdout, `(?m)^Conflicts \(not changed; values differ\): 1 resources?$`)
	requireMatch(t, "stdout", r.stdout, `(?m)^  `+regexp.QuoteMeta(arnQC)+`\s+env="dev"\s+Environment="prod"$`)
}

// AC6.1.4: Given scan found problems, When the exit code is checked, Then it
// is 2; without problems it is 0, and on errors it is 1.
func TestAcceptance_AC6_1_4_ScanExitCodes(t *testing.T) {
	t.Run("variants found", func(t *testing.T) {
		w := newWorld(t)
		w.givenResources(sandboxResources()...)

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 2)
		requireContains(t, "stderr", r.stderr, "Exit code: 2 (issues found)")
		requireMatch(t, "stdout", r.stdout, `(?m)^  env\s+4 resources$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    env\s+3$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^    Env\s+1$`)
		requireMatch(t, "stdout", r.stdout, `(?m)^  cost-center\s+1 resources?$`)
		requireNotContains(t, "stdout", r.stdout, "aws:cloudformation")
		requireContains(t, "stdout", r.stdout, "Summary: 4 resources scanned in 1 region (us-east-1) · 2 variant groups")
	})
	t.Run("no variants", func(t *testing.T) {
		w := newWorld(t)
		w.givenResources(testutil.TaggedResource{ARN: arnQA, Tags: map[string]string{"Environment": "dev"}})

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 0)
		requireContains(t, "stderr", r.stderr, "Exit code: 0")
	})
	t.Run("error", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Handle("tag:GetResources", testutil.Static(testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "denied")))

		r := w.run("scan", "--region", "us-east-1")

		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "Exit code: 1")
	})
}
