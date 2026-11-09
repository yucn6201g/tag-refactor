package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/iac"
)

// Unit tests of the iac command (U8 BR1.1, BR6.2, BR6.3, NFR3.16), written after
// the implementation. The writer is stood in for: its own tests are in
// internal/iac.

func runIacStub(t *testing.T, run iacRunner, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), append([]string{"iac", "--lang", "en"}, args...), &stdout, &stderr, Deps{iac: run})
	return code, stdout.String(), stderr.String()
}

func TestIacPassesItsOptionsOn(t *testing.T) {
	var got iac.Request
	code, _, _ := runIacStub(t, func(_ context.Context, r iac.Request) (*iac.Result, error) {
		got = r
		return &iac.Result{}, nil
	}, "--plan", "p.json", "--dir", "d", "--tf-state", "s.json", "--write")
	if code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	// v0.2.0 C6: the printer receives the states of the CDK apps.
	if got.Progress == nil {
		t.Fatal("the request has no progress sink")
	}
	got.Progress = nil
	if got != (iac.Request{PlanPath: "p.json", Dir: "d", TFStatePath: "s.json", Write: true}) {
		t.Fatalf("request = %+v", got)
	}
}

func TestIacExitCodesAndTheirMeanings(t *testing.T) {
	for name, tc := range map[string]struct {
		res     *iac.Result
		code    int
		meaning string
	}{
		"clean":       {&iac.Result{}, ExitOK, "Exit code: 0 (nothing to fix by hand)"},
		"manual":      {&iac.Result{Manual: []iac.ManualItem{{Reason: iac.ReasonNotLiteral}}}, ExitNeedsReview, "Exit code: 2 (some changes to fix by hand)"},
		"interrupted": {&iac.Result{Interrupted: true}, ExitError, "Exit code: 1 (the files were not all written)"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runIacStub(t, func(context.Context, iac.Request) (*iac.Result, error) { return tc.res, nil }, "--plan", "p", "--dir", "d")
			if code != tc.code || !strings.Contains(stderr, tc.meaning) {
				t.Fatalf("exit %d, stderr:\n%s", code, stderr)
			}
		})
	}
}

// A stopped --write shows the result and the two lines of the error.
func TestIacWriteErrorComesAfterTheResult(t *testing.T) {
	stop := &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, DetailID: "iac.notCommitted", DetailArgs: []any{"a.yaml"}, HintID: "iac.gitFix"}
	code, stdout, stderr := runIacStub(t, func(context.Context, iac.Request) (*iac.Result, error) {
		return &iac.Result{WriteRequested: true, WriteErr: stop, Files: []iac.FileResult{{File: "a.yaml", Reflected: 1, Diff: "--- a/a.yaml"}}}, nil
	}, "--plan", "p", "--dir", "d", "--write")
	if code != ExitError {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "--- a/a.yaml") || !strings.Contains(stdout, "a.yaml: 1 change not written") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Error: a.yaml has changes that are not committed") || !strings.Contains(stderr, "  Fix: nothing was written") {
		t.Fatalf("stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Exit code: 1 (the files were not all written)") {
		t.Fatalf("stderr:\n%s", stderr)
	}
}

func TestIacAnErrorOfTheWriterHasNoResult(t *testing.T) {
	code, stdout, stderr := runIacStub(t, func(context.Context, iac.Request) (*iac.Result, error) {
		return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, DetailID: "iac.tooManyFiles", DetailArgs: []any{20000}, HintID: "iac.tooManyFilesFix"}
	}, "--plan", "p", "--dir", "d")
	if code != ExitError || stdout != "" || !strings.Contains(stderr, "--dir holds more than 20000 candidate files") {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", code, stdout, stderr)
	}
}

func TestIacAnUnclassifiedErrorSaysLittle(t *testing.T) {
	code, _, stderr := runIacStub(t, func(context.Context, iac.Request) (*iac.Result, error) {
		return nil, errors.New("SECRET in the text")
	}, "--plan", "p", "--dir", "d")
	if code != ExitError || strings.Contains(stderr, "SECRET") {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
}

func TestIacCtrlCBeforeWritingIsAnInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"iac", "--lang", "en", "--plan", "p", "--dir", "d"}, &stdout, &stderr, Deps{iac: func(ctx context.Context, _ iac.Request) (*iac.Result, error) {
		return nil, ctx.Err()
	}})
	if code != ExitError || !strings.Contains(stderr.String(), "Exit code: 1") {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr.String())
	}
}

func TestIacInJapanese(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"iac", "--lang", "ja", "--plan", "p", "--dir", "d"}, &stdout, &stderr, Deps{iac: func(context.Context, iac.Request) (*iac.Result, error) {
		return &iac.Result{AlreadyReflected: 3}, nil
	}})
	if code != ExitOK || !strings.Contains(stdout.String(), "反映済みの変更: 3件") || !strings.Contains(stderr.String(), "手で直すものはありません") {
		t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	var out2, err2 bytes.Buffer
	if code := Run(context.Background(), []string{"iac", "--lang", "ja", "--plan", "p"}, &out2, &err2, Deps{}); code != ExitError || !strings.Contains(err2.String(), "--dir で渡してください") {
		t.Fatalf("exit %d\n%s", code, err2.String())
	}
}

// NFR3.16: the writer has nothing to hold an AWS connection or a writer of AWS,
// and the command puts it together with no such thing. What the command takes
// from Deps for AWS stays out of it: a stand-in shows the request only. Since
// v0.2.0 the writer holds the synthesizer and the analyzer of CDK (contract
// C2, C3), and the request the sink of their states (C6): nothing else.
func TestIacIsPutTogetherWithoutAWS(t *testing.T) {
	writer := reflect.TypeFor[iac.Writer]()
	for i := range writer.NumField() {
		f := writer.Field(i)
		if f.Type.Kind() != reflect.Interface || !strings.HasPrefix(f.Name, "synth") && !strings.HasPrefix(f.Name, "analyzer") {
			t.Fatalf("the writer holds %s of type %s", f.Name, f.Type)
		}
		for j := range f.Type.NumMethod() {
			if m := f.Type.Method(j).Type.String(); strings.Contains(m, "awsaccess") || strings.Contains(m, "aws-sdk") {
				t.Fatalf("the writer's %s reaches AWS: %s", f.Name, m)
			}
		}
	}
	if n := reflect.TypeOf(iac.New).NumIn(); n != 0 {
		t.Fatalf("the writer takes %d values", n)
	}
	options := reflect.TypeFor[iac.Options]()
	for i := range options.NumField() {
		if name := options.Field(i).Type.String(); strings.Contains(name, "awsaccess") || strings.Contains(name, "aws.") {
			t.Fatalf("the options of the writer hold %s", name)
		}
	}
	run := reflect.TypeFor[iacRunner]()
	for i := range run.NumIn() {
		if name := run.In(i).String(); strings.Contains(name, "awsaccess") {
			t.Fatalf("the runner takes %s", name)
		}
	}
	for i := range reflect.TypeFor[iac.Request]().NumField() {
		f := reflect.TypeFor[iac.Request]().Field(i)
		if k := f.Type.Kind(); k != reflect.String && k != reflect.Bool && f.Type.String() != "cdk.ProgressSink" {
			t.Fatalf("the request holds %s of type %s", f.Name, f.Type)
		}
	}
}

func TestIacIsInTheHelpOfTheTool(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"--help"}, &stdout, &stderr, Deps{}); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "iac ") || !strings.Contains(stdout.String(), "without\ncalling AWS") {
		t.Fatalf("help:\n%s", stdout.String())
	}
}
