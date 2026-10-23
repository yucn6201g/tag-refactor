package cli

import (
	"context"
	"slices"
	"sync"

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
	config   awsaccess.ConfigReader
	explorer awsaccess.ExplorerReader
}

// connect builds the AWS session and identifies the account (BR1.1 to BR1.3).
// The session's region is the first --region value, or the profile's region:
// STS, EC2 and IAM are asked there.
func (a *app) connect(ctx context.Context, profile string, regions []string) (readers, awsaccess.AccountContext, error) {
	opts := awsaccess.Options{
		Profile:           profile,
		Transport:         a.deps.Transport,
		Now:               a.deps.Now,
		AttemptTimeout:    a.deps.AttemptTimeout,
		CallTimeout:       a.deps.CallTimeout,
		CredentialTimeout: a.deps.CredentialTimeout,
		RetryDelay:        a.deps.RetryDelay,
		ExplorerPageSize:  a.deps.ExplorerPageSize,
	}
	if len(regions) > 0 {
		opts.Region = regions[0]
	}
	if a.debug {
		opts.Debug = a.p
	}
	session, err := awsaccess.NewSession(ctx, opts)
	if err != nil {
		return readers{}, awsaccess.AccountContext{}, err
	}
	r := readers{account: session, tags: session, policies: session, config: session, explorer: session}
	acc, err := r.account.Resolve(ctx)
	if err != nil {
		return readers{}, awsaccess.AccountContext{}, err
	}
	a.p.Account(acc)
	return r, acc, nil
}

// checkRegionNames rejects a --region value that is not a region name before
// anything else happens (BR1.1). One value per --region: a comma-separated
// list is not a region name.
func checkRegionNames(regions []string) error {
	for _, r := range regions {
		if !awsaccess.IsRegionName(r) {
			return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionInvalid, Region: r}
		}
	}
	return nil
}

// regionsToRead returns the regions to read: every enabled region, or the
// --region values without repeats. A value that is not enabled for the
// account is an error before any resource is read (BR1.1).
func regionsToRead(acc awsaccess.AccountContext, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return acc.EnabledRegions, nil
	}
	for _, r := range requested {
		if !slices.Contains(acc.EnabledRegions, r) {
			return nil, &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionUnavailable, Region: r}
		}
	}
	out := slices.Clone(requested)
	slices.Sort(out)
	return slices.Compact(out), nil
}

// collect connects and reads the tagged resources of the regions, under one
// cancellation shared by the whole run (reliability design §4).
func (a *app) collect(ctx context.Context, profile string, requested []string) (readers, *inventory.Snapshot, error) {
	if err := checkRegionNames(requested); err != nil {
		return readers{}, nil, err
	}
	r, acc, err := a.connect(ctx, profile, requested)
	if err != nil {
		return readers{}, nil, err
	}
	regions, err := regionsToRead(acc, requested)
	if err != nil {
		return readers{}, nil, err
	}
	snap, err := inventory.Collect(ctx, r.tags, a.p, acc, regions, a.deps.Now())
	if err != nil {
		return readers{}, nil, err
	}
	return r, snap, nil
}

// requiredKeys returns the required keys scan counts. Keys starting with aws:
// belong to AWS: they are left out with a warning (BR5.2).
func (a *app) requiredKeys(rs *rules.RuleSet) []string {
	keys := []string{}
	for _, req := range rs.RequiredTags {
		if inventory.IsAWSReservedKey(req.Key) {
			a.p.FileWarning(rs.SourcePath, req.Line,
				`the required key "`+req.Key+`" is ignored: keys that start with aws: belong to AWS`)
			continue
		}
		keys = append(keys, req.Key)
	}
	return keys
}

// runScan is workflow W1.
func (a *app) runScan(ctx context.Context, rulesPath, profile string, regions []string) error {
	a.started = a.deps.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var required []string
	if rulesPath != "" {
		rs, err := rules.Load(rulesPath)
		if err != nil {
			return err
		}
		for _, w := range rs.Warnings {
			// scan uses the required keys, so it does not say they are unused.
			if w.Key != rules.RequiredNotUsedKey {
				a.p.FileWarning(rulesPath, w.Line, w.Message)
			}
		}
		required = a.requiredKeys(rs)
	}
	r, snap, err := a.collect(ctx, profile, regions)
	if err != nil {
		return err
	}
	if err := inventory.Discover(ctx, r.config, r.explorer, a.p, snap); err != nil {
		return err
	}
	report := inventory.NewScanReport(snap, rulesPath, required)
	a.p.ScanResult(report)
	a.p.ScanScope(snap)
	a.result = &outcome{
		command:        "scan",
		regionsMissing: !snap.Complete(),
		needsReview:    report.IssuesFound(),
		resources:      len(snap.Resources),
	}
	return nil
}

// runPlan is workflow W2. It reads every region like scan but does not look
// for never-tagged resources: only tagged resources are renamed (BR5.6).
func (a *app) runPlan(ctx context.Context, rulesPath, profile string, regions []string) error {
	a.started = a.deps.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
	r, snap, err := a.collect(ctx, profile, regions)
	if err != nil {
		return err
	}
	plan := planner.Build(snap, rs, a.deps.Now())
	if len(plan.Changes) > 0 {
		policies := &stoppingPolicyReader{inner: r.policies, cancel: cancel}
		var analyzer planner.ImpactAnalyzer = impact.New(policies)
		result, err := analyzer.Analyze(ctx, plan.ImpactInputs())
		if stop := policies.stopped(); stop != nil {
			return stop
		}
		if err != nil {
			return err
		}
		plan.AttachImpact(result)
	}
	a.p.ResultsIncomplete(snap.FailedRegions)
	a.p.PlanResult(plan)
	a.p.PlanScope(snap, len(plan.Changes) > 0)
	a.result = &outcome{
		command:        "plan",
		regionsMissing: !snap.Complete(),
		incomplete:     !plan.ImpactAnalysisComplete,
		needsReview:    plan.NeedsReview(),
		resources:      plan.ScannedResources,
	}
	return nil
}

// stoppingPolicyReader stops the run when reading a policy finds missing or
// expired credentials: the impact analysis would otherwise report those
// policies as unread and show a partial plan (NFR5.8, NFR8.11). It cancels
// the run's context, which makes the analysis return, and keeps the error.
type stoppingPolicyReader struct {
	inner  awsaccess.PolicyReader
	cancel context.CancelFunc
	mu     sync.Mutex
	err    error
}

func (s *stoppingPolicyReader) check(err error) error {
	if errkind.KindOf(err) == errkind.CredentialsMissing {
		s.mu.Lock()
		if s.err == nil {
			s.err = err
		}
		s.mu.Unlock()
		s.cancel()
	}
	return err
}

func (s *stoppingPolicyReader) stopped() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *stoppingPolicyReader) ListAttachedCustomerPolicies(ctx context.Context) ([]awsaccess.IAMPolicyRef, error) {
	refs, err := s.inner.ListAttachedCustomerPolicies(ctx)
	return refs, s.check(err)
}

func (s *stoppingPolicyReader) GetPolicyDocument(ctx context.Context, ref awsaccess.IAMPolicyRef) (string, error) {
	doc, err := s.inner.GetPolicyDocument(ctx, ref)
	return doc, s.check(err)
}
