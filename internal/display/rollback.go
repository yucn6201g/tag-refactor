package display

import (
	"github.com/yucn6201g/tag-refactor/internal/apply"
)

// The output of rollback (U7 BR4.1, BR7.1, NFR11.9). The confirmation and
// the messages go to standard error, the counts and the lists of the result
// to standard output (contract C5). Every mark is written in words too, so
// the screens mean the same without colors.

// RollbackCommands are the commands the result of rollback shows, empty when
// they do not apply (BR7.1): with --force for the resources changed after
// apply, and the same rollback again after a stop or a resource not
// restored. Both carry --record-from and --profile when they were given.
type RollbackCommands struct {
	Force, Again string
}

// RollbackSummary prints the confirmation screen (BR4.1): the account, the
// run and the record, the marks of an apply that stopped half way and of a
// record already undone, and the counts of the judgement.
func (p *Printer) RollbackSummary(s apply.RollbackSummary) {
	p.locked(func() {
		p.endProgress()
		p.say(styleNone, "rollback.header", s.AccountID, s.RunID)
		p.say(styleNone, "apply.record", s.Record)
		if !s.ApplyCompleted {
			p.say(styleImpact, "rollback.applyStopped")
		}
		if s.UndoneAt != "" {
			p.say(styleImpact, "rollback.alreadyUndone", s.UndoneAt)
		}
		p.sayText(styleNone, "")
		changed, changedNote := "rollback.rowChanged", "rollback.rowChangedNote"
		if s.Force {
			changed, changedNote = "rollback.rowChangedForce", ""
		}
		rows := []struct {
			label, note string
			n           int
			always      bool
			st          style
		}{
			{"rollback.rowRestore", "", s.ToRestore, true, styleNone},
			{changed, changedNote, s.ChangedSinceApply, true, styleChange},
			{"rollback.rowAlready", "", s.AlreadyRestored, true, styleNone},
			{"rollback.rowGone", "", s.Gone, true, styleChange},
			{"rollback.rowOverLimit", "", s.OverLimit, false, styleChange},
			{"rollback.rowUnreadable", "", s.Unreadable, false, styleRemove},
		}
		shown := rows[:0]
		for _, r := range rows {
			if r.n > 0 || r.always {
				shown = append(shown, r)
			}
		}
		labelWidth, countWidth := 0, 0
		for _, r := range shown {
			labelWidth = max(labelWidth, Width(p.tr(r.label)))
			countWidth = max(countWidth, Width(p.count("unit.item", r.n)))
		}
		for _, r := range shown {
			line := "  " + padRight(p.tr(r.label), labelWidth) + "  " + padLeft(p.count("unit.item", r.n), countWidth)
			if r.note != "" && r.n > 0 {
				line += "   " + p.tr(r.note)
			}
			st := r.st
			if r.n == 0 {
				st = styleNone
			}
			p.sayText(st, line)
		}
		p.sayText(styleNone, "")
	})
}

// RollbackPrompt asks for yes on standard error, without a line break.
func (p *Printer) RollbackPrompt() {
	p.locked(func() { p.writeRaw(p.tr("rollback.prompt")) })
}

// RollbackCancelled says that nothing was undone (BR4.2). The prompt's line
// is ended first.
func (p *Printer) RollbackCancelled() {
	p.locked(func() {
		p.writeRaw("\n")
		p.say(styleNone, "rollback.cancelled")
	})
}

// RollbackNothing says that apply wrote nothing, so there is nothing to undo
// (BR1.4).
func (p *Printer) RollbackNothing() {
	p.locked(func() {
		p.endProgress()
		p.say(styleNone, "rollback.nothing")
	})
}

// RollbackProgress prints a line at every tenth of the resources to write
// back and at the end (contract C11).
func (p *Printer) RollbackProgress(done, total int) {
	if total <= 0 {
		return
	}
	p.locked(func() {
		step := max(total/10, 1)
		if done == total || done/step != (done-1)/step {
			p.say(styleNone, "rollback.progress", FormatCount(done), FormatCount(total))
		}
	})
}

// RollbackResult prints the result of rollback (BR7.1): on standard output
// the counts of restored, already restored, skipped and failed resources
// with the lists of the skipped and failed ones and their reasons; on
// standard error the stop, the log file, the record, a lock that is left and
// the commands to run next.
func (p *Printer) RollbackResult(o *apply.RollbackOutcome, cmds RollbackCommands) {
	p.locked(func() {
		p.endProgress()
		r := p.newResults()
		if o.Started {
			r.line(styleNone, "rollback.restored", rawString(r.count("unit.resource", o.Restored)))
			if o.Unconfirmed > 0 {
				r.line(styleNote, "apply.unconfirmed", rawString(r.count("unit.resource", o.Unconfirmed)))
			}
			r.line(styleNone, "rollback.already", rawString(r.count("unit.resource", o.AlreadyRestored)))
			r.line(styleNone, "apply.skipped", rawString(r.count("unit.resource", len(o.Skipped))))
			r.rollbackTargets(styleChange, o.Skipped)
			r.line(styleNone, "apply.failed", rawString(r.count("unit.resource", len(o.Failed))))
			r.rollbackTargets(styleRemove, o.Failed)
			if o.NotWritten > 0 {
				r.line(styleImpact, "rollback.notWritten", rawString(r.count("unit.resource", o.NotWritten)))
			}
		}
		p.flush(r)
		p.rollbackFooter(o, cmds)
	})
}

// rollbackTargets prints one line per resource with its reason.
func (r *results) rollbackTargets(st style, ts []*apply.RollbackTarget) {
	for _, t := range ts {
		r.text(st, "  "+Sanitize(t.ARN)+"  "+rollbackReason(r.lang, t))
	}
}

// rollbackReason says in words why a resource of rollback was skipped or
// failed.
func rollbackReason(lang Lang, t *apply.RollbackTarget) string {
	cause := Sanitize(t.ErrorKind)
	if t.Operation != "" {
		cause += " (" + Sanitize(t.Operation) + ")"
	}
	switch t.Reason {
	case apply.ReasonChangedSinceApply, apply.ReasonNotRestored:
		return T(lang, "reason."+string(t.Reason))
	case apply.ReasonResourceNotFound:
		return T(lang, "reason.resourceNotFound")
	case apply.ReasonTagLimitExceeded:
		return T(lang, "reason.tagLimitExceeded", t.TagCount)
	case apply.ReasonReadFailed, apply.ReasonWriteFailed, apply.ReasonPartialWrite:
		return T(lang, "reason."+string(t.Reason), cause)
	case apply.ReasonResultUnknown:
		if t.ErrorKind == "" {
			return T(lang, "reason.stopped")
		}
		return T(lang, "reason.resultUnknown", cause)
	}
	return Sanitize(string(t.Reason))
}

// rollbackFooter prints the messages after the lists. The caller holds the
// lock.
func (p *Printer) rollbackFooter(o *apply.RollbackOutcome, cmds RollbackCommands) {
	switch {
	case o.Stop == apply.StopSignal && !o.Started:
		p.say(styleImpact, "rollback.stopBeforeUndo")
	case o.Stop == apply.StopSignal:
		p.say(styleImpact, "apply.stopSignal")
	case o.Stop == apply.StopCredentials:
		p.say(styleImpact, "apply.stopCredentials")
	case o.Stop == apply.StopTransient:
		p.say(styleImpact, "apply.stopTransient", apply.TransientLimit())
	case o.Stop == apply.StopStorage:
		p.say(styleImpact, "apply.stopStorage")
	case o.Stop == apply.StopInternal:
		p.say(styleImpact, "apply.stopInternal")
	}
	if o.LogFile != "" {
		p.say(styleNone, "apply.log", o.LogFile)
	}
	if o.LogErr != nil {
		p.say(styleChange, "stderr.warning", rawString(p.tr("apply.logFailed", Sanitize(errSubject(o.LogErr)))))
	}
	if o.Started {
		if o.Completed {
			p.say(styleNone, "apply.record", o.Record)
		} else {
			p.say(styleImpact, "rollback.recordOpen", o.Record)
		}
	}
	if o.SaveErr != nil {
		p.say(styleImpact, "apply.saveFailed", errSubject(o.SaveErr))
	}
	if o.ReleaseErr != nil {
		p.say(styleImpact, "apply.lockNotRemoved", errSubject(o.ReleaseErr))
	}
	if cmds.Force != "" {
		p.say(styleNone, "rollback.forceCommand", cmds.Force)
	}
	if cmds.Again != "" {
		p.say(styleNone, "rollback.againCommand", cmds.Again)
	}
}
