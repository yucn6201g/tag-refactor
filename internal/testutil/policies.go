package testutil

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Answers of IAM, S3, KMS, SQS, SNS and Secrets Manager for the impact
// analysis (U5). PolicyAccount describes the policies of a fake account, and
// HandlePolicyAccount answers every read of the impact analysis from it, page
// by page with the page sizes of the APIs (U5 logical-components.md), so the
// tests can count the calls.

// Operation names of the impact analysis calls, as the fake AWS records them.
const (
	OpListPolicies        = "iam:ListPolicies"
	OpGetPolicyVersion    = "iam:GetPolicyVersion"
	OpListUsers           = "iam:ListUsers"
	OpListRoles           = "iam:ListRoles"
	OpListGroups          = "iam:ListGroups"
	OpListUserPolicies    = "iam:ListUserPolicies"
	OpListRolePolicies    = "iam:ListRolePolicies"
	OpListGroupPolicies   = "iam:ListGroupPolicies"
	OpGetUserPolicy       = "iam:GetUserPolicy"
	OpGetRolePolicy       = "iam:GetRolePolicy"
	OpGetGroupPolicy      = "iam:GetGroupPolicy"
	OpListBuckets         = "s3:ListBuckets"
	OpGetBucketLocation   = "s3:GetBucketLocation"
	OpGetBucketPolicy     = "s3:GetBucketPolicy"
	OpListKeys            = "kms:ListKeys"
	OpDescribeKey         = "kms:DescribeKey"
	OpGetKeyPolicy        = "kms:GetKeyPolicy"
	OpListQueues          = "sqs:ListQueues"
	OpGetQueueAttributes  = "sqs:GetQueueAttributes"
	OpListTopics          = "sns:ListTopics"
	OpGetTopicAttributes  = "sns:GetTopicAttributes"
	OpListSecrets         = "secretsmanager:ListSecrets"
	OpGetResourcePolicy   = "secretsmanager:GetResourcePolicy"
	sqsJSONContentType    = "application/x-amz-json-1.0"
	s3XMLNamespace        = "http://s3.amazonaws.com/doc/2006-03-01/"
	iamXMLNamespace       = "https://iam.amazonaws.com/doc/2010-05-08/"
	snsXMLNamespace       = "http://sns.amazonaws.com/doc/2010-03-31/"
	fakeRequestIDMetadata = `<ResponseMetadata><RequestId>fake-request-id</RequestId></ResponseMetadata>`
)

// PageSize is the largest page of a list API and the page it gives when the
// request does not ask for a size.
type PageSize struct{ Max, Default int }

// PageSizes are the page sizes of the list APIs of the impact analysis. A
// request for more than Max is rejected like AWS does.
var PageSizes = map[string]PageSize{
	OpListPolicies:      {1000, 100},
	OpListUsers:         {1000, 100},
	OpListRoles:         {1000, 100},
	OpListGroups:        {1000, 100},
	OpListUserPolicies:  {1000, 100},
	OpListRolePolicies:  {1000, 100},
	OpListGroupPolicies: {1000, 100},
	OpListBuckets:       {10000, 10000},
	OpListKeys:          {1000, 100},
	OpListQueues:        {1000, 1000},
	OpListTopics:        {100, 100},
	OpListSecrets:       {100, 100},
}

// ManagedPolicyDoc is one customer managed policy and its default version's
// document.
type ManagedPolicyDoc struct {
	Name     string
	Document string
	// NotAttached gives the policy an AttachmentCount of 0.
	NotAttached bool
	// Boundaries is its PermissionsBoundaryUsageCount.
	Boundaries int
}

// Principal is an IAM user, role or group with its inline policies (policy
// name to document).
type Principal struct {
	Name   string
	Inline map[string]string
}

// BucketPolicy is an S3 bucket and its bucket policy.
type BucketPolicy struct {
	Name   string
	Region string
	// NoRegionInList leaves BucketRegion out of the ListBuckets answer, so the
	// region has to be read with GetBucketLocation.
	NoRegionInList bool
	// Policy is the bucket policy; empty means the bucket has none.
	Policy string
}

// KeyPolicy is a KMS key and its key policy.
type KeyPolicy struct {
	Region string
	ID     string
	// AWSManaged makes DescribeKey answer KeyManager AWS.
	AWSManaged bool
	Policy     string
}

// RegionalPolicy is an SQS queue, an SNS topic or a Secrets Manager secret
// and its resource policy; an empty Policy means it has none.
type RegionalPolicy struct {
	Region string
	Name   string
	Policy string
}

// PolicyAccount is the content of a fake account that the impact analysis
// reads. Build it completely before HandlePolicyAccount: the handlers index
// it once.
type PolicyAccount struct {
	Managed []ManagedPolicyDoc
	Users   []Principal
	Roles   []Principal
	Groups  []Principal
	Buckets []BucketPolicy
	Keys    []KeyPolicy
	Queues  []RegionalPolicy
	Topics  []RegionalPolicy
	Secrets []RegionalPolicy
	// Fail replaces answers. The key is "<op> <target>", "<op> <region>" or
	// "<op>", tried in that order. The target is the policy reference of what
	// the call reads: the policy ARN, "<principal ARN>" for the list of a
	// principal's inline policies, "<principal ARN>#<name>" for one inline
	// policy, or the ARN of the bucket, key, queue, topic or secret.
	Fail map[string]Reply
}

// ManagedPolicyARN is the ARN of a customer managed policy of TestAccount.
func ManagedPolicyARN(name string) string { return "arn:aws:iam::" + TestAccount + ":policy/" + name }

// UserARN is the ARN of an IAM user of TestAccount.
func UserARN(name string) string { return "arn:aws:iam::" + TestAccount + ":user/" + name }

// RoleARN is the ARN of an IAM role of TestAccount.
func RoleARN(name string) string { return "arn:aws:iam::" + TestAccount + ":role/" + name }

// GroupARN is the ARN of an IAM group of TestAccount.
func GroupARN(name string) string { return "arn:aws:iam::" + TestAccount + ":group/" + name }

// BucketARN is the ARN of an S3 bucket.
func BucketARN(name string) string { return "arn:aws:s3:::" + name }

// KeyARN is the ARN of a KMS key of TestAccount.
func KeyARN(region, id string) string {
	return "arn:aws:kms:" + region + ":" + TestAccount + ":key/" + id
}

// QueueARN is the ARN of an SQS queue of TestAccount.
func QueueARN(region, name string) string {
	return "arn:aws:sqs:" + region + ":" + TestAccount + ":" + name
}

// QueueURL is the URL of an SQS queue of TestAccount.
func QueueURL(region, name string) string {
	return "https://sqs." + region + ".amazonaws.com/" + TestAccount + "/" + name
}

// TopicARN is the ARN of an SNS topic of TestAccount.
func TopicARN(region, name string) string {
	return "arn:aws:sns:" + region + ":" + TestAccount + ":" + name
}

// SecretARN is the ARN of a Secrets Manager secret of TestAccount.
func SecretARN(region, name string) string {
	return "arn:aws:secretsmanager:" + region + ":" + TestAccount + ":secret:" + name
}

// principalKinds names the three kinds of IAM principals as the API does.
var principalKinds = []struct {
	kind string // "User", "Role" or "Group"
	arn  func(string) string
	list func(a *PolicyAccount) []Principal
}{
	{"User", UserARN, func(a *PolicyAccount) []Principal { return a.Users }},
	{"Role", RoleARN, func(a *PolicyAccount) []Principal { return a.Roles }},
	{"Group", GroupARN, func(a *PolicyAccount) []Principal { return a.Groups }},
}

// HandleNoPolicySources registers the answers of an account without IAM
// users, roles, groups, customer managed policies or resources with resource
// policies. HandleDefaults registers it, so tests that do not care about the
// impact analysis keep working when plan reads every policy source (U5).
func (f *FakeAWS) HandleNoPolicySources() {
	f.HandlePolicyAccount(&PolicyAccount{})
}

// HandlePolicyAccount registers the answers of every read of the impact
// analysis from a.
func (f *FakeAWS) HandlePolicyAccount(a *PolicyAccount) {
	x := newAccountIndex(a)
	managed := make([]ManagedPolicy, 0, len(a.Managed))
	docs := map[string]string{}
	for _, m := range a.Managed {
		arn := ManagedPolicyARN(m.Name)
		managed = append(managed, ManagedPolicy{ARN: arn, NotAttached: m.NotAttached, Boundaries: m.Boundaries})
		docs[arn] = m.Document
	}
	f.Handle(OpListPolicies, x.guard(OpListPolicies, func(c Call) Reply {
		start, end, next, bad := pageOf(c, OpListPolicies, len(managed), c.Form.Get("MaxItems"), c.Form.Get("Marker"))
		if bad != nil {
			return *bad
		}
		return PolicyPage(next, managed[start:end]...)
	}, nil))
	f.Handle(OpGetPolicyVersion, x.guard(OpGetPolicyVersion, PolicyDocuments(docs), func(c Call) string {
		return c.Form.Get("PolicyArn")
	}))
	for _, k := range principalKinds {
		f.handlePrincipals(x, k.kind, k.arn, k.list(a))
	}
	f.handleBuckets(x)
	f.handleKeys(x)
	f.handleQueues(x)
	f.handleTopics(x)
	f.handleSecrets(x)
}

// accountIndex looks the resources of a PolicyAccount up by the names the
// requests carry.
type accountIndex struct {
	a       *PolicyAccount
	buckets map[string]BucketPolicy
	keys    map[string]KeyPolicy
	queues  map[string]RegionalPolicy
	topics  map[string]RegionalPolicy
	secrets map[string]RegionalPolicy
}

func newAccountIndex(a *PolicyAccount) *accountIndex {
	x := &accountIndex{a: a, buckets: map[string]BucketPolicy{}, keys: map[string]KeyPolicy{},
		queues: map[string]RegionalPolicy{}, topics: map[string]RegionalPolicy{}, secrets: map[string]RegionalPolicy{}}
	for _, b := range a.Buckets {
		x.buckets[b.Name] = b
	}
	for _, k := range a.Keys {
		x.keys[k.ID] = k
		x.keys[KeyARN(k.Region, k.ID)] = k
	}
	for _, q := range a.Queues {
		x.queues[QueueURL(q.Region, q.Name)] = q
	}
	for _, t := range a.Topics {
		x.topics[TopicARN(t.Region, t.Name)] = t
	}
	for _, s := range a.Secrets {
		x.secrets[SecretARN(s.Region, s.Name)] = s
		x.secrets[s.Name] = s
	}
	return x
}

// guard answers a call from Fail when a replacement is registered for it,
// and from h otherwise. target names what the call reads (nil for lists).
func (x *accountIndex) guard(op string, h Handler, target func(Call) string) Handler {
	return func(c Call) Reply {
		if target != nil {
			if r, ok := x.a.Fail[op+" "+target(c)]; ok {
				return r
			}
		}
		if r, ok := x.a.Fail[op+" "+c.Region]; ok {
			return r
		}
		if r, ok := x.a.Fail[op]; ok {
			return r
		}
		return h(c)
	}
}

// pageOf returns the part [start, end) of a list of n items that the call
// asks for, and the token of the next page ("" on the last one). size is the
// requested page size ("" for none) and token the token of the request.
// The tokens are "page-<start>".
func pageOf(c Call, op string, n int, size, token string) (start, end int, next string, bad *Reply) {
	limits := PageSizes[op]
	per := limits.Default
	if size != "" {
		v, err := strconv.Atoi(size)
		if err != nil || v <= 0 || v > limits.Max {
			r := pageError(c, "page size "+size+" is not allowed")
			return 0, 0, "", &r
		}
		per = v
	}
	if token != "" {
		v, err := strconv.Atoi(strings.TrimPrefix(token, "page-"))
		if err != nil || v <= 0 || v >= n {
			r := pageError(c, "bad pagination token")
			return 0, 0, "", &r
		}
		start = v
	}
	end = min(start+per, n)
	if end < n {
		next = "page-" + strconv.Itoa(end)
	}
	return start, end, next, nil
}

// pageError is the validation error of the service of c.
func pageError(c Call, message string) Reply {
	switch c.Service {
	case "iam", "sns":
		return QueryError(http.StatusBadRequest, "InvalidInput", message)
	case "s3":
		return S3Error(http.StatusBadRequest, "InvalidArgument", message)
	case "sqs":
		return SQSError(http.StatusBadRequest, "InvalidParameterValue", "InvalidParameterValue", message)
	default:
		return JSONError(http.StatusBadRequest, "ValidationException", message)
	}
}

// wrongRegion is the answer to a call made in another region than the
// resource's. Its code is unknown to tag-refactor, so such a mistake shows up as
// an unread policy instead of passing as "no policy".
func wrongRegion(c Call) Reply {
	const code, message = "FakeAWSWrongRegion", "the resource is in another region"
	switch c.Service {
	case "sns":
		return QueryError(http.StatusBadRequest, code, message)
	case "sqs":
		return SQSError(http.StatusBadRequest, code, code, message)
	default:
		return JSONError(http.StatusBadRequest, code, message)
	}
}

// jsonNumber renders a number of a JSON request as the text pageOf reads.
func jsonNumber(v any) string {
	if n := intParam(v, 0); n > 0 {
		return strconv.Itoa(n)
	}
	if v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

func jsonString(v any) string {
	s, _ := v.(string)
	return s
}

// --- IAM users, roles and groups ---

func (f *FakeAWS) handlePrincipals(x *accountIndex, kind string, arnOf func(string) string, principals []Principal) {
	byName := map[string]Principal{}
	for _, p := range principals {
		byName[p.Name] = p
	}
	listOp, namesOp, getOp := "iam:List"+kind+"s", "iam:List"+kind+"Policies", "iam:Get"+kind+"Policy"
	f.Handle(listOp, x.guard(listOp, func(c Call) Reply {
		start, end, next, bad := pageOf(c, listOp, len(principals), c.Form.Get("MaxItems"), c.Form.Get("Marker"))
		if bad != nil {
			return *bad
		}
		return principalPage(kind, arnOf, next, principals[start:end])
	}, nil))
	f.Handle(namesOp, x.guard(namesOp, func(c Call) Reply {
		p, ok := byName[c.Form.Get(kind+"Name")]
		if !ok {
			return QueryError(http.StatusNotFound, "NoSuchEntity", "no such "+strings.ToLower(kind))
		}
		names := make([]string, 0, len(p.Inline))
		for n := range p.Inline {
			names = append(names, n)
		}
		sort.Strings(names)
		start, end, next, bad := pageOf(c, namesOp, len(names), c.Form.Get("MaxItems"), c.Form.Get("Marker"))
		if bad != nil {
			return *bad
		}
		return inlineNamesPage(kind, next, names[start:end])
	}, func(c Call) string { return arnOf(c.Form.Get(kind + "Name")) }))
	f.Handle(getOp, x.guard(getOp, func(c Call) Reply {
		p, ok := byName[c.Form.Get(kind+"Name")]
		doc, found := p.Inline[c.Form.Get("PolicyName")]
		if !ok || !found {
			return QueryError(http.StatusNotFound, "NoSuchEntity", "no such policy")
		}
		return InlinePolicy(kind, p.Name, c.Form.Get("PolicyName"), doc)
	}, func(c Call) string { return arnOf(c.Form.Get(kind+"Name")) + "#" + c.Form.Get("PolicyName") }))
}

// principalPage is one iam:ListUsers, ListRoles or ListGroups answer.
func principalPage(kind string, arnOf func(string) string, marker string, principals []Principal) Reply {
	var b strings.Builder
	fmt.Fprintf(&b, `<List%ssResponse xmlns="%s"><List%ssResult>`, kind, iamXMLNamespace, kind)
	writeMarker(&b, marker)
	fmt.Fprintf(&b, `<%ss>`, kind)
	for i, p := range principals {
		fmt.Fprintf(&b, `<member><Path>/</Path><%sName>%s</%sName><%sId>AIDAFAKE%06d</%sId><Arn>%s</Arn>`+
			`<CreateDate>2026-01-01T00:00:00Z</CreateDate></member>`,
			kind, xmlText(p.Name), kind, kind, i, kind, xmlText(arnOf(p.Name)))
	}
	fmt.Fprintf(&b, `</%ss></List%ssResult>%s</List%ssResponse>`, kind, kind, fakeRequestIDMetadata, kind)
	return Reply{Body: b.String()}
}

func writeMarker(b *strings.Builder, marker string) {
	fmt.Fprintf(b, `<IsTruncated>%t</IsTruncated>`, marker != "")
	if marker != "" {
		fmt.Fprintf(b, `<Marker>%s</Marker>`, xmlText(marker))
	}
}

// inlineNamesPage is one iam:ListUserPolicies, ListRolePolicies or
// ListGroupPolicies answer.
func inlineNamesPage(kind, marker string, names []string) Reply {
	var b strings.Builder
	fmt.Fprintf(&b, `<List%sPoliciesResponse xmlns="%s"><List%sPoliciesResult>`, kind, iamXMLNamespace, kind)
	writeMarker(&b, marker)
	b.WriteString(`<PolicyNames>`)
	for _, n := range names {
		fmt.Fprintf(&b, `<member>%s</member>`, xmlText(n))
	}
	fmt.Fprintf(&b, `</PolicyNames></List%sPoliciesResult>%s</List%sPoliciesResponse>`, kind, fakeRequestIDMetadata, kind)
	return Reply{Body: b.String()}
}

// InlinePolicy is one iam:GetUserPolicy, GetRolePolicy or GetGroupPolicy
// answer; the document is URL-encoded like IAM does.
func InlinePolicy(kind, principal, name, doc string) Reply {
	return Reply{Body: fmt.Sprintf(`<Get%sPolicyResponse xmlns="%s"><Get%sPolicyResult><%sName>%s</%sName>`+
		`<PolicyName>%s</PolicyName><PolicyDocument>%s</PolicyDocument></Get%sPolicyResult>%s</Get%sPolicyResponse>`,
		kind, iamXMLNamespace, kind, kind, xmlText(principal), kind, xmlText(name),
		xmlText(EncodePolicyDocument(doc)), kind, fakeRequestIDMetadata, kind)}
}

// --- S3 ---

func (f *FakeAWS) handleBuckets(x *accountIndex) {
	buckets := x.a.Buckets
	bucketTarget := func(c Call) string { return BucketARN(c.Bucket) }
	f.Handle(OpListBuckets, x.guard(OpListBuckets, func(c Call) Reply {
		start, end, next, bad := pageOf(c, OpListBuckets, len(buckets), c.Query.Get("max-buckets"), c.Query.Get("continuation-token"))
		if bad != nil {
			return *bad
		}
		return BucketPage(next, buckets[start:end]...)
	}, nil))
	f.Handle(OpGetBucketLocation, x.guard(OpGetBucketLocation, func(c Call) Reply {
		b, ok := x.buckets[c.Bucket]
		if !ok {
			return S3Error(http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		}
		return BucketLocation(b.Region)
	}, bucketTarget))
	f.Handle(OpGetBucketPolicy, x.guard(OpGetBucketPolicy, func(c Call) Reply {
		b, ok := x.buckets[c.Bucket]
		switch {
		case !ok:
			return S3Error(http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		case b.Region != c.Region:
			return S3Error(http.StatusMovedPermanently, "PermanentRedirect", "The bucket is in another region")
		case b.Policy == "":
			return S3Error(http.StatusNotFound, "NoSuchBucketPolicy", "The bucket policy does not exist")
		}
		return Reply{Body: b.Policy, ContentType: "application/json"}
	}, bucketTarget))
}

// BucketPage is one s3:ListBuckets answer; a non-empty token means more
// pages follow.
func BucketPage(token string, buckets ...BucketPolicy) Reply {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult xmlns="%s"><Owner><ID>fake-owner</ID></Owner><Buckets>`, s3XMLNamespace)
	for _, bucket := range buckets {
		fmt.Fprintf(&b, `<Bucket><Name>%s</Name><CreationDate>2026-01-01T00:00:00.000Z</CreationDate>`, xmlText(bucket.Name))
		if !bucket.NoRegionInList {
			fmt.Fprintf(&b, `<BucketRegion>%s</BucketRegion>`, xmlText(bucket.Region))
		}
		b.WriteString(`</Bucket>`)
	}
	b.WriteString(`</Buckets>`)
	if token != "" {
		fmt.Fprintf(&b, `<ContinuationToken>%s</ContinuationToken>`, xmlText(token))
	}
	b.WriteString(`</ListAllMyBucketsResult>`)
	return Reply{Body: b.String(), ContentType: "application/xml"}
}

// BucketLocation is the s3:GetBucketLocation answer; us-east-1 is an empty
// location constraint, like S3 answers.
func BucketLocation(region string) Reply {
	if region == "us-east-1" {
		region = ""
	}
	return Reply{Body: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="%s">%s</LocationConstraint>`,
		s3XMLNamespace, xmlText(region)), ContentType: "application/xml"}
}

// S3Error is an error answer of S3 (REST-XML).
func S3Error(status int, code, message string) Reply {
	return Reply{Status: status, ContentType: "application/xml", Body: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<Error><Code>%s</Code><Message>%s</Message><RequestId>fake-request-id</RequestId></Error>`, xmlText(code), xmlText(message))}
}

// --- KMS ---

func (f *FakeAWS) handleKeys(x *accountIndex) {
	byRegion := map[string][]KeyPolicy{}
	for _, k := range x.a.Keys {
		byRegion[k.Region] = append(byRegion[k.Region], k)
	}
	keyTarget := func(c Call) string {
		if k, ok := x.keys[jsonString(c.JSON["KeyId"])]; ok {
			return KeyARN(k.Region, k.ID)
		}
		return jsonString(c.JSON["KeyId"])
	}
	f.Handle(OpListKeys, x.guard(OpListKeys, func(c Call) Reply {
		keys := byRegion[c.Region]
		start, end, next, bad := pageOf(c, OpListKeys, len(keys), jsonNumber(c.JSON["Limit"]), jsonString(c.JSON["Marker"]))
		if bad != nil {
			return *bad
		}
		entries := make([]map[string]string, 0, end-start)
		for _, k := range keys[start:end] {
			entries = append(entries, map[string]string{"KeyId": k.ID, "KeyArn": KeyARN(k.Region, k.ID)})
		}
		page := map[string]any{"Keys": entries, "Truncated": next != ""}
		if next != "" {
			page["NextMarker"] = next
		}
		return jsonReply(page)
	}, nil))
	find := func(c Call) (KeyPolicy, *Reply) {
		k, ok := x.keys[jsonString(c.JSON["KeyId"])]
		var r Reply
		switch {
		case !ok:
			r = JSONError(http.StatusBadRequest, "NotFoundException", "Key does not exist")
		case k.Region != c.Region:
			r = wrongRegion(c)
		default:
			return k, nil
		}
		return k, &r
	}
	f.Handle(OpDescribeKey, x.guard(OpDescribeKey, func(c Call) Reply {
		k, bad := find(c)
		if bad != nil {
			return *bad
		}
		manager := "CUSTOMER"
		if k.AWSManaged {
			manager = "AWS"
		}
		return jsonReply(map[string]any{"KeyMetadata": map[string]any{
			"KeyId": k.ID, "Arn": KeyARN(k.Region, k.ID), "KeyManager": manager, "KeyState": "Enabled", "AWSAccountId": TestAccount,
		}})
	}, keyTarget))
	f.Handle(OpGetKeyPolicy, x.guard(OpGetKeyPolicy, func(c Call) Reply {
		k, bad := find(c)
		if bad != nil {
			return *bad
		}
		if name := jsonString(c.JSON["PolicyName"]); name != "" && name != "default" {
			return JSONError(http.StatusBadRequest, "NotFoundException", "no such policy name")
		}
		return jsonReply(map[string]any{"Policy": k.Policy, "PolicyName": "default"})
	}, keyTarget))
}

// --- SQS ---

func (f *FakeAWS) handleQueues(x *accountIndex) {
	byRegion := regionalByRegion(x.a.Queues)
	f.Handle(OpListQueues, x.guard(OpListQueues, func(c Call) Reply {
		queues := byRegion[c.Region]
		start, end, next, bad := pageOf(c, OpListQueues, len(queues), jsonNumber(c.JSON["MaxResults"]), jsonString(c.JSON["NextToken"]))
		if bad != nil {
			return *bad
		}
		urls := make([]string, 0, end-start)
		for _, q := range queues[start:end] {
			urls = append(urls, QueueURL(q.Region, q.Name))
		}
		page := map[string]any{"QueueUrls": urls}
		if next != "" {
			page["NextToken"] = next
		}
		return Reply{Body: mustJSON(page), ContentType: sqsJSONContentType}
	}, nil))
	f.Handle(OpGetQueueAttributes, x.guard(OpGetQueueAttributes, func(c Call) Reply {
		q, ok := x.queues[jsonString(c.JSON["QueueUrl"])]
		switch {
		case !ok:
			return SQSError(http.StatusBadRequest, "QueueDoesNotExist", "AWS.SimpleQueueService.NonExistentQueue", "The specified queue does not exist.")
		case q.Region != c.Region:
			return wrongRegion(c)
		}
		attrs := map[string]string{"QueueArn": QueueARN(q.Region, q.Name)}
		if q.Policy != "" {
			attrs["Policy"] = q.Policy
		}
		return Reply{Body: mustJSON(map[string]any{"Attributes": attrs}), ContentType: sqsJSONContentType}
	}, func(c Call) string {
		if q, ok := x.queues[jsonString(c.JSON["QueueUrl"])]; ok {
			return QueueARN(q.Region, q.Name)
		}
		return jsonString(c.JSON["QueueUrl"])
	}))
}

// SQSError is an error answer of SQS (JSON 1.0 with the query error code in
// the X-Amzn-Query-Error header, as SQS answers).
func SQSError(status int, code, queryCode, message string) Reply {
	return Reply{Status: status, ContentType: sqsJSONContentType,
		Body:   mustJSON(map[string]string{"__type": "com.amazonaws.sqs#" + code, "message": message}),
		Header: map[string]string{"X-Amzn-Query-Error": queryCode + ";Sender"}}
}

func regionalByRegion(items []RegionalPolicy) map[string][]RegionalPolicy {
	out := map[string][]RegionalPolicy{}
	for _, it := range items {
		out[it.Region] = append(out[it.Region], it)
	}
	return out
}

// --- SNS ---

func (f *FakeAWS) handleTopics(x *accountIndex) {
	byRegion := regionalByRegion(x.a.Topics)
	f.Handle(OpListTopics, x.guard(OpListTopics, func(c Call) Reply {
		topics := byRegion[c.Region]
		start, end, next, bad := pageOf(c, OpListTopics, len(topics), "", c.Form.Get("NextToken"))
		if bad != nil {
			return *bad
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<ListTopicsResponse xmlns="%s"><ListTopicsResult><Topics>`, snsXMLNamespace)
		for _, t := range topics[start:end] {
			fmt.Fprintf(&b, `<member><TopicArn>%s</TopicArn></member>`, xmlText(TopicARN(t.Region, t.Name)))
		}
		b.WriteString(`</Topics>`)
		if next != "" {
			fmt.Fprintf(&b, `<NextToken>%s</NextToken>`, xmlText(next))
		}
		fmt.Fprintf(&b, `</ListTopicsResult>%s</ListTopicsResponse>`, fakeRequestIDMetadata)
		return Reply{Body: b.String()}
	}, nil))
	f.Handle(OpGetTopicAttributes, x.guard(OpGetTopicAttributes, func(c Call) Reply {
		t, ok := x.topics[c.Form.Get("TopicArn")]
		switch {
		case !ok:
			return QueryError(http.StatusNotFound, "NotFound", "Topic does not exist")
		case t.Region != c.Region:
			return wrongRegion(c)
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<GetTopicAttributesResponse xmlns="%s"><GetTopicAttributesResult><Attributes>`, snsXMLNamespace)
		fmt.Fprintf(&b, `<entry><key>TopicArn</key><value>%s</value></entry>`, xmlText(TopicARN(t.Region, t.Name)))
		if t.Policy != "" {
			fmt.Fprintf(&b, `<entry><key>Policy</key><value>%s</value></entry>`, xmlText(t.Policy))
		}
		fmt.Fprintf(&b, `</Attributes></GetTopicAttributesResult>%s</GetTopicAttributesResponse>`, fakeRequestIDMetadata)
		return Reply{Body: b.String()}
	}, func(c Call) string { return c.Form.Get("TopicArn") }))
}

// --- Secrets Manager ---

func (f *FakeAWS) handleSecrets(x *accountIndex) {
	byRegion := regionalByRegion(x.a.Secrets)
	f.Handle(OpListSecrets, x.guard(OpListSecrets, func(c Call) Reply {
		secrets := byRegion[c.Region]
		start, end, next, bad := pageOf(c, OpListSecrets, len(secrets), jsonNumber(c.JSON["MaxResults"]), jsonString(c.JSON["NextToken"]))
		if bad != nil {
			return *bad
		}
		list := make([]map[string]string, 0, end-start)
		for _, s := range secrets[start:end] {
			list = append(list, map[string]string{"ARN": SecretARN(s.Region, s.Name), "Name": s.Name})
		}
		page := map[string]any{"SecretList": list}
		if next != "" {
			page["NextToken"] = next
		}
		return jsonReply(page)
	}, nil))
	f.Handle(OpGetResourcePolicy, x.guard(OpGetResourcePolicy, func(c Call) Reply {
		s, ok := x.secrets[jsonString(c.JSON["SecretId"])]
		switch {
		case !ok:
			return JSONError(http.StatusBadRequest, "ResourceNotFoundException", "Secrets Manager can't find the specified secret.")
		case s.Region != c.Region:
			return wrongRegion(c)
		}
		answer := map[string]any{"ARN": SecretARN(s.Region, s.Name), "Name": s.Name}
		if s.Policy != "" {
			answer["ResourcePolicy"] = s.Policy
		}
		return jsonReply(answer)
	}, func(c Call) string {
		if s, ok := x.secrets[jsonString(c.JSON["SecretId"])]; ok {
			return SecretARN(s.Region, s.Name)
		}
		return jsonString(c.JSON["SecretId"])
	}))
}
