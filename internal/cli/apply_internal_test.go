package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Unit tests of apply inside the package (U6 step 10): the two stages of
// Ctrl+C, the confirmation while it waits, and the types handed to the
// readers (NFR3.10).

type applyRig struct {
	t       *testing.T
	env     *testutil.AWSEnv
	fake    *testutil.FakeAWS
	account *testutil.TagAccount
	plan    string
}

const (
	qa = "arn:aws:sqs:us-east-1:123456789012:qa"
	qb = "arn:aws:sqs:us-east-1:123456789012:qb"
)

func newApplyRig(t *testing.T) *applyRig {
	t.Helper()
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "C")
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	account := testutil.NewTagAccount()
	fake.HandleTagAccount(account)
	account.Add("us-east-1", qa, map[string]string{"env": "a"})
	account.Add("us-east-1", qb, map[string]string{"env": "b"})
	r := &applyRig{t: t, env: env, fake: fake, account: account}
	rules := filepath.Join(t.TempDir(), "r.yaml")
	_ = os.WriteFile(rules, []byte("version: 1\nkeys:\n  Environment:\n    aliases: [env]\n"), 0o600)
	r.plan = filepath.Join(t.TempDir(), "plan.json")
	if res := r.run(context.Background(), Deps{}, "plan", "--rules", rules, "--region", "us-east-1", "--out", r.plan); res.code != 2 {
		t.Fatalf("plan: %d %s", res.code, res.stderr)
	}
	return r
}

func (r *applyRig) run(ctx context.Context, deps Deps, args ...string) runResult {
	r.t.Helper()
	deps.Transport = r.fake
	deps.RetryDelay = func(int) time.Duration { return 0 }
	var stdout, stderr bytes.Buffer
	code := Run(ctx, args, &stdout, &stderr, deps)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func (r *applyRig) record() journal.RunRecord {
	r.t.Helper()
	paths, _ := filepath.Glob(filepath.Join(r.env.Dir, ".tag-refactor", "runs", testutil.TestAccount, "*.json"))
	if len(paths) != 1 {
		r.t.Fatalf("%d records", len(paths))
	}
	j := journal.New(journal.Options{Home: r.env.Dir})
	id := strings.TrimSuffix(filepath.Base(paths[0]), ".json")
	rec, err := j.Load(context.Background(), testutil.TestAccount, id, journal.Location{})
	if err != nil {
		r.t.Fatal(err)
	}
	return *rec
}

// signalPipe stands in for the subscription to Ctrl+C.
func signalPipe() (chan os.Signal, func() (<-chan os.Signal, func())) {
	ch := make(chan os.Signal, 4)
	return ch, func() (<-chan os.Signal, func()) { return ch, func() {} }
}

// Two Ctrl+C while a call runs: the first stops new batches, the second
// cancels the call; the record is saved, open, with the call's resource
// unknown, and the lock is removed.
func TestTwoCtrlCCancelTheRunningCall(t *testing.T) {
	r := newApplyRig(t)
	signals, subscribe := signalPipe()
	var once sync.Once
	r.account.OnWrite(func(testutil.Call) {
		once.Do(func() {
			signals <- os.Interrupt
			signals <- os.Interrupt
		})
	})
	r.account.ReplyInstead(func(c testutil.Call) *testutil.Reply {
		if c.Op == testutil.OpGetResources {
			return nil
		}
		return &testutil.Reply{Delay: time.Minute} // a call that does not end by itself
	})

	res := r.run(context.Background(), Deps{interrupts: subscribe}, "apply", "--plan", r.plan, "--yes")

	if res.code != ExitError || !strings.Contains(res.stderr, "Stopped by Ctrl+C") || !strings.Contains(res.stderr, "tag-refactor rollback --run") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
	rec := r.record()
	reasons := map[string]string{}
	for _, e := range rec.Entries {
		reasons[e.ARN] = string(e.Status) + ":" + e.Reason
	}
	if rec.Completed || strings.Count(strings.Join(mapValues(reasons), ","), "pending") != 2 {
		t.Fatalf("record %v", reasons)
	}
	if _, err := os.Stat(filepath.Join(r.env.Dir, ".tag-refactor", "runs", testutil.TestAccount, ".lock")); !os.IsNotExist(err) {
		t.Fatal("the lock was left")
	}
	if n := r.fake.InFlight(); n != 0 {
		t.Fatalf("%d calls still running", n)
	}
}

func mapValues(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// blockingReader never answers, like a person who does not type.
type blockingReader struct{ done chan struct{} }

func (b blockingReader) Read([]byte) (int, error) {
	<-b.done
	return 0, io.EOF
}

// BR2.2: Ctrl+C while the confirmation waits is not yes: nothing is written
// and the exit code is 0.
func TestCtrlCAtTheConfirmationChangesNothing(t *testing.T) {
	r := newApplyRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	in := blockingReader{done: make(chan struct{})}
	defer close(in.done)
	_, subscribe := signalPipe()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	res := r.run(ctx, Deps{Stdin: in, StdinIsTerminal: func() bool { return true }, interrupts: subscribe}, "apply", "--plan", r.plan)

	if res.code != ExitOK || !strings.Contains(res.stderr, "Nothing was changed.") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
	if r.fake.Count(testutil.OpTagResources) != 0 {
		t.Fatal("something was written")
	}
}

// A line without its line break (the input ended) is not yes either.
func TestYesWithoutALineBreakIsNotYes(t *testing.T) {
	r := newApplyRig(t)
	res := r.run(context.Background(), Deps{Stdin: strings.NewReader("yes"), StdinIsTerminal: func() bool { return true }}, "apply", "--plan", r.plan)
	if res.code != ExitOK || !strings.Contains(res.stderr, "Nothing was changed.") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
}

func TestAHomeFolderIsNeeded(t *testing.T) {
	r := newApplyRig(t)
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("no home") }
	defer func() { userHomeDir = old }()
	res := r.run(context.Background(), Deps{}, "apply", "--plan", r.plan, "--yes")
	if res.code != ExitError || !strings.Contains(res.stderr, "the home folder could not be found") {
		t.Fatalf("exit %d\n%s", res.code, res.stderr)
	}
}

// NFR3.10: the readers of scan, rules suggest and plan cannot write; only
// apply builds a writer, from the session it keeps for itself.
func TestTheReadersOfTheReadCommandsCannotWrite(t *testing.T) {
	for name, v := range map[string]any{"readers": readers{}, "tagReaders": tagReaders{}, "session": &awsaccess.Session{}} {
		if _, ok := v.(awsaccess.TagWriter); ok {
			t.Errorf("%s is a TagWriter", name)
		}
		if _, ok := v.(awsaccess.ObjectStore); ok {
			t.Errorf("%s is an ObjectStore", name)
		}
	}
	if rollbackCommand("r", journal.Location{}, "") != "tag-refactor rollback --run r" || shellArg(`a "b"`) != `"a \"b\""` {
		t.Fatal("rollbackCommand or shellArg")
	}
	ch, unsubscribe := notifyInterrupts()
	unsubscribe()
	if ch == nil {
		t.Fatal("notifyInterrupts")
	}
}
