package testutil

import (
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// TagAccount is an in-memory account of the tagging API for the tests of
// apply (U6). It answers tag:GetResources, by page or by a list of ARNs,
// from the tags it holds, and changes them on tag:TagResources and
// tag:UntagResources, so a test can plan, apply and look at the tags
// afterwards. Like the real API it is regional, rejects keys starting with
// aws:, writes at most 20 resources per call, reads at most 100 ARNs per
// call, and does not return a resource that never had a tag.
type TagAccount struct {
	mu        sync.Mutex
	resources map[string]*accountResource
	failure   func(op, arn string) *ResourceFailure
	callReply func(c Call) *Reply
	onWrite   func(c Call)
}

type accountResource struct {
	region     string
	tags       map[string]string
	everTagged bool
}

// ResourceFailure is one entry of the FailedResourcesMap of a write answer.
type ResourceFailure struct {
	// StatusCode is the HTTP status of the failure; 0 leaves it out of the
	// answer.
	StatusCode int
	Code       string
	Message    string
}

// MaxUserTags is the number of tags, not counting those starting with aws:,
// that the fake lets a resource have (U6 BR3.6).
const MaxUserTags = 50

// OpTagResources and the other operation names of the tests of apply, as the
// fake AWS records them.
const (
	OpTagResources   = "tag:TagResources"
	OpUntagResources = "tag:UntagResources"
	OpGetResources   = "tag:GetResources"
	OpPutObject      = "s3:PutObject"
	OpGetObject      = "s3:GetObject"
	OpDeleteObject   = "s3:DeleteObject"
)

// NewTagAccount returns an empty account.
func NewTagAccount() *TagAccount {
	return &TagAccount{resources: map[string]*accountResource{}}
}

// Add puts a resource in region with the given tags; a resource without tags
// counts as one that had tags before.
func (a *TagAccount) Add(region, arn string, tags map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resources[arn] = &accountResource{region: region, tags: maps.Clone(nonNilMap(tags)), everTagged: true}
}

// AddNeverTagged puts a resource that never had a tag in region: the tagging
// API does not return it until it is tagged.
func (a *TagAccount) AddNeverTagged(region, arn string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resources[arn] = &accountResource{region: region, tags: map[string]string{}}
}

// SetTags replaces the tags of a resource, as a person changing them by hand
// would.
func (a *TagAccount) SetTags(arn string, tags map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r, ok := a.resources[arn]; ok {
		r.tags = maps.Clone(nonNilMap(tags))
		r.everTagged = true
	}
}

// Delete removes a resource.
func (a *TagAccount) Delete(arn string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.resources, arn)
}

// Tags returns a copy of the tags of a resource, nil when it does not exist.
func (a *TagAccount) Tags(arn string) map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r, ok := a.resources[arn]; ok {
		return maps.Clone(r.tags)
	}
	return nil
}

// FailWrites makes the writes fail for some resources: f returns the
// failure of the resource for the operation (tag:TagResources or
// tag:UntagResources), or nil to write it.
func (a *TagAccount) FailWrites(f func(op, arn string) *ResourceFailure) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failure = f
}

// ReplyInstead makes f decide the answer of a call: a non-nil reply is given
// instead of the account's answer, and nothing changes (an error of the whole
// call, a slow answer, a broken connection).
func (a *TagAccount) ReplyInstead(f func(c Call) *Reply) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.callReply = f
}

// OnWrite calls f at the start of every write call, before it changes
// anything. Tests use it to look at the run record before the first write or
// to stop a run while it writes.
func (a *TagAccount) OnWrite(f func(c Call)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onWrite = f
}

// HandleTagAccount makes the fake answer the tagging API from the account and
// allows its writes.
func (f *FakeAWS) HandleTagAccount(a *TagAccount) {
	f.AllowWrites()
	f.Handle(OpGetResources, a.getResources)
	f.Handle(OpTagResources, a.write)
	f.Handle(OpUntagResources, a.write)
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (a *TagAccount) replyInstead(c Call) *Reply {
	a.mu.Lock()
	f := a.callReply
	a.mu.Unlock()
	if f == nil {
		return nil
	}
	return f(c)
}

// visible reports whether GetResources returns the resource: it is in the
// region of the call and has, or had, a tag.
func (r *accountResource) visible(region string) bool {
	return r.region == region && (r.everTagged || len(r.tags) > 0)
}

func (a *TagAccount) getResources(c Call) Reply {
	if r := a.replyInstead(c); r != nil {
		return *r
	}
	if list, ok := c.JSON["ResourceARNList"]; ok {
		if _, paged := c.JSON["PaginationToken"]; paged {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", "ResourceARNList cannot be used with pagination")
		}
		if _, paged := c.JSON["ResourcesPerPage"]; paged {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", "ResourceARNList cannot be used with pagination")
		}
		arns := jsonStrings(list)
		if len(arns) == 0 || len(arns) > 100 {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", fmt.Sprintf("ResourceARNList has %d ARNs", len(arns)))
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		var found []TaggedResource
		for _, arn := range arns {
			if r, ok := a.resources[arn]; ok && r.visible(c.Region) {
				found = append(found, TaggedResource{ARN: arn, Tags: maps.Clone(r.tags)})
			}
		}
		return TagPage("", found...)
	}
	a.mu.Lock()
	var all []TaggedResource
	for arn, r := range a.resources {
		if r.visible(c.Region) {
			all = append(all, TaggedResource{ARN: arn, Tags: maps.Clone(r.tags)})
		}
	}
	a.mu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].ARN < all[j].ARN })
	start := 0
	if tok, _ := c.JSON["PaginationToken"].(string); tok != "" {
		n, err := strconv.Atoi(strings.TrimPrefix(tok, "page-"))
		if err != nil || n <= 0 || n > len(all) {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", "bad pagination token")
		}
		start = n
	}
	end := min(start+100, len(all))
	next := ""
	if end < len(all) {
		next = "page-" + strconv.Itoa(end)
	}
	return TagPage(next, all[start:end]...)
}

// jsonStrings reads a JSON array of strings.
func jsonStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func isAWSKey(k string) bool { return strings.HasPrefix(strings.ToLower(k), "aws:") }

func userTagCount(tags map[string]string) int {
	n := 0
	for k := range tags {
		if !isAWSKey(k) {
			n++
		}
	}
	return n
}

// write answers tag:TagResources and tag:UntagResources.
func (a *TagAccount) write(c Call) Reply {
	a.mu.Lock()
	hook := a.onWrite
	a.mu.Unlock()
	if hook != nil {
		hook(c)
	}
	if r := a.replyInstead(c); r != nil {
		return *r
	}
	arns := jsonStrings(c.JSON["ResourceARNList"])
	if len(arns) == 0 || len(arns) > 20 {
		return JSONError(http.StatusBadRequest, "InvalidParameterException", fmt.Sprintf("ResourceARNList has %d ARNs", len(arns)))
	}
	tags := map[string]string{}
	var keys []string
	if c.Op == OpTagResources {
		m, _ := c.JSON["Tags"].(map[string]any)
		for k, v := range m {
			s, _ := v.(string)
			tags[k] = s
			keys = append(keys, k)
		}
	} else {
		keys = jsonStrings(c.JSON["TagKeys"])
		if len(keys) > 50 {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", "too many keys")
		}
	}
	if len(keys) == 0 {
		return JSONError(http.StatusBadRequest, "InvalidParameterException", "no tags")
	}
	for _, k := range keys {
		if isAWSKey(k) {
			return JSONError(http.StatusBadRequest, "InvalidParameterException", "keys starting with aws: are reserved")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	failed := map[string]ResourceFailure{}
	for _, arn := range arns {
		if a.failure != nil {
			if f := a.failure(c.Op, arn); f != nil {
				failed[arn] = *f
				continue
			}
		}
		r, ok := a.resources[arn]
		if !ok || r.region != c.Region {
			failed[arn] = ResourceFailure{StatusCode: http.StatusNotFound, Code: "ResourceNotFoundException", Message: "not found"}
			continue
		}
		next := maps.Clone(r.tags)
		if c.Op == OpTagResources {
			maps.Copy(next, tags)
		} else {
			for _, k := range keys {
				delete(next, k)
			}
		}
		if userTagCount(next) > MaxUserTags {
			failed[arn] = ResourceFailure{StatusCode: http.StatusBadRequest, Code: "InvalidParameterException", Message: "too many tags"}
			continue
		}
		r.tags = next
		r.everTagged = true
	}
	return WriteAnswer(failed)
}

// WriteAnswer is the answer of tag:TagResources or tag:UntagResources with
// the given failures by ARN.
func WriteAnswer(failed map[string]ResourceFailure) Reply {
	type failureInfo struct {
		StatusCode   int    `json:"StatusCode,omitempty"`
		ErrorCode    string `json:"ErrorCode,omitempty"`
		ErrorMessage string `json:"ErrorMessage,omitempty"`
	}
	m := map[string]failureInfo{}
	for arn, f := range failed {
		m[arn] = failureInfo{StatusCode: f.StatusCode, ErrorCode: f.Code, ErrorMessage: f.Message}
	}
	return jsonReply(map[string]any{"FailedResourcesMap": m})
}
