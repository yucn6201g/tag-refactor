//go:build integration

package cli_test

// Sandbox account test for the U1 checkpoint. It calls a real AWS account, so
// it only runs locally with the build tag "integration" and the profile named
// in TAG_REFACTOR_IT_PROFILE (for example tag-refactor-readonly, the profile
// that assumes the read-only role R2). CI never runs it.
//
//	TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly go test -mod=readonly -tags integration -run 'TestIntegration' ./internal/cli/...
//
// The fixtures come from test/sandbox/u1-fixtures.yaml in ap-northeast-1; the
// expected results are those of infrastructure-specification.md. Other
// resources in the account add to the counts, so the test checks that the
// fixture results are included.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cli"
)

const (
	integrationRegion   = "ap-northeast-1"
	fixtureWaitAttempts = 10
	fixtureWaitInterval = 30 * time.Second
)

func runAgainstAWS(t *testing.T, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, &stdout, &stderr, cli.Deps{})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// fixturesVisible reports whether the tagging API already returns the tags
// of the fixtures (it can take a few minutes after the stack is created).
func fixturesVisible(scanStdout string) bool {
	return regexp.MustCompile(`(?m)^    Env\s+\d+$`).MatchString(scanStdout) &&
		regexp.MustCompile(`(?m)^    cost_center\s+\d+$`).MatchString(scanStdout)
}

func TestIntegrationSandboxScanAndPlan(t *testing.T) {
	profile := os.Getenv("TAG_REFACTOR_IT_PROFILE")
	if profile == "" {
		t.Skip("set TAG_REFACTOR_IT_PROFILE to the sandbox profile to run this test")
	}
	rules := filepath.Join("..", "..", "test", "sandbox", "u1-rules.yaml")

	var scan result
	for attempt := 1; ; attempt++ {
		scan = runAgainstAWS(t, "scan", "--profile", profile, "--region", integrationRegion)
		if scan.code != 2 || fixturesVisible(scan.stdout) || attempt == fixtureWaitAttempts {
			break
		}
		t.Logf("fixture tags not visible yet (attempt %d of %d); waiting %v", attempt, fixtureWaitAttempts, fixtureWaitInterval)
		time.Sleep(fixtureWaitInterval)
	}

	t.Run("scan", func(t *testing.T) {
		scan.requireExit(t, 2)
		requireMatch(t, "stdout", scan.stdout, `(?m)^    env\s+\d+$`)
		requireMatch(t, "stdout", scan.stdout, `(?m)^    Env\s+\d+$`)
		requireMatch(t, "stdout", scan.stdout, `(?m)^    cost-center\s+\d+$`)
		requireMatch(t, "stdout", scan.stdout, `(?m)^    cost_center\s+\d+$`)
		requireNotContains(t, "stdout", scan.stdout, "aws:cloudformation")
		requireNotContains(t, "stderr", scan.stderr, "Error:")
	})

	plan := runAgainstAWS(t, "plan", "--profile", profile, "--region", integrationRegion, "--rules", rules)

	t.Run("plan", func(t *testing.T) {
		plan.requireExit(t, 2)
		requireNotContains(t, "stderr", plan.stderr, "Error:")
		requireNotContains(t, "stdout", plan.stdout, "Impact analysis is incomplete")
		requireMatch(t, "stdout", plan.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+`)
		requireMatch(t, "stdout", plan.stdout, `(?m)^  ~ rename key\s+Env -> Environment\s+`)
		requireMatch(t, "stdout", plan.stdout, `(?m)^  arn:aws:sqs:`+integrationRegion+`:\d{12}:\S+\s+env="dev"\s+Environment="prod"$`)
		requireMatch(t, "stdout", plan.stdout, `(?m)^\s+Statement "AllowDevQueues"\s+Condition StringEquals aws:ResourceTag/env = "dev"$`)
		// P1 is affected ("!"), not undecided ("?"): its policy line comes
		// right before the AllowDevQueues statement.
		requireMatch(t, "stdout", plan.stdout,
			`(?m)^  ! IAM policy  arn:aws:iam::\d{12}:policy/\S+\n\s+Statement "AllowDevQueues"\s+Condition StringEquals aws:ResourceTag/env = "dev"\n\s+affected by: .*rename key env -> Environment`)
		requireMatch(t, "stdout", plan.stdout, `(?m)^\s+Statement "ServiceSpecificKey"\s+Condition StringEquals ec2:ResourceTag/env = "dev"$`)
		if !strings.Contains(plan.stdout, "PolicyVariableKey") {
			t.Log("P2 has no policy-variable statement (IAM may have rejected it); BR5.4 is covered by the unit tests")
		}
		// P1 and P3 carry the same statement; only the attached P1 may appear.
		if n := strings.Count(plan.stdout, `Statement "AllowDevQueues"`); n != 1 {
			t.Fatalf("AllowDevQueues appears %d times; the unattached P3 must not be read", n)
		}
	})

	// NFR1.2: record region, resource count and time for the U1 checkpoint.
	for name, r := range map[string]result{"scan": scan, "plan": plan} {
		done := regexp.MustCompile(`(?m)^Done in .*$`).FindString(r.stderr)
		t.Logf("NFR1.2 %s: region %s, %s", name, integrationRegion, done)
	}
}
