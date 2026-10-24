package display

import (
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// The JSON and Markdown formats of scan (BR3.4, BR3.5, contract C3).

type jsonFailedRegion struct {
	Region string `json:"region"`
	Reason string `json:"reason"`
}

// failedRegionsJSON writes the failedRegions field that scan and plan share.
func failedRegionsJSON(o *planner.JSONObject, failed []inventory.FailedRegion) {
	o.Array("failedRegions", len(failed), func(i int) any {
		f := failed[i]
		return jsonFailedRegion{Region: f.Region, Reason: f.Kind + " (" + f.Operation + ")"}
	})
}

type jsonVariant struct {
	Spelling      string `json:"spelling"`
	ResourceCount int    `json:"resourceCount"`
}

type jsonVariantGroup struct {
	Target string `json:"target"`
	// NormalizedKey is the key the spellings are grouped by.
	NormalizedKey    string        `json:"normalizedKey"`
	AbbreviationOnly bool          `json:"abbreviationOnly"`
	Variants         []jsonVariant `json:"variants"`
}

type jsonMissing struct {
	Key  string   `json:"key"`
	ARNs []string `json:"arns"`
}

// untaggedDiscovery is the method of contract C3: awsConfig when AWS Config
// answered for a region, resourceExplorer when only Resource Explorer did,
// unavailable otherwise. The reason says which regions were not checked.
func untaggedDiscovery(d *inventory.Discovery) (string, string) {
	if d == nil {
		return "unavailable", T(English, "scope.notLookedFor")
	}
	reason := ""
	if d.Status != inventory.StatusComplete {
		reason = discoveryState(English, d)
	}
	config, explorer, aggregator, _ := d.MethodCounts()
	switch {
	case d.Status == inventory.StatusUnavailable:
		return "unavailable", reason
	case config > 0:
		return "awsConfig", reason
	case explorer > 0 || aggregator > 0:
		return "resourceExplorer", reason
	}
	return "unavailable", reason
}

func (r *results) scanJSON(rep *inventory.ScanReport, out Output) {
	snap := rep.Snapshot
	w := planner.NewSafeJSONWriter(r.w)
	o := planner.NewJSONObject(w, "")
	o.Field("version", 1)
	o.Field("toolVersion", out.ToolVersion)
	o.Field("command", "scan")
	o.Field("exitCode", out.ExitCode)
	o.Field("accountId", snap.AccountID)
	method, reason := untaggedDiscovery(snap.Discovery)
	o.Object("complete", func(c *planner.JSONObject) {
		c.Field("inventory", snap.Complete())
		c.Field("untaggedDiscovery", method)
		if reason != "" {
			c.Field("untaggedDiscoveryReason", reason)
		}
	})
	failedRegionsJSON(o, snap.FailedRegions)
	types := planner.TypeCountsJSON(rep.Unsupported)
	o.Array("unsupportedResourceTypes", len(types), func(i int) any { return types[i] })
	o.Object("summary", func(c *planner.JSONObject) {
		c.Field("scannedResources", len(snap.Resources))
		c.Field("variantGroups", inventory.CountVariants(rep.Groups))
		if rep.Missing != nil {
			missing := map[string]int{}
			for _, m := range rep.Missing {
				missing[m.Key] = m.ResourceCount
			}
			c.Field("missingRequired", missing)
		}
	})
	variants := inventory.Variants(rep.Groups)
	o.Object("details", func(d *planner.JSONObject) {
		d.Array("variantGroups", len(variants), func(i int) any {
			g := variants[i]
			j := jsonVariantGroup{Target: "key", NormalizedKey: g.NormalizedKey, AbbreviationOnly: g.AbbreviationOnly, Variants: []jsonVariant{}}
			for _, s := range g.Spellings {
				j.Variants = append(j.Variants, jsonVariant{Spelling: s.Key, ResourceCount: s.ResourceCount})
			}
			return j
		})
		if rep.Missing != nil {
			d.Array("missingRequired", len(rep.Missing), func(i int) any {
				m := rep.Missing[i]
				return jsonMissing{Key: m.Key, ARNs: nonNilStrings(m.ARNs)}
			})
		}
	})
	_ = o.Close()
	_, _ = w.Write([]byte("\n"))
	_ = w.Close()
}

func (r *results) scanMarkdown(rep *inventory.ScanReport) {
	snap := rep.Snapshot
	r.mdLine("md.target")
	r.mdLine("md.scanTarget", mdSpan(snap.AccountID), mdCount(snap.DoneRegions()), mdCount(len(snap.EnabledRegions)),
		mdTool(planner.FormatTime(snap.ScannedAt.Truncate(time.Second))))
	r.blank()
	r.mdIncompleteRegions(snap.FailedRegions)
	if variants := inventory.Variants(rep.Groups); len(variants) > 0 {
		r.mdLine("md.variants")
		r.mdLine("md.variantsNote")
		r.blank()
		r.mdTable("md.variantHead")
		for _, g := range variants {
			group := mdArg(mdCell(g.NormalizedKey))
			if g.AbbreviationOnly {
				group = mdTool(r.tr("md.abbreviation", mdCell(g.NormalizedKey).mdText()))
			}
			for _, s := range g.Spellings {
				r.mdRow(group, mdCell(s.Key), mdCount(s.ResourceCount))
			}
		}
		r.blank()
	}
	r.mdLine("md.required")
	switch {
	case rep.Missing == nil:
		r.mdLine("md.noRules")
	case rep.MissingKeys() == 0:
		r.mdLine("md.requiredFrom", mdSpan(rep.RulesPath))
		r.blank()
		r.mdLine("md.none")
	default:
		r.mdLine("md.requiredFrom", mdSpan(rep.RulesPath))
		r.blank()
		r.mdTable("md.notTaggable")
		for _, m := range rep.Missing {
			if m.ResourceCount > 0 || m.NotTaggableCount > 0 {
				r.mdRow(mdCell(m.Key), mdCount(m.ResourceCount), mdCount(m.NotTaggableCount))
			}
		}
	}
	r.blank()
	r.mdLine("md.neverTagged")
	r.mdLine("scan.noUserTags", mdCount(rep.NeverTagged))
	r.blank()
	if d := snap.Discovery; d != nil {
		// The regions that were not checked are in the table format and the
		// JSON; here the status word of the tool is enough.
		r.mdLine("scan.discovery", mdTool(statusWord(r.lang, d.Status)))
		r.blank()
	}
	if len(rep.Unsupported) > 0 {
		r.mdLine("md.unsupported")
		r.mdTable("md.typeHead")
		for _, t := range rep.Unsupported {
			r.mdRow(mdCell(t.ResourceType), mdCount(t.Count))
		}
		r.blank()
	}
	r.mdLine("md.summary")
	_, _ = r.w.WriteString(r.scanSummary(rep) + "\n")
}
