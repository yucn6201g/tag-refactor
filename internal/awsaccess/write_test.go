package awsaccess

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Unit tests of the write interfaces of U6 (TagWriter, ObjectStore and the
// reads by ARN), written after the implementation, against the HTTP-level
// fake AWS.

const (
	qa = "arn:aws:sqs:us-east-1:123456789012:qa"
	qb = "arn:aws:sqs:us-east-1:123456789012:qb"
)

func newWriterSession(t *testing.T) (*Writer, *testutil.FakeAWS, *testutil.TagAccount) {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	account := testutil.NewTagAccount()
	fake.HandleTagAccount(account)
	s, err := NewSession(context.Background(), Options{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }})
	if err != nil {
		t.Fatal(err)
	}
	return NewWriter(s), fake, account
}

func TestTagResourcesWritesAndReportsFailuresByStatus(t *testing.T) {
	w, fake, account := newWriterSession(t)
	account.Add("us-east-1", qa, map[string]string{"env": "dev"})
	account.Add("us-east-1", qb, map[string]string{"env": "dev"})
	gone := "arn:aws:sqs:us-east-1:123456789012:gone"
	account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		if arn == qb {
			return &testutil.ResourceFailure{StatusCode: http.StatusForbidden, Code: "AccessDeniedException", Message: "secret text"}
		}
		return nil
	})

	res, err := w.TagResources(context.Background(), "us-east-1", []string{qa, qb, gone}, map[string]string{"Environment": "dev"})

	if err != nil {
		t.Fatal(err)
	}
	if account.Tags(qa)["Environment"] != "dev" || len(res.Failed) != 2 {
		t.Fatalf("tags %v, failed %v", account.Tags(qa), res.Failed)
	}
	if e := res.Failed[qb]; e.Kind != errkind.AccessDenied || e.Op != "sqs:TagQueue" || strings.Contains(e.Error(), "secret") {
		t.Fatalf("qb: %+v", e)
	}
	if e := res.Failed[gone]; e.Kind != errkind.NotFound {
		t.Fatalf("gone: %+v", e)
	}
	res, err = w.UntagResources(context.Background(), "us-east-1", []string{qa}, []string{"env"})
	if err != nil || len(res.Failed) != 0 || account.Tags(qa)["env"] != "" {
		t.Fatalf("untag: %v %v %v", res, err, account.Tags(qa))
	}
	if fake.Count(OpTagResources) != 1 || fake.Count(OpUntagResources) != 1 {
		t.Fatalf("calls: %v", fake.Ops())
	}
}

// NFR5.12: only the resources that failed with a transient error are written
// again, and after MaxAttempts calls they stay Transient.
func TestTransientResourcesAreWrittenAgain(t *testing.T) {
	w, fake, account := newWriterSession(t)
	account.Add("us-east-1", qa, nil)
	account.Add("us-east-1", qb, nil)
	failures := 0
	account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		if arn == qb && failures < 2 {
			failures++
			return &testutil.ResourceFailure{StatusCode: http.StatusInternalServerError, Code: "InternalServiceException"}
		}
		return nil
	})

	res, err := w.TagResources(context.Background(), "us-east-1", []string{qa, qb}, map[string]string{"a": "1"})

	if err != nil || len(res.Failed) != 0 || account.Tags(qb)["a"] != "1" {
		t.Fatalf("res %v err %v", res, err)
	}
	calls := fake.Calls()
	if len(calls) != 3 || len(calls[2].JSON["ResourceARNList"].([]any)) != 1 {
		t.Fatalf("the retry must name qb only: %d calls", len(calls))
	}

	account.FailWrites(func(op, arn string) *testutil.ResourceFailure {
		return &testutil.ResourceFailure{StatusCode: http.StatusTooManyRequests, Code: "ThrottlingException"}
	})
	res, err = w.TagResources(context.Background(), "us-east-1", []string{qa}, map[string]string{"a": "2"})
	if err != nil || res.Failed[qa].Kind != errkind.Transient || fake.Count(OpTagResources) != 3+MaxAttempts {
		t.Fatalf("res %v err %v calls %d", res, err, fake.Count(OpTagResources))
	}
}

// NFR3.12: keys starting with aws:, in any case, never reach AWS.
func TestAWSKeysAreRefusedWithoutCallingAWS(t *testing.T) {
	w, fake, _ := newWriterSession(t)
	for _, key := range []string{"aws:x", "AWS:x", "Aws:x"} {
		if _, err := w.TagResources(context.Background(), "us-east-1", []string{qa}, map[string]string{key: "v"}); errkind.KindOf(err) != errkind.InvalidInput {
			t.Errorf("tag %s: %v", key, err)
		}
		if _, err := w.UntagResources(context.Background(), "us-east-1", []string{qa}, []string{"a", key}); errkind.KindOf(err) != errkind.InvalidInput {
			t.Errorf("untag %s: %v", key, err)
		}
	}
	if _, err := w.TagResources(context.Background(), "us-east-1", make([]string, 21), map[string]string{"a": "b"}); errkind.KindOf(err) != errkind.InvalidInput {
		t.Errorf("21 resources: %v", err)
	}
	if n := len(fake.Calls()); n != 0 {
		t.Fatalf("%d calls reached AWS", n)
	}
}

func TestWholeCallErrorsAndGetTags(t *testing.T) {
	w, _, account := newWriterSession(t)
	account.Add("us-east-1", qa, map[string]string{"env": "dev"})
	account.AddNeverTagged("us-east-1", qb)

	tags, err := w.s.GetTags(context.Background(), "us-east-1", []string{qa, qb})
	if err != nil || len(tags) != 1 || tags[qa]["env"] != "dev" {
		t.Fatalf("tags %v err %v", tags, err)
	}
	if _, err := w.s.GetTags(context.Background(), "us-east-1", make([]string, 101)); err == nil {
		t.Fatal("101 ARNs must be refused")
	}
	account.ReplyInstead(func(testutil.Call) *testutil.Reply {
		r := testutil.JSONError(http.StatusBadRequest, "ExpiredTokenException", "expired")
		return &r
	})
	if _, err := w.TagResources(context.Background(), "us-east-1", []string{qa}, map[string]string{"a": "b"}); errkind.KindOf(err) != errkind.CredentialsMissing {
		t.Fatalf("expired token: %v", err)
	}
}

func TestClassifyFailure(t *testing.T) {
	for _, c := range []struct {
		status int32
		code   string
		kind   errkind.Kind
	}{
		{429, "", errkind.Transient}, {503, "", errkind.Transient}, {403, "", errkind.AccessDenied},
		{404, "", errkind.NotFound}, {400, "ThrottlingException", errkind.InvalidInput},
		{0, "InternalServiceException", errkind.Transient}, {0, "AccessDenied", errkind.AccessDenied},
		{0, "NoSuchEntity", errkind.NotFound}, {0, "Throttling", errkind.Transient}, {0, "Odd", errkind.Internal},
		{418, "", errkind.Internal},
	} {
		e := classifyFailure(OpUntagResources, "us-east-1", "arn:aws:iam::123456789012:role/r",
			rgttypes.FailureInfo{StatusCode: c.status, ErrorCode: rgttypes.ErrorCode(c.code)})
		if e.Kind != c.kind {
			t.Errorf("%d %s: %v, want %v", c.status, c.code, e.Kind, c.kind)
		}
		if e.Kind == errkind.AccessDenied && e.Op != "iam:UntagRole" {
			t.Errorf("AccessDenied names %s", e.Op)
		}
	}
	if tagActionFor(OpTagResources, "arn:aws:s3:::b") != "s3:PutBucketTagging" || tagActionFor(OpTagResources, "arn:aws:foo:x") != OpTagResources {
		t.Fatal("tagActionFor")
	}
}

func TestObjectStoreConditionalWritesAndErrors(t *testing.T) {
	w, fake, _ := newWriterSession(t)
	bucket := testutil.NewS3Bucket("records", "eu-west-1")
	fake.HandleS3Buckets(bucket)
	ctx := context.Background()
	uri := "s3://records/runs/123456789012/.lock"

	if err := w.PutIfAbsent(ctx, uri, []byte(`{"runId":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.PutIfAbsent(ctx, uri, []byte("b")); errkind.KindOf(err) != errkind.Conflict {
		t.Fatalf("second lock: %v", err)
	}
	if data, err := w.Get(ctx, uri); err != nil || string(data) != `{"runId":"a"}` {
		t.Fatalf("get: %q %v", data, err)
	}
	bucket.ConditionalStatus(http.StatusConflict)
	if err := w.PutIfAbsent(ctx, "s3://records/x", nil); errkind.KindOf(err) != errkind.Conflict {
		t.Fatalf("409: %v", err)
	}
	bucket.ConditionalStatus(http.StatusNotImplemented)
	if err := w.PutIfAbsent(ctx, "s3://records/x", nil); errkind.KindOf(err) != errkind.StorageFailed {
		t.Fatalf("501: %v", err)
	}
	if err := w.Put(ctx, "s3://records/r.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := w.Delete(ctx, uri); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Get(ctx, uri); errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("deleted: %v", err)
	}
	if _, err := w.Get(ctx, "not-s3"); errkind.KindOf(err) != errkind.InvalidInput {
		t.Fatalf("bad URI: %v", err)
	}
	if _, err := w.Get(ctx, "s3://other/k"); errkind.KindOf(err) != errkind.NotFound {
		t.Fatalf("other bucket: %v", err)
	}
	bucket.ReplyInstead(func(testutil.Call) *testutil.Reply {
		r := testutil.S3Error(http.StatusForbidden, "AccessDenied", "denied")
		return &r
	})
	if err := w.Put(ctx, "s3://records/r.json", nil); errkind.KindOf(err) != errkind.AccessDenied {
		t.Fatalf("denied: %v", err)
	}
	// NFR8.25: no request sets an ACL; the location was read once.
	for _, put := range bucket.Puts() {
		if testutil.HasACL(put) {
			t.Errorf("PutObject %s sets an ACL", put.Key)
		}
	}
	if n := fake.Count(testutil.OpGetBucketLocation); n != 2 {
		t.Fatalf("GetBucketLocation called %d times, want 2 (records and other)", n)
	}
}

// NFR3.10: the readers of scan, rules suggest and plan are the session,
// which cannot write; only the writer built from it can.
func TestOnlyTheWriterWrites(t *testing.T) {
	var s any = &Session{}
	if _, ok := s.(TagWriter); ok {
		t.Fatal("the session must not be a TagWriter")
	}
	if _, ok := s.(ObjectStore); ok {
		t.Fatal("the session must not be an ObjectStore")
	}
	if b, k, ok := ParseS3URI("s3://b/p/k"); !ok || b != "b" || k != "p/k" {
		t.Fatal("ParseS3URI")
	}
	if !IsAWSKey("AwS:x") || IsAWSKey("xaws:") || IAMAction(OpListBucket) != OpListBucket {
		t.Fatal("IsAWSKey or IAMAction")
	}
}
