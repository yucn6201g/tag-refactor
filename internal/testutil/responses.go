package testutil

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// TestAccount is the account ID the fake AWS reports by default.
const TestAccount = "123456789012"

// TestCallerARN is the caller ARN the fake AWS reports by default.
const TestCallerARN = "arn:aws:iam::123456789012:user/tester"

const jsonContentType = "application/x-amz-json-1.1"

func xmlText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// HandleDefaults registers the answers every connected command needs: the
// caller identity (account TestAccount) and the enabled regions.
func (f *FakeAWS) HandleDefaults(regions ...string) {
	f.Handle("sts:GetCallerIdentity", Static(CallerIdentity(TestAccount, TestCallerARN)))
	f.Handle("ec2:DescribeRegions", Static(Regions(regions...)))
}

// Static returns a handler that always gives r.
func Static(r Reply) Handler {
	return func(Call) Reply { return r }
}

// Sequence returns a handler that gives the replies in order; the last reply
// repeats once the list is used up.
func Sequence(replies ...Reply) Handler {
	return func(c Call) Reply {
		if c.N <= len(replies) {
			return replies[c.N-1]
		}
		return replies[len(replies)-1]
	}
}

// CallerIdentity is the sts:GetCallerIdentity answer.
func CallerIdentity(account, arn string) Reply {
	return Reply{Body: fmt.Sprintf(`<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">`+
		`<GetCallerIdentityResult><Arn>%s</Arn><UserId>AIDAFAKEUSERID</UserId><Account>%s</Account></GetCallerIdentityResult>`+
		`<ResponseMetadata><RequestId>fake-request-id</RequestId></ResponseMetadata></GetCallerIdentityResponse>`,
		xmlText(arn), xmlText(account))}
}

// Regions is the ec2:DescribeRegions answer listing the enabled regions.
func Regions(names ...string) Reply {
	var b strings.Builder
	b.WriteString(`<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>fake-request-id</requestId><regionInfo>`)
	for _, n := range names {
		fmt.Fprintf(&b, `<item><regionName>%s</regionName><regionEndpoint>ec2.%s.amazonaws.com</regionEndpoint><optInStatus>opt-in-not-required</optInStatus></item>`, xmlText(n), xmlText(n))
	}
	b.WriteString(`</regionInfo></DescribeRegionsResponse>`)
	return Reply{Body: b.String()}
}

// TaggedResource is one entry of a tag:GetResources page.
type TaggedResource struct {
	ARN  string
	Tags map[string]string
}

type jsonTag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type jsonMapping struct {
	ResourceARN string    `json:"ResourceARN"`
	Tags        []jsonTag `json:"Tags"`
}

// TagPage is one tag:GetResources answer. An empty token marks the last page.
func TagPage(token string, resources ...TaggedResource) Reply {
	page := struct {
		PaginationToken        string        `json:"PaginationToken"`
		ResourceTagMappingList []jsonMapping `json:"ResourceTagMappingList"`
	}{PaginationToken: token, ResourceTagMappingList: []jsonMapping{}}
	for _, r := range resources {
		m := jsonMapping{ResourceARN: r.ARN, Tags: []jsonTag{}}
		keys := make([]string, 0, len(r.Tags))
		for k := range r.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m.Tags = append(m.Tags, jsonTag{Key: k, Value: r.Tags[k]})
		}
		page.ResourceTagMappingList = append(page.ResourceTagMappingList, m)
	}
	body, err := json.Marshal(page)
	if err != nil {
		panic(err) // the page is built from strings only
	}
	return Reply{Body: string(body), ContentType: jsonContentType}
}

// TagPages returns a tag:GetResources handler that serves the pages in order,
// following the PaginationToken of each request. Pages may be empty.
func TagPages(pages ...[]TaggedResource) Handler {
	return func(c Call) Reply {
		index := 0
		if tok, _ := c.JSON["PaginationToken"].(string); tok != "" {
			n, err := strconv.Atoi(strings.TrimPrefix(tok, "page-"))
			if err != nil || n <= 0 || n >= len(pages) {
				return JSONError(http.StatusBadRequest, "InvalidParameterException", "bad pagination token")
			}
			index = n
		}
		next := ""
		if index+1 < len(pages) {
			next = "page-" + strconv.Itoa(index+1)
		}
		var page []TaggedResource
		if index < len(pages) {
			page = pages[index]
		}
		return TagPage(next, page...)
	}
}

// ManagedPolicy is one entry of an iam:ListPolicies page.
type ManagedPolicy struct {
	ARN            string
	DefaultVersion string
}

// PolicyPage is one iam:ListPolicies answer. A non-empty marker means more
// pages follow.
func PolicyPage(marker string, policies ...ManagedPolicy) Reply {
	var b strings.Builder
	b.WriteString(`<ListPoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><ListPoliciesResult>`)
	fmt.Fprintf(&b, `<IsTruncated>%t</IsTruncated>`, marker != "")
	if marker != "" {
		fmt.Fprintf(&b, `<Marker>%s</Marker>`, xmlText(marker))
	}
	b.WriteString(`<Policies>`)
	for i, p := range policies {
		version := p.DefaultVersion
		if version == "" {
			version = "v1"
		}
		name := p.ARN[strings.LastIndex(p.ARN, "/")+1:]
		fmt.Fprintf(&b, `<member><PolicyName>%s</PolicyName><PolicyId>ANPAFAKE%04d</PolicyId><Arn>%s</Arn><Path>/</Path>`+
			`<DefaultVersionId>%s</DefaultVersionId><AttachmentCount>1</AttachmentCount><IsAttachable>true</IsAttachable></member>`,
			xmlText(name), i, xmlText(p.ARN), xmlText(version))
	}
	b.WriteString(`</Policies></ListPoliciesResult><ResponseMetadata><RequestId>fake-request-id</RequestId></ResponseMetadata></ListPoliciesResponse>`)
	return Reply{Body: b.String()}
}

// PolicyPages returns an iam:ListPolicies handler that serves the pages in
// order, following the Marker of each request.
func PolicyPages(pages ...[]ManagedPolicy) Handler {
	return func(c Call) Reply {
		index := 0
		if m := c.Form.Get("Marker"); m != "" {
			n, err := strconv.Atoi(strings.TrimPrefix(m, "marker-"))
			if err != nil || n <= 0 || n >= len(pages) {
				return QueryError(http.StatusBadRequest, "InvalidInput", "bad marker")
			}
			index = n
		}
		marker := ""
		if index+1 < len(pages) {
			marker = "marker-" + strconv.Itoa(index+1)
		}
		var page []ManagedPolicy
		if index < len(pages) {
			page = pages[index]
		}
		return PolicyPage(marker, page...)
	}
}

// EncodePolicyDocument URL-encodes a policy document the way IAM returns it
// (RFC 3986 percent-encoding).
func EncodePolicyDocument(doc string) string {
	return strings.ReplaceAll(url.QueryEscape(doc), "+", "%20")
}

// PolicyVersion is one iam:GetPolicyVersion answer with the given document.
func PolicyVersion(doc string) Reply {
	return Reply{Body: fmt.Sprintf(`<GetPolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">`+
		`<GetPolicyVersionResult><PolicyVersion><Document>%s</Document><VersionId>v1</VersionId><IsDefaultVersion>true</IsDefaultVersion></PolicyVersion></GetPolicyVersionResult>`+
		`<ResponseMetadata><RequestId>fake-request-id</RequestId></ResponseMetadata></GetPolicyVersionResponse>`,
		xmlText(EncodePolicyDocument(doc)))}
}

// PolicyDocuments returns an iam:GetPolicyVersion handler that answers with
// the document of the requested policy ARN, or NoSuchEntity.
func PolicyDocuments(docs map[string]string) Handler {
	return func(c Call) Reply {
		doc, ok := docs[c.Form.Get("PolicyArn")]
		if !ok {
			return QueryError(http.StatusNotFound, "NoSuchEntity", "policy not found")
		}
		return PolicyVersion(doc)
	}
}

// QueryError is an error answer of the query protocol (STS, IAM).
func QueryError(status int, code, message string) Reply {
	return Reply{Status: status, Body: fmt.Sprintf(`<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">`+
		`<Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>fake-request-id</RequestId></ErrorResponse>`,
		xmlText(code), xmlText(message))}
}

// EC2Error is an error answer of the EC2 query protocol.
func EC2Error(status int, code, message string) Reply {
	return Reply{Status: status, Body: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Response><Errors><Error>`+
		`<Code>%s</Code><Message>%s</Message></Error></Errors><RequestID>fake-request-id</RequestID></Response>`,
		xmlText(code), xmlText(message))}
}

// JSONError is an error answer of the JSON protocol (tagging API).
func JSONError(status int, errorType, message string) Reply {
	body, err := json.Marshal(map[string]string{"__type": errorType, "message": message})
	if err != nil {
		panic(err) // built from strings only
	}
	return Reply{Status: status, Body: string(body), ContentType: jsonContentType}
}
