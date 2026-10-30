package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Tests of the fakes of apply (U6): the tag account, the write answers and
// the S3 bucket, called handler by handler.

func tagCall(op, region string, body map[string]any) Call {
	return Call{Op: op, Region: region, JSON: body}
}

func decodeJSON(t *testing.T, r Reply, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(r.Body), v); err != nil {
		t.Fatalf("answer %q: %v", r.Body, err)
	}
}

type writeAnswer struct {
	FailedResourcesMap map[string]struct {
		StatusCode int
		ErrorCode  string
	}
}

func TestTagAccountReadsByARNAndHidesNeverTaggedResources(t *testing.T) {
	a := NewTagAccount()
	a.Add("us-east-1", "arn:q1", map[string]string{"env": "dev"})
	a.Add("eu-west-1", "arn:q2", map[string]string{"env": "dev"})
	a.AddNeverTagged("us-east-1", "arn:q3")

	r := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"ResourceARNList": []any{"arn:q1", "arn:q2", "arn:q3", "arn:none"}}))

	var page struct {
		ResourceTagMappingList []struct{ ResourceARN string }
	}
	decodeJSON(t, r, &page)
	if len(page.ResourceTagMappingList) != 1 || page.ResourceTagMappingList[0].ResourceARN != "arn:q1" {
		t.Fatalf("found %+v, want arn:q1 only (other region, never tagged, missing)", page)
	}
	for _, body := range []map[string]any{
		{"ResourceARNList": []any{"arn:q1"}, "ResourcesPerPage": 100.0},
		{"ResourceARNList": []any{"arn:q1"}, "PaginationToken": "x"},
		{"ResourceARNList": make([]any, 101)},
	} {
		if r := a.getResources(tagCall(OpGetResources, "us-east-1", body)); r.Status != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", body, r.Status)
		}
	}
}

func TestTagAccountWritesChangeTheTagsAndReportFailures(t *testing.T) {
	a := NewTagAccount()
	a.AddNeverTagged("us-east-1", "arn:q1")
	big := map[string]string{}
	for i := range MaxUserTags {
		big[strings.Repeat("k", i+1)] = "v"
	}
	a.Add("us-east-1", "arn:full", big)
	a.FailWrites(func(op, arn string) *ResourceFailure {
		if arn == "arn:denied" {
			return &ResourceFailure{StatusCode: http.StatusForbidden, Code: "AccessDeniedException"}
		}
		return nil
	})

	r := a.write(tagCall(OpTagResources, "us-east-1", map[string]any{
		"ResourceARNList": []any{"arn:q1", "arn:full", "arn:denied", "arn:none"}, "Tags": map[string]any{"env": "prod"}}))

	var ans writeAnswer
	decodeJSON(t, r, &ans)
	want := map[string]int{"arn:full": 400, "arn:denied": 403, "arn:none": 404}
	if len(ans.FailedResourcesMap) != len(want) {
		t.Fatalf("failures %+v", ans.FailedResourcesMap)
	}
	for arn, status := range want {
		if ans.FailedResourcesMap[arn].StatusCode != status {
			t.Errorf("%s: status %d, want %d", arn, ans.FailedResourcesMap[arn].StatusCode, status)
		}
	}
	if got := a.Tags("arn:q1"); got["env"] != "prod" {
		t.Fatalf("tags of q1 = %v", got)
	}
	a.write(tagCall(OpUntagResources, "us-east-1", map[string]any{"ResourceARNList": []any{"arn:q1"}, "TagKeys": []any{"env"}}))
	if got := a.Tags("arn:q1"); len(got) != 0 {
		t.Fatalf("tags of q1 after untag = %v", got)
	}
	// A resource that had a tag is returned with no tags.
	page := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"ResourceARNList": []any{"arn:q1"}}))
	if !strings.Contains(page.Body, "arn:q1") {
		t.Fatalf("a resource that had a tag must be returned: %s", page.Body)
	}
	for name, body := range map[string]map[string]any{
		"aws: key":      {"ResourceARNList": []any{"arn:q1"}, "Tags": map[string]any{"AWS:x": "1"}},
		"21 resources":  {"ResourceARNList": make([]any, 21), "Tags": map[string]any{"a": "1"}},
		"no tags given": {"ResourceARNList": []any{"arn:q1"}},
	} {
		if r := a.write(tagCall(OpTagResources, "us-east-1", body)); r.Status != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, r.Status)
		}
	}
}

func TestTagAccountListsPagesAndHooks(t *testing.T) {
	a := NewTagAccount()
	for i := range 150 {
		a.Add("us-east-1", "arn:"+strings.Repeat("x", i+1), map[string]string{"a": "b"})
	}
	first := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"ResourcesPerPage": 100.0}))
	var page struct {
		PaginationToken        string
		ResourceTagMappingList []any
	}
	decodeJSON(t, first, &page)
	if len(page.ResourceTagMappingList) != 100 || page.PaginationToken == "" {
		t.Fatalf("first page: %d resources, token %q", len(page.ResourceTagMappingList), page.PaginationToken)
	}
	second := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"PaginationToken": page.PaginationToken}))
	decodeJSON(t, second, &page)
	if len(page.ResourceTagMappingList) != 50 || page.PaginationToken != "" {
		t.Fatalf("second page: %d resources, token %q", len(page.ResourceTagMappingList), page.PaginationToken)
	}
	if r := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"PaginationToken": "bad"})); r.Status != http.StatusBadRequest {
		t.Fatal("a bad token must be rejected")
	}
	seen := 0
	a.OnWrite(func(Call) { seen++ })
	a.ReplyInstead(func(c Call) *Reply {
		r := JSONError(http.StatusInternalServerError, "InternalServiceException", "x")
		return &r
	})
	if r := a.write(tagCall(OpTagResources, "us-east-1", map[string]any{})); r.Status != http.StatusInternalServerError || seen != 1 {
		t.Fatalf("status %d, hook calls %d", r.Status, seen)
	}
	a.SetTags("arn:x", map[string]string{"b": "c"})
	a.Delete("arn:xx")
	if a.Tags("arn:x")["b"] != "c" || a.Tags("arn:xx") != nil {
		t.Fatal("SetTags or Delete did not work")
	}
}

// readARNs reads the ARNs by list in us-east-1 and returns the tags found.
func readARNs(t *testing.T, a *TagAccount, arns ...string) map[string]map[string]string {
	t.Helper()
	list := make([]any, len(arns))
	for i, arn := range arns {
		list[i] = arn
	}
	var page struct {
		ResourceTagMappingList []struct {
			ResourceARN string
			Tags        []struct{ Key, Value string }
		}
	}
	decodeJSON(t, a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{"ResourceARNList": list})), &page)
	out := map[string]map[string]string{}
	for _, m := range page.ResourceTagMappingList {
		tags := map[string]string{}
		for _, tag := range m.Tags {
			tags[tag.Key] = tag.Value
		}
		out[m.ResourceARN] = tags
	}
	return out
}

// U7: a resource whose last tag was removed is returned with no tags by
// default, and left out, by ARN and by page, once the account hides them.
func TestTagAccountCanHideResourcesWithoutTags(t *testing.T) {
	a := NewTagAccount()
	a.Add("us-east-1", "arn:empty", map[string]string{"k": "v"})
	a.Add("us-east-1", "arn:tagged", map[string]string{"k": "v"})
	a.write(tagCall(OpUntagResources, "us-east-1", map[string]any{"ResourceARNList": []any{"arn:empty"}, "TagKeys": []any{"k"}}))

	if got := readARNs(t, a, "arn:empty", "arn:tagged"); len(got) != 2 || len(got["arn:empty"]) != 0 {
		t.Fatalf("default: %v", got)
	}
	a.HideEmpty(true)
	if got := readARNs(t, a, "arn:empty", "arn:tagged"); len(got) != 1 || got["arn:tagged"]["k"] != "v" {
		t.Fatalf("hidden: %v", got)
	}
	page := a.getResources(tagCall(OpGetResources, "us-east-1", map[string]any{}))
	if strings.Contains(page.Body, "arn:empty") || !strings.Contains(page.Body, "arn:tagged") {
		t.Fatalf("paged read while hidden: %s", page.Body)
	}
	// Tagging it again makes it visible.
	a.write(tagCall(OpTagResources, "us-east-1", map[string]any{"ResourceARNList": []any{"arn:empty"}, "Tags": map[string]any{"k": "w"}}))
	if got := readARNs(t, a, "arn:empty"); got["arn:empty"]["k"] != "w" {
		t.Fatalf("tagged again: %v", got)
	}
}

// U7: the reads by ARN of a lagging resource give its tags of before its
// first write as many times as asked, then the real ones; reads before that
// write and other resources are not late.
func TestTagAccountLagsTheReadsAfterAWrite(t *testing.T) {
	a := NewTagAccount()
	a.Add("us-east-1", "arn:late", map[string]string{"k": "old"})
	a.Add("us-east-1", "arn:other", map[string]string{"k": "old"})
	a.LagReads(2, "arn:late")
	if got := readARNs(t, a, "arn:late"); got["arn:late"]["k"] != "old" || a.LaggedReads() != 0 {
		t.Fatalf("before the write: %v, lagged %d", got, a.LaggedReads())
	}
	for _, op := range []string{OpTagResources, OpUntagResources} {
		body := map[string]any{"ResourceARNList": []any{"arn:late", "arn:other"}, "Tags": map[string]any{"n": "new"}}
		if op == OpUntagResources {
			body = map[string]any{"ResourceARNList": []any{"arn:late", "arn:other"}, "TagKeys": []any{"k"}}
		}
		a.write(tagCall(op, "us-east-1", body))
	}
	for i := range 2 {
		got := readARNs(t, a, "arn:late", "arn:other")
		if got["arn:late"]["k"] != "old" || got["arn:late"]["n"] != "" || got["arn:other"]["n"] != "new" {
			t.Fatalf("late read %d: %v", i+1, got)
		}
	}
	if got := readARNs(t, a, "arn:late"); got["arn:late"]["n"] != "new" || got["arn:late"]["k"] != "" || a.LaggedReads() != 2 {
		t.Fatalf("after the late reads: %v, lagged %d", got, a.LaggedReads())
	}
	if a.Tags("arn:late")["n"] != "new" {
		t.Fatal("the account itself is never late")
	}
}

func TestS3BucketConditionalWritesAndACLs(t *testing.T) {
	fake := NewFakeAWS(t)
	b := NewS3Bucket("records", "eu-west-1")
	fake.HandleS3Buckets(b)
	put := func(key, ifNoneMatch string) Reply {
		h := http.Header{}
		if ifNoneMatch != "" {
			h.Set("If-None-Match", ifNoneMatch)
		}
		return fake.handler(OpPutObject)(Call{Op: OpPutObject, Bucket: "records", Key: key, Header: h, Body: []byte("x")})
	}
	if r := put("runs/.lock", "*"); r.Status != 0 {
		t.Fatalf("first conditional write: %d", r.Status)
	}
	if r := put("runs/.lock", "*"); r.Status != http.StatusPreconditionFailed {
		t.Fatalf("second conditional write: %d", r.Status)
	}
	for _, status := range []int{http.StatusConflict, http.StatusNotImplemented} {
		b.ConditionalStatus(status)
		if r := put("runs/other", "*"); r.Status != status {
			t.Fatalf("conditional status %d: got %d", status, r.Status)
		}
	}
	b.ConditionalStatus(0)
	if r := put("runs/etag", "abc"); r.Status != http.StatusNotImplemented {
		t.Fatal("only If-None-Match: * is accepted")
	}
	get := fake.handler(OpGetObject)
	if r := get(Call{Bucket: "records", Key: "runs/.lock"}); r.Body != "x" {
		t.Fatalf("get: %+v", r)
	}
	fake.handler(OpDeleteObject)(Call{Bucket: "records", Key: "runs/.lock"})
	if r := get(Call{Bucket: "records", Key: "runs/.lock"}); r.Status != http.StatusNotFound {
		t.Fatalf("deleted object: %d", r.Status)
	}
	if r := get(Call{Bucket: "other"}); r.Status != http.StatusNotFound || !strings.Contains(r.Body, "NoSuchBucket") {
		t.Fatalf("other bucket: %+v", r)
	}
	if loc := fake.handler(OpGetBucketLocation)(Call{Bucket: "records"}); !strings.Contains(loc.Body, "eu-west-1") {
		t.Fatalf("location: %s", loc.Body)
	}
	acl := Call{Header: http.Header{"X-Amz-Acl": {"public-read"}}}
	if !HasACL(acl) || HasACL(Call{Header: http.Header{}}) {
		t.Fatal("HasACL")
	}
	b.PutObjectDirect("k", []byte("v"))
	if keys := b.Keys(); len(keys) != 1 || len(b.Puts()) != 5 {
		t.Fatalf("keys %v, puts %d", keys, len(b.Puts()))
	}
	if data, ok := b.Object("k"); !ok || string(data) != "v" {
		t.Fatal("Object")
	}
	b.ReplyInstead(func(Call) *Reply { r := S3Error(http.StatusInternalServerError, "InternalError", "x"); return &r })
	if r := get(Call{Bucket: "records", Key: "k"}); r.Status != http.StatusInternalServerError {
		t.Fatal("ReplyInstead")
	}
}

func TestDecodeAWSChunkedAndS3Keys(t *testing.T) {
	h := http.Header{"Content-Encoding": {"aws-chunked"}}
	got, err := decodeAWSChunked(h, []byte("5\r\nhello\r\n3;chunk-signature=x\r\n!!!\r\n0\r\nx-amz-checksum-crc32:abc\r\n\r\n"))
	if err != nil || string(got) != "hello!!!" {
		t.Fatalf("decoded %q, %v", got, err)
	}
	for _, bad := range []string{"zz\r\n", "5\r\nab", "no header"} {
		if _, err := decodeAWSChunked(h, []byte(bad)); err == nil {
			t.Errorf("%q must not decode", bad)
		}
	}
	if got, _ := decodeAWSChunked(http.Header{}, []byte("plain")); string(got) != "plain" {
		t.Fatal("a plain body is kept")
	}
	if k := s3Key("records.s3.eu-west-1.amazonaws.com", "/runs/a.json", "records"); k != "runs/a.json" {
		t.Fatalf("virtual-hosted key %q", k)
	}
	if k := s3Key("s3.eu-west-1.amazonaws.com", "/records/runs/a.json", "records"); k != "runs/a.json" {
		t.Fatalf("path-style key %q", k)
	}
}
