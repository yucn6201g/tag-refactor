package inventory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// memoryConfig replaces the ConfigReader with in-memory data per region.
type memoryConfig struct {
	recorders map[string][]awsaccess.ConfigRecorder
	resources map[string][]awsaccess.ConfigResource
	errs      map[string]error // by region, for ConfigResources
}

func (m memoryConfig) ConfigRecorders(_ context.Context, region string) ([]awsaccess.ConfigRecorder, error) {
	return m.recorders[region], nil
}

func (m memoryConfig) ConfigResources(_ context.Context, region string) ([]awsaccess.ConfigResource, error) {
	if err := m.errs[region]; err != nil {
		return nil, err
	}
	return m.resources[region], nil
}

// memoryExplorer replaces the ExplorerReader with in-memory data per region.
type memoryExplorer struct {
	mu        sync.Mutex
	indexes   []awsaccess.ExplorerIndex
	indexErr  error
	filters   map[string]string
	noView    map[string]bool
	resources map[string][]awsaccess.ExplorerResource
	listErrs  map[string]error
	// listErrsOnce fail only the first ListResources of their region.
	listErrsOnce map[string]error
	listed       []string
}

func (m *memoryExplorer) ExplorerIndexes(context.Context) ([]awsaccess.ExplorerIndex, error) {
	return m.indexes, m.indexErr
}

func (m *memoryExplorer) DefaultView(_ context.Context, region string) (awsaccess.ExplorerView, error) {
	if m.noView[region] {
		return awsaccess.ExplorerView{}, &errkind.Error{Kind: errkind.AccessDenied, Reason: errkind.ReasonNoDefaultView, Op: awsaccess.OpGetDefaultView}
	}
	return awsaccess.ExplorerView{ARN: "view-" + region, Filter: m.filters[region]}, nil
}

func (m *memoryExplorer) UntaggedResources(_ context.Context, region, viewARN string) ([]awsaccess.ExplorerResource, error) {
	m.mu.Lock()
	m.listed = append(m.listed, region)
	once := m.listErrsOnce[region]
	delete(m.listErrsOnce, region)
	m.mu.Unlock()
	if once != nil {
		return nil, once
	}
	if viewARN != "view-"+region {
		return nil, fmt.Errorf("view %s used in %s", viewARN, region)
	}
	if err := m.listErrs[region]; err != nil {
		return nil, err
	}
	return m.resources[region], nil
}

func queue(region, name string) string { return "arn:aws:sqs:" + region + ":123456789012:" + name }

func snapshotOf(regions ...string) *Snapshot {
	return &Snapshot{EnabledRegions: regions, ScannedRegions: regions, FailedRegions: []FailedRegion{}, Resources: []Resource{}}
}

func recording(all bool, strategy string, excluded int) []awsaccess.ConfigRecorder {
	return []awsaccess.ConfigRecorder{{Name: "default", AllSupported: all, Strategy: strategy, ExcludedTypes: excluded, Recording: true}}
}

// Reliability design §3: the five recorder settings choose the path.
func TestDiscoverChoosesThePathFromTheRecorderSettings(t *testing.T) {
	cases := map[string]struct {
		recorders []awsaccess.ConfigRecorder
		methods   string
	}{
		"allSupported (default setting)":  {recording(true, "", 0), "awsConfig"},
		"exclusion with nothing excluded": {recording(false, "EXCLUSION_BY_RESOURCE_TYPES", 0), "awsConfig"},
		"exclusion with a type excluded":  {recording(false, "EXCLUSION_BY_RESOURCE_TYPES", 1), "awsConfig,resourceExplorer"},
		"inclusion of chosen types":       {recording(false, "INCLUSION_BY_RESOURCE_TYPES", 0), "awsConfig,resourceExplorer"},
		"not recording":                   {[]awsaccess.ConfigRecorder{{AllSupported: true, Recording: false}}, "resourceExplorer"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := snapshotOf("eu-west-1")
			config := memoryConfig{recorders: map[string][]awsaccess.ConfigRecorder{"eu-west-1": tc.recorders},
				resources: map[string][]awsaccess.ConfigResource{"eu-west-1": {{ARN: queue("eu-west-1", "c"), ResourceType: "AWS::SQS::Queue"}}}}
			explorer := &memoryExplorer{indexes: []awsaccess.ExplorerIndex{{Region: "eu-west-1"}},
				resources: map[string][]awsaccess.ExplorerResource{"eu-west-1": {{ARN: queue("eu-west-1", "e"), ResourceType: "sqs:queue", Region: "eu-west-1"}}}}

			if err := Discover(context.Background(), config, explorer, &recordingProgress{}, snap); err != nil {
				t.Fatal(err)
			}

			r := snap.Discovery.Regions[0]
			if got := strings.Join(r.Methods, ","); got != tc.methods || r.Status != StatusComplete {
				t.Fatalf("methods %s status %s, want %s complete", got, r.Status, tc.methods)
			}
			if snap.Discovery.Status != StatusComplete || r.Found != len(snap.Resources) {
				t.Fatalf("discovery %+v, %d resources", snap.Discovery, len(snap.Resources))
			}
		})
	}
}

// Reliability design §3: an aggregator index is asked once and its results
// are shared out by region; a region without an index tries AWS Config.
func TestDiscoverUsesTheAggregatorIndexOnce(t *testing.T) {
	snap := snapshotOf("ap-northeast-1", "eu-west-1", "us-east-1")
	explorer := &memoryExplorer{
		indexes: []awsaccess.ExplorerIndex{{Region: "us-east-1", Aggregator: true}, {Region: "eu-west-1"}},
		resources: map[string][]awsaccess.ExplorerResource{"us-east-1": {
			{ARN: queue("eu-west-1", "a"), ResourceType: "sqs:queue", Region: "eu-west-1"},
			{ARN: queue("us-east-1", "b"), ResourceType: "sqs:queue", Region: "us-east-1"},
			{ARN: queue("sa-east-1", "not-scanned"), ResourceType: "sqs:queue", Region: "sa-east-1"},
			{ARN: "arn:aws:cloudfront::123456789012:distribution/D", ResourceType: "cloudfront:distribution", Region: "global"},
		}},
	}
	config := memoryConfig{recorders: map[string][]awsaccess.ConfigRecorder{"ap-northeast-1": recording(true, "", 0)},
		resources: map[string][]awsaccess.ConfigResource{"ap-northeast-1": {{ARN: queue("ap-northeast-1", "c"), ResourceType: "AWS::SQS::Queue"}}}}

	if err := Discover(context.Background(), config, explorer, &recordingProgress{}, snap); err != nil {
		t.Fatal(err)
	}

	if strings.Join(explorer.listed, ",") != "us-east-1" {
		t.Fatalf("ListResources was called in %v, want the aggregator region once", explorer.listed)
	}
	byRegion := map[string]DiscoveryRegion{}
	for _, r := range snap.Discovery.Regions {
		byRegion[r.Region] = r
	}
	if r := byRegion["eu-west-1"]; r.Methods[0] != MethodAggregator || r.Found != 2 {
		t.Fatalf("eu-west-1 (first indexed region, holds the global resource) = %+v", r)
	}
	if r := byRegion["us-east-1"]; r.Methods[0] != MethodAggregator || r.Found != 1 {
		t.Fatalf("us-east-1 = %+v", r)
	}
	if r := byRegion["ap-northeast-1"]; r.Methods[0] != MethodConfig || r.Status != StatusComplete {
		t.Fatalf("ap-northeast-1 = %+v", r)
	}
	if len(snap.Resources) != 4 {
		t.Fatalf("resources = %+v", snap.Resources)
	}
	global := snap.Resources[0]
	if !global.Global || global.Region != "eu-west-1" || global.EverTagged {
		t.Fatalf("global resource = %+v", global)
	}
}

// Reliability design §3: when the aggregator index cannot be used, every
// indexed region is searched on its own. The default view of the aggregator
// region still returns the resources of every region: each keeps the region
// it is in, those of regions that were not scanned are left out, and global
// resources go to the region that returned them.
func TestDiscoverFallsBackToRegionsAfterAnAggregatorFailure(t *testing.T) {
	snap := snapshotOf("us-east-1", "us-west-2")
	explorer := &memoryExplorer{
		indexes:      []awsaccess.ExplorerIndex{{Region: "us-east-1", Aggregator: true}, {Region: "us-west-2"}},
		listErrsOnce: map[string]error{"us-east-1": &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpListResources}},
		resources: map[string][]awsaccess.ExplorerResource{
			"us-east-1": {
				{ARN: queue("us-west-2", "a"), ResourceType: "sqs:queue", Region: "us-west-2"},
				{ARN: queue("us-east-1", "b"), ResourceType: "sqs:queue", Region: "us-east-1"},
				{ARN: queue("sa-east-1", "not-scanned"), ResourceType: "sqs:queue", Region: "sa-east-1"},
				{ARN: "arn:aws:cloudfront::123456789012:distribution/D", ResourceType: "cloudfront:distribution", Region: "global"},
			},
			"us-west-2": {{ARN: queue("us-west-2", "a"), ResourceType: "sqs:queue", Region: "us-west-2"}},
		},
	}

	if err := Discover(context.Background(), memoryConfig{}, explorer, &recordingProgress{}, snap); err != nil {
		t.Fatal(err)
	}

	if len(explorer.listed) != 3 || explorer.listed[0] != "us-east-1" {
		t.Fatalf("ListResources was called in %v, want the aggregator, then each region", explorer.listed)
	}
	for _, r := range snap.Discovery.Regions {
		if r.Methods[0] != MethodExplorer || r.Status != StatusComplete {
			t.Fatalf("%s = %+v, want searched on its own", r.Region, r)
		}
	}
	found := map[string]int{}
	for _, r := range snap.Discovery.Regions {
		found[r.Region] = r.Found
	}
	if found["us-east-1"] != 2 || found["us-west-2"] != 1 {
		t.Fatalf("found by region = %v, want us-east-1 2 (b and the global resource), us-west-2 1", found)
	}
	byARN := map[string]Resource{}
	for _, r := range snap.Resources {
		byARN[r.ARN] = r
	}
	if len(snap.Resources) != 3 || byARN[queue("us-west-2", "a")].Region != "us-west-2" || byARN[queue("us-east-1", "b")].Region != "us-east-1" {
		t.Fatalf("resources = %+v", snap.Resources)
	}
	if g := byARN["arn:aws:cloudfront::123456789012:distribution/D"]; !g.Global || g.Region != "us-east-1" {
		t.Fatalf("global resource = %+v", g)
	}
}

// A filtered default view still gives results, but the region is failed and
// the whole discovery partial.
func TestDiscoverMarksAFilteredViewAsFailed(t *testing.T) {
	snap := snapshotOf("eu-west-1", "us-east-1")
	explorer := &memoryExplorer{indexes: []awsaccess.ExplorerIndex{{Region: "eu-west-1"}, {Region: "us-east-1"}},
		filters:   map[string]string{"eu-west-1": "service:ec2"},
		resources: map[string][]awsaccess.ExplorerResource{"eu-west-1": {{ARN: queue("eu-west-1", "a"), ResourceType: "sqs:queue", Region: "eu-west-1"}}}}

	if err := Discover(context.Background(), memoryConfig{}, explorer, &recordingProgress{}, snap); err != nil {
		t.Fatal(err)
	}

	r := snap.Discovery.Regions[0]
	if r.Status != StatusFailed || r.Reason != reasonViewFiltered || r.Found != 1 {
		t.Fatalf("eu-west-1 = %+v", r)
	}
	if snap.Discovery.Status != StatusPartial {
		t.Fatalf("status = %s", snap.Discovery.Status)
	}
	groups := snap.Discovery.NotChecked()
	if len(groups) != 1 || groups[0].Regions[0] != "eu-west-1" {
		t.Fatalf("not checked = %+v", groups)
	}
}

// BR3.1: resources the tagging API returned, resources with user tags and
// IAM's global types are left out; for the same ARN, AWS Config wins over
// Resource Explorer; resources without an ARN are kept as untaggable.
func TestDiscoverKeepsOnlyNewUntaggedResourcesAndPrefersConfig(t *testing.T) {
	snap := snapshotOf("eu-west-1")
	snap.Resources = []Resource{{ARN: queue("eu-west-1", "tagged"), Region: "eu-west-1", EverTagged: true, Tags: map[string]string{}}}
	both := queue("eu-west-1", "both")
	config := memoryConfig{recorders: map[string][]awsaccess.ConfigRecorder{"eu-west-1": recording(false, "INCLUSION_BY_RESOURCE_TYPES", 0)},
		resources: map[string][]awsaccess.ConfigResource{"eu-west-1": {
			{ARN: queue("eu-west-1", "tagged"), ResourceType: "AWS::SQS::Queue"},
			{ARN: queue("eu-west-1", "user-tags"), ResourceType: "AWS::SQS::Queue", Tags: map[string]string{"team": "a"}},
			{ARN: both, ResourceType: "AWS::SQS::Queue", Tags: map[string]string{"aws:cloudformation:stack-name": "s"}},
			{ARN: "arn:aws:iam::123456789012:role/r", ResourceType: "AWS::IAM::Role"},
			{ResourceType: "AWS::SSM::PatchCompliance", ResourceID: "p1"},
			{ResourceType: "AWS::SSM::PatchCompliance", ResourceID: "p2"},
		}}}
	explorer := &memoryExplorer{indexes: []awsaccess.ExplorerIndex{{Region: "eu-west-1"}},
		resources: map[string][]awsaccess.ExplorerResource{"eu-west-1": {
			{ARN: both, ResourceType: "sqs:queue", Region: "eu-west-1"},
			{ARN: "arn:aws:iam::123456789012:role/tagged-role", ResourceType: "iam:role", Region: "global"},
		}}}

	if err := Discover(context.Background(), config, explorer, &recordingProgress{}, snap); err != nil {
		t.Fatal(err)
	}

	if len(snap.Resources) != 4 {
		t.Fatalf("resources = %+v", snap.Resources)
	}
	kept := snap.Resources[0]
	if kept.ARN != both || kept.ResourceType != "sqs" || kept.IaCManagedBy != "cloudformation:s" || kept.EverTagged {
		t.Fatalf("the AWS Config resource must be kept: %+v", kept)
	}
	noARN := snap.Resources[2:]
	if noARN[0].ARN != "" || !noARN[0].NotTaggable || noARN[1].ResourceType != "ssm:patch-compliance" {
		t.Fatalf("resources without an ARN = %+v", noARN)
	}
	if NeverTagged(snap.Resources) != 3 {
		t.Fatalf("never tagged = %d", NeverTagged(snap.Resources))
	}
}

// Security design §4 and BR3.2: the reasons of an unavailable region name
// what is missing, and never AWS error text.
func TestDiscoverReasonsOfUnavailableRegions(t *testing.T) {
	snap := snapshotOf("ap-northeast-1", "eu-west-1", "us-east-1", "us-west-2")
	config := memoryConfig{
		recorders: map[string][]awsaccess.ConfigRecorder{"us-west-2": recording(true, "", 0)},
		errs: map[string]error{"us-west-2": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpSelectResourceConfig,
			Cause: fmt.Errorf("secret text")}},
	}
	explorer := &memoryExplorer{indexes: []awsaccess.ExplorerIndex{{Region: "eu-west-1"}, {Region: "ap-northeast-1"}},
		noView:   map[string]bool{"ap-northeast-1": true},
		listErrs: map[string]error{"eu-west-1": &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpListResources}}}

	if err := Discover(context.Background(), config, explorer, &recordingProgress{}, snap); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"ap-northeast-1": "AWS Config not recording, no default Resource Explorer view",
		"eu-west-1":      "AWS Config not recording, default Resource Explorer view not usable (AccessDenied resource-explorer-2:Search)",
		"us-east-1":      "AWS Config not recording, no Resource Explorer index",
		"us-west-2":      "AccessDenied config:SelectResourceConfig, no Resource Explorer index",
	}
	for _, r := range snap.Discovery.Regions {
		if r.Status != StatusUnavailable || r.Reason != want[r.Region] {
			t.Errorf("%s: %s %q, want unavailable %q", r.Region, r.Status, r.Reason, want[r.Region])
		}
	}
	if snap.Discovery.Status != StatusUnavailable {
		t.Fatalf("status = %s", snap.Discovery.Status)
	}
	if strings.Contains(fmt.Sprint(snap.Discovery), "secret") {
		t.Fatal("an AWS error text reached the reasons")
	}
	snap2 := snapshotOf("eu-west-1")
	explorer2 := &memoryExplorer{indexErr: &errkind.Error{Kind: errkind.Transient, Op: awsaccess.OpListIndexes}}
	if err := Discover(context.Background(), memoryConfig{}, explorer2, &recordingProgress{}, snap2); err != nil {
		t.Fatal(err)
	}
	if r := snap2.Discovery.Regions[0]; r.Reason != "AWS Config not recording, Transient resource-explorer-2:ListIndexes" {
		t.Fatalf("reason = %q", r.Reason)
	}
}

// BR4.3: expired credentials during the discovery stop it with the error.
func TestDiscoverStopsOnExpiredCredentials(t *testing.T) {
	expired := &errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsExpired, Op: awsaccess.OpListResources}
	regions := []string{"a-1", "b-1", "c-1", "d-1", "e-1", "f-1", "g-1", "h-1", "i-1", "j-1"}
	snap := snapshotOf(regions...)
	var indexes []awsaccess.ExplorerIndex
	for _, r := range regions {
		indexes = append(indexes, awsaccess.ExplorerIndex{Region: r})
	}
	explorer := &memoryExplorer{indexes: indexes, listErrs: map[string]error{"c-1": expired}}

	err := Discover(context.Background(), memoryConfig{}, explorer, &recordingProgress{}, snap)

	if err != expired || snap.Discovery != nil {
		t.Fatalf("err = %v, discovery = %+v", err, snap.Discovery)
	}
}

// NFR1.9: at most Concurrency regions are read at the same time.
func TestCollectReadsAtMostConcurrencyRegionsAtOnce(t *testing.T) {
	var regions []string
	for i := range 20 {
		regions = append(regions, fmt.Sprintf("r%02d-1", i))
	}
	reader := &countingReader{}

	snap, err := Collect(context.Background(), reader, &recordingProgress{}, account(regions...), regions, time.Now())

	if err != nil || len(snap.ScannedRegions) != 20 {
		t.Fatalf("snapshot %+v, err %v", snap, err)
	}
	if reader.peak > Concurrency || reader.peak < 2 {
		t.Fatalf("peak of regions read at once = %d, want 2..%d", reader.peak, Concurrency)
	}
}

type countingReader struct {
	mu        sync.Mutex
	now, peak int
}

func (c *countingReader) ListTaggedResources(context.Context, string) ([]awsaccess.TaggedResource, error) {
	c.mu.Lock()
	c.now++
	c.peak = max(c.peak, c.now)
	c.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	c.mu.Lock()
	c.now--
	c.mu.Unlock()
	return nil, nil
}

// BR5.1: the dictionary only replaces a whole normalized key.
func TestAbbreviationDictionaryGroups(t *testing.T) {
	for key, want := range map[string]string{"env": "environment", "ENV": "environment", "Prod": "production",
		"dev": "development", "stg": "staging", "envname": "envname", "env-name": "envname", "Environment": "environment"} {
		if got := CanonicalKey(key); got != want {
			t.Errorf("CanonicalKey(%q) = %q, want %q", key, got, want)
		}
	}
	groups := GroupKeys([]Resource{
		resource(map[string]string{"stg": "1"}),
		resource(map[string]string{"staging": "1"}),
		resource(map[string]string{"dev": "1"}),
		resource(map[string]string{"Owner": "1"}),
	})
	byKey := map[string]KeyVariantGroup{}
	for _, g := range groups {
		byKey[g.NormalizedKey] = g
	}
	if g := byKey["staging"]; !g.AbbreviationOnly || g.IsVariant || g.TotalResources != 2 {
		t.Fatalf("staging = %+v", g)
	}
	if g := byKey["development"]; !g.AbbreviationOnly || !g.Shown() {
		t.Fatalf("development = %+v", g)
	}
	if g := byKey["owner"]; g.Shown() {
		t.Fatalf("owner = %+v", g)
	}
	if CountVariants(groups) != 0 || len(Variants(groups)) != 2 {
		t.Fatalf("variants %d, shown %d", CountVariants(groups), len(Variants(groups)))
	}
	if len(Abbreviations()) != 4 {
		t.Fatal("the dictionary has the four entries of BR5.1")
	}
}

// BR5.2, BR3.3: required keys are counted with their case, untaggable types
// apart, and the count is a lower bound unless discovery was complete.
func TestMissingRequiredAndUnsupportedTypes(t *testing.T) {
	snap := &Snapshot{Discovery: &Discovery{Status: StatusPartial}, Resources: []Resource{
		{ARN: "a", Tags: map[string]string{"Owner": "x"}, ResourceType: "sqs"},
		{ARN: "b", Tags: map[string]string{"owner": "x"}, ResourceType: "sqs"},
		{ARN: "c", Tags: map[string]string{}, ResourceType: "sqs"},
		{Tags: map[string]string{}, ResourceType: "ssm:patch-compliance", NotTaggable: true},
		{Tags: map[string]string{}, ResourceType: "ssm:patch-compliance", NotTaggable: true},
		{ARN: "g", Tags: map[string]string{}, ResourceType: "iam:group", NotTaggable: true},
	}}

	got := MissingRequired(snap, []string{"Owner", "CostCenter"})

	if len(got) != 2 || got[0].ResourceCount != 2 || got[0].NotTaggableCount != 3 || !got[0].LowerBound ||
		strings.Join(got[0].ARNs, ",") != "b,c" || got[1].ResourceCount != 3 {
		t.Fatalf("missing = %+v", got)
	}
	if fmt.Sprint(UnsupportedResourceTypes(snap.Resources)) != "[{ssm:patch-compliance 2} {iam:group 1}]" {
		t.Fatalf("unsupported = %v", UnsupportedResourceTypes(snap.Resources))
	}
	report := NewScanReport(snap, "r.yaml", []string{"Owner"})
	if !report.IssuesFound() || report.MissingKeys() != 1 || report.NeverTagged != 6 {
		t.Fatalf("report = %+v", report)
	}
	if NewScanReport(snap, "", nil).Missing != nil {
		t.Fatal("without a rule file, required tags are not checked")
	}
	snap.Discovery.Status = StatusComplete
	if MissingRequired(snap, []string{"Owner"})[0].LowerBound {
		t.Fatal("a complete discovery is not a lower bound")
	}
}

var arnForm = regexp.MustCompile(`^[a-z0-9-]+(:[A-Za-z0-9/-]+)?$`)

// tech-stack-decisions.md: the type tables are checked for shape: ARN form,
// no repeats, and every untaggable type reachable from the AWS Config table.
func TestTypeTablesHaveTheARNForm(t *testing.T) {
	targets := map[string]bool{}
	for configType, arnType := range configToARNForm {
		if !strings.HasPrefix(configType, "AWS::") || strings.Count(configType, "::") != 2 {
			t.Errorf("%q is not an AWS Config type", configType)
		}
		if !arnForm.MatchString(arnType) {
			t.Errorf("%s maps to %q, not the ARN form", configType, arnType)
		}
		if targets[arnType] {
			t.Errorf("%q is the target of two AWS Config types", arnType)
		}
		targets[arnType] = true
	}
	for typ := range notTaggableTypes {
		if !arnForm.MatchString(typ) || !targets[typ] {
			t.Errorf("untaggable type %q is not an ARN-form target of the table", typ)
		}
	}
	if ConfigTypeToARNForm("AWS::New::Thing") != "AWS::New::Thing" || ConfigTypeToARNForm("AWS::EC2::Instance") != "ec2:instance" {
		t.Fatal("unknown types keep the AWS Config form")
	}
	if !IsNotTaggable("iam:group") || IsNotTaggable("sqs") {
		t.Fatal("IsNotTaggable does not follow the table")
	}
}
