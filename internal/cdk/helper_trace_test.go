package cdk

// Unit tests of the places of the trace in the helper (v0.2.0 U3 BR4.4,
// contract C3 construct_rules, security design §5 (a) to (e)), written after
// the implementation. They run the real helper with Node.js on a copy of
// ts-trace, with the node_modules of ts-minimal.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

const (
	traceStack  = "tag-refactor-cdk-trace"
	traceBucket = traceStack + "/Logs"
)

// traceAppCopy copies ts-trace with the node_modules of ts-minimal and writes
// files into the copy.
func traceAppCopy(t *testing.T, files map[string]string) App {
	t.Helper()
	modules := testutil.CDKTestApp("ts-minimal")
	if !testutil.RequireCDKTools(t, modules) {
		t.FailNow()
	}
	dir := realTemp(t)
	testutil.CopyCDKApp(t, testutil.CDKTestApp("ts-trace"), dir)
	testutil.LinkNodeModules(t, modules, dir)
	writeFiles(t, dir, files)
	return App{Dir: ".", Abs: dir, Language: TypeScript, CDKMajor: 2}
}

// sitesOf returns the construct sites of path.
func sitesOf(an Analysis, path string) []ConstructSite {
	var out []ConstructSite
	for _, s := range an.ConstructSites {
		if s.ConstructPath == path {
			out = append(out, s)
		}
	}
	return out
}

// (a) to (c): with node, a TraceSite gives the place made in a function the
// source does not follow, at the line of the trace; a path the source finds
// stays from the source; a line with no new, a file that is not a source and
// a file that does not parse give nothing.
func TestCDKHelperUsesTheTrace(t *testing.T) {
	app := traceAppCopy(t, map[string]string{
		"lib/broken.ts": "export const x = ;\nnew Thing(this, 'Broken');\n",
		"notes.txt":     "new s3.Bucket(this, 'Notes');\n",
	})

	an, err := NewAnalyzer(Config{}).Analyze(context.Background(), app, AnalyzeRequest{TraceSites: []TraceSite{
		{ConstructPath: traceStack, File: "bin/app.ts", Line: 6},
		{ConstructPath: traceBucket, File: "lib/stack.ts", Line: 8},
		{ConstructPath: traceStack + "/NoNew", File: "lib/stack.ts", Line: 9},
		{ConstructPath: traceStack + "/Notes", File: "notes.txt", Line: 1},
		{ConstructPath: traceStack + "/Broken", File: "lib/broken.ts", Line: 2},
		{ConstructPath: traceStack + "/Zero", File: "lib/stack.ts", Line: 0},
	}})

	if err != nil {
		t.Fatal(err)
	}
	if got := sitesOf(an, traceBucket); len(got) != 1 || got[0] != (ConstructSite{ConstructPath: traceBucket, File: "lib/stack.ts", Line: 8,
		ConstructID: "Logs", From: "trace"}) {
		t.Fatalf("(a) the bucket: %+v", got)
	}
	if got := sitesOf(an, traceStack); len(got) != 1 || got[0].From != "source" || got[0].File != "bin/app.ts" {
		t.Fatalf("(b) the stack: %+v", got)
	}
	for _, p := range []string{"/NoNew", "/Notes", "/Broken", "/Zero"} {
		if got := sitesOf(an, traceStack+p); len(got) != 0 {
			t.Errorf("(c) %s: %+v", p, got)
		}
	}
	if last := an.ConstructSites[len(an.ConstructSites)-1]; last.From != "trace" {
		t.Errorf("the places of the trace do not come after those of the source: %+v", an.ConstructSites)
	}
}

// (d), (e): with node, Recheck with the same TraceSites finds the place again
// when the lines moved, by the string ID; with an ID that is not a string it
// finds nothing.
func TestCDKHelperRecheckFollowsTheTrace(t *testing.T) {
	app := traceAppCopy(t, nil)
	path := filepath.Join(app.Abs, "lib", "stack.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	traces := []TraceSite{{ConstructPath: traceBucket, File: "lib/stack.ts", Line: 8}}
	moved := strings.Replace(source, "function makeBucket", "// one\n// two\nfunction makeBucket", 1)
	notString := strings.Replace(moved, "'Logs'", "logsID", 1)
	notString = strings.Replace(notString, "// one\n", "const logsID = 'Logs';\n", 1)
	an := NewAnalyzer(Config{})

	got, err := an.Recheck(context.Background(), app, map[string][]byte{"lib/stack.ts": []byte(moved)}, traces)
	if err != nil {
		t.Fatal(err)
	}
	if sites := sitesOf(got, traceBucket); len(sites) != 1 || sites[0].Line != 10 || sites[0].File != "lib/stack.ts" || sites[0].From != "trace" {
		t.Fatalf("(d) %+v", sites)
	}

	got, err = an.Recheck(context.Background(), app, map[string][]byte{"lib/stack.ts": []byte(notString)}, traces)
	if err != nil {
		t.Fatal(err)
	}
	if sites := sitesOf(got, traceBucket); len(sites) != 0 {
		t.Fatalf("(e) %+v", sites)
	}
	if after, _ := os.ReadFile(path); string(after) != source {
		t.Fatal("Recheck changed the file")
	}
}
