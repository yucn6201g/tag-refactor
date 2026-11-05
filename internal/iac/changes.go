package iac

import (
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// simEntry is a tag in the list the changes are applied to in memory.
type simEntry struct {
	// orig is the index of the entry in the site, or -1 for a tag added here.
	orig       int
	key, value string
	literal    bool
}

// outcomeOfTarget is what the changes of one target come to (BR4.3).
type outcomeOfTarget struct {
	// final is the list after the changes.
	final []simEntry
	// reflected are the changes the edits will make.
	reflected []planner.Change
	// already counts the changes the source shows already.
	already int
	manual  []ManualItem
	assumed []ManualItem
}

// simulator applies the changes of one target to the tags of its place.
type simulator struct {
	t    *target
	site *tagSite
	at   place
	// noRemoveOrAdd is true when the whole place cannot be replaced, so that
	// neither a removal nor an addition can be made (security design §4).
	noRemoveOrAdd bool
	list          []simEntry
	out           outcomeOfTarget
}

// simulate decides each change of the target (BR4.3). The whole resource is
// looked at first: when the keys the changes touch are all as they should be
// after the changes, everything is reflected already. Otherwise the changes go
// in order over the list, each seeing what the ones before it did.
func simulate(t *target, site *tagSite, at place, noRemoveOrAdd bool) *outcomeOfTarget {
	s := &simulator{t: t, site: site, at: at, noRemoveOrAdd: noRemoveOrAdd}
	if site != nil && site.issue != "" {
		for _, c := range t.changes {
			s.manual(c, site.issue, at.line)
		}
		return &s.out
	}
	if site != nil {
		for i := range site.entries {
			e := &site.entries[i]
			s.list = append(s.list, simEntry{orig: i, key: e.key, value: e.value, literal: e.literal})
		}
	}
	if s.alreadyAfter() {
		s.markAllReflected()
		s.out.final = s.list
		return &s.out
	}
	for _, c := range t.changes {
		s.apply(c)
	}
	s.out.final = s.list
	return &s.out
}

func (s *simulator) find(key string) *simEntry {
	if key == "" {
		return nil
	}
	for i := range s.list {
		if s.list[i].key == key {
			return &s.list[i]
		}
	}
	return nil
}

// lineOf returns the line of the first entry of a key that is still in the
// original place, or the line of the resource.
func (s *simulator) lineOf(e *simEntry) int {
	if e != nil && e.orig >= 0 && s.site != nil {
		return s.site.entries[e.orig].line
	}
	return s.at.line
}

func (s *simulator) manual(c planner.Change, reason ManualReason, line int) {
	s.out.manual = append(s.out.manual, ManualItem{ARN: s.t.arn, ChangeID: c.ChangeID, Key: changeKey(c),
		File: s.at.file, Line: line, Reason: reason})
}

func (s *simulator) assumed(c planner.Change) {
	s.out.already++
	s.out.assumed = append(s.out.assumed, ManualItem{ARN: s.t.arn, ChangeID: c.ChangeID, Key: changeKey(c), File: s.at.file})
}

// afterTags is beforeTags with every change applied.
func afterTags(t *target) map[string]string {
	after := make(map[string]string, len(t.before))
	for k, v := range t.before {
		after[k] = v
	}
	for _, c := range t.changes {
		switch c.Kind {
		case planner.KindRenameKey:
			v, ok := after[c.FromKey]
			if !ok {
				v = c.FromValue
			}
			delete(after, c.FromKey)
			if !c.MergesIntoExisting {
				after[c.ToKey] = v
			}
		case planner.KindChangeValue:
			after[c.FromKey] = c.ToValue
		case planner.KindAddRequired:
			after[c.ToKey] = c.ToValue
		case planner.KindRemoveKey:
			delete(after, c.FromKey)
		}
	}
	return after
}

// alreadyAfter reports whether every key the changes touch is as it should be
// after the changes.
func (s *simulator) alreadyAfter() bool {
	after := afterTags(s.t)
	check := func(key string) bool {
		if key == "" {
			return true
		}
		e := s.find(key)
		want, present := after[key]
		if !present {
			return e == nil
		}
		return e != nil && e.literal && e.value == want
	}
	for _, c := range s.t.changes {
		if !check(c.FromKey) || !check(c.ToKey) {
			return false
		}
	}
	return true
}

// markAllReflected counts every change as reflected already. A removal whose
// key is not in the source may be a key the source never had, so it is listed.
func (s *simulator) markAllReflected() {
	for _, c := range s.t.changes {
		if isRemoval(c) {
			s.assumed(c)
			continue
		}
		s.out.already++
	}
}

// isRemoval reports a change that only takes a key away.
func isRemoval(c planner.Change) bool {
	return c.Kind == planner.KindRemoveKey || c.Kind == planner.KindRenameKey && c.MergesIntoExisting
}

func (s *simulator) apply(c planner.Change) {
	switch c.Kind {
	case planner.KindRenameKey:
		if c.MergesIntoExisting {
			s.applyRemoval(c)
			return
		}
		s.applyRename(c)
	case planner.KindChangeValue:
		s.applyChangeValue(c)
	case planner.KindAddRequired:
		s.applyAdd(c)
	case planner.KindRemoveKey:
		s.applyRemoval(c)
	}
}

func (s *simulator) applyRename(c planner.Change) {
	from, to := s.find(c.FromKey), s.find(c.ToKey)
	switch {
	case from == nil && to == nil:
		s.manual(c, ReasonKeyNotInSource, s.at.line)
	case from == nil:
		s.out.already++
	case !from.literal:
		s.manual(c, ReasonNotLiteral, s.lineOf(from))
	case to != nil:
		s.manual(c, ReasonKeyAlreadyExists, s.lineOf(to))
	case from.value != c.FromValue:
		s.manual(c, ReasonValueDiffers, s.lineOf(from))
	default:
		from.key = c.ToKey
		s.out.reflected = append(s.out.reflected, c)
	}
}

func (s *simulator) applyChangeValue(c planner.Change) {
	from := s.find(c.FromKey)
	switch {
	case from == nil:
		s.manual(c, ReasonKeyNotInSource, s.at.line)
	case !from.literal:
		s.manual(c, ReasonNotLiteral, s.lineOf(from))
	case from.value == c.FromValue:
		from.value = c.ToValue
		s.out.reflected = append(s.out.reflected, c)
	case from.value == c.ToValue:
		s.out.already++
	default:
		s.manual(c, ReasonValueDiffers, s.lineOf(from))
	}
}

func (s *simulator) applyAdd(c planner.Change) {
	to := s.find(c.ToKey)
	switch {
	case to == nil:
		if reason := s.cannotAdd(); reason != "" {
			s.manual(c, reason, s.at.line)
			return
		}
		s.list = append(s.list, simEntry{orig: -1, key: c.ToKey, value: c.ToValue, literal: true})
		s.out.reflected = append(s.out.reflected, c)
	case !to.literal:
		s.manual(c, ReasonNotLiteral, s.lineOf(to))
	case to.value == c.ToValue:
		s.out.already++
	default:
		s.manual(c, ReasonKeyAlreadyExists, s.lineOf(to))
	}
}

// cannotAdd says why a tag cannot be added to the place (BR4.5), or nothing.
func (s *simulator) cannotAdd() ManualReason {
	switch {
	case s.site == nil || len(s.site.entries) == 0:
		return ReasonNoTagSite
	case !s.site.multiline || s.site.extraFields || s.noRemoveOrAdd || s.site.copySource() == nil:
		return ReasonUnsupportedTagShape
	}
	return ""
}

func (s *simulator) applyRemoval(c planner.Change) {
	from := s.find(c.FromKey)
	switch {
	case from == nil:
		s.assumed(c)
	case !from.literal:
		s.manual(c, ReasonNotLiteral, s.lineOf(from))
	case from.value != c.FromValue:
		s.manual(c, ReasonValueDiffers, s.lineOf(from))
	case !s.site.multiline || s.noRemoveOrAdd:
		s.manual(c, ReasonUnsupportedTagShape, s.lineOf(from))
	default:
		for i := range s.list {
			if &s.list[i] == from {
				s.list = append(s.list[:i:i], s.list[i+1:]...)
				break
			}
		}
		s.out.reflected = append(s.out.reflected, c)
	}
}

// copySource returns the entry a new tag copies its form from: the last one
// with a literal key and value and no comment (BR4.5).
func (s *tagSite) copySource() *tagEntry {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if e := &s.entries[i]; e.literal && !e.hasComment {
			return e
		}
	}
	return nil
}
