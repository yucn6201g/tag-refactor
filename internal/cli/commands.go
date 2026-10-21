package cli

import (
	"context"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/impact"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// readers is everything scan and plan may use to reach AWS. It only has
// read-only interfaces: there is no way to hand them a writer (BR1.5, NFR3.1).
type readers struct {
	account  awsaccess.AccountResolver
	tags     awsaccess.TagReader
	policies awsaccess.PolicyReader
}

// connect builds the AWS session and identifies the account (BR1.1 to BR1.3).
func (a *app) connect(ctx context.Context, profile, region string) (readers, awsaccess.AccountContext, error) {
	opts := awsaccess.Options{
		Profile:           profile,
		Region:            region,
		Transport:         a.deps.Transport,
		Now:               a.deps.Now,
		AttemptTimeout:    a.deps.AttemptTimeout,
		CallTimeout:       a.deps.CallTimeout,
		CredentialTimeout: a.deps.CredentialTimeout,
		RetryDelay:        a.deps.RetryDelay,
	}
	if a.debug {
		opts.Debug = a.p
	}
	session, err := awsaccess.NewSession(ctx, opts)
	if err != nil {
		return readers{}, awsaccess.AccountContext{}, err
	}
	r := readers{account: session, tags: session, policies: session}
	acc, err := r.account.Resolve(ctx)
	if err != nil {
		return readers{}, awsaccess.AccountContext{}, err
	}
	a.p.Account(acc)
	return r, acc, nil
}

// runScan is workflow W1.
func (a *app) runScan(ctx context.Context, profile, region string) error {
	a.started = a.deps.Now()
	r, acc, err := a.connect(ctx, profile, region)
	if err != nil {
		return err
	}
	snap, err := inventory.Collect(ctx, r.tags, a.p, acc.AccountID, []string{acc.Region}, a.deps.Now())
	if err != nil {
		return err
	}
	groups := inventory.GroupKeys(snap.Resources)
	a.p.ScanResult(snap, groups)
	a.p.ScanScope(snap.ScannedRegions)
	a.result = &outcome{command: "scan", needsReview: len(inventory.Variants(groups)) > 0, resources: len(snap.Resources)}
	return nil
}

// runPlan is workflow W2.
func (a *app) runPlan(ctx context.Context, rulesPath, profile, region string) error {
	a.started = a.deps.Now()
	if rulesPath == "" {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
			Detail: "plan needs a rule file", Hint: "pass it with --rules, for example: tag-refactor plan --rules tagrules.yaml"}
	}
	rs, err := rules.Load(rulesPath)
	if err != nil {
		return err
	}
	for _, w := range rs.Warnings {
		a.p.FileWarning(rulesPath, w.Line, w.Message)
	}
	r, acc, err := a.connect(ctx, profile, region)
	if err != nil {
		return err
	}
	snap, err := inventory.Collect(ctx, r.tags, a.p, acc.AccountID, []string{acc.Region}, a.deps.Now())
	if err != nil {
		return err
	}
	plan := planner.Build(snap, rs, a.deps.Now())
	if len(plan.Changes) > 0 {
		var analyzer planner.ImpactAnalyzer = impact.New(r.policies)
		result, err := analyzer.Analyze(ctx, plan.ImpactInputs())
		if err != nil {
			return err
		}
		plan.AttachImpact(result)
	}
	a.p.PlanResult(plan)
	a.p.PlanScope(snap.ScannedRegions, len(plan.Changes) > 0)
	a.result = &outcome{
		command:     "plan",
		incomplete:  !plan.ImpactAnalysisComplete,
		needsReview: plan.NeedsReview(),
		resources:   plan.ScannedResources,
	}
	return nil
}
