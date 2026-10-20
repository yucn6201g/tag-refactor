package awsaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

// NFR3.2: the example policy only reads, and never uses wildcards in actions.
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
			name := action[strings.Index(action, ":")+1:]
			if strings.Contains(action, "*") {
				t.Errorf("action %q contains *", action)
			}
			if !strings.HasPrefix(name, "Get") && !strings.HasPrefix(name, "List") && !strings.HasPrefix(name, "Describe") {
				t.Errorf("action %q is not a read (Get, List, Describe)", action)
			}
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

// The read-only role R2 of the sandbox template must carry exactly the
// example policy, so the checkpoint tests what the documentation promises.
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
	if !reflect.DeepEqual(normalize(inline), normalize(example)) {
		t.Fatalf("R2 inline policy differs from docs/iam/readonly-policy.json:\n%#v\n%#v", inline, example)
	}
	if _, attached := template.Resources["P3"].Properties["Roles"]; attached {
		t.Fatal("P3 must not be attached")
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
