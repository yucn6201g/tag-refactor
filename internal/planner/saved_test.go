package planner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Unit tests of the saved plan (contract C2, BR4.2 to BR4.4, NFR8.15,
// NFR8.18, NFR8.19), written after the implementation.

// samplePlan has every kind of change, a conflict, a required value not set,
// a remove candidate, findings, an unread policy and an unsupported type.
func samplePlan(t testing.TB) *Plan {
	t.Helper()
	rs := parse(t, "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n    values:\n      prod: [Prod]\n"+
		"required:\n  Owner:\n    default: o\n  CostCenter: {}\nremove: [tmp]\n")
	unsupported := res("arn:aws:ssm:us-east-1:123456789012:patch-compliance/x", nil)
	unsupported.ResourceType = "ssm:patch-compliance"
	managed := res("arn:aws:s3:::logs", map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s", "tmp": "1", "CostCenter": "c"})
	managed.Global, managed.IaCManagedBy = true, "cloudformation:s"
	p := Build(snapshot(
		managed,
		res("arn:b", map[string]string{"env": "dev", "Environment": "test"}),
		res("arn:c", map[string]string{"Name": "x", "Owner": "y", "CostCenter": "c"}),
		unsupported,
	), rs, fixed())
	p.AttachImpact(ImpactResult{Complete: false,
		Findings: []ImpactFinding{
			{Verdict: VerdictAffected, PolicyRef: "arn:aws:iam::123456789012:policy/P", SourceKind: SourceIAMManaged, StatementID: "S",
				ConditionOperator: "StringEquals", ConditionKey: "aws:ResourceTag/env", ConditionValue: "dev", ChangeIDs: []string{"c-000001"}},
			{Verdict: VerdictUndecided, PolicyRef: "arn:aws:iam::123456789012:policy/Q", SourceKind: SourceIAMManaged, StatementID: "#0",
				ConditionKey: "aws:ResourceTag/${aws:PrincipalTag/t}", ChangeIDs: []string{"c-000001", "c-000002"}},
		},
		UnreadPolicies: []UnreadPolicy{{SourceKind: SourceIAMManaged, PolicyRef: "arn:x", Reason: "access denied to iam:GetPolicyVersion"}},
	})
	return p
}

func encode(t testing.TB, p *Plan) []byte {
	t.Helper()
	var b bytes.Buffer
	w := NewSafeJSONWriter(&b)
	if err := WritePlan(w, p); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func writeFile(t testing.TB, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// loadedForm is what a plan keeps through the file: the fields contract C2
// does not have are cleared.
func loadedForm(p *Plan) *Plan {
	q := *p
	q.ScannedResources = 0
	q.FailedRegions = []inventory.FailedRegion{}
	q.Conflicts = nil
	for _, c := range p.Conflicts {
		c.KeyOrder = nil
		q.Conflicts = append(q.Conflicts, c)
	}
	q.RemoveCandidates = nil
	for _, c := range p.RemoveCandidates {
		q.RemoveCandidates = append(q.RemoveCandidates, RemoveCandidate{Key: c.Key})
	}
	q.ImpactFindings = nil
	for _, f := range p.ImpactFindings {
		f.ConditionOperator = ""
		q.ImpactFindings = append(q.ImpactFindings, f)
	}
	q.UnreadPolicies = nil
	for _, u := range p.UnreadPolicies {
		u.PolicyRef = ""
		q.UnreadPolicies = append(q.UnreadPolicies, u)
	}
	return &q
}

func TestWritePlanAndLoadPlanRoundTrip(t *testing.T) {
	p := samplePlan(t)
	data := encode(t, p)

	loaded, err := LoadPlan(writeFile(t, data))
	if err != nil {
		t.Fatal(err)
	}

	got, want := *loaded, *loadedForm(p)
	got.Conflicts = nil
	for _, c := range loaded.Conflicts {
		c.KeyOrder = nil
		got.Conflicts = append(got.Conflicts, c)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded plan differs:\n%+v\nwant:\n%+v", got, want)
	}
	if !bytes.Equal(encode(t, loaded), data) {
		t.Fatal("saving the loaded plan again must give the same bytes")
	}
	if strings.Join(loaded.Conflicts[0].KeyOrder, ",") != "env,Environment" {
		t.Fatalf("key order of a loaded conflict = %v", loaded.Conflicts[0].KeyOrder)
	}
	if !bytes.HasSuffix(data, []byte("}\n")) || bytes.Contains(data, []byte("\r")) || !json.Valid(data) {
		t.Fatalf("the file must be one JSON object ending with a newline:\n%s", data)
	}
}

// NFR8.19: only the fields of contract C2 are written; the rule file is kept
// as --rules gave it.
func TestWritePlanWritesOnlyTheFieldsOfTheContract(t *testing.T) {
	p := samplePlan(t)
	p.RuleFile = filepath.Join("rules", "tagrules.yaml")
	var doc map[string]any
	if err := json.Unmarshal(encode(t, p), &doc); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range doc {
		keys = append(keys, k)
	}
	want := "accountId,createdAt,impactAnalysisComplete,impactFindings,inventoryComplete,keyConflicts,planId,regions,removeCandidates," +
		"resources,ruleFile,toolVersion,unreadPolicies,unsetRequiredTags,unsupportedResourceTypes,version"
	if got := strings.Join(sortedKeys(keys), ","); got != want {
		t.Fatalf("fields = %s", got)
	}
	if doc["ruleFile"] != filepath.Join("rules", "tagrules.yaml") || doc["createdAt"] != "2026-10-20T09:00:00Z" || doc["planId"] != "plan-20261020T090000Z-abcd" {
		t.Fatalf("header = %v", doc)
	}
	change := doc["resources"].([]any)[0].(map[string]any)["changes"].([]any)[0].(map[string]any)
	if strings.Join(sortedKeys(mapKeys(change)), ",") != "changeId,fromKey,fromValue,impacted,kind,mergesIntoExisting,toKey" {
		t.Fatalf("a rename has the fields %v", mapKeys(change))
	}
	finding := doc["impactFindings"].([]any)[0].(map[string]any)
	if strings.Join(sortedKeys(mapKeys(finding)), ",") != "changeIds,conditionKey,conditionValue,policyRef,sourceKind,statementId,verdict" {
		t.Fatalf("a finding has the fields %v", mapKeys(finding))
	}
}

func mapKeys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// BR4.4: the marks are worked out again from the findings; the saved
// impacted flags are not used.
func TestLoadPlanWorksTheMarksOutAgain(t *testing.T) {
	data := encode(t, samplePlan(t))
	data = bytes.ReplaceAll(data, []byte(`"impacted":false`), []byte(`"impacted":true`))
	loaded, err := ParsePlan("plan.json", data)
	if err != nil {
		t.Fatal(err)
	}
	marks := map[string]Mark{}
	for _, c := range loaded.Changes() {
		marks[c.ChangeID] = c.Mark
		if c.Impacted != (c.Mark != MarkNone) {
			t.Errorf("%s: impacted %v with mark %s", c.ChangeID, c.Impacted, c.Mark)
		}
	}
	if marks["c-000001"] != MarkAffected || marks["c-000002"] != MarkUndecided || marks["c-000003"] != MarkNone {
		t.Fatalf("marks = %v", marks)
	}
}

func TestLoadPlanSkipsAUTF8ByteOrderMarkAndRefusesUTF16(t *testing.T) {
	data := encode(t, samplePlan(t))
	if _, err := ParsePlan("p.json", append([]byte("\xef\xbb\xbf"), data...)); err != nil {
		t.Fatalf("a UTF-8 BOM must be skipped: %v", err)
	}
	for _, bom := range []string{"\xff\xfe", "\xfe\xff"} {
		_, err := ParsePlan("p.json", append([]byte(bom), data...))
		e, ok := errkind.As(err)
		if !ok || e.Kind != errkind.InvalidInput || !strings.Contains(e.Hint, "UTF-8") || !strings.Contains(e.Hint, "--out") {
			t.Fatalf("UTF-16 BOM %q: %v", bom, err)
		}
	}
}

// BR4.4: each mistake is InvalidInput with the place in the file, and an
// unknown version is UnsupportedVersion with the tool version of the file.
func TestLoadPlanReportsEachMistakeWithItsPlace(t *testing.T) {
	good := string(encode(t, samplePlan(t)))
	edit := func(old, repl string) string {
		if !strings.Contains(good, old) {
			t.Fatalf("the sample has no %q", old)
		}
		return strings.Replace(good, old, repl, 1)
	}
	cases := []struct {
		name, data, detail string
		kind               errkind.Kind
	}{
		{"version 2", edit(`"version": 1`, `"version": 2`), `plan file version 2 (written by tag-refactor "v0.0.0-test") is not supported`, errkind.UnsupportedVersion},
		{"version missing", edit(`"version": 1,`, ``), "version is missing", errkind.InvalidInput},
		{"version text", edit(`"version": 1`, `"version": "1"`), "version must be a whole number", errkind.InvalidInput},
		{"not JSON", "{\"version\": 1,", "the plan file is not valid JSON", errkind.InvalidInput},
		{"trailing data", good + "{}", "not valid JSON", errkind.InvalidInput},
		{"wrong type", edit(`"inventoryComplete": true`, `"inventoryComplete": "yes"`), "inventoryComplete must be true or false", errkind.InvalidInput},
		{"planId missing", edit(`"planId": "plan-20261020T090000Z-abcd",`, ``), "planId is missing", errkind.InvalidInput},
		{"createdAt", edit(`"createdAt": "2026-10-20T09:00:00Z"`, `"createdAt": "yesterday"`), `createdAt "yesterday" is not a date`, errkind.InvalidInput},
		{"createdAt range", edit(`"createdAt": "2026-10-20T09:00:00Z"`, `"createdAt": "9999-12-31T23:00:00-05:00"`), "out of range", errkind.InvalidInput},
		{"resources missing", edit(`"resources": [`, `"resourcesX": [`), "resources is missing", errkind.InvalidInput},
		{"unknown kind", edit(`"kind":"removeKey"`, `"kind":"deleteKey"`), `.kind "deleteKey" is not renameKey`, errkind.InvalidInput},
		{"kind field missing", edit(`"kind":"renameKey","fromKey":"env",`, `"kind":"renameKey",`), "resources[0].changes[0].fromKey is missing", errkind.InvalidInput},
		{"merges missing", edit(`,"mergesIntoExisting":false`, ``), "resources[0].changes[0].mergesIntoExisting is missing", errkind.InvalidInput},
		{"aws key", edit(`"toKey":"Environment"`, `"toKey":"AWS:Environment"`), `toKey "AWS:Environment" starts with aws:`, errkind.InvalidInput},
		{"repeated change ID", edit(`"changeId":"c-000002"`, `"changeId":"c-000001"`), `changeId "c-000001" appears twice`, errkind.InvalidInput},
		{"repeated ARN", edit(`"arn":"arn:b","region"`, `"arn":"arn:aws:s3:::logs","region"`), `resources[1].arn "arn:aws:s3:::logs" appears twice`, errkind.InvalidInput},
		{"unknown change in a finding", edit(`"changeIds":["c-000001"]`, `"changeIds":["c-999999"]`), `names no change of the plan`, errkind.InvalidInput},
		{"unknown verdict", edit(`"verdict":"affected"`, `"verdict":"maybe"`), `verdict "maybe" is not affected or undecided`, errkind.InvalidInput},
		{"unknown source", edit(`"sourceKind":"iamManaged"`, `"sourceKind":"iamUnknown"`), `sourceKind "iamUnknown" is not one of`, errkind.InvalidInput},
		{"conflict shape", edit(`"presentTags":{`, `"presentTagsX":{`), "keyConflicts[0].presentTags is missing", errkind.InvalidInput},
		{"unset shape", edit(`"arns":[`, `"arnsX":[`), "unsetRequiredTags[0].arns is missing", errkind.InvalidInput},
		{"beforeTags null", edit(`"beforeTags":{"CostCenter"`, `"beforeTags":null,"x":{"CostCenter"`), "resources[0].beforeTags is missing", errkind.InvalidInput},
		{"empty changes", edit(`"changes":[{"changeId":"c-000005"`, `"changes":[],"y":[{"changeId":"c-000005"`), "resources[1].changes is empty", errkind.InvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePlan("plan.json", []byte(tc.data))
			e, ok := errkind.As(err)
			if !ok || e.Kind != tc.kind || e.Reason != errkind.ReasonPlanFile || e.Subject != "plan.json" || e.Hint == "" ||
				!strings.Contains(e.Detail, tc.detail) {
				t.Fatalf("err = %#v, want %s with %q", err, tc.kind, tc.detail)
			}
		})
	}
}

// Unknown fields are ignored (contract rules: adding fields keeps version 1).
func TestLoadPlanIgnoresUnknownFields(t *testing.T) {
	data := strings.Replace(string(encode(t, samplePlan(t))), `"version": 1,`, `"version": 1, "addedLater": {"x": [1, 2]},`, 1)
	data = strings.Replace(data, `"changeId":"c-000001"`, `"changeId":"c-000001","note":"new"`, 1)
	if _, err := ParsePlan("p.json", []byte(data)); err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
}

func TestLoadPlanChecksTheFileBeforeReadingIt(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPlan(filepath.Join(dir, "missing.json"))
	if errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("missing file: %v", err)
	}
	_, err = LoadPlan(dir)
	if e, ok := errkind.As(err); !ok || e.Kind != errkind.InvalidInput || !strings.Contains(e.Detail, "not a regular file") {
		t.Fatalf("folder: %v", err)
	}
	// A file one byte over the limit is refused from its size: a sparse file
	// of 256 MiB + 1 byte takes no space and is never read.
	big := filepath.Join(dir, "big.json")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxPlanBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = LoadPlan(big)
	if e, ok := errkind.As(err); !ok || e.Kind != errkind.InvalidInput || !strings.Contains(e.Detail, "larger than 256 MiB") || !strings.Contains(e.Hint, "--region") {
		t.Fatalf("too large: %v", err)
	}
	if _, err := ParsePlan("p.json", make([]byte, MaxPlanBytes+1)); err == nil {
		t.Fatal("ParsePlan must refuse more than the limit too")
	}
}

// NFR8.15: the plan file escapes what encoding/json leaves, and reads back
// to the same strings.
func TestWritePlanEscapesUnprintableCharacters(t *testing.T) {
	value := "a" + string(rune(0x7f)) + string(rune(0x9b)) + string(rune(0x202e)) + string(rune(0xe0001)) + "<&>\x1b\n" + "\xff"
	p := Build(snapshot(res("arn:a", map[string]string{"tmp": value})), parse(t, "version: 1\nremove: [tmp]\n"), fixed())
	data := encode(t, p)

	for _, raw := range []string{string(rune(0x7f)), string(rune(0x9b)), string(rune(0x202e)), string(rune(0xe0001)), "<", "&", ">", "\x1b", "\xff"} {
		if bytes.Contains(data, []byte(raw)) {
			t.Errorf("the file holds %q as it is", raw)
		}
	}
	for _, escaped := range []string{esc("007f"), esc("009b"), esc("202e"), esc("db40") + esc("dc01"), esc("003c"), esc("0026"),
		// Invalid UTF-8 becomes U+FFFD itself, which is printable.
		esc("003e"), esc("001b"), `\n`, string(rune(0xfffd))} {
		if !bytes.Contains(data, []byte(escaped)) {
			t.Errorf("the file has no %s", escaped)
		}
	}
	loaded, err := ParsePlan("p.json", data)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(value, "\xff", string(rune(0xfffd)))
	if got := loaded.Resources[0].BeforeTags["tmp"]; got != want {
		t.Fatalf("read back %q, want %q", got, want)
	}
}

// The writer stops at the first error and returns it.
func TestWritePlanReturnsTheErrorOfItsWriter(t *testing.T) {
	err := WritePlan(failingWriter{}, samplePlan(t))
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("err = %v", err)
	}
	o := NewJSONObject(failingWriter{}, "")
	o.Field("x", 1)
	o.Object("y", func(*JSONObject) { t.Fatal("no write after an error") })
	if o.Close() == nil {
		t.Fatal("Close must return the first error")
	}
	bad := NewJSONObject(&bytes.Buffer{}, "")
	bad.Field("f", func() {})
	if bad.Close() == nil {
		t.Fatal("a value that cannot be marshaled is an error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errFull }

var errFull = &errkind.Error{Kind: errkind.StorageFailed, Detail: "disk full"}
