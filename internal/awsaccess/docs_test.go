package awsaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

var (
	policyExamplePath = filepath.Join("..", "..", "docs", "iam", "readonly-policy.json")
	sandboxPath       = filepath.Join("..", "..", "test", "sandbox", "u1-fixtures.yaml")
)

type policyDocument struct {
	Version   string `json:"Version"`
	Statement []struct {
		Effect   string   `json:"Effect"`
		Action   []string `json:"Action"`
		Resource string   `json:"Resource"`
	} `json:"Statement"`
}

func readPolicyExample(t *testing.T) (policyDocument, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(policyExamplePath)
	if err != nil {
		t.Fatal(err)
	}
	var doc policyDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", policyExamplePath, err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	return doc, generic
}

// readActionsByName are the two IAM actions that read without starting with
// Get, List or Describe (NFR3.3). resource-explorer-2:Search is the IAM
// action of ListResources; the API Search itself is never called.
var readActionsByName = map[string]bool{
	"config:SelectResourceConfig": true,
	"resource-explorer-2:Search":  true,
}

// checkReadOnlyAction returns why action is not allowed in the example
// policy, or "" when it is a read without wildcards.
func checkReadOnlyAction(action string) string {
	if strings.Contains(action, "*") {
		return "contains *"
	}
	if readActionsByName[action] {
		return ""
	}
	name := action[strings.Index(action, ":")+1:]
	for _, prefix := range []string{"Get", "List", "Describe"} {
		if strings.HasPrefix(name, prefix) {
			return ""
		}
	}
	return "is not a read (Get, List, Describe, or one of the two allowed by name)"
}

// NFR3.2, NFR3.3: the example policy only reads, and never uses wildcards in
// actions.
func TestReadOnlyPolicyExampleAllowsOnlyReads(t *testing.T) {
	doc, _ := readPolicyExample(t)
	if doc.Version != "2012-10-17" || len(doc.Statement) == 0 {
		t.Fatalf("policy = %+v", doc)
	}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" {
			t.Errorf("unexpected effect %q", st.Effect)
		}
		for _, action := range st.Action {
			if why := checkReadOnlyAction(action); why != "" {
				t.Errorf("action %q %s", action, why)
			}
		}
	}
}

// NFR3.3: only the two actions named in readActionsByName pass by name;
// other Select or Search actions are rejected.
func TestReadOnlyCheckAllowsOnlyTheTwoNamedExceptions(t *testing.T) {
	for action, ok := range map[string]bool{
		"config:SelectResourceConfig": true, "resource-explorer-2:Search": true, "resource-explorer-2:GetView": true,
		"config:SelectAggregateResourceConfig": false, "resource-explorer-2:SearchViews": false,
		"config:StartConfigurationRecorder": false, "resource-explorer-2:CreateIndex": false, "config:Describe*": false,
	} {
		if got := checkReadOnlyAction(action) == ""; got != ok {
			t.Errorf("%s accepted = %v, want %v", action, got, ok)
		}
	}
}

// The example policy and the table of operations in the code must list the
// same permissions, so adding an API call without updating the example fails.
func TestReadOnlyPolicyExampleMatchesTheOperationsTable(t *testing.T) {
	doc, _ := readPolicyExample(t)
	var inDoc []string
	for _, st := range doc.Statement {
		inDoc = append(inDoc, st.Action...)
	}
	inCode := RequiredPermissions()
	sort.Strings(inDoc)
	sort.Strings(inCode)
	if strings.Join(inDoc, ",") != strings.Join(inCode, ",") {
		t.Fatalf("docs/iam/readonly-policy.json allows %v, the code calls %v", inDoc, inCode)
	}
}

// accountPlaceholder is the part of the example policy that the user replaces
// with the account ID.
const accountPlaceholder = "<account-id>"

// iam:GetPolicyVersion is limited to the customer managed policies of the
// account (review R-06); the other actions cannot be limited to resources.
func TestReadOnlyPolicyExampleLimitsPolicyVersionsToTheAccount(t *testing.T) {
	doc, _ := readPolicyExample(t)
	found := false
	for _, st := range doc.Statement {
		if !slices.Contains(st.Action, OpGetPolicyVersion) {
			if st.Resource != "*" {
				t.Errorf("statement for %v has resource %q, want *", st.Action, st.Resource)
			}
			continue
		}
		found = true
		if len(st.Action) != 1 {
			t.Errorf("%s must have its own statement, got %v", OpGetPolicyVersion, st.Action)
		}
		if st.Resource != "arn:aws:iam::"+accountPlaceholder+":policy/*" {
			t.Errorf("%s resource = %q", OpGetPolicyVersion, st.Resource)
		}
	}
	if !found {
		t.Fatalf("no statement allows %s", OpGetPolicyVersion)
	}
}

// The read-only role R2 of the sandbox template must carry exactly the
// example policy, so the checkpoint tests what the documentation promises.
// The template's Fn::Sub values are compared after resolving them, with the
// account ID standing for the example's placeholder.
func TestSandboxReadOnlyRoleMatchesThePolicyExample(t *testing.T) {
	raw, err := os.ReadFile(sandboxPath)
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Resources map[string]struct {
			Type       string         `yaml:"Type"`
			Properties map[string]any `yaml:"Properties"`
		} `yaml:"Resources"`
	}
	if err := yaml.Unmarshal(raw, &template); err != nil {
		t.Fatalf("parse %s: %v", sandboxPath, err)
	}
	for _, name := range []string{"QA", "QB", "QC", "B1", "P1", "P2", "P3", "R1", "R2"} {
		if _, ok := template.Resources[name]; !ok {
			t.Errorf("fixture %s is missing", name)
		}
	}
	r2 := template.Resources["R2"]
	if r2.Type != "AWS::IAM::Role" {
		t.Fatalf("R2 type = %q", r2.Type)
	}
	policies, ok := r2.Properties["Policies"].([]any)
	if !ok || len(policies) != 1 {
		t.Fatalf("R2 must have exactly one inline policy: %#v", r2.Properties["Policies"])
	}
	inline := policies[0].(map[string]any)["PolicyDocument"]
	_, example := readPolicyExample(t)
	if !reflect.DeepEqual(resolveSub(normalize(inline)), normalize(example)) {
		t.Fatalf("R2 inline policy differs from docs/iam/readonly-policy.json:\n%#v\n%#v", inline, example)
	}
	if _, attached := template.Resources["P3"].Properties["Roles"]; attached {
		t.Fatal("P3 must not be attached")
	}
}

// The U2 template gives ap-northeast-1 a local index and a default view
// without a filter (infrastructure-specification.md IX, VW, DV), each behind
// its parameter so an existing index or view can be kept.
func TestSandboxU2TemplateHasALocalIndexAndAnUnfilteredDefaultView(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "test", "sandbox", "u2-fixtures.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Parameters map[string]any `yaml:"Parameters"`
		Resources  map[string]struct {
			Type       string         `yaml:"Type"`
			Condition  string         `yaml:"Condition"`
			Properties map[string]any `yaml:"Properties"`
		} `yaml:"Resources"`
	}
	if err := yaml.Unmarshal(raw, &template); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"CreateIndex", "CreateDefaultView"} {
		if _, ok := template.Parameters[p]; !ok {
			t.Errorf("parameter %s is missing", p)
		}
	}
	ix := template.Resources["IX"]
	if ix.Type != "AWS::ResourceExplorer2::Index" || ix.Properties["Type"] != "LOCAL" || ix.Condition == "" {
		t.Errorf("IX = %+v", ix)
	}
	for _, name := range []string{"VW", "VWOnly"} {
		v := template.Resources[name]
		if v.Type != "AWS::ResourceExplorer2::View" || v.Condition == "" {
			t.Errorf("%s = %+v", name, v)
		}
		if _, filtered := v.Properties["Filters"]; filtered {
			t.Errorf("%s must not have a filter: a filtered default view fails the discovery", name)
		}
		// tag:none is rejected unless the view includes the tags property.
		includesTags := false
		props, _ := v.Properties["IncludedProperties"].([]any)
		for _, p := range props {
			if m, ok := p.(map[string]any); ok && m["Name"] == "tags" {
				includesTags = true
			}
		}
		if !includesTags {
			t.Errorf("%s must include the tags property: %v", name, v.Properties["IncludedProperties"])
		}
	}
	if dv := template.Resources["DV"]; dv.Type != "AWS::ResourceExplorer2::DefaultViewAssociation" {
		t.Errorf("DV = %+v", dv)
	}
	for name, r := range template.Resources {
		if !strings.HasPrefix(r.Type, "AWS::ResourceExplorer2::") {
			t.Errorf("%s (%s): the U2 stack holds only Resource Explorer resources", name, r.Type)
		}
	}
}

// resolveSub replaces each {"Fn::Sub": "..."} of a CloudFormation value with
// its string, resolving ${AWS::Partition} to aws and ${AWS::AccountId} to the
// placeholder of the example policy. Other variables stay as written, so they
// make the comparison fail.
func resolveSub(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t["Fn::Sub"].(string); ok && len(t) == 1 {
			return strings.NewReplacer("${AWS::Partition}", "aws", "${AWS::AccountId}", accountPlaceholder).Replace(s)
		}
		out := map[string]any{}
		for k, x := range t {
			out[k] = resolveSub(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = resolveSub(x)
		}
		return out
	default:
		return t
	}
}

func TestResolveSub(t *testing.T) {
	got := resolveSub(map[string]any{"a": []any{map[string]any{"Fn::Sub": "arn:${AWS::Partition}:iam::${AWS::AccountId}:x/${Other}"}}, "b": "c"})
	want := map[string]any{"a": []any{"arn:aws:iam::<account-id>:x/${Other}"}, "b": "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveSub = %#v", got)
	}
}

// normalize turns YAML and JSON values into the same Go types.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			out[k] = normalize(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = normalize(x)
		}
		return out
	default:
		return t
	}
}
