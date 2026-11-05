package cli_test

// Acceptance tests for U8 (iac: the changes of a saved plan reflected in
// CloudFormation templates and Terraform files, as a difference by default and
// in the files with --write). Like the earlier units, each one is an
// acceptance criterion written as Given/When/Then and run from the command
// entry point (cli.Run). The fake AWS stands there only to show that iac never
// calls it. The files are in a temporary folder, and the git repositories are
// temporary ones made inside the tests.
//
// These tests were written and committed before the U8 implementation (team
// testing posture: acceptance criteria first).

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

const (
	arnOrders  = "arn:aws:sqs:ap-northeast-1:111122223333:orders"
	arnBilling = "arn:aws:sqs:ap-northeast-1:111122223333:billing"
	arnOther   = "arn:aws:sqs:ap-northeast-1:111122223333:not-in-iac"
)

// ordersYAML is the template of the functional specification, with values only
// (the !Ref of the example has a test of its own).
const ordersYAML = `Resources:
  OrdersQueue:
    Type: AWS::SQS::Queue
    Properties:
      Tags:
        - Key: env          # 環境
          Value: Prod
        - Key: obsolete
          Value: x
        - Key: Owner
          Value: team-a
`

const ordersYAMLAfter = `Resources:
  OrdersQueue:
    Type: AWS::SQS::Queue
    Properties:
      Tags:
        - Key: Environment          # 環境
          Value: prod
        - Key: Owner
          Value: team-a
        - Key: CostCenter
          Value: "100"
`

const ordersJSON = `{
  "Resources": {
    "OrdersQueue": {
      "Type": "AWS::SQS::Queue",
      "Properties": {
        "Tags": [
          {"Key": "env", "Value": "Prod"},
          {"Key": "obsolete", "Value": "x"},
          {"Key": "Owner", "Value": "team-a"}
        ]
      }
    }
  }
}
`

const ordersJSONAfter = `{
  "Resources": {
    "OrdersQueue": {
      "Type": "AWS::SQS::Queue",
      "Properties": {
        "Tags": [
          {"Key": "Environment", "Value": "prod"},
          {"Key": "Owner", "Value": "team-a"},
          {"Key": "CostCenter", "Value": "100"}
        ]
      }
    }
  }
}
`

const ordersTF = `resource "aws_sqs_queue" "orders" {
  name = "orders"
  tags = {
    env      = "Prod"
    obsolete = "x"
  }
}
`

const ordersTFAfter = `resource "aws_sqs_queue" "orders" {
  name = "orders"
  tags = {
    Environment      = "prod"
    CostCenter = "100"
  }
}
`

// iacWorld is the Given part of the U8 tests: a folder of IaC sources, a plan
// and the fake AWS that must never be called.
type iacWorld struct {
	*world
	dir string
}

func newIacWorld(t *testing.T) *iacWorld {
	t.Helper()
	return &iacWorld{world: newWorld(t), dir: t.TempDir()}
}

func (w *iacWorld) givenFile(rel, content string) string {
	w.t.Helper()
	path := filepath.Join(w.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

func (w *iacWorld) requireFile(rel, want string) {
	w.t.Helper()
	got, err := os.ReadFile(filepath.Join(w.dir, filepath.FromSlash(rel)))
	if err != nil {
		w.t.Fatal(err)
	}
	if string(got) != want {
		w.t.Fatalf("%s is\n%s\nwant\n%s", rel, got, want)
	}
}

// git runs git in the folder of the world without the user's settings.
func (w *iacWorld) git(args ...string) {
	w.t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		w.t.Skip("git is not installed")
	}
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = w.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		w.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// givenCommitted makes the folder a repository with its files committed.
func (w *iacWorld) givenCommitted() {
	w.t.Helper()
	w.git("init", "-q")
	w.git("add", "-A")
	w.git("commit", "-q", "-m", "sources")
}

// changesOfTheExample are the four changes of the functional specification.
func changesOfTheExample() []map[string]any {
	return []map[string]any{
		{"changeId": "c-000001", "kind": "renameKey", "fromKey": "env", "toKey": "Environment", "fromValue": "Prod", "mergesIntoExisting": false, "impacted": false},
		{"changeId": "c-000002", "kind": "changeValue", "fromKey": "Environment", "fromValue": "Prod", "toValue": "prod", "impacted": false},
		{"changeId": "c-000003", "kind": "addRequired", "toKey": "CostCenter", "toValue": "100", "impacted": false},
		{"changeId": "c-000004", "kind": "removeKey", "fromKey": "obsolete", "fromValue": "x", "impacted": false},
	}
}

func beforeOfTheExample(extra map[string]string) map[string]string {
	tags := map[string]string{"env": "Prod", "obsolete": "x", "Owner": "team-a"}
	for k, v := range extra {
		tags[k] = v
	}
	return tags
}

func cloudFormationTags(stack, logicalID string) map[string]string {
	return map[string]string{"aws:cloudformation:stack-name": stack, "aws:cloudformation:logical-id": logicalID}
}

func planResource(arn string, before map[string]string, changes []map[string]any) map[string]any {
	return map[string]any{"arn": arn, "region": "ap-northeast-1", "global": false, "resourceType": "sqs",
		"beforeTags": before, "iacManagedBy": nil, "changes": changes}
}

// givenPlan saves a plan of the resources outside the sources.
func (w *iacWorld) givenPlan(resources ...map[string]any) string {
	w.t.Helper()
	data, err := json.Marshal(map[string]any{"version": 1, "toolVersion": "test", "planId": "p", "accountId": "111122223333",
		"createdAt": "2026-10-31T00:00:00Z", "regions": []string{"ap-northeast-1"}, "inventoryComplete": true,
		"impactAnalysisComplete": true, "ruleFile": "r.yaml", "resources": resources})
	if err != nil {
		w.t.Fatal(err)
	}
	path := filepath.Join(w.t.TempDir(), "plan.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

// givenState saves the output of terraform show -json with the addresses and
// ARNs given as pairs.
func (w *iacWorld) givenState(pairs ...string) string {
	w.t.Helper()
	resources := []map[string]any{}
	for i := 0; i < len(pairs); i += 2 {
		resources = append(resources, map[string]any{"address": pairs[i], "mode": "managed", "type": strings.SplitN(pairs[i], ".", 2)[0],
			"name": "x", "values": map[string]any{"arn": pairs[i+1], "id": "x"}})
	}
	data, err := json.Marshal(map[string]any{"format_version": "1.0", "terraform_version": "1.9.0",
		"values": map[string]any{"root_module": map[string]any{"resources": resources}}})
	if err != nil {
		w.t.Fatal(err)
	}
	path := filepath.Join(w.t.TempDir(), "state.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

func (w *iacWorld) iac(plan string, args ...string) result {
	w.t.Helper()
	r := w.run(append([]string{"iac", "--lang", "en", "--plan", plan, "--dir", w.dir}, args...)...)
	if n := len(w.fake.Calls()); n != 0 {
		w.t.Fatalf("iac called AWS %d times: %v", n, w.fake.Ops())
	}
	return r
}

// AC7.1.2, without --write: Given a CloudFormation template that writes the
// tags as strings, When iac runs without --write, Then the difference is on
// standard output and the file does not change.
func TestAcceptanceU8TemplateDifferenceWithoutWrite(t *testing.T) {
	w := newIacWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))

	r := w.iac(plan)

	r.requireExit(t, 0)
	w.requireFile("stacks/orders.yaml", ordersYAML)
	requireContains(t, "stdout", r.stdout, "--- a/stacks/orders.yaml\n+++ b/stacks/orders.yaml\n@@ ")
	requireContains(t, "stdout", r.stdout, "\n-        - Key: env          # 環境\n")
	requireContains(t, "stdout", r.stdout, "\n+        - Key: Environment          # 環境\n")
	requireContains(t, "stdout", r.stdout, "\n+          Value: \"100\"\n")
	requireContains(t, "stdout", r.stdout, "stacks/orders.yaml: 4 changes can be reflected\n")
	requireContains(t, "stderr", r.stderr, "No file was changed. Add --write to change the files.")
}

// AC7.1.2, with --write: Given committed CloudFormation templates in YAML and
// JSON, When iac runs with --write, Then both templates show the tags after
// the change and nothing else in them changes.
func TestAcceptanceU8TemplatesWritten(t *testing.T) {
	w := newIacWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenFile("json/billing.json", strings.ReplaceAll(ordersJSON, "OrdersQueue", "BillingQueue"))
	w.givenCommitted()
	plan := w.givenPlan(
		planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()),
		planResource(arnBilling, beforeOfTheExample(cloudFormationTags("billing", "BillingQueue")), []map[string]any{
			{"changeId": "c-000011", "kind": "renameKey", "fromKey": "env", "toKey": "Environment", "fromValue": "Prod", "mergesIntoExisting": false, "impacted": false},
			{"changeId": "c-000012", "kind": "changeValue", "fromKey": "Environment", "fromValue": "Prod", "toValue": "prod", "impacted": false},
			{"changeId": "c-000013", "kind": "addRequired", "toKey": "CostCenter", "toValue": "100", "impacted": false},
			{"changeId": "c-000014", "kind": "removeKey", "fromKey": "obsolete", "fromValue": "x", "impacted": false},
		}))

	r := w.iac(plan, "--write")

	r.requireExit(t, 0)
	w.requireFile("stacks/orders.yaml", ordersYAMLAfter)
	w.requireFile("json/billing.json", strings.ReplaceAll(ordersJSONAfter, "OrdersQueue", "BillingQueue"))
	requireContains(t, "stdout", r.stdout, "stacks/orders.yaml: 4 changes reflected\n")
	requireContains(t, "stdout", r.stdout, "json/billing.json: 4 changes reflected\n")
	requireContains(t, "stderr", r.stderr, "Review the difference before you apply the IaC")
	requireNotContains(t, "stderr", r.stderr, "Add --write")
}

// AC7.1.1: Given the output of terraform show -json with the ARN of the
// resource and a committed .tf, When iac runs with --tf-state and --write,
// Then the .tf shows the tags after the change.
func TestAcceptanceU8TerraformWritten(t *testing.T) {
	w := newIacWorld(t)
	w.givenFile("main.tf", ordersTF)
	w.givenCommitted()
	plan := w.givenPlan(planResource(arnOrders, map[string]string{"env": "Prod", "obsolete": "x"}, changesOfTheExample()))
	state := w.givenState("aws_sqs_queue.orders", arnOrders)

	r := w.iac(plan, "--tf-state", state, "--write")

	r.requireExit(t, 0)
	w.requireFile("main.tf", ordersTFAfter)
	requireContains(t, "stdout", r.stdout, "main.tf: 4 changes reflected\n")
}

// AC7.1.1, without --tf-state: the resource counts as not managed by IaC, and
// the message says how to pass the state.
func TestAcceptanceU8TerraformNeedsTheState(t *testing.T) {
	w := newIacWorld(t)
	w.givenFile("main.tf", ordersTF)
	plan := w.givenPlan(planResource(arnOrders, map[string]string{"env": "Prod", "obsolete": "x"}, changesOfTheExample()))

	r := w.iac(plan)

	r.requireExit(t, 0)
	w.requireFile("main.tf", ordersTF)
	requireContains(t, "stdout", r.stdout, "Not managed by IaC: 1 resource")
	requireContains(t, "stderr", r.stderr, "terraform show -json")
	requireContains(t, "stderr", r.stderr, "Nothing was reflected in IaC")
}

// BR6.2: Given a tag whose value is an expression among others, When iac runs
// with --write, Then the others are written, the tag is on the list to fix by
// hand with its file and line, and the exit code is 2.
func TestAcceptanceU8ManualItemsExitTwo(t *testing.T) {
	w := newIacWorld(t)
	template := strings.ReplaceAll(ordersYAML, "Value: team-a", "Value: !Ref OwnerParam")
	w.givenFile("stacks/orders.yaml", template)
	w.givenCommitted()
	changes := append(changesOfTheExample(), map[string]any{"changeId": "c-000005", "kind": "changeValue",
		"fromKey": "Owner", "fromValue": "team-a", "toValue": "team-b", "impacted": false})
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changes))

	r := w.iac(plan, "--write")

	r.requireExit(t, 2)
	w.requireFile("stacks/orders.yaml", strings.ReplaceAll(ordersYAMLAfter, "Value: team-a", "Value: !Ref OwnerParam"))
	requireContains(t, "stdout", r.stdout, "To fix by hand (1):\n")
	requireContains(t, "stdout", r.stdout, arnOrders+"  Owner  stacks/orders.yaml:10  ")
	requireContains(t, "stdout", r.stdout, "the value is an expression")
}

// BR4.3: Given the plan was written already, When iac runs again, Then there
// is no difference, the changes count as already reflected and the exit code
// is 0.
func TestAcceptanceU8SecondRunFindsThemReflected(t *testing.T) {
	w := newIacWorld(t)
	w.givenFile("stacks/orders.yaml", ordersYAML)
	w.givenCommitted()
	plan := w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))
	w.iac(plan, "--write").requireExit(t, 0)
	w.git("add", "-A")
	w.git("commit", "-q", "-m", "reflected")

	r := w.iac(plan, "--write")

	r.requireExit(t, 0)
	w.requireFile("stacks/orders.yaml", ordersYAMLAfter)
	requireNotContains(t, "stdout", r.stdout, "--- a/")
	requireContains(t, "stdout", r.stdout, "already reflected: 4\n")
}

// BR5.2: Given a file to change with a change not committed, or a folder that
// is no repository, When iac runs with --write, Then nothing is written, the
// difference is shown and the exit code is 1.
func TestAcceptanceU8WriteStopsOnUncommittedChanges(t *testing.T) {
	plan := func(w *iacWorld) string {
		return w.givenPlan(planResource(arnOrders, beforeOfTheExample(cloudFormationTags("orders", "OrdersQueue")), changesOfTheExample()))
	}
	t.Run("a change not committed", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenFile("stacks/orders.yaml", ordersYAML)
		w.givenCommitted()
		w.givenFile("stacks/orders.yaml", "# edited by hand\n"+ordersYAML)

		r := w.iac(plan(w), "--write")

		r.requireExit(t, 1)
		w.requireFile("stacks/orders.yaml", "# edited by hand\n"+ordersYAML)
		requireContains(t, "stdout", r.stdout, "--- a/stacks/orders.yaml")
		requireContains(t, "stderr", r.stderr, "stacks/orders.yaml has changes that are not committed")
	})
	t.Run("not a repository", func(t *testing.T) {
		w := newIacWorld(t)
		w.givenFile("stacks/orders.yaml", ordersYAML)

		r := w.iac(plan(w), "--write")

		r.requireExit(t, 1)
		w.requireFile("stacks/orders.yaml", ordersYAML)
		requireContains(t, "stderr", r.stderr, "is not in a git working tree")
		requireContains(t, "stderr", r.stderr, "without --write")
	})
}

// BR1.1, BR1.2, BR1.3: wrong input stops before anything is read, with the two
// lines and exit code 1.
func TestAcceptanceU8InputErrors(t *testing.T) {
	w := newIacWorld(t)
	good := w.givenPlan(planResource(arnOrders, beforeOfTheExample(nil), changesOfTheExample()))
	notAFolder := w.givenFile("file.txt", "x")
	cases := map[string]struct {
		args []string
		what string
	}{
		"no plan":     {[]string{"iac", "--dir", w.dir}, "iac needs --plan"},
		"no dir":      {[]string{"iac", "--plan", good}, "iac needs --dir"},
		"not a dir":   {[]string{"iac", "--plan", good, "--dir", notAFolder}, "is not a folder"},
		"no such dir": {[]string{"iac", "--plan", good, "--dir", filepath.Join(w.dir, "none")}, "is not a folder"},
		"profile":     {[]string{"iac", "--plan", good, "--dir", w.dir, "--profile", "x"}, "unknown flag: --profile"},
		"no plan file": {[]string{"iac", "--plan", filepath.Join(w.dir, "none.json"), "--dir", w.dir},
			"the plan file does not exist"},
		"state file itself": {[]string{"iac", "--plan", good, "--dir", w.dir, "--tf-state", w.givenFile("terraform.tfstate", `{"version":4,"serial":1,"resources":[]}`)},
			"is not the output of terraform show -json"},
		"plan output": {[]string{"iac", "--plan", good, "--dir", w.dir, "--tf-state", w.givenFile("p.json", `{"format_version":"1.0","planned_values":{}}`)},
			"is the output for a plan file"},
		"no state file": {[]string{"iac", "--plan", good, "--dir", w.dir, "--tf-state", filepath.Join(w.dir, "none.json")},
			"does not exist"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := w.run(tc.args...)

			r.requireExit(t, 1)
			requireTwoLineError(t, r)
			requireContains(t, "stderr", r.stderr, tc.what)
			requireContains(t, "stderr", r.stderr, "Exit code: 1 (error)")
		})
	}
	t.Run("a plan of another version", func(t *testing.T) {
		path := w.givenFile("v9.json", `{"version":9,"toolVersion":"v9.0.0"}`)
		r := w.run("iac", "--plan", path, "--dir", w.dir)
		r.requireExit(t, 1)
		requireContains(t, "stderr", r.stderr, "version 9")
	})
}

// BR6.2, NFR3.16: a plan with nothing managed by IaC ends with 0 and asks AWS
// nothing, without any credentials.
func TestAcceptanceU8NothingManagedAndNoAWS(t *testing.T) {
	fixLanguage(t)
	// An AWS environment without any credentials, and a fake AWS that answers nothing.
	w := &iacWorld{world: &world{t: t, env: testutil.IsolateAWSEnv(t), fake: testutil.NewFakeAWS(t)}, dir: t.TempDir()}
	plan := w.givenPlan(planResource(arnOther, map[string]string{"env": "Prod"}, []map[string]any{
		{"changeId": "c-000001", "kind": "changeValue", "fromKey": "env", "fromValue": "Prod", "toValue": "prod", "impacted": false},
	}))

	r := w.iac(plan, "--write")

	r.requireExit(t, 0)
	requireContains(t, "stdout", r.stdout, "Not managed by IaC: 1 resource")
	requireNotContains(t, "stdout", r.stdout, "--- a/")
	if len(w.fake.Calls()) != 0 {
		t.Fatalf("AWS was called: %v", w.fake.Ops())
	}
}

// BR6.3: the help says what the options do, what is left to do by hand, the
// exit codes, and what iac does not do yet.
func TestAcceptanceU8Help(t *testing.T) {
	w := newIacWorld(t)

	r := w.run("iac", "--help")

	r.requireExit(t, 0)
	for _, want := range []string{"--plan", "--dir", "--tf-state", "--write", "terraform show -json", "root module",
		"git", "by hand", "Exit codes:", "  0 ", "  1 ", "  2 ", "CDK", "pull request"} {
		requireContains(t, "stdout", r.stdout, want)
	}
	requireNotContains(t, "stdout", r.stdout, "--profile")
}
