package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func restRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://resource-explorer-2.us-east-1.amazonaws.com"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", signedHeader("resource-explorer-2", "us-east-1"))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// NFR3.4: Resource Explorer names the operation in the URL path, AWS Config
// in X-Amz-Target; both are recorded under their IAM-style service prefix.
func TestFakeAWSNamesRESTJSONCallsFromThePath(t *testing.T) {
	fake := NewFakeAWS(t)
	fake.Handle(OpListResources, ExplorerPages())
	fake.Handle(OpGetDefaultView, Static(DefaultView(ViewARN("us-east-1"))))
	fake.Handle(OpSelectResourceConfig, ConfigPages())

	if _, err := fake.RoundTrip(restRequest(t, "/ListResources", `{"MaxResults":500,"Filters":{"FilterString":"tag:none"}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.RoundTrip(restRequest(t, "/GetDefaultView", "")); err != nil {
		t.Fatal(err)
	}
	cfg, _ := http.NewRequest(http.MethodPost, "https://config.us-east-1.amazonaws.com/", strings.NewReader(`{"Expression":"SELECT arn","Limit":100}`))
	cfg.Header.Set("Authorization", signedHeader("config", "us-east-1"))
	cfg.Header.Set("X-Amz-Target", "StarlingDoveService.SelectResourceConfig")
	if _, err := fake.RoundTrip(cfg); err != nil {
		t.Fatal(err)
	}

	calls := fake.Calls()
	if len(calls) != 3 || calls[0].Op != OpListResources || calls[1].Op != OpGetDefaultView || calls[2].Op != OpSelectResourceConfig {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[0].JSON["MaxResults"] != float64(500) || calls[0].Path != "/ListResources" {
		t.Fatalf("REST-JSON body or path not parsed: %+v", calls[0])
	}
}

func TestIsReadOperationAllowsOnlySelectResourceConfigByName(t *testing.T) {
	for op, want := range map[string]bool{
		"config:SelectResourceConfig": true, "config:DescribeConfigurationRecorders": true,
		"resource-explorer-2:ListResources": true, "resource-explorer-2:Search": false,
		"config:SelectAggregateResourceConfig": false, "config:StartConfigurationRecorder": false,
	} {
		if got := IsReadOperation(op); got != want {
			t.Errorf("IsReadOperation(%q) = %v, want %v", op, got, want)
		}
	}
}

// NFR1.10: the fake reproduces AWS, which returns no NextToken when
// MaxResults is 1000.
func TestExplorerPagesFollowMaxResultsAndStopAt1000(t *testing.T) {
	var resources []ExplorerResource
	for i := 0; i < 2500; i++ {
		resources = append(resources, ExplorerResource{ARN: "arn:aws:sqs:us-east-1:123456789012:q" + strings.Repeat("x", i%3), ResourceType: "sqs:queue", Region: "us-east-1"})
	}
	h := ExplorerPages(resources...)
	read := func(size int) int {
		total, token := 0, ""
		for range 100 {
			body := map[string]any{"MaxResults": float64(size)}
			if token != "" {
				body["NextToken"] = token
			}
			var page struct {
				Resources []any
				NextToken string
			}
			if err := json.Unmarshal([]byte(h(Call{JSON: body}).Body), &page); err != nil {
				t.Fatal(err)
			}
			total += len(page.Resources)
			if token = page.NextToken; token == "" {
				return total
			}
		}
		t.Fatal("too many pages")
		return 0
	}
	if got := read(500); got != 2500 {
		t.Fatalf("pages of 500 read %d resources, want 2500", got)
	}
	if got := read(1000); got != 1000 {
		t.Fatalf("MaxResults 1000 read %d resources, want the AWS cut-off at 1000", got)
	}
	if bad := h(Call{JSON: map[string]any{"NextToken": "nope"}}); bad.Status != http.StatusBadRequest {
		t.Fatalf("bad token accepted: %+v", bad)
	}
}

func TestConfigAnswersAndPages(t *testing.T) {
	rec := Recorder{AllSupported: true, Strategy: StrategyAll, Recording: true}
	if body := ConfigRecorders(rec).Body; !strings.Contains(body, `"allSupported":true`) || !strings.Contains(body, `"useOnly":"ALL_SUPPORTED_RESOURCE_TYPES"`) {
		t.Fatalf("recorders: %s", body)
	}
	if body := ConfigRecorderStatus(rec).Body; !strings.Contains(body, `"recording":true`) {
		t.Fatalf("status: %s", body)
	}
	var items []ConfigItem
	for i := 0; i < 150; i++ {
		items = append(items, ConfigItem{ARN: "arn:aws:sqs:us-east-1:123456789012:q", ResourceType: "AWS::SQS::Queue", Tags: map[string]string{"b": "2", "a": "1"}})
	}
	h := ConfigPages(items...)
	first := h(Call{JSON: map[string]any{"Limit": float64(100)}})
	var page struct {
		Results   []string
		NextToken string
	}
	if err := json.Unmarshal([]byte(first.Body), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 100 || page.NextToken != "cfg-100" || !strings.Contains(page.Results[0], `"tags":[{"key":"a","value":"1"},{"key":"b","value":"2"}]`) {
		t.Fatalf("first page: %d results, token %q, %s", len(page.Results), page.NextToken, page.Results[0])
	}
	second := h(Call{JSON: map[string]any{"Limit": float64(100), "NextToken": "cfg-100"}})
	if !strings.Contains(second.Body, `"Results":[`) || strings.Contains(second.Body, "NextToken") {
		t.Fatalf("second page: %s", second.Body)
	}
}

// NFR5.8: the fake counts the requests it is still answering.
func TestFakeAWSCountsRequestsInFlightAndAddsRegionDelays(t *testing.T) {
	fake := NewFakeAWS(t)
	release := make(chan struct{})
	fake.Handle(OpGetDefaultView, func(Call) Reply { <-release; return DefaultView("") })
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = fake.RoundTrip(restRequest(t, "/GetDefaultView", ""))
	}()
	deadline := time.Now().Add(5 * time.Second)
	for fake.InFlight() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if fake.InFlight() != 1 {
		t.Fatalf("in flight = %d, want 1", fake.InFlight())
	}
	close(release)
	wg.Wait()
	if fake.InFlight() != 0 {
		t.Fatalf("in flight = %d after the answer, want 0", fake.InFlight())
	}

	d1, d2 := RegionDelays(1, time.Millisecond), RegionDelays(1, time.Millisecond)
	if d1(Call{Region: "eu-west-1"}) != d2(Call{Region: "eu-west-1"}) {
		t.Fatal("the same seed must give the same delay")
	}
	differs := false
	for seed := int64(2); seed < 12; seed++ {
		if RegionDelays(seed, time.Millisecond)(Call{Region: "eu-west-1"}) != d1(Call{Region: "eu-west-1"}) {
			differs = true
		}
	}
	if !differs {
		t.Fatal("different seeds never changed the delay")
	}
}

func TestPerRegionAndTagPagesByRegion(t *testing.T) {
	h := TagPagesByRegion(map[string][]TaggedResource{"eu-west-1": {{ARN: "arn:eu"}}})
	if body := h(Call{Region: "eu-west-1", JSON: map[string]any{}}).Body; !strings.Contains(body, "arn:eu") {
		t.Fatalf("eu-west-1: %s", body)
	}
	if body := h(Call{Region: "us-east-1", JSON: map[string]any{}}).Body; strings.Contains(body, "arn:eu") {
		t.Fatalf("us-east-1 got another region's resources: %s", body)
	}
	if r := RestJSONError(401, "UnauthorizedException", "no view"); r.Status != 401 || !strings.Contains(r.Body, `"__type":"UnauthorizedException"`) {
		t.Fatalf("REST-JSON error: %+v", r)
	}
	if body := View(ViewARN("us-east-1"), "service:ec2").Body; !strings.Contains(body, `"FilterString":"service:ec2"`) {
		t.Fatalf("view: %s", body)
	}
	if body := ExplorerIndexes(Index{Region: "us-east-1", Aggregator: true}).Body; !strings.Contains(body, `"Type":"AGGREGATOR"`) {
		t.Fatalf("indexes: %s", body)
	}
}
