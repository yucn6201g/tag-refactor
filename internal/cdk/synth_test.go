package cdk

// Unit tests of the environment, the discovery and the synth (written after
// the implementation). The programs are stood in for; the synth that is stood
// in for writes a cloud assembly into the folder of --output.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
)

// BR1.3, NFR2.1, NFR2.2: the environment of synth has no AWS variable (lower
// case too), no token, empty AWS files and the variables that stop calls; the
// empty files go with close.
func TestCDKIsolatedEnvironment(t *testing.T) {
	parent := []string{"PATH=/bin", "HOME=/home/u", "USERPROFILE=C:\\Users\\u", "AWS_PROFILE=sandbox",
		"aws_secret_access_key=s", "AWS_SHARED_CREDENTIALS_FILE=/home/u/.aws/credentials", "GH_TOKEN=ghp_x",
		"GITHUB_TOKEN=ghs_x", "GH_ENTERPRISE_TOKEN=a", "GITHUB_ENTERPRISE_TOKEN=b", "TAG_REFACTOR_TEST_REPO_TOKEN=c",
		"CDK_DISABLE_VERSION_CHECK=0"}

	env, err := isolate(parent)

	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, kv := range env.vars {
		name, value, _ := strings.Cut(kv, "=")
		got[strings.ToUpper(name)] = append(got[strings.ToUpper(name)], value)
	}
	for _, name := range []string{"AWS_PROFILE", "AWS_SECRET_ACCESS_KEY", "GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN",
		"GITHUB_ENTERPRISE_TOKEN", "TAG_REFACTOR_TEST_REPO_TOKEN"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s is in the environment", name)
		}
	}
	for name, want := range map[string]string{"PATH": "/bin", "HOME": "/home/u", "AWS_EC2_METADATA_DISABLED": "true",
		"CDK_DISABLE_VERSION_CHECK": "1", "CDK_DISABLE_CLI_TELEMETRY": "true"} {
		if v := got[name]; len(v) != 1 || v[0] != want {
			t.Errorf("%s = %v, want %q", name, v, want)
		}
	}
	for _, name := range []string{"AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
		info, err := os.Stat(got[name][0])
		if err != nil || info.Size() != 0 || len(got[name]) != 1 {
			t.Errorf("%s is not one empty file: %v %v", name, got[name], err)
		}
	}
	if strings.Contains(env.names(), "=") || !strings.Contains(env.names(), "PATH") {
		t.Errorf("names = %q", env.names())
	}
	env.close()
	for _, f := range env.files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s was left", f)
		}
	}
}

// BR1.1: the apps are the folders with cdk.json, in name order, without the
// folders of tools and without following links.
func TestCDKDiscoverFindsTheApps(t *testing.T) {
	root := t.TempDir()
	app := `{"app": "npx ts-node --prefer-ts-exts bin/app.ts"}`
	writeFiles(t, root, map[string]string{
		"b/cdk.json": app, "b/package.json": `{"dependencies": {"aws-cdk-lib": "2.0.0"}}`,
		"a/infra/cdk.json":        `{"app": "python3 app.py"}`,
		"node_modules/x/cdk.json": app, "cdk.out/cdk.json": app, ".git/cdk.json": app,
		"c/cdk.json": "not json",
	})
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, "b"), filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
	}
	s := NewSynthesizer(Config{Runner: &fakeRunner{}})

	apps, err := s.Discover(context.Background(), root, nil)

	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, a := range apps {
		dirs = append(dirs, a.Dir+":"+string(a.Language))
	}
	if want := []string{"a/infra:python", "b:typescript", "c:other"}; !slices.Equal(dirs, want) {
		t.Fatalf("apps %v, want %v", dirs, want)
	}
	if apps[1].CDKMajor != 2 || apps[1].Abs != filepath.Join(root, "b") || !apps[1].Synthesizable() || apps[0].Synthesizable() {
		t.Fatalf("app b: %+v", apps[1])
	}
	only, err := s.Discover(context.Background(), root, []string{"b"})
	if err != nil || len(only) != 1 || only[0].Dir != "b" {
		t.Fatalf("only b: %v, %v", only, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Discover(cancelled, root, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled discovery returned %v", err)
	}
}

// BR1.2: the language comes from the app command, the version from the first
// package.json up to --dir.
func TestCDKLanguageAndVersion(t *testing.T) {
	for command, want := range map[string]Language{
		"npx ts-node --prefer-ts-exts bin/app.ts": TypeScript, "node bin/app.js": TypeScript, "tsx app.ts": TypeScript,
		"./run.sh bin/app.ts": TypeScript, "python3 app.py": Python, "py app.py": Python, `C:\py\python.exe app.py`: Python,
		"mvn -e -q compile exec:java": Other, "go mod download && go run app.go": Other, "": Other,
	} {
		if got := languageOf(command); got != want {
			t.Errorf("languageOf(%q) = %s, want %s", command, got, want)
		}
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json":        `{"devDependencies": {"@aws-cdk/core": "1.0.0"}}`,
		"v1/cdk.json":         `{"app": "node app.js"}`,
		"v2/package.json":     `{"dependencies": {"aws-cdk-lib": "2.0.0"}}`,
		"v2/app/cdk.json":     `{"app": "node app.js"}`,
		"none/package.json":   `{"dependencies": {}}`,
		"none/cdk.json":       `{"app": "node app.js"}`,
		"broken/package.json": `{`,
		"broken/cdk.json":     `{"app": "node app.js"}`,
	})
	for dir, want := range map[string]int{"v1": 1, "v2/app": 2, "none": 0, "broken": 0} {
		if got := cdkMajor(root, filepath.Join(root, filepath.FromSlash(dir))); got != want {
			t.Errorf("cdkMajor(%s) = %d, want %d", dir, got, want)
		}
	}
	if got := cdkMajor(filepath.Join(root, "v1"), filepath.Join(root, "v1")); got != 0 {
		t.Errorf("the search went above --dir: %d", got)
	}
}

// writeAssembly writes the cloud assembly synth would write to the folder of
// --output.
func writeAssembly(t *testing.T, c procexec.Command, files map[string]string) {
	t.Helper()
	i := slices.Index(c.Args, "--output")
	if i < 0 || i+1 >= len(c.Args) {
		t.Fatalf("no --output in %v", c.Args)
	}
	writeFiles(t, c.Args[i+1], files)
}

const twoStacks = `{"version": "55.0.0", "artifacts": {
  "tree": {"type": "cdk:tree"},
  "web": {"type": "aws:cloudformation:stack", "environment": "aws://111122223333/ap-northeast-1",
    "properties": {"templateFile": "web.template.json", "stackName": "web-prod"}},
  "skeleton": {"type": "aws:cloudformation:stack", "environment": "aws://unknown-account/unknown-region",
    "properties": {"templateFile": "skeleton.template.json"}}}}`

const webTemplate = `{"Resources": {
  "Role1": {"Type": "AWS::IAM::Role", "Properties": {"Tags": [{"Key": "Env", "Value": "dev"}, {"Key": "n", "Value": 1}]},
    "Metadata": {"aws:cdk:path": "web-prod/Role/Resource"}},
  "Param": {"Type": "AWS::SSM::Parameter", "Properties": {"Tags": {"Env": "dev", "x": {"Ref": "y"}}}},
  "Plain": {"Type": "AWS::SQS::Queue"}}}`

// BR1.4, NFR4.3: synth runs into a temporary folder, the stacks are read from
// the manifest and the templates, and the folder and the files of the
// environment are gone afterwards.
func TestCDKSynthesizeReadsTheAssembly(t *testing.T) {
	var out string
	var envFiles []string
	runner := &fakeRunner{run: func(c procexec.Command) (procexec.Result, error) {
		out = c.Args[slices.Index(c.Args, "--output")+1]
		for _, kv := range c.Env {
			if name, value, _ := strings.Cut(kv, "="); name == "AWS_CONFIG_FILE" || name == "AWS_SHARED_CREDENTIALS_FILE" {
				envFiles = append(envFiles, value)
			}
		}
		writeAssembly(t, c, map[string]string{"manifest.json": twoStacks, "web.template.json": webTemplate,
			"skeleton.template.json": `{"Resources": {}}`})
		return procexec.Result{}, nil
	}}
	sink := &recordingSink{}
	s := NewSynthesizer(Config{Runner: runner, Now: fixedClock()})
	app := App{Dir: "infra", Abs: t.TempDir(), Language: TypeScript, CDKMajor: 2}

	syn, err := s.Synthesize(context.Background(), app, Options{}, sink)

	if err != nil || syn.Failure != nil {
		t.Fatalf("Synthesize: %v, %+v", err, syn.Failure)
	}
	if len(syn.Stacks) != 2 || syn.Stacks[0].Name != "skeleton" || syn.Stacks[1].Name != "web-prod" {
		t.Fatalf("stacks %+v", syn.Stacks)
	}
	if s0, s1 := syn.Stacks[0], syn.Stacks[1]; s0.Account != "" || s0.Region != "" || s1.Account != "111122223333" || s1.Region != "ap-northeast-1" {
		t.Fatalf("environments %+v", syn.Stacks)
	}
	res := syn.Stacks[1].Resources
	if len(res) != 3 || res[0].LogicalID != "Param" || res[2].ConstructPath != "web-prod/Role/Resource" ||
		res[2].Tags["Env"] != "dev" || len(res[2].Tags) != 1 || res[0].Tags["Env"] != "dev" || len(res[0].Tags) != 1 || len(res[1].Tags) != 0 {
		t.Fatalf("resources %+v", res)
	}
	if want := "started infra typescript\ndone infra 10s"; sink.got() != want {
		t.Fatalf("sink:\n%s\nwant:\n%s", sink.got(), want)
	}
	c := runner.recorded()[0]
	if c.Dir != app.Abs || !slices.Equal(c.Args[:3], []string{"synth", "--quiet", "--no-notices"}) || c.Timeout != DefaultSynthTimeout {
		t.Fatalf("command %+v", c)
	}
	for _, p := range append(envFiles, out) {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s was left", p)
		}
	}
}

// BR1.5: the failures are sorted into their reasons, the detail keeps only the
// known lines, and the sink gets the reason without the detail.
func TestCDKSynthesizeSortsTheFailures(t *testing.T) {
	cases := []struct {
		name    string
		missing map[string]bool
		res     procexec.Result
		err     error
		files   map[string]string
		reason  FailureReason
		tool    string
	}{
		{"needs AWS", nil, procexec.Result{ExitCode: 1, Stderr: []byte("Unable to locate credentials\n")}, nil, nil, ReasonNeedsAWS, "cdk"},
		{"needs AWS calls", nil, procexec.Result{ExitCode: 1, Stderr: []byte("Error: Need to perform AWS calls for account 111122223333\n")}, nil, nil, ReasonNeedsAWS, "cdk"},
		{"synth error", nil, procexec.Result{ExitCode: 1, Stderr: []byte("lib/stack.ts(3,1): error TS1005: ';' expected.\n  const x = 'secret source'\n")}, nil, nil, ReasonSynthError, "cdk"},
		{"timeout", nil, procexec.Result{}, procexec.ErrTimeout, nil, ReasonTimeout, "cdk"},
		{"cdk cannot start", nil, procexec.Result{}, &procexec.StartError{Name: "cdk", Err: errors.New("x")}, nil, ReasonToolMissing, "cdk"},
		{"other error", nil, procexec.Result{}, errors.New("broken pipe"), nil, ReasonSynthError, "cdk"},
		{"no node", map[string]bool{"node": true}, procexec.Result{}, nil, nil, ReasonToolMissing, "node"},
		{"no cdk", map[string]bool{"cdk": true}, procexec.Result{}, nil, nil, ReasonToolMissing, "cdk"},
		{"no manifest", nil, procexec.Result{}, nil, map[string]string{}, ReasonSynthError, "cdk"},
		{"bad template name", nil, procexec.Result{}, nil, map[string]string{"manifest.json": `{"artifacts": {"s": {"type": "aws:cloudformation:stack",
			"properties": {"templateFile": "../s.json"}}}}`}, ReasonSynthError, "cdk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{missing: tc.missing, run: func(c procexec.Command) (procexec.Result, error) {
				writeAssembly(t, c, tc.files)
				return tc.res, tc.err
			}}
			sink := &recordingSink{}
			s := NewSynthesizer(Config{Runner: runner, Now: fixedClock()})

			syn, err := s.Synthesize(context.Background(), App{Dir: "a", Abs: t.TempDir(), Language: TypeScript, CDKMajor: 2}, Options{}, sink)

			if err != nil || syn.Failure == nil || syn.Failure.Reason != tc.reason || syn.Failure.Tool != tc.tool {
				t.Fatalf("got %v, %+v", err, syn.Failure)
			}
			if !strings.HasSuffix(sink.got(), "failed a 10s "+string(tc.reason)) {
				t.Fatalf("sink: %s", sink.got())
			}
			if strings.Contains(syn.Failure.Detail, "secret source") || strings.Contains(syn.Failure.Detail, "111122223333") {
				t.Fatalf("detail %q", syn.Failure.Detail)
			}
		})
	}
}

// BR1.2: an app that is not TypeScript with CDK v2 is skipped without running
// anything.
func TestCDKSynthesizeSkipsOtherApps(t *testing.T) {
	for _, tc := range []struct {
		app    App
		reason FailureReason
	}{
		{App{Dir: "py", Language: Python, CDKMajor: 2}, ReasonUnsupportedLanguage},
		{App{Dir: "v1", Language: TypeScript, CDKMajor: 1}, ReasonCDKV1},
		{App{Dir: "v0", Language: TypeScript}, ReasonUnknownVersion},
	} {
		runner := &fakeRunner{}
		sink := &recordingSink{}

		syn, err := NewSynthesizer(Config{Runner: runner}).Synthesize(context.Background(), tc.app, Options{}, sink)

		if err != nil || syn.Failure == nil || syn.Failure.Reason != tc.reason || len(runner.recorded()) != 0 {
			t.Fatalf("%s: %v, %+v, %d calls", tc.app.Dir, err, syn.Failure, len(runner.recorded()))
		}
		if sink.got() != "skipped "+tc.app.Dir+" "+string(tc.reason) {
			t.Fatalf("sink: %s", sink.got())
		}
	}
}

// Ctrl+C: a cancelled context is the error of Synthesize, not a failure of
// the app, and the temporary folder goes all the same.
func TestCDKSynthesizeStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out string
	runner := &fakeRunner{run: func(c procexec.Command) (procexec.Result, error) {
		out = c.Args[slices.Index(c.Args, "--output")+1]
		cancel()
		return procexec.Result{}, context.Canceled
	}}

	_, err := NewSynthesizer(Config{Runner: runner}).Synthesize(ctx, App{Dir: "a", Abs: t.TempDir(), Language: TypeScript, CDKMajor: 2}, Options{}, nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("%s was left", out)
	}
}

// NFR6.6: the cdk of the app comes before the one of PATH.
func TestCDKUsesTheCDKOfTheApp(t *testing.T) {
	dir := t.TempDir()
	name := "cdk"
	if runtime.GOOS == "windows" {
		name = "cdk.cmd"
	}
	local := filepath.Join(dir, "node_modules", ".bin", name)
	writeFiles(t, dir, map[string]string{"node_modules/.bin/" + name: ""})
	runner := &fakeRunner{run: func(c procexec.Command) (procexec.Result, error) {
		writeAssembly(t, c, map[string]string{"manifest.json": `{"artifacts": {}}`})
		return procexec.Result{}, nil
	}}

	_, err := NewSynthesizer(Config{Runner: runner}).Synthesize(context.Background(), App{Dir: ".", Abs: dir, Language: TypeScript, CDKMajor: 2}, Options{Timeout: 5}, nil)

	if err != nil {
		t.Fatal(err)
	}
	if c := runner.recorded()[0]; c.Name != local || c.Timeout != 5 {
		t.Fatalf("ran %s with %v", c.Name, c.Timeout)
	}
}

func TestCDKParseEnvironment(t *testing.T) {
	for env, want := range map[string][2]string{
		"aws://111122223333/us-east-1":         {"111122223333", "us-east-1"},
		"aws://unknown-account/unknown-region": {"", ""},
		"aws://111122223333/unknown-region":    {"111122223333", ""},
		"":                                     {"", ""},
	} {
		if a, r := parseEnvironment(env); a != want[0] || r != want[1] {
			t.Errorf("parseEnvironment(%q) = %q, %q", env, a, r)
		}
	}
}
