package testutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingTB captures Errorf calls so tests can check that the fake AWS
// reports unexpected calls.
type recordingTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func signedHeader(service, region string) string {
	return "AWS4-HMAC-SHA256 Credential=" + FakeAccessKeyID + "/20261020/" + region + "/" + service + "/aws4_request, SignedHeaders=host, Signature=abc"
}

func jsonRequest(t *testing.T, target, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://tagging.us-east-1.amazonaws.com/", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", signedHeader("tagging", "us-east-1"))
	req.Header.Set("X-Amz-Target", target)
	return req
}

func queryRequest(t *testing.T, service string, form url.Values) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://"+service+".amazonaws.com/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", signedHeader(service, "us-east-1"))
	return req
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFakeAWSRoutesJSONAndQueryCalls(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	fake.Handle("tag:GetResources", TagPages([]TaggedResource{{ARN: "arn:aws:sqs:us-east-1:123456789012:q", Tags: map[string]string{"env": "dev"}}}))

	resp, err := fake.RoundTrip(jsonRequest(t, "ResourceGroupsTaggingAPI_20170126.GetResources", `{"ResourcesPerPage":100}`))
	if err != nil {
		t.Fatal(err)
	}
	if body := readBody(t, resp); !strings.Contains(body, `"Key":"env"`) || resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected tag page: %d %s", resp.StatusCode, body)
	}
	resp, err = fake.RoundTrip(queryRequest(t, "sts", url.Values{"Action": {"GetCallerIdentity"}}))
	if err != nil {
		t.Fatal(err)
	}
	if body := readBody(t, resp); !strings.Contains(body, "<Account>"+TestAccount+"</Account>") {
		t.Fatalf("unexpected identity: %s", body)
	}

	calls := fake.Calls()
	if len(calls) != 2 || calls[0].Op != "tag:GetResources" || calls[1].Op != "sts:GetCallerIdentity" {
		t.Fatalf("unexpected calls: %+v", calls)
	}
	if calls[0].JSON["ResourcesPerPage"] != float64(100) || calls[0].Region != "us-east-1" || calls[0].AccessKeyID != FakeAccessKeyID {
		t.Fatalf("call details not parsed: %+v", calls[0])
	}
	if got := fake.Ops(); len(got) != 2 || got[0] != "sts:GetCallerIdentity" {
		t.Fatalf("Ops() = %v", got)
	}
	if fake.Count("tag:GetResources") != 1 {
		t.Fatalf("Count() = %d", fake.Count("tag:GetResources"))
	}
}

func TestFakeAWSFailsTheTestOnWritesAndUnknownCalls(t *testing.T) {
	rec := &recordingTB{TB: t}
	fake := NewFakeAWS(rec)
	fake.Handle("tag:TagResources", Static(Reply{}))

	resp, err := fake.RoundTrip(jsonRequest(t, "ResourceGroupsTaggingAPI_20170126.TagResources", `{}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("write was not rejected: %v %v", resp, err)
	}
	resp, err = fake.RoundTrip(queryRequest(t, "iam", url.Values{"Action": {"ListRoles"}}))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown call was not rejected: %v %v", resp, err)
	}
	unsigned, _ := http.NewRequest(http.MethodPost, "https://iam.amazonaws.com/", strings.NewReader("Action=ListPolicies"))
	if resp, _ = fake.RoundTrip(unsigned); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsigned call was not rejected")
	}
	if len(rec.errs) != 3 {
		t.Fatalf("expected 3 reported errors, got %v", rec.errs)
	}
}

func TestIsReadOperation(t *testing.T) {
	for op, want := range map[string]bool{
		"tag:GetResources": true, "iam:ListPolicies": true, "ec2:DescribeRegions": true,
		"tag:TagResources": false, "tag:UntagResources": false, "iam:CreatePolicy": false,
	} {
		if got := IsReadOperation(op); got != want {
			t.Errorf("IsReadOperation(%q) = %v, want %v", op, got, want)
		}
	}
}

func TestTagPagesFollowsTokensIncludingEmptyPages(t *testing.T) {
	h := TagPages(nil, []TaggedResource{{ARN: "arn:a"}}, nil)
	first := h(Call{JSON: map[string]any{}})
	if !strings.Contains(first.Body, `"PaginationToken":"page-1"`) {
		t.Fatalf("first page: %s", first.Body)
	}
	second := h(Call{JSON: map[string]any{"PaginationToken": "page-1"}})
	if !strings.Contains(second.Body, `"arn:a"`) || !strings.Contains(second.Body, `"PaginationToken":"page-2"`) {
		t.Fatalf("second page: %s", second.Body)
	}
	last := h(Call{JSON: map[string]any{"PaginationToken": "page-2"}})
	if !strings.Contains(last.Body, `"PaginationToken":""`) {
		t.Fatalf("last page: %s", last.Body)
	}
	if bad := h(Call{JSON: map[string]any{"PaginationToken": "nope"}}); bad.Status != http.StatusBadRequest {
		t.Fatalf("bad token accepted: %+v", bad)
	}
}

func TestPolicyDocumentsAreEncodedAsIAMDoes(t *testing.T) {
	if got := EncodePolicyDocument(`{"Statement":[{"Sid":"A B+C"}]}`); got != "%7B%22Statement%22%3A%5B%7B%22Sid%22%3A%22A%20B%2BC%22%7D%5D%7D" {
		t.Fatalf("document not encoded as IAM does: %s", got)
	}
	// A page with a marker that is not a page of the list is rejected.
	fake := NewFakeAWS(t)
	fake.HandlePolicyAccount(&PolicyAccount{Users: []Principal{{Name: "u"}}})
	resp, err := fake.RoundTrip(queryRequest(t, "iam", url.Values{"Action": {"GetAccountAuthorizationDetails"}, "Marker": {"x"}}))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad marker accepted: %v %v", resp, err)
	}
}

func TestErrorRepliesAndSequence(t *testing.T) {
	if r := QueryError(403, "AccessDenied", "no <access>"); r.Status != 403 || !strings.Contains(r.Body, "<Code>AccessDenied</Code>") || !strings.Contains(r.Body, "no &lt;access&gt;") {
		t.Fatalf("query error: %+v", r)
	}
	if r := EC2Error(403, "UnauthorizedOperation", "m"); !strings.Contains(r.Body, "<Errors><Error><Code>UnauthorizedOperation</Code>") {
		t.Fatalf("ec2 error: %+v", r)
	}
	if r := JSONError(400, "AccessDeniedException", "m"); r.ContentType != jsonContentType || !strings.Contains(r.Body, `"__type":"AccessDeniedException"`) {
		t.Fatalf("json error: %+v", r)
	}
	seq := Sequence(Reply{Status: 500}, Reply{Status: 200})
	if seq(Call{N: 1}).Status != 500 || seq(Call{N: 2}).Status != 200 || seq(Call{N: 5}).Status != 200 {
		t.Fatal("sequence did not follow call numbers")
	}
}

func TestFakeAWSDelayHonoursRequestContextAndTransportErrors(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.Handle("sts:GetCallerIdentity", Static(Reply{Delay: time.Hour}))
	fake.Handle("iam:ListPolicies", Static(Reply{Err: errors.New("connection reset")}))
	fake.Handle("ec2:DescribeRegions", Static(Reply{Delay: time.Millisecond, Body: "ok"}))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req := queryRequest(t, "sts", url.Values{"Action": {"GetCallerIdentity"}}).WithContext(ctx)
	if _, err := fake.RoundTrip(req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("delay did not stop at the deadline: %v", err)
	}
	if _, err := fake.RoundTrip(queryRequest(t, "iam", url.Values{"Action": {"ListPolicies"}})); err == nil {
		t.Fatal("transport error not returned")
	}
	resp, err := fake.RoundTrip(queryRequest(t, "ec2", url.Values{"Action": {"DescribeRegions"}}))
	if err != nil || readBody(t, resp) != "ok" {
		t.Fatalf("short delay: %v", err)
	}
}

func TestIsolateAWSEnvRemovesRealSettings(t *testing.T) {
	t.Setenv("AWS_PROFILE", "real-profile")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAREALKEY")
	env := IsolateAWSEnv(t)
	if _, ok := os.LookupEnv("AWS_PROFILE"); ok {
		t.Fatal("AWS_PROFILE still set")
	}
	if _, ok := os.LookupEnv("AWS_ACCESS_KEY_ID"); ok {
		t.Fatal("AWS_ACCESS_KEY_ID still set")
	}
	if os.Getenv("AWS_EC2_METADATA_DISABLED") != "true" || os.Getenv("AWS_CONFIG_FILE") != env.ConfigFile {
		t.Fatal("isolation variables not set")
	}
	env.UseFakeCredentials("us-east-1")
	env.SetProfile("dev", Profile{Region: "eu-west-1", AccessKeyID: "AKIADEV", SecretAccessKey: "s"})
	cfg, _ := os.ReadFile(env.ConfigFile)
	creds, _ := os.ReadFile(env.CredentialsFile)
	if !strings.Contains(string(cfg), "[profile dev]\nregion = eu-west-1") || !strings.Contains(string(cfg), "[default]\nregion = us-east-1") {
		t.Fatalf("config: %s", cfg)
	}
	if !strings.Contains(string(creds), "aws_session_token = "+FakeSessionToken) || !strings.Contains(string(creds), "[dev]\naws_access_key_id = AKIADEV") {
		t.Fatalf("credentials: %s", creds)
	}
	env.WriteSSOToken("https://example.awsapps.com/start", time.Now().Add(-time.Hour))
	entries, err := os.ReadDir(env.Dir + "/.aws/sso/cache")
	if err != nil || len(entries) != 1 {
		t.Fatalf("SSO cache not written: %v", err)
	}
}

func TestFakeClockAdvancesByStep(t *testing.T) {
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(start, time.Second)
	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("first Now() = %v", got)
	}
	c.Advance(time.Minute)
	if got := c.Now(); !got.Equal(start.Add(time.Second + time.Minute)) {
		t.Fatalf("second Now() = %v", got)
	}
}
