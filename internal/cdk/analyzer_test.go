package cdk

// Unit tests of the analyzer and of the embedded helper (written after the
// implementation). The tests named "with node" run the helper with Node.js and
// the TypeScript of the test app; the others stand in for node.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
)

// BR2.2 (example): with node, the helper finds the stack, the bucket and the
// tag of the test app, with the range of the key in UTF-8 bytes.
func TestCDKHelperReadsTheTestApp(t *testing.T) {
	app := testAppCopy(t)

	an, err := NewAnalyzer(Config{}).Analyze(context.Background(), app, AnalyzeRequest{
		ConstructPaths: []string{"tag-refactor-cdk-skeleton/Bucket/Resource"}})

	if err != nil {
		t.Fatal(err)
	}
	want := []ConstructSite{
		{ConstructPath: "tag-refactor-cdk-skeleton", File: "bin/app.ts", Line: 6, ConstructID: "tag-refactor-cdk-skeleton", From: "source"},
		{ConstructPath: "tag-refactor-cdk-skeleton/Bucket", File: "lib/stack.ts", Line: 9, ConstructID: "Bucket", From: "source"},
	}
	if !slices.Equal(an.ConstructSites, want) {
		t.Fatalf("construct sites %+v", an.ConstructSites)
	}
	if len(an.TagSites) != 1 {
		t.Fatalf("tag sites %+v", an.TagSites)
	}
	s := an.TagSites[0]
	source, err := os.ReadFile(filepath.Join(app.Abs, "lib", "stack.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if s.ScopeConstructPath != "tag-refactor-cdk-skeleton/Bucket" || s.UseLine != 14 || s.Op != "add" ||
		string(source[s.KeyEdit.Start:s.KeyEdit.End]) != "'Env'" || s.KeyEdit.Shape != ShapeLiteral || s.KeyEdit.Text != "Env" ||
		string(source[s.ValueEdit.Start:s.ValueEdit.End]) != "'dev'" || !strings.HasPrefix(string(source[s.RemoveEdit.Start:s.RemoveEdit.End]), "Tags.of(bucket)") {
		t.Fatalf("tag site %+v", s)
	}
	if !strings.HasPrefix(an.TypeScriptVersion, "5.") || len(an.Unsupported) != 0 {
		t.Fatalf("TypeScript %q, unsupported %+v", an.TypeScriptVersion, an.Unsupported)
	}
}

// stackSource is a stack class with the shapes of U1 and the ones U1 lists.
const stackSource = `import { Stack, Tags } from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import { Construct } from 'constructs';

// 日本語のコメント 🎌 before the tags: the offsets are UTF-8 bytes.
const OWNER_KEY = "Owner";

export class Pair extends Stack {
  public readonly logs: s3.Bucket;
  constructor(scope: Construct, id: string) {
    super(scope, id);
    const data = new s3.Bucket(this, 'Data');
    this.logs = new s3.Bucket(this, "Logs");
    Tags.of(data).add('Env', 'dev');
    Tags.of(this.logs).add(` + "`Env`" + `, 'd' + 'ev');
    Tags.of(this).add(OWNER_KEY, 'team-a');
    Tags.of(pick()).add('Cost', 'x');
    Tags.of(data).remove('Old');
  }
}

function pick(): Construct { throw new Error('not run'); }
`

// With node: two stacks of one class share their ranges; the shapes are
// literal, sameFileConst and expression; a tag call on an unknown scope is
// unsupported; a file that does not parse is listed; ranges are UTF-8 bytes.
func TestCDKHelperShapesAndSharing(t *testing.T) {
	app := nodeApp(t, map[string]string{
		"cdk.json":      `{"app": "npx ts-node bin/main.ts"}`,
		"bin/main.ts":   "import { App } from 'aws-cdk-lib';\nimport { Pair } from '../lib/pair';\nconst app = new App();\nnew Pair(app, 'one');\nnew Pair(app, 'two');\n",
		"lib/pair.ts":   stackSource,
		"lib/broken.ts": "export const x = ;\n",
	})

	an, err := NewAnalyzer(Config{}).Analyze(context.Background(), app, AnalyzeRequest{})

	if err != nil {
		t.Fatal(err)
	}
	source := []byte(stackSource)
	byScope := map[string]TagSite{}
	for _, s := range an.TagSites {
		byScope[s.Op+" "+s.ScopeConstructPath] = s
	}
	data, logs, stack := byScope["add one/Data"], byScope["add one/Logs"], byScope["add one"]
	if string(source[data.KeyEdit.Start:data.KeyEdit.End]) != "'Env'" || !slices.Equal(data.KeyEdit.SharedWith, []string{"two/Data"}) {
		t.Errorf("data: %+v", data)
	}
	if logs.KeyEdit.Shape != ShapeLiteral || string(source[logs.KeyEdit.Start:logs.KeyEdit.End]) != "`Env`" || logs.ValueEdit.Shape != ShapeExpression {
		t.Errorf("logs: %+v", logs)
	}
	if stack.KeyEdit.Shape != ShapeSameFileConst || string(source[stack.KeyEdit.Start:stack.KeyEdit.End]) != `"Owner"` {
		t.Errorf("stack: %+v", stack)
	}
	if removed := byScope["remove two/Data"]; removed.ValueEdit != nil || removed.KeyEdit.Text != "Old" {
		t.Errorf("remove: %+v", removed)
	}
	if len(an.TagSites) != 8 {
		t.Errorf("%d tag sites, want 4 for each stack", len(an.TagSites))
	}
	var unsupported []string
	for _, u := range an.Unsupported {
		unsupported = append(unsupported, fmt.Sprintf("%s:%d:%s", u.File, u.Line, u.Reason))
	}
	if want := []string{"lib/broken.ts:1:parseError", "lib/pair.ts:17:unsupportedShape", "lib/pair.ts:17:unsupportedShape"}; !slices.Equal(unsupported, want) {
		t.Errorf("unsupported %v, want %v", unsupported, want)
	}
}

// BR3.4: with node, Recheck reads the edited contents instead of the disk,
// which stays as it is.
func TestCDKRecheckReadsTheEditedContent(t *testing.T) {
	app := testAppCopy(t)
	path := filepath.Join(app.Abs, "lib", "stack.ts")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(before), "add('Env'", "add('Environment'", 1)

	an, err := NewAnalyzer(Config{}).Recheck(context.Background(), app, map[string][]byte{"lib/stack.ts": []byte(edited)})

	if err != nil {
		t.Fatal(err)
	}
	if len(an.TagSites) != 1 || an.TagSites[0].KeyEdit.Text != "Environment" {
		t.Fatalf("tag sites %+v", an.TagSites)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("Recheck changed the file")
	}
}

// helperRunner stands in for node: --version answers version, the helper
// answers answer.
func helperRunner(version string, answer func() (procexec.Result, error)) *fakeRunner {
	return &fakeRunner{run: func(c procexec.Command) (procexec.Result, error) {
		if slices.Equal(c.Args, []string{"--version"}) {
			return procexec.Result{Stdout: []byte(version + "\n")}, nil
		}
		return answer()
	}}
}

func answerJSON(v any) func() (procexec.Result, error) {
	return func() (procexec.Result, error) {
		data, _ := json.Marshal(v)
		return procexec.Result{Stdout: data}, nil
	}
}

// BR1.6, NFR6.4, NFR6.5: the failures of node and of the helper become a
// HelperFailure with the tool, and an answer of another version stops all.
func TestCDKHelperFailures(t *testing.T) {
	good := map[string]any{"version": 1, "typescriptVersion": "5.9.3", "tagSites": []any{}}
	cases := []struct {
		name    string
		runner  *fakeRunner
		reason  string
		tool    string
		version bool
	}{
		{"node missing", &fakeRunner{missing: map[string]bool{"node": true}}, "nodeMissing", "node", false},
		{"node too old", helperRunner("v16.20.0", answerJSON(good)), "nodeTooOld", "node", false},
		{"node answers nonsense", helperRunner("hello", answerJSON(good)), "nodeTooOld", "node", false},
		{"no typescript", helperRunner("v22.0.0", answerJSON(map[string]any{"version": 1, "error": map[string]string{"kind": "noTypeScript"}})), "noTypeScript", "typescript", false},
		{"internal", helperRunner("v22.0.0", answerJSON(map[string]any{"version": 1, "error": map[string]string{"kind": "internal"}})), "internal", "helper", false},
		{"TypeScript 3", helperRunner("v22.0.0", answerJSON(map[string]any{"version": 1, "typescriptVersion": "3.9.7"})), "typescriptVersion", "typescript", false},
		{"TypeScript 7", helperRunner("v22.0.0", answerJSON(map[string]any{"version": 1, "typescriptVersion": "7.0.2"})), "typescriptVersion", "typescript", false},
		{"not JSON", helperRunner("v22.0.0", func() (procexec.Result, error) { return procexec.Result{Stdout: []byte("{")}, nil }), "internal", "helper", false},
		{"exit 1", helperRunner("v22.0.0", func() (procexec.Result, error) { return procexec.Result{ExitCode: 1}, nil }), "internal", "helper", false},
		{"too much", helperRunner("v22.0.0", func() (procexec.Result, error) { return procexec.Result{StdoutDropped: 1}, nil }), "internal", "helper", false},
		{"timeout", helperRunner("v22.0.0", func() (procexec.Result, error) { return procexec.Result{}, procexec.ErrTimeout }), "timeout", "helper", false},
		{"node cannot start", helperRunner("v22.0.0", func() (procexec.Result, error) {
			return procexec.Result{}, &procexec.StartError{Name: "node", Err: errors.New("x")}
		}), "nodeMissing", "node", false},
		{"version 2", helperRunner("v22.0.0", answerJSON(map[string]any{"version": 2})), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAnalyzer(Config{Runner: tc.runner}).Analyze(context.Background(), App{Dir: ".", Abs: t.TempDir()}, AnalyzeRequest{})

			if tc.version {
				if !errors.Is(err, ErrHelperVersion) {
					t.Fatalf("error %v, want ErrHelperVersion", err)
				}
				return
			}
			var hf *HelperFailure
			if !errors.As(err, &hf) || hf.Reason != tc.reason || hf.Tool != tc.tool {
				t.Fatalf("error %v, want %s (%s)", err, tc.reason, tc.tool)
			}
		})
	}
	ok, err := NewAnalyzer(Config{Runner: helperRunner("v18.0.0", answerJSON(good))}).Analyze(context.Background(), App{Abs: t.TempDir()}, AnalyzeRequest{})
	if err != nil || ok.TypeScriptVersion != "5.9.3" {
		t.Fatalf("a good answer: %+v, %v", ok, err)
	}
}

// NFR2.3, NFR4.3: the helper is written to a private folder as a private file
// that matches the embedded one, sent the request on standard input, run in
// the app with the isolated environment and a limit, and the folder goes.
func TestCDKHelperIsWrittenPrivatelyAndRemoved(t *testing.T) {
	var written string
	afterHelperWritten = func(path string) {
		written = path
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		dir, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && (info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700) {
			t.Errorf("file %v, folder %v", info.Mode().Perm(), dir.Mode().Perm())
		}
	}
	t.Cleanup(func() { afterHelperWritten = nil })
	var helperCall procexec.Command
	runner := helperRunner("v22.0.0", nil)
	runner.run = func(c procexec.Command) (procexec.Result, error) {
		if slices.Equal(c.Args, []string{"--version"}) {
			return procexec.Result{Stdout: []byte("v22.0.0")}, nil
		}
		helperCall = c
		data, err := os.ReadFile(c.Args[0])
		if err != nil || string(data) != string(tsHelper) {
			t.Errorf("the helper run is not the embedded one: %v", err)
		}
		return procexec.Result{Stdout: []byte(`{"version": 1, "typescriptVersion": "4.9.5"}`)}, nil
	}
	app := App{Dir: "infra", Abs: t.TempDir()}

	_, err := NewAnalyzer(Config{Runner: runner}).Analyze(context.Background(), app, AnalyzeRequest{ConstructPaths: []string{"s/b/Resource"}})

	if err != nil {
		t.Fatal(err)
	}
	var req helperRequest
	if err := json.Unmarshal(helperCall.Stdin, &req); err != nil || req.Version != 1 || req.Mode != "analyze" || req.AppDir != app.Abs ||
		!slices.Equal(req.ConstructPaths, []string{"s/b/Resource"}) {
		t.Fatalf("request %+v, %v", req, err)
	}
	if helperCall.Dir != app.Abs || helperCall.Timeout != DefaultHelperTimeout || helperCall.StdoutLimit != helperOutputLimit {
		t.Fatalf("call %+v", helperCall)
	}
	for _, kv := range helperCall.Env {
		if strings.HasPrefix(strings.ToUpper(kv), "GH_TOKEN=") {
			t.Fatal("the helper got GH_TOKEN")
		}
	}
	if _, err := os.Stat(filepath.Dir(written)); !os.IsNotExist(err) {
		t.Fatalf("the folder of the helper was left: %v", err)
	}
}

// NFR2.3: a helper changed between writing and checking does not run.
func TestCDKChangedHelperDoesNotRun(t *testing.T) {
	afterHelperWritten = func(path string) {
		if err := os.WriteFile(path, []byte("console.log('other');"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { afterHelperWritten = nil })
	runner := helperRunner("v22.0.0", answerJSON(map[string]any{"version": 1, "typescriptVersion": "5.0.0"}))

	_, err := NewAnalyzer(Config{Runner: runner}).Analyze(context.Background(), App{Abs: t.TempDir()}, AnalyzeRequest{})

	var hf *HelperFailure
	if !errors.As(err, &hf) || hf.Reason != "internal" || hf.Tool != "helper" {
		t.Fatalf("error %v", err)
	}
	if calls := runner.recorded(); len(calls) != 1 || !slices.Equal(calls[0].Args, []string{"--version"}) {
		t.Fatalf("calls %+v", calls)
	}
}

// NFR3.4, NFR3.5 (benchmark, failing over 60 s, ND-Q4): with node, analyze and
// recheck of an app of 200 TypeScript files each start the helper once and end
// in time.
func TestCDKHelperTwoHundredFiles(t *testing.T) {
	files := map[string]string{
		"cdk.json":    `{"app": "npx ts-node bin/main.ts"}`,
		"bin/main.ts": "import { App } from 'aws-cdk-lib';\nimport { Big } from '../lib/big';\nconst app = new App();\nnew Big(app, 'big');\n",
		"lib/big.ts":  "import { Stack, Tags } from 'aws-cdk-lib';\nimport * as s3 from 'aws-cdk-lib/aws-s3';\nexport class Big extends Stack {\n  constructor(scope: any, id: string) {\n    super(scope, id);\n    const b = new s3.Bucket(this, 'B');\n    Tags.of(b).add('Env', 'dev');\n  }\n}\n",
	}
	for i := range 198 {
		var b strings.Builder
		for j := range 40 {
			fmt.Fprintf(&b, "export function f%d_%d(x: number): number {\n  return x * %d + %d;\n}\n", i, j, i, j)
		}
		files[fmt.Sprintf("lib/part%03d.ts", i)] = b.String()
	}
	app := nodeApp(t, files)
	runner := &countingRunner{Runner: procexec.System{}}
	an := NewAnalyzer(Config{Runner: runner})

	start := time.Now()
	first, err := an.Analyze(context.Background(), app, AnalyzeRequest{})
	analyzeTook := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(files["lib/big.ts"], "'Env'", "'Environment'", 1)
	start = time.Now()
	again, err := an.Recheck(context.Background(), app, map[string][]byte{"lib/big.ts": []byte(edited)})
	recheckTook := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("200 files: analyze %v, recheck %v", analyzeTook, recheckTook)
	if analyzeTook > 60*time.Second || recheckTook > 60*time.Second {
		t.Fatalf("analyze %v, recheck %v: over 60 seconds", analyzeTook, recheckTook)
	}
	if runner.helpers != 2 || len(first.TagSites) != 1 || again.TagSites[0].KeyEdit.Text != "Environment" {
		t.Fatalf("%d starts of the helper, sites %+v then %+v", runner.helpers, first.TagSites, again.TagSites)
	}
}

// countingRunner counts the starts of the helper.
type countingRunner struct {
	procexec.Runner
	helpers int
}

func (c *countingRunner) Run(ctx context.Context, cmd procexec.Command) (procexec.Result, error) {
	if len(cmd.Args) == 1 && strings.HasSuffix(cmd.Args[0], "ts-helper.js") {
		c.helpers++
	}
	return c.Runner.Run(ctx, cmd)
}
