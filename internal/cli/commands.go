package cli

import (
	"context"
	"io"
	"slices"
	"sync"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/fileout"
	"github.com/yucn6201g/tag-refactor/internal/impact"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// tagReaders is everything rules suggest may use to reach AWS: the account
// and the tagged resources. It has no AWS Config, Resource Explorer or IAM
// reader, so rules suggest cannot call them (U3 security design §1, NFR3.6).
type tagReaders struct {
	account awsaccess.AccountResolver
	tags    awsaccess.TagReader
}

// readers is everything scan and plan may use to reach AWS. It only has
// read-only interfaces: there is no way to hand them a writer (BR1.5, NFR3.1).
type readers struct {
	tagReaders
	policies awsaccess.PolicyReader
	config   awsaccess.ConfigReader
	explorer awsaccess.ExplorerReader
}

func (t tagReaders) accountResolver() awsaccess.AccountResolver { return t.account }
func (t tagReaders) tagReader() awsaccess.TagReader             { return t.tags }

// readerSet is a set of readers built for one command; readers has the
// methods of the tagReaders it embeds.
type readerSet interface {
	accountResolver() awsaccess.AccountResolver
	tagReader() awsaccess.TagReader
}

// tagReadersOf is the first stage of connecting: the account and the tagged
// resources only.
func tagReadersOf(s *awsaccess.Session) tagReaders {
	return tagReaders{account: s, tags: s}
}

// allReadersOf is the second stage: the first stage plus the policies, AWS
// Config and Resource Explorer readers that scan and plan use.
func allReadersOf(s *awsaccess.Session) readers {
	return readers{tagReaders: tagReadersOf(s), policies: s, config: s, explorer: s}
}

// connect builds the AWS session and, through stage, the readers the command
// may use, and identifies the account (BR1.1 to BR1.3). The session's region
// is the first --region value, or the profile's region: STS, EC2 and IAM are
// asked there.
func connect[R readerSet](ctx context.Context, a *app, profile string, regions []string,
	stage func(*awsaccess.Session) R) (R, awsaccess.AccountContext, error) {
	var none R
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
		return none, awsaccess.AccountContext{}, err
	}
	r := stage(session)
	acc, err := r.accountResolver().Resolve(ctx)
	if err != nil {
		return none, awsaccess.AccountContext{}, err
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

// collect connects with the readers of stage and reads the tagged resources
// of the regions, under one cancellation shared by the whole run
// (reliability design §4).
func collect[R readerSet](ctx context.Context, a *app, profile string, requested []string,
	stage func(*awsaccess.Session) R) (R, *inventory.Snapshot, error) {
	var none R
	if err := checkRegionNames(requested); err != nil {
		return none, nil, err
	}
	r, acc, err := connect(ctx, a, profile, requested, stage)
	if err != nil {
		return none, nil, err
	}
	regions, err := regionsToRead(acc, requested)
	if err != nil {
		return none, nil, err
	}
	snap, err := inventory.Collect(ctx, r.tagReader(), a.p, acc, regions, a.deps.Now())
	if err != nil {
		return none, nil, err
	}
	return r, snap, nil
}

// collectAll is collect with every reader scan and plan use.
func (a *app) collectAll(ctx context.Context, profile string, requested []string) (readers, *inventory.Snapshot, error) {
	return collect(ctx, a, profile, requested, allReadersOf)
}

// loadRules reads the rule file before anything calls AWS, so a broken file
// stops the command without a request (NFR3.7), and prints its warnings. The
// inventory's type table names the form to write for a CloudFormation type
// (U3 BR3.4).
func (a *app) loadRules(path string) (*rules.RuleSet, error) {
	rs, err := rules.Load(path, rules.Options{TypeHint: inventory.ARNFormOf})
	if err != nil {
		return nil, err
	}
	for _, w := range rs.Warnings {
		a.p.FileWarning(path, w.Line, w.Message)
	}
	return rs, nil
}

// requiredKeys returns the required keys scan counts. The rule file reader
// has already left out keys starting with aws:, with a warning (U3 BR5.1).
func requiredKeys(rs *rules.RuleSet) []string {
	keys := make([]string, 0, len(rs.RequiredTags))
	for _, req := range rs.RequiredTags {
		keys = append(keys, req.Key)
	}
	return keys
}

// runScan is workflow W1, with the result in the given format (U4 W3).
func (a *app) runScan(ctx context.Context, rulesPath, profile string, regions []string, format display.Format) error {
	a.started = a.deps.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var required []string
	if rulesPath != "" {
		rs, err := a.loadRules(rulesPath)
		if err != nil {
			return err
		}
		required = requiredKeys(rs)
	}
	r, snap, err := a.collectAll(ctx, profile, regions)
	if err != nil {
		return err
	}
	if err := inventory.Discover(ctx, r.config, r.explorer, a.p, snap); err != nil {
		return err
	}
	report := inventory.NewScanReport(snap, rulesPath, required)
	a.result = &outcome{
		command:        "scan",
		regionsMissing: !snap.Complete(),
		needsReview:    report.IssuesFound(),
		resources:      len(snap.Resources),
	}
	code, _ := a.result.exitCode()
	a.p.ScanResult(report, display.Output{Format: format, ExitCode: code, ToolVersion: toolVersion()})
	a.p.ScanScope(snap)
	return nil
}

// planArgs are the options of plan.
type planArgs struct {
	rules, profile string
	regions        []string
	format         display.Format
	// out is the --out file, or "".
	out string
}

// planPrefix starts the name of the temporary file of plan --out.
const planPrefix = ".tag-refactor-plan-"

// runPlan is workflow W1 of U4 (W2 of U1). It reads every region like scan
// but does not look for never-tagged resources: only tagged resources are
// changed (BR5.6). The options are checked in the order of the U4 security
// design §1, so a mistake stops the command before any AWS call: --format
// and --lang (already checked), --out, the rule file, then the connection.
// With --out the plan is saved before the result is printed (W2).
func (a *app) runPlan(ctx context.Context, args planArgs) error {
	a.started = a.deps.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if args.rules == "" {
		l := a.p.Lang()
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage,
			Detail: display.T(l, "err.planNeedsRules"), Hint: display.T(l, "err.planNeedsRulesFix")}
	}
	if args.out != "" {
		if err := fileout.Prepare(args.out, planPrefix); err != nil {
			return withOption(err, "--out")
		}
	}
	rs, err := a.loadRules(args.rules)
	if err != nil {
		return err
	}
	r, snap, err := a.collectAll(ctx, args.profile, args.regions)
	if err != nil {
		return err
	}
	plan := planner.Build(snap, rs, planner.Options{Now: a.deps.Now(), RuleFile: args.rules, ToolVersion: toolVersion()})
	if plan.NeedsAnalysis() {
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
	if args.out != "" {
		if err := a.saveFile(ctx, args.out, planPrefix, planner.MaxPlanBytes, func(w io.Writer) error {
			safe := planner.NewSafeJSONWriter(w)
			if err := planner.WritePlan(safe, plan); err != nil {
				return err
			}
			return safe.Close()
		}); err != nil {
			return withOption(err, "--out")
		}
		a.p.PlanSaved(args.out, plan)
	}
	a.result = &outcome{
		command:        "plan",
		regionsMissing: !plan.InventoryComplete,
		incomplete:     !plan.ImpactAnalysisComplete,
		needsReview:    plan.NeedsReview(),
		resources:      plan.ScannedResources,
	}
	if args.out == "" && plan.NeedsAnalysis() {
		a.result.saveHint = args.rules
	}
	code, _ := a.result.exitCode()
	if args.format == display.FormatTable || args.format == display.FormatDiff {
		a.p.PlanHeader(plan)
	}
	a.p.PlanResult(plan, display.Output{Format: args.format, ExitCode: code, ToolVersion: plan.ToolVersion})
	a.p.PlanScope(snap, plan.NeedsAnalysis())
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
