package iac

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	yaml "go.yaml.in/yaml/v3"
)

// Unit tests of applying the changes, the edits, the quotes and the check after
// the edits (BR4.3 to BR4.6, security design §4, §5), written after the
// implementation.

// runTemplate puts the template in a folder, commits it, reflects the changes on
// resource Q1 with --write, and returns the result and the new content.
func runTemplate(t *testing.T, rel, content string, changes ...change) (*Result, string) {
	t.Helper()
	f := newFixture(t)
	f.file(rel, content)
	plan := f.plan(stackResource(arnQ1, "Q1", changes...))
	noGit(t)
	res := f.runWrite(plan)
	if res.WriteErr != nil {
		t.Fatalf("write stopped: %v", res.WriteErr)
	}
	return res, f.read(rel)
}

// noGit lets the writing go on without git: these tests look at the edits, and
// git_test.go at the check.
func noGit(t *testing.T) {
	t.Helper()
	saved := gitCheck
	gitCheck = func(context.Context, string, []gitFile, *[]string) error { return nil }
	t.Cleanup(func() { gitCheck = saved })
}

// runTF does the same with main.tf and the block aws_sqs_queue.q.
func runTF(t *testing.T, content string, changes ...change) (*Result, string) {
	t.Helper()
	f := newFixture(t)
	f.file("main.tf", content)
	plan := f.plan(tfResource(arnQ1, changes...))
	state := f.state("aws_sqs_queue.q", arnQ1)
	noGit(t)
	res, err := New().Run(t.Context(), Request{PlanPath: plan, Dir: f.dir, TFStatePath: state, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.WriteErr != nil {
		t.Fatalf("write stopped: %v", res.WriteErr)
	}
	return res, f.read("main.tf")
}

func kv(pairs ...string) string {
	var b strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		b.WriteString("        - Key: " + pairs[i] + "\n          Value: " + pairs[i+1] + "\n")
	}
	return b.String()
}

func requireText(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// BR4.3: a change of each kind before, after and in neither state.
func TestChangesBeforeAfterAndOtherwise(t *testing.T) {
	cases := map[string]struct {
		tags       string
		change     change
		reflected  int
		already    int
		assumed    int
		manual     string
		wantInText string
	}{
		"rename, before":          {kv("env", "Prod", "z", "1"), rename("env", "Environment", "Prod"), 1, 0, 0, "", "Key: Environment"},
		"rename, after":           {kv("Environment", "Prod"), rename("env", "Environment", "Prod"), 0, 1, 0, "", "Key: Environment"},
		"rename, neither":         {kv("z", "1"), rename("env", "Environment", "Prod"), 0, 0, 0, "env keyNotInSource", ""},
		"rename, other value":     {kv("env", "Dev"), rename("env", "Environment", "Prod"), 0, 0, 0, "env valueDiffers", ""},
		"rename, both":            {kv("env", "Prod", "Environment", "Prod"), rename("env", "Environment", "Prod"), 0, 0, 0, "env keyAlreadyExists", ""},
		"rename, expression":      {kv("env", "!Ref X"), rename("env", "Environment", "Prod"), 0, 0, 0, "env notLiteral", ""},
		"rename after, any value": {kv("Environment", "!Ref X"), rename("env", "Environment", "Prod"), 0, 1, 0, "", ""},
		"merge, before":           {kv("env", "Prod", "z", "1"), merge("env", "Environment", "Prod"), 1, 0, 0, "", ""},
		"merge, gone":             {kv("z", "1"), merge("env", "Environment", "Prod"), 0, 1, 1, "", ""},
		"merge, other value":      {kv("env", "Dev"), merge("env", "Environment", "Prod"), 0, 0, 0, "env valueDiffers", ""},
		"merge, expression":       {kv("env", "!Ref X"), merge("env", "Environment", "Prod"), 0, 0, 0, "env notLiteral", ""},
		"value, before":           {kv("env", "Prod"), setValue("env", "Prod", "prod"), 1, 0, 0, "", "Value: prod"},
		"value, after":            {kv("env", "prod"), setValue("env", "Prod", "prod"), 0, 1, 0, "", ""},
		"value, other":            {kv("env", "Dev"), setValue("env", "Prod", "prod"), 0, 0, 0, "env valueDiffers", ""},
		"value, no key":           {kv("z", "1"), setValue("env", "Prod", "prod"), 0, 0, 0, "env keyNotInSource", ""},
		"value, expression":       {kv("env", "!Sub x"), setValue("env", "Prod", "prod"), 0, 0, 0, "env notLiteral", ""},
		"add, before":             {kv("z", "1"), addTag("Owner", "a"), 1, 0, 0, "", "Key: Owner"},
		"add, after":              {kv("Owner", "a"), addTag("Owner", "a"), 0, 1, 0, "", ""},
		"add, other value":        {kv("Owner", "b"), addTag("Owner", "a"), 0, 0, 0, "Owner keyAlreadyExists", ""},
		"add, expression":         {kv("Owner", "!Ref X", "z", "1"), addTag("Owner", "a"), 0, 0, 0, "Owner notLiteral", ""},
		"remove, before":          {kv("old", "x", "z", "1"), remove("old", "x"), 1, 0, 0, "", ""},
		"remove, gone":            {kv("z", "1"), remove("old", "x"), 0, 1, 1, "", ""},
		"remove, other value":     {kv("old", "y"), remove("old", "x"), 0, 0, 0, "old valueDiffers", ""},
		"remove, expression":      {kv("old", "!Ref X"), remove("old", "x"), 0, 0, 0, "old notLiteral", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tc.tags), tc.change)

			if res.Reflected != tc.reflected || res.AlreadyReflected != tc.already || len(res.AssumedRemovals) != tc.assumed {
				t.Fatalf("reflected %d, already %d, assumed %d", res.Reflected, res.AlreadyReflected, len(res.AssumedRemovals))
			}
			if got := strings.Join(reasons(res), "|"); got != tc.manual {
				t.Fatalf("manual = %q, want %q", got, tc.manual)
			}
			if !strings.Contains(text, tc.wantInText) {
				t.Fatalf("the file has no %q:\n%s", tc.wantInText, text)
			}
		})
	}
}

// BR4.3: the example of the rule: two aliases of one key and a value.
func TestAliasesGoInOrderAndTheSecondRunFindsThemReflected(t *testing.T) {
	changes := []change{rename("env", "Environment", "Prod"), merge("env2", "Environment", "Prod"), setValue("Environment", "Prod", "prod")}
	f := newFixture(t)
	f.file("t.yaml", yamlTemplate("Q1", kv("env", "Prod", "env2", "Prod", "z", "1")))
	plan := f.plan(stackResource(arnQ1, "Q1", changes...))
	f.commit()

	res := f.runWrite(plan)

	if res.Reflected != 3 {
		t.Fatalf("reflected = %d: %v", res.Reflected, reasons(res))
	}
	want := yamlTemplate("Q1", kv("Environment", "prod", "z", "1"))
	requireText(t, f.read("t.yaml"), want)

	f.git("add", "-A")
	f.git("commit", "-q", "-m", "x")
	res = f.runWrite(plan)
	if res.Reflected != 0 || res.AlreadyReflected != 3 || len(res.Files) != 0 || len(res.Manual) != 0 {
		t.Fatalf("second run = %+v", res)
	}
	if len(res.AssumedRemovals) != 1 || res.AssumedRemovals[0].Key != "env2" {
		t.Fatalf("assumed = %+v", res.AssumedRemovals)
	}
}

func TestMergeDoesNotNeedTheKeyItMergesInto(t *testing.T) {
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", kv("env2", "Prod", "z", "1")), merge("env2", "Environment", "Prod"))
	if res.Reflected != 1 || len(res.Manual) != 0 {
		t.Fatalf("result = %+v", res)
	}
	requireText(t, text, yamlTemplate("Q1", kv("z", "1")))
}

// A manual change does not stop the others of the resource.
func TestOneManualChangeLeavesTheOthers(t *testing.T) {
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", kv("a", "!Ref X", "b", "1")), setValue("a", "1", "2"), setValue("b", "1", "2"))
	requireReasons(t, res, "a notLiteral")
	if res.Reflected != 1 || !strings.Contains(text, "Key: b\n          Value: \"2\"") {
		t.Fatalf("result = %+v\n%s", res, text)
	}
	if res.Manual[0].Line != 6 || res.Manual[0].File != "t.yaml" {
		t.Fatalf("place of the item = %+v", res.Manual[0])
	}
}

func TestChangesOfAWholePlaceThatIsAnExpression(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: !Ref T\n")
	res := f.run(f.plan(stackResource(arnQ1, "Q1", setValue("a", "1", "2"), addTag("b", "c"))))
	requireReasons(t, res, "a tagsNotLiteral", "b tagsNotLiteral")
	if res.Manual[0].File != "t.yaml" || res.Manual[0].Line != 2 {
		t.Fatalf("place = %+v", res.Manual[0])
	}
}

func TestAddNeedsAPlaceAndAFormToCopy(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want string
	}{
		"no tags":        {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      A: 1\n", "x noTagSite"},
		"no properties":  {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n", "x noTagSite"},
		"empty list":     {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: []\n", "x noTagSite"},
		"empty map":      {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: {}\n", "x noTagSite"},
		"one line":       {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: [{Key: a, Value: b}]\n", "x unsupportedTagShape"},
		"extra fields":   {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: b\n          PropagateAtLaunch: true\n", "x unsupportedTagShape"},
		"all with notes": {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a # note\n          Value: b\n", "x unsupportedTagShape"},
		"all expression": {"Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: !Ref B\n", "x unsupportedTagShape"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.file("t.yaml", tc.text)
			res := f.run(f.plan(stackResource(arnQ1, "Q1", addTag("x", "y"))))
			requireReasons(t, res, tc.want)
		})
	}
}

func TestExtraFieldsTakeChangesAndRemovalsButNoAddition(t *testing.T) {
	asg := "        - Key: a\n          Value: b\n          PropagateAtLaunch: true\n        - Key: c\n          Value: d\n          PropagateAtLaunch: false\n"
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", asg), setValue("a", "b", "z"), remove("c", "d"), addTag("x", "y"))
	requireReasons(t, res, "x unsupportedTagShape")
	requireText(t, text, yamlTemplate("Q1", "        - Key: a\n          Value: z\n          PropagateAtLaunch: true\n"))
}

// BR4.4: in one line only keys and values change.
func TestOneLineFormsTakeOnlyChangesOfKeysAndValues(t *testing.T) {
	changes := func() []change {
		return []change{rename("a", "A", "1"), setValue("b", "2", "3"), remove("c", "4"), addTag("d", "5")}
	}
	want := []string{"c unsupportedTagShape", "d unsupportedTagShape"}

	res, text := runTemplate(t, "t.yaml", "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: [{Key: a, Value: 1}, {Key: b, Value: 2}, {Key: c, Value: 4}]\n", changes()...)
	requireReasons(t, res, want...)
	requireText(t, text, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: [{Key: A, Value: 1}, {Key: b, Value: \"3\"}, {Key: c, Value: 4}]\n")

	res, text = runTemplate(t, "t.yaml", "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: {a: 1, b: 2, c: 4}\n", changes()...)
	requireReasons(t, res, want...)
	requireText(t, text, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: {A: 1, b: \"3\", c: 4}\n")

	res, text = runTemplate(t, "t.json", jsonTemplate(`[{"Key": "a", "Value": "1"}, {"Key": "b", "Value": "2"}, {"Key": "c", "Value": "4"}]`), changes()...)
	requireReasons(t, res, want...)
	requireText(t, text, jsonTemplate(`[{"Key": "A", "Value": "1"}, {"Key": "b", "Value": "3"}, {"Key": "c", "Value": "4"}]`))

	res, text = runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tags = { a = \"1\", b = \"2\", c = \"4\" }\n}\n", changes()...)
	requireReasons(t, res, want...)
	requireText(t, text, "resource \"aws_sqs_queue\" \"q\" {\n  tags = { A = \"1\", b = \"3\", c = \"4\" }\n}\n")
}

// Security design §4: comments, a commented-out tag and blank lines between the
// elements stay where they are.
func TestRemovalsAndAdditionsLeaveTextBetweenElements(t *testing.T) {
	tags := "        # first\n        - Key: a\n          Value: '1'\n\n        # - Key: off\n        #   Value: x\n        - Key: b\n          Value: \"2\"\n\n        # last\n        - Key: c\n          Value: 3\n"
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags)+"      # after\n", remove("b", "2"), addTag("d", "4"))
	if res.Reflected != 2 || len(res.Manual) != 0 {
		t.Fatalf("result = %+v", res)
	}
	want := "        # first\n        - Key: a\n          Value: '1'\n\n        # - Key: off\n        #   Value: x\n\n        # last\n        - Key: c\n          Value: 3\n        - Key: d\n          Value: \"4\"\n"
	requireText(t, text, yamlTemplate("Q1", want)+"      # after\n")
}

func TestRemovingTheLastElementLeavesCommentsAndAddsAfterTheLastKept(t *testing.T) {
	tags := kv("a", "1") + "        # note\n" + kv("b", "2")
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), remove("b", "2"), addTag("c", "3"))
	if res.Reflected != 2 {
		t.Fatalf("result = %+v", res)
	}
	requireText(t, text, yamlTemplate("Q1", "        - Key: a\n          Value: 1\n        - Key: c\n          Value: \"3\"\n        # note\n"))
}

func TestAdditionsGoInPlanOrderWithTheFormOfTheCopy(t *testing.T) {
	tags := "        - Value: \"1\"\n          Key: 'a'\n"
	res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), addTag("b c", "it's"), addTag("d", "yes"))
	if res.Reflected != 2 {
		t.Fatalf("result = %+v", res)
	}
	want := tags + "        - Value: \"it's\"\n          Key: 'b c'\n        - Value: \"yes\"\n          Key: 'd'\n"
	requireText(t, text, yamlTemplate("Q1", want))
}

func TestCopySourceSkipsCommentsAndExpressions(t *testing.T) {
	tags := "        - Key: a\n          Value: '1'\n        - Key: b # note\n          Value: 2\n        - Key: c\n          Value: !Ref X\n"
	_, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), addTag("d", "4"))
	requireText(t, text, yamlTemplate("Q1", tags+"        - Key: d\n          Value: '4'\n"))
}

func TestRemovingAllElements(t *testing.T) {
	t.Run("list to empty", func(t *testing.T) {
		res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", kv("a", "1", "b", "2"))+"      Other: 1\n", remove("a", "1"), remove("b", "2"))
		if res.Reflected != 2 {
			t.Fatalf("result = %+v", res)
		}
		requireText(t, text, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: []\n      Other: 1\n")
	})
	t.Run("map to empty", func(t *testing.T) {
		_, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", "        a: 1\n        b: 2\n"), remove("a", "1"), remove("b", "2"))
		requireText(t, text, "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags: {}\n")
	})
	t.Run("list, and one added", func(t *testing.T) {
		_, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", kv("a", "1", "b", "2")), remove("a", "1"), remove("b", "2"), addTag("c", "3"), addTag("d", "e"))
		requireText(t, text, yamlTemplate("Q1", kv("c", `"3"`, "d", "e")))
	})
	t.Run("JSON", func(t *testing.T) {
		raw := jsonTemplate("[\n  {\"Key\": \"a\", \"Value\": \"1\"},\n  {\"Key\": \"b\", \"Value\": \"2\"}\n]")
		_, text := runTemplate(t, "t.json", raw, remove("a", "1"), remove("b", "2"))
		requireText(t, text, jsonTemplate("[]"))
	})
	t.Run("JSON, and one added", func(t *testing.T) {
		raw := jsonTemplate("[\n  {\"Key\": \"a\", \"Value\": \"1\"},\n  {\"Key\": \"b\", \"Value\": \"2\"}\n]")
		_, text := runTemplate(t, "t.json", raw, remove("a", "1"), remove("b", "2"), addTag("c", "3"), addTag("d", "4"))
		requireText(t, text, jsonTemplate("[\n  {\"Key\": \"c\", \"Value\": \"3\"},\n  {\"Key\": \"d\", \"Value\": \"4\"}\n]"))
	})
	t.Run("JSON map", func(t *testing.T) {
		raw := jsonTemplate("{\n  \"a\": \"1\"\n}")
		_, text := runTemplate(t, "t.json", raw, remove("a", "1"))
		requireText(t, text, jsonTemplate("{}"))
	})
	t.Run("Terraform", func(t *testing.T) {
		_, text := runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    a = \"1\"\n    b = \"2\"\n  }\n}\n", remove("a", "1"), remove("b", "2"))
		requireText(t, text, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {}\n}\n")
	})
	t.Run("Terraform, and one added", func(t *testing.T) {
		_, text := runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    a = \"1\"\n  }\n}\n", remove("a", "1"), addTag("b", "2"))
		requireText(t, text, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    b = \"2\"\n  }\n}\n")
	})
	t.Run("a comment in the place stops removals and additions", func(t *testing.T) {
		tags := kv("a", "1") + "        # note\n" + kv("b", "2")
		res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), remove("a", "1"), remove("b", "2"), addTag("c", "3"))
		requireReasons(t, res, "a unsupportedTagShape", "b unsupportedTagShape", "c unsupportedTagShape")
		requireText(t, text, yamlTemplate("Q1", tags))
	})
	t.Run("a comment in Terraform stops them too", func(t *testing.T) {
		raw := "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    # note\n    a = \"1\"\n  }\n}\n"
		res, text := runTF(t, raw, remove("a", "1"))
		requireReasons(t, res, "a unsupportedTagShape")
		requireText(t, text, raw)
	})
	t.Run("the keys and values still change", func(t *testing.T) {
		tags := kv("a", "1") + "\n" + kv("b", "2")
		res, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), remove("a", "1"), rename("b", "B", "2"), remove("B", "2"))
		if res.Reflected == 0 {
			t.Fatalf("result = %+v\n%s", res, text)
		}
	})
}

func TestJSONRemovalsAndAdditionsKeepItValid(t *testing.T) {
	elems := func(names ...string) string {
		parts := make([]string, 0, len(names))
		for _, n := range names {
			parts = append(parts, `    {"Key": "`+n+`", "Value": "v"}`)
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n  ]"
	}
	cases := map[string]struct {
		changes func() []change
		want    string
	}{
		"first":            {func() []change { return []change{remove("a", "v")} }, elems("b", "c", "d")},
		"middle":           {func() []change { return []change{remove("b", "v")} }, elems("a", "c", "d")},
		"last":             {func() []change { return []change{remove("d", "v")} }, elems("a", "b", "c")},
		"last two":         {func() []change { return []change{remove("c", "v"), remove("d", "v")} }, elems("a", "b")},
		"first two":        {func() []change { return []change{remove("a", "v"), remove("b", "v")} }, elems("c", "d")},
		"every other":      {func() []change { return []change{remove("a", "v"), remove("c", "v")} }, elems("b", "d")},
		"one out, one in":  {func() []change { return []change{remove("b", "v"), addTag("x", "v")} }, elems("a", "c", "d", "x")},
		"last out, one in": {func() []change { return []change{remove("d", "v"), addTag("x", "v")} }, elems("a", "b", "c", "x")},
		"last two, two in": {func() []change {
			return []change{remove("c", "v"), remove("d", "v"), addTag("x", "v"), addTag("y", "v")}
		}, elems("a", "b", "x", "y")},
		"only in":            {func() []change { return []change{addTag("x", "v")} }, elems("a", "b", "c", "d", "x")},
		"rename and last in": {func() []change { return []change{rename("d", "D", "v"), addTag("x", "v")} }, elems("a", "b", "c", "D", "x")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, text := runTemplate(t, "t.json", jsonTemplate(elems("a", "b", "c", "d")), tc.changes()...)
			if len(res.Manual) != 0 {
				t.Fatalf("manual = %v", reasons(res))
			}
			requireText(t, text, jsonTemplate(tc.want))
			var check any
			if err := json.Unmarshal([]byte(text), &check); err != nil {
				t.Fatalf("not JSON: %v", err)
			}
		})
	}
}

func TestJSONFirstElementOnTheLineOfTheBracket(t *testing.T) {
	raw := jsonTemplate("[{\n  \"Key\": \"a\",\n  \"Value\": \"b\"\n}]")
	_, text := runTemplate(t, "t.json", raw, addTag("c", "d"))
	requireText(t, text, jsonTemplate("[{\n  \"Key\": \"a\",\n  \"Value\": \"b\"\n}, {\n  \"Key\": \"c\",\n  \"Value\": \"d\"\n}]"))
}

func TestJSONMapRemovalsAndAdditions(t *testing.T) {
	raw := jsonTemplate("{\n  \"a\": \"1\",\n  \"b\": \"2\",\n  \"c\": \"3\"\n}")
	res, text := runTemplate(t, "t.json", raw, remove("c", "3"), rename("a", "A", "1"), addTag("d", "4"))
	if res.Reflected != 3 {
		t.Fatalf("result = %+v", res)
	}
	requireText(t, text, jsonTemplate("{\n  \"A\": \"1\",\n  \"b\": \"2\",\n  \"d\": \"4\"\n}"))
}

func TestTerraformRemovalsAndAdditions(t *testing.T) {
	raw := "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    # note\n    a = \"1\" # own\n\n    \"b\" = \"2\",\n    c = \"3\"\n  }\n}\n"
	res, text := runTF(t, raw, remove("a", "1"), rename("b", "x/y", "2"), rename("c", "C", "3"), addTag("d e", "f${g}"))
	if res.Reflected != 4 {
		t.Fatalf("result = %+v", res)
	}
	requireText(t, text, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    # note\n\n    \"x/y\" = \"2\",\n    C = \"3\"\n    \"d e\" = \"f$${g}\"\n  }\n}\n")
}

func TestTerraformTagBlockAndExpressionsGoToTheList(t *testing.T) {
	res, _ := runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tag {\n    key = \"a\"\n  }\n}\n", setValue("a", "1", "2"))
	requireReasons(t, res, "a unsupportedTagShape")
	res, _ = runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tags = merge(var.a, { a = \"1\" })\n}\n", setValue("a", "1", "2"))
	requireReasons(t, res, "a tagsNotLiteral")
	res, _ = runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  tags = {\n    a = var.x\n  }\n}\n", setValue("a", "1", "2"))
	requireReasons(t, res, "a notLiteral")
	res, _ = runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  name = \"x\"\n}\n", setValue("a", "1", "2"), addTag("b", "c"), remove("d", "e"))
	requireReasons(t, res, "a keyNotInSource", "b noTagSite")
	if len(res.AssumedRemovals) != 1 {
		t.Fatalf("assumed = %+v", res.AssumedRemovals)
	}
}

// NFR3.17: only the ranges of the edits change, in every combination of line
// break, BOM, comment, last line break and Japanese.
func TestBytesOutsideTheEditsStayTheSame(t *testing.T) {
	yamlText := "# コメント\nResources:\n  Other:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: 日本語\n          Value: そのまま\n" +
		"  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      UserData: |\n        echo 日本語\n      Tags:\n        - Key: 環境   # 環境のコメント\n          Value: 本番\n        - Key: b\n          Value: 'c'"
	yamlWant := strings.ReplaceAll(strings.ReplaceAll(yamlText, "Key: 環境", "Key: Environment"), "Value: 本番", `Value: "開発"`)
	jsonText := "{\n  \"Description\": \"日本語\",\n  \"Resources\": {\n    \"Q1\": {\n      \"Type\": \"AWS::SQS::Queue\",\n      \"Properties\": {\n        \"Tags\": [\n          {\"Key\": \"環境\", \"Value\": \"本番\"}\n        ]\n      }\n    }\n  }\n}"
	jsonWant := strings.ReplaceAll(strings.ReplaceAll(jsonText, `"環境"`, `"Environment"`), `"本番"`, `"開発"`)
	hclText := "# コメント\nresource \"aws_sqs_queue\" \"q\" {\n  name = \"日本語\"\n  tags = {\n    環境   = \"本番\" # 環境\n    b = \"c\"\n  }\n}"
	hclWant := strings.ReplaceAll(strings.ReplaceAll(hclText, "環境   =", "Environment   ="), `"本番"`, `"開発"`)
	shapes := map[string]func(string) string{
		"LF, no BOM, no last break": func(s string) string { return s },
		"LF, last break":            func(s string) string { return s + "\n" },
		"CRLF":                      func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") + "\r\n" },
		"CRLF, no last break":       func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") },
		"BOM":                       func(s string) string { return "\xef\xbb\xbf" + s + "\n" },
		"BOM, CRLF":                 func(s string) string { return "\xef\xbb\xbf" + strings.ReplaceAll(s, "\n", "\r\n") + "\r\n" },
		"mixed": func(s string) string {
			return strings.Replace(s, "\n", "\r\n", 3) + "\n"
		},
	}
	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			changes := func() []change {
				return []change{rename("環境", "Environment", "本番"), setValue("Environment", "本番", "開発")}
			}
			_, got := runTemplate(t, "t.yaml", shape(yamlText), changes()...)
			requireText(t, got, shape(yamlWant))
			_, got = runTemplate(t, "t.json", shape(jsonText), changes()...)
			requireText(t, got, shape(jsonWant))
			_, got = runTF(t, shape(hclText), changes()...)
			requireText(t, got, shape(hclWant))
		})
	}
}

// A file without a last line break stays without one when its last element goes
// or a new one comes.
func TestFileWithoutALastLineBreakKeepsIt(t *testing.T) {
	head := "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n"
	cases := map[string]struct {
		tags, want string
		changes    func() []change
	}{
		"add":          {"        - Key: a\n          Value: 1", "        - Key: a\n          Value: 1\n        - Key: b\n          Value: c", func() []change { return []change{addTag("b", "c")} }},
		"remove last":  {kv("a", "1") + "        - Key: b\n          Value: 2", "        - Key: a\n          Value: 1", func() []change { return []change{remove("b", "2")} }},
		"remove last2": {kv("a", "1", "b", "2") + "        - Key: c\n          Value: 3", "        - Key: a\n          Value: 1", func() []change { return []change{remove("b", "2"), remove("c", "3")} }},
		"swap last":    {kv("a", "1") + "        - Key: b\n          Value: 2", "        - Key: a\n          Value: 1\n        - Key: c\n          Value: d", func() []change { return []change{remove("b", "2"), addTag("c", "d")} }},
		"all":          {"        - Key: a\n          Value: 1", "", func() []change { return []change{remove("a", "1")} }},
		"crlf":         {"        - Key: a\r\n          Value: 1\r\n        - Key: b\r\n          Value: 2", "        - Key: a\r\n          Value: 1", func() []change { return []change{remove("b", "2")} }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, got := runTemplate(t, "t.yaml", head+tc.tags, tc.changes()...)
			want := head + tc.want
			if tc.want == "" {
				want = strings.TrimSuffix(head, ":\n") + ": []"
			}
			requireText(t, got, want)
		})
	}
}

// Security design §5: each row of the table. The check is the reader of the
// format: what is written reads back as the same string.
func TestYAMLPlainValuesThatWouldReadAsAnotherType(t *testing.T) {
	mustQuote := []string{"y", "Y", "n", "yes", "YES", "Yes", "no", "on", "ON", "off", "true", "True", "TRUE", "false", "null", "Null", "NULL",
		"100", "007", "1_000", "1.5", "1.", "1e3", "1E3", "1.5e3", "0x1F", "0XFF", "0o17", "0b101", "2026-10-31", "2026-1-1",
		"", "a b", "a:b", "-a", "_a", ".a", "@a", "a#b", "日本語", "a,b", "~", "*a", "&a", "!a", "%a"}
	for _, s := range mustQuote {
		if yamlPlainOK(s) {
			t.Errorf("%q must be quoted", s)
		}
	}
	mustNot := []string{"Prod", "a-b", "a_b", "a.b", "a/b", "a@b", "tag-refactor-fixture-u8", "1a", "2026-10-31x", "nUll", "1.2.3x", "0x", "yes1", "nan", "inf", "E1"}
	for _, s := range mustNot {
		if !yamlPlainOK(s) {
			t.Errorf("%q needs no quotes", s)
		}
	}
}

func TestQuotedTextReadsBackTheSame(t *testing.T) {
	values := []string{"a", "it's", `say "hi"`, `back\slash`, "tab\tin", "line\nbreak", "cr\rhere", "日本語", "${x}", "%{y}", "$${z}", "a: b", "# no",
		"\x01ctl", "\x7fdel", "\u0085nel", "\u2028ls", "\u2029ps", "\ufeffbom", "", " space ", "'", `"`, `\`}
	for _, v := range values {
		for name, written := range map[string]string{"single": yamlSingle(v), "double": yamlDouble(v), "plain or double": writeText(formatYAML, quotePlain, v, false)} {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte("k: "+written+"\n"), &doc); err != nil {
				t.Errorf("YAML %s %q: %s: %v", name, v, written, err)
				continue
			}
			got := doc.Content[0].Content[1]
			if got.Value != v || got.Tag != "!!str" {
				t.Errorf("YAML %s %q: %s reads as %q (%s)", name, v, written, got.Value, got.Tag)
			}
		}
		var fromJSON string
		if err := json.Unmarshal([]byte(writeText(formatJSON, quoteDouble, v, false)), &fromJSON); err != nil || fromJSON != v {
			t.Errorf("JSON %q: %q %v", v, fromJSON, err)
		}
		written := hclText(v)
		expr, diags := hclsyntax.ParseExpression([]byte(written), "x", hcl.InitialPos)
		if diags.HasErrors() {
			t.Errorf("HCL %q: %s: %v", v, written, diags)
			continue
		}
		value, diags := expr.Value(nil)
		if diags.HasErrors() || value.AsString() != v {
			t.Errorf("HCL %q: %s: %v %v", v, written, value, diags)
		}
	}
}

func TestQuotesFollowTheTextTheyReplace(t *testing.T) {
	if got := writeText(formatYAML, quoteSingle, "a", false); got != "'a'" {
		t.Fatal(got)
	}
	if got := writeText(formatYAML, quoteSingle, "a\nb", false); got != `"a\nb"` {
		t.Fatal(got)
	}
	if got := writeText(formatYAML, quoteDouble, "a", false); got != `"a"` {
		t.Fatal(got)
	}
	if got := writeText(formatYAML, quotePlain, "100", false); got != `"100"` {
		t.Fatal(got)
	}
	if got := writeText(formatJSON, quoteDouble, "<a&b>", false); got != `"<a&b>"` {
		t.Fatal(got)
	}
	if got := writeText(formatTerraform, quoteDouble, "a", false); got != `"a"` {
		t.Fatal(got)
	}
	if got := writeText(formatTerraform, quotePlain, "a-b_c", true); got != "a-b_c" {
		t.Fatal(got)
	}
	if got := writeText(formatTerraform, quoteDouble, "a", true); got != `"a"` {
		t.Fatal(got)
	}
	for _, s := range []string{"a/b", "a b", "1a", "for", "null", "true", "false"} {
		if got := writeText(formatTerraform, quotePlain, s, true); got != `"`+s+`"` {
			t.Fatal(got)
		}
	}
}

// The example of the functional specification, with both tools.
func TestTheExampleOfTheSpecification(t *testing.T) {
	changes := func() []change {
		return []change{rename("env", "Environment", "Prod"), setValue("Environment", "Prod", "prod"), addTag("CostCenter", "100"), remove("obsolete", "x")}
	}
	tags := "        - Key: env          # 環境\n          Value: Prod\n        - Key: obsolete\n          Value: x\n        - Key: Owner\n          Value: !Ref OwnerParam\n"
	_, text := runTemplate(t, "t.yaml", yamlTemplate("Q1", tags), changes()...)
	want := "        - Key: Environment          # 環境\n          Value: prod\n        - Key: Owner\n          Value: !Ref OwnerParam\n        - Key: CostCenter\n          Value: \"100\"\n"
	requireText(t, text, yamlTemplate("Q1", want))

	_, text = runTF(t, "resource \"aws_sqs_queue\" \"q\" {\n  name = \"orders\"\n  tags = {\n    env      = \"Prod\"\n    obsolete = \"x\"\n  }\n}\n", changes()...)
	requireText(t, text, "resource \"aws_sqs_queue\" \"q\" {\n  name = \"orders\"\n  tags = {\n    Environment      = \"prod\"\n    CostCenter = \"100\"\n  }\n}\n")
}

// BR4.6: the check after the edits, on purpose wrong.
func TestCheckAfterTheEdits(t *testing.T) {
	raw := yamlTemplate("Q1", kv("a", "1")) + "  Q2:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n" + kv("b", "2")
	ct, _, _ := prepare(&candidate{rel: "x", format: formatYAML}, []byte(raw))
	parsed, _ := parseContent(ct)
	before := parsed.allTags()
	site := parsed.site("Q1")
	expected := []expectedTags{{key: "Q1", final: []simEntry{{orig: 0, key: "a", value: "9", literal: true}}, others: site.others}}
	e := site.entries[0]
	good := []edit{{e.valueSpan.start, e.valueSpan.end, "9"}}

	if why := checkEdits(ct, applyEdits(ct.raw, good), good, expected, before); why != "" {
		t.Fatalf("a good edit: %s", why)
	}
	cases := map[string]struct {
		edits []edit
		want  string
	}{
		"other value":        {[]edit{{e.valueSpan.start, e.valueSpan.end, "8"}}, "(a)"},
		"broken YAML":        {[]edit{{e.valueSpan.start, e.valueSpan.end, "[9"}}, "does not parse"},
		"another resource":   {[]edit{{e.valueSpan.start, e.valueSpan.end, "9"}, {strings.Index(raw, "Value: 2") + 7, strings.Index(raw, "Value: 2") + 8, "7"}}, "(b)"},
		"a resource removed": {[]edit{{e.valueSpan.start, e.valueSpan.end, "9"}, {strings.Index(raw, "  Q2:"), len(raw), ""}}, "(b)"},
		"a resource renamed": {[]edit{{e.valueSpan.start, e.valueSpan.end, "9"}, {strings.Index(raw, "Q2:"), strings.Index(raw, "Q2:") + 2, "Q3"}}, "(b)"},
		"a comment added":    {[]edit{{e.valueSpan.start, e.valueSpan.end, "9"}, {e.elem.end, e.elem.end, "\n        # new"}}, "(d)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			why := checkEdits(ct, applyEdits(ct.raw, tc.edits), tc.edits, expected, before)
			if !strings.Contains(why, tc.want) {
				t.Fatalf("why = %q, want %q", why, tc.want)
			}
		})
	}
	t.Run("bytes outside the edits", func(t *testing.T) {
		updated := applyEdits(ct.raw, good)
		updated[0] = 'X'
		if why := checkEdits(ct, updated, good, expected, before); !strings.Contains(why, "(c)") {
			t.Fatalf("why = %q", why)
		}
	})
	if !sameTags(nil, nil) || sameTags(nil, []simEntry{{}}) {
		t.Fatal("no place equals an empty list")
	}
}

func TestNormalizeJoinsAndRefuses(t *testing.T) {
	got, err := normalize([]edit{{10, 20, ""}, {10, 10, "x"}, {1, 2, "y"}})
	if err != nil || len(got) != 2 || got[0].start != 1 || got[1].start != 10 || got[1].end != 20 || got[1].text != "x" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := normalize([]edit{{1, 5, ""}, {3, 4, "x"}}); err == nil {
		t.Fatal("overlapping edits")
	}
	if _, err := normalize([]edit{{1, 5, "a"}, {1, 4, "b"}}); err == nil {
		t.Fatal("two removals at one place")
	}
}

// A fault of the tool found by the check leaves the file alone and puts the
// changes on the list.
func TestFileThatFailsTheCheckIsLeftAlone(t *testing.T) {
	f := newFixture(t)
	// The value is the empty key of an element that is written twice, so the
	// reader sees another list after the edit than the changes made.
	raw := yamlTemplate("Q1", kv("a", "1", "b", "2"))
	f.file("t.yaml", raw)
	plan := f.plan(stackResource(arnQ1, "Q1", rename("a", "b2", "1"), setValue("b", "2", "3")))
	// A wrong writer of text makes the file not parse.
	saved := writeTextForTest
	writeTextForTest = func(string) string { return "[broken" }
	t.Cleanup(func() { writeTextForTest = saved })

	res := f.run(plan)

	requireReasons(t, res, "a unreadableFile", "b unreadableFile")
	if len(res.Files) != 0 || len(res.Debug) != 1 || !strings.Contains(res.Debug[0], "t.yaml: the file does not parse") {
		t.Fatalf("result = %+v", res)
	}
	if res.ExitCode() != 2 {
		t.Fatalf("exit code = %d", res.ExitCode())
	}
}

// Text a user could plant cannot be taken for a key or a value (position check).
func TestRangesHoldWhatTheParserRead(t *testing.T) {
	raw := yamlTemplate("Q1", "        - Key: \"a\\u0041\"\n          Value: 'b''c'\n")
	site := parseFor(t, formatYAML, raw).site("Q1")
	if e := site.entries[0]; e.key != "aA" || e.value != "b'c" || !regexp.MustCompile(`^"a\\u0041"$`).MatchString(text(raw, e.keySpan)) {
		t.Fatalf("entry = %+v", e)
	}
}
