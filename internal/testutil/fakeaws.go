// Package testutil holds test helpers shared by the tag-refactor packages: an
// HTTP-level fake AWS, the isolation of the AWS environment, and a fake clock.
//
// The fake AWS sits below the AWS SDK: the real SDK configuration, signing,
// time limits, retries, pagination and error mapping all run, and only the
// HTTP round trip is answered by the fake. It never opens a network
// connection.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Call describes one HTTP request the fake AWS received.
type Call struct {
	// Op is the IAM-style operation name, for example "tag:GetResources".
	Op string
	// Service is the IAM action prefix, for example "tag" or "iam".
	Service string
	// Action is the API action, for example "GetResources".
	Action string
	// Region is the signing region taken from the request signature.
	Region string
	// AccessKeyID is the access key ID taken from the request signature.
	AccessKeyID string
	// Host is the request host.
	Host string
	// Path is the URL path of the request.
	Path string
	// Form holds the parameters of query-protocol requests (STS, IAM, EC2).
	Form url.Values
	// JSON holds the body of JSON-protocol requests (tagging API, AWS Config)
	// and of REST-JSON requests (Resource Explorer).
	JSON map[string]any
	// N counts the calls of the same Op so far, starting at 1.
	N int
}

// Reply is the fake AWS answer to one call.
type Reply struct {
	// Status is the HTTP status code. Zero means 200.
	Status int
	// Body is the response body.
	Body string
	// ContentType is the response content type. Empty means text/xml.
	ContentType string
	// Delay makes the fake wait before answering. The wait ends early when the
	// request context is done, which is how the SDK's time limits are tested.
	Delay time.Duration
	// Err, when set, is returned as a transport error instead of a response.
	Err error
}

// Handler answers the calls of one operation.
type Handler func(c Call) Reply

// FakeAWS is an http.RoundTripper that answers AWS API calls from registered
// handlers and records every call. Calls to operations that are not reads
// (Get, List, Describe, or config:SelectResourceConfig) or that have no
// handler fail the test.
type FakeAWS struct {
	t        testing.TB
	mu       sync.Mutex
	handlers map[string]Handler
	calls    []Call
	counts   map[string]int
	delay    func(Call) time.Duration
	inFlight atomic.Int64
}

// NewFakeAWS returns a fake AWS that reports unexpected calls to t.
func NewFakeAWS(t testing.TB) *FakeAWS {
	return &FakeAWS{t: t, handlers: map[string]Handler{}, counts: map[string]int{}}
}

// Handle registers the handler of op, for example "iam:ListPolicies".
func (f *FakeAWS) Handle(op string, h Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[op] = h
}

// Calls returns a copy of the calls received so far.
func (f *FakeAWS) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Ops returns the distinct operation names received so far, sorted.
func (f *FakeAWS) Ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ops := make([]string, 0, len(f.counts))
	for op := range f.counts {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	return ops
}

// Count returns how many calls of op were received.
func (f *FakeAWS) Count(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[op]
}

// InFlight returns the number of requests that have arrived and not yet been
// answered. A command that has returned must leave none behind (NFR5.8).
func (f *FakeAWS) InFlight() int {
	return int(f.inFlight.Load())
}

// SetDelay makes the fake wait for delay(call) before answering every call,
// in addition to the handler's own delay. Tests use it to shuffle the order
// in which regions finish (NFR5.7).
func (f *FakeAWS) SetDelay(delay func(Call) time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delay = delay
}

// RegionDelays returns a delay function that gives each region a fixed
// pseudo-random delay below max, chosen by seed. The same seed always gives
// the same delays; different seeds give different finishing orders.
func RegionDelays(seed int64, max time.Duration) func(Call) time.Duration {
	return func(c Call) time.Duration {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%d/%s", seed, c.Region)
		return time.Duration(h.Sum64() % uint64(max))
	}
}

// readOperationsByName are the operations that read but do not start with
// Get, List or Describe (security design §1).
var readOperationsByName = map[string]bool{"config:SelectResourceConfig": true}

// IsReadOperation reports whether the action of op is a read: it starts with
// Get, List or Describe, or it is config:SelectResourceConfig. The API
// resource-explorer-2:Search is not one of them; tag-refactor never calls it.
func IsReadOperation(op string) bool {
	if readOperationsByName[op] {
		return true
	}
	action := op[strings.Index(op, ":")+1:]
	for _, prefix := range []string{"Get", "List", "Describe"} {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

// RoundTrip implements http.RoundTripper.
func (f *FakeAWS) RoundTrip(req *http.Request) (*http.Response, error) {
	f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	call, err := parseCall(req)
	if err != nil {
		f.t.Errorf("fake AWS: cannot parse request to %s: %v", req.URL.Host, err)
		return newResponse(req, Reply{Status: http.StatusBadRequest, Body: "unparsable request"}), nil
	}
	f.mu.Lock()
	f.counts[call.Op]++
	call.N = f.counts[call.Op]
	f.calls = append(f.calls, call)
	h, ok := f.handlers[call.Op]
	delay := f.delay
	f.mu.Unlock()

	if !IsReadOperation(call.Op) {
		f.t.Errorf("fake AWS: non-read operation %s was called", call.Op)
		return newResponse(req, QueryError(http.StatusBadRequest, "FakeAWSRejectedWrite", "write operations are not allowed")), nil
	}
	if !ok {
		f.t.Errorf("fake AWS: no handler for operation %s", call.Op)
		return newResponse(req, QueryError(http.StatusBadRequest, "FakeAWSNoHandler", "no handler")), nil
	}
	reply := h(call)
	if delay != nil {
		reply.Delay += delay(call)
	}
	if reply.Delay > 0 {
		timer := time.NewTimer(reply.Delay)
		defer timer.Stop()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
	if reply.Err != nil {
		return nil, reply.Err
	}
	return newResponse(req, reply), nil
}

func newResponse(req *http.Request, r Reply) *http.Response {
	status := r.Status
	if status == 0 {
		status = http.StatusOK
	}
	contentType := r.ContentType
	if contentType == "" {
		contentType = "text/xml"
	}
	header := http.Header{}
	header.Set("Content-Type", contentType)
	header.Set("X-Amzn-Requestid", "fake-request-id")
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(r.Body)),
		ContentLength: int64(len(r.Body)),
		Request:       req,
	}
}

// signingServiceToIAMPrefix maps SigV4 signing names to IAM action prefixes.
var signingServiceToIAMPrefix = map[string]string{"tagging": "tag"}

func parseCall(req *http.Request) (Call, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return Call{}, err
		}
		body = b
	}
	accessKey, region, service, err := parseCredentialScope(req.Header.Get("Authorization"))
	if err != nil {
		return Call{}, err
	}
	if p, ok := signingServiceToIAMPrefix[service]; ok {
		service = p
	}
	c := Call{Service: service, Region: region, AccessKeyID: accessKey, Host: req.URL.Host, Path: req.URL.Path}
	switch {
	case req.Header.Get("X-Amz-Target") != "":
		// JSON 1.1 (tagging API, AWS Config): the operation is in the header.
		target := req.Header.Get("X-Amz-Target")
		c.Action = target[strings.LastIndex(target, ".")+1:]
		if c.JSON, err = parseJSONBody(body); err != nil {
			return Call{}, err
		}
	case isRESTJSON(req):
		// REST-JSON (Resource Explorer): the operation is the last part of
		// the URL path, for example POST /ListResources.
		c.Action = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		if c.JSON, err = parseJSONBody(body); err != nil {
			return Call{}, err
		}
	default:
		// Query protocol (STS, IAM, EC2): the operation is the Action parameter.
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return Call{}, fmt.Errorf("form body: %w", err)
		}
		c.Form = form
		c.Action = form.Get("Action")
	}
	if c.Action == "" {
		return Call{}, fmt.Errorf("no action in request")
	}
	c.Op = c.Service + ":" + c.Action
	return c, nil
}

// isRESTJSON reports whether a request without X-Amz-Target uses REST-JSON:
// such requests name the operation in the path, while the query protocol
// always posts to "/".
func isRESTJSON(req *http.Request) bool {
	return strings.Trim(req.URL.Path, "/") != ""
}

func parseJSONBody(body []byte) (map[string]any, error) {
	m := map[string]any{}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, fmt.Errorf("JSON body: %w", err)
		}
	}
	return m, nil
}

// parseCredentialScope reads "Credential=AKID/date/region/service/aws4_request"
// from a SigV4 Authorization header.
func parseCredentialScope(auth string) (accessKey, region, service string, err error) {
	const marker = "Credential="
	i := strings.Index(auth, marker)
	if i < 0 {
		return "", "", "", fmt.Errorf("request is not signed with SigV4")
	}
	scope := auth[i+len(marker):]
	if j := strings.IndexAny(scope, ", "); j >= 0 {
		scope = scope[:j]
	}
	parts := strings.Split(scope, "/")
	if len(parts) != 5 {
		return "", "", "", fmt.Errorf("unexpected credential scope %q", scope)
	}
	return parts[0], parts[2], parts[3], nil
}
