package cdk

// Unit tests of the trace of cdk synth --debug (v0.2.0 U3 BR4.2, security
// design §5), written after the implementation. The paths are built in the
// form of the OS the tests run on; the form of the other OS is dropped, so the
// CI of the three OSes checks both forms.

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// otherOSFile is an absolute path of the other OS, which is never absolute
// here.
func otherOSFile() string {
	if runtime.GOOS == "windows" {
		return "/home/alice/app/lib/stack.ts"
	}
	return `C:\Users\alice\app\lib\stack.ts`
}

// fileURL is the file:// form of an absolute path of this OS.
func fileURL(p string) string {
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return "file://" + s
}

func TestTraceFrame(t *testing.T) {
	windows := runtime.GOOS == "windows"
	file := filepath.Join(t.TempDir(), "app", "lib", "stack.ts")
	for _, tc := range []struct {
		line string
		file string
		n    int
		ok   bool
	}{
		{"    at makeBucket (" + file + ":8:12)", file, 8, true},
		{"at " + file + ":19:5", file, 19, true},
		{file + ":3", file, 3, true},
		{"at Object.<anonymous> (" + fileURL(file) + ":3:1)", file, 3, true},
		{"at x (" + file + ":0:1)", file, 0, true},
		{"at x (" + otherOSFile() + ":12:5)", "", 0, false},
		{"at new Stack (node:internal/modules/cjs/loader:1:1)", "", 0, false},
		{"at lib/stack.ts:3:1", "", 0, false},
		{"at <anonymous>", "", 0, false},
		{"at x (file://%zz/a.ts:1:1)", "", 0, false},
		{"", "", 0, false},
	} {
		f, n, ok := traceFrame(tc.line, windows)
		if f != tc.file || n != tc.n || ok != tc.ok {
			t.Errorf("traceFrame(%q) = %q, %d, %v; want %q, %d, %v", tc.line, f, n, ok, tc.file, tc.n, tc.ok)
		}
	}
}

func TestTraceUserFile(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	in := func(parts ...string) string { return filepath.Join(append([]string{app}, parts...)...) }
	for _, tc := range []struct {
		file    string
		windows bool
		want    string
		ok      bool
	}{
		{in("lib", "stack.ts"), false, "lib/stack.ts", true},
		{in("..", "app", "bin", "app.ts"), false, "bin/app.ts", true},
		{app, false, "", false},
		{filepath.Join(root, "other", "x.ts"), false, "", false},
		{filepath.Join(root, "app2", "x.ts"), false, "", false},
		{in("node_modules", "aws-cdk-lib", "core", "lib", "x.js"), false, "", false},
		{in(".venv", "lib", "x.py"), false, "", false},
		{in("venv", "x.py"), false, "", false},
		{in("lib", "python3.12", "site-packages", "x.py"), false, "", false},
		{in("Node_Modules", "x.js"), true, "", false},
		{in("Node_Modules", "x.js"), false, "Node_Modules/x.js", true},
	} {
		got, ok := userFile(tc.file, app, tc.windows)
		if got != tc.want || ok != tc.ok {
			t.Errorf("userFile(%q, %v) = %q, %v; want %q, %v", tc.file, tc.windows, got, ok, tc.want, tc.ok)
		}
	}
}

// writeJSON writes v as JSON into dir.
func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{name: string(data)})
}

// BR4.2: for each construct path, the first frame in a source of the user is
// used, from the metadata file of a stack and from the manifest, in the order
// of the paths; the other lines are counted.
func TestTraceReadSites(t *testing.T) {
	out := t.TempDir()
	app := filepath.Join(t.TempDir(), "app")
	at := func(name string, line string, parts ...string) string {
		return "at " + name + " (" + filepath.Join(append([]string{app}, parts...)...) + ":" + line + ":1)"
	}
	stack := func(lines ...string) []map[string]any {
		return []map[string]any{{"type": creationStack, "data": lines}}
	}
	writeJSON(t, out, "manifest.json", map[string]any{"artifacts": map[string]any{
		"tree": map[string]any{"type": "cdk:tree"},
		"b":    map[string]any{"type": "aws:cloudformation:stack", "additionalMetadataFile": "b.metadata.json"},
		"a": map[string]any{"type": "aws:cloudformation:stack", "metadata": map[string]any{
			"/a/Queue": []map[string]any{{"type": "aws:cdk:logicalId", "data": "Queue", "trace": []string{at("q", "3", "bin", "app.ts")}}},
		}},
	}})
	writeJSON(t, out, "b.metadata.json", map[string]any{
		"/b/Logs/Resource": stack(at("new Bucket", "1", "node_modules", "aws-cdk-lib", "x.js"), at("makeBucket", "8", "lib", "stack.ts"),
			at("new B", "19", "lib", "stack.ts")),
		"/b/Zero":  stack(at("x", "0", "lib", "stack.ts")),
		"/b/Other": []map[string]any{{"type": "aws:cdk:logicalId", "data": "Other"}},
		"/a/Queue": stack(at("again", "5", "lib", "stack.ts")),
		"/b/Far":   stack("at y (" + otherOSFile() + ":3:1)"),
	})

	sites, dropped, err := readTraceSites(out, app, runtime.GOOS == "windows")

	if err != nil {
		t.Fatal(err)
	}
	want := []TraceSite{{ConstructPath: "a/Queue", File: "bin/app.ts", Line: 3}, {ConstructPath: "b/Logs/Resource", File: "lib/stack.ts", Line: 8}}
	if !slices.Equal(sites, want) {
		t.Fatalf("sites %+v, want %+v", sites, want)
	}
	// node_modules and the line after the one used, line 0, the second
	// /a/Queue and the path of the other OS.
	if dropped != 5 {
		t.Fatalf("dropped %d, want 5", dropped)
	}
	for _, s := range sites {
		if filepath.IsAbs(s.File) || strings.Contains(s.File, app) {
			t.Fatalf("an absolute path in %+v", s)
		}
	}
}

// A trace that cannot be read is an error with no site.
func TestTraceUnreadable(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no manifest":       {},
		"manifest not JSON": {"manifest.json": "{"},
		"metadata outside": {"manifest.json": `{"artifacts": {"s": {"type": "aws:cloudformation:stack",
			"additionalMetadataFile": "../s.json"}}}`},
		"metadata missing": {"manifest.json": `{"artifacts": {"s": {"type": "aws:cloudformation:stack",
			"additionalMetadataFile": "s.json"}}}`},
		"metadata not JSON": {"manifest.json": `{"artifacts": {"s": {"type": "aws:cloudformation:stack",
			"additionalMetadataFile": "s.json"}}}`, "s.json": "["},
	} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			writeFiles(t, out, files)

			sites, _, err := readTraceSites(out, t.TempDir(), runtime.GOOS == "windows")

			if err == nil || sites != nil {
				t.Fatalf("sites %+v, error %v", sites, err)
			}
		})
	}
}
