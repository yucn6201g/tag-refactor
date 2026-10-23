package awsaccess

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// Security design §1: API and IAM names live in one table; only ListResources
// needs a differently named IAM action, and no API name repeats.
func TestOperationsTableMapsAPIToIAMNames(t *testing.T) {
	seen := map[string]bool{}
	for _, op := range Operations {
		if seen[op.API] {
			t.Errorf("API %s is listed twice", op.API)
		}
		seen[op.API] = true
		if op.API != OpListResources && op.IAMAction != op.API {
			t.Errorf("%s has IAM action %s; only ListResources may differ", op.API, op.IAMAction)
		}
	}
	if got := IAMAction(OpListResources); got != "resource-explorer-2:Search" {
		t.Fatalf("IAMAction(ListResources) = %q", got)
	}
	if got := IAMAction("x:Unknown"); got != "x:Unknown" {
		t.Fatalf("an unknown name must stay as it is, got %q", got)
	}
	if seen["resource-explorer-2:Search"] {
		t.Fatal("the API Search must not be in the table")
	}
}

// NFR5.4, NFR5.5: ten attempts, waits of at most 20 seconds that grow with
// the attempt, no retry budget, and five minutes for one call.
func TestRetryerAndTimeLimitsFollowTheReliabilityDesign(t *testing.T) {
	opts := Options{}.withDefaults()
	if opts.CallTimeout != 5*time.Minute || opts.AttemptTimeout != 30*time.Second {
		t.Fatalf("time limits = %v per call, %v per attempt", opts.CallTimeout, opts.AttemptTimeout)
	}
	if opts.ExplorerPageSize != 500 || (Options{ExplorerPageSize: 1000}).withDefaults().ExplorerPageSize != 500 ||
		(Options{ExplorerPageSize: 5}).withDefaults().ExplorerPageSize != 5 {
		t.Fatal("ListResources must use 500, or a test value below 1000")
	}
	r := newRetryer(nil)
	if r.MaxAttempts() != 10 {
		t.Fatalf("MaxAttempts = %d, want 10", r.MaxAttempts())
	}
	throttle := &smithy.GenericAPIError{Code: "ThrottlingException"}
	maxAt := func(attempt int) time.Duration {
		var longest time.Duration
		for range 200 {
			d, err := r.RetryDelay(attempt, throttle)
			if err != nil {
				t.Fatal(err)
			}
			longest = max(longest, d)
		}
		return longest
	}
	if d := maxAt(1); d > 2*time.Second {
		t.Fatalf("first wait %v exceeds 2s", d)
	}
	late := maxAt(9)
	if late > MaxBackoff {
		t.Fatalf("wait %v exceeds the 20s limit", late)
	}
	if late < 10*time.Second {
		t.Fatalf("waits do not grow towards 20s: longest of 200 at attempt 9 is %v", late)
	}
	// The retry budget is off: many failures in a row still get a retry token.
	for range 500 {
		release, err := r.GetRetryToken(context.Background(), throttle)
		if err != nil {
			t.Fatalf("the retry budget stopped the retries: %v", err)
		}
		_ = release(nil)
	}
}

// NFR8.11: when the SDK cannot obtain credentials again during a call (for
// example an SSO session that ended), the error is CredentialsMissing.
func TestCredentialFailureDuringACallIsCredentialsMissing(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle("tag:GetResources", testutil.TagPages(nil))
	s, _ := newSession(t, fake, nil)
	s.cfg.Credentials = &credentialsMarker{inner: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, errors.New("refresh failed")
	})}

	_, err := s.ListTaggedResources(context.Background(), "eu-west-1")

	e := checkKind(t, err, errkind.CredentialsMissing, errkind.ReasonCredentialsExpired)
	if e.Op != OpGetResources || len(fake.Calls()) != 0 {
		t.Fatalf("op %q, %d calls", e.Op, len(fake.Calls()))
	}
	s.cfg.Credentials = &credentialsMarker{inner: s.cfg.Credentials.(*credentialsMarker).inner, sso: true}
	_, err = s.ListTaggedResources(context.Background(), "eu-west-1")
	requireKind(t, err, errkind.CredentialsMissing, errkind.ReasonSSOSessionExpired)
}

func TestConfigRecordersReadsStatusOnlyWhenThereAreRecorders(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle(testutil.OpDescribeConfigurationRecorders, testutil.Static(testutil.ConfigRecorders()))
	s, _ := newSession(t, fake, nil)

	recs, err := s.ConfigRecorders(context.Background(), "eu-west-1")

	if err != nil || len(recs) != 0 || fake.Count(testutil.OpDescribeConfigurationRecorderStatus) != 0 {
		t.Fatalf("recorders %v, err %v, status calls %d", recs, err, fake.Count(testutil.OpDescribeConfigurationRecorderStatus))
	}
	fake.HandleConfigRecorders(testutil.Recorder{Name: "r", Strategy: testutil.StrategyExclusion, Excluded: []string{"AWS::EC2::Volume"}, Recording: true})
	recs, err = s.ConfigRecorders(context.Background(), "eu-west-1")
	if err != nil || len(recs) != 1 {
		t.Fatalf("recorders %v, err %v", recs, err)
	}
	if r := recs[0]; r.Name != "r" || !r.Recording || r.Strategy != testutil.StrategyExclusion || r.ExcludedTypes != 1 || r.AllSupported {
		t.Fatalf("recorder = %+v", r)
	}
}

// Security design §3: the Config results are read one by one; an answer the
// tool does not understand fails the AWS Config path.
func TestConfigResourcesChecksEveryResult(t *testing.T) {
	item := testutil.ConfigItem{ARN: "arn:aws:sqs:eu-west-1:123456789012:q", ResourceType: "AWS::SQS::Queue", ResourceID: "q",
		Region: "eu-west-1", Tags: map[string]string{"aws:cloudformation:stack-name": "s"}}
	t.Run("pages", func(t *testing.T) {
		fake := testutil.NewFakeAWS(t)
		var items []testutil.ConfigItem
		for range 150 {
			items = append(items, item)
		}
		items = append(items, testutil.ConfigItem{ARN: "not-an-arn", ResourceType: "AWS::SSM::PatchCompliance", Region: "eu-west-1"})
		fake.Handle(testutil.OpSelectResourceConfig, testutil.ConfigPages(items...))
		s, _ := newSession(t, fake, nil)

		got, err := s.ConfigResources(context.Background(), "eu-west-1")

		if err != nil || len(got) != 151 {
			t.Fatalf("%d resources, err %v", len(got), err)
		}
		if got[0].ARN != item.ARN || got[0].Tags["aws:cloudformation:stack-name"] != "s" || got[150].ARN != "" {
			t.Fatalf("results = %+v / %+v", got[0], got[150])
		}
		for _, c := range fake.Calls() {
			if c.JSON["Limit"] != float64(100) || c.JSON["Expression"] != "SELECT arn, resourceType, resourceId, awsRegion, tags" {
				t.Fatalf("query = %v", c.JSON)
			}
		}
	})
	for name, raw := range map[string]string{
		"not JSON":         "{",
		"no resource type": `{"arn":"arn:aws:sqs:eu-west-1:1:q"}`,
		"too large":        `{"resourceType":"AWS::SQS::Queue","resourceId":"` + strings.Repeat("x", 256<<10) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			fake := testutil.NewFakeAWS(t)
			fake.Handle(testutil.OpSelectResourceConfig, testutil.Static(testutil.ConfigResults("", item.JSON(), raw)))
			s, _ := newSession(t, fake, nil)

			_, err := s.ConfigResources(context.Background(), "eu-west-1")

			e := checkKind(t, err, errkind.Internal, errkind.ReasonNone)
			if e.Op != OpSelectResourceConfig || strings.Contains(e.Error(), "xxxx") {
				t.Fatalf("error = %v", e)
			}
		})
	}
}

func TestExplorerIndexesReadsEveryPage(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle(testutil.OpListIndexes, func(c testutil.Call) testutil.Reply {
		if c.JSON["NextToken"] == "next" {
			return testutil.ExplorerIndexPage("", testutil.Index{Region: "us-east-1", Aggregator: true})
		}
		return testutil.ExplorerIndexPage("next", testutil.Index{Region: "eu-west-1"})
	})
	s, _ := newSession(t, fake, nil)

	got, err := s.ExplorerIndexes(context.Background())

	if err != nil || len(got) != 2 || got[0].Aggregator || !got[1].Aggregator || got[1].Region != "us-east-1" {
		t.Fatalf("indexes %+v, err %v", got, err)
	}
	if c := fake.Calls()[0]; c.JSON["MaxResults"] != float64(100) || c.Region != "us-east-1" {
		t.Fatalf("ListIndexes asked %v in %s", c.JSON, c.Region)
	}
}

// Security design §4: no default view, and a view that ListResources may
// not use (401), are both AccessDenied but with different reasons.
func TestDefaultViewAndListResourcesTellTheTwoFailuresApart(t *testing.T) {
	cases := map[string]testutil.Reply{
		"empty ViewArn":    testutil.DefaultView(""),
		"ResourceNotFound": testutil.RestJSONError(http.StatusNotFound, "ResourceNotFoundException", "no view"),
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			fake := testutil.NewFakeAWS(t)
			fake.Handle(testutil.OpGetDefaultView, testutil.Static(reply))
			s, _ := newSession(t, fake, nil)

			_, err := s.DefaultView(context.Background(), "eu-west-1")

			requireKind(t, err, errkind.AccessDenied, errkind.ReasonNoDefaultView)
			if fake.Count(testutil.OpGetView) != 0 {
				t.Fatal("GetView must not be called without a default view")
			}
		})
	}
	fake := testutil.NewFakeAWS(t)
	fake.HandleExplorerViews()
	fake.Handle(testutil.OpGetView, testutil.Static(testutil.View(testutil.ViewARN("eu-west-1"), "service:ec2")))
	fake.Handle(testutil.OpListResources, testutil.Static(testutil.RestJSONError(http.StatusUnauthorized, "UnauthorizedException", "no")))
	s, _ := newSession(t, fake, nil)

	view, err := s.DefaultView(context.Background(), "eu-west-1")
	if err != nil || view.ARN != testutil.ViewARN("eu-west-1") || view.Filter != "service:ec2" || !view.IncludesTags {
		t.Fatalf("view %+v, err %v", view, err)
	}
	// A view without the tags property is reported, so tag:none is not sent.
	fake.Handle(testutil.OpGetView, testutil.Static(testutil.ViewWithoutTags(testutil.ViewARN("eu-west-1"))))
	if bare, err := s.DefaultView(context.Background(), "eu-west-1"); err != nil || bare.IncludesTags {
		t.Fatalf("view without tags %+v, err %v", bare, err)
	}
	_, err = s.UntaggedResources(context.Background(), "eu-west-1", view.ARN)
	e := checkKind(t, err, errkind.AccessDenied, errkind.ReasonNone)
	if e.Op != OpListResources || IAMAction(e.Op) != "resource-explorer-2:Search" {
		t.Fatalf("op = %q", e.Op)
	}
	// Elsewhere, 401 stays an unknown error: it is mapped for ListResources only.
	fake.Handle(testutil.OpGetDefaultView, testutil.Static(testutil.RestJSONError(http.StatusUnauthorized, "UnauthorizedException", "no")))
	_, err = s.DefaultView(context.Background(), "eu-west-1")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
}

// NFR1.10: ListResources reads every page with MaxResults 500 and tag:none,
// through the view it is given; 2,500 resources need five pages.
func TestUntaggedResourcesReadsEveryPageOf500(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	var resources []testutil.ExplorerResource
	for i := range 2500 {
		resources = append(resources, testutil.ExplorerResource{ARN: "arn:aws:sqs:eu-west-1:123456789012:q" + string(rune('a'+i%26)), ResourceType: "sqs:queue", Region: "eu-west-1"})
	}
	fake.Handle(testutil.OpListResources, testutil.ExplorerPages(resources...))
	s, _ := newSession(t, fake, nil)

	got, err := s.UntaggedResources(context.Background(), "eu-west-1", testutil.ViewARN("eu-west-1"))

	if err != nil || len(got) != 2500 {
		t.Fatalf("%d resources, err %v", len(got), err)
	}
	if fake.Count(testutil.OpListResources) != 5 {
		t.Fatalf("%d pages, want 5", fake.Count(testutil.OpListResources))
	}
	for _, c := range fake.Calls() {
		filters, _ := c.JSON["Filters"].(map[string]any)
		if c.JSON["MaxResults"] != float64(500) || filters["FilterString"] != "tag:none" || c.JSON["ViewArn"] != testutil.ViewARN("eu-west-1") {
			t.Fatalf("ListResources asked %v", c.JSON)
		}
	}
	if got[0].ResourceType != "sqs:queue" || got[0].Region != "eu-west-1" {
		t.Fatalf("resource = %+v", got[0])
	}
}

// Security design §4: the expired-credential names of each service stop the
// run, whatever protocol carries them.
func TestExpiredCredentialNamesOfEveryService(t *testing.T) {
	for _, code := range []string{"ExpiredToken", "ExpiredTokenException", "RequestExpired", "TokenRefreshRequired"} {
		fake := testutil.NewFakeAWS(t)
		fake.Handle(testutil.OpListIndexes, testutil.Static(testutil.RestJSONError(http.StatusForbidden, code, "expired")))
		fake.Handle(testutil.OpDescribeConfigurationRecorders, testutil.Static(testutil.JSONError(http.StatusBadRequest, code, "expired")))
		s, _ := newSession(t, fake, nil)

		_, err := s.ExplorerIndexes(context.Background())
		requireKind(t, err, errkind.CredentialsMissing, errkind.ReasonCredentialsExpired)
		_, err = s.ConfigRecorders(context.Background(), "eu-west-1")
		requireKind(t, err, errkind.CredentialsMissing, errkind.ReasonCredentialsExpired)
	}
	for _, code := range []string{"InternalServerException", "ThrottlingException"} {
		fake := testutil.NewFakeAWS(t)
		fake.Handle(testutil.OpListIndexes, testutil.Static(testutil.RestJSONError(http.StatusInternalServerError, code, "later")))
		s, _ := newSession(t, fake, nil)
		_, err := s.ExplorerIndexes(context.Background())
		requireKind(t, err, errkind.Transient, errkind.ReasonRetriesExhausted)
	}
}
