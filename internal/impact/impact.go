// Package impact reads the customer managed IAM policies that are attached to
// a user, role or group and finds conditions that reference the tag keys being
// renamed (contract C9, minimal U1 version: BR5.1 to BR5.6).
package impact

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Analyzer implements planner.ImpactAnalyzer with a read-only policy reader.
type Analyzer struct {
	reader awsaccess.PolicyReader
}

var _ planner.ImpactAnalyzer = (*Analyzer)(nil)

// New returns an analyzer that reads policies through reader.
func New(reader awsaccess.PolicyReader) *Analyzer {
	return &Analyzer{reader: reader}
}

// Analyze matches the conditions of every attached customer managed policy
// against the changes. Policies that cannot be read are reported in
// UnreadPolicies and make the result incomplete; the other policies are still
// matched (BR5.5). It returns an error only when ctx is cancelled.
func (a *Analyzer) Analyze(ctx context.Context, changes []planner.TagChangeInput) (planner.ImpactResult, error) {
	res := planner.ImpactResult{Findings: []planner.ImpactFinding{}, UnreadPolicies: []planner.UnreadPolicy{}, Complete: true}
	if len(changes) == 0 {
		return res, nil
	}
	refs, err := a.reader.ListAttachedCustomerPolicies(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return res, ctxErr
		}
		res.UnreadPolicies = append(res.UnreadPolicies, planner.UnreadPolicy{
			SourceKind: planner.SourceIAMManaged, Reason: unreadReason(err, awsaccess.OpListPolicies),
		})
		res.Complete = false
		return res, nil
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ARN < refs[j].ARN })
	index := newChangeIndex(changes)
	for _, ref := range refs {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return res, ctxErr
		}
		doc, err := a.reader.GetPolicyDocument(ctx, ref)
		if err != nil {
			res.UnreadPolicies = append(res.UnreadPolicies, planner.UnreadPolicy{
				SourceKind: planner.SourceIAMManaged, PolicyRef: ref.ARN, Reason: unreadReason(err, awsaccess.OpGetPolicyVersion),
			})
			res.Complete = false
			continue
		}
		conditions, err := parsePolicy(doc)
		if err != nil {
			res.UnreadPolicies = append(res.UnreadPolicies, planner.UnreadPolicy{
				SourceKind: planner.SourceIAMManaged, PolicyRef: ref.ARN, Reason: "the policy document could not be parsed",
			})
			res.Complete = false
			continue
		}
		for _, c := range conditions {
			if f, ok := index.match(ref.ARN, c); ok {
				res.Findings = append(res.Findings, f)
			}
		}
	}
	return res, nil
}

// unreadReason names the error kind and the operation, never the error text.
func unreadReason(err error, fallbackOp string) string {
	op := fallbackOp
	e, ok := errkind.As(err)
	if ok && e.Op != "" {
		op = e.Op
	}
	switch errkind.KindOf(err) {
	case errkind.AccessDenied:
		return "access denied to " + op
	case errkind.CredentialsMissing:
		return "credentials were rejected for " + op
	case errkind.Transient:
		return "AWS did not answer " + op + " (temporary failure)"
	default:
		return fmt.Sprintf("unexpected error (%s) in %s", errkind.KindOf(err), op)
	}
}

// changeIndex looks changes up by the lowercased keys they touch.
type changeIndex struct {
	changes  []planner.TagChangeInput
	byKey    map[string][]int
	keys     []string
	patterns map[string]*regexp.Regexp
}

func newChangeIndex(changes []planner.TagChangeInput) *changeIndex {
	x := &changeIndex{changes: changes, byKey: map[string][]int{}, patterns: map[string]*regexp.Regexp{}}
	for i, c := range changes {
		from, to := strings.ToLower(c.FromKey), strings.ToLower(c.ToKey)
		x.byKey[from] = append(x.byKey[from], i)
		if to != from {
			x.byKey[to] = append(x.byKey[to], i)
		}
	}
	for k := range x.byKey {
		x.keys = append(x.keys, k)
	}
	sort.Strings(x.keys)
	return x
}

// caseOnly reports whether a change differs only in letter case.
func caseOnly(c planner.TagChangeInput) bool {
	return strings.EqualFold(c.FromKey, c.ToKey)
}

// match applies BR5.3 and BR5.4 to one condition.
func (x *changeIndex) match(policyARN string, c condition) (planner.ImpactFinding, bool) {
	selected := make([]bool, len(x.changes))
	verdict := planner.VerdictAffected
	tagKey := strings.ToLower(c.tagKey)
	switch {
	case strings.Contains(tagKey, "${"):
		// A policy variable: the key is only known when IAM evaluates it.
		verdict = planner.VerdictUndecided
		for i := range selected {
			selected[i] = true
		}
	case strings.ContainsAny(tagKey, "*?"):
		re := x.pattern(tagKey)
		for _, k := range x.keys {
			if re.MatchString(k) {
				for _, i := range x.byKey[k] {
					selected[i] = true
				}
			}
		}
	default:
		for _, i := range x.byKey[tagKey] {
			selected[i] = true
		}
	}
	var ids []string
	for i, ok := range selected {
		if !ok {
			continue
		}
		// Tag keys are not case-sensitive in aws:ResourceTag, so a case-only
		// rename cannot break it. For service-specific keys this is not
		// documented, so they stay affected (BR5.3).
		if c.isAWSPrefix && caseOnly(x.changes[i]) {
			continue
		}
		ids = append(ids, x.changes[i].ChangeID)
	}
	if len(ids) == 0 {
		return planner.ImpactFinding{}, false
	}
	return planner.ImpactFinding{
		Verdict:           verdict,
		PolicyRef:         policyARN,
		SourceKind:        planner.SourceIAMManaged,
		StatementID:       c.statementID,
		ConditionOperator: c.operator,
		ConditionKey:      c.key,
		ConditionValue:    c.value,
		ChangeIDs:         ids,
	}, true
}

// pattern turns an IAM wildcard pattern into a regular expression in which
// only * and ? are special. Go regular expressions run in time linear in the
// input, so patterns such as a*a*a*a*b stay fast (NFR1.5).
func (x *changeIndex) pattern(p string) *regexp.Regexp {
	if re, ok := x.patterns[p]; ok {
		return re
	}
	var b strings.Builder
	b.WriteString(`(?s)^`)
	for _, r := range p {
		switch r {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(`$`)
	re := regexp.MustCompile(b.String())
	x.patterns[p] = re
	return re
}
