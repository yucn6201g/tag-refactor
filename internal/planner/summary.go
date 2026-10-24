package planner

import "sort"

// PlanSummary holds the counts every output format shows (contract C3
// summary, BR2.1 to BR2.5). The formats take them from here instead of
// counting again, so they always agree (BR3.1, NFR10.6).
type PlanSummary struct {
	ScannedResources int
	// ResourcesToChange counts a resource with several changes once.
	ResourcesToChange int
	// ChangesByKind has an entry for each of the four kinds.
	ChangesByKind map[ChangeKind]int
	// ChangeLines are the lines of the table format, in display order.
	ChangeLines            []ChangeLine
	ConflictResources      int
	UnsetRequiredResources int
	// RemoveCandidateKeys is the number of remove candidates (keys).
	RemoveCandidateKeys int
	AffectedReferences  int
	UndecidedReferences int
	// IaCManagedResources counts the resources with changes that IaC manages
	// (BR2.4).
	IaCManagedResources int
}

// ChangeLine is one line of the table format: the changes with the same
// content (BR2.1). Renames are grouped by (FromKey, ToKey), value changes by
// (FromKey, FromValue, ToValue), required tags by ToKey and removals by
// FromKey; the reference values of renames and removals do not split lines.
type ChangeLine struct {
	Kind      ChangeKind
	FromKey   string
	ToKey     string
	FromValue string
	// ToValue is the value of a value change, or the value every addRequired
	// of the line gives; empty with SeveralValues.
	ToValue string
	// SeveralValues is true when the addRequired of the line give different
	// values: the table shows "(from rules)".
	SeveralValues bool
	Resources     int
}

// lineKey is what groups changes into one ChangeLine.
type lineKey struct {
	kind                               ChangeKind
	fromKey, toKey, fromValue, toValue string
}

// Summary counts the plan for display (BR2.1 to BR2.5).
func (p *Plan) Summary() PlanSummary {
	s := PlanSummary{
		ScannedResources:       p.ScannedResources,
		ResourcesToChange:      p.ResourcesToChange(),
		ChangesByKind:          map[ChangeKind]int{},
		ChangeLines:            []ChangeLine{},
		ConflictResources:      p.ConflictResources(),
		UnsetRequiredResources: p.UnsetRequiredResources(),
		RemoveCandidateKeys:    len(p.RemoveCandidates),
	}
	for _, k := range Kinds {
		s.ChangesByKind[k] = 0
	}
	s.AffectedReferences, s.UndecidedReferences = p.CountFindings()
	index := map[lineKey]int{}
	for _, r := range p.Resources {
		if r.IaCManagedBy != "" {
			s.IaCManagedResources++
		}
		for _, c := range r.Changes {
			s.ChangesByKind[c.Kind]++
			key := lineKey{kind: c.Kind}
			switch c.Kind {
			case KindRenameKey:
				key.fromKey, key.toKey = c.FromKey, c.ToKey
			case KindChangeValue:
				key.fromKey, key.fromValue, key.toValue = c.FromKey, c.FromValue, c.ToValue
			case KindAddRequired:
				key.toKey = c.ToKey
			case KindRemoveKey:
				key.fromKey = c.FromKey
			}
			i, ok := index[key]
			if !ok {
				i = len(s.ChangeLines)
				index[key] = i
				line := ChangeLine{Kind: c.Kind, FromKey: key.fromKey, ToKey: key.toKey, FromValue: key.fromValue, ToValue: key.toValue}
				if c.Kind == KindAddRequired {
					line.ToValue = c.ToValue
				}
				s.ChangeLines = append(s.ChangeLines, line)
			}
			line := &s.ChangeLines[i]
			line.Resources++
			if c.Kind == KindAddRequired && !line.SeveralValues && line.ToValue != c.ToValue {
				line.SeveralValues = true
				line.ToValue = ""
			}
		}
	}
	rank := map[ChangeKind]int{}
	for i, k := range Kinds {
		rank[k] = i
	}
	sort.SliceStable(s.ChangeLines, func(i, j int) bool {
		a, b := s.ChangeLines[i], s.ChangeLines[j]
		if a.Kind != b.Kind {
			return rank[a.Kind] < rank[b.Kind]
		}
		if a.Resources != b.Resources {
			return a.Resources > b.Resources
		}
		if a.ToKey != b.ToKey {
			return a.ToKey < b.ToKey
		}
		if a.FromKey != b.FromKey {
			return a.FromKey < b.FromKey
		}
		if a.FromValue != b.FromValue {
			return a.FromValue < b.FromValue
		}
		return a.ToValue < b.ToValue
	})
	return s
}
