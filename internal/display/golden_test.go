package display

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// The expected files of the four formats in two languages, the saved plan
// and the JSON and Markdown of scan (NFR9.6): the same input gives the same
// bytes on Linux, macOS and Windows. Make them again with
//
//	go test -run TestGolden ./internal/display/... -update
//
// and look at the difference before committing it.

var update = flag.Bool("update", false, "write the expected files of TestGolden again")

var goldenTime = time.Date(2026, 10, 19, 12, 4, 0, 0, time.UTC)

// hostile is a value with characters that the outputs must defuse: a
// control character, a bidirectional override, a table bar, a backtick and
// HTML, written from code points so the source holds none of them raw.
var hostile = "a|b`c<d>&" + string(rune(0x1b)) + "[2J" + string(rune(0x202e)) + "e"

// goldenPlan is a plan with every section: changes of the four kinds, a
// conflict, a required value not set, remove candidates, an IaC resource,
// an affected and an undecided finding, an unread policy and a failed
// region.
func goldenPlan(t testing.TB) *planner.Plan {
	t.Helper()
	rs, err := rules.Parse("tagrules.yaml", []byte("version: 1\nkeys:\n  Environment:\n    aliases: [env, Env]\n    values:\n      prod: [Prod]\n"+
		"required:\n  Owner:\n    default: platform\n    when:\n      - match: {resourceType: \"ec2:instance\"}\n        value: compute-team\n"+
		"  CostCenter:\n    when:\n      - match: {resourceType: \"ec2:instance\"}\n        value: cc-1\n"+
		"remove: [tmp-owner]\n"), rules.Options{})
	if err != nil {
		t.Fatal(err)
	}
	logs := inventory.Resource{ARN: "arn:aws:s3:::my-logs-bucket", Region: "us-east-1", Global: true, ResourceType: "s3",
		Tags:         map[string]string{"Env": "dev", "aws:cloudformation:stack-name": "logging", "CostCenter": "c"},
		IaCManagedBy: "cloudformation:logging"}
	snap := &inventory.Snapshot{AccountID: "123456789012", ScannedRegions: []string{"eu-west-1", "us-east-1"},
		FailedRegions: []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}},
		Resources: []inventory.Resource{
			{ARN: "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc", Region: "us-east-1", ResourceType: "ec2:instance",
				Tags: map[string]string{"env": "Prod"}},
			logs,
			{ARN: "arn:aws:sqs:us-east-1:123456789012:qa", Region: "us-east-1", ResourceType: "sqs",
				Tags: map[string]string{"env": "dev", "Environment": "test", "tmp-owner": hostile, "Name": hostile, "Owner": "x"}},
			{ARN: "arn:aws:sqs:us-east-1:123456789012:qb", Region: "us-east-1", ResourceType: "sqs",
				Tags: map[string]string{"Environment": "Prod", "tmp-owner": "y", "日本語": "値", "CostCenter": "c"}},
		}}
	p := planner.Build(snap, rs, planner.Options{Now: goldenTime, RuleFile: "tagrules.yaml", ToolVersion: "v0.1.0",
		Rand: bytes.NewReader([]byte{0x7f, 0x3a})})
	p.AttachImpact(planner.ImpactResult{Complete: false,
		Findings: []planner.ImpactFinding{
			{Verdict: planner.VerdictAffected, PolicyRef: "arn:aws:iam::123456789012:policy/DevAccess", SourceKind: planner.SourceIAMManaged,
				StatementID: "AllowDevEC2", ConditionOperator: "StringEquals", ConditionKey: "aws:ResourceTag/env", ConditionValue: "dev",
				ChangeIDs: []string{"c-000001", "c-000005"}},
			{Verdict: planner.VerdictUndecided, PolicyRef: "arn:aws:iam::123456789012:policy/Team" + hostile, SourceKind: planner.SourceIAMManaged,
				StatementID: "#0", ConditionOperator: "StringEquals", ConditionKey: "aws:ResourceTag/${aws:PrincipalTag/team}", ConditionValue: "yes",
				ChangeIDs: []string{"c-000002"}},
		},
		UnreadPolicies: []planner.UnreadPolicy{{SourceKind: planner.SourceIAMManaged, PolicyRef: "arn:aws:iam::123456789012:policy/Hidden",
			Reason: "the default version has no document in iam:GetAccountAuthorizationDetails"}},
	})
	return p
}

func goldenScan() *inventory.ScanReport {
	snap := &inventory.Snapshot{AccountID: "123456789012", ScannedAt: goldenTime, EnabledRegions: []string{"eu-west-1", "us-east-1", "ap-northeast-1"},
		ScannedRegions: []string{"eu-west-1", "us-east-1", "ap-northeast-1"},
		FailedRegions:  []inventory.FailedRegion{{Region: "eu-west-1", Kind: "AccessDenied", Operation: "tag:GetResources"}},
		Resources: []inventory.Resource{
			{ARN: "arn:a", Tags: map[string]string{"env": "dev"}, EverTagged: true},
			{ARN: "arn:b", Tags: map[string]string{"Env": "dev", "cost-center": hostile}, EverTagged: true},
			{ARN: "arn:c", Tags: map[string]string{"Environment": "prod", "cost_center": "x"}, EverTagged: true},
			{ARN: "arn:d", Tags: map[string]string{}},
			{ARN: "arn:e", ResourceType: "ssm:patch-compliance", NotTaggable: true, Tags: map[string]string{}},
		},
		Discovery: &inventory.Discovery{Status: inventory.StatusPartial, Regions: []inventory.DiscoveryRegion{
			{Region: "us-east-1", Status: inventory.StatusComplete, Methods: []string{inventory.MethodConfig}},
			{Region: "ap-northeast-1", Status: inventory.StatusUnavailable, Reason: "no Resource Explorer index"},
		}}}
	return inventory.NewScanReport(snap, "tagrules.yaml", []string{"Owner"})
}

// render prints the plan or the scan in one format and language.
func render(t testing.TB, lang Lang, f func(p *Printer)) string {
	t.Helper()
	p, out, _ := newPrinter()
	p.SetLang(lang)
	f(p)
	return out.String()
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", filepath.FromSlash(name))
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (make the expected files with -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the expected file:\n%s\nwant:\n%s", name, got, want)
	}
	if bytes.Contains(got, []byte("\r")) || bytes.HasPrefix(got, []byte("\xef\xbb\xbf")) {
		t.Errorf("%s must be LF and without a BOM", name)
	}
}

func TestGoldenPlanFormats(t *testing.T) {
	plan := goldenPlan(t)
	for _, f := range PlanFormats {
		for _, lang := range []Lang{English, Japanese} {
			got := render(t, lang, func(p *Printer) {
				p.PlanResult(plan, Output{Format: f, ExitCode: plan.ExitCode(), ToolVersion: "v0.1.0"})
			})
			checkGolden(t, "plan/"+string(f)+"."+string(lang)+".golden", []byte(got))
		}
	}
}

func TestGoldenSavedPlan(t *testing.T) {
	var b bytes.Buffer
	w := planner.NewSafeJSONWriter(&b)
	if err := planner.WritePlan(w, goldenPlan(t)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "plan/saved.json", b.Bytes())
	// The saved plan and the details of the JSON output are the same bytes
	// (both go through WriteDetails and SafeJSONWriter).
	view := render(t, English, func(p *Printer) { p.PlanResult(goldenPlan(t), Output{Format: FormatJSON}) })
	start := strings.Index(b.String(), `  "resources"`)
	end := strings.Index(b.String(), ",\n  \"unsupportedResourceTypes\"")
	saved := strings.ReplaceAll(b.String()[start:end], "\n  ", "\n    ")
	if !strings.Contains(view, "  "+saved) {
		t.Fatal("the details of plan --format json differ from the saved plan")
	}
}

func TestGoldenScanFormats(t *testing.T) {
	rep := goldenScan()
	for _, f := range []Format{FormatJSON, FormatMarkdown, FormatTable} {
		for _, lang := range []Lang{English, Japanese} {
			got := render(t, lang, func(p *Printer) { p.ScanResult(rep, Output{Format: f, ExitCode: 1, ToolVersion: "v0.1.0"}) })
			checkGolden(t, "scan/"+string(f)+"."+string(lang)+".golden", []byte(got))
		}
	}
}

// A plan saved before U5 has no condition operator (contract C2 kept none
// until U5 added conditionOperator): the table, the diff and Markdown leave
// out the word and its space instead of an empty word.
func TestALoadedPlanShowsItsConditionsWithoutAnOperator(t *testing.T) {
	var b bytes.Buffer
	w := planner.NewSafeJSONWriter(&b)
	if err := planner.WritePlan(w, goldenPlan(t)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	beforeU5 := regexp.MustCompile(`"conditionOperator":"[^"]*",`).ReplaceAll(b.Bytes(), nil)
	if bytes.Equal(beforeU5, b.Bytes()) {
		t.Fatal("the saved plan has no conditionOperator to take out")
	}
	loaded, err := planner.ParsePlan("plan.json", beforeU5)
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []Lang{English, Japanese} {
		table := render(t, lang, func(p *Printer) { p.PlanResult(loaded, Output{Format: FormatTable}) })
		want := map[Lang]string{English: "Condition aws:ResourceTag/env = \"dev\"\n", Japanese: "条件 aws:ResourceTag/env = \"dev\"\n"}[lang]
		if !strings.Contains(table, want) || strings.Contains(table, "Condition  ") || strings.Contains(table, "条件  ") {
			t.Errorf("%s table:\n%s", lang, table)
		}
		md := render(t, lang, func(p *Printer) { p.PlanResult(loaded, Output{Format: FormatMarkdown}) })
		if !strings.Contains(md, "| ` aws:ResourceTag/env ` = ` dev ` |") || strings.Contains(md, "`  ` ") {
			t.Errorf("%s Markdown:\n%s", lang, md)
		}
		diff := render(t, lang, func(p *Printer) { p.PlanResult(loaded, Output{Format: FormatDiff}) })
		if !strings.Contains(diff, "! - env         = \"Prod\"\n") || strings.Contains(diff, "  \n") {
			t.Errorf("%s diff:\n%s", lang, diff)
		}
	}
}

// BR3.4: JSON is never translated, so both languages give the same bytes.
func TestJSONIsTheSameInEveryLanguage(t *testing.T) {
	plan := goldenPlan(t)
	en := render(t, English, func(p *Printer) { p.PlanResult(plan, Output{Format: FormatJSON, ExitCode: 1}) })
	ja := render(t, Japanese, func(p *Printer) { p.PlanResult(plan, Output{Format: FormatJSON, ExitCode: 1}) })
	if en != ja {
		t.Fatal("plan JSON differs between languages")
	}
	rep := goldenScan()
	en = render(t, English, func(p *Printer) { p.ScanResult(rep, Output{Format: FormatJSON}) })
	ja = render(t, Japanese, func(p *Printer) { p.ScanResult(rep, Output{Format: FormatJSON}) })
	if en != ja {
		t.Fatal("scan JSON differs between languages")
	}
}
