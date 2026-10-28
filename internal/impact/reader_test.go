package impact

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
)

// emptyReader is a PolicyReader of an account without policies: every list
// is empty. Test readers embed it and replace the reads they need.
type emptyReader struct{}

func (emptyReader) ReadIAMPolicies(context.Context) (awsaccess.IAMPolicies, error) {
	return awsaccess.IAMPolicies{}, nil
}
func (emptyReader) ListBuckets(context.Context) ([]awsaccess.Bucket, error) { return nil, nil }
func (emptyReader) GetBucketRegion(context.Context, string) (string, error) { return "", nil }
func (emptyReader) GetBucketPolicy(context.Context, string, string) (awsaccess.ResourcePolicy, error) {
	return awsaccess.ResourcePolicy{}, nil
}
func (emptyReader) ListKeys(context.Context, string) ([]awsaccess.KMSKey, error) { return nil, nil }
func (emptyReader) IsCustomerManagedKey(context.Context, string, awsaccess.KMSKey) (bool, error) {
	return true, nil
}
func (emptyReader) GetKeyPolicy(context.Context, string, awsaccess.KMSKey) (awsaccess.ResourcePolicy, error) {
	return awsaccess.ResourcePolicy{}, nil
}
func (emptyReader) ListQueues(context.Context, string) ([]string, error) { return nil, nil }
func (emptyReader) GetQueuePolicy(context.Context, string, string) (awsaccess.ResourcePolicy, error) {
	return awsaccess.ResourcePolicy{}, nil
}
func (emptyReader) ListTopics(context.Context, string) ([]string, error) { return nil, nil }
func (emptyReader) GetTopicPolicy(context.Context, string, string) (awsaccess.ResourcePolicy, error) {
	return awsaccess.ResourcePolicy{}, nil
}
func (emptyReader) ListSecrets(context.Context, string) ([]string, error) { return nil, nil }
func (emptyReader) GetSecretPolicy(context.Context, string, string) (awsaccess.ResourcePolicy, error) {
	return awsaccess.ResourcePolicy{}, nil
}

// accountReader is an in-memory account for the impact tests. Documents are
// given as JSON; IAM documents are URL-encoded on the way out like IAM does.
// errs fails single reads: the key is "<method>", "<method> <region>" or
// "<method> <target>", where the target is the policy reference of what is
// read. ReadIAMPolicies reads every IAM policy at once, so it fails only as
// a whole ("ReadIAMPolicies").
type accountReader struct {
	managed  map[string]string // ARN to document; attached
	boundary map[string]string // ARN to document; only a permissions boundary
	detached map[string]string // ARN to document; neither
	users    map[string]map[string]string
	roles    map[string]map[string]string
	groups   map[string]map[string]string
	// missing are the IAM policies (the ARN of a managed policy, or
	// "<principal ARN>#<name>") that ReadIAMPolicies gives without a
	// document.
	missing map[string]bool
	// resource policies by region and resource ARN; "" means none.
	buckets map[string]map[string]string
	keys    map[string]map[string]string
	queues  map[string]map[string]string
	topics  map[string]map[string]string
	secrets map[string]map[string]string
	// unlocated buckets are listed without a region.
	unlocated map[string]bool
	// awsKeys are AWS managed KMS keys.
	awsKeys map[string]bool
	errs    map[string]error

	mu    sync.Mutex
	calls map[string]int
}

func (a *accountReader) record(method string, targets ...string) error {
	a.mu.Lock()
	if a.calls == nil {
		a.calls = map[string]int{}
	}
	a.calls[method]++
	a.mu.Unlock()
	for _, t := range targets {
		if err := a.errs[method+" "+t]; err != nil {
			return err
		}
	}
	return a.errs[method]
}

func (a *accountReader) count(method string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls[method]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ReadIAMPolicies gives the managed policies (attached, then boundaries,
// then neither) and the inline policies of the users, roles and groups.
func (a *accountReader) ReadIAMPolicies(context.Context) (awsaccess.IAMPolicies, error) {
	if err := a.record("ReadIAMPolicies"); err != nil {
		return awsaccess.IAMPolicies{}, err
	}
	var out awsaccess.IAMPolicies
	for _, set := range []struct {
		docs                    map[string]string
		attachments, boundaries int
	}{{a.managed, 1, 0}, {a.boundary, 0, 1}, {a.detached, 0, 0}} {
		for _, arn := range sortedKeys(set.docs) {
			doc, missing := a.iamDocument(arn, set.docs[arn])
			out.Managed = append(out.Managed, awsaccess.ManagedPolicy{ARN: arn, AttachmentCount: set.attachments,
				BoundaryCount: set.boundaries, Document: doc, DocumentMissing: missing})
		}
	}
	for _, set := range []struct {
		kind       awsaccess.PrincipalKind
		principals map[string]map[string]string
	}{{awsaccess.PrincipalUser, a.users}, {awsaccess.PrincipalRole, a.roles}, {awsaccess.PrincipalGroup, a.groups}} {
		for _, name := range sortedKeys(set.principals) {
			p := awsaccess.IAMPrincipal{Kind: set.kind, Name: name, ARN: "arn:aws:iam::123456789012:" + string(set.kind) + "/" + name}
			for _, policy := range sortedKeys(set.principals[name]) {
				doc, missing := a.iamDocument(p.ARN+"#"+policy, set.principals[name][policy])
				out.Inline = append(out.Inline, awsaccess.InlinePolicy{Principal: p, Name: policy, Document: doc, DocumentMissing: missing})
			}
		}
	}
	return out, nil
}

// iamDocument is the document of an IAM policy as IAM gives it:
// URL-encoded, or missing.
func (a *accountReader) iamDocument(ref, doc string) (string, bool) {
	if a.missing[ref] {
		return "", true
	}
	return url.PathEscape(doc), false
}

func (a *accountReader) ListBuckets(context.Context) ([]awsaccess.Bucket, error) {
	if err := a.record("ListBuckets"); err != nil {
		return nil, err
	}
	var out []awsaccess.Bucket
	for _, region := range sortedKeys(a.buckets) {
		for _, arn := range sortedKeys(a.buckets[region]) {
			b := awsaccess.Bucket{Name: strings.TrimPrefix(arn, "arn:aws:s3:::"), Region: region}
			if a.unlocated[arn] {
				b.Region = ""
			}
			out = append(out, b)
		}
	}
	return out, nil
}

func (a *accountReader) GetBucketRegion(_ context.Context, bucket string) (string, error) {
	arn := "arn:aws:s3:::" + bucket
	if err := a.record("GetBucketRegion", arn); err != nil {
		return "", err
	}
	for region, m := range a.buckets {
		if _, ok := m[arn]; ok {
			return region, nil
		}
	}
	return "", nil
}

// policyIn returns the resource policy of arn in region.
func policyIn(m map[string]map[string]string, region, arn string) awsaccess.ResourcePolicy {
	doc := m[region][arn]
	return awsaccess.ResourcePolicy{ARN: arn, Document: doc, Found: doc != ""}
}

func (a *accountReader) GetBucketPolicy(_ context.Context, region, bucket string) (awsaccess.ResourcePolicy, error) {
	arn := "arn:aws:s3:::" + bucket
	if err := a.record("GetBucketPolicy", arn, region); err != nil {
		return awsaccess.ResourcePolicy{ARN: arn}, err
	}
	return policyIn(a.buckets, region, arn), nil
}

func (a *accountReader) ListKeys(_ context.Context, region string) ([]awsaccess.KMSKey, error) {
	if err := a.record("ListKeys", region); err != nil {
		return nil, err
	}
	var out []awsaccess.KMSKey
	for _, arn := range sortedKeys(a.keys[region]) {
		out = append(out, awsaccess.KMSKey{ID: arn[strings.LastIndex(arn, "/")+1:], ARN: arn})
	}
	return out, nil
}

func (a *accountReader) IsCustomerManagedKey(_ context.Context, region string, key awsaccess.KMSKey) (bool, error) {
	if err := a.record("IsCustomerManagedKey", key.ARN, region); err != nil {
		return false, err
	}
	return !a.awsKeys[key.ARN], nil
}

func (a *accountReader) GetKeyPolicy(_ context.Context, region string, key awsaccess.KMSKey) (awsaccess.ResourcePolicy, error) {
	if err := a.record("GetKeyPolicy", key.ARN, region); err != nil {
		return awsaccess.ResourcePolicy{ARN: key.ARN}, err
	}
	return policyIn(a.keys, region, key.ARN), nil
}

// queueURLOf turns a queue ARN of the tests into its URL.
func queueURLOf(arn string) string {
	parts := strings.Split(arn, ":")
	return "https://sqs." + parts[3] + ".amazonaws.com/" + parts[4] + "/" + parts[5]
}

func (a *accountReader) ListQueues(_ context.Context, region string) ([]string, error) {
	if err := a.record("ListQueues", region); err != nil {
		return nil, err
	}
	var out []string
	for _, arn := range sortedKeys(a.queues[region]) {
		out = append(out, queueURLOf(arn))
	}
	return out, nil
}

func (a *accountReader) GetQueuePolicy(_ context.Context, region, queueURL string) (awsaccess.ResourcePolicy, error) {
	arn := queueARN(queueURL, region)
	if err := a.record("GetQueuePolicy", arn, region); err != nil {
		return awsaccess.ResourcePolicy{}, err
	}
	return policyIn(a.queues, region, arn), nil
}

func (a *accountReader) ListTopics(_ context.Context, region string) ([]string, error) {
	if err := a.record("ListTopics", region); err != nil {
		return nil, err
	}
	return sortedKeys(a.topics[region]), nil
}

func (a *accountReader) GetTopicPolicy(_ context.Context, region, topicARN string) (awsaccess.ResourcePolicy, error) {
	if err := a.record("GetTopicPolicy", topicARN, region); err != nil {
		return awsaccess.ResourcePolicy{ARN: topicARN}, err
	}
	return policyIn(a.topics, region, topicARN), nil
}

func (a *accountReader) ListSecrets(_ context.Context, region string) ([]string, error) {
	if err := a.record("ListSecrets", region); err != nil {
		return nil, err
	}
	return sortedKeys(a.secrets[region]), nil
}

func (a *accountReader) GetSecretPolicy(_ context.Context, region, secretARN string) (awsaccess.ResourcePolicy, error) {
	if err := a.record("GetSecretPolicy", secretARN, region); err != nil {
		return awsaccess.ResourcePolicy{ARN: secretARN}, err
	}
	return policyIn(a.secrets, region, secretARN), nil
}
