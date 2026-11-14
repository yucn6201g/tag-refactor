package iac

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// The CDK apps in the flow of iac (v0.2.0 U1, functional spec §2). Before the
// templates are looked for, the first app below --dir is synthesized, the
// targets are matched with its resources (BR2.1), and the helper reads its
// sources once (BR2.2). Each change of a matched target is decided on the tag
// sites whose scope holds the resource (BR3.1 to BR3.3), placed in the file of
// the key, and edited there with the code of the templates: simulate,
// buildEdits, the difference and the writing. After the edits, the helper
// reads the app again with the edited contents (BR3.4).
//
// U1 reflects the rename of a key written as a string in Tags.of(x).add; the
// other kinds of change and the other ways of writing go on the list to fix
// by hand with cdkUnsupportedShape (U4 takes them).

// cdkTarget is what a target matched with synth carries.
type cdkTarget struct {
	run      *cdkRun
	resource cdk.SynthesizedResource
}

// cdkRun is one app synthesized and read.
type cdkRun struct {
	app      cdk.App
	syn      cdk.Synthesis
	analysis cdk.Analysis
}

// rel returns a file of the app relative to --dir.
func (c *cdkRun) rel(file string) string { return path.Join(c.app.Dir, file) }

// cdkTask is the work on one source file of an app.
type cdkTask struct {
	run   *cdkRun
	file  string // relative to the app
	c     *candidate
	parts []*cdkPart
}

// cdkPart is a target with its changes placed in one file.
type cdkPart struct {
	// t is a copy of the target with the changes of the file.
	t     *target
	at    place
	sites []cdk.TagSite
	// riders are, by change ID of t, the same change of other targets that
	// one edit reflects too (contract C7 same_site_rule).
	riders map[string][]rider
}

// rider is a change of another target that an edit of a part reflects.
type rider struct {
	t *target
	c planner.Change
}

// cdkDecision is a change of a target and the place it is decided at.
type cdkDecision struct {
	t          *target
	c          planner.Change
	file       string // relative to the app
	line       int
	editAt     string // the range of the key, when the key is in the source
	sites      []cdk.TagSite
	ridesOn    *cdkDecision
	passengers []rider
}

// cdkReflected is a change an edit makes, with its part.
type cdkReflected struct {
	part *cdkPart
	c    planner.Change
}

// cdkOutcome is what the check after the edits needs of a file.
type cdkOutcome struct {
	run  *cdkRun
	file string
	// expect gives, by range of a key in the file read first, the key after
	// the edits.
	expect    map[string]string
	reflected []cdkReflected
}

func (r *runner) sink() cdk.ProgressSink {
	if r.req.Progress == nil {
		return cdk.NopSink{}
	}
	return r.req.Progress
}

// prepareCDK finds, synthesizes and reads the app, and matches the targets
// made by CloudFormation with its resources. It returns the work on the
// sources. Nothing runs when no target was made by CloudFormation (BR1.1,
// NFR5).
func (r *runner) prepareCDK(ctx context.Context, targets []*target) ([]*cdkTask, error) {
	var stacked []*target
	for _, t := range targets {
		if t.tool == ToolCloudFormation {
			stacked = append(stacked, t)
		}
	}
	if len(stacked) == 0 || r.w == nil || r.w.synth == nil {
		return nil, nil
	}
	apps, err := r.w.synth.Discover(ctx, r.root, nil)
	if err != nil || len(apps) == 0 {
		return nil, err
	}
	run, entry, err := r.synthesizeFirst(ctx, apps)
	if err != nil || run == nil {
		r.res.CDKApps = append(r.res.CDKApps, entry)
		return nil, err
	}
	matched := r.matchTargets(stacked, run)
	if len(matched) == 0 {
		r.res.CDKApps = append(r.res.CDKApps, entry)
		return nil, nil
	}
	paths := make([]string, 0, len(matched))
	for _, t := range matched {
		paths = append(paths, t.cdk.resource.ConstructPath)
	}
	run.analysis, err = r.w.analyzer.Analyze(ctx, run.app, cdk.AnalyzeRequest{ConstructPaths: paths})
	r.res.Debug = append(r.res.Debug, run.analysis.Debug...)
	if err != nil {
		var hf *cdk.HelperFailure
		if !errors.As(err, &hf) {
			return nil, helperError(err)
		}
		entry.Status, entry.FailureReason, entry.Tool = CDKAppFailed, "helper", hf.Tool
		r.res.CDKApps = append(r.res.CDKApps, entry)
		r.sink().AppFailed(run.app.Dir, run.syn.Elapsed, "helper")
		for _, t := range matched {
			manualAll(r.res, t, ReasonCDKAppFailed, DetailTool, hf.Tool)
		}
		return nil, nil
	}
	r.res.CDKApps = append(r.res.CDKApps, entry)
	return r.planCDK(run, matched), nil
}

// synthesizeFirst synthesizes the first app in name order; U1 skips the others
// (scalability design). The run is nil when the app was not synthesized.
func (r *runner) synthesizeFirst(ctx context.Context, apps []cdk.App) (*cdkRun, CDKApp, error) {
	sink := r.sink()
	first := apps[0]
	if first.Synthesizable() {
		sink.AppsWaiting([]string{first.Dir})
	}
	for _, a := range apps[1:] {
		sink.AppSkipped(a.Dir, ReasonNotInU1)
		r.res.CDKApps = append(r.res.CDKApps, CDKApp{Dir: a.Dir, Language: string(a.Language), Status: CDKAppSkipped,
			FailureReason: ReasonNotInU1})
	}
	syn, err := r.w.synth.Synthesize(ctx, first, cdk.Options{}, sink)
	r.res.Debug = append(r.res.Debug, syn.Debug...)
	entry := CDKApp{Dir: first.Dir, Language: string(first.Language), Status: CDKAppDone, Elapsed: syn.Elapsed}
	if err != nil {
		return nil, entry, err
	}
	if f := syn.Failure; f != nil {
		entry.Status, r.cdkReason = failureStatus(f.Reason)
		entry.FailureReason, entry.Tool = string(f.Reason), f.Tool
		if r.cdkReason == ReasonCDKAppFailed {
			r.cdkTool = f.Tool
		}
		return nil, entry, nil
	}
	return &cdkRun{app: first, syn: syn}, entry, nil
}

// failureStatus maps a failure of synth to the state of the app and the reason
// of its resources (contract C7 failure_to_manual).
func failureStatus(reason cdk.FailureReason) (CDKAppStatus, ManualReason) {
	switch reason {
	case cdk.ReasonCDKV1, cdk.ReasonUnknownVersion:
		return CDKAppSkipped, ReasonCDKV1
	case cdk.ReasonUnsupportedLanguage:
		return CDKAppSkipped, ReasonCDKUnsupportedLanguage
	}
	return CDKAppFailed, ReasonCDKAppFailed
}

// helperError turns an error of the helper that is not a failure of the app
// into the error of the run: a result of another version is a fault of the
// build, and stops iac before anything is written (BR1.6).
func helperError(err error) error {
	if errors.Is(err, cdk.ErrHelperVersion) {
		return &errkind.Error{Kind: errkind.Internal, Op: "iac", Detail: "the embedded CDK helper answered with another version", Cause: err}
	}
	return err
}

// matchTargets matches the targets with the resources of synth by stack name,
// logical ID, account and region (BR2.1, contract C7 match_rule). An axis that
// is empty on either side is not used. A target with no candidate stays a
// CloudFormation target; one with several goes on the list (cdkAmbiguous).
func (r *runner) matchTargets(targets []*target, run *cdkRun) []*target {
	var matched []*target
	for _, t := range targets {
		var found []cdkTarget
		for _, st := range run.syn.Stacks {
			if st.Name != t.stackName || st.Account != "" && t.account != "" && st.Account != t.account ||
				st.Region != "" && !t.global && t.region != "" && st.Region != t.region {
				continue
			}
			for _, res := range st.Resources {
				if res.LogicalID == t.logicalID {
					found = append(found, cdkTarget{run: run, resource: res})
				}
			}
		}
		switch len(found) {
		case 0:
		case 1:
			t.tool, t.cdk = ToolCDK, &found[0]
			matched = append(matched, t)
		default:
			t.tool = ToolCDK
			manualAll(r.res, t, ReasonCDKAmbiguous, DetailNone, "")
		}
	}
	return matched
}

// cdkPlanner decides the changes of the matched targets of one app.
type cdkPlanner struct {
	r         *runner
	run       *cdkRun
	matched   []*target
	decisions []*cdkDecision
}

// planCDK decides every change of the matched targets and gathers them by file.
func (r *runner) planCDK(run *cdkRun, matched []*target) []*cdkTask {
	p := &cdkPlanner{r: r, run: run, matched: matched}
	for _, u := range run.analysis.Unsupported {
		if u.Reason == cdk.ReasonParseError {
			r.res.SkippedFiles = append(r.res.SkippedFiles, SkippedFile{File: run.rel(u.File), Reason: SkipParseError,
				Line: u.Line, Kind: "typescript"})
		}
	}
	for _, t := range matched {
		p.planTarget(t)
	}
	p.shareEdits()
	return p.tasks()
}

// manual puts one change on the list to fix by hand at a place of the app.
func (p *cdkPlanner) manual(t *target, c planner.Change, reason ManualReason, file string, line int) {
	p.r.res.Manual = append(p.r.res.Manual, ManualItem{ARN: t.arn, ChangeID: c.ChangeID, Key: changeKey(c),
		File: p.run.rel(file), Line: line, Reason: reason})
}

// planTarget decides the changes of one target (BR2.2, BR3.1, BR3.2).
func (p *cdkPlanner) planTarget(t *target) {
	resPath := t.cdk.resource.ConstructPath
	cs, ok := p.constructSite(resPath)
	if !ok {
		manualAll(p.r.res, t, ReasonCDKNoSource, DetailNone, "")
		return
	}
	p.r.res.Debug = append(p.r.res.Debug, redact.String(fmt.Sprintf("cdk: %s/%s is made at %s:%d (%s)",
		t.stackName, t.logicalID, p.run.rel(cs.File), cs.Line, cs.ConstructPath)))
	var sites []cdk.TagSite
	for _, s := range p.run.analysis.TagSites {
		if under(resPath, s.ScopeConstructPath) {
			sites = append(sites, s)
		}
	}
	for _, s := range sites {
		if s.Op != "add" {
			for _, c := range t.changes {
				p.manual(t, c, ReasonCDKUnsupportedShape, s.UseFile, s.UseLine)
			}
			return
		}
	}
	for _, c := range t.changes {
		p.planChange(t, c, cs, sites)
	}
}

// planChange decides one change: at the one site of its key, or, when the key
// is in no site, at the construct site, where it is reflected already or not
// in the source (BR3.3).
func (p *cdkPlanner) planChange(t *target, c planner.Change, cs cdk.ConstructSite, sites []cdk.TagSite) {
	if c.Kind != planner.KindRenameKey || c.MergesIntoExisting {
		p.manual(t, c, ReasonCDKUnsupportedShape, cs.File, cs.Line)
		return
	}
	var keys []cdk.TagSite
	for _, s := range sites {
		if s.KeyEdit.Shape != cdk.ShapeExpression && s.KeyEdit.Text == c.FromKey {
			keys = append(keys, s)
		}
	}
	switch len(keys) {
	case 0:
		p.decisions = append(p.decisions, &cdkDecision{t: t, c: c, file: cs.File, line: cs.Line, sites: sites})
		return
	case 1:
	default:
		p.manual(t, c, ReasonCDKAmbiguous, keys[0].UseFile, keys[0].UseLine)
		return
	}
	s := keys[0]
	if reason := p.check(t, c, &s); reason != "" {
		p.manual(t, c, reason, s.UseFile, s.UseLine)
		return
	}
	p.decisions = append(p.decisions, &cdkDecision{t: t, c: c, file: s.KeyEdit.File, line: s.UseLine,
		editAt: rangeKey(s.KeyEdit), sites: sites})
}

// check says why the one site of a key cannot be edited, or nothing (BR3.1,
// BR3.2).
func (p *cdkPlanner) check(t *target, c planner.Change, s *cdk.TagSite) ManualReason {
	switch {
	case s.ValueEdit == nil:
		return ReasonCDKUnsupportedShape
	case s.ValueEdit.Shape == cdk.ShapeExpression:
		return ReasonCDKExpression
	case s.KeyEdit.Shape != cdk.ShapeLiteral || s.ValueEdit.Shape != cdk.ShapeLiteral:
		return ReasonCDKUnsupportedShape
	}
	for _, st := range p.run.syn.Stacks {
		for _, res := range st.Resources {
			if res.ConstructPath == t.cdk.resource.ConstructPath || !under(res.ConstructPath, s.ScopeConstructPath) {
				continue
			}
			if _, ok := res.Tags[c.FromKey]; ok {
				return ReasonCDKScopeTag
			}
		}
	}
	for _, scope := range s.KeyEdit.SharedWith {
		if !p.sameChangeUnder(t, c, scope) {
			return ReasonCDKSharedValue
		}
	}
	return ""
}

// sameChangeUnder reports another target below scope with the same change.
func (p *cdkPlanner) sameChangeUnder(t *target, c planner.Change, scope string) bool {
	for _, u := range p.matched {
		if u == t || !under(u.cdk.resource.ConstructPath, scope) {
			continue
		}
		for _, uc := range u.changes {
			if uc.Kind == c.Kind && uc.FromKey == c.FromKey && uc.ToKey == c.ToKey && uc.FromValue == c.FromValue {
				return true
			}
		}
	}
	return false
}

// constructSite finds where the source makes the resource: the construct of
// its path, or of its parent when the path ends with Resource or Default (the
// L1 resource of an L2 construct).
func (p *cdkPlanner) constructSite(resPath string) (cdk.ConstructSite, bool) {
	want := []string{resPath}
	if parent, last, ok := cutLast(resPath); ok && (last == "Resource" || last == "Default") {
		want = append(want, parent)
	}
	for _, w := range want {
		for _, cs := range p.run.analysis.ConstructSites {
			if cs.ConstructPath == w {
				return cs, true
			}
		}
	}
	return cdk.ConstructSite{}, false
}

func cutLast(p string) (string, string, bool) {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return "", "", false
	}
	return p[:i], p[i+1:], true
}

// under reports that p is scope or below it, by whole parts of the path.
func under(p, scope string) bool {
	return p == scope || strings.HasPrefix(p, scope+"/")
}

func rangeKey(e cdk.Edit) string {
	return e.File + "\x00" + strconv.Itoa(e.Start) + "\x00" + strconv.Itoa(e.End)
}

// shareEdits makes one decision of the decisions that edit the same range:
// the first by ARN edits, the others ride on it (contract C7 same_site_rule).
func (p *cdkPlanner) shareEdits() {
	byRange := map[string][]*cdkDecision{}
	for _, d := range p.decisions {
		if d.editAt != "" {
			byRange[d.editAt] = append(byRange[d.editAt], d)
		}
	}
	for _, group := range byRange {
		if len(group) < 2 {
			continue
		}
		slices.SortFunc(group, func(a, b *cdkDecision) int { return strings.Compare(a.t.arn, b.t.arn) })
		for _, d := range group[1:] {
			d.ridesOn = group[0]
			group[0].passengers = append(group[0].passengers, rider{t: d.t, c: d.c})
		}
	}
}

// tasks gathers the decisions by file, and in a file by target.
func (p *cdkPlanner) tasks() []*cdkTask {
	byFile := map[string]*cdkTask{}
	var tasks []*cdkTask
	parts := map[string]*cdkPart{}
	for _, d := range p.decisions {
		if d.ridesOn != nil {
			continue
		}
		task := byFile[d.file]
		if task == nil {
			task = &cdkTask{run: p.run, file: d.file, c: &candidate{rel: p.run.rel(d.file),
				path: filepath.Join(p.run.app.Abs, filepath.FromSlash(d.file)), format: formatTypeScript}}
			byFile[d.file] = task
			tasks = append(tasks, task)
		}
		key := d.file + "\x00" + d.t.arn
		part := parts[key]
		if part == nil {
			copied := *d.t
			copied.changes = nil
			part = &cdkPart{t: &copied, at: place{file: task.c.rel, line: d.line}, sites: d.sites, riders: map[string][]rider{}}
			parts[key] = part
			task.parts = append(task.parts, part)
		}
		part.t.changes = append(part.t.changes, d.c)
		part.riders[d.c.ChangeID] = d.passengers
	}
	slices.SortFunc(tasks, func(a, b *cdkTask) int { return strings.Compare(a.c.rel, b.c.rel) })
	return tasks
}

// processCDK makes the edits of the sources, one file after the other: they
// are few.
func (r *runner) processCDK(ctx context.Context, tasks []*cdkTask) ([]*fileOutcome, error) {
	outs := make([]*fileOutcome, 0, len(tasks))
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		outs = append(outs, processCDKFile(task))
	}
	return outs, nil
}

// processCDKFile reads one source and applies the changes of its parts, with the
// code of the templates. The check of the edits is the one of the helper,
// after every file of the app (recheckCDK).
func processCDKFile(task *cdkTask) *fileOutcome {
	out := &fileOutcome{c: task.c, cdk: &cdkOutcome{run: task.run, file: task.file, expect: map[string]string{}}}
	raw, err := readFile(task.c.path)
	if err != nil || int64(len(raw)) > maxFileBytes || !utf8.Valid(raw) {
		out.unreadableParts(task.parts, "the source could not be read again")
		return out
	}
	var edits []edit
	for _, part := range task.parts {
		site := cdkSite(raw, task.file, part.sites)
		sim := simulate(part.t, site, part.at, true)
		out.manual = append(out.manual, sim.manual...)
		out.assumed = append(out.assumed, sim.assumed...)
		out.already += sim.already
		out.followRiders(part, sim)
		for _, c := range sim.reflected {
			out.cdk.reflected = append(out.cdk.reflected, cdkReflected{part: part, c: c})
		}
		if site.issue != "" {
			continue
		}
		edits = append(edits, buildEdits(raw, formatTypeScript, site, sim.final)...)
		for _, e := range sim.final {
			if e.orig >= 0 && e.key != site.entries[e.orig].key {
				out.cdk.expect[keyAt(task.file, site.entries[e.orig].keySpan)] = e.key
			}
		}
	}
	if len(out.cdk.reflected) == 0 {
		return out
	}
	normal, err := normalize(edits)
	if err != nil {
		out.failCDK(err.Error(), ReasonUnreadableFile)
		return out
	}
	updated := applyEdits(raw, normal)
	for _, rc := range out.cdk.reflected {
		out.reflected += 1 + len(rc.part.riders[rc.c.ChangeID])
	}
	out.updated = updated
	out.diff = unifiedDiff(task.c.rel, raw, updated, normal)
	out.blob = blobHashes(task.c.rel, raw)
	out.sum = sha256.Sum256(raw)
	return out
}

func keyAt(file string, s span) string {
	return file + "\x00" + strconv.Itoa(s.start)
}

// cdkSite makes the tag set of a resource from the tag sites whose scope holds
// it: one entry for each site, with the ranges of the key and the value when
// they are in this file. A range that does not hold the quoted string the
// helper read makes the set unreadable (functional spec §2).
func cdkSite(raw []byte, file string, sites []cdk.TagSite) *tagSite {
	site := &tagSite{shape: shapeKeyValueList}
	for _, s := range sites {
		e := tagEntry{key: s.KeyEdit.Text, line: s.UseLine, literal: s.KeyEdit.Shape == cdk.ShapeLiteral &&
			s.ValueEdit != nil && s.ValueEdit.Shape == cdk.ShapeLiteral}
		if s.ValueEdit != nil {
			e.value = s.ValueEdit.Text
		}
		if e.literal && s.KeyEdit.File == file && s.ValueEdit.File == file {
			var okKey, okValue bool
			e.keySpan, e.keyQuote, okKey = quotedSpan(raw, s.KeyEdit)
			e.valueSpan, e.valueQuote, okValue = quotedSpan(raw, *s.ValueEdit)
			if !okKey || !okValue {
				site.issue = ReasonUnreadableFile
			}
		}
		site.entries = append(site.entries, e)
	}
	return site
}

// quotedSpan checks that the range of an edit holds its text between two equal
// quotes, and returns the range and the quote.
func quotedSpan(raw []byte, e cdk.Edit) (span, quoteStyle, bool) {
	if e.Start < 0 || e.End > len(raw) || e.End-e.Start < 2 || raw[e.Start] != raw[e.End-1] {
		return span{}, 0, false
	}
	var q quoteStyle
	switch raw[e.Start] {
	case '\'':
		q = quoteSingle
	case '"':
		q = quoteDouble
	case '`':
		q = quoteBacktick
	default:
		return span{}, 0, false
	}
	if string(raw[e.Start+1:e.End-1]) != e.Text {
		return span{}, 0, false
	}
	return span{e.Start, e.End}, q, true
}

// followRiders gives the riders of a part the outcome of the change they ride
// on: listed with the same reason, or reflected already. A reflected change
// counts them when its file is edited.
func (o *fileOutcome) followRiders(part *cdkPart, sim *outcomeOfTarget) {
	for id, riders := range part.riders {
		if len(riders) == 0 || slices.ContainsFunc(sim.reflected, func(c planner.Change) bool { return c.ChangeID == id }) {
			continue
		}
		var item *ManualItem
		for i := range sim.manual {
			if sim.manual[i].ChangeID == id {
				item = &sim.manual[i]
			}
		}
		for _, rd := range riders {
			if item == nil {
				o.already++
				continue
			}
			o.manual = append(o.manual, ManualItem{ARN: rd.t.arn, ChangeID: rd.c.ChangeID, Key: changeKey(rd.c),
				File: item.File, Line: item.Line, Reason: item.Reason})
		}
	}
}

// unreadableParts puts every change of the parts, riders too, on the list.
func (o *fileOutcome) unreadableParts(parts []*cdkPart, why string) {
	for _, part := range parts {
		for _, c := range part.t.changes {
			o.manual = append(o.manual, ManualItem{ARN: part.t.arn, ChangeID: c.ChangeID, Key: changeKey(c),
				File: part.at.file, Line: part.at.line, Reason: ReasonUnreadableFile})
			for _, rd := range part.riders[c.ChangeID] {
				o.manual = append(o.manual, ManualItem{ARN: rd.t.arn, ChangeID: rd.c.ChangeID, Key: changeKey(rd.c),
					File: part.at.file, Line: part.at.line, Reason: ReasonUnreadableFile})
			}
		}
	}
	o.debug = append(o.debug, o.c.rel+": "+why)
}

// failCDK drops the edits of the file and puts the changes they made on the
// list with reason.
func (o *fileOutcome) failCDK(why string, reason ManualReason) {
	for _, rc := range o.cdk.reflected {
		at := rc.part.at
		o.manual = append(o.manual, ManualItem{ARN: rc.part.t.arn, ChangeID: rc.c.ChangeID, Key: changeKey(rc.c),
			File: at.file, Line: at.line, Reason: reason})
		for _, rd := range rc.part.riders[rc.c.ChangeID] {
			o.manual = append(o.manual, ManualItem{ARN: rd.t.arn, ChangeID: rd.c.ChangeID, Key: changeKey(rd.c),
				File: at.file, Line: at.line, Reason: reason})
		}
	}
	o.cdk.reflected = nil
	o.reflected, o.updated, o.diff = 0, nil, ""
	o.debug = append(o.debug, o.c.rel+": "+why)
}

// recheckCDK reads every app with edits again with the edited contents, before
// the difference is shown, with or without --write (BR3.4). An app whose
// sources do not read back as expected keeps none of its edits; their changes
// go on the list with cdkUnsupportedShape (contract C7 recheck_failed).
func (r *runner) recheckCDK(ctx context.Context, outs []*fileOutcome) error {
	byRun := map[*cdkRun][]*fileOutcome{}
	var runs []*cdkRun
	for _, o := range outs {
		if o.cdk == nil || o.updated == nil {
			continue
		}
		if byRun[o.cdk.run] == nil {
			runs = append(runs, o.cdk.run)
		}
		byRun[o.cdk.run] = append(byRun[o.cdk.run], o)
	}
	for _, run := range runs {
		files := byRun[run]
		edited := make(map[string][]byte, len(files))
		expect := map[string]string{}
		for _, o := range files {
			edited[o.cdk.file] = o.updated
			for k, v := range o.cdk.expect {
				expect[k] = v
			}
		}
		again, err := r.w.analyzer.Recheck(ctx, run.app, edited)
		r.res.Debug = append(r.res.Debug, again.Debug...)
		var why string
		var hf *cdk.HelperFailure
		switch {
		case errors.As(err, &hf):
			why = "the helper could not read the edited sources: " + hf.Reason
		case err != nil:
			return helperError(err)
		default:
			why = compareAnalyses(run.analysis, again, expect)
		}
		if why != "" {
			for _, o := range files {
				o.failCDK("the edited source did not read back as expected: "+why, ReasonCDKUnsupportedShape)
			}
		}
	}
	return nil
}

// compareAnalyses says how the analysis after the edits differs from the one
// before, or nothing: the sites are the same, in the same places, with the
// keys of expect and the same values (BR3.4).
func compareAnalyses(before, after cdk.Analysis, expect map[string]string) string {
	if len(after.TagSites) != len(before.TagSites) {
		return "the number of tag sites changed"
	}
	if !slices.Equal(after.ConstructSites, before.ConstructSites) {
		return "the construct sites changed"
	}
	for i := range before.TagSites {
		b, a := &before.TagSites[i], &after.TagSites[i]
		if a.UseFile != b.UseFile || a.UseLine != b.UseLine || a.Op != b.Op || a.ScopeConstructPath != b.ScopeConstructPath {
			return "a tag site moved or changed its scope"
		}
		want := b.KeyEdit.Text
		if k, ok := expect[keyAt(b.KeyEdit.File, span{start: b.KeyEdit.Start})]; ok {
			want = k
		}
		if a.KeyEdit.Text != want || a.KeyEdit.Shape != b.KeyEdit.Shape {
			return "a key is not the expected one"
		}
		if (a.ValueEdit == nil) != (b.ValueEdit == nil) ||
			a.ValueEdit != nil && (a.ValueEdit.Text != b.ValueEdit.Text || a.ValueEdit.Shape != b.ValueEdit.Shape) {
			return "a value changed"
		}
	}
	return ""
}
