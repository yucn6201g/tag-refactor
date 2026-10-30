package apply

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The cases of "no difference" (U7 NFR4.2), written after the
// implementation: apply then rollback through the real SDK against the
// HTTP-level fake account, and the tags of every resource compared, aws:
// keys included, with what they were before apply.

// scenario is one account of the HTTP fake with an applier and a
// rollbacker sharing a home folder.
type scenario struct {
	t       *testing.T
	fake    *testutil.FakeAWS
	account *testutil.TagAccount
	session *awsaccess.Session
	j       *journal.Journal
	applier *Applier
	rb      *Rollbacker
	arns    []string
	before  map[string]map[string]string
	plan    *planner.Plan
}

func newScenario(t *testing.T, hideEmpty bool) *scenario {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	account := testutil.NewTagAccount()
	fake.HandleTagAccount(account)
	session, err := awsaccess.NewSession(context.Background(), awsaccess.Options{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }})
	if err != nil {
		t.Fatal(err)
	}
	s := &scenario{t: t, fake: fake, account: account, session: session, before: map[string]map[string]string{},
		plan: &planner.Plan{PlanID: "plan-1", AccountID: testutil.TestAccount, Regions: []string{"us-east-1"},
			InventoryComplete: true, ImpactAnalysisComplete: true}}
	s.journal(nil)
	account.HideEmpty(hideEmpty)
	return s
}

// journal builds the journal, the applier and the rollbacker, with store
// for S3 records.
func (s *scenario) journal(store awsaccess.ObjectStore) {
	s.j = journal.New(journal.Options{Home: s.t.TempDir(), Store: store, ToolVersion: "v-test"})
	aws := struct {
		awsaccess.ARNTagReader
		awsaccess.TagWriter
	}{s.session, awsaccess.NewWriter(s.session)}
	s.applier = New(Options{AWS: aws, Journal: s.j, Sink: &sink{}})
	s.rb = NewRollbacker(Options{AWS: aws, Journal: s.j, Sink: &sink{}})
}

// resource puts a queue with tags in the account and in the plan with its
// changes.
func (s *scenario) resource(name string, tags map[string]string, changes ...planner.Change) string {
	arn := "arn:aws:sqs:us-east-1:" + testutil.TestAccount + ":" + name
	s.account.Add("us-east-1", arn, tags)
	s.arns = append(s.arns, arn)
	s.before[arn] = maps.Clone(tags)
	s.plan.Resources = append(s.plan.Resources, planner.PlannedResource{ARN: arn, Region: "us-east-1", ResourceType: "sqs",
		BeforeTags: maps.Clone(tags), Changes: changes})
	return arn
}

func (s *scenario) apply(loc journal.Location) *Result {
	s.t.Helper()
	res, err := s.applier.Run(context.Background(), Request{Plan: s.plan, AccountID: testutil.TestAccount, Location: loc}, nil)
	if err != nil || !res.Completed || res.Applied == 0 {
		s.t.Fatalf("apply: %v %+v", err, res)
	}
	return res
}

func (s *scenario) rollback(ctx context.Context, runID string, loc journal.Location, stop <-chan struct{}) *RollbackOutcome {
	s.t.Helper()
	out, err := s.rb.Run(ctx, RollbackRequest{RunID: runID, AccountID: testutil.TestAccount, Location: loc}, stop)
	if err != nil {
		s.t.Fatalf("rollback: %v", err)
	}
	return out
}

// requireNoDifference requires every resource to have its tags of before
// apply, aws: keys included.
func (s *scenario) requireNoDifference() {
	s.t.Helper()
	for _, arn := range s.arns {
		if got := s.account.Tags(arn); !maps.Equal(got, s.before[arn]) {
			s.t.Fatalf("%s has %v, want %v", arn, got, s.before[arn])
		}
	}
}

// rawRecord reads the record at path as generic JSON.
func rawRecord(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// requireKept requires the entries, completed and the earlier rollbacks of
// the record to be what they were (NFR6.3), and n rollbacks.
func requireKept(t *testing.T, before, after []byte, n int) {
	t.Helper()
	b, a := rawRecord(t, before), rawRecord(t, after)
	if !reflect.DeepEqual(b["entries"], a["entries"]) || b["completed"] != a["completed"] {
		t.Fatal("the entries or completed of the apply changed")
	}
	rolls, _ := a["rollbacks"].([]any)
	if len(rolls) != n {
		t.Fatalf("%d rollbacks, want %d", len(rolls), n)
	}
	earlier, _ := b["rollbacks"].([]any)
	if len(earlier) > 0 && !reflect.DeepEqual(earlier, rolls[:len(earlier)]) {
		t.Fatal("an earlier rollback changed")
	}
}

// NFR4.2 (a), (e): the four kinds of change are undone with no difference;
// a second rollback writes nothing and finds everything restored; the
// record keeps the entries and the first rollback.
func TestNoDifferenceAfterTheFourKindsOfChange(t *testing.T) {
	s := newScenario(t, false)
	s.resource("rename", map[string]string{"env": "Prod", "aws:cloudformation:stack-name": "s"},
		rename("env", "Environment", false), value("Environment", "Prod", "prod"))
	s.resource("value", map[string]string{"Environment": "Prod", "owner": "o"}, value("Environment", "Prod", "prod"))
	s.resource("add", map[string]string{"team": "a"}, add("CostCenter", "7"))
	s.resource("remove", map[string]string{"obsolete": "x", "team": "b"}, planner.Change{Kind: planner.KindRemoveKey, FromKey: "obsolete"})
	res := s.apply(journal.Location{})
	applied, _ := os.ReadFile(res.Record)

	out := s.rollback(context.Background(), res.RunID, journal.Location{}, nil)

	if out.Restored != 4 || out.Unconfirmed != 0 || out.ExitCode() != 0 {
		t.Fatalf("restored %d unconfirmed %d exit %d", out.Restored, out.Unconfirmed, out.ExitCode())
	}
	s.requireNoDifference()
	first, _ := os.ReadFile(res.Record)
	requireKept(t, applied, first, 1)
	writes := s.fake.Count(testutil.OpTagResources) + s.fake.Count(testutil.OpUntagResources)

	again := s.rollback(context.Background(), res.RunID, journal.Location{}, nil)

	if again.AlreadyRestored != 4 || again.ExitCode() != 0 || s.fake.Count(testutil.OpTagResources)+s.fake.Count(testutil.OpUntagResources) != writes {
		t.Fatalf("second rollback: %+v", again)
	}
	second, _ := os.ReadFile(res.Record)
	requireKept(t, first, second, 2)
	s.requireNoDifference()
}

// stopApply makes the record of res look like an apply stopped half way:
// not completed, and the entries pending.
func stopApply(t *testing.T, res *Result) {
	t.Helper()
	data, _ := os.ReadFile(res.Record)
	var rec map[string]any
	_ = json.Unmarshal(data, &rec)
	rec["completed"] = false
	for _, e := range rec["entries"].([]any) {
		e.(map[string]any)["status"] = "pending"
	}
	data, _ = json.Marshal(rec)
	if err := os.WriteFile(res.Record, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// NFR4.2 (b): an apply that stopped after the tags were set and before the
// keys were removed, after the keys were removed first and before the tags
// were set, and with a result it never knew, is undone with no difference.
func TestNoDifferenceAfterAStoppedApply(t *testing.T) {
	s := newScenario(t, false)
	half := s.resource("half", map[string]string{"env": "Prod"}, rename("env", "Environment", false), value("Environment", "Prod", "prod"))
	first := s.resource("removedfirst", map[string]string{"env": "Prod", "other": "1"}, rename("env", "Environment", false))
	unknown := s.resource("unknown", map[string]string{"env": "dev"}, rename("env", "Environment", false))
	never := s.resource("never", map[string]string{"env": "stg"}, rename("env", "Environment", false))
	res := s.apply(journal.Location{})
	stopApply(t, res)
	s.account.SetTags(half, map[string]string{"env": "Prod", "Environment": "prod"})
	s.account.SetTags(first, map[string]string{"other": "1"})
	_ = unknown // written in full, its entry left pending
	s.account.SetTags(never, map[string]string{"env": "stg"})

	out := s.rollback(context.Background(), res.RunID, journal.Location{}, nil)

	if out.ExitCode() != 0 || out.Restored != 3 || out.AlreadyRestored != 1 || out.Summary.ApplyCompleted {
		t.Fatalf("restored %d already %d summary %+v", out.Restored, out.AlreadyRestored, out.Summary)
	}
	s.requireNoDifference()
}

// NFR4.2 (c): a resource whose last tag apply removed, and an untagged
// resource apply tagged, are undone with no difference whether the tagging
// API returns resources without tags or not; the read back confirms them and
// a second rollback finds them restored.
func TestNoDifferenceWithResourcesWithoutTags(t *testing.T) {
	for _, hide := range []bool{false, true} {
		t.Run(map[bool]string{false: "returned", true: "left out"}[hide], func(t *testing.T) {
			s := newScenario(t, hide)
			s.resource("lasttag", map[string]string{"only": "1"}, planner.Change{Kind: planner.KindRemoveKey, FromKey: "only"})
			s.resource("untagged", map[string]string{}, add("CostCenter", "7"))
			res := s.apply(journal.Location{})

			out := s.rollback(context.Background(), res.RunID, journal.Location{}, nil)

			if out.Restored != 2 || out.Unconfirmed != 0 || out.ExitCode() != 0 {
				t.Fatalf("restored %d unconfirmed %d skipped %d", out.Restored, out.Unconfirmed, len(out.Skipped))
			}
			s.requireNoDifference()
			again := s.rollback(context.Background(), res.RunID, journal.Location{}, nil)
			if again.AlreadyRestored != 2 || again.ExitCode() != 0 {
				t.Fatalf("second rollback: already %d skipped %d", again.AlreadyRestored, len(again.Skipped))
			}
		})
	}
}

// NFR4.2 (d): a rollback that stopped (the second write of a resource kept
// failing; Ctrl+C while reading again; the last save failed; the second
// Ctrl+C) leaves no difference once the same rollback runs again, and the
// record then has both rollbacks with the entries unchanged.
func TestNoDifferenceAfterAStoppedRollback(t *testing.T) {
	cases := map[string]func(s *scenario, ctx context.Context, cancel context.CancelFunc, stop chan struct{}, bucket *testutil.S3Bucket){
		"between the two calls": func(s *scenario, _ context.Context, _ context.CancelFunc, _ chan struct{}, _ *testutil.S3Bucket) {
			s.account.ReplyInstead(func(c testutil.Call) *testutil.Reply {
				if c.Op == testutil.OpUntagResources {
					r := testutil.JSONError(http.StatusInternalServerError, "InternalServiceException", "x")
					return &r
				}
				return nil
			})
		},
		"while reading again": func(s *scenario, _ context.Context, _ context.CancelFunc, stop chan struct{}, _ *testutil.S3Bucket) {
			s.account.LagReads(10, s.arns...)
			var once sync.Once
			old := newTimer
			newTimer = func(time.Duration) (<-chan time.Time, func()) {
				once.Do(func() { close(stop) })
				return make(chan time.Time), func() {}
			}
			s.t.Cleanup(func() { newTimer = old })
		},
		"the last save failed": func(s *scenario, _ context.Context, _ context.CancelFunc, _ chan struct{}, bucket *testutil.S3Bucket) {
			puts := 0
			var mu sync.Mutex
			bucket.ReplyInstead(func(c testutil.Call) *testutil.Reply {
				mu.Lock()
				defer mu.Unlock()
				if c.Op == testutil.OpPutObject && c.Key != "runs/"+testutil.TestAccount+"/.lock" {
					if puts++; puts > 1 {
						r := testutil.S3Error(http.StatusForbidden, "AccessDenied", "x")
						return &r
					}
				}
				return nil
			})
		},
		"the second Ctrl+C": func(s *scenario, _ context.Context, cancel context.CancelFunc, stop chan struct{}, _ *testutil.S3Bucket) {
			var once sync.Once
			s.account.OnWrite(func(testutil.Call) {
				once.Do(func() {
					close(stop)
					cancel()
				})
			})
			s.account.ReplyInstead(func(c testutil.Call) *testutil.Reply {
				if c.Op == testutil.OpGetResources {
					return nil
				}
				return &testutil.Reply{Delay: time.Minute}
			})
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			s := newScenario(t, false)
			bucket := testutil.NewS3Bucket("records", "us-east-1")
			s.fake.HandleS3Buckets(bucket)
			s.journal(awsaccess.NewWriter(s.session))
			loc, _ := journal.ParseLocation("s3://records/runs")
			s.resource("a", map[string]string{"env": "Prod", "aws:x": "1"}, rename("env", "Environment", false), value("Environment", "Prod", "prod"))
			s.resource("b", map[string]string{"env": "dev"}, rename("env", "Environment", false))
			res := s.apply(loc)
			key := "runs/" + testutil.TestAccount + "/" + res.RunID + ".json"
			applied, _ := bucket.Object(key)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stop := make(chan struct{})
			setup(s, ctx, cancel, stop, bucket)

			out := s.rollback(ctx, res.RunID, loc, stop)

			unknown := 0
			for _, tg := range out.Failed {
				if tg.Reason == ReasonResultUnknown {
					unknown++
				}
			}
			stopped := map[string]bool{
				"between the two calls": unknown == 2 && out.Stop == StopNone,
				"while reading again":   unknown == 2 && out.Stop == StopSignal,
				"the last save failed":  out.SaveErr != nil && out.Restored == 2,
				"the second Ctrl+C":     unknown >= 1 && out.Stop == StopSignal,
			}[name]
			if out.ExitCode() != 1 || !stopped {
				t.Fatalf("the stopped rollback: exit %d, stop %q, unknown %d, save %v", out.ExitCode(), out.Stop, unknown, out.SaveErr)
			}
			s.account.ReplyInstead(nil)
			s.account.OnWrite(nil)
			s.account.LagReads(0, s.arns...)
			bucket.ReplyInstead(nil)
			again := s.rollback(context.Background(), res.RunID, loc, nil)
			if again.ExitCode() != 0 || !again.Completed {
				t.Fatalf("the rollback again: exit %d, failed %d", again.ExitCode(), len(again.Failed))
			}
			s.requireNoDifference()
			// The first rollback is in the record, as its last save that
			// worked left it, and the second one after it.
			after, _ := bucket.Object(key)
			requireKept(t, applied, after, 2)
		})
	}
}
