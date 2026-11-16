package iac

// Unit tests of the CDK apps of iac in v0.2.0 U3, written after the
// implementation: --cdk-app, every app in name order, the reason of the
// resources of no app, the synth with the trace and the counts of the
// programs, and the comparisons after the edits. Synth and the helper are
// stood in for by fakeCDK.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// recordingSink keeps the calls of the sink, one line each.
type recordingSink struct {
	mu    sync.Mutex
	lines []string
}

func (s *recordingSink) add(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, fmt.Sprintf(format, args...))
}

func (s *recordingSink) AppsWaiting(apps []string) { s.add("waiting %s", strings.Join(apps, ",")) }
func (s *recordingSink) AppStarted(app, _ string)  { s.add("started %s", app) }
func (s *recordingSink) AppDone(app string, _ time.Duration) {
	s.add("done %s", app)
}
func (s *recordingSink) AppFailed(app string, _ time.Duration, reason string) {
	s.add("failed %s %s", app, reason)
}
func (s *recordingSink) AppSkipped(app, reason string) { s.add("skipped %s %s", app, reason) }

func (f *fixture) runReq(fake *fakeCDK, req Request) (*Result, error) {
	f.t.Helper()
	req.Dir = f.dir
	return (&Writer{synth: fake, analyzer: fake}).Run(context.Background(), req)
}

func tsApp(dir string) cdk.App {
	return cdk.App{Dir: dir, Language: cdk.TypeScript, CDKMajor: 2}
}

func TestSelectApps(t *testing.T) {
	apps := []cdk.App{tsApp("infra/ts"), tsApp("web"), {Dir: "infra/py", Language: cdk.Python, CDKMajor: 2}}
	dirs := func(apps []cdk.App) string {
		var out []string
		for _, a := range apps {
			out = append(out, a.Dir)
		}
		return strings.Join(out, ",")
	}
	for _, tc := range []struct {
		name    string
		values  []string
		windows bool
		want    string
		detail  string
	}{
		{"no value", nil, false, "infra/ts,web,infra/py", ""},
		{"in name order, once", []string{"web", "infra/ts", "./web", "infra/ts/"}, false, "infra/ts,web", ""},
		{"Windows without case and with backslashes", []string{`Infra\TS`}, true, "infra/ts", ""},
		{"Unix with case", []string{"Infra/TS"}, false, "", "iac.cdkAppNotFound"},
		{"a backslash on Unix is a name", []string{`infra\ts`}, false, "", "iac.cdkAppNotFound"},
		{"..", []string{".."}, false, "", "iac.cdkAppOutside"},
		{"../other", []string{"../other"}, false, "", "iac.cdkAppOutside"},
		{"through .. out", []string{"a/../../other"}, false, "", "iac.cdkAppOutside"},
		{`..\other on Windows`, []string{`..\other`}, true, "", "iac.cdkAppOutside"},
		{"empty", []string{""}, false, "", "iac.cdkAppOutside"},
		{"absolute", []string{"/etc"}, false, "", "iac.cdkAppNotFound"},
		{"a drive", []string{`C:\x`}, true, "", "iac.cdkAppNotFound"},
		{"not an app", []string{"infra"}, false, "", "iac.cdkAppNotFound"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectApps(apps, tc.values, tc.windows)
			if tc.detail == "" {
				if err != nil || dirs(got) != tc.want {
					t.Fatalf("got %s, %v", dirs(got), err)
				}
				return
			}
			var e *errkind.Error
			if !errors.As(err, &e) || e.DetailID != tc.detail || e.Subject != "--cdk-app" || e.HintID != "iac.cdkAppFix" {
				t.Fatalf("error %v", err)
			}
		})
	}
}

// NFR2.7: --cdk-app is checked before anything runs, even when the plan has
// no resource made by CloudFormation; the apps it picks are found once.
func TestCDKAppsPickedByTheOption(t *testing.T) {
	t.Run("a wrong value", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)

		_, err := f.runReq(fake, Request{PlanPath: f.plan(tfResource(arnQ1, rename("env", "Environment", "Prod"))), CDKApps: []string{"none"}})

		if errkind.KindOf(err) != errkind.InvalidInput || fake.synthed != 0 {
			t.Fatalf("error %v, synth %d", err, fake.synthed)
		}
	})
	t.Run("one of two apps", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.apps = append(fake.apps, tsApp("zz"))

		res, err := f.runReq(fake, Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))),
			CDKApps: []string{"infra"}})

		if err != nil || fake.discovered != 1 || !slices.Equal(fake.synthDirs, []string{"infra"}) || len(res.CDKApps) != 1 || res.Reflected != 1 {
			t.Fatalf("error %v, discovered %d, synth %v, apps %+v", err, fake.discovered, fake.synthDirs, res.CDKApps)
		}
	})
	t.Run("a writer without a synthesizer", func(t *testing.T) {
		f := newFixture(t)
		res, err := (&Writer{}).Run(context.Background(), Request{PlanPath: f.plan(tfResource(arnQ1, rename("env", "Environment", "Prod"))),
			Dir: f.dir, CDKApps: []string{"../anything"}})
		if err != nil || res.Unmanaged != 1 {
			t.Fatalf("error %v, result %+v", err, res)
		}
	})
	t.Run("an error of discovery", func(t *testing.T) {
		f, fake := newCDKWorld(t, stackTS)
		fake.discoverErr = errors.New("too deep")

		if _, err := f.runReq(fake, Request{PlanPath: f.plan(stackResource(arnQ1, "Q1")), CDKApps: []string{"infra"}}); err == nil {
			t.Fatal("no error")
		}
	})
}

// BR1.4, BR2.7, contract C2: the skipped apps are told about before the apps
// to synthesize, every app is synthesized in name order with the timeout of
// the request, and a failed app does not stop the next one.
func TestCDKEveryAppInNameOrder(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.apps = []cdk.App{tsApp("zz"), {Dir: "java", Language: cdk.Other}, fake.apps[0], {Dir: "v1", Language: cdk.TypeScript, CDKMajor: 1}}
	fake.synOf = map[string]cdk.Synthesis{"zz": {Failure: &cdk.Failure{Reason: cdk.ReasonTimeout, Tool: "cdk"}}}
	sink := &recordingSink{}

	res, err := f.runReq(fake, Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))),
		Progress: sink, SynthTimeout: 90 * time.Second})

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"skipped java unsupportedLanguage", "skipped v1 cdkV1", "waiting infra,zz", "started infra", "done infra",
		"started zz", "failed zz timeout"}
	if !slices.Equal(sink.lines, want) {
		t.Fatalf("sink:\n%s\nwant:\n%s", strings.Join(sink.lines, "\n"), strings.Join(want, "\n"))
	}
	for _, o := range fake.options {
		if o.Timeout != 90*time.Second || o.WithTrace {
			t.Fatalf("options %+v", fake.options)
		}
	}
	if res.Reflected != 1 || res.ExitCode() != 2 || len(res.CDKApps) != 4 {
		t.Fatalf("reflected %d, exit %d, apps %+v", res.Reflected, res.ExitCode(), res.CDKApps)
	}
}

// BR3.1: a resource that two apps make, or one app twice, is listed as
// ambiguous and not edited.
func TestCDKAmbiguousAcrossApps(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.apps = append(fake.apps, tsApp("other"))

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env cdkAmbiguous")
	if fake.analyzed != 0 || len(res.Files) != 0 {
		t.Fatalf("analyzed %d, files %+v", fake.analyzed, res.Files)
	}
}

// BR3.2: the reason of a resource of no app that no template has comes from
// the apps that were not synthesized; the program is named only when one
// synth failed, and a failure of the helper does not count.
func TestCDKReasonOfAResourceOfNoApp(t *testing.T) {
	failed := func(tool string) cdk.Synthesis {
		return cdk.Synthesis{Failure: &cdk.Failure{Reason: cdk.ReasonToolMissing, Tool: tool}}
	}
	for _, tc := range []struct {
		name   string
		apps   []cdk.App
		synOf  map[string]cdk.Synthesis
		helper bool
		want   string
		detail string
	}{
		{"one failed app", []cdk.App{tsApp("a"), tsApp("b")}, map[string]cdk.Synthesis{"b": failed("python")}, false, "cdkAppFailed", "python"},
		{"two failed apps", []cdk.App{tsApp("a"), tsApp("b"), tsApp("c")}, map[string]cdk.Synthesis{"b": failed("python"), "c": failed("node")}, false, "cdkAppFailed", ""},
		{"a failed app before CDK v1", []cdk.App{tsApp("a"), tsApp("b"), {Dir: "v1", Language: cdk.TypeScript, CDKMajor: 1}}, map[string]cdk.Synthesis{"b": failed("cdk")}, false, "cdkAppFailed", "cdk"},
		{"CDK v1 before another language", []cdk.App{tsApp("a"), {Dir: "v0", Language: cdk.TypeScript}, {Dir: "java", Language: cdk.Other}}, nil, false, "cdkV1", ""},
		{"another language", []cdk.App{tsApp("a"), {Dir: "java", Language: cdk.Other}}, nil, false, "cdkUnsupportedLanguage", ""},
		{"the helper only", []cdk.App{tsApp("a")}, nil, true, "templateNotFound", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, fake := newCDKWorld(t, stackTS)
			fake.apps, fake.synOf = tc.apps, tc.synOf
			if tc.helper {
				fake.analyzeErr = &cdk.HelperFailure{Reason: "noTypeScript", Tool: "typescript"}
			}

			res := f.runCDK(fake, f.plan(stackResource(arnQ2, "Q9", rename("env", "Environment", "Prod"))), false)

			requireReasons(t, res, "env "+tc.want)
			if m := res.Manual[0]; m.Detail != tc.detail || tc.detail != "" && m.DetailKind != DetailTool {
				t.Fatalf("detail %s %q", m.DetailKind, m.Detail)
			}
		})
	}
}

// tracedWorld is newCDKWorld where the source does not show where the queue is
// made, and the trace does: line 5 of lib/stack.ts.
func tracedWorld(t *testing.T) (*fixture, *fakeCDK) {
	t.Helper()
	f, fake := newCDKWorld(t, stackTS)
	full := fake.analysis
	fake.analysis.ConstructSites = []cdk.ConstructSite{full.ConstructSites[0], full.ConstructSites[2]}
	traced := full
	traced.ConstructSites = []cdk.ConstructSite{full.ConstructSites[0], full.ConstructSites[2],
		{ConstructPath: "stack/Queue", File: "lib/stack.ts", Line: 5, ConstructID: "Queue", From: "trace"}}
	fake.traced = &traced
	syn := fake.syn
	syn.TraceSites = []cdk.TraceSite{
		{ConstructPath: "stack/Other/Resource", File: "lib/stack.ts", Line: 6},
		{ConstructPath: "stack/Queue", File: "lib/stack.ts", Line: 5},
		{ConstructPath: "stack/Queue/Resource", File: "lib/stack.ts", Line: 5},
	}
	fake.traceSyn = &syn
	// The helper reads the edited sources with the trace too; an edit above
	// would move the line of the construct.
	fake.recheck = func(edited map[string][]byte) cdk.Analysis {
		after := readAgain(traced, edited)
		after.ConstructSites = slices.Clone(traced.ConstructSites)
		after.ConstructSites[2].Line = 7
		return after
	}
	return f, fake
}

// BR4.1, BR4.4, NFR3.13, security design §5 and §8: the app is synthesized
// once more with the trace when the source does not show where a resource is
// made; the second reading gets the trace sites of the missing paths (the
// parent of Resource), and so does Recheck; the edit is kept although the line
// of the construct moved.
func TestCDKTraceFindsTheMaker(t *testing.T) {
	f, fake := tracedWorld(t)
	f.commit()
	sink := &recordingSink{}

	res, err := f.runReq(fake, Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))),
		Write: true, Progress: sink})

	if err != nil || len(res.Manual) != 0 || res.Reflected != 1 {
		t.Fatalf("error %v, manual %v, reflected %d", err, reasons(res), res.Reflected)
	}
	want := []cdk.TraceSite{{ConstructPath: "stack/Queue", File: "lib/stack.ts", Line: 5}}
	if fake.synthed != 2 || !fake.options[1].WithTrace || fake.analyzed != 2 || !slices.Equal(fake.requests[1].TraceSites, want) {
		t.Fatalf("synth %d %+v, analyze %d %+v", fake.synthed, fake.options, fake.analyzed, fake.requests)
	}
	if fake.rechecked != 1 || !slices.Equal(fake.recheckTraces[0], want) {
		t.Fatalf("recheck %d %+v", fake.rechecked, fake.recheckTraces)
	}
	if res.CDKApps[0].Elapsed != 2*time.Second || res.CDKApps[0].Status != CDKAppDone {
		t.Fatalf("app %+v", res.CDKApps[0])
	}
	if got := strings.Join(sink.lines, "|"); got != "waiting infra|started infra|done infra|started infra|done infra" {
		t.Fatalf("sink %s", got)
	}
}

// BR4.3, BR4.4: a trace synth that fails or makes other stacks, or a trace
// without the missing paths, keeps the first result: the resource is listed as
// cdkNoSource, the app is done, and Analyze ran once.
func TestCDKTraceNotUsed(t *testing.T) {
	for name, set := range map[string]func(*fakeCDK){
		"the synth failed": func(fake *fakeCDK) {
			fake.traceSyn.Failure = &cdk.Failure{Reason: cdk.ReasonTimeout, Tool: "cdk"}
		},
		"other stacks": func(fake *fakeCDK) {
			fake.traceSyn.Stacks = []cdk.Stack{{Name: "stack", Resources: fake.syn.Stacks[0].Resources[:1]}}
		},
		"no trace of the path": func(fake *fakeCDK) { fake.traceSyn.TraceSites = fake.traceSyn.TraceSites[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			f, fake := tracedWorld(t)
			set(fake)
			sink := &recordingSink{}

			res, err := f.runReq(fake, Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), Progress: sink})

			if err != nil {
				t.Fatal(err)
			}
			requireReasons(t, res, "env cdkNoSource")
			if fake.synthed != 2 || fake.analyzed != 1 || res.CDKApps[0].Status != CDKAppDone || res.ExitCode() != 2 {
				t.Fatalf("synth %d, analyze %d, apps %+v", fake.synthed, fake.analyzed, res.CDKApps)
			}
			if slices.ContainsFunc(sink.lines, func(l string) bool { return strings.HasPrefix(l, "failed") }) {
				t.Fatalf("the trace synth failed the app: %v", sink.lines)
			}
			if !strings.Contains(strings.Join(res.Debug, "\n"), "cdk: infra: ") {
				t.Fatalf("debug %q", res.Debug)
			}
		})
	}
	t.Run("the helper failed with the trace", func(t *testing.T) {
		f, fake := tracedWorld(t)
		fake.tracedErr = &cdk.HelperFailure{Reason: "timeout", Tool: "helper"}

		res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

		requireReasons(t, res, "env cdkNoSource")
		if res.CDKApps[0].Status != CDKAppDone || fake.analyzed != 2 {
			t.Fatalf("apps %+v, analyze %d", res.CDKApps, fake.analyzed)
		}
	})
	t.Run("a stop during the trace synth", func(t *testing.T) {
		f, fake := tracedWorld(t)
		calls := 0

		// The first synth works; the stop comes with the second.
		res, err := (&Writer{synth: stopSecond{fakeCDK: fake, calls: &calls}, analyzer: fake}).Run(context.Background(),
			Request{PlanPath: f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), Dir: f.dir, Write: true})
		if !errors.Is(err, context.Canceled) || res != nil || f.read("infra/lib/stack.ts") != stackTS {
			t.Fatalf("error %v, result %+v", err, res)
		}
	})
}

// stopSecond stands in for synth and stops the second synth as Ctrl+C would.
type stopSecond struct {
	*fakeCDK
	calls *int
}

func (s stopSecond) Synthesize(ctx context.Context, app cdk.App, opts cdk.Options, sink cdk.ProgressSink) (cdk.Synthesis, error) {
	*s.calls++
	syn, err := s.fakeCDK.Synthesize(ctx, app, opts, sink)
	if *s.calls == 2 {
		return syn, context.Canceled
	}
	return syn, err
}

// BR3.3: a Python app is synthesized but not read: its matched changes are
// listed as not handled yet, nothing is analyzed, and the app is done.
func TestCDKPythonAppIsNotRead(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)
	fake.apps[0].Language = cdk.Python

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	requireReasons(t, res, "env cdkUnsupportedShape")
	if fake.synthed != 1 || fake.analyzed != 0 || fake.rechecked != 0 || res.CDKApps[0].Status != CDKAppDone {
		t.Fatalf("synth %d, analyze %d, recheck %d, apps %+v", fake.synthed, fake.analyzed, fake.rechecked, res.CDKApps)
	}
}

// NFR3.13: an app whose sources show every maker is synthesized and read once,
// and Recheck gets no trace sites.
func TestCDKNoTraceWhenTheSourceShowsTheMaker(t *testing.T) {
	f, fake := newCDKWorld(t, stackTS)

	res := f.runCDK(fake, f.plan(stackResource(arnQ1, "Q1", rename("env", "Environment", "Prod"))), false)

	if res.Reflected != 1 || fake.synthed != 1 || fake.analyzed != 1 || fake.rechecked != 1 || len(fake.recheckTraces[0]) != 0 {
		t.Fatalf("reflected %d, synth %d, analyze %d, recheck %+v", res.Reflected, fake.synthed, fake.analyzed, fake.recheckTraces)
	}
}

// Security design §9: the sink of the trace synth tells a failure as done
// with the same time, and is not a ToolFailureSink, so every failure reaches
// AppFailed.
func TestCDKTraceSinkReadsAFailureAsDone(t *testing.T) {
	inner := &recordingSink{}
	var sink cdk.ProgressSink = traceSink{inner: inner}
	if _, ok := sink.(cdk.ToolFailureSink); ok {
		t.Fatal("traceSink is a ToolFailureSink")
	}
	sink.AppsWaiting([]string{"a"})
	sink.AppStarted("a", "typescript")
	sink.AppFailed("a", time.Second, "timeout")
	sink.AppDone("b", time.Second)
	sink.AppSkipped("c", "cdkV1")
	if got := strings.Join(inner.lines, "|"); got != "waiting a|started a|done a|done b|skipped c cdkV1" {
		t.Fatalf("sink %s", got)
	}
}

func TestSameConstructSites(t *testing.T) {
	src := cdk.ConstructSite{ConstructPath: "s/A", File: "lib/a.ts", Line: 3, ConstructID: "A", From: "source"}
	tr := cdk.ConstructSite{ConstructPath: "s/B", File: "lib/b.ts", Line: 9, ConstructID: "B", From: "trace"}
	with := func(cs cdk.ConstructSite, f func(*cdk.ConstructSite)) cdk.ConstructSite { f(&cs); return cs }
	for _, tc := range []struct {
		name  string
		after []cdk.ConstructSite
		same  bool
	}{
		{"the same", []cdk.ConstructSite{src, tr}, true},
		{"a trace site moved", []cdk.ConstructSite{src, with(tr, func(c *cdk.ConstructSite) { c.Line = 11; c.ConstructID = "X" })}, true},
		{"a source site moved", []cdk.ConstructSite{with(src, func(c *cdk.ConstructSite) { c.Line = 4 }), tr}, false},
		{"a trace site lost", []cdk.ConstructSite{src}, false},
		{"a trace site in another file", []cdk.ConstructSite{src, with(tr, func(c *cdk.ConstructSite) { c.File = "lib/c.ts" })}, false},
		{"a trace site of another path", []cdk.ConstructSite{src, with(tr, func(c *cdk.ConstructSite) { c.ConstructPath = "s/C" })}, false},
		{"a trace site from the source", []cdk.ConstructSite{src, with(tr, func(c *cdk.ConstructSite) { c.From = "source" })}, false},
	} {
		if got := sameConstructSites([]cdk.ConstructSite{src, tr}, tc.after); got != tc.same {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
	if compareAnalyses(cdk.Analysis{ConstructSites: []cdk.ConstructSite{src, tr}}, cdk.Analysis{ConstructSites: []cdk.ConstructSite{src}}, nil) != "the construct sites changed" {
		t.Fatal("a lost trace site was not told")
	}
}

func TestSameStacks(t *testing.T) {
	stacks := func(f func([]cdk.Stack)) []cdk.Stack {
		s := []cdk.Stack{{Name: "s", Account: "1", Region: "r", Resources: []cdk.SynthesizedResource{
			{LogicalID: "A", ConstructPath: "s/A/Resource", Tags: map[string]string{"k": "v"}}}}}
		if f != nil {
			f(s)
		}
		return s
	}
	for _, tc := range []struct {
		name string
		f    func([]cdk.Stack)
		same bool
	}{
		{"the same", nil, true},
		{"other tags", func(s []cdk.Stack) { s[0].Resources[0].Tags = nil }, true},
		{"another name", func(s []cdk.Stack) { s[0].Name = "t" }, false},
		{"another account", func(s []cdk.Stack) { s[0].Account = "2" }, false},
		{"another region", func(s []cdk.Stack) { s[0].Region = "q" }, false},
		{"another logical ID", func(s []cdk.Stack) { s[0].Resources[0].LogicalID = "B" }, false},
		{"another path", func(s []cdk.Stack) { s[0].Resources[0].ConstructPath = "s/B/Resource" }, false},
	} {
		if got := sameStacks(stacks(nil), stacks(tc.f)); got != tc.same {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
	if sameStacks(stacks(nil), nil) {
		t.Error("no stacks are the same")
	}
}

func TestMakerPath(t *testing.T) {
	for in, want := range map[string]string{"s/A/Resource": "s/A", "s/A/Default": "s/A", "s/A": "s/A", "Resource": "Resource", "s/A/Res": "s/A/Res"} {
		if got := makerPath(in); got != want {
			t.Errorf("makerPath(%q) = %q", in, got)
		}
	}
}
