package inventory

import (
	"context"
	"errors"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The search for resources that were never tagged (BR3.1 to BR3.3,
// reliability design §3). The tagging API does not return them, so they are
// looked for through AWS Config and Resource Explorer, region by region.

// Discovery statuses: of the whole run (complete, partial, unavailable) and of
// one region (complete, unavailable, failed).
const (
	StatusComplete    = "complete"
	StatusPartial     = "partial"
	StatusUnavailable = "unavailable"
	StatusFailed      = "failed"
)

// Discovery methods (UntaggedDiscoveryRegion.methods).
const (
	MethodConfig     = "awsConfig"
	MethodExplorer   = "resourceExplorer"
	MethodAggregator = "resourceExplorerAggregator"
)

// DiscoveryRegion is the search in one region.
type DiscoveryRegion struct {
	Region string
	// Methods are the methods whose results were used, in the order tried.
	Methods []string
	// Status is StatusComplete, StatusUnavailable or StatusFailed.
	Status string
	// Reason says why the region is unavailable or failed, from fixed words
	// and IAM action names only; never AWS error text.
	Reason string
	// Found is the number of never-tagged resources kept for the region.
	Found int
}

// Discovery is the search for never-tagged resources in every scanned region.
type Discovery struct {
	// Status is StatusComplete, StatusPartial or StatusUnavailable (BR3.2).
	Status  string
	Regions []DiscoveryRegion
}

// Fixed reasons of the discovery (observability design §4).
const (
	reasonConfigNotRecording = "AWS Config not recording"
	reasonConfigSomeTypes    = "AWS Config records only some resource types"
	reasonNoIndex            = "no Resource Explorer index"
	reasonNoDefaultView      = "no default Resource Explorer view"
	reasonViewFiltered       = "the default Resource Explorer view has a filter"
)

// configMode is what AWS Config records in a region.
type configMode int

const (
	configNone configMode = iota // no recorder is recording
	configSome                   // recording, but not every type
	configAll                    // recording every supported type
)

// regionOutcome is the search in one region with what it found.
type regionOutcome struct {
	DiscoveryRegion
	config   []awsaccess.ConfigResource
	explorer []awsaccess.ExplorerResource
	// covered is true when the aggregator index already searched the region.
	covered bool
}

type discoverer struct {
	config   awsaccess.ConfigReader
	explorer awsaccess.ExplorerReader
	sink     ProgressSink
	fatal    *fatalError
	// indexed are the regions with a Resource Explorer index.
	indexed map[string]bool
	// indexReason is set when the indexes could not be listed.
	indexReason string
}

// Discover looks for never-tagged resources in the scanned regions of snap
// and adds them to it, with everTagged false and no user tags (BR3.1). Where
// Resource Explorer has an aggregator index, it is asked once for every
// indexed region; the other regions try AWS Config, then Resource Explorer,
// at most Concurrency at once. A region that cannot be searched is recorded
// in snap.Discovery and changes nothing else (BR3.2). Only missing or expired
// credentials stop the search, and then the whole run (BR4.3).
func Discover(ctx context.Context, config awsaccess.ConfigReader, explorer awsaccess.ExplorerReader, sink ProgressSink, snap *Snapshot) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d := &discoverer{config: config, explorer: explorer, sink: sink, fatal: &fatalError{cancel: cancel}}
	regions := snap.ScannedRegions
	sink.DiscoveryStarted(regions)
	aggregator := d.listIndexes(ctx)
	outcomes := make([]regionOutcome, len(regions))
	if aggregator != "" {
		d.searchAggregator(ctx, aggregator, regions, outcomes)
	}
	if err := d.fatal.get(); err != nil {
		return err
	}
	g := new(errgroup.Group)
	g.SetLimit(Concurrency)
	for i, region := range regions {
		if outcomes[i].covered {
			continue
		}
		g.Go(func() error {
			outcomes[i] = d.searchRegion(ctx, region)
			if ctx.Err() == nil {
				sink.DiscoveryRegionDone(region, outcomes[i].Status)
			}
			return nil
		})
	}
	_ = g.Wait()
	if err := d.fatal.get(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	addDiscovered(snap, outcomes)
	return nil
}

// stop records err as fatal when it ends the run: credentials, or a run
// already cancelled. It reports whether the caller must stop.
func (d *discoverer) stop(ctx context.Context, err error) bool {
	if errkind.KindOf(err) == errkind.CredentialsMissing || ctx.Err() != nil {
		d.fatal.set(err)
		return true
	}
	return false
}

// listIndexes reads the Resource Explorer indexes and returns the region of
// the aggregator index, if any.
func (d *discoverer) listIndexes(ctx context.Context) string {
	d.indexed = map[string]bool{}
	indexes, err := d.explorer.ExplorerIndexes(ctx)
	if err != nil {
		if !d.stop(ctx, err) {
			d.indexReason = failureReason(err)
		}
		return ""
	}
	aggregator := ""
	for _, ix := range indexes {
		d.indexed[ix.Region] = true
		if ix.Aggregator {
			aggregator = ix.Region
		}
	}
	return aggregator
}

// searchAggregator searches every indexed region once through the
// aggregator index and shares the results out by region. When the aggregator
// cannot be used, the regions are searched one by one instead.
func (d *discoverer) searchAggregator(ctx context.Context, aggregator string, regions []string, outcomes []regionOutcome) {
	view, found, err := d.explorerSearch(ctx, aggregator)
	if err != nil {
		return
	}
	status, reason := StatusComplete, ""
	if view.Filter != "" {
		status, reason = StatusFailed, reasonViewFiltered
	}
	byRegion := map[string][]awsaccess.ExplorerResource{}
	var global []awsaccess.ExplorerResource
	for _, r := range found {
		if arnRegion(r.ARN) == "" {
			global = append(global, r)
			continue
		}
		byRegion[r.Region] = append(byRegion[r.Region], r)
	}
	first := true
	for i, region := range regions {
		if !d.indexed[region] {
			continue
		}
		o := regionOutcome{DiscoveryRegion: DiscoveryRegion{Region: region, Methods: []string{MethodAggregator},
			Status: status, Reason: reason}, explorer: byRegion[region], covered: true}
		if first {
			// Global resources belong to no region; the first region in name
			// order holds them (BR2.2).
			o.explorer = append(o.explorer, global...)
			first = false
		}
		outcomes[i] = o
		d.sink.DiscoveryRegionDone(region, status)
	}
}

// searchRegion searches one region: AWS Config first, then Resource
// Explorer (reliability design §3).
func (d *discoverer) searchRegion(ctx context.Context, region string) regionOutcome {
	o := regionOutcome{DiscoveryRegion: DiscoveryRegion{Region: region}}
	var reasons []string
	mode, err := d.configMode(ctx, region)
	configOK := false
	switch {
	case err != nil:
		if d.stop(ctx, err) {
			return o
		}
		reasons = append(reasons, failureReason(err))
	case mode == configNone:
		reasons = append(reasons, reasonConfigNotRecording)
	default:
		o.config, err = d.config.ConfigResources(ctx, region)
		if err != nil {
			if d.stop(ctx, err) {
				return o
			}
			reasons = append(reasons, failureReason(err))
		} else {
			configOK = true
			o.Methods = append(o.Methods, MethodConfig)
		}
	}
	if configOK && mode == configAll {
		o.Status = StatusComplete
		return o
	}
	if configOK {
		reasons = append(reasons, reasonConfigSomeTypes)
	}
	view, found, err := d.explorerSearch(ctx, region)
	switch {
	case err != nil:
		if d.fatal.get() != nil {
			return o
		}
		reasons = append(reasons, explorerReason(err))
	case view.Filter != "":
		o.explorer = found
		o.Methods = append(o.Methods, MethodExplorer)
		o.Status, o.Reason = StatusFailed, reasonViewFiltered
		return o
	default:
		o.explorer = found
		o.Methods = append(o.Methods, MethodExplorer)
		o.Status = StatusComplete
		return o
	}
	o.Status = StatusUnavailable
	if configOK {
		o.Status = StatusFailed
	}
	o.Reason = strings.Join(reasons, ", ")
	return o
}

// configMode reads the recorders of the region and tells what they record.
func (d *discoverer) configMode(ctx context.Context, region string) (configMode, error) {
	recorders, err := d.config.ConfigRecorders(ctx, region)
	if err != nil {
		return configNone, err
	}
	mode := configNone
	for _, r := range recorders {
		if !r.Recording {
			continue
		}
		if recordsAllTypes(r) {
			return configAll, nil
		}
		mode = configSome
	}
	return mode, nil
}

// recordsAllTypes applies the first row of the table of reliability design
// §3: allSupported (with the strategy ALL_SUPPORTED_RESOURCE_TYPES or none),
// or EXCLUSION_BY_RESOURCE_TYPES with nothing excluded.
// includeGlobalResourceTypes is not used: the global IAM types are left out of
// the discovery anyway.
func recordsAllTypes(r awsaccess.ConfigRecorder) bool {
	switch r.Strategy {
	case "", "ALL_SUPPORTED_RESOURCE_TYPES":
		return r.AllSupported
	case "EXCLUSION_BY_RESOURCE_TYPES":
		return r.ExcludedTypes == 0
	default:
		return false
	}
}

// explorerSearch reads the default view of the region and the resources
// without tags. Fatal errors are recorded and returned too.
func (d *discoverer) explorerSearch(ctx context.Context, region string) (awsaccess.ExplorerView, []awsaccess.ExplorerResource, error) {
	if d.indexReason != "" {
		return awsaccess.ExplorerView{}, nil, &explorerUnavailable{reason: d.indexReason}
	}
	if !d.indexed[region] {
		return awsaccess.ExplorerView{}, nil, &explorerUnavailable{reason: reasonNoIndex}
	}
	view, err := d.explorer.DefaultView(ctx, region)
	if err != nil {
		d.stop(ctx, err)
		return view, nil, err
	}
	found, err := d.explorer.UntaggedResources(ctx, region, view.ARN)
	if err != nil {
		d.stop(ctx, err)
		return view, nil, err
	}
	return view, found, nil
}

// explorerUnavailable is a region where Resource Explorer cannot be asked.
type explorerUnavailable struct{ reason string }

func (e *explorerUnavailable) Error() string { return e.reason }

// explorerReason words a Resource Explorer failure. A missing default view
// and a view that ListResources may not use (401, shown with the IAM action
// resource-explorer-2:Search) are told apart (security design §4).
func explorerReason(err error) string {
	var u *explorerUnavailable
	if errors.As(err, &u) {
		return u.reason
	}
	e, ok := errkind.As(err)
	switch {
	case ok && e.Reason == errkind.ReasonNoDefaultView:
		return reasonNoDefaultView
	case ok && e.Kind == errkind.AccessDenied && e.Op == awsaccess.OpListResources:
		return "default Resource Explorer view not usable (" + failureReason(err) + ")"
	default:
		return failureReason(err)
	}
}

// failureReason words a failed call as its error kind and the IAM action, for
// example "AccessDenied config:SelectResourceConfig" (NFR8.10).
func failureReason(err error) string {
	e, ok := errkind.As(err)
	if !ok {
		return "unexpected error"
	}
	op := awsaccess.IAMAction(e.Op)
	if e.Kind == errkind.Internal {
		return "unexpected answer from " + op
	}
	return e.Kind.String() + " " + op
}

// addDiscovered adds the never-tagged resources to the snapshot, region by
// region in name order (NFR5.7). Resources the tagging API returned are left
// out, as are resources with user tags and IAM's global types; when AWS
// Config and Resource Explorer return the same ARN, AWS Config's resource is
// kept (BR3.1).
func addDiscovered(snap *Snapshot, outcomes []regionOutcome) {
	known := make(map[string]bool, len(snap.Resources))
	for _, r := range snap.Resources {
		known[r.ARN] = true
	}
	found := map[string]int{} // key -> index in added
	var added []Resource
	keep := func(key string, r Resource, fromConfig bool) {
		if i, ok := found[key]; ok {
			if fromConfig {
				added[i] = r
			}
			return
		}
		found[key] = len(added)
		added = append(added, r)
	}
	for _, o := range outcomes {
		for _, c := range o.config {
			if c.ARN != "" && known[c.ARN] || hasUserTags(c.Tags) || isIAMType(c.ResourceType) {
				continue
			}
			key := c.ARN
			if key == "" {
				// Without an ARN, the region, type and identifier tell
				// resources apart.
				key = "\x00" + o.Region + "\x00" + c.ResourceType + "\x00" + c.ResourceID
			}
			keep(key, configResource(c, o.Region), true)
		}
		for _, e := range o.explorer {
			if known[e.ARN] || isIAMType(e.ResourceType) {
				continue
			}
			keep(e.ARN, explorerResource(e, o.Region), false)
		}
	}
	counts := map[string]int{}
	for _, r := range added {
		counts[r.Region]++
	}
	disc := &Discovery{Regions: make([]DiscoveryRegion, 0, len(outcomes))}
	for _, o := range outcomes {
		o.Found = counts[o.Region]
		disc.Regions = append(disc.Regions, o.DiscoveryRegion)
	}
	disc.Status = overallStatus(disc.Regions)
	snap.Discovery = disc
	snap.Resources = append(snap.Resources, added...)
	sortResources(snap.Resources)
}

func configResource(c awsaccess.ConfigResource, region string) Resource {
	resourceType := ConfigTypeToARNForm(c.ResourceType)
	r := Resource{ARN: c.ARN, Region: region, ResourceType: resourceType, Tags: c.Tags,
		Global: c.ARN != "" && arnRegion(c.ARN) == "", NotTaggable: c.ARN == "" || IsNotTaggable(resourceType)}
	if r.Tags == nil {
		r.Tags = map[string]string{}
	}
	if stack := c.Tags[cloudFormationStackKey]; stack != "" {
		r.IaCManagedBy = "cloudformation:" + stack
	}
	return r
}

func explorerResource(e awsaccess.ExplorerResource, region string) Resource {
	return Resource{ARN: e.ARN, Region: region, ResourceType: e.ResourceType, Tags: map[string]string{},
		Global: arnRegion(e.ARN) == "", NotTaggable: IsNotTaggable(e.ResourceType)}
}

// overallStatus is complete when every region is complete, unavailable when
// none could be searched at all, partial otherwise (BR3.2).
func overallStatus(regions []DiscoveryRegion) string {
	complete, unavailable := 0, 0
	for _, r := range regions {
		switch r.Status {
		case StatusComplete:
			complete++
		case StatusUnavailable:
			unavailable++
		}
	}
	switch {
	case complete == len(regions):
		return StatusComplete
	case unavailable == len(regions):
		return StatusUnavailable
	default:
		return StatusPartial
	}
}

// NotChecked returns the regions whose discovery is not complete, grouped by
// reason: each group lists its regions in name order, and groups come in the
// order of their first region.
func (d *Discovery) NotChecked() []ReasonGroup {
	index := map[string]int{}
	var groups []ReasonGroup
	for _, r := range d.Regions {
		if r.Status == StatusComplete {
			continue
		}
		i, ok := index[r.Reason]
		if !ok {
			i = len(groups)
			index[r.Reason] = i
			groups = append(groups, ReasonGroup{Reason: r.Reason})
		}
		groups[i].Regions = append(groups[i].Regions, r.Region)
	}
	return groups
}

// ReasonGroup is a reason and the regions it applies to.
type ReasonGroup struct {
	Reason  string
	Regions []string
}

// MethodCounts counts the regions by the methods that searched them, for the
// line that says what was checked (BR5.7).
func (d *Discovery) MethodCounts() (config, explorer, aggregator, notChecked int) {
	for _, r := range d.Regions {
		if r.Status != StatusComplete {
			notChecked++
		}
		if slices.Contains(r.Methods, MethodConfig) {
			config++
		}
		if slices.Contains(r.Methods, MethodExplorer) {
			explorer++
		}
		if slices.Contains(r.Methods, MethodAggregator) {
			aggregator++
		}
	}
	return config, explorer, aggregator, notChecked
}
