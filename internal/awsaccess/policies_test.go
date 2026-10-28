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

func TestIAMPrincipalsAndInlinePoliciesAreReadPageByPage(t *testing.T) {
	roles := names("r", 1001)
	roles[1000].Inline = map[string]string{"b": `{"Statement":[]}`, "a": `{"x":"100%"}`}
	s, fake, _ := policySession(t, &testutil.PolicyAccount{Users: names("u", 2), Roles: roles})
	ctx := context.Background()

	got, err := s.ListPrincipals(ctx, PrincipalRole)
	if err != nil || len(got) != 1001 || got[1000] != (IAMPrincipal{Kind: PrincipalRole, Name: "r1000", ARN: testutil.RoleARN("r1000")}) {
		t.Fatalf("roles: %d %+v %v", len(got), got[len(got)-1], err)
	}
	if fake.Count(testutil.OpListRoles) != 2 {
		t.Fatalf("ListRoles called %d times, want 2 pages of 1000", fake.Count(testutil.OpListRoles))
	}
	users, err := s.ListPrincipals(ctx, PrincipalUser)
	if err != nil || len(users) != 2 || users[0].Kind != PrincipalUser {
		t.Fatalf("users: %+v %v", users, err)
	}
	if groups, err := s.ListPrincipals(ctx, PrincipalGroup); err != nil || len(groups) != 0 {
		t.Fatalf("groups: %+v %v", groups, err)
	}
	policyNames, err := s.ListInlinePolicyNames(ctx, got[1000])
	if err != nil || strings.Join(policyNames, ",") != "a,b" {
		t.Fatalf("names: %v %v", policyNames, err)
	}
	doc, err := s.GetInlinePolicyDocument(ctx, got[1000], "a")
	if err != nil || doc != testutil.EncodePolicyDocument(`{"x":"100%"}`) {
		t.Fatalf("document %q %v", doc, err)
	}
	for _, c := range fake.Calls() {
		if strings.HasPrefix(c.Action, "List") && c.Form.Get("MaxItems") != "1000" {
			t.Errorf("%s asked for %q items", c.Op, c.Form.Get("MaxItems"))
		}
	}
	if _, err := s.ListPrincipals(ctx, "service"); err == nil {
		t.Fatal("an unknown kind must fail")
	}
	if _, err := s.ListInlinePolicyNames(ctx, IAMPrincipal{Kind: "x"}); err == nil {
		t.Fatal("an unknown kind must fail")
	}
	if _, err := s.GetInlinePolicyDocument(ctx, IAMPrincipal{Kind: "x"}, "a"); err == nil {
		t.Fatal("an unknown kind must fail")
	}
}

func TestEveryKindOfPrincipalHasItsInlinePolicyCalls(t *testing.T) {
	inline := map[string]string{"p": "{}"}
	s, fake, _ := policySession(t, &testutil.PolicyAccount{
		Users: []testutil.Principal{{Name: "u", Inline: inline}}, Groups: []testutil.Principal{{Name: "g", Inline: inline}},
		Roles: []testutil.Principal{{Name: "r", Inline: inline}}})
	for _, kind := range PrincipalKinds {
		list, err := s.ListPrincipals(context.Background(), kind)
		if err != nil || len(list) != 1 {
			t.Fatalf("%s: %+v %v", kind, list, err)
		}
		if names, err := s.ListInlinePolicyNames(context.Background(), list[0]); err != nil || len(names) != 1 {
			t.Fatalf("%s names: %v %v", kind, names, err)
		}
		if doc, err := s.GetInlinePolicyDocument(context.Background(), list[0], "p"); err != nil || doc != "%7B%7D" {
			t.Fatalf("%s doc: %q %v", kind, doc, err)
		}
	}
	for _, op := range []string{testutil.OpListUserPolicies, testutil.OpListRolePolicies, testutil.OpListGroupPolicies,
		testutil.OpGetUserPolicy, testutil.OpGetRolePolicy, testutil.OpGetGroupPolicy} {
		if fake.Count(op) != 1 {
			t.Errorf("%s called %d times", op, fake.Count(op))
		}
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
// for the reads of one resource only; IAM's NoSuchEntity keeps its U1 kind
// for iam:GetPolicyVersion.
func TestNotFoundCodesOfTheSingleResourceReads(t *testing.T) {
	region := "us-east-1"
	s, _, _ := policySession(t, &testutil.PolicyAccount{})
	ctx := context.Background()
	gone := IAMPrincipal{Kind: PrincipalRole, Name: "gone", ARN: testutil.RoleARN("gone")}
	_, err := s.ListInlinePolicyNames(ctx, gone)
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
	_, err = s.GetInlinePolicyDocument(ctx, gone, "p")
	requireKind(t, err, errkind.NotFound, errkind.ReasonNone)
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
	_, err = s.GetPolicyDocument(ctx, IAMPolicyRef{ARN: testutil.ManagedPolicyARN("gone"), DefaultVersionID: "v1"})
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
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
		testutil.OpListUsers:          testutil.QueryError(http.StatusForbidden, "AccessDenied", "secret text"),
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
	_, err = s.ListPrincipals(ctx, PrincipalUser)
	check(err, errkind.AccessDenied, OpListUsers)
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
	fake.Handle(testutil.OpListUsers, testutil.Static(testutil.Reply{Body: `<ListUsersResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
		`<ListUsersResult><IsTruncated>true</IsTruncated><Users></Users></ListUsersResult></ListUsersResponse>`}))
	fake.Handle(testutil.OpGetUserPolicy, testutil.Static(testutil.Reply{Body: `<GetUserPolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">` +
		`<GetUserPolicyResult></GetUserPolicyResult></GetUserPolicyResponse>`}))
	fake.Handle(testutil.OpGetQueueAttributes, testutil.Static(testutil.Reply{Body: `{"Attributes":{}}`, ContentType: "application/x-amz-json-1.0"}))
	fake.Handle(testutil.OpDescribeKey, testutil.Static(testutil.Reply{Body: `{}`, ContentType: "application/x-amz-json-1.1"}))
	s, _ := newSession(t, fake, nil)
	ctx := context.Background()
	_, err := s.ListSecrets(ctx, "us-east-1")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.ListKeys(ctx, "us-east-1")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.ListPrincipals(ctx, PrincipalUser)
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.GetInlinePolicyDocument(ctx, IAMPrincipal{Kind: PrincipalUser, Name: "u"}, "p")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.GetQueuePolicy(ctx, "us-east-1", "https://sqs.us-east-1.amazonaws.com/123456789012/q")
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
	_, err = s.IsCustomerManagedKey(ctx, "us-east-1", KMSKey{ID: "k"})
	requireKind(t, err, errkind.Internal, errkind.ReasonNone)
}
