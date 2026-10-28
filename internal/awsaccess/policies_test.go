package awsaccess

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// The reads of the U5 impact analysis through the real SDK and the HTTP
// fake: every page is read with the API's largest page size, a resource
// without a policy is not an error, and the services' NotFound codes mean a
// resource deleted after the list (security design §1).

func policySession(t *testing.T, a *testutil.PolicyAccount) (*Session, *testutil.FakeAWS, *recordingSink) {
	t.Helper()
	fake := testutil.NewFakeAWS(t)
	fake.HandlePolicyAccount(a)
	s, sink := newSession(t, fake, nil)
	return s, fake, sink
}

func names(prefix string, n int) []testutil.Principal {
	out := make([]testutil.Principal, n)
	for i := range out {
		out[i] = testutil.Principal{Name: fmt.Sprintf("%s%04d", prefix, i)}
	}
	return out
}

// U5 BR1.1, BR1.2: one read gives every customer managed policy, used or
// not, with its counts and its default version's document, and every inline
// policy of the users, roles and groups. Each page asks for the users,
// roles, groups and customer managed policies only, with the largest page
// size; IAM giving fewer items than asked for while more follow is read to
// the end.
func TestReadIAMPoliciesReadsEveryPageWithCountsAndDefaultVersions(t *testing.T) {
	current, older := `{"Statement":[{"Sid":"Current 1","Effect":"Allow"}]}`, `{"Statement":[{"Sid":"Older","Effect":"Deny"}]}`
	s, fake, sink := policySession(t, &testutil.PolicyAccount{
		Managed: []testutil.ManagedPolicyDoc{
			{Name: "attached", Document: `{"a":1}`},
			{Name: "boundary", NotAttached: true, Boundaries: 3, Document: current, OlderDocument: older},
			{Name: "unused", NotAttached: true, Document: `{"u":1}`},
		},
		Users:            []testutil.Principal{{Name: "u1", Inline: map[string]string{"p": `{"x":"100%"}`}}, {Name: "u2"}},
		Roles:            []testutil.Principal{{Name: "app", Inline: map[string]string{"b": `{"Statement":[]}`, "a": "{}"}}},
		Groups:           []testutil.Principal{{Name: "g", Inline: map[string]string{"p": `{"g":1}`}}},
		DetailsPageItems: 2,
	})

	got, err := s.ReadIAMPolicies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantManaged := []ManagedPolicy{
		{ARN: testutil.ManagedPolicyARN("attached"), AttachmentCount: 1, Document: testutil.EncodePolicyDocument(`{"a":1}`)},
		{ARN: testutil.ManagedPolicyARN("boundary"), BoundaryCount: 3, Document: testutil.EncodePolicyDocument(current)},
		{ARN: testutil.ManagedPolicyARN("unused"), Document: testutil.EncodePolicyDocument(`{"u":1}`)},
	}
	if !slices.Equal(got.Managed, wantManaged) {
		t.Fatalf("managed:\n%+v\nwant:\n%+v", got.Managed, wantManaged)
	}
	user, role, group := IAMPrincipal{PrincipalUser, "u1", testutil.UserARN("u1")}, IAMPrincipal{PrincipalRole, "app", testutil.RoleARN("app")},
		IAMPrincipal{PrincipalGroup, "g", testutil.GroupARN("g")}
	wantInline := []InlinePolicy{
		{Principal: user, Name: "p", Document: testutil.EncodePolicyDocument(`{"x":"100%"}`)},
		{Principal: role, Name: "a", Document: "%7B%7D"},
		{Principal: role, Name: "b", Document: testutil.EncodePolicyDocument(`{"Statement":[]}`)},
		{Principal: group, Name: "p", Document: testutil.EncodePolicyDocument(`{"g":1}`)},
	}
	if !slices.Equal(got.Inline, wantInline) {
		t.Fatalf("inline (the trust policy of a role is not one):\n%+v\nwant:\n%+v", got.Inline, wantInline)
	}
	// 7 items in pages of 2 although 1,000 were asked for.
	calls := fake.Calls()
	if len(calls) != 4 {
		t.Fatalf("%d calls, want 4 pages", len(calls))
	}
	for i, c := range calls {
		var filter []string
		for n := 1; c.Form.Has(fmt.Sprintf("Filter.member.%d", n)); n++ {
			filter = append(filter, c.Form.Get(fmt.Sprintf("Filter.member.%d", n)))
		}
		if c.Op != OpGetAccountAuthorizationDetails || c.Form.Get("MaxItems") != "1000" ||
			strings.Join(filter, ",") != "User,Role,Group,LocalManagedPolicy" || (i == 0) != (c.Form.Get("Marker") == "") {
			t.Errorf("call %d: %s %v", i, c.Op, c.Form)
		}
	}
	if rec := sink.last(t); rec.Op != OpGetAccountAuthorizationDetails || rec.Region != "us-east-1" || rec.ErrorKind != "" {
		t.Fatalf("debug record %+v", rec)
	}
}

// Full pages: 1,000 items per page, and a policy on the second page.
func TestReadIAMPoliciesFollowsFullPages(t *testing.T) {
	s, fake, _ := policySession(t, &testutil.PolicyAccount{
		Roles: names("r", 1001), Managed: []testutil.ManagedPolicyDoc{{Name: "last", Document: "{}"}}})

	got, err := s.ReadIAMPolicies(context.Background())
	if err != nil || len(got.Managed) != 1 || got.Managed[0].ARN != testutil.ManagedPolicyARN("last") || len(got.Inline) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if fake.Count(testutil.OpGetAccountAuthorizationDetails) != 2 {
		t.Fatalf("%d calls, want 2 pages of 1000", fake.Count(testutil.OpGetAccountAuthorizationDetails))
	}
}

// A policy whose default version, or an inline policy whose answer, has no
// document is kept and marked, so the analysis reports it as unread.
func TestIAMPoliciesWithoutADocumentAreMarked(t *testing.T) {
	s, _, _ := policySession(t, &testutil.PolicyAccount{
		Managed: []testutil.ManagedPolicyDoc{{Name: "empty", NoDocument: true, OlderDocument: `{"old":1}`}}})
	got, err := s.ReadIAMPolicies(context.Background())
	if err != nil || len(got.Managed) != 1 || !got.Managed[0].DocumentMissing || got.Managed[0].Document != "" {
		t.Fatalf("an older version must not stand in for the default one: %+v %v", got, err)
	}

	fake := testutil.NewFakeAWS(t)
	fake.Handle(testutil.OpGetAccountAuthorizationDetails, testutil.Static(testutil.Reply{Body: `<GetAccountAuthorizationDetailsResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
		`<GetAccountAuthorizationDetailsResult><IsTruncated>false</IsTruncated><RoleDetailList><member><RoleName>app</RoleName>` +
		`<Arn>arn:aws:iam::123456789012:role/app</Arn><RolePolicyList><member><PolicyName>p</PolicyName></member></RolePolicyList>` +
		`</member></RoleDetailList><Policies><member><Arn>arn:aws:iam::123456789012:policy/v</Arn><AttachmentCount>1</AttachmentCount>` +
		`<PolicyVersionList></PolicyVersionList></member></Policies></GetAccountAuthorizationDetailsResult></GetAccountAuthorizationDetailsResponse>`}))
	s, _ = newSession(t, fake, nil)
	got, err = s.ReadIAMPolicies(context.Background())
	want := IAMPolicies{
		Managed: []ManagedPolicy{{ARN: "arn:aws:iam::123456789012:policy/v", AttachmentCount: 1, DocumentMissing: true}},
		Inline:  []InlinePolicy{{Principal: IAMPrincipal{PrincipalRole, "app", "arn:aws:iam::123456789012:role/app"}, Name: "p", DocumentMissing: true}},
	}
	if err != nil || !slices.Equal(got.Managed, want.Managed) || !slices.Equal(got.Inline, want.Inline) {
		t.Fatalf("%+v %v", got, err)
	}
}

// The read fails as a whole: a denied, throttled or credential-rejected
// page, the first or a later one, gives no policies at all and names the
// operation; AWS's error text never reaches the error.
func TestReadIAMPoliciesFailsAsAWhole(t *testing.T) {
	op := testutil.OpGetAccountAuthorizationDetails
	for name, sc := range map[string]struct {
		fail   map[string]testutil.Reply
		kind   errkind.Kind
		reason errkind.Reason
	}{
		"denied":            {map[string]testutil.Reply{op: testutil.QueryError(http.StatusForbidden, "AccessDenied", "secret text")}, errkind.AccessDenied, errkind.ReasonNone},
		"denied third page": {map[string]testutil.Reply{op + " page-4": testutil.QueryError(http.StatusForbidden, "AccessDenied", "secret text")}, errkind.AccessDenied, errkind.ReasonNone},
		"throttled":         {map[string]testutil.Reply{op + " page-2": testutil.QueryError(http.StatusBadRequest, "Throttling", "secret text")}, errkind.Transient, errkind.ReasonRetriesExhausted},
		"expired":           {map[string]testutil.Reply{op: testutil.QueryError(http.StatusForbidden, "ExpiredToken", "secret text")}, errkind.CredentialsMissing, errkind.ReasonCredentialsExpired},
	} {
		t.Run(name, func(t *testing.T) {
			s, _, _ := policySession(t, &testutil.PolicyAccount{Roles: names("r", 6), DetailsPageItems: 2, Fail: sc.fail})
			got, err := s.ReadIAMPolicies(context.Background())
			e := checkKind(t, err, sc.kind, sc.reason)
			if e.Op != OpGetAccountAuthorizationDetails || strings.Contains(fmt.Sprint(e.Detail, e.Hint), "secret text") {
				t.Fatalf("error %+v", e)
			}
			if got.Managed != nil || got.Inline != nil {
				t.Fatalf("a failed read gives no policies: %+v", got)
			}
		})
	}
}

func TestBucketsTheirRegionsAndPolicies(t *testing.T) {
	buckets := []testutil.BucketPolicy{
		{Name: "logs", Region: "eu-west-1", Policy: `{"Statement":[]}`},
		{Name: "plain", Region: "us-east-1"},
		{Name: "old", Region: "us-east-1", NoRegionInList: true},
		{Name: "europe", Region: "eu-west-1", NoRegionInList: true},
	}
	s, fake, sink := policySession(t, &testutil.PolicyAccount{Buckets: buckets})
	ctx := context.Background()

	list, err := s.ListBuckets(ctx)
	want := []Bucket{{"logs", "eu-west-1"}, {"plain", "us-east-1"}, {"old", ""}, {"europe", ""}}
	if err != nil || !slices.Equal(list, want) {
		t.Fatalf("buckets %+v %v", list, err)
	}
	if c := fake.Calls()[0]; c.Query.Get("max-buckets") != "10000" {
		t.Fatalf("ListBuckets asked for %q", c.Query.Get("max-buckets"))
	}
	if region, err := s.GetBucketRegion(ctx, "old"); err != nil || region != "us-east-1" {
		t.Fatalf("an empty location is us-east-1: %q %v", region, err)
	}
	if region, err := s.GetBucketRegion(ctx, "europe"); err != nil || region != "eu-west-1" {
		t.Fatalf("location: %q %v", region, err)
	}
	policy, err := s.GetBucketPolicy(ctx, "eu-west-1", "logs")
	if err != nil || policy != (ResourcePolicy{ARN: "arn:aws:s3:::logs", Document: `{"Statement":[]}`, Found: true}) {
		t.Fatalf("policy %+v %v", policy, err)
	}
	none, err := s.GetBucketPolicy(ctx, "us-east-1", "plain")
	if err != nil || none.Found || none.ARN != "arn:aws:s3:::plain" {
		t.Fatalf("a bucket without a policy is not an error: %+v %v", none, err)
	}
	if rec := sink.last(t); rec.Op != OpGetBucketPolicy || rec.ErrorKind != "" || rec.Region != "us-east-1" {
		t.Fatalf("debug record %+v", rec)
	}
	_, err = s.GetBucketPolicy(ctx, "us-east-1", "deleted")
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetBucketRegion(ctx, "deleted")
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	for _, c := range fake.Calls() {
		if c.Op == testutil.OpGetBucketPolicy && c.Bucket == "logs" && c.Region != "eu-west-1" {
			t.Fatalf("the policy of a bucket is read in its region, got %s", c.Region)
		}
	}
}

func TestBucketListPagesAndPartitions(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle(testutil.OpListBuckets, testutil.Sequence(
		testutil.BucketPage("tok", testutil.BucketPolicy{Name: "a", Region: "us-east-1"}),
		testutil.BucketPage("", testutil.BucketPolicy{Name: "b", Region: "us-east-1"})))
	s, _ := newSession(t, fake, nil)
	list, err := s.ListBuckets(context.Background())
	if err != nil || len(list) != 2 || fake.Calls()[1].Query.Get("continuation-token") != "tok" {
		t.Fatalf("%+v %v", list, err)
	}
	for region, want := range map[string]string{"cn-north-1": "aws-cn", "us-gov-west-1": "aws-us-gov", "eu-west-1": "aws"} {
		if got := partitionOf(region); got != want {
			t.Errorf("%s: %s", region, got)
		}
	}
}

func TestKeysQueuesTopicsAndSecrets(t *testing.T) {
	region := "us-east-1"
	var keys []testutil.KeyPolicy
	for i := 0; i < 1001; i++ {
		keys = append(keys, testutil.KeyPolicy{Region: region, ID: fmt.Sprintf("k%04d", i), Policy: `{"k":1}`, AWSManaged: i == 0})
	}
	var topics []testutil.RegionalPolicy
	for i := 0; i < 250; i++ {
		topics = append(topics, testutil.RegionalPolicy{Region: region, Name: fmt.Sprintf("t%03d", i)})
	}
	topics[249].Policy = `{"t":1}`
	s, fake, _ := policySession(t, &testutil.PolicyAccount{Keys: keys, Topics: topics,
		Queues:  []testutil.RegionalPolicy{{Region: region, Name: "q", Policy: `{"q":1}`}, {Region: region, Name: "plain"}},
		Secrets: []testutil.RegionalPolicy{{Region: region, Name: "s", Policy: `{"s":1}`}, {Region: region, Name: "plain"}}})
	ctx := context.Background()

	list, err := s.ListKeys(ctx, region)
	if err != nil || len(list) != 1001 || list[0] != (KMSKey{ID: "k0000", ARN: testutil.KeyARN(region, "k0000")}) || fake.Count(testutil.OpListKeys) != 2 {
		t.Fatalf("keys: %d %v, %d calls", len(list), err, fake.Count(testutil.OpListKeys))
	}
	if customer, err := s.IsCustomerManagedKey(ctx, region, list[0]); err != nil || customer {
		t.Fatalf("an AWS managed key: %v %v", customer, err)
	}
	if customer, err := s.IsCustomerManagedKey(ctx, region, list[1]); err != nil || !customer {
		t.Fatalf("a customer key: %v %v", customer, err)
	}
	if p, err := s.GetKeyPolicy(ctx, region, KMSKey{ID: "k0001"}); err != nil || !p.Found || p.Document != `{"k":1}` {
		t.Fatalf("key policy: %+v %v", p, err)
	}

	urls, err := s.ListQueues(ctx, region)
	if err != nil || len(urls) != 2 {
		t.Fatalf("queues: %v %v", urls, err)
	}
	q, err := s.GetQueuePolicy(ctx, region, urls[0])
	if err != nil || q != (ResourcePolicy{ARN: testutil.QueueARN(region, "q"), Document: `{"q":1}`, Found: true}) {
		t.Fatalf("queue policy: %+v %v", q, err)
	}
	if plain, err := s.GetQueuePolicy(ctx, region, urls[1]); err != nil || plain.Found || plain.ARN != testutil.QueueARN(region, "plain") {
		t.Fatalf("a queue without Policy: %+v %v", plain, err)
	}

	topicARNs, err := s.ListTopics(ctx, region)
	if err != nil || len(topicARNs) != 250 || fake.Count(testutil.OpListTopics) != 3 {
		t.Fatalf("topics: %d %v, %d pages", len(topicARNs), err, fake.Count(testutil.OpListTopics))
	}
	if p, err := s.GetTopicPolicy(ctx, region, topicARNs[249]); err != nil || !p.Found || p.Document != `{"t":1}` {
		t.Fatalf("topic policy: %+v %v", p, err)
	}
	if p, err := s.GetTopicPolicy(ctx, region, topicARNs[0]); err != nil || p.Found {
		t.Fatalf("a topic without Policy: %+v %v", p, err)
	}

	secrets, err := s.ListSecrets(ctx, region)
	if err != nil || len(secrets) != 2 {
		t.Fatalf("secrets: %v %v", secrets, err)
	}
	if p, err := s.GetSecretPolicy(ctx, region, secrets[0]); err != nil || !p.Found || p.Document != `{"s":1}` {
		t.Fatalf("secret policy: %+v %v", p, err)
	}
	if p, err := s.GetSecretPolicy(ctx, region, secrets[1]); err != nil || p.Found {
		t.Fatalf("a secret without policy: %+v %v", p, err)
	}
	if fake.Count("secretsmanager:GetSecretValue") != 0 {
		t.Fatal("a secret's value must never be read")
	}
	for _, c := range fake.Calls() {
		switch c.Op {
		case testutil.OpListKeys:
			if c.JSON["Limit"] != float64(1000) {
				t.Errorf("ListKeys Limit %v", c.JSON["Limit"])
			}
		case testutil.OpListQueues:
			if c.JSON["MaxResults"] != float64(1000) {
				t.Errorf("ListQueues MaxResults %v", c.JSON["MaxResults"])
			}
		case testutil.OpListSecrets:
			if c.JSON["MaxResults"] != float64(100) {
				t.Errorf("ListSecrets MaxResults %v", c.JSON["MaxResults"])
			}
		case testutil.OpGetKeyPolicy:
			if c.JSON["PolicyName"] != "default" {
				t.Errorf("GetKeyPolicy PolicyName %v", c.JSON["PolicyName"])
			}
		case testutil.OpGetQueueAttributes:
			if fmt.Sprint(c.JSON["AttributeNames"]) != "[Policy QueueArn]" {
				t.Errorf("GetQueueAttributes asked for %v", c.JSON["AttributeNames"])
			}
		}
	}
}

// Security design §1: each service's NotFound codes mean a deleted resource
// for the reads of one resource only; IAM's NoSuchEntity, which no read of
// the impact analysis expects, stays an unexpected error.
func TestNotFoundCodesOfTheSingleResourceReads(t *testing.T) {
	region := "us-east-1"
	s, _, _ := policySession(t, &testutil.PolicyAccount{Fail: map[string]testutil.Reply{
		testutil.OpGetAccountAuthorizationDetails: testutil.QueryError(http.StatusNotFound, "NoSuchEntity", "no such entity"),
	}})
	ctx := context.Background()
	_, err := s.ReadIAMPolicies(ctx)
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.IsCustomerManagedKey(ctx, region, KMSKey{ID: "gone"})
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetKeyPolicy(ctx, region, KMSKey{ID: "gone"})
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetQueuePolicy(ctx, region, testutil.QueueURL(region, "gone"))
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetTopicPolicy(ctx, region, testutil.TopicARN(region, "gone"))
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetSecretPolicy(ctx, region, testutil.SecretARN(region, "gone"))
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
}

// Security design §1: the services' own names for access denied and for
// temporary failures, and their error text never reaches the error.
func TestAccessDeniedAndTransientCodesOfTheNewServices(t *testing.T) {
	region := "us-east-1"
	denied := map[string]testutil.Reply{
		testutil.OpListBuckets:        testutil.S3Error(http.StatusForbidden, "AccessDenied", "secret text"),
		testutil.OpListKeys:           testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "secret text"),
		testutil.OpListQueues:         testutil.SQSError(http.StatusForbidden, "AccessDenied", "AccessDenied", "secret text"),
		testutil.OpListTopics:         testutil.QueryError(http.StatusForbidden, "AuthorizationError", "secret text"),
		testutil.OpListSecrets:        testutil.JSONError(http.StatusBadRequest, "AccessDeniedException", "secret text"),
		testutil.OpGetKeyPolicy:       testutil.JSONError(http.StatusBadRequest, "KMSInvalidStateException", "secret text"),
		testutil.OpGetTopicAttributes: testutil.QueryError(http.StatusBadRequest, "Throttling", "secret text"),
	}
	s, _, _ := policySession(t, &testutil.PolicyAccount{Fail: denied,
		Topics: []testutil.RegionalPolicy{{Region: region, Name: "t"}}})
	ctx := context.Background()
	check := func(err error, kind errkind.Kind, op string) {
		t.Helper()
		e := checkKind(t, err, kind, map[errkind.Kind]errkind.Reason{errkind.Transient: errkind.ReasonRetriesExhausted}[kind])
		if e.Op != op || strings.Contains(fmt.Sprint(e.Detail, e.Hint), "secret text") {
			t.Fatalf("error %+v", e)
		}
	}
	_, err := s.ListBuckets(ctx)
	check(err, errkind.AccessDenied, OpListBuckets)
	_, err = s.ListKeys(ctx, region)
	check(err, errkind.AccessDenied, OpListKeys)
	_, err = s.ListQueues(ctx, region)
	check(err, errkind.AccessDenied, OpListQueues)
	_, err = s.ListTopics(ctx, region)
	check(err, errkind.AccessDenied, OpListTopics)
	_, err = s.ListSecrets(ctx, region)
	check(err, errkind.AccessDenied, OpListSecrets)
	_, err = s.GetKeyPolicy(ctx, region, KMSKey{ID: "k"})
	check(err, errkind.Internal, OpGetKeyPolicy)
	_, err = s.GetTopicPolicy(ctx, region, testutil.TopicARN(region, "t"))
	check(err, errkind.Transient, OpGetTopicAttributes)
}

// A list that repeats its token or stops without one is an Internal error,
// never an endless loop.
func TestListsThatRepeatTheirTokenStop(t *testing.T) {
	fake := testutil.NewFakeAWS(t)
	fake.Handle(testutil.OpListSecrets, testutil.Static(testutil.Reply{Body: `{"SecretList":[],"NextToken":"same"}`,
		ContentType: "application/x-amz-json-1.1"}))
	fake.Handle(testutil.OpListKeys, testutil.Static(testutil.Reply{Body: `{"Keys":[],"Truncated":true}`,
		ContentType: "application/x-amz-json-1.1"}))
	// IAM: a truncated page without a marker, then one that repeats it.
	details := func(marker string) testutil.Reply {
		return testutil.Reply{Body: `<GetAccountAuthorizationDetailsResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
			`<GetAccountAuthorizationDetailsResult><IsTruncated>true</IsTruncated>` + marker +
			`</GetAccountAuthorizationDetailsResult></GetAccountAuthorizationDetailsResponse>`}
	}
	fake.Handle(testutil.OpGetAccountAuthorizationDetails, testutil.Sequence(details(""), details("<Marker>m</Marker>")))
	fake.Handle(testutil.OpGetQueueAttributes, testutil.Static(testutil.Reply{Body: `{"Attributes":{}}`, ContentType: "application/x-amz-json-1.0"}))
	fake.Handle(testutil.OpDescribeKey, testutil.Static(testutil.Reply{Body: `{}`, ContentType: "application/x-amz-json-1.1"}))
	s, _ := newSession(t, fake, nil)
	ctx := context.Background()
	_, err := s.ListSecrets(ctx, "us-east-1")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.ListKeys(ctx, "us-east-1")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.ReadIAMPolicies(ctx)
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.ReadIAMPolicies(ctx) // m, then m again
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	if n := fake.Count(testutil.OpGetAccountAuthorizationDetails); n != 3 {
		t.Fatalf("%d IAM calls, want 1 and 2", n)
	}
	_, err = s.GetQueuePolicy(ctx, "us-east-1", "https://sqs.us-east-1.amazonaws.com/123456789012/q")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.IsCustomerManagedKey(ctx, "us-east-1", KMSKey{ID: "k"})
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
}
