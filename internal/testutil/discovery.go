package testutil

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Answers of AWS Config (JSON 1.1) and Resource Explorer (REST-JSON) for the
// discovery of never-tagged resources (U2).

const restJSONContentType = "application/json"

// Operation names of the discovery calls, as the fake AWS records them.
const (
	OpDescribeConfigurationRecorders      = "config:DescribeConfigurationRecorders"
	OpDescribeConfigurationRecorderStatus = "config:DescribeConfigurationRecorderStatus"
	OpSelectResourceConfig                = "config:SelectResourceConfig"
	OpListIndexes                         = "resource-explorer-2:ListIndexes"
	OpGetDefaultView                      = "resource-explorer-2:GetDefaultView"
	OpGetView                             = "resource-explorer-2:GetView"
	OpListResources                       = "resource-explorer-2:ListResources"
)

// HandleNoDiscovery registers the answers of an account without AWS Config
// recording and without Resource Explorer: discovery of never-tagged
// resources is unavailable, which changes no exit code. HandleDefaults
// registers it so that tests that do not care about discovery keep working.
func (f *FakeAWS) HandleNoDiscovery() {
	f.Handle(OpDescribeConfigurationRecorders, Static(ConfigRecorders()))
	f.Handle(OpListIndexes, Static(ExplorerIndexes()))
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // built from plain values only
	}
	return string(b)
}

func jsonReply(v any) Reply {
	return Reply{Body: mustJSON(v), ContentType: jsonContentType}
}

func restReply(v any) Reply {
	return Reply{Body: mustJSON(v), ContentType: restJSONContentType}
}

// Recorder describes one AWS Config configuration recorder and its status.
type Recorder struct {
	Name string
	// AllSupported is recordingGroup.allSupported.
	AllSupported bool
	// IncludeGlobal is recordingGroup.includeGlobalResourceTypes.
	IncludeGlobal bool
	// Strategy is recordingGroup.recordingStrategy.useOnly; empty leaves the
	// field out, as older recorders do.
	Strategy string
	// Types is recordingGroup.resourceTypes (INCLUSION_BY_RESOURCE_TYPES).
	Types []string
	// Excluded is recordingGroup.exclusionByResourceTypes.resourceTypes.
	Excluded []string
	// Recording is the recording flag of the recorder's status.
	Recording bool
}

func (r Recorder) name() string {
	if r.Name == "" {
		return "default"
	}
	return r.Name
}

// Recording strategies of recordingGroup.recordingStrategy.useOnly.
const (
	StrategyAll       = "ALL_SUPPORTED_RESOURCE_TYPES"
	StrategyInclusion = "INCLUSION_BY_RESOURCE_TYPES"
	StrategyExclusion = "EXCLUSION_BY_RESOURCE_TYPES"
)

// ConfigRecorders is the config:DescribeConfigurationRecorders answer.
// Without recorders it is the answer of an account that never set up Config.
func ConfigRecorders(recorders ...Recorder) Reply {
	list := []map[string]any{}
	for _, r := range recorders {
		group := map[string]any{
			"allSupported":               r.AllSupported,
			"includeGlobalResourceTypes": r.IncludeGlobal,
			"resourceTypes":              nonNil(r.Types),
			"exclusionByResourceTypes":   map[string]any{"resourceTypes": nonNil(r.Excluded)},
		}
		if r.Strategy != "" {
			group["recordingStrategy"] = map[string]any{"useOnly": r.Strategy}
		}
		list = append(list, map[string]any{
			"name":           r.name(),
			"roleARN":        "arn:aws:iam::" + TestAccount + ":role/config",
			"recordingGroup": group,
		})
	}
	return jsonReply(map[string]any{"ConfigurationRecorders": list})
}

// ConfigRecorderStatus is the config:DescribeConfigurationRecorderStatus
// answer for the recorders.
func ConfigRecorderStatus(recorders ...Recorder) Reply {
	list := []map[string]any{}
	for _, r := range recorders {
		list = append(list, map[string]any{"name": r.name(), "recording": r.Recording, "lastStatus": "SUCCESS"})
	}
	return jsonReply(map[string]any{"ConfigurationRecordersStatus": list})
}

// HandleConfigRecorders registers both recorder answers for every region.
func (f *FakeAWS) HandleConfigRecorders(recorders ...Recorder) {
	f.Handle(OpDescribeConfigurationRecorders, Static(ConfigRecorders(recorders...)))
	f.Handle(OpDescribeConfigurationRecorderStatus, Static(ConfigRecorderStatus(recorders...)))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ConfigItem is one result of config:SelectResourceConfig.
type ConfigItem struct {
	ARN          string
	ResourceType string // AWS Config form, for example AWS::EC2::Instance
	ResourceID   string
	Region       string
	Tags         map[string]string
}

// JSON returns the item as AWS Config returns one result: a JSON document in
// a string.
func (c ConfigItem) JSON() string {
	keys := make([]string, 0, len(c.Tags))
	for k := range c.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tags := []map[string]string{}
	for _, k := range keys {
		tags = append(tags, map[string]string{"key": k, "value": c.Tags[k]})
	}
	item := map[string]any{"resourceType": c.ResourceType, "resourceId": c.ResourceID, "awsRegion": c.Region, "tags": tags}
	if c.ARN != "" {
		item["arn"] = c.ARN
	}
	return mustJSON(item)
}

// ConfigResults is one config:SelectResourceConfig answer with raw result
// strings. An empty token marks the last page.
func ConfigResults(token string, results ...string) Reply {
	body := map[string]any{
		"Results":   nonNil(results),
		"QueryInfo": map[string]any{"SelectFields": []map[string]string{{"Name": "arn"}, {"Name": "resourceType"}}},
	}
	if token != "" {
		body["NextToken"] = token
	}
	return jsonReply(body)
}

// ConfigPages returns a config:SelectResourceConfig handler that serves the
// items in pages of the requested Limit (default 100), following NextToken.
func ConfigPages(items ...ConfigItem) Handler {
	return func(c Call) Reply {
		size := intParam(c.JSON["Limit"], 100)
		start := 0
		if tok, _ := c.JSON["NextToken"].(string); tok != "" {
			n, err := strconv.Atoi(strings.TrimPrefix(tok, "cfg-"))
			if err != nil || n <= 0 || n >= len(items) {
				return JSONError(http.StatusBadRequest, "InvalidNextTokenException", "bad token")
			}
			start = n
		}
		end := min(start+size, len(items))
		var results []string
		for _, it := range items[start:end] {
			results = append(results, it.JSON())
		}
		next := ""
		if end < len(items) {
			next = "cfg-" + strconv.Itoa(end)
		}
		return ConfigResults(next, results...)
	}
}

func intParam(v any, fallback int) int {
	if f, ok := v.(float64); ok && f > 0 {
		return int(f)
	}
	return fallback
}

// Index is one Resource Explorer index.
type Index struct {
	Region     string
	Aggregator bool
}

// ExplorerIndexes is the resource-explorer-2:ListIndexes answer (one page).
func ExplorerIndexes(indexes ...Index) Reply {
	return ExplorerIndexPage("", indexes...)
}

// ExplorerIndexPage is one resource-explorer-2:ListIndexes page; a non-empty
// token means more pages follow.
func ExplorerIndexPage(token string, indexes ...Index) Reply {
	list := []map[string]string{}
	for _, ix := range indexes {
		typ := "LOCAL"
		if ix.Aggregator {
			typ = "AGGREGATOR"
		}
		list = append(list, map[string]string{
			"Region": ix.Region,
			"Arn":    "arn:aws:resource-explorer-2:" + ix.Region + ":" + TestAccount + ":index/fake-" + ix.Region,
			"Type":   typ,
		})
	}
	body := map[string]any{"Indexes": list}
	if token != "" {
		body["NextToken"] = token
	}
	return restReply(body)
}

// ViewARN is the ARN of the fake default view of a region.
func ViewARN(region string) string {
	return "arn:aws:resource-explorer-2:" + region + ":" + TestAccount + ":view/default/fake-" + region
}

// DefaultView is the resource-explorer-2:GetDefaultView answer; an empty ARN
// gives the answer of a region without a default view.
func DefaultView(viewARN string) Reply {
	if viewARN == "" {
		return restReply(map[string]any{})
	}
	return restReply(map[string]any{"ViewArn": viewARN})
}

// View is the resource-explorer-2:GetView answer with the view's filter.
func View(viewARN, filter string) Reply {
	view := map[string]any{"ViewArn": viewARN, "Owner": TestAccount, "Scope": "arn:aws:iam::" + TestAccount + ":root",
		"IncludedProperties": []any{}}
	if filter != "" {
		view["Filters"] = map[string]string{"FilterString": filter}
	}
	return restReply(map[string]any{"View": view, "Tags": map[string]string{}})
}

// HandleExplorerViews answers GetDefaultView and GetView for every region
// with an unfiltered default view.
func (f *FakeAWS) HandleExplorerViews() {
	f.Handle(OpGetDefaultView, func(c Call) Reply { return DefaultView(ViewARN(c.Region)) })
	f.Handle(OpGetView, func(c Call) Reply {
		arn, _ := c.JSON["ViewArn"].(string)
		return View(arn, "")
	})
}

// ExplorerResource is one result of resource-explorer-2:ListResources.
type ExplorerResource struct {
	ARN          string
	ResourceType string // ARN form, for example sqs:queue
	Region       string
}

// ExplorerPages returns a resource-explorer-2:ListResources handler that
// serves the resources in pages of the requested MaxResults, following
// NextToken. Like AWS, it never returns a NextToken when MaxResults is 1000,
// so a reader that asks for 1000 stops at the first 1000 resources.
func ExplorerPages(resources ...ExplorerResource) Handler {
	return func(c Call) Reply {
		size := intParam(c.JSON["MaxResults"], 100)
		start := 0
		if tok, _ := c.JSON["NextToken"].(string); tok != "" {
			n, err := strconv.Atoi(strings.TrimPrefix(tok, "re-"))
			if err != nil || n <= 0 || n >= len(resources) {
				return RestJSONError(http.StatusBadRequest, "ValidationException", "bad token")
			}
			start = n
		}
		end := min(start+size, len(resources))
		list := []map[string]any{}
		for _, r := range resources[start:end] {
			service := r.ResourceType
			if i := strings.Index(service, ":"); i >= 0 {
				service = service[:i]
			}
			list = append(list, map[string]any{"Arn": r.ARN, "ResourceType": r.ResourceType, "Region": r.Region,
				"Service": service, "OwningAccountId": TestAccount, "Properties": []any{}})
		}
		body := map[string]any{"Resources": list}
		if end < len(resources) && size != 1000 {
			body["NextToken"] = "re-" + strconv.Itoa(end)
		}
		if arn, ok := c.JSON["ViewArn"].(string); ok {
			body["ViewArn"] = arn
		}
		return restReply(body)
	}
}

// RestJSONError is an error answer of the REST-JSON protocol (Resource
// Explorer).
func RestJSONError(status int, errorType, message string) Reply {
	return Reply{Status: status, Body: mustJSON(map[string]string{"__type": errorType, "message": message}),
		ContentType: restJSONContentType}
}

// PerRegion returns a handler that answers each region with its own handler
// and every other region with other.
func PerRegion(handlers map[string]Handler, other Handler) Handler {
	return func(c Call) Reply {
		if h, ok := handlers[c.Region]; ok {
			return h(c)
		}
		return other(c)
	}
}

// TagPagesByRegion returns a tag:GetResources handler that serves each
// region's resources (in pages of 100, as tag-refactor asks for) and an empty
// page for every other region.
func TagPagesByRegion(byRegion map[string][]TaggedResource) Handler {
	handlers := map[string]Handler{}
	for region, resources := range byRegion {
		var pages [][]TaggedResource
		for start := 0; start < len(resources); start += 100 {
			pages = append(pages, resources[start:min(start+100, len(resources))])
		}
		if len(pages) == 0 {
			pages = [][]TaggedResource{nil}
		}
		handlers[region] = TagPages(pages...)
	}
	return PerRegion(handlers, TagPages(nil))
}
