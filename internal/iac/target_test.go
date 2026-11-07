package iac

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/restype"
)

// Unit tests of the input and the targets (BR1.1 to BR2.4), written after the
// implementation.

func requireDetail(t *testing.T, err error, id string) {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok || e.DetailID != id {
		t.Fatalf("err = %#v, want the message %s", err, id)
	}
}

func TestPlanThatIsMissingOrOfAnotherVersionStops(t *testing.T) {
	f := newFixture(t)
	_, err := New().Run(context.Background(), Request{PlanPath: filepath.Join(f.dir, "none.json"), Dir: f.dir})
	if errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("a missing plan: %v", err)
	}
	other := f.file("v9.json", `{"version":9,"toolVersion":"v9"}`)
	_, err = New().Run(context.Background(), Request{PlanPath: other, Dir: f.dir})
	if errkind.KindOf(err) != errkind.UnsupportedVersion {
		t.Fatalf("a plan of another version: %v", err)
	}
	broken := f.file("broken.json", `{"version":`)
	_, err = New().Run(context.Background(), Request{PlanPath: broken, Dir: f.dir})
	if errkind.KindOf(err) != errkind.InvalidInput {
		t.Fatalf("a broken plan: %v", err)
	}
}

func TestPlanLargerThanTheLimitOfIacIsNotRead(t *testing.T) {
	f := newFixture(t)
	maxPlanBytes = 10
	t.Cleanup(func() { maxPlanBytes = 200 << 20 })
	_, err := New().Run(context.Background(), Request{PlanPath: f.plan(), Dir: f.dir})
	requireDetail(t, err, "iac.planTooLarge")
}

// BR4.4: the loader of U4 refuses a plan that touches an aws: key, so iac never
// gets one to write.
func TestPlanThatTouchesAWSKeysIsRefused(t *testing.T) {
	f := newFixture(t)
	plan := f.plan(stackResource(arnQ1, "Q1", remove("aws:createdBy", "x")))
	_, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: f.dir})
	if errkind.KindOf(err) != errkind.InvalidInput {
		t.Fatalf("err = %v", err)
	}
}

func TestDirMustBeAFolder(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.txt", "x")
	for _, dir := range []string{path, filepath.Join(f.dir, "none")} {
		_, err := New().Run(context.Background(), Request{PlanPath: f.plan(), Dir: dir})
		requireDetail(t, err, "iac.notFolder")
	}
}

func stateFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStateIsReadWithModulesAndOnlyManagedResourcesWithARNs(t *testing.T) {
	// The form of terraform show -json in the Terraform documentation (JSON
	// Output Format, "State Representation"): values.root_module with resources
	// and child_modules, nested.
	path := stateFile(t, `{"format_version":"1.0","terraform_version":"1.9.0","values":{"outputs":{"o":{"value":"secret","sensitive":true}},
	"root_module":{"resources":[
	  {"address":"aws_sqs_queue.a","mode":"managed","type":"aws_sqs_queue","name":"a","values":{"arn":"arn:a","policy":"{\"big\":\"value\"}"},"sensitive_values":{}},
	  {"address":"data.aws_sqs_queue.d","mode":"data","type":"aws_sqs_queue","name":"d","values":{"arn":"arn:d"}},
	  {"address":"aws_thing.noarn","mode":"managed","type":"aws_thing","name":"noarn","values":{"id":"x"}},
	  {"address":"aws_thing.notstring","mode":"managed","type":"aws_thing","name":"n","values":{"arn":7}}],
	"child_modules":[{"address":"module.m","resources":[{"address":"module.m.aws_sqs_queue.b","mode":"managed","type":"aws_sqs_queue","name":"b","values":{"arn":"arn:b"}}],
	  "child_modules":[{"address":"module.m.module.n","resources":[{"address":"module.m.module.n.aws_sqs_queue.c","mode":"managed","type":"aws_sqs_queue","name":"c","values":{"arn":"arn:c"}}]}]}]}}}`)

	index, err := readTFState(path)

	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 3 || index["arn:a"][0] != "aws_sqs_queue.a" || index["arn:b"][0] != "module.m.aws_sqs_queue.b" ||
		index["arn:c"][0] != "module.m.module.n.aws_sqs_queue.c" {
		t.Fatalf("index = %v", index)
	}
}

func TestStateWithoutValuesIsAnEmptyState(t *testing.T) {
	for _, content := range []string{`{"format_version":"1.0"}`, `{"format_version":"1.0","values":null}`,
		`{"format_version":"1.0","values":{"root_module":{}}}`} {
		index, err := readTFState(stateFile(t, content))
		if err != nil || len(index) != 0 {
			t.Fatalf("%s: index %v, err %v", content, index, err)
		}
	}
}

func TestStateErrorsSayWhatToPass(t *testing.T) {
	cases := map[string]struct{ content, id string }{
		"the state file itself": {`{"version":4,"terraform_version":"1.9","resources":[]}`, "iac.stateNotOutput"},
		"not JSON":              {`terraform show`, "iac.stateNotOutput"},
		"a list":                {`[]`, "iac.stateNotOutput"},
		"values of a wrong kind": {`{"format_version":"1.0","values":{"root_module":{"resources":[{"address":7}]}}}`,
			"iac.stateNotOutput"},
		"plan values":      {`{"format_version":"1.0","planned_values":{}}`, "iac.statePlan"},
		"resource changes": {`{"format_version":"1.0","values":{},"resource_changes":[]}`, "iac.statePlan"},
		"prior state":      {`{"format_version":"1.0","prior_state":{}}`, "iac.statePlan"},
		"UTF-16":           {"\xff\xfe{\x00", "iac.stateUTF16"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := readTFState(stateFile(t, tc.content))
			requireDetail(t, err, tc.id)
			if strings.Contains(err.Error(), "terraform show") {
				t.Fatalf("the error text holds the content: %v", err)
			}
		})
	}
	_, err := readTFState(filepath.Join(t.TempDir(), "none.json"))
	requireDetail(t, err, "iac.stateNotExist")
	if errkind.KindOf(err) != errkind.NotFound {
		t.Fatal(err)
	}
	_, err = readTFState(t.TempDir())
	requireDetail(t, err, "iac.stateUnreadable")
}

func TestStateWithAByteOrderMarkIsRead(t *testing.T) {
	index, err := readTFState(stateFile(t, "\xef\xbb\xbf"+`{"format_version":"1.0","values":{"root_module":{"resources":[{"address":"a.b","mode":"managed","values":{"arn":"arn:x"}}]}}}`))
	if err != nil || index["arn:x"][0] != "a.b" {
		t.Fatalf("index %v, err %v", index, err)
	}
}

func TestStateLargerThanTheLimitIsNotRead(t *testing.T) {
	maxTFStateBytes = 5
	t.Cleanup(func() { maxTFStateBytes = 500 << 20 })
	_, err := readTFState(stateFile(t, `{"format_version":"1.0"}`))
	requireDetail(t, err, "iac.stateTooLarge")
}

func TestStateWithTooManyNestedModulesStopsWithoutCrashing(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"format_version":"1.0","values":{"root_module":`)
	const depth = maxStateModuleDepth + 10
	for range depth {
		b.WriteString(`{"child_modules":[`)
	}
	b.WriteString(`{}`)
	for range depth {
		b.WriteString(`]}`)
	}
	b.WriteString(`}}`)
	_, err := readTFState(stateFile(t, b.String()))
	requireDetail(t, err, "iac.stateNotOutput")
}

// The state reader keeps nothing of the other values: a secret in the state
// reaches neither the index nor the error.
func TestStateKeepsOnlyTheAddressAndTheARN(t *testing.T) {
	index, err := readTFState(stateFile(t, `{"format_version":"1.0","values":{"root_module":{"resources":[
	  {"address":"a.b","mode":"managed","values":{"arn":"arn:x","password":"SECRET-VALUE"}}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SECRET-VALUE") {
		t.Fatalf("the index holds the secret: %s", data)
	}
}

func TestTargetsByStackTagsAndState(t *testing.T) {
	f := newFixture(t)
	onlyStack := resource{arn: arnQ1, resourceType: "sqs", changes: []change{remove("a", "1")}, before: map[string]string{stackNameTag: "s"}}
	plain := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:plain", remove("a", "1"))
	inState := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:instate", remove("a", "1"))
	notExact := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:instate2", remove("a", "1"))
	module := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:mod", remove("a", "1"))
	counted := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:count", remove("a", "1"))
	each := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:each", remove("a", "1"))
	twice := tfResource("arn:aws:sqs:ap-northeast-1:111122223333:twice", remove("a", "1"))
	state := f.state("aws_sqs_queue.instate", inState.arn, "module.m.aws_sqs_queue.x", module.arn, "aws_sqs_queue.c[0]", counted.arn,
		`aws_sqs_queue.e["a"]`, each.arn, "aws_sqs_queue.t1", twice.arn, "aws_sqs_queue.t2", twice.arn)
	f.file("main.tf", "resource \"aws_sqs_queue\" \"instate\" {\n  tags = {\n    a = \"1\"\n    b = \"2\"\n  }\n}\n")

	res := f.runState(f.plan(onlyStack, plain, inState, notExact, module, counted, each, twice), state)

	if res.Unmanaged != 2 {
		t.Fatalf("unmanaged = %d, want the plain resource and the one whose ARN only starts the same", res.Unmanaged)
	}
	got := map[string]ManualReason{}
	for _, m := range res.Manual {
		got[m.ARN] = m.Reason
	}
	want := map[string]ManualReason{arnQ1: ReasonTemplateNotFound, module.arn: ReasonInModule, counted.arn: ReasonMultipleInstances,
		each.arn: ReasonMultipleInstances, twice.arn: ReasonAmbiguous}
	for arn, reason := range want {
		if got[arn] != reason {
			t.Errorf("%s: %s, want %s", arn, got[arn], reason)
		}
	}
	if res.Reflected != 1 || res.Resources != 8 {
		t.Fatalf("reflected %d of %d resources", res.Reflected, res.Resources)
	}
}

func TestACloudFormationResourceInTheStateStaysCloudFormation(t *testing.T) {
	f := newFixture(t)
	f.file("main.tf", "resource \"aws_sqs_queue\" \"x\" {\n  tags = {\n    a = \"1\"\n    b = \"2\"\n  }\n}\n")
	f.file("t.yaml", yamlTemplate("Q1", "        - Key: a\n          Value: '1'\n"))
	r := stackResource(arnQ1, "Q1", setValue("a", "1", "9"))

	res := f.runState(f.plan(r), f.state("aws_sqs_queue.x", arnQ1))

	if len(res.Files) != 1 || res.Files[0].File != "t.yaml" {
		t.Fatalf("files = %+v", res.Files)
	}
}

func TestDetailsNameAddressesAndPlaces(t *testing.T) {
	f := newFixture(t)
	module := tfResource(arnQ1, remove("a", "1"))
	res := f.runState(f.plan(module), f.state("module.m.aws_sqs_queue.x", arnQ1))
	if m := res.Manual[0]; m.DetailKind != DetailAddress || m.Detail != "module.m.aws_sqs_queue.x" || m.File != "" {
		t.Fatalf("item = %+v", m)
	}
}

// NFR10.14: the shape of terraform show -json, and every Terraform type of the
// table with values.arn. testdata/README.md says where the example comes from.
func TestStateExampleHasEveryTerraformTypeOfTheTable(t *testing.T) {
	index, err := readTFState(filepath.Join("testdata", "terraform-show-example.json"))
	if err != nil {
		t.Fatal(err)
	}
	addresses := map[string]bool{}
	for _, list := range index {
		for _, a := range list {
			addresses[a] = true
		}
	}
	for _, r := range restype.Rows {
		if !addresses[r.TerraformType+".example"] {
			t.Errorf("no %s with values.arn in the example", r.TerraformType)
		}
	}
	if !addresses["module.logs.aws_s3_bucket.this"] || !addresses["aws_sns_topic.counted[0]"] ||
		!addresses[`aws_iam_policy.each["a"]`] || addresses["data.aws_caller_identity.me"] {
		t.Fatalf("addresses = %v", addresses)
	}
}

func TestListNamesShowsFiveAndHowManyMore(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	if got := listNames(names); got != "a, b, c, d, e, +2" {
		t.Fatal(got)
	}
	if got := listNames(names[:2]); got != "a, b" {
		t.Fatal(got)
	}
}

func TestResultsAreSortedByARNAndChange(t *testing.T) {
	items := []ManualItem{{ARN: "b", ChangeID: "c-1"}, {ARN: "a", ChangeID: "c-2"}, {ARN: "a", ChangeID: "c-1"}}
	sortItems(items)
	if !slices.Equal([]string{items[0].ARN + items[0].ChangeID, items[1].ARN + items[1].ChangeID, items[2].ARN + items[2].ChangeID}, []string{"ac-1", "ac-2", "bc-1"}) {
		t.Fatalf("%v", items)
	}
}
