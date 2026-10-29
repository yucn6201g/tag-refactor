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

// The example policy of apply (U6 NFR3.11, security design §2) and the role
// R3 of the U6 sandbox template.

var applyPolicyPath = filepath.Join("..", "..", "docs", "iam", "apply-policy.json")

// applyStatement is a statement of the apply example: Resource is a string
// or a list.
type applyStatement struct {
	Sid       string
	Effect    string
	Action    []string
	Resource  any
	Condition map[string]any
}

func readApplyPolicy(t *testing.T) ([]applyStatement, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(applyPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version   string
		Statement []applyStatement
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", applyPolicyPath, err)
	}
	if doc.Version != "2012-10-17" {
		t.Fatalf("version %q", doc.Version)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	return doc.Statement, generic
}

func resources(r any) []string {
	switch v := r.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, x := range v {
			s, _ := x.(string)
			out = append(out, s)
		}
		return out
	}
	return nil
}

// NFR3.11: the actions of the example are exactly the reads of scan and
// plan, the tagging API, the actions of the table of resource types and the
// record in S3: nothing missing, nothing more.
func TestApplyPolicyExampleMatchesTheTables(t *testing.T) {
	statements, _ := readApplyPolicy(t)
	var inDoc []string
	for _, st := range statements {
		if st.Effect != "Allow" {
			t.Errorf("%s: effect %q", st.Sid, st.Effect)
		}
		for _, a := range st.Action {
			if strings.Contains(a, "*") {
				t.Errorf("%s: action %q has a wildcard", st.Sid, a)
			}
		}
		inDoc = append(inDoc, st.Action...)
	}
	sort.Strings(inDoc)
	if strings.Join(slices.Compact(inDoc), ",") != strings.Join(ApplyPermissions(), ",") || len(slices.Compact(slices.Clone(inDoc))) != len(inDoc) {
		t.Fatalf("docs/iam/apply-policy.json allows %v, the code needs %v", inDoc, ApplyPermissions())
	}
}

// The example starts with every statement of the read-only example, and
// gives each row of the table of resource types one statement with its
// actions and resources; the tagging API is "*", and the record is limited
// to the bucket and the prefix.
func TestApplyPolicyExampleStatements(t *testing.T) {
	statements, generic := readApplyPolicy(t)
	_, readOnly := readPolicyExample(t)
	readStatements := readOnly["Statement"].([]any)
	all := generic["Statement"].([]any)
	if !reflect.DeepEqual(all[:len(readStatements)], readStatements) {
		t.Fatal("the apply example must start with the statements of the read-only example")
	}
	bySid := map[string]applyStatement{}
	for _, st := range statements {
		bySid[st.Sid] = st
	}
	if st := bySid["TagRefactorTaggingAPI"]; strings.Join(st.Action, ",") != OpTagResources+","+OpUntagResources || st.Resource != "*" {
		t.Errorf("tagging API statement = %+v", st)
	}
	typeStatements := 0
	for _, st := range statements[len(readStatements):] {
		if !strings.HasPrefix(st.Sid, "TagRefactorTag") || st.Sid == "TagRefactorTaggingAPI" {
			continue
		}
		typeStatements++
		service := st.Action[0][:strings.Index(st.Action[0], ":")]
		row, ok := TaggingTypeOf("arn:aws:" + service + ":")
		if !ok {
			t.Errorf("%s: no row of the table for %s", st.Sid, service)
			continue
		}
		want := slices.Compact(slices.Concat(row.TagActions, row.UntagActions))
		got := slices.Clone(st.Action)
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, slices.Compact(want)) || !slices.Equal(resources(st.Resource), row.Resources) {
			t.Errorf("%s = %v on %v, want %v on %v", st.Sid, st.Action, st.Resource, want, row.Resources)
		}
	}
	if typeStatements != len(TaggingTypes) {
		t.Errorf("%d statements for resource types, the table has %d rows", typeStatements, len(TaggingTypes))
	}
	objects, list := bySid["TagRefactorRecordObjects"], bySid["TagRefactorRecordList"]
	if objects.Resource != "arn:aws:s3:::<record-bucket>/<record-prefix>/*" || list.Resource != "arn:aws:s3:::<record-bucket>" ||
		strings.Join(list.Action, ",") != OpListBucket {
		t.Errorf("record statements = %+v %+v", objects, list)
	}
	like, _ := list.Condition["StringLike"].(map[string]any)
	if like["s3:prefix"] != "<record-prefix>/*" {
		t.Errorf("ListBucket must be limited to the prefix: %v", list.Condition)
	}
}

// Every Resource of the table is an ARN of its service in the account,
// except the S3 buckets and the EC2 snapshots and images, whose ARNs have no
// account.
func TestTaggingTypesResourcesMatchTheService(t *testing.T) {
	for _, row := range TaggingTypes {
		for _, r := range row.Resources {
			if !strings.HasPrefix(r, "arn:aws:"+row.Service+":") {
				t.Errorf("%s: %s is not an ARN of %s", row.Name, r, row.Service)
			}
			noAccount := row.Service == "s3" || strings.HasSuffix(r, "::snapshot/*") || strings.HasSuffix(r, "::image/*")
			if !noAccount && !strings.Contains(r, ":"+accountPlaceholder+":") {
				t.Errorf("%s: %s is not limited to the account", row.Name, r)
			}
		}
		for _, a := range slices.Concat(row.TagActions, row.UntagActions) {
			if !strings.HasPrefix(a, row.Service+":") {
				t.Errorf("%s: action %s is not of %s", row.Name, a, row.Service)
			}
		}
	}
}

// The role R3 of the U6 template carries exactly the apply example, with the
// account, the bucket BU6R and the prefix runs in place of the placeholders,
// and the template holds the five fixtures and three outputs of the U6
// infrastructure design.
func TestSandboxApplyRoleMatchesThePolicyExample(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "test", "sandbox", "u6-fixtures.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Parameters map[string]any `yaml:"Parameters"`
		Resources  map[string]struct {
			Type       string         `yaml:"Type"`
			Properties map[string]any `yaml:"Properties"`
		} `yaml:"Resources"`
		Outputs map[string]any `yaml:"Outputs"`
	}
	if err := yaml.Unmarshal(raw, &template); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"VU6": "AWS::EC2::VPC", "GU6": "AWS::EC2::SecurityGroup", "BU6R": "AWS::S3::Bucket",
		"BU6RPolicy": "AWS::S3::BucketPolicy", "R3": "AWS::IAM::Role"}
	if len(template.Resources) != len(want) {
		t.Errorf("%d resources, want %d", len(template.Resources), len(want))
	}
	for name, typ := range want {
		if template.Resources[name].Type != typ {
			t.Errorf("%s has type %q, want %q", name, template.Resources[name].Type, typ)
		}
	}
	for _, out := range []string{"ApplyRoleArn", "SecurityGroupArn", "RecordBucket"} {
		if _, ok := template.Outputs[out]; !ok {
			t.Errorf("output %s is missing", out)
		}
	}
	if _, ok := template.Parameters["SsoPermissionSetName"]; !ok || len(template.Parameters) != 1 {
		t.Errorf("parameters = %v", template.Parameters)
	}
	policies, ok := template.Resources["R3"].Properties["Policies"].([]any)
	if !ok || len(policies) != 1 {
		t.Fatalf("R3 must have exactly one inline policy")
	}
	inline := policies[0].(map[string]any)["PolicyDocument"]
	resolved := resolveSub(normalize(inline))
	data, _ := json.Marshal(resolved)
	text := strings.ReplaceAll(string(data), "${BU6R}", "<record-bucket>")
	text = strings.NewReplacer(":::<record-bucket>/runs/", ":::<record-bucket>/<record-prefix>/",
		`"runs/*"`, `"<record-prefix>/*"`).Replace(text)
	var got any
	_ = json.Unmarshal([]byte(text), &got)
	_, example := readApplyPolicy(t)
	if !reflect.DeepEqual(got, normalize(example)) {
		t.Fatalf("R3 inline policy differs from docs/iam/apply-policy.json:\n%s", text)
	}
	if strings.Contains(string(raw), "AccessControl") {
		t.Error("the record bucket must not use ACLs")
	}
}
