package apply

import (
	"encoding/json"
	"maps"
	"slices"
	"sort"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
)

// The sizes of the work (NFR5.13). They are variables so that tests can make
// them smaller; users cannot change them (U6 Q2).
var (
	// regionWorkers is the number of regions written at once.
	regionWorkers = 4
	// batchSize is the most resources one write call takes.
	batchSize = awsaccess.MaxWriteARNs
	// unitSize is the most resources one read takes: a read-again unit.
	unitSize = awsaccess.MaxReadARNs
	// transientLimit is the number of write calls in a row, over every
	// region, that may end Transient before the run stops (NFR5.17).
	transientLimit = 10
)

// batch is the resources of a region with the same tags to set, the same
// keys to remove and the same order, written by one call of each kind
// (logical-components.md).
type batch struct {
	key     string
	targets []*Target
}

// unit is consecutive batches of a region, at most unitSize resources: read
// again before writing and read back after (BR4.7, BR4.3).
type unit struct {
	region  string
	batches []batch
}

// targets returns the resources of the unit.
func (u unit) targets() []*Target {
	var out []*Target
	for _, b := range u.batches {
		out = append(out, b.targets...)
	}
	return out
}

// groupKey is the key of the batches of a target: what it sets, as sorted
// [key, value] pairs, what it removes and whether it removes first, encoded
// as JSON, so keys and values with =, commas, line breaks or quotes can never
// make two different writes look the same.
func groupKey(t *Target) string {
	var k struct {
		Set         [][2]string `json:"set"`
		Remove      []string    `json:"remove"`
		RemoveFirst bool        `json:"removeFirst"`
	}
	for _, key := range slices.Sorted(maps.Keys(t.TagsToSet)) {
		k.Set = append(k.Set, [2]string{key, t.TagsToSet[key]})
	}
	k.Remove, k.RemoveFirst = t.KeysToRemove, t.RemoveFirst
	data, err := json.Marshal(k)
	if err != nil {
		panic(err) // strings and a bool only
	}
	return string(data)
}

// planUnits groups the targets of each region by groupKey, splits each group
// in ARN order into batches of batchSize, orders the batches by key and
// first ARN, and packs consecutive batches into units of at most unitSize
// resources. The regions come back sorted.
func planUnits(targets []*Target) (regions []string, units map[string][]unit) {
	groups := map[string]map[string][]*Target{}
	for _, t := range targets {
		if groups[t.Region] == nil {
			groups[t.Region] = map[string][]*Target{}
		}
		key := groupKey(t)
		groups[t.Region][key] = append(groups[t.Region][key], t)
	}
	units = map[string][]unit{}
	for region, byKey := range groups {
		var batches []batch
		for _, key := range slices.Sorted(maps.Keys(byKey)) {
			ts := byKey[key]
			sort.Slice(ts, func(i, j int) bool { return ts[i].ARN < ts[j].ARN })
			for start := 0; start < len(ts); start += batchSize {
				batches = append(batches, batch{key: key, targets: ts[start:min(start+batchSize, len(ts))]})
			}
		}
		var current unit
		size := 0
		for _, b := range batches {
			if size+len(b.targets) > unitSize && size > 0 {
				units[region] = append(units[region], current)
				current, size = unit{}, 0
			}
			current.region = region
			current.batches = append(current.batches, b)
			size += len(b.targets)
		}
		if size > 0 {
			units[region] = append(units[region], current)
		}
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions, units
}

// chunks splits arns into lists of at most n.
func chunks(arns []string, n int) [][]string {
	var out [][]string
	for start := 0; start < len(arns); start += n {
		out = append(out, arns[start:min(start+n, len(arns))])
	}
	return out
}
