package awsaccess

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go/logging"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

type recordingSink struct {
	mu   sync.Mutex
	recs []CallRecord
}

func (s *recordingSink) APICall(rec CallRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, rec)
}

func (s *recordingSink) last(t *testing.T) CallRecord {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recs) == 0 {
		t.Fatal("no call was recorded")
	}
	return s.recs[len(s.recs)-1]
}

// newSession isolates the AWS environment with fake credentials in us-east-1
// and builds a session whose HTTP requests go to fake.
func newSession(t *testing.T, fake *testutil.FakeAWS, mutate func(o *Options)) (*Session, *recordingSink) {
	t.Helper()
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	sink := &recordingSink{}
	clock := testutil.NewFakeClock(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), 5*time.Millisecond)
	opts := Options{Transport: fake, Debug: sink, Now: clock.Now, RetryDelay: func(int) time.Duration { return 0 }}
	if mutate != nil {
		mutate(&opts)
	}
	s, err := NewSession(context.Background(), opts)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s, sink
}

func requireKind(t *testing.T, err error, kind errkind.Kind, reason errkind.Reason) {
	t.Helper()
	_ = checkKind(t, err, kind, reason)
}

// checkKind is requireKind returning the error for further checks.
func checkKind(t *testing.T, err error, kind errkind.Kind, reason errkind.Reason) *errkind.Error {
	t.Helper()
	e, ok := errkind.As(err)
	if !ok {
		t.Fatalf("error has no kind: %v", err)
	}
	if e.Kind != kind || e.Reason != reason {
		t.Fatalf("got %s/%q, want %s/%q", e.Kind, e.Reason, kind, reason)
	}
	return e
}

func TestSessionUsesStandardRetriesAndNoSDKLogging(t *testing.T) {
	s, _ := newSession(t, testutil.NewFakeAWS(t), nil)
	if s.cfg.ClientLogMode != 0 {
		t.Fatalf("ClientLogMode = %v, want 0 (NFR8.8)", s.cfg.ClientLogMode)
	}
	if _, ok := s.cfg.Logger.(logging.Nop); !ok {
		t.Fatalf("Logger = %T, want logging.Nop", s.cfg.Logger)
	}
	if _, ok := s.cfg.Retryer().(*retry.Standard); !ok {
		t.Fatalf("Retryer = %T, want the standard retryer", s.cfg.Retryer())
	}
	if s.Region() != "us-east-1" {
		t.Fatalf("Region() = %q", s.Region())
	}
	want := []string{OpDescribeRegions, OpGetResources, OpListPolicies, OpGetPolicyVersion,
		OpDescribeConfigurationRecorders, OpDescribeConfigurationRecorderStatus, OpSelectResourceConfig,
		OpListIndexes, OpGetDefaultView, OpGetView, "resource-explorer-2:Search",
		// U5: the reads of the impact analysis.
		OpListUsers, OpListRoles, OpListGroups, OpListUserPolicies, OpListRolePolicies, OpListGroupPolicies,
		OpGetUserPolicy, OpGetRolePolicy, OpGetGroupPolicy, "s3:ListAllMyBuckets", OpGetBucketLocation, OpGetBucketPolicy,
		OpListKeys, OpDescribeKey, OpGetKeyPolicy, OpListQueues, OpGetQueueAttributes, OpListTopics, OpGetTopicAttributes,
		OpListSecrets, OpGetResourcePolicy}
	if got := RequiredPermissions(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("RequiredPermissions() = %v", got)
	}
}

func TestListTaggedResourcesReadsEveryPageIncludingEmptyOnes(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("tag:GetResources", testutil.TagPages(
		[]testutil.TaggedResource{{ARN: "arn:aws:sqs:us-east-1:123456789012:a", Tags: map[string]string{"env": "dev"}}},
		nil, // an empty page that still carries a token
		[]testutil.TaggedResource{{ARN: "arn:aws:sqs:us-east-1:123456789012:b", Tags: map[string]string{}}},
	))
	s, _ := newSession(t, fake, nil)

	got, err := s.ListTaggedResources(context.Background(), "us-east-1")

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Tags["env"] != "dev" || got[1].ARN != "arn:aws:sqs:us-east-1:123456789012:b" {
		t.Fatalf("resources = %+v", got)
	}
	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("%d calls, want 3", len(calls))
	}
	for _, c := range calls {
		if c.JSON["ResourcesPerPage"] != float64(100) {
			t.Errorf("ResourcesPerPage = %v, want 100 (the API maximum)", c.JSON["ResourcesPerPage"])
		}
	}
}

func TestListTaggedResourcesStopsOnARepeatedToken(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("tag:GetResources", testutil.Static(testutil.TagPage("same-token")))
	s, _ := newSession(t, fake, nil)

	_, err := s.ListTaggedResources(context.Background(), "us-east-1")

	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	if fake.Count("tag:GetResources") != 2 {
		t.Fatalf("%d calls, want 2", fake.Count("tag:GetResources"))
	}
}

// U5 BR1.1: every customer managed policy is listed with its attachment and
// permissions boundary counts; the attached-only filter would leave out the
// policies used only as a boundary.
func TestPolicyReaderListsCustomerPoliciesWithTheirCountsAndReadsDefaultVersions(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("iam:ListPolicies", testutil.PolicyPages(
		[]testutil.ManagedPolicy{{ARN: "arn:aws:iam::123456789012:policy/A"}},
		[]testutil.ManagedPolicy{{ARN: "arn:aws:iam::123456789012:policy/B", DefaultVersion: "v4", NotAttached: true, Boundaries: 3}},
	))
	doc := `{"Statement":[{"Sid":"S 1","Effect":"Allow"}]}`
	fake.Handle("iam:GetPolicyVersion", testutil.PolicyDocuments(map[string]string{"arn:aws:iam::123456789012:policy/B": doc}))
	s, _ := newSession(t, fake, nil)

	refs, err := s.ListCustomerPolicies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[1].DefaultVersionID != "v4" || refs[0].AttachmentCount != 1 || refs[0].BoundaryCount != 0 ||
		refs[1].AttachmentCount != 0 || refs[1].BoundaryCount != 3 {
		t.Fatalf("refs = %+v", refs)
	}
	for _, c := range fake.Calls() {
		if c.Form.Get("Scope") != "Local" || c.Form.Has("OnlyAttached") || c.Form.Get("MaxItems") != "1000" {
			t.Fatalf("ListPolicies parameters = %v", c.Form)
		}
	}
	got, err := s.GetPolicyDocument(context.Background(), refs[1])
	if err != nil {
		t.Fatal(err)
	}
	if got != testutil.EncodePolicyDocument(doc) {
		t.Fatalf("document = %q", got)
	}
	last := fake.Calls()[len(fake.Calls())-1]
	if last.Form.Get("VersionId") != "v4" {
		t.Fatalf("GetPolicyVersion asked for %q", last.Form.Get("VersionId"))
	}
}

func TestPolicyReaderRejectsMalformedIAMAnswers(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("iam:ListPolicies", testutil.Static(testutil.Reply{Body: `<ListPoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
		`<ListPoliciesResult><IsTruncated>true</IsTruncated><Policies></Policies></ListPoliciesResult></ListPoliciesResponse>`}))
	fake.Handle("iam:GetPolicyVersion", testutil.Static(testutil.Reply{Body: `<GetPolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
		`<GetPolicyVersionResult></GetPolicyVersionResult></GetPolicyVersionResponse>`}))
	s, _ := newSession(t, fake, nil)

	_, err := s.ListCustomerPolicies(context.Background())
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.GetPolicyDocument(context.Background(), IAMPolicyRef{ARN: "arn:aws:iam::123456789012:policy/A", DefaultVersionID: "v1"})
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
}

func TestTransientFailuresAreRetriedWithTheStandardRetryer(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	failure := testutil.JSONError(http.StatusInternalServerError, "InternalFailure", "try again")
	fake.Handle("tag:GetResources", testutil.Sequence(failure, failure, testutil.TagPage("")))
	s, sink := newSession(t, fake, nil)

	_, err := s.ListTaggedResources(context.Background(), "us-east-1")

	if err != nil {
		t.Fatalf("the call should succeed on the third attempt: %v", err)
	}
	rec := sink.last(t)
	if rec.Op != OpGetResources || rec.Region != "us-east-1" || rec.Retries != 2 || rec.ErrorKind != "" {
		t.Fatalf("debug record = %+v", rec)
	}
}

func TestEachAttemptHasATimeLimitAndExhaustedAttemptsAreTransient(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("tag:GetResources", testutil.Static(testutil.Reply{Delay: time.Hour}))
	s, sink := newSession(t, fake, func(o *Options) { o.AttemptTimeout = 20 * time.Millisecond })

	start := time.Now()
	_, err := s.ListTaggedResources(context.Background(), "us-east-1")

	requireKind(t, err, errkind.Transient, errkind.ReasonRetriesExhausted)
	// NFR5.4: ten attempts since U2 (U1 used the SDK default of three).
	if fake.Count("tag:GetResources") != MaxAttempts {
		t.Fatalf("%d attempts, want %d", fake.Count("tag:GetResources"), MaxAttempts)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("the attempt limit did not stop the calls: %v", time.Since(start))
	}
	if rec := sink.last(t); rec.ErrorKind != "Transient" || rec.Retries != MaxAttempts-1 {
		t.Fatalf("debug record = %+v", rec)
	}
}

func TestEachCallHasATimeLimitIncludingRetries(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("tag:GetResources", testutil.Static(testutil.Reply{Delay: time.Hour}))
	s, _ := newSession(t, fake, func(o *Options) { o.CallTimeout = 50 * time.Millisecond })

	_, err := s.ListTaggedResources(context.Background(), "us-east-1")

	requireKind(t, err, errkind.Transient, errkind.ReasonTimeout)
}

func TestSDKErrorsBecomeErrorKinds(t *testing.T) {
	cases := []struct {
		name   string
		reply  testutil.Reply
		kind   errkind.Kind
		reason errkind.Reason
	}{
		{"access denied", testutil.JSONError(400, "AccessDeniedException", "no"), errkind.AccessDenied, errkind.ReasonNone},
		{"expired token", testutil.JSONError(400, "ExpiredTokenException", "old"), errkind.CredentialsMissing, errkind.ReasonCredentialsExpired},
		{"rejected key", testutil.JSONError(400, "UnrecognizedClientException", "bad"), errkind.CredentialsMissing, errkind.ReasonCredentialsRejected},
		{"throttled", testutil.JSONError(400, "ThrottlingException", "slow"), errkind.Transient, errkind.ReasonRetriesExhausted},
		{"server error page", testutil.Reply{Status: 502, Body: "<html>bad gateway</html>", ContentType: "text/html"}, errkind.Transient, errkind.ReasonRetriesExhausted},
		{"disabled region", testutil.JSONError(400, "RegionDisabledException", "off"), errkind.InvalidInput, errkind.ReasonRegionUnavailable},
		{"unknown endpoint", testutil.Reply{Err: &net.DNSError{Name: "tagging.xx-none-1.amazonaws.com", Err: "no such host", IsNotFound: true}}, errkind.InvalidInput, errkind.ReasonRegionUnavailable},
		{"unknown code", testutil.JSONError(400, "WeirdException", "?"), errkind.Internal, errkind.ReasonNone},
		{"unreadable answer", testutil.Reply{Body: "{not json", ContentType: "application/x-amz-json-1.1"}, errkind.Internal, errkind.ReasonNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := testutil.NewFakeAWS(t)
			fake.Handle("tag:GetResources", testutil.Static(tc.reply))
			s, sink := newSession(t, fake, nil)

			_, err := s.ListTaggedResources(context.Background(), "us-east-1")

			e := checkKind(t, err, tc.kind, tc.reason)
			if e.Op != OpGetResources || e.Region != "us-east-1" {
				t.Fatalf("op/region = %q/%q", e.Op, e.Region)
			}
			rec := sink.last(t)
			if rec.ErrorKind != tc.kind.String() {
				t.Fatalf("debug error kind = %q", rec.ErrorKind)
			}
			if (tc.kind == errkind.Internal) != (rec.CauseType != "") {
				t.Fatalf("cause type %q for kind %s", rec.CauseType, tc.kind)
			}
		})
	}
}

func TestResolveIdentifiesTheAccountInAnEnabledRegion(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("eu-west-1", "us-east-1")
	s, _ := newSession(t, fake, nil)

	acc, err := s.Resolve(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if acc.AccountID != testutil.TestAccount || acc.CallerARN != testutil.TestCallerARN || acc.Region != "us-east-1" || acc.Profile != "default" {
		t.Fatalf("account = %+v", acc)
	}
}

func TestResolveRejectsARegionThatIsNotEnabled(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("eu-west-1")
	s, _ := newSession(t, fake, nil)

	_, err := s.Resolve(context.Background())

	requireKind(t, err, errkind.InvalidInput, errkind.ReasonRegionUnavailable)
}

// STS in an opt-in region the account has not enabled rejects the
// credentials (InvalidClientTokenId). The enabled regions are then read from
// us-east-1, so the region is reported as unavailable, not the credentials
// as bad (BR1.1, review R-02). Real bad credentials are still reported as such.
func TestResolveRechecksARejectionByStsAgainstTheEnabledRegions(t *testing.T) {
	rejected := testutil.QueryError(http.StatusForbidden, "InvalidClientTokenId", "The security token included in the request is invalid.")
	cases := []struct {
		name        string
		region      string
		describe    testutil.Reply
		kind        errkind.Kind
		reason      errkind.Reason
		wantRecheck bool
	}{
		{"disabled opt-in region", "me-south-1", testutil.Regions("us-east-1", "eu-west-1"), errkind.InvalidInput, errkind.ReasonRegionUnavailable, true},
		{"enabled region, bad credentials", "eu-west-1", testutil.Regions("us-east-1", "eu-west-1"), errkind.CredentialsMissing, errkind.ReasonCredentialsRejected, true},
		{"re-check also rejected", "me-south-1", testutil.EC2Error(http.StatusUnauthorized, "AuthFailure", "no"), errkind.CredentialsMissing, errkind.ReasonCredentialsRejected, true},
		{"us-east-1 itself", "us-east-1", testutil.Regions("us-east-1"), errkind.CredentialsMissing, errkind.ReasonCredentialsRejected, false},
		{"GovCloud has no opt-in regions", "us-gov-west-1", testutil.Regions("us-east-1"), errkind.CredentialsMissing, errkind.ReasonCredentialsRejected, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := testutil.NewFakeAWS(t)
			fake.Handle("sts:GetCallerIdentity", testutil.Static(rejected))
			fake.Handle("ec2:DescribeRegions", func(c testutil.Call) testutil.Reply {
				if c.Region != "us-east-1" {
					t.Errorf("DescribeRegions was asked in %s, want us-east-1", c.Region)
				}
				return tc.describe
			})
			s, _ := newSession(t, fake, func(o *Options) { o.Region = tc.region })

			_, err := s.Resolve(context.Background())

			e := checkKind(t, err, tc.kind, tc.reason)
			if tc.kind == errkind.InvalidInput && e.Region != tc.region {
				t.Fatalf("region = %q, want %q", e.Region, tc.region)
			}
			if tc.kind == errkind.CredentialsMissing && e.Op != OpGetCallerIdentity {
				t.Fatalf("op = %q, want the original STS error", e.Op)
			}
			if got := fake.Count("ec2:DescribeRegions") == 1; got != tc.wantRecheck {
				t.Fatalf("re-check made = %v, want %v (calls: %v)", got, tc.wantRecheck, fake.Ops())
			}
		})
	}
}

func TestOtherStsErrorsAreNotRechecked(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("sts:GetCallerIdentity", testutil.Static(testutil.QueryError(http.StatusForbidden, "ExpiredToken", "expired")))
	s, _ := newSession(t, fake, func(o *Options) { o.Region = "me-south-1" })

	_, err := s.Resolve(context.Background())

	requireKind(t, err, errkind.CredentialsMissing, errkind.ReasonCredentialsExpired)
	if fake.Count("ec2:DescribeRegions") != 0 {
		t.Fatalf("an expired token must not be re-checked: %v", fake.Ops())
	}
}

func TestResolveReportsAnExpiredSSOSessionWithoutCallingAWS(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	env := testutil.IsolateAWSEnv(t)
	env.WriteConfig("[profile dev]\nsso_start_url = https://example.awsapps.com/start\nsso_region = us-east-1\n" +
		"sso_account_id = 123456789012\nsso_role_name = ReadOnly\nregion = us-east-1\n")
	env.WriteSSOToken("https://example.awsapps.com/start", time.Now().Add(-time.Hour))
	s, err := NewSession(context.Background(), Options{Profile: "dev", Transport: fake})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Resolve(context.Background())

	e := checkKind(t, err, errkind.CredentialsMissing, errkind.ReasonSSOSessionExpired)
	if e.Subject != "dev" {
		t.Fatalf("subject = %q", e.Subject)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("AWS was called: %v", fake.Ops())
	}
}

func TestNewSessionValidatesProfileAndRegion(t *testing.T) {
	cases := []struct {
		name    string
		region  string // region of the default profile
		opts    Options
		reason  errkind.Reason
		subject string
	}{
		{"unknown profile", "us-east-1", Options{Profile: "nope"}, errkind.ReasonProfileNotFound, "nope"},
		{"no region anywhere", "", Options{}, errkind.ReasonRegionNotSet, "default"},
		{"malformed region", "us-east-1", Options{Region: "us-east-1.evil.example/"}, errkind.ReasonRegionInvalid, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.IsolateAWSEnv(t)
			env.UseFakeCredentials(tc.region)

			_, err := NewSession(context.Background(), tc.opts)

			e := checkKind(t, err, errkind.InvalidInput, tc.reason)
			if e.Subject != tc.subject {
				t.Fatalf("subject = %q, want %q", e.Subject, tc.subject)
			}
		})
	}
}

func TestCheckCredentialsWithoutAnyCredentials(t *testing.T) {
	env := testutil.IsolateAWSEnv(t)
	env.WriteConfig("[default]\nregion = us-east-1\n")
	fake := testutil.NewFakeAWS(t)
	s, err := NewSession(context.Background(), Options{Transport: fake})
	if err != nil {
		t.Fatal(err)
	}

	err = s.checkCredentials(context.Background())

	requireKind(t, err, errkind.CredentialsMissing, errkind.ReasonCredentialsNotFound)
	s.cfg.Credentials = nil
	requireKind(t, s.checkCredentials(context.Background()), errkind.CredentialsMissing, errkind.ReasonCredentialsNotFound)
}
