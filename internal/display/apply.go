package display

import (
	"fmt"

	"github.com/yucn6201g/tag-refactor/internal/apply"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// The output of apply (U6 BR2.1, BR2.7, BR6.1, BR6.3). The confirmation and
// the messages go to standard error, the lists of the result to standard
// output (contract C5). The meaning of every mark is written in words too
// (NFR11.8).

var _ apply.ProgressSink = (*Printer)(nil)

// ApplyView is what the confirmation shows besides the plan.
type ApplyView struct {
	// RuleFile is set when the plan was made now from --rules.
	RuleFile string
	// Record is the folder or S3 URI of the records.
	Record string
}

// ApplySummary prints the confirmation screen (BR2.1): the account, the
// regions and the plan; the resources to change by kind; the changes with
// impact with ! and ?; the incomplete marks; the resources managed by IaC;
// what the plan does not change; and where the record goes.
func (p *Printer) ApplySummary(plan *planner.Plan, v ApplyView) {
	p.locked(func() {
		p.endProgress()
		s := plan.Summary()
		regions := rawString(p.count("unit.region", len(plan.Regions)))
		if v.RuleFile != "" {
			p.say(styleNone, "apply.headerRules", plan.AccountID, regions, plan.PlanID, v.RuleFile)
		} else {
			p.say(styleNone, "apply.header", plan.AccountID, regions, plan.PlanID, planner.FormatTime(plan.CreatedAt))
		}
		p.sayText(styleNone, "")
		p.say(styleNone, "apply.toChange", FormatCount(s.ResourcesToChange))
		width := 0
		for _, k := range planner.Kinds {
			width = max(width, Width(p.tr("kind."+string(k))))
		}
		for _, k := range planner.Kinds {
			p.sayText(styleNone, "  "+padRight(p.tr("kind."+string(k)), width)+" "+padLeft(FormatCount(s.ChangesByKind[k]), 7))
		}
		p.sayText(styleNone, "")
		p.applyImpact(plan)
		if !plan.InventoryComplete {
			p.say(styleImpact, "apply.inventoryIncomplete")
		}
		if !plan.ImpactAnalysisComplete {
			p.say(styleImpact, "apply.impactIncomplete")
		}
		if s.IaCManagedResources > 0 {
			p.say(styleChange, "apply.iac", FormatCount(s.IaCManagedResources))
		}
		if s.ConflictResources > 0 || s.UnsetRequiredResources > 0 {
			p.say(styleNone, "apply.notChanged", rawString(p.count("unit.conflict", s.ConflictResources)), FormatCount(s.UnsetRequiredResources))
		}
		p.say(styleNone, "apply.record", v.Record)
	})
}

// applyImpact prints the findings of the plan: ! affected with its causes, ?
// cannot decide. The caller holds the lock.
func (p *Printer) applyImpact(plan *planner.Plan) {
	if len(plan.ImpactFindings) == 0 {
		return
	}
	byID := make(map[string]planner.Change, plan.ChangeCount())
	for _, c := range plan.Changes() {
		byID[c.ChangeID] = c
	}
	p.say(styleImpact, "apply.impact", FormatCount(len(plan.ImpactFindings)))
	for _, f := range plan.ImpactFindings {
		if f.Verdict == planner.VerdictUndecided {
			p.say(styleUndecided, "apply.undecided", f.PolicyRef, f.StatementID, f.ConditionKey)
			continue
		}
		p.say(styleImpact, "apply.affected", f.PolicyRef, f.StatementID, f.ConditionKey, rawString(causes(p.lang, f.ChangeIDs, byID)))
	}
}

// ApplyPrompt asks for yes on standard error, without a line break.
func (p *Printer) ApplyPrompt() {
	p.locked(func() { p.writeRaw(p.tr("apply.prompt")) })
}

// ApplyCancelled says that nothing was changed (BR2.2). The prompt's line
// is ended first.
func (p *Printer) ApplyCancelled() {
	p.locked(func() {
		p.writeRaw("\n")
		p.say(styleNone, "apply.cancelled")
	})
}

// ApplyNothing says the plan has nothing to write, with what it does not
// change (BR1.4).
func (p *Printer) ApplyNothing(plan *planner.Plan) {
	p.locked(func() {
		p.endProgress()
		p.say(styleNone, "apply.nothing")
		if plan.ConflictResources() > 0 || plan.UnsetRequiredResources() > 0 {
			p.say(styleNone, "apply.notChanged", rawString(p.count("unit.conflict", plan.ConflictResources())),
				FormatCount(plan.UnsetRequiredResources()))
		}
	})
}

// ApplyImpactStop says that --yes alone stopped on changes with impact
// (BR2.5).
func (p *Printer) ApplyImpactStop() {
	p.locked(func() {
		p.say(styleImpact, "apply.impactStop")
		p.say(styleNone, "apply.impactStopFix")
	})
}

// AllowImpactIgnored warns that --allow-impact without --yes changes nothing
// (BR2.7).
func (p *Printer) AllowImpactIgnored() {
	p.locked(func() { p.say(styleChange, "stderr.warning", rawString(p.tr("apply.allowImpactIgnored"))) })
}

// LocksHeld shows the content of locks (BR3.2): the one in the way, or the
// ones --force-unlock removed.
func (p *Printer) LocksHeld(locks []journal.HeldLock, removed bool) {
	p.locked(func() {
		for _, l := range locks {
			if !l.Readable {
				p.say(styleNone, "apply.lockUnreadable", l.Where)
			} else {
				i := l.Info
				p.say(styleNone, "apply.lockHeld", l.Where, i.RunID, string(i.Purpose), i.CreatedAt, i.Host, i.PID)
			}
			if removed {
				p.say(styleNone, "apply.lockRemoved", l.Where)
			}
		}
	})
}

// ItemsProcessed implements apply.ProgressSink: a line at every tenth of the
// resources and at the end.
func (p *Printer) ItemsProcessed(done, total int) {
	if total <= 0 {
		return
	}
	p.locked(func() {
		step := max(total/10, 1)
		if done == total || done/step != (done-1)/step {
			p.say(styleNone, "apply.progress", FormatCount(done), FormatCount(total))
		}
	})
}

// Notice implements apply.ProgressSink: a warning from the catalog with its
// values.
func (p *Printer) Notice(id string, args ...any) {
	p.locked(func() {
		p.say(styleChange, "stderr.warning", rawString(fmt.Sprintf(p.tr(id), sanitizeArgs(args)...)))
	})
}

// ApplyResult prints the result of apply (BR6.1): on standard output the
// counts and the lists of the skipped and failed resources with their
// reasons; on standard error the stop, the log file, the record, a lock
// that is left and the rollback command (BR6.3, empty when no record was
// started).
func (p *Printer) ApplyResult(res *apply.Result, rollback string) {
	p.locked(func() {
		p.endProgress()
		r := p.newResults()
		r.line(styleNone, "apply.applied", rawString(r.count("unit.resource", res.Applied)))
		if res.Unconfirmed > 0 {
			r.line(styleNote, "apply.unconfirmed", rawString(r.count("unit.resource", res.Unconfirmed)))
		}
		r.targets("apply.skipped", styleChange, res.Skipped)
		r.targets("apply.failed", styleRemove, res.Failed)
		if res.NotWritten > 0 {
			r.line(styleImpact, "apply.notWritten", rawString(r.count("unit.resource", res.NotWritten)))
		}
		p.flush(r)
		p.applyFooter(res, rollback)
	})
}

// targets prints a heading with the count and one line per resource.
func (r *results) targets(id string, st style, ts []*apply.Target) {
	if len(ts) == 0 {
		return
	}
	r.line(st, id, rawString(r.count("unit.resource", len(ts))))
	for _, t := range ts {
		r.text(styleNone, "  "+Sanitize(t.ARN)+"  "+reasonText(r.lang, t))
	}
}

// reasonText says in words why a resource was skipped or failed.
func reasonText(lang Lang, t *apply.Target) string {
	cause := Sanitize(t.ErrorKind)
	if t.Operation != "" {
		cause += " (" + Sanitize(t.Operation) + ")"
	}
	switch t.Reason {
	case apply.ReasonTagLimitExceeded:
		return T(lang, "reason.tagLimitExceeded", t.TagCount)
	case apply.ReasonReadFailed, apply.ReasonWriteFailed, apply.ReasonPartialWrite:
		return T(lang, "reason."+string(t.Reason), cause)
	case apply.ReasonResultUnknown:
		if t.ErrorKind == "" {
			return T(lang, "reason.stopped")
		}
		return T(lang, "reason.resultUnknown", cause)
	case apply.ReasonChangedSincePlan, apply.ReasonResourceNotFound:
		return T(lang, "reason."+string(t.Reason))
	}
	return Sanitize(string(t.Reason))
}

// applyFooter prints the messages after the lists. The caller holds the
// lock.
func (p *Printer) applyFooter(res *apply.Result, rollback string) {
	switch res.Stop {
	case apply.StopSignal:
		p.say(styleImpact, "apply.stopSignal")
	case apply.StopCredentials:
		p.say(styleImpact, "apply.stopCredentials")
	case apply.StopTransient:
		p.say(styleImpact, "apply.stopTransient", apply.TransientLimit())
	case apply.StopStorage:
		p.say(styleImpact, "apply.stopStorage")
	case apply.StopInternal:
		p.say(styleImpact, "apply.stopInternal")
	}
	if res.LogFile != "" {
		p.say(styleNone, "apply.log", res.LogFile)
	}
	if res.LogErr != nil {
		p.say(styleChange, "stderr.warning", rawString(p.tr("apply.logFailed", Sanitize(errSubject(res.LogErr)))))
	}
	if res.Record != "" {
		if res.Completed {
			p.say(styleNone, "apply.record", res.Record)
		} else {
			p.say(styleImpact, "apply.recordOpen", res.Record)
		}
	}
	if res.SaveErr != nil {
		p.say(styleImpact, "apply.saveFailed", errSubject(res.SaveErr))
	}
	if res.ReleaseErr != nil {
		p.say(styleImpact, "apply.lockNotRemoved", errSubject(res.ReleaseErr))
	}
	if rollback != "" {
		p.say(styleNone, "apply.rollback", rollback)
	}
}

// LockNotReleased reports a lock left behind by a run that ended with an
// error (BR3.8).
func (p *Printer) LockNotReleased(err error) {
	p.locked(func() { p.say(styleImpact, "apply.lockNotRemoved", errSubject(err)) })
}

// errSubject is the tool-made subject of an error (a path or a URI), never
// its text.
func errSubject(err error) string {
	if e, ok := errkind.As(err); ok && e.Subject != "" {
		return e.Subject
	}
	return "-"
}
