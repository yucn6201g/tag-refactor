package iac

// Unit tests of the CDK path of iac (v0.2.0 U1), written after the
// implementation: the match with synth, the counts of the key, the shapes, the
// scope and shared rules, the check after the edits, the failures of the app
// and the exit code. Synth and the helper are stood in for: the analysis is
// made from the sources of the test, as the helper would make it.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// stackTS is the source of a stack with two queues, one of them tagged.
const stackTS = `import { Stack, Tags } from 'aws-cdk-lib';
export class S extends Stack {
  constructor(scope: any, id: string) {
    super(scope, id);
    const queue = new Queue(this, 'Queue');
    const other = new Queue(this, 'Other');
    Tags.of(queue).add('env', 'Prod');
  }
}
`

// fakeCDK stands in for synth and the helper.
type fakeCDK struct {
	apps       []cdk.App
	syn        cdk.Synthesis
	analysis   cdk.Analysis
	analyzeErr error
	recheckErr error
	recheck    func(edited map[string][]byte) cdk.Analysis
	discovered int
	synthed    int
	analyzed   int
	rechecked  int
}

func (f *fakeCDK) Discover(context.Context, string, []string) ([]cdk.App, error) {
	f.discovered++
	return f.apps, nil
}

func (f *fakeCDK) Synthesize(_ context.Context, app cdk.App, _ cdk.Options, sink cdk.ProgressSink) (cdk.Synthesis, error) {
	f.synthed++
	s := f.syn
	s.App = app
	if s.Failure != nil {
		sink.AppFailed(app.Dir, time.Second, string(s.Failure.Reason))
	} else {
		sink.AppDone(app.Dir, time.Second)
	}
	return s, nil
}

func (f *fakeCDK) Analyze(context.Context, cdk.App, cdk.AnalyzeRequest) (cdk.Analysis, error) {
	f.analyzed++
	return f.analysis, f.analyzeErr
}

func (f *fakeCDK) Recheck(_ context.Context, _ cdk.App, edited map[string][]byte) (cdk.Analysis, error) {
	f.rechecked++
	if f.recheckErr != nil {
		return cdk.Analysis{}, f.recheckErr
	}
	if f.recheck != nil {
		return f.recheck(edited), nil
	}
	return readAgain(f.analysis, edited), nil
}

// readAgain plays the helper on edited contents: the keys of the sites of an
// edited file are read again at their place, which an edit before them in
// the same line does not move in these tests.
func readAgain(before cdk.Analysis, edited map[string][]byte) cdk.Analysis {
	after := before
	after.TagSites = append([]cdk.TagSite(nil), before.TagSites...)
	for i := range after.TagSites {
		s := &after.TagSites[i]
		content, ok := edited[s.KeyEdit.File]
		if !ok || s.KeyEdit.Start >= len(content) {
			continue
		}
		quote := content[s.KeyEdit.Start]
		end := strings.IndexByte(string(content[s.KeyEdit.Start+1:]), quote)
		if end >= 0 {
			s.KeyEdit.Text = string(content[s.KeyEdit.Start+1 : s.KeyEdit.Start+1+end])
		}
	}
	return after
}

// edit returns the range of the n-th quoted s in content.
func editOf(content, file, s, shape string, n int) cdk.Edit {
	at := -1
	for range n + 1 {
		next := strings.Index(content[at+1:], s)
		at += next + 1
	}
	text := strings.Trim(s, "'\"`")
	if shape == cdk.ShapeExpression {
		text = ""
	}
	return cdk.Edit{File: file, Start: at, End: at + len(s), Shape: shape, Text: text, SharedWith: []string{}}
}

func siteOf(content, scope, key, value string, line int) cdk.TagSite {
	k, v := editOf(content, "lib/stack.ts", "'"+key+"'", cdk.ShapeLiteral, 0), editOf(content, "lib/stack.ts", "'"+value+"'", cdk.ShapeLiteral, 0)
	return cdk.TagSite{UseFile: "lib/stack.ts", UseLine: line, Op: "add", ScopeConstructPath: scope, KeyEdit: k, ValueEdit: &v,
		RemoveEdit: cdk.Edit{File: "lib/stack.ts", Shape: cdk.ShapeLiteral}}
}

// newCDKWorld writes stackTS as the app infra and stands in for its synth and
// helper: the stack "stack" with Q1 (stack/Queue, env=Prod) and Q2
// (stack/Other).
func newCDKWorld(t *testing.T, source string) (*fixture, *fakeCDK) {
	t.Helper()
	f := newFixture(t)
	f.file("infra/lib/stack.ts", source)
	app := cdk.App{Dir: "infra", Abs: filepath.Join(f.dir, "infra"), Language: cdk.TypeScript, CDKMajor: 2}
	fake := &fakeCDK{
		apps: []cdk.App{app},
		syn: cdk.Synthesis{Stacks: []cdk.Stack{{Name: "stack", Resources: []cdk.SynthesizedResource{
			{LogicalID: "Q1", Type: "AWS::SQS::Queue", ConstructPath: "stack/Queue/Resource", Tags: map[string]string{"env": "Prod"}},
			{LogicalID: "Q2", Type: "AWS::SQS::Queue", ConstructPath: "stack/Other/Resource", Tags: map[string]string{}},
		}}}},
		analysis: cdk.Analysis{
			ConstructSites: []cdk.ConstructSite{
				{ConstructPath: "stack", File: "bin/app.ts", Line: 3, ConstructID: "stack", From: "source"},
				{ConstructPath: "stack/Queue", File: "lib/stack.ts", Line: 5, ConstructID: "Queue", From: "source"},
				{ConstructPath: "stack/Other", File: "lib/stack.ts", Line: 6, ConstructID: "Other", From: "source"},
			},
			TagSites: []cdk.TagSite{siteOf(source, "stack/Queue", "env", "Prod", 7)},
		},
	}
	return f, fake
}

func (f *fixture) runCDK(fake *fakeCDK, plan string, write bool) *Result {
	f.t.Helper()
	w := &Writer{synth: fake, analyzer: fake}
	res, err := w.Run(context.Background(), Request{PlanPath: plan, Dir: f.dir, Write: write})
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// BR3.1, BR3.4, BR3.3: the rename of a string key changes the range of the key
// alone, is checked by reading the app again, and is written with --write.
func TestCDKRenameChangesTheKeyOnly(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	f.commit()
	plan := f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod")))

	res := f.runCDK(fake, plan, true)

	if len(res.Manual) != 0 || res.Reflected != 1 || len(res.Files) != 1 {
		t.Fatalf("manual %v, reflected %d, files %+v", res.Manual, res.Reflected, res.Files)
	}
	file := res.Files[0]
	if file.File != "infra/lib/stack.ts" || file.Tool != ToolCDK || file.Language != "typescript" || !file.Written {
		t.Fatalf("file %+v", file)
	}
	if !strings.Contains(file.Diff, "-    Tags.of(queue).add('env', 'Prod');\n+    Tags.of(queue).add('Environment', 'Prod');") {
		t.Fatalf("diff:\n%s", file.Diff)
	}
	if got := f.read("infra/lib/stack.ts"); got != strings.Replace(stackTS, "'env'", "'Environment'", 1) {
		t.Fatalf("file:\n%s", got)
	}
	if strings.Join(res.WrittenFiles, ",") != "infra/lib/stack.ts" || fake.analyzed != 1 || fake.rechecked != 1 {
		t.Fatalf("written %v, analyzed %d, rechecked %d", res.WrittenFiles, fake.analyzed, fake.rechecked)
	}
	if !strings.Contains(strings.Join(res.Debug, "\n"), "cdk: stack/Q1 is made at infra/lib/stack.ts:5 (stack/Queue)") {
		t.Fatalf("debug %q", res.Debug)
	}
	if len(res.CDKApps) != 1 || res.CDKApps[0].Status != CDKAppDone || res.ExitCode() != 0 {
		t.Fatalf("apps %+v, exit %d", res.CDKApps, res.ExitCode())
	}
}

// BR2.1, contract C7 match_rule: the account and the region of a stack must
// match when both sides have them; a global resource matches any region.
func TestCDKMatchUsesAccountAndRegion(t *testing.T) {
	for _, tc := range []struct {
		name, account, region string
		global, matched       bool
	}{
		{"no environment", "", "", false, true},
		{"same environment", "111122223333", "ap-northeast-1", false, true},
		{"another account", "999988887777", "", false, false},
		{"another region", "", "us-east-1", false, false},
		{"another region, global resource", "", "us-east-1", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, fake := newCDKWorld(t, stackTS)
			fake.syn.Stacks[0].Account, fake.syn.Stacks[0].Region = tc.account, tc.region
			r := stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))
			plan := f.plan(r)
			if tc.global {
				plan = f.planGlobal(r)
			}

			res := f.runCDK(fake, plan, false)

			if matched := res.Reflected == 1; matched != tc.matched {
				t.Fatalf("matched %v, manual %v", matched, reasons(res))
			}
			if !tc.matched {
				requireReasons(t, res, "env templateNotFound")
			}
		})
	}
}

// planGlobal is plan with the resources global and without a region.
func (f *fixture) planGlobal(resources ...resource) string {
	f.t.Helper()
	p := f.plan(resources...)
	data := f.readAbs(p)
	data = strings.ReplaceAll(data, `"global":false`, `"global":true`)
	data = strings.ReplaceAll(data, `"region":"ap-northeast-1",`, `"region":"",`)
	return f.writeAbs(p, data)
}

func (f *fixture) readAbs(p string) string {
	f.t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) writeAbs(p, content string) string {
	f.t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// BR2.1: two resources of synth for one resource of the plan are not chosen
// from.
func TestCDKTwoCandidatesAreAmbiguous(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	twin := fake.syn.Stacks[0]
	twin.Account = "111122223333"
	fake.syn.Stacks = append(fake.syn.Stacks, twin)

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env cdkAmbiguous")
	if fake.analyzed != 0 {
		t.Fatal("the helper ran for no target")
	}
}

// Plan of the functional design: the key in no site is reflected already when
// the new key is there, and not in the source otherwise; a key in two sites is
// not chosen from, and the place is the first site.
func TestCDKKeyCounts(t *testing.T) {
	t.Run("already reflected", func(t *testing.T) {
		source := strings.Replace(stackTS, "'env'", "'Environment'", 1)
		f, fake := newCDKWorld(t, source)
		fake.analysis.TagSites = []cdk.TagSite{siteOf(source, "stack/Queue", "Environment", "Prod", 7)}

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		if res.AlreadyReflected != 1 || len(res.Manual) != 0 || len(res.Files) != 0 {
			t.Fatalf("already %d, manual %v", res.AlreadyReflected, reasons(res))
		}
	})
	t.Run("not in the source", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.analysis.TagSites = nil

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		requireReasons(t, res, "env keyNotInSource")
		if m := res.Manual[0]; m.File != "infra/lib/stack.ts" || m.Line != 5 {
			t.Fatalf("place %s:%d", m.File, m.Line)
		}
	})
	t.Run("two sites", func(t *testing.T) {
		source := stackTS + "// Tags.of(queue).add('env', 'Prod');\n"
		f, fake := newCDKWorld(t, source)
		second := siteOf(source, "stack", "env", "Prod", 11)
		second.KeyEdit = editOf(source, "lib/stack.ts", "'env'", cdk.ShapeLiteral, 1)
		fake.analysis.TagSites = append(fake.analysis.TagSites, second)

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		requireReasons(t, res, "env cdkAmbiguous")
		if m := res.Manual[0]; m.File != "infra/lib/stack.ts" || m.Line != 7 {
			t.Fatalf("place %s:%d", m.File, m.Line)
		}
	})
}

// BR3.1: an expression for the value, a const for the key, another kind of
// change and a removal by Tags.of(x).remove are listed.
func TestCDKShapesU1DoesNotEdit(t *testing.T) {
	cases := map[string]struct {
		change func() change
		site   func(*cdk.TagSite)
		want   string
	}{
		"value expression": {func() change { return rename("env", "Environment", "Prod") },
			func(s *cdk.TagSite) { s.ValueEdit.Shape, s.ValueEdit.Text = cdk.ShapeExpression, "" }, "env cdkExpression"},
		"key const": {func() change { return rename("env", "Environment", "Prod") },
			func(s *cdk.TagSite) { s.KeyEdit.Shape = cdk.ShapeSameFileConst }, "env cdkUnsupportedShape"},
		"value change": {func() change { return setValue("env", "Prod", "prod") }, func(*cdk.TagSite) {}, "env cdkUnsupportedShape"},
		"merge":        {func() change { return merge("env", "Environment", "Prod") }, func(*cdk.TagSite) {}, "env cdkUnsupportedShape"},
		"remove call": {func() change { return rename("env", "Environment", "Prod") },
			func(s *cdk.TagSite) { s.Op, s.ValueEdit = "remove", nil }, "env cdkUnsupportedShape"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, fake := newCDKWorld(t, stackTS)
			tc.site(&fake.analysis.TagSites[0])

			res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", tc.change())), false)

			requireReasons(t, res, tc.want)
			if len(res.Files) != 0 {
				t.Fatalf("files %+v", res.Files)
			}
		})
	}
}

// BR3.2: a tag on a scope with another resource of the same key is listed.
func TestCDKScopeTagIsListed(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.analysis.TagSites[0].ScopeConstructPath = "stack"
	fake.syn.Stacks[0].Resources[1].Tags = map[string]string{"env": "Prod"}

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env cdkScopeTag")
}

// BR3.2, contract C7 same_site_rule: a range shared by two stacks of one class
// is edited once when both resources change the same way, and listed when one
// of them does not.
func TestCDKSharedSite(t *testing.T) {
	setup := func(t *testing.T) (*fixture, *fakeCDK) {
		f, fake := newCDKWorld(t, stackTS)
		res := fake.syn.Stacks[0].Resources[0]
		fake.syn.Stacks = []cdk.Stack{
			{Name: "one", Resources: []cdk.SynthesizedResource{{LogicalID: "Q1", ConstructPath: "one/Queue/Resource", Tags: res.Tags}}},
			{Name: "two", Resources: []cdk.SynthesizedResource{{LogicalID: "Q1", ConstructPath: "two/Queue/Resource", Tags: res.Tags}}},
		}
		one, two := siteOf(stackTS, "one/Queue", "env", "Prod", 7), siteOf(stackTS, "two/Queue", "env", "Prod", 7)
		one.KeyEdit.SharedWith, two.KeyEdit.SharedWith = []string{"two/Queue"}, []string{"one/Queue"}
		fake.analysis.TagSites = []cdk.TagSite{one, two}
		fake.analysis.ConstructSites = []cdk.ConstructSite{
			{ConstructPath: "one/Queue", File: "lib/stack.ts", Line: 5}, {ConstructPath: "two/Queue", File: "lib/stack.ts", Line: 5}}
		return f, fake
	}
	resourceOf := func(arn, stack string, c change) resource {
		return resource{arn: arn, resourceType: "sqs", changes: []change{c}, before: map[string]string{stackNameTag: stack, logicalIDTag: "Q1"}}
	}
	t.Run("the same change", func(t *testing.T) {
		f, fake := setup(t)

		res := f.runCDK(fake, f.plan(resourceOf(arnQ1, "one", rename("env", "Environment", "Prod")),
			resourceOf(arnQ2, "two", rename("env", "Environment", "Prod"))), false)

		if len(res.Manual) != 0 || res.Reflected != 2 || len(res.Files) != 1 || strings.Count(res.Files[0].Diff, "+    Tags") != 1 {
			t.Fatalf("manual %v, reflected %d, files %+v", reasons(res), res.Reflected, res.Files)
		}
	})
	t.Run("one of them", func(t *testing.T) {
		f, fake := setup(t)

		res := f.runCDK(fake, f.plan(resourceOf(arnQ1, "one", rename("env", "Environment", "Prod"))), false)

		requireReasons(t, res, "env cdkSharedValue")
	})
}

// BR3.4, contract C7 recheck_failed: when the app does not read back as
// expected, its edits are dropped, nothing is written and the change is
// listed; a failure of the helper at the check does the same.
func TestCDKRecheckFailureDropsTheEdits(t *testing.T) {
	for name, set := range map[string]func(*fakeCDK){
		"another key": func(fake *fakeCDK) {
			fake.recheck = func(map[string][]byte) cdk.Analysis { return fake.analysis }
		},
		"a site less": func(fake *fakeCDK) {
			fake.recheck = func(map[string][]byte) cdk.Analysis {
				return cdk.Analysis{ConstructSites: fake.analysis.ConstructSites}
			}
		},
		"a construct moved": func(fake *fakeCDK) {
			fake.recheck = func(edited map[string][]byte) cdk.Analysis {
				a := readAgain(fake.analysis, edited)
				a.ConstructSites = a.ConstructSites[1:]
				return a
			}
		},
		"helper failed": func(fake *fakeCDK) { fake.recheckErr = &cdk.HelperFailure{Reason: "timeout", Tool: "helper"} },
	} {
		t.Run(name, func(t *testing.T) {
			f, fake := newCDKWorld(t, stackTS)
			set(fake)
			f.commit()

			res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), true)

			requireReasons(t, res, "env cdkUnsupportedShape")
			if len(res.Files) != 0 || len(res.WrittenFiles) != 0 || f.read("infra/lib/stack.ts") != stackTS {
				t.Fatalf("files %+v", res.Files)
			}
		})
	}
	t.Run("another version", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.recheckErr = cdk.ErrHelperVersion

		_, err := (&Writer{synth: fake, analyzer: fake}).Run(context.Background(),
			Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), Dir: f.dir})

		if errkind.KindOf(err) != errkind.Internal {
			t.Fatalf("error %v", err)
		}
	})
}

// BR2.3, contract C7 failure_to_manual and exit_code_rule: a resource of a
// failed or skipped app that no template has is listed with the reason of the
// app, and the app makes the exit code 2; an app after the first does not.
func TestCDKFailedAppsAndTheExitCode(t *testing.T) {
	cases := []struct {
		failure cdk.FailureReason
		tool    string
		want    string
		status  CDKAppStatus
	}{
		{cdk.ReasonNeedsAWS, "cdk", "env cdkAppFailed", CDKAppFailed},
		{cdk.ReasonToolMissing, "node", "env cdkAppFailed", CDKAppFailed},
		{cdk.ReasonUnsupportedLanguage, "", "env cdkUnsupportedLanguage", CDKAppSkipped},
		{cdk.ReasonUnknownVersion, "", "env cdkV1", CDKAppSkipped},
	}
	for _, tc := range cases {
		t.Run(string(tc.failure), func(t *testing.T) {
			f, fake := newCDKWorld(t, stackTS)
			fake.syn = cdk.Synthesis{Failure: &cdk.Failure{Reason: tc.failure, Tool: tc.tool}}

			res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

			requireReasons(t, res, tc.want)
			if m := res.Manual[0]; tc.tool != "" && (m.DetailKind != DetailTool || m.Detail != tc.tool) {
				t.Fatalf("detail %s %q", m.DetailKind, m.Detail)
			}
			if len(res.CDKApps) != 1 || res.CDKApps[0].Status != tc.status || res.ExitCode() != 2 {
				t.Fatalf("apps %+v, exit %d", res.CDKApps, res.ExitCode())
			}
		})
	}
	t.Run("the second app", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.apps = append(fake.apps, cdk.App{Dir: "zz", Language: cdk.TypeScript, CDKMajor: 2})

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		if len(res.CDKApps) != 2 || res.CDKApps[1].FailureReason != ReasonNotInU1 || res.ExitCode() != 0 || fake.synthed != 1 {
			t.Fatalf("apps %+v, exit %d, synth %d", res.CDKApps, res.ExitCode(), fake.synthed)
		}
	})
	t.Run("a failure of the helper", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.analyzeErr = &cdk.HelperFailure{Reason: "noTypeScript", Tool: "typescript"}

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		requireReasons(t, res, "env cdkAppFailed")
		if res.Manual[0].Detail != "typescript" || res.CDKApps[0].Status != CDKAppFailed || res.ExitCode() != 2 {
			t.Fatalf("manual %+v, apps %+v", res.Manual, res.CDKApps)
		}
	})
	t.Run("another version of the helper", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.analyzeErr = cdk.ErrHelperVersion

		_, err := (&Writer{synth: fake, analyzer: fake}).Run(context.Background(),
			Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), Dir: f.dir})

		if errkind.KindOf(err) != errkind.Internal || !errors.Is(err, cdk.ErrHelperVersion) {
			t.Fatalf("error %v", err)
		}
	})
}

// BR1.1, NFR5: without a resource made by CloudFormation, nothing of CDK runs.
func TestCDKNothingRunsWithoutStackTags(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)

	res := f.runCDK(fake, f.plan(tfResource(arnQ1, rename("env", "Environment", "Prod"))), false)

	if fake.discovered != 0 || fake.synthed != 0 || res.Unmanaged != 1 {
		t.Fatalf("discovered %d, synthesized %d, unmanaged %d", fake.discovered, fake.synthed, res.Unmanaged)
	}
}

// BR2.2: a resource whose construct is not in the source is listed; a file the
// helper could not parse is left out.
func TestCDKNoSourceAndParseErrors(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.analysis.ConstructSites = fake.analysis.ConstructSites[:1]
	fake.analysis.Unsupported = []cdk.UnsupportedSite{{File: "lib/broken.ts", Line: 3, Reason: cdk.ReasonParseError},
		{File: "lib/stack.ts", Line: 9, Reason: "unsupportedShape"}}

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env cdkNoSource")
	if len(res.SkippedFiles) != 1 || res.SkippedFiles[0].File != "infra/lib/broken.ts" || res.SkippedFiles[0].Reason != SkipParseError {
		t.Fatalf("skipped %+v", res.SkippedFiles)
	}
}

// Functional spec §2: a range that does not hold the quoted string the helper
// read makes the file unreadable, and nothing is edited.
func TestCDKRangeThatDoesNotMatchIsUnreadable(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.analysis.TagSites[0].KeyEdit.Start++

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env unreadableFile")
	if len(res.Files) != 0 {
		t.Fatalf("files %+v", res.Files)
	}
}

// BR3.1: the new key keeps the quote of the old one, with the escapes of
// TypeScript.
func TestCDKKeysKeepTheirQuote(t *testing.T) {
	for _, tc := range []struct {
		q    quoteStyle
		s    string
		want string
	}{
		{quoteSingle, "Environment", "'Environment'"},
		{quoteSingle, "it's", `'it\'s'`},
		{quoteDouble, `say "hi"`, `"say \"hi\""`},
		{quoteBacktick, "a`b${c}", "`a\\`b\\${c}`"},
		{quoteSingle, "a\\b\nc\td\x01", "'a\\\\b\\nc\\td\\" + "u0001'"},
		{quoteDouble, "$x {y}", `"$x {y}"`},
	} {
		if got := writeText(formatTypeScript, tc.q, tc.s, true); got != tc.want {
			t.Errorf("tsText(%q) = %s, want %s", tc.s, got, tc.want)
		}
	}
}
