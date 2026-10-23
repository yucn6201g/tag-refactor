// Package inventory collects the resources of an account and groups tag keys
// that are written in different ways (contract C7). It reads every requested
// region at the same time (at most Concurrency at once), keeps the regions
// that could not be read as part of the result, looks for resources that were
// never tagged through AWS Config and Resource Explorer, and counts missing
// required tags.
package inventory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Concurrency is the number of regions read at the same time (NFR1.9,
// performance design §1). It is fixed: no option or environment variable
// changes it. Tests in this package may lower it.
var Concurrency = 8

// Resource is one resource. Only the ARN, the type and the tags are kept, so
// memory grows with the number of resources only (NFR1.4).
type Resource struct {
	// ARN is empty for a resource AWS Config knows without an ARN; such a
	// resource can only be counted (BR3.3).
	ARN    string
	Region string
	// ResourceType is the service and type part of the ARN, for example
	// "ec2:instance", or the service alone ("sqs") when the ARN has no type.
	ResourceType string
	// Tags holds all tags, including those that start with aws:.
	Tags map[string]string
	// Global is true when the ARN has no region (BR2.2).
	Global bool
	// EverTagged is true for resources the tagging API returned, false for
	// those found by AWS Config or Resource Explorer (BR3.1).
	EverTagged bool
	// IaCManagedBy is "cloudformation:<stack name>" when the resource carries
	// the aws:cloudformation:stack-name tag (BR2.3), empty otherwise.
	IaCManagedBy string
	// NotTaggable is true for resources of a type the tagging API cannot
	// change, or without an ARN (BR3.3).
	NotTaggable bool
}

// Reasons a region could not be read (FailedRegion.Reason).
const (
	FailedAccessDenied = "accessDenied"
	FailedTransient    = "transient"
	FailedTimeout      = "timeout"
)

// FailedRegion is a region the tagging API could not read (BR4.1).
type FailedRegion struct {
	Region string
	// Reason is FailedAccessDenied, FailedTransient or FailedTimeout.
	Reason string
	// Kind is the error kind, for example "AccessDenied".
	Kind string
	// Operation is the IAM action of the failed call, for example
	// "tag:GetResources".
	Operation string
}

// Snapshot is the result of one collection.
type Snapshot struct {
	SnapshotID string
	AccountID  string
	ScannedAt  time.Time
	// EnabledRegions are the regions enabled for the account (BR1.2).
	EnabledRegions []string
	// ScannedRegions are the regions read, sorted.
	ScannedRegions []string
	// FailedRegions are the scanned regions the tagging API could not read.
	FailedRegions []FailedRegion
	// Resources are sorted by ARN; each ARN appears once.
	Resources []Resource
	// Discovery is the search for never-tagged resources; nil when it did not
	// run (plan).
	Discovery *Discovery
}

// Complete reports whether every scanned region was read. The discovery of
// never-tagged resources does not count (BR5.3).
func (s *Snapshot) Complete() bool { return len(s.FailedRegions) == 0 }

// DoneRegions is the number of scanned regions that were read (BR1.2).
func (s *Snapshot) DoneRegions() int { return len(s.ScannedRegions) - len(s.FailedRegions) }

// ProgressSink receives progress while resources are collected (contract
// C11). The display package implements it. Its methods are called from
// several goroutines.
type ProgressSink interface {
	// RegionsWaiting lists the regions that wait to be read.
	RegionsWaiting(regions []string)
	RegionStarted(region string)
	RegionDone(region string, count int)
	RegionFailed(region, reason string)
	// DiscoveryStarted starts the search for never-tagged resources in the
	// regions.
	DiscoveryStarted(regions []string)
	// DiscoveryRegionDone gives a region's discovery status: complete,
	// unavailable or failed.
	DiscoveryRegionDone(region, status string)
	Warn(message string)
}

// regionResult is what reading one region left: resources, or a failure.
type regionResult struct {
	tagged []awsaccess.TaggedResource
	failed *FailedRegion
}

// fatalError keeps the first error that stops the whole run (BR4.3).
type fatalError struct {
	mu     sync.Mutex
	err    error
	cancel context.CancelFunc
}

// set records err as the error of the run, unless one was recorded already,
// and cancels the reads still running.
func (f *fatalError) set(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.err = err
		f.cancel()
	}
}

func (f *fatalError) get() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// Collect reads every tagged resource of the regions through the read-only
// TagReader, at most Concurrency regions at once (BR1.1, BR2.1, BR2.4). A
// region that denies the read, or keeps failing after the retries, is kept
// in FailedRegions and the others go on (BR4.1). Any other error, notably
// missing or expired credentials, cancels every read and is returned once all
// of them have ended (BR4.3, NFR5.8); no snapshot is returned then.
func Collect(ctx context.Context, reader awsaccess.TagReader, sink ProgressSink, acc awsaccess.AccountContext, regions []string, now time.Time) (*Snapshot, error) {
	regions = sortedUnique(regions)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fatal := &fatalError{cancel: cancel}
	results := make([]regionResult, len(regions))
	sink.RegionsWaiting(regions)
	g := new(errgroup.Group)
	g.SetLimit(Concurrency)
	for i, region := range regions {
		g.Go(func() error {
			// A region never returns an error to the group: a failure must
			// not cancel the other regions through errgroup (BR4.1).
			results[i] = readRegion(ctx, reader, sink, fatal, region)
			return nil
		})
	}
	_ = g.Wait()
	if err := fatal.get(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return merge(acc, regions, results, now), nil
}

func readRegion(ctx context.Context, reader awsaccess.TagReader, sink ProgressSink, fatal *fatalError, region string) regionResult {
	if ctx.Err() != nil {
		return regionResult{}
	}
	sink.RegionStarted(region)
	tagged, err := reader.ListTaggedResources(ctx, region)
	if err == nil {
		sink.RegionDone(region, len(tagged))
		return regionResult{tagged: tagged}
	}
	failed, ok := regionFailure(err, region)
	if !ok || ctx.Err() != nil {
		fatal.set(err)
		return regionResult{}
	}
	sink.RegionFailed(region, failed.Kind+" ("+failed.Operation+")")
	return regionResult{failed: failed}
}

// regionFailure turns an error of one region into a FailedRegion when the
// error belongs to that region only: a denial or a failure that the retries
// could not overcome (BR4.1). Everything else stops the run: missing or
// expired credentials concern every region (BR4.3), and an unexpected answer
// is not something to carry on from.
func regionFailure(err error, region string) (*FailedRegion, bool) {
	e, ok := errkind.As(err)
	if !ok {
		return nil, false
	}
	f := &FailedRegion{Region: region, Kind: e.Kind.String(), Operation: awsaccess.IAMAction(e.Op)}
	switch {
	case e.Kind == errkind.AccessDenied:
		f.Reason = FailedAccessDenied
	case e.Kind == errkind.Transient && e.Reason == errkind.ReasonTimeout:
		f.Reason = FailedTimeout
	case e.Kind == errkind.Transient:
		f.Reason = FailedTransient
	default:
		return nil, false
	}
	return f, true
}

// merge puts the regions together in region name order, whatever order they
// finished in (NFR5.7). A resource returned by several regions, such as a
// global one, is kept once, with the first region in name order (BR2.2).
func merge(acc awsaccess.AccountContext, regions []string, results []regionResult, now time.Time) *Snapshot {
	snap := &Snapshot{
		SnapshotID:     "snap-" + now.UTC().Format("20060102T150405Z"),
		AccountID:      acc.AccountID,
		ScannedAt:      now.UTC(),
		EnabledRegions: append([]string(nil), acc.EnabledRegions...),
		ScannedRegions: regions,
		FailedRegions:  []FailedRegion{},
		Resources:      []Resource{},
	}
	seen := map[string]bool{}
	for i, region := range regions {
		if f := results[i].failed; f != nil {
			snap.FailedRegions = append(snap.FailedRegions, *f)
			continue
		}
		for _, t := range results[i].tagged {
			if seen[t.ARN] {
				continue
			}
			seen[t.ARN] = true
			snap.Resources = append(snap.Resources, taggedResource(t, region))
		}
	}
	sortResources(snap.Resources)
	return snap
}

func taggedResource(t awsaccess.TaggedResource, region string) Resource {
	r := Resource{
		ARN:          t.ARN,
		Region:       region,
		ResourceType: ResourceTypeOf(t.ARN),
		Tags:         t.Tags,
		Global:       arnRegion(t.ARN) == "",
		EverTagged:   true,
	}
	if stack := t.Tags[cloudFormationStackKey]; stack != "" {
		r.IaCManagedBy = "cloudformation:" + stack
	}
	return r
}

// cloudFormationStackKey is the tag CloudFormation puts on the resources of a
// stack (BR2.3).
const cloudFormationStackKey = "aws:cloudformation:stack-name"

// sortResources sorts by ARN; resources without an ARN come last, by region,
// type and identifier, so the order never depends on timing (NFR5.7).
func sortResources(resources []Resource) {
	sort.SliceStable(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if (a.ARN == "") != (b.ARN == "") {
			return a.ARN != ""
		}
		if a.ARN != b.ARN {
			return a.ARN < b.ARN
		}
		if a.Region != b.Region {
			return a.Region < b.Region
		}
		return a.ResourceType < b.ResourceType
	})
}

func sortedUnique(regions []string) []string {
	out := append([]string(nil), regions...)
	sort.Strings(out)
	n := 0
	for i, r := range out {
		if i == 0 || r != out[n-1] {
			out[n] = r
			n++
		}
	}
	return out[:n]
}

// arnRegion returns the region part of an ARN, empty for global resources.
func arnRegion(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 {
		return ""
	}
	return parts[3]
}

// ResourceTypeOf returns the service and type part of an ARN (BR2.1): for
// "arn:aws:ec2:us-east-1:123456789012:instance/i-1" it is "ec2:instance", for
// "arn:aws:sqs:us-east-1:123456789012:queue" it is "sqs".
func ResourceTypeOf(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" || parts[2] == "" {
		return "unknown"
	}
	service, resource := parts[2], parts[5]
	if i := strings.IndexAny(resource, "/:"); i > 0 {
		return service + ":" + resource[:i]
	}
	return service
}

// IsAWSReservedKey reports whether a tag key starts with "aws:" in any case.
// Those keys belong to AWS and are never grouped or changed.
func IsAWSReservedKey(key string) bool {
	return len(key) >= 4 && strings.EqualFold(key[:4], "aws:")
}

// hasUserTags reports whether tags has a key that does not start with aws:.
func hasUserTags(tags map[string]string) bool {
	for k := range tags {
		if !IsAWSReservedKey(k) {
			return true
		}
	}
	return false
}
