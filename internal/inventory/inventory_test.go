package inventory

import (
	"context"
	"fmt"
	"strings"
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

type recordingProgress struct{ lines []string }

func (r *recordingProgress) RegionStarted(region string) {
	r.lines = append(r.lines, "started "+region)
}
func (r *recordingProgress) RegionDone(region string, n int) {
	r.lines = append(r.lines, fmt.Sprintf("done %s %d", region, n))
}
func (r *recordingProgress) RegionFailed(region, reason string) {
	r.lines = append(r.lines, "failed "+region+" "+reason)
}
func (r *recordingProgress) Warn(message string) { r.lines = append(r.lines, "warn "+message) }

func resource(tags map[string]string) Resource {
	return Resource{ARN: "arn:aws:sqs:us-east-1:123456789012:q", Tags: tags}
}

func TestCollectReadsTheRegionAndKeepsAllTags(t *testing.T) {
	reader := memoryTagReader{byRegion: map[string][]awsaccess.TaggedResource{"us-east-1": {
		{ARN: "arn:aws:ec2:us-east-1:123456789012:instance/i-1", Tags: map[string]string{"env": "dev", "aws:cloudformation:stack-name": "s"}},
		{ARN: "arn:aws:s3:::bucket", Tags: map[string]string{}},
	}}}
	progress := &recordingProgress{}
	now := time.Date(2026, 10, 20, 9, 30, 0, 0, time.FixedZone("JST", 9*3600))

	snap, err := Collect(context.Background(), reader, progress, "123456789012", []string{"us-east-1"}, now)

	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(progress.lines, "|") != "started us-east-1|done us-east-1 2" {
		t.Fatalf("progress = %v", progress.lines)
	}
	if snap.SnapshotID != "snap-20261020T003000Z" || !snap.ScannedAt.Equal(now) || snap.ScannedAt.Location() != time.UTC {
		t.Fatalf("snapshot id/time = %q %v", snap.SnapshotID, snap.ScannedAt)
	}
	if snap.AccountID != "123456789012" || len(snap.FailedRegions) != 0 || len(snap.Resources) != 2 {
		t.Fatalf("snapshot = %+v", snap)
	}
	first := snap.Resources[0]
	if first.ResourceType != "ec2:instance" || first.Region != "us-east-1" || first.Tags["aws:cloudformation:stack-name"] != "s" {
		t.Fatalf("resource = %+v", first)
	}
}

func TestCollectStopsAndReportsTheRegionOnAReadError(t *testing.T) {
	denied := &errkind.Error{Kind: errkind.AccessDenied, Op: awsaccess.OpGetResources}
	progress := &recordingProgress{}

	_, err := Collect(context.Background(), memoryTagReader{err: denied}, progress, "1", []string{"eu-west-1"}, time.Now())

	if err != denied {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(progress.lines, "|") != "started eu-west-1|failed eu-west-1 AccessDenied" {
		t.Fatalf("progress = %v", progress.lines)
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
	if env.NormalizedKey != "env" || env.TotalResources != 3 || !env.IsVariant {
		t.Fatalf("env group = %+v", env)
	}
	if fmt.Sprint(env.Spellings) != "[{Env 2} {env 2}]" {
		t.Fatalf("spellings (count, then key order) = %v", env.Spellings)
	}
	if groups[1].IsVariant || groups[1].TotalResources != 1 {
		t.Fatalf("owner group = %+v", groups[1])
	}
	if v := Variants(groups); len(v) != 1 || v[0].NormalizedKey != "env" {
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
