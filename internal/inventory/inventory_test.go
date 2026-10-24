package inventory

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// memoryTagReader replaces the TagReader of contract C6 with in-memory data.
type memoryTagReader struct {
	byRegion map[string][]awsaccess.TaggedResource
	err      error
}

func (m memoryTagReader) ListTaggedResources(_ context.Context, region string) ([]awsaccess.TaggedResource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.byRegion[region], nil
}

// recordingProgress records the progress calls; several regions call it at
// the same time.
type recordingProgress struct {
	mu    sync.Mutex
	lines []string
}

func (r *recordingProgress) add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
}

func (r *recordingProgress) RegionsWaiting(regions []string) {
	r.add("waiting " + strings.Join(regions, ","))
}
func (r *recordingProgress) RegionStarted(region string) { r.add("started " + region) }
func (r *recordingProgress) RegionDone(region string, n int) {
	r.add(fmt.Sprintf("done %s %d", region, n))
}
func (r *recordingProgress) RegionFailed(region, reason string) {
	r.add("failed " + region + " " + reason)
}
func (r *recordingProgress) DiscoveryStarted(regions []string) {
	r.add("discovery " + strings.Join(regions, ","))
}
func (r *recordingProgress) DiscoveryRegionDone(region, status string) {
	r.add("discovered " + region + " " + status)
}
func (r *recordingProgress) Warn(message string) { r.add("warn " + message) }

func (r *recordingProgress) sorted() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := append([]string(nil), r.lines...)
	sort.Strings(lines)
	return strings.Join(lines, "|")
}

func resource(tags map[string]string) Resource {
	return Resource{ARN: "arn:aws:sqs:us-east-1:123456789012:q", Tags: tags}
}

func account(regions ...string) awsaccess.AccountContext {
	return awsaccess.AccountContext{AccountID: "123456789012", Region: "us-east-1", EnabledRegions: regions}
}

func TestCollectReadsTheRegionAndKeepsAllTags(t *testing.T) {
	reader := memoryTagReader{byRegion: map[string][]awsaccess.TaggedResource{"us-east-1": {
		{ARN: "arn:aws:ec2:us-east-1:123456789012:instance/i-1", Tags: map[string]string{"env": "dev", "aws:cloudformation:stack-name": "s"}},
		{ARN: "arn:aws:s3:::bucket", Tags: map[string]string{}},
	}}}
	progress := &recordingProgress{}
	now := time.Date(2026, 10, 20, 9, 30, 0, 0, time.FixedZone("JST", 9*3600))

	snap, err := Collect(context.Background(), reader, progress, account("eu-west-1", "us-east-1"), []string{"us-east-1"}, now)

	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(progress.lines, "|") != "waiting us-east-1|started us-east-1|done us-east-1 2" {
		t.Fatalf("progress = %v", progress.lines)
	}
	if snap.SnapshotID != "snap-20261020T003000Z" || !snap.ScannedAt.Equal(now) || snap.ScannedAt.Location() != time.UTC {
		t.Fatalf("snapshot id/time = %q %v", snap.SnapshotID, snap.ScannedAt)
	}
	if snap.AccountID != "123456789012" || len(snap.FailedRegions) != 0 || len(snap.Resources) != 2 || len(snap.EnabledRegions) != 2 {
		t.Fatalf("snapshot = %+v", snap)
	}
	instance, bucket := snap.Resources[0], snap.Resources[1] // sorted by ARN
	if instance.ResourceType != "ec2:instance" || instance.Region != "us-east-1" || instance.Tags["aws:cloudformation:stack-name"] != "s" ||
		instance.IaCManagedBy != "cloudformation:s" || !instance.EverTagged || instance.Global {
		t.Fatalf("instance = %+v", instance)
	}
	if !bucket.Global || bucket.IaCManagedBy != "" {
		t.Fatalf("bucket = %+v", bucket)
	}
}

// BR4.1: a denied region is recorded and the other regions are still read.
func TestCollectKeepsADeniedRegionAndReadsTheOthers(t *testing.T) {
	denied := &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetResources, Region: "eu-west-1"}
	reader := regionErrReader{memoryTagReader: memoryTagReader{byRegion: map[string][]awsaccess.TaggedResource{
		"us-east-1": {{ARN: "arn:aws:sqs:us-east-1:123456789012:q", Tags: map[string]string{}}},
	}}, errs: map[string]error{
		"eu-west-1":      denied,
		"ap-northeast-1": &errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout, Op: awsaccess.OpGetResources},
		"sa-east-1":      &errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonRetriesExhausted, Op: awsaccess.OpGetResources},
	}}
	progress := &recordingProgress{}

	snap, err := Collect(context.Background(), reader, progress, account(), []string{"us-east-1", "eu-west-1", "ap-northeast-1", "sa-east-1"}, time.Now())

	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Resources) != 1 || snap.Complete() || snap.DoneRegions() != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	got := fmt.Sprint(snap.FailedRegions)
	want := "[{ap-northeast-1 timeout Transient tag:GetResources} {eu-west-1 accessDenied AccessDenied tag:GetResources} {sa-east-1 transient Transient tag:GetResources}]"
	if got != want {
		t.Fatalf("failed regions = %s", got)
	}
	if !strings.Contains(progress.sorted(), "failed eu-west-1 AccessDenied (tag:GetResources)") {
		t.Fatalf("progress = %v", progress.lines)
	}
}

// regionErrReader fails the regions listed in errs.
type regionErrReader struct {
	memoryTagReader
	errs map[string]error
}

func (r regionErrReader) ListTaggedResources(ctx context.Context, region string) ([]awsaccess.TaggedResource, error) {
	if err := r.errs[region]; err != nil {
		return nil, err
	}
	return r.memoryTagReader.ListTaggedResources(ctx, region)
}

// BR4.3: missing credentials and unexpected errors stop the whole run.
func TestCollectStopsOnCredentialAndUnexpectedErrors(t *testing.T) {
	for _, stop := range []error{
		&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsExpired, Op: awsaccess.OpGetResources},
		&errkind.Error{Kind: errkind.Internal, Op: awsaccess.OpGetResources},
		fmt.Errorf("no kind"),
	} {
		_, err := Collect(context.Background(), memoryTagReader{err: stop}, &recordingProgress{}, account(), []string{"eu-west-1"}, time.Now())
		if err != stop {
			t.Fatalf("err = %v, want %v", err, stop)
		}
	}
}

func TestResourceTypeOfUsesServiceAndType(t *testing.T) {
	cases := map[string]string{
		"arn:aws:ec2:us-east-1:123456789012:instance/i-0abc":     "ec2:instance",
		"arn:aws:lambda:us-east-1:123456789012:function:handler": "lambda:function",
		"arn:aws:sqs:us-east-1:123456789012:queue-name":          "sqs",
		"arn:aws:s3:::my-bucket":                                 "s3",
		"arn:aws-cn:rds:cn-north-1:123456789012:db:mydb":         "rds:db",
		"not-an-arn":                        "unknown",
		"arn:aws::us-east-1:123456789012:x": "unknown",
	}
	for arn, want := range cases {
		if got := ResourceTypeOf(arn); got != want {
			t.Errorf("ResourceTypeOf(%q) = %q, want %q", arn, got, want)
		}
	}
}

func TestNormalizeKeyAndReservedKeys(t *testing.T) {
	for key, want := range map[string]string{
		"Cost-Center": "costcenter", "cost_center": "costcenter", "Cost Center": "costcenter",
		"ENV": "env", "Ümlaut-Key": "ümlautkey", "a.b": "a.b",
	} {
		if got := NormalizeKey(key); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", key, got, want)
		}
	}
	for key, want := range map[string]bool{"aws:x": true, "AWS:x": true, "Aws:": true, "awsx": false, "aw": false, "my:aws:x": false} {
		if got := IsAWSReservedKey(key); got != want {
			t.Errorf("IsAWSReservedKey(%q) = %v", key, got)
		}
	}
}

func TestGroupKeysCountsEachResourceOncePerGroup(t *testing.T) {
	groups := GroupKeys([]Resource{
		resource(map[string]string{"env": "dev", "Env": "dev"}), // both spellings on one resource
		resource(map[string]string{"env": "prod"}),
		resource(map[string]string{"Env": "test"}),
		resource(map[string]string{"Owner": "a"}),
	})

	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	env := groups[0]
	// U2: env is an abbreviation, so the group is headed by environment.
	if env.NormalizedKey != "environment" || env.TotalResources != 3 || !env.IsVariant || env.AbbreviationOnly {
		t.Fatalf("env group = %+v", env)
	}
	if fmt.Sprint(env.Spellings) != "[{Env 2} {env 2}]" {
		t.Fatalf("spellings (count, then key order) = %v", env.Spellings)
	}
	if groups[1].IsVariant || groups[1].TotalResources != 1 {
		t.Fatalf("owner group = %+v", groups[1])
	}
	if v := Variants(groups); len(v) != 1 || v[0].NormalizedKey != "environment" {
		t.Fatalf("Variants = %+v", v)
	}
}

func TestGroupKeysOrderAndAWSKeysLeftOut(t *testing.T) {
	groups := GroupKeys([]Resource{
		resource(map[string]string{"b": "1", "aws:cloudformation:stack-id": "x", "AWS:Weird": "y"}),
		resource(map[string]string{"a": "1"}),
		resource(map[string]string{"cost-center": "1"}),
		resource(map[string]string{"cost_center": "1"}),
	})

	var order []string
	for _, g := range groups {
		order = append(order, g.NormalizedKey)
		if strings.HasPrefix(g.NormalizedKey, "aws:") {
			t.Fatalf("aws: key grouped: %+v", g)
		}
	}
	if strings.Join(order, ",") != "costcenter,a,b" {
		t.Fatalf("group order = %v (by total resources, then key)", order)
	}
	if len(GroupKeys(nil)) != 0 || Variants(nil) != nil {
		t.Fatal("empty input must give no groups")
	}
}

// U3 BR3.4: the rule file reader asks the type table for the form to write
// in a condition; a type the table does not know has none.
func TestARNFormOfLooksUpTheTypeTable(t *testing.T) {
	for configType, want := range map[string]string{"AWS::EC2::Instance": "ec2:instance", "AWS::S3::Bucket": "s3", "AWS::SQS::Queue": "sqs"} {
		if got, ok := ARNFormOf(configType); !ok || got != want {
			t.Errorf("ARNFormOf(%q) = %q, %v; want %q", configType, got, ok, want)
		}
	}
	for _, configType := range []string{"AWS::Example::Thing", "ec2:instance", "", "aws::ec2::instance"} {
		if got, ok := ARNFormOf(configType); ok || got != "" {
			t.Errorf("ARNFormOf(%q) = %q, %v; want no form", configType, got, ok)
		}
	}
	// Every form the table gives can be written in a condition: it is the
	// service, or the service and a type without blanks or colons.
	condition := regexp.MustCompile(`^[a-z0-9-]+(:[^\s:]+)?$`)
	for configType := range configToARNForm {
		if form, _ := ARNFormOf(configType); !condition.MatchString(form) {
			t.Errorf("%s gives %q, which a condition cannot use", configType, form)
		}
	}
}
