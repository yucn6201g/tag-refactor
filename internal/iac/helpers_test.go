package iac

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helpers of the unit tests of iac, which are written after the implementation
// (team testing posture). Every test builds its sources in a temporary folder
// and its plan with the shape of contract C2.

const (
	arnQ1 = "arn:aws:sqs:ap-northeast-1:111122223333:q1"
	arnQ2 = "arn:aws:sqs:ap-northeast-1:111122223333:q2"
)

// fixture is a folder of sources.
type fixture struct {
	t   *testing.T
	dir string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, dir: dir}
}

func (f *fixture) file(rel, content string) string {
	f.t.Helper()
	p := filepath.Join(f.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) read(rel string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(rel)))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

// change is a change of a plan.
type change map[string]any

var changeNumber int

func newID() string {
	changeNumber++
	return "c-" + strings.Repeat("0", 6-len(itoa(changeNumber))) + itoa(changeNumber)
}

func itoa(n int) string {
	digits := ""
	for {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
		if n == 0 {
			return digits
		}
	}
}

func rename(from, to, value string) change {
	return change{"changeId": newID(), "kind": "renameKey", "fromKey": from, "toKey": to, "fromValue": value, "mergesIntoExisting": false, "impacted": false}
}

func merge(from, to, value string) change {
	return change{"changeId": newID(), "kind": "renameKey", "fromKey": from, "toKey": to, "fromValue": value, "mergesIntoExisting": true, "impacted": false}
}

func setValue(key, from, to string) change {
	return change{"changeId": newID(), "kind": "changeValue", "fromKey": key, "fromValue": from, "toValue": to, "impacted": false}
}

func addTag(key, value string) change {
	return change{"changeId": newID(), "kind": "addRequired", "toKey": key, "toValue": value, "impacted": false}
}

func remove(key, value string) change {
	return change{"changeId": newID(), "kind": "removeKey", "fromKey": key, "fromValue": value, "impacted": false}
}

// resource is a resource of a plan.
type resource struct {
	arn, resourceType string
	before            map[string]string
	changes           []change
}

// stackResource is a resource of the stack "stack" with the given logical ID.
func stackResource(arn, logicalID string, changes ...change) resource {
	return resource{arn: arn, resourceType: "sqs", changes: changes,
		before: map[string]string{stackNameTag: "stack", logicalIDTag: logicalID}}
}

// tfResource is a resource to find in the state.
func tfResource(arn string, changes ...change) resource {
	return resource{arn: arn, resourceType: "sqs", changes: changes, before: map[string]string{}}
}

// plan writes a plan outside the sources.
func (f *fixture) plan(resources ...resource) string {
	f.t.Helper()
	list := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		before := map[string]string{}
		for k, v := range r.before {
			before[k] = v
		}
		// The plan's tags before the changes follow from the changes.
		for _, c := range r.changes {
			if from, ok := c["fromKey"].(string); ok {
				if v, ok := c["fromValue"].(string); ok {
					if _, set := before[from]; !set {
						before[from] = v
					}
				}
			}
		}
		list = append(list, map[string]any{"arn": r.arn, "region": "ap-northeast-1", "global": false, "resourceType": r.resourceType,
			"beforeTags": before, "iacManagedBy": nil, "changes": r.changes})
	}
	data, err := json.Marshal(map[string]any{"version": 1, "toolVersion": "test", "planId": "p", "accountId": "111122223333",
		"createdAt": "2026-10-31T00:00:00Z", "regions": []string{"ap-northeast-1"}, "inventoryComplete": true,
		"impactAnalysisComplete": true, "ruleFile": "r.yaml", "resources": list})
	if err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(f.t.TempDir(), "plan.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// state writes the output of terraform show -json for address and ARN pairs.
func (f *fixture) state(pairs ...string) string {
	f.t.Helper()
	resources := []map[string]any{}
	for i := 0; i < len(pairs); i += 2 {
		resources = append(resources, map[string]any{"address": pairs[i], "mode": "managed", "type": "aws_sqs_queue", "name": "x",
			"values": map[string]any{"arn": pairs[i+1]}})
	}
	data, err := json.Marshal(map[string]any{"format_version": "1.0", "values": map[string]any{"root_module": map[string]any{"resources": resources}}})
	if err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(f.t.TempDir(), "state.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// run runs iac on the folder without writing and requires no error.
func (f *fixture) run(plan string) *Result {
	f.t.Helper()
	res, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: f.dir})
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// runState is run with a state.
func (f *fixture) runState(plan, state string) *Result {
	f.t.Helper()
	res, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: f.dir, TFStatePath: state})
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// runWrite runs iac with --write and requires no error.
func (f *fixture) runWrite(plan string) *Result {
	f.t.Helper()
	res, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: f.dir, Write: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// git runs git in the folder, without the user's settings.
func (f *fixture) git(args ...string) string {
	f.t.Helper()
	requireGit(f.t)
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// commit makes the folder a repository with everything committed, keeping the
// line breaks of the files.
func (f *fixture) commit() {
	f.t.Helper()
	f.git("init", "-q")
	f.git("config", "core.autocrlf", "false")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "sources")
}

// reasons lists "key reason" for each manual item.
func reasons(res *Result) []string {
	out := make([]string, 0, len(res.Manual))
	for _, m := range res.Manual {
		out = append(out, m.Key+" "+string(m.Reason))
	}
	return out
}

func requireReasons(t *testing.T, res *Result, want ...string) {
	t.Helper()
	got := reasons(res)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("manual items = %v, want %v", got, want)
	}
}

// yamlTemplate makes a template of one queue with the given lines below Tags,
// indented by the caller, with the line break of the text.
func yamlTemplate(logicalID, tags string) string {
	return "Resources:\n  " + logicalID + ":\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n" + tags
}
