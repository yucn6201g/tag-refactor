package testutil

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// signedRequest builds a request signed for service in region.
func signedRequest(t *testing.T, method, rawURL, service, region, body string, header map[string]string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", signedHeader(service, region))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return req
}

// roundTrip sends req to fake and returns the status, the body and the
// recorded call.
func roundTrip(t *testing.T, fake *FakeAWS, req *http.Request) (int, string, http.Header, Call) {
	t.Helper()
	resp, err := fake.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	return resp.StatusCode, readBody(t, resp), resp.Header, calls[len(calls)-1]
}

func testAccount() *PolicyAccount {
	return &PolicyAccount{
		Managed: []ManagedPolicyDoc{{Name: "attached", Document: `{"a":1}`}, {Name: "boundary", NotAttached: true, Boundaries: 2}},
		Roles:   []Principal{{Name: "app", Inline: map[string]string{"b": `{"x":"%"}`, "a": "{}"}}},
		Users:   []Principal{{Name: "u1"}, {Name: "u2"}, {Name: "u3"}},
		Buckets: []BucketPolicy{{Name: "logs", Region: "eu-west-1", Policy: `{"p":1}`}, {Name: "old", Region: "us-east-1", NoRegionInList: true}},
		Keys:    []KeyPolicy{{Region: "eu-west-1", ID: "k1", Policy: `{"k":1}`}, {Region: "eu-west-1", ID: "aws", AWSManaged: true}},
		Queues:  []RegionalPolicy{{Region: "eu-west-1", Name: "q", Policy: `{"q":1}`}, {Region: "eu-west-1", Name: "plain"}},
		Topics:  []RegionalPolicy{{Region: "eu-west-1", Name: "t", Policy: `{"t":1}`}},
		Secrets: []RegionalPolicy{{Region: "eu-west-1", Name: "s"}},
		Fail: map[string]Reply{
			OpListGroups + " " + "us-east-1":              QueryError(http.StatusForbidden, "AccessDenied", "denied"),
			OpGetRolePolicy + " " + RoleARN("app") + "#b": QueryError(http.StatusForbidden, "AccessDenied", "denied"),
		},
	}
}

// The S3 requests are told apart by bucket and query, in virtual-hosted and
// path style, and the list is cut at max-buckets.
func TestFakeS3RequestsAndBucketPages(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.HandlePolicyAccount(testAccount())

	status, body, _, call := roundTrip(t, fake, signedRequest(t, http.MethodGet, "https://s3.us-east-1.amazonaws.com/?max-buckets=1", "s3", "us-east-1", "", nil))
	if status != 200 || call.Op != OpListBuckets || !strings.Contains(body, "<BucketRegion>eu-west-1</BucketRegion>") ||
		!strings.Contains(body, "<ContinuationToken>page-1</ContinuationToken>") {
		t.Fatalf("ListBuckets: %d %s %+v", status, body, call)
	}
	_, body, _, _ = roundTrip(t, fake, signedRequest(t, http.MethodGet, "https://s3.us-east-1.amazonaws.com/?continuation-token=page-1", "s3", "us-east-1", "", nil))
	if !strings.Contains(body, "<Name>old</Name>") || strings.Contains(body, "BucketRegion") || strings.Contains(body, "ContinuationToken") {
		t.Fatalf("second page: %s", body)
	}
	_, body, _, call = roundTrip(t, fake, signedRequest(t, http.MethodGet, "https://old.s3.us-east-1.amazonaws.com/?location=", "s3", "us-east-1", "", nil))
	if call.Op != OpGetBucketLocation || call.Bucket != "old" || !strings.Contains(body, `"></LocationConstraint>`) {
		t.Fatalf("GetBucketLocation: %s %+v", body, call)
	}
	status, body, _, call = roundTrip(t, fake, signedRequest(t, http.MethodGet, "https://s3.eu-west-1.amazonaws.com/logs?policy=", "s3", "eu-west-1", "", nil))
	if status != 200 || call.Op != OpGetBucketPolicy || body != `{"p":1}` {
		t.Fatalf("GetBucketPolicy: %d %s %+v", status, body, call)
	}
	for url, code := range map[string]string{
		"https://old.s3.us-east-1.amazonaws.com/?policy=":   "NoSuchBucketPolicy",
		"https://logs.s3.us-east-1.amazonaws.com/?policy=":  "PermanentRedirect",
		"https://gone.s3.us-east-1.amazonaws.com/?policy=":  "NoSuchBucket",
		"https://s3.us-east-1.amazonaws.com/?max-buckets=0": "InvalidArgument",
	} {
		status, body, _, _ := roundTrip(t, fake, signedRequest(t, http.MethodGet, url, "s3", "us-east-1", "", nil))
		if status < 300 || !strings.Contains(body, "<Code>"+code+"</Code>") {
			t.Errorf("%s: %d %s", url, status, body)
		}
	}
}

// IAM principals and their inline policies page by MaxItems, and Fail
// replaces single answers by region or by target.
func TestFakeIAMPrincipalsInlinePoliciesAndFailures(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.HandlePolicyAccount(testAccount())
	iam := func(form url.Values) (int, string) {
		status, body, _, _ := roundTrip(t, fake, queryRequest(t, "iam", form))
		return status, body
	}
	_, body := iam(url.Values{"Action": {"ListUsers"}, "MaxItems": {"2"}})
	if !strings.Contains(body, "<UserName>u2</UserName>") || strings.Contains(body, "u3") || !strings.Contains(body, "<Marker>page-2</Marker>") {
		t.Fatalf("ListUsers: %s", body)
	}
	_, body = iam(url.Values{"Action": {"ListRolePolicies"}, "RoleName": {"app"}})
	if strings.Index(body, "<member>a</member>") > strings.Index(body, "<member>b</member>") {
		t.Fatalf("inline names must be sorted: %s", body)
	}
	_, body = iam(url.Values{"Action": {"GetRolePolicy"}, "RoleName": {"app"}, "PolicyName": {"a"}})
	if !strings.Contains(body, "<PolicyDocument>%7B%7D</PolicyDocument>") || !strings.Contains(body, "<RoleName>app</RoleName>") {
		t.Fatalf("GetRolePolicy: %s", body)
	}
	if status, body := iam(url.Values{"Action": {"GetRolePolicy"}, "RoleName": {"app"}, "PolicyName": {"b"}}); status != 403 || !strings.Contains(body, "AccessDenied") {
		t.Fatalf("Fail by target: %d %s", status, body)
	}
	if status, _ := iam(url.Values{"Action": {"ListGroups"}}); status != 403 {
		t.Fatalf("Fail by region: %d", status)
	}
	if status, body := iam(url.Values{"Action": {"ListUserPolicies"}, "UserName": {"nobody"}}); status != 404 || !strings.Contains(body, "NoSuchEntity") {
		t.Fatalf("unknown user: %d %s", status, body)
	}
	_, body = iam(url.Values{"Action": {"ListPolicies"}, "Scope": {"Local"}, "MaxItems": {"1000"}})
	if !strings.Contains(body, "<AttachmentCount>1</AttachmentCount><PermissionsBoundaryUsageCount>0</PermissionsBoundaryUsageCount>") ||
		!strings.Contains(body, "<AttachmentCount>0</AttachmentCount><PermissionsBoundaryUsageCount>2</PermissionsBoundaryUsageCount>") {
		t.Fatalf("ListPolicies: %s", body)
	}
	if status, _ := iam(url.Values{"Action": {"ListUsers"}, "MaxItems": {"1001"}}); status != 400 {
		t.Fatalf("a page above the API maximum must be rejected: %d", status)
	}
}

// KMS, SQS, SNS and Secrets Manager answer per region, and a call in the
// wrong region gets an error tag-refactor does not know.
func TestFakeRegionalPolicies(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.HandlePolicyAccount(testAccount())
	jsonCall := func(service, region, target, body string) (int, string, http.Header) {
		status, out, header, _ := roundTrip(t, fake, signedRequest(t, http.MethodPost, "https://"+service+"."+region+".amazonaws.com/", service, region, body,
			map[string]string{"X-Amz-Target": target}))
		return status, out, header
	}
	if _, body, _ := jsonCall("kms", "eu-west-1", "TrentService.ListKeys", `{"Limit":1}`); !strings.Contains(body, `"NextMarker":"page-1"`) {
		t.Fatalf("ListKeys: %s", body)
	}
	if _, body, _ := jsonCall("kms", "eu-west-1", "TrentService.DescribeKey", `{"KeyId":"aws"}`); !strings.Contains(body, `"KeyManager":"AWS"`) {
		t.Fatalf("DescribeKey: %s", body)
	}
	if _, body, _ := jsonCall("kms", "eu-west-1", "TrentService.GetKeyPolicy", `{"KeyId":"`+KeyARN("eu-west-1", "k1")+`","PolicyName":"default"}`); !strings.Contains(body, `{\"k\":1}`) {
		t.Fatalf("GetKeyPolicy: %s", body)
	}
	if status, body, _ := jsonCall("kms", "us-east-1", "TrentService.GetKeyPolicy", `{"KeyId":"k1"}`); status != 400 || !strings.Contains(body, "FakeAWSWrongRegion") {
		t.Fatalf("wrong region: %d %s", status, body)
	}
	if _, body, _ := jsonCall("sqs", "eu-west-1", "AmazonSQS.ListQueues", `{"MaxResults":1000}`); !strings.Contains(body, QueueURL("eu-west-1", "plain")) {
		t.Fatalf("ListQueues: %s", body)
	}
	_, body, _ := jsonCall("sqs", "eu-west-1", "AmazonSQS.GetQueueAttributes", `{"QueueUrl":"`+QueueURL("eu-west-1", "plain")+`"}`)
	if strings.Contains(body, "Policy") || !strings.Contains(body, QueueARN("eu-west-1", "plain")) {
		t.Fatalf("a queue without policy: %s", body)
	}
	status, body, header := jsonCall("sqs", "eu-west-1", "AmazonSQS.GetQueueAttributes", `{"QueueUrl":"https://sqs.eu-west-1.amazonaws.com/1/none"}`)
	if status != 400 || header.Get("X-Amzn-Query-Error") != "AWS.SimpleQueueService.NonExistentQueue;Sender" || !strings.Contains(body, "QueueDoesNotExist") {
		t.Fatalf("missing queue: %d %s %v", status, body, header)
	}
	if _, body, _ := jsonCall("secretsmanager", "eu-west-1", "secretsmanager.GetResourcePolicy", `{"SecretId":"`+SecretARN("eu-west-1", "s")+`"}`); strings.Contains(body, "ResourcePolicy") {
		t.Fatalf("a secret without policy: %s", body)
	}
	status, body, _, call := roundTrip(t, fake, signedRequest(t, http.MethodPost, "https://sns.eu-west-1.amazonaws.com/", "sns", "eu-west-1",
		url.Values{"Action": {"GetTopicAttributes"}, "TopicArn": {TopicARN("eu-west-1", "t")}}.Encode(), nil))
	if status != 200 || call.Op != OpGetTopicAttributes || !strings.Contains(body, "<key>Policy</key><value>{&#34;t&#34;:1}</value>") {
		t.Fatalf("GetTopicAttributes: %d %s", status, body)
	}
}

// TrackConcurrency remembers the most requests in flight per key and the
// most keys of one prefix in flight at once.
func TestFakeAWSTracksConcurrencyPerKey(t *testing.T) {
	fake := NewFakeAWS(t)
	release := make(chan struct{})
	arrived := make(chan struct{}, 3)
	fake.Handle("sns:ListTopics", func(Call) Reply {
		arrived <- struct{}{}
		<-release
		return Reply{Body: "<ListTopicsResponse/>"}
	})
	fake.TrackConcurrency(func(c Call) []string { return []string{"region:" + c.Region, "sns"} })
	var wg sync.WaitGroup
	for _, region := range []string{"eu-west-1", "eu-west-1", "us-east-1"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := fake.RoundTrip(signedRequest(t, http.MethodPost, "https://sns/", "sns", region, "Action=ListTopics", nil))
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	}
	for range 3 {
		<-arrived
	}
	close(release)
	wg.Wait()
	if fake.MaxConcurrent("sns") != 3 || fake.MaxConcurrent("region:eu-west-1") != 2 || fake.MaxDistinct("region:") != 2 {
		t.Fatalf("max sns %d, eu-west-1 %d, regions %d", fake.MaxConcurrent("sns"), fake.MaxConcurrent("region:eu-west-1"), fake.MaxDistinct("region:"))
	}
	if d := ServiceRegionDelays(1, 100)(Call{Service: "sns", Region: "eu-west-1"}); d < 0 || d >= 100 {
		t.Fatalf("delay %v", d)
	}
}
