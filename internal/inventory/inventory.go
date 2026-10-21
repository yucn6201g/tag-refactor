// Package inventory collects the tagged resources of a region and groups tag
// keys that differ only in case and separators (contract C7).
package inventory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Resource is one tagged resource. Only the ARN, the type and the tags are
// kept, so memory grows with the number of resources only (NFR1.4).
type Resource struct {
	ARN    string
	Region string
	// ResourceType is the service and type part of the ARN, for example
	// "ec2:instance", or the service alone ("sqs") when the ARN has no type.
	ResourceType string
	// Tags holds all tags, including those that start with aws:.
	Tags map[string]string
}

// FailedRegion is a region that could not be read.
type FailedRegion struct {
	Region string
	Reason string
}

// Snapshot is the result of one collection.
type Snapshot struct {
	SnapshotID     string
	AccountID      string
	ScannedAt      time.Time
	ScannedRegions []string
	// FailedRegions is always empty in U1: a read failure stops the command.
	FailedRegions []FailedRegion
	Resources     []Resource
}

// ProgressSink receives progress while resources are collected (contract
// C11). The display package implements it.
type ProgressSink interface {
	RegionStarted(region string)
	RegionDone(region string, count int)
	RegionFailed(region, reason string)
	Warn(message string)
}

// Collect reads every tagged resource of the regions through the read-only
// TagReader (BR1.5, BR2.1). In U1 a failed region stops the collection with
// the reader's error.
func Collect(ctx context.Context, reader awsaccess.TagReader, sink ProgressSink, accountID string, regions []string, now time.Time) (*Snapshot, error) {
	snap := &Snapshot{
		SnapshotID:     "snap-" + now.UTC().Format("20060102T150405Z"),
		AccountID:      accountID,
		ScannedAt:      now.UTC(),
		ScannedRegions: append([]string(nil), regions...),
		FailedRegions:  []FailedRegion{},
	}
	for _, region := range regions {
		sink.RegionStarted(region)
		tagged, err := reader.ListTaggedResources(ctx, region)
		if err != nil {
			sink.RegionFailed(region, errkind.KindOf(err).String())
			return nil, err
		}
		for _, t := range tagged {
			snap.Resources = append(snap.Resources, Resource{
				ARN:          t.ARN,
				Region:       region,
				ResourceType: ResourceTypeOf(t.ARN),
				Tags:         t.Tags,
			})
		}
		sink.RegionDone(region, len(tagged))
	}
	return snap, nil
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

// NormalizeKey lowercases a key and removes the separators "-", "_" and
// space (BR2.2).
func NormalizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', ' ':
			return -1
		}
		return r
	}, strings.ToLower(key))
}

// Spelling is one way a key is written, with the number of resources that
// carry it.
type Spelling struct {
	Key           string
	ResourceCount int
}

// KeyVariantGroup holds the keys that normalize to the same key (BR2.3).
type KeyVariantGroup struct {
	NormalizedKey string
	// Spellings are sorted by count, highest first, then by key.
	Spellings []Spelling
	// TotalResources counts each resource once, even when it carries more
	// than one spelling.
	TotalResources int
	// IsVariant is true when the group has two or more spellings.
	IsVariant bool
}

// GroupKeys groups the tag keys of the resources by their normalized form,
// leaving out aws: keys (BR2.2, BR2.3). Groups are sorted by total resources,
// highest first, then by normalized key.
func GroupKeys(resources []Resource) []KeyVariantGroup {
	spellings := map[string]map[string]int{}
	totals := map[string]int{}
	for _, r := range resources {
		counted := make(map[string]bool, len(r.Tags))
		for key := range r.Tags {
			if IsAWSReservedKey(key) {
				continue
			}
			norm := NormalizeKey(key)
			if spellings[norm] == nil {
				spellings[norm] = map[string]int{}
			}
			spellings[norm][key]++
			if !counted[norm] {
				counted[norm] = true
				totals[norm]++
			}
		}
	}
	groups := make([]KeyVariantGroup, 0, len(spellings))
	for norm, keys := range spellings {
		g := KeyVariantGroup{NormalizedKey: norm, TotalResources: totals[norm], IsVariant: len(keys) >= 2}
		for key, n := range keys {
			g.Spellings = append(g.Spellings, Spelling{Key: key, ResourceCount: n})
		}
		sort.Slice(g.Spellings, func(i, j int) bool {
			a, b := g.Spellings[i], g.Spellings[j]
			if a.ResourceCount != b.ResourceCount {
				return a.ResourceCount > b.ResourceCount
			}
			return a.Key < b.Key
		})
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalResources != groups[j].TotalResources {
			return groups[i].TotalResources > groups[j].TotalResources
		}
		return groups[i].NormalizedKey < groups[j].NormalizedKey
	})
	return groups
}

// Variants returns only the groups with two or more spellings, in order.
func Variants(groups []KeyVariantGroup) []KeyVariantGroup {
	var out []KeyVariantGroup
	for _, g := range groups {
		if g.IsVariant {
			out = append(out, g)
		}
	}
	return out
}
