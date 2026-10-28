// Package impact finds the policies that the tag changes of a plan would
// break (contract C9, U5). It reads the IAM policies in one go
// (iam:GetAccountAuthorizationDetails) and matches the customer managed
// policies that are attached or used as a permissions boundary and the
// inline policies of every user, role and group; it also reads and matches
// the resource policies of S3 buckets, KMS keys, SQS queues, SNS topics and
// Secrets Manager secrets. Every tag condition key and policy variable is
// matched against the four kinds of change.
package impact

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// How many reads of the resource policies run at once (U5 NFR1.18,
// logical-components.md). They are fixed: no option or environment variable
// changes them. Tests replace them to check that the analysis also finishes
// with one of each. IAM is read by one call at a time: the pages of
// iam:GetAccountAuthorizationDetails follow each other.
var (
	// locationWorkers read the regions of buckets the bucket list gave none.
	locationWorkers = 4
	// regionWorkers read regions; each lists its keys, queues, topics and
	// secrets in turn and then reads their policies.
	regionWorkers = 8
	// regionJobWorkers read the policies of one region.
	regionJobWorkers = 4
)

// Analyzer implements planner.ImpactAnalyzer with a read-only policy reader.
type Analyzer struct {
	reader  awsaccess.PolicyReader
	regions []string
}

var _ planner.ImpactAnalyzer = (*Analyzer)(nil)

// New returns an analyzer that reads policies through reader. Resource
// policies are looked for in regions, the regions of the plan (U5 BR1.3).
func New(reader awsaccess.PolicyReader, regions []string) *Analyzer {
	return &Analyzer{reader: reader, regions: slices.Clone(regions)}
}

// Analyze matches every policy against the changes. Policies and lists that
// cannot be read are reported in UnreadPolicies and make the result
// incomplete; the others are still matched (BR1.5). It returns an error only
// when ctx is cancelled, and then no partial result (BR5.4).
func (a *Analyzer) Analyze(ctx context.Context, changes []planner.TagChangeInput) (planner.ImpactResult, error) {
	res := planner.ImpactResult{Findings: []planner.ImpactFinding{}, UnreadPolicies: []planner.UnreadPolicy{}, Complete: true}
	if len(changes) == 0 {
		return res, nil
	}
	r := &run{
		ctx:     ctx,
		reader:  a.reader,
		regions: a.regions,
		match:   &matcher{index: newChangeIndex(changes), patterns: newPatternCache()},
	}
	var iam, resources []*jobResult
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); iam = r.readIAM() }()
	go func() { defer wg.Done(); resources = r.readResources() }()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return planner.ImpactResult{}, err
	}
	var fs []finding
	for _, j := range append(iam, resources...) {
		fs = append(fs, j.findings...)
		res.UnreadPolicies = append(res.UnreadPolicies, j.unread...)
	}
	sortFindings(fs)
	for _, f := range fs {
		res.Findings = append(res.Findings, f.toImpactFinding(changes))
	}
	sortUnread(res.UnreadPolicies)
	res.Complete = len(res.UnreadPolicies) == 0
	return res, nil
}

// run is one analysis. The change index is only read; the pattern cache has
// its own lock. Every job writes only its own jobResult.
type run struct {
	ctx     context.Context
	reader  awsaccess.PolicyReader
	regions []string
	match   *matcher
}

// jobResult is what one reading job found.
type jobResult struct {
	findings []finding
	unread   []planner.UnreadPolicy
}

func (j *jobResult) unreadable(kind, ref, region string, err error, op string) {
	j.unread = append(j.unread, planner.UnreadPolicy{SourceKind: kind, PolicyRef: ref, Region: region, Reason: unreadReason(err, op)})
}

// pool runs jobs with a limit. Only the dispatcher that owns it adds jobs, so
// a job never waits for a slot held by its own dispatcher.
type pool struct {
	ctx     context.Context
	g       errgroup.Group
	results []*jobResult
}

func newPool(ctx context.Context, limit int) *pool {
	p := &pool{ctx: ctx}
	p.g.SetLimit(limit)
	return p
}

// add runs job unless the analysis was cancelled. The jobs never return an
// error: a failure is recorded in the job's result, so it never stops the
// other jobs (NFR5.10).
func (p *pool) add(job func(*jobResult)) {
	if p.ctx.Err() != nil {
		return
	}
	j := &jobResult{}
	p.results = append(p.results, j)
	p.g.Go(func() error {
		job(j)
		return nil
	})
}

func (p *pool) wait() []*jobResult {
	_ = p.g.Wait() // the jobs never fail
	return p.results
}

// The reasons of IAM policies that the read gave without a document. They
// are fixed texts, like the reasons of documents that cannot be parsed.
const (
	reasonNoDefaultDocument = "the default version has no document in " + awsaccess.OpGetAccountAuthorizationDetails
	reasonNoInlineDocument  = "the policy has no document in " + awsaccess.OpGetAccountAuthorizationDetails
)

// readIAM is the IAM column: one read of every IAM policy, then the
// matching, one policy after the other (it needs no AWS call). A read that
// fails on any page leaves both kinds of IAM policy unread as whole lists
// and matches nothing of IAM, so that part of the account never looks
// complete (BR1.5). Managed policies are matched only when attached or used
// as a permissions boundary (BR1.1).
func (r *run) readIAM() []*jobResult {
	j := &jobResult{}
	policies, err := r.reader.ReadIAMPolicies(r.ctx)
	if err != nil {
		j.unreadable(planner.SourceIAMManaged, "", "", err, awsaccess.OpGetAccountAuthorizationDetails)
		j.unreadable(planner.SourceIAMInline, "", "", err, awsaccess.OpGetAccountAuthorizationDetails)
		return []*jobResult{j}
	}
	for _, m := range policies.Managed {
		if r.ctx.Err() != nil {
			return nil
		}
		if m.AttachmentCount == 0 && m.BoundaryCount == 0 {
			continue
		}
		r.matchIAM(j, source{kind: planner.SourceIAMManaged, ref: m.ARN}, m.Document, m.DocumentMissing, reasonNoDefaultDocument)
	}
	for _, p := range policies.Inline {
		if r.ctx.Err() != nil {
			return nil
		}
		src := source{kind: planner.SourceIAMInline, ref: p.Principal.ARN + "#" + p.Name}
		r.matchIAM(j, src, p.Document, p.DocumentMissing, reasonNoInlineDocument)
	}
	return []*jobResult{j}
}

// matchIAM matches one IAM policy, or records it as unread with reason when
// the read gave no document for it.
func (r *run) matchIAM(j *jobResult, src source, doc string, missing bool, reason string) {
	if missing {
		j.unread = append(j.unread, planner.UnreadPolicy{SourceKind: src.kind, PolicyRef: src.ref, Reason: reason})
		return
	}
	r.matchDocument(j, src, doc, true)
}

// readResources is the column of the resource policies: the bucket list,
// the regions of the buckets it gave none, then the regions.
func (r *run) readResources() []*jobResult {
	lists := &jobResult{}
	buckets, err := r.reader.ListBuckets(r.ctx)
	if err != nil {
		lists.unreadable(planner.SourceS3Bucket, "", "", err, awsaccess.OpListBuckets)
	}
	located := r.locateBuckets(buckets)
	byRegion := map[string][]string{}
	for _, b := range buckets {
		if region := located.regions[b.Name]; region != "" {
			byRegion[region] = append(byRegion[region], b.Name)
		}
	}
	p := newPool(r.ctx, regionWorkers)
	for _, region := range r.regions {
		p.add(func(j *jobResult) { r.readRegion(j, region, byRegion[region]) })
	}
	return append(append(p.wait(), located.results...), lists)
}

// locatedBuckets are the regions of the buckets, and the failures to read
// them.
type locatedBuckets struct {
	regions map[string]string
	results []*jobResult
}

// locateBuckets reads the regions the bucket list did not give. The regions
// the list gave are recorded before any job starts, because the jobs write
// to the same map.
func (r *run) locateBuckets(buckets []awsaccess.Bucket) locatedBuckets {
	located := locatedBuckets{regions: make(map[string]string, len(buckets))}
	for _, b := range buckets {
		if b.Region != "" {
			located.regions[b.Name] = b.Region
		}
	}
	var mu sync.Mutex
	p := newPool(r.ctx, locationWorkers)
	for _, b := range buckets {
		if b.Region != "" {
			continue
		}
		p.add(func(j *jobResult) {
			region, err := r.reader.GetBucketRegion(r.ctx, b.Name)
			switch {
			case errkind.KindOf(err) == errkind.NotFound:
				return
			case err != nil:
				j.unreadable(planner.SourceS3Bucket, bucketARN(b.Name), "", err, awsaccess.OpGetBucketLocation)
				return
			}
			mu.Lock()
			located.regions[b.Name] = region
			mu.Unlock()
		})
	}
	located.results = p.wait()
	return located
}

// bucketARN is the ARN of a bucket whose region is not known.
func bucketARN(name string) string { return "arn:aws:s3:::" + name }

// readRegion is the dispatcher of one region: it lists the keys, queues,
// topics and secrets in turn, then its jobs read the policies.
func (r *run) readRegion(lists *jobResult, region string, buckets []string) {
	keys, err := r.reader.ListKeys(r.ctx, region)
	if err != nil {
		lists.unreadable(planner.SourceKMSKey, "", region, err, awsaccess.OpListKeys)
	}
	queues, err := r.reader.ListQueues(r.ctx, region)
	if err != nil {
		lists.unreadable(planner.SourceSQSQueue, "", region, err, awsaccess.OpListQueues)
	}
	topics, err := r.reader.ListTopics(r.ctx, region)
	if err != nil {
		lists.unreadable(planner.SourceSNSTopic, "", region, err, awsaccess.OpListTopics)
	}
	secrets, err := r.reader.ListSecrets(r.ctx, region)
	if err != nil {
		lists.unreadable(planner.SourceSecret, "", region, err, awsaccess.OpListSecrets)
	}
	p := newPool(r.ctx, regionJobWorkers)
	for _, b := range buckets {
		p.add(func(j *jobResult) {
			policy, err := r.reader.GetBucketPolicy(r.ctx, region, b)
			r.matchResource(j, planner.SourceS3Bucket, region, policy, err, awsaccess.OpGetBucketPolicy)
		})
	}
	for _, k := range keys {
		p.add(func(j *jobResult) { r.readKey(j, region, k) })
	}
	for _, q := range queues {
		p.add(func(j *jobResult) {
			policy, err := r.reader.GetQueuePolicy(r.ctx, region, q)
			if policy.ARN == "" {
				policy.ARN = queueARN(q, region)
			}
			r.matchResource(j, planner.SourceSQSQueue, region, policy, err, awsaccess.OpGetQueueAttributes)
		})
	}
	for _, t := range topics {
		p.add(func(j *jobResult) {
			policy, err := r.reader.GetTopicPolicy(r.ctx, region, t)
			r.matchResource(j, planner.SourceSNSTopic, region, policy, err, awsaccess.OpGetTopicAttributes)
		})
	}
	for _, s := range secrets {
		p.add(func(j *jobResult) {
			policy, err := r.reader.GetSecretPolicy(r.ctx, region, s)
			r.matchResource(j, planner.SourceSecret, region, policy, err, awsaccess.OpGetResourcePolicy)
		})
	}
	results := p.wait()
	for _, j := range results {
		lists.findings = append(lists.findings, j.findings...)
		lists.unread = append(lists.unread, j.unread...)
	}
}

// readKey reads the policy of a key the account created; AWS managed keys
// are not read (BR1.4).
func (r *run) readKey(j *jobResult, region string, key awsaccess.KMSKey) {
	customer, err := r.reader.IsCustomerManagedKey(r.ctx, region, key)
	switch {
	case errkind.KindOf(err) == errkind.NotFound:
		return
	case err != nil:
		j.unreadable(planner.SourceKMSKey, key.ARN, region, err, awsaccess.OpDescribeKey)
		return
	case !customer:
		return
	}
	policy, err := r.reader.GetKeyPolicy(r.ctx, region, key)
	if policy.ARN == "" {
		policy.ARN = key.ARN
	}
	r.matchResource(j, planner.SourceKMSKey, region, policy, err, awsaccess.OpGetKeyPolicy)
}

// matchResource matches a resource policy that was read, or records why it
// could not be. A resource without a policy, or one deleted after it was
// listed, has nothing to match (BR1.3).
func (r *run) matchResource(j *jobResult, kind, region string, policy awsaccess.ResourcePolicy, err error, op string) {
	switch {
	case errkind.KindOf(err) == errkind.NotFound:
		return
	case err != nil:
		j.unreadable(kind, policy.ARN, region, err, op)
		return
	case !policy.Found:
		return
	}
	src := source{kind: kind, ref: policy.ARN, resourceARN: policy.ARN, region: region}
	r.matchDocument(j, src, policy.Document, false)
}

// queueURLPattern reads the account and the name of a queue URL.
var queueURLPattern = regexp.MustCompile(`^https://[^/]+/([0-9]{12})/([^/]+)$`)

// queueARN is the ARN of a queue whose attributes could not be read, made
// from its URL.
func queueARN(url, region string) string {
	m := queueURLPattern.FindStringSubmatch(url)
	if m == nil {
		return url
	}
	partition := "aws"
	switch {
	case strings.HasPrefix(region, "cn-"):
		partition = "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		partition = "aws-us-gov"
	}
	return "arn:" + partition + ":sqs:" + region + ":" + m[1] + ":" + m[2]
}

// matchDocument reads one document and matches its references; the
// document is dropped afterwards.
func (r *run) matchDocument(j *jobResult, src source, doc string, urlEncoded bool) {
	statements, err := parseDocument(doc, urlEncoded)
	if err != nil {
		j.unread = append(j.unread, planner.UnreadPolicy{SourceKind: src.kind, PolicyRef: src.ref, Region: src.region, Reason: err.Error()})
		return
	}
	pf := newPolicyFindings(src)
	for s := range statements {
		pf.stmtID = append(pf.stmtID, statements[s].id)
		refs := statements[s].references()
		for k := range refs {
			ref := &refs[k]
			for _, i := range r.match.candidates(src, ref) {
				pf.add(s, ref, i, r.match.judge(ref, r.match.index.changes[i]))
			}
		}
	}
	j.findings = append(j.findings, pf.findings()...)
}

// unreadReason names the error kind and the operation, never the error text
// (BR1.5, NFR8.22). fallbackOp names the operation of an error without one.
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
