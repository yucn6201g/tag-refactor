package cli

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Unit tests of Ctrl+C (U4 NFR design §3, NFR8.17), written after the
// implementation. The outer context of Run is cancelled while the regions
// are read, while the impact analysis reads policies, and while the file is
// saved: the command says it was interrupted, prints no result, exits 1 and
// leaves no temporary file.

const interruptRules = "version: 1\nkeys:\n  Environment:\n    aliases: [env]\n"

type interruptWorld struct {
	t     *testing.T
	fake  *testutil.FakeAWS
	dir   string
	rules string
}

func newInterruptWorld(t *testing.T) *interruptWorld {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	dir := t.TempDir()
	rules := filepath.Join(t.TempDir(), "tagrules.yaml")
	if err := os.WriteFile(rules, []byte(interruptRules), 0o600); err != nil {
		t.Fatal(err)
	}
	return &interruptWorld{t: t, fake: fake, dir: dir, rules: rules}
}

func (w *interruptWorld) resources() {
	w.fake.Handle("tag:GetResources", testutil.TagPages([]testutil.TaggedResource{{ARN: "arn:aws:sqs:us-east-1:123456789012:q", Tags: map[string]string{"env": "dev"}}}))
}

// policies gives the account a customer managed policy P. h, when not nil,
// answers the read of the IAM policies instead.
func (w *interruptWorld) policies(h testutil.Handler) {
	w.fake.HandlePolicyAccount(&testutil.PolicyAccount{Managed: []testutil.ManagedPolicyDoc{
		{Name: "P", Document: `{"Version":"2012-10-17","Statement":[]}`},
	}})
	if h != nil {
		w.fake.Handle(testutil.OpGetAccountAuthorizationDetails, h)
	}
}

type runResult struct {
	code           int
	stdout, stderr string
}

func (w *interruptWorld) run(ctx context.Context, hook func(), args ...string) runResult {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, args, &stdout, &stderr, Deps{Transport: w.fake, RetryDelay: func(int) time.Duration { return 0 }, savingHook: hook})
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func (w *interruptWorld) requireInterrupted(r runResult) {
	w.t.Helper()
	if r.code != ExitError {
		w.t.Fatalf("exit code %d, want 1\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stderr, "Interrupted. Nothing was saved.\nExit code: 1 (interrupted)\n") {
		w.t.Fatalf("stderr does not say it was interrupted:\n%s", r.stderr)
	}
	if strings.Contains(r.stderr, "Error:") {
		w.t.Fatalf("an interruption is not reported as an error:\n%s", r.stderr)
	}
	if r.stdout != "" {
		w.t.Fatalf("stdout must be empty, got:\n%s", r.stdout)
	}
	if entries, err := os.ReadDir(w.dir); err != nil || len(entries) != 0 {
		w.t.Fatalf("the folder of the file holds %v (err %v), want nothing", entries, err)
	}
	if n := w.fake.InFlight(); n != 0 {
		w.t.Fatalf("%d AWS calls are still running", n)
	}
}

// cancelOn returns a handler that cancels the run, then waits for the
// request to give up, as a slow AWS answer would.
func cancelOn(cancel context.CancelFunc) testutil.Handler {
	return func(testutil.Call) testutil.Reply {
		cancel()
		return testutil.Reply{Delay: time.Minute}
	}
}

func TestCtrlCWhileTheRegionsAreRead(t *testing.T) {
	for _, args := range [][]string{
		{"plan", "--region", "us-east-1", "--out", "OUT/plan.json"},
		{"rules", "suggest", "--region", "us-east-1", "--output", "OUT/tagrules.yaml"},
	} {
		t.Run(args[0], func(t *testing.T) {
			w := newInterruptWorld(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w.fake.Handle("tag:GetResources", cancelOn(cancel))
			if args[0] == "plan" {
				args = append(args, "--rules", w.rules)
			}

			r := w.run(ctx, nil, w.resolve(args)...)

			w.requireInterrupted(r)
		})
	}
}

func TestCtrlCWhileTheImpactAnalysisReadsPolicies(t *testing.T) {
	w := newInterruptWorld(t)
	w.resources()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.policies(cancelOn(cancel))

	r := w.run(ctx, nil, w.resolve([]string{"plan", "--region", "us-east-1", "--rules", w.rules, "--out", "OUT/plan.json"})...)

	w.requireInterrupted(r)
}

func TestCtrlCWhileTheFileIsSaved(t *testing.T) {
	for _, args := range [][]string{
		{"plan", "--region", "us-east-1", "--out", "OUT/plan.json"},
		{"rules", "suggest", "--region", "us-east-1", "--output", "OUT/tagrules.yaml"},
	} {
		t.Run(args[0], func(t *testing.T) {
			w := newInterruptWorld(t)
			w.resources()
			w.policies(nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if args[0] == "plan" {
				args = append(args, "--rules", w.rules)
			}
			saving := false
			hook := func() {
				saving = true
				// The temporary file exists while the command saves.
				if entries, _ := os.ReadDir(w.dir); len(entries) != 1 {
					t.Errorf("the temporary file is missing while saving: %v", entries)
				}
				cancel()
			}

			r := w.run(ctx, hook, w.resolve(args)...)

			if !saving {
				t.Fatal("the run never reached the saving")
			}
			w.requireInterrupted(r)
		})
	}
}

// The inner cancellation of a run, which the stop on expired credentials
// uses, leaves the outer context alone: it is reported as its error, not as
// an interruption.
func TestAnInnerCancellationIsNotAnInterruption(t *testing.T) {
	w := newInterruptWorld(t)
	w.resources()
	w.policies(testutil.Static(testutil.QueryError(http.StatusForbidden, "ExpiredToken", "The security token included in the request is expired")))

	r := w.run(context.Background(), nil, w.resolve([]string{"plan", "--region", "us-east-1", "--rules", w.rules, "--out", "OUT/plan.json"})...)

	if r.code != ExitError || strings.Contains(r.stderr, "Interrupted") || !strings.Contains(r.stderr, "Error: the AWS credentials have expired") {
		t.Fatalf("exit %d, stderr:\n%s", r.code, r.stderr)
	}
	if entries, _ := os.ReadDir(w.dir); len(entries) != 0 {
		t.Fatalf("nothing may be saved: %v", entries)
	}
}

// A run that ends normally while nothing was cancelled is not affected by
// the check, and Japanese says the same.
func TestInterruptedInJapanese(t *testing.T) {
	w := newInterruptWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.fake.Handle("tag:GetResources", cancelOn(cancel))

	r := w.run(ctx, nil, "scan", "--region", "us-east-1", "--lang", "ja")

	if r.code != ExitError || !strings.Contains(r.stderr, "中断しました。何も保存していません。\n終了コード: 1(中断)\n") || r.stdout != "" {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", r.code, r.stdout, r.stderr)
	}
}

// resolve puts the world's folder in place of OUT.
func (w *interruptWorld) resolve(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.Replace(a, "OUT", w.dir, 1)
	}
	return out
}
