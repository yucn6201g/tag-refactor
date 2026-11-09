package cli_test

// Tests of iac --pr from the command line beyond the acceptance criteria
// (v0.2.0 U1), written after the implementation: a step that fails, a check
// that fails, nothing to write, and the help. They use a CloudFormation
// template, so that no synth is needed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NFR4.4, BR4.2: when gh cannot open the pull request after the push, iac stops
// with 1, says what was done, keeps the body, and leaves the working tree on
// the new branch with its commit pushed.
func TestV02U1FailedCreateLeavesWhatWasDone(t *testing.T) {
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenPushed()
	w.gh.FailCreate()
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

	r := w.iac(plan, "--pr")

	r.requireExit(t, 1)
	requireContains(t, "stderr", r.stderr, "Error: the create step of the pull request failed")
	requireContains(t, "stderr", r.stderr, "the steps done are not undone (done: branch, commit, push)")
	requireContains(t, "stderr", r.stderr, "The body of the pull request is in ")
	requireContains(t, "stderr", r.stderr, "Exit code: 1 (the pull request was not opened)")
	branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD"))
	if !strings.HasPrefix(branch, "tag-refactor/iac-") {
		t.Fatalf("the working tree is on %s", branch)
	}
	gitAt(t, "", "--git-dir", w.bare, "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	bodies, err := filepath.Glob(filepath.Join(w.dir, ".git", "tag-refactor", "pr-body-*.md"))
	if err != nil || len(bodies) != 1 {
		t.Fatalf("body files %v, %v", bodies, err)
	}
	if body, _ := os.ReadFile(bodies[0]); !strings.Contains(string(body), "`stacks/orders.yaml` (CloudFormation)") {
		t.Fatalf("body:\n%s", body)
	}
}

// BR4.1: a check that does not pass stops iac before anything is written.
func TestV02U1FailedCheckChangesNothing(t *testing.T) {
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenPushed()
	w.gh.FailAuth()
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

	r := w.iac(plan, "--pr")

	r.requireExit(t, 1)
	requireTwoLineError(t, r)
	requireContains(t, "stderr", r.stderr, "gh is not logged in to github.com")
	requireContains(t, "stderr", r.stderr, "nothing was changed")
	w.requireFile("stacks/orders.yaml", ordersYAML)
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		t.Fatalf("the working tree is on %s", branch)
	}
}

// X1 nothing_written: when the sources show the plan already, no branch and no
// pull request are made, and the exit code is 0.
func TestV02U1NothingToWriteOpensNothing(t *testing.T) {
	w := newPRWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAMLAfter)
	w.givenPushed()
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

	r := w.iac(plan, "--pr")

	r.requireExit(t, 0)
	requireContains(t, "stderr", r.stderr, "No file was written, so no pull request was opened.")
	if _, created := w.gh.Find("pr", "create"); created {
		t.Fatal("gh pr create was called")
	}
	if branch := strings.TrimSpace(w.out("symbolic-ref", "--short", "HEAD")); branch != "main" {
		t.Fatalf("the working tree is on %s", branch)
	}
}

// X1 and security design §7: the help says what --pr does, what CDK apps are
// handled, and that synth runs the code of the repository.
func TestV02U1Help(t *testing.T) {
	w := newIacWorld(t)

	r := w.run("iac", "--help")

	r.requireExit(t, 0)
	for _, want := range []string{"--pr", "implies --write", "CDK app", "TypeScript", "Tags.of(x).add", "repositories you trust",
		"no AWS and no GitHub credentials", "cdk.context.json", "git commit --only", "a step of --pr failed", "failed or was skipped"} {
		requireContains(t, "stdout", r.stdout, want)
	}
}
