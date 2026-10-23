// Package awsaccess is the only part of tag-refactor that talks to AWS
// (contract C6). It builds the SDK configuration, applies time limits and the
// SDK's standard retries, converts SDK errors into error kinds, and exposes
// read-only interfaces: tags, IAM policies, AWS Config and Resource Explorer.
// The write interface is a separate type and has no implementation yet, so
// scan and plan cannot be given one (NFR3.1).
package awsaccess

import (
	"context"
	"time"
)

// Operation names, written as API operations with the IAM service prefix
// ("service:Action"). For every operation but ListResources the IAM action
// has the same name; see Operations.
const (
	OpGetCallerIdentity                   = "sts:GetCallerIdentity"
	OpDescribeRegions                     = "ec2:DescribeRegions"
	OpGetResources                        = "tag:GetResources"
	OpListPolicies                        = "iam:ListPolicies"
	OpGetPolicyVersion                    = "iam:GetPolicyVersion"
	OpDescribeConfigurationRecorders      = "config:DescribeConfigurationRecorders"
	OpDescribeConfigurationRecorderStatus = "config:DescribeConfigurationRecorderStatus"
	OpSelectResourceConfig                = "config:SelectResourceConfig"
	OpListIndexes                         = "resource-explorer-2:ListIndexes"
	OpGetDefaultView                      = "resource-explorer-2:GetDefaultView"
	OpGetView                             = "resource-explorer-2:GetView"
	OpListResources                       = "resource-explorer-2:ListResources"
)

// Operation is one AWS operation tag-refactor calls (security design §1).
type Operation struct {
	// API is the operation called, for example
	// "resource-explorer-2:ListResources". The fake AWS of the tests records
	// this name, and --debug prints it.
	API string
	// IAMAction is the IAM action AWS checks for the call, for example
	// "resource-explorer-2:Search" for ListResources. The example policy
	// lists this name, and AccessDenied messages show it.
	IAMAction string
	// NeedsPermission is false for operations that work without any IAM
	// permission (sts:GetCallerIdentity).
	NeedsPermission bool
}

// Operations lists every AWS operation scan and plan call. The example
// read-only policy docs/iam/readonly-policy.json must allow exactly the IAM
// actions of the operations that need a permission; a test compares the two
// so they cannot drift apart. The IAM names were checked against the Service
// Authorization Reference: only ListResources differs from its API name.
var Operations = []Operation{
	{API: OpGetCallerIdentity, IAMAction: OpGetCallerIdentity, NeedsPermission: false},
	{API: OpDescribeRegions, IAMAction: OpDescribeRegions, NeedsPermission: true},
	{API: OpGetResources, IAMAction: OpGetResources, NeedsPermission: true},
	{API: OpListPolicies, IAMAction: OpListPolicies, NeedsPermission: true},
	{API: OpGetPolicyVersion, IAMAction: OpGetPolicyVersion, NeedsPermission: true},
	{API: OpDescribeConfigurationRecorders, IAMAction: OpDescribeConfigurationRecorders, NeedsPermission: true},
	{API: OpDescribeConfigurationRecorderStatus, IAMAction: OpDescribeConfigurationRecorderStatus, NeedsPermission: true},
	{API: OpSelectResourceConfig, IAMAction: OpSelectResourceConfig, NeedsPermission: true},
	{API: OpListIndexes, IAMAction: OpListIndexes, NeedsPermission: true},
	{API: OpGetDefaultView, IAMAction: OpGetDefaultView, NeedsPermission: true},
	{API: OpGetView, IAMAction: OpGetView, NeedsPermission: true},
	{API: OpListResources, IAMAction: "resource-explorer-2:Search", NeedsPermission: true},
}

// RequiredPermissions returns the IAM actions scan and plan need, in table
// order.
func RequiredPermissions() []string {
	var actions []string
	for _, op := range Operations {
		if op.NeedsPermission {
			actions = append(actions, op.IAMAction)
		}
	}
	return actions
}

// IAMAction returns the IAM action AWS checks for the API operation api, so
// that an AccessDenied names what the user has to allow. Names outside the
// table are returned unchanged.
func IAMAction(api string) string {
	for _, op := range Operations {
		if op.API == api {
			return op.IAMAction
		}
	}
	return api
}

// AccountContext is the account and region a command works on.
type AccountContext struct {
	AccountID string
	// CallerARN identifies the caller. It contains no credentials.
	CallerARN string
	// Profile is the profile name, empty for the SDK's default resolution.
	Profile string
	// Region is the region of the session: STS, EC2 and IAM are asked there.
	Region string
	// EnabledRegions are the regions enabled for the account, sorted.
	EnabledRegions []string
}

// TaggedResource is one resource returned by the tagging API, with all its
// tags including the aws: ones.
type TaggedResource struct {
	ARN  string
	Tags map[string]string
}

// IAMPolicyRef identifies a customer managed policy and its default version.
type IAMPolicyRef struct {
	ARN              string
	DefaultVersionID string
}

// AccountResolver checks the credentials and identifies the account (C6).
type AccountResolver interface {
	Resolve(ctx context.Context) (AccountContext, error)
}

// TagReader reads tags (C6).
type TagReader interface {
	// ListTaggedResources returns every tagged resource of the region, reading
	// all pages.
	ListTaggedResources(ctx context.Context, region string) ([]TaggedResource, error)
}

// PolicyReader reads IAM policies (C6, minimal U1 version). Listing and
// reading one document are separate so that one unreadable policy does not
// stop the others (BR5.5).
type PolicyReader interface {
	// ListAttachedCustomerPolicies returns the customer managed policies that
	// are attached to at least one user, role or group, reading all pages.
	ListAttachedCustomerPolicies(ctx context.Context) ([]IAMPolicyRef, error)
	// GetPolicyDocument returns the default version's document as IAM returns
	// it (URL-encoded JSON).
	GetPolicyDocument(ctx context.Context, ref IAMPolicyRef) (string, error)
}

// ConfigRecorder is one AWS Config configuration recorder of a region with
// the parts of its recording group that tell what it records.
type ConfigRecorder struct {
	Name string
	// AllSupported is recordingGroup.allSupported.
	AllSupported bool
	// Strategy is recordingGroup.recordingStrategy.useOnly, empty when the
	// recorder does not set it.
	Strategy string
	// IncludedTypes and ExcludedTypes count the resource types listed for
	// INCLUSION_BY_RESOURCE_TYPES and EXCLUSION_BY_RESOURCE_TYPES.
	IncludedTypes int
	ExcludedTypes int
	// Recording is the recording flag of the recorder's status.
	Recording bool
}

// ConfigResource is one result of the AWS Config query.
type ConfigResource struct {
	// ARN is empty when AWS Config has no ARN for the resource.
	ARN string
	// ResourceType is the AWS Config form, for example AWS::EC2::Instance.
	ResourceType string
	ResourceID   string
	Region       string
	// Tags holds every tag, including those that start with aws:.
	Tags map[string]string
}

// ConfigReader reads AWS Config, the first way to find never-tagged
// resources (BR3.1). It never starts or changes a recorder (NFR3.4).
type ConfigReader interface {
	// ConfigRecorders returns the recorders of the region with their
	// recording status. A region without AWS Config has none.
	ConfigRecorders(ctx context.Context, region string) ([]ConfigRecorder, error)
	// ConfigResources returns every resource AWS Config records in the
	// region, reading all pages.
	ConfigResources(ctx context.Context, region string) ([]ConfigResource, error)
}

// ExplorerIndex is one Resource Explorer index.
type ExplorerIndex struct {
	Region string
	// Aggregator is true for the index that covers every indexed region.
	Aggregator bool
}

// ExplorerView is the default view of a region.
type ExplorerView struct {
	ARN string
	// Filter is the view's filter string; empty when the view shows every
	// resource.
	Filter string
}

// ExplorerResource is one resource found by Resource Explorer.
type ExplorerResource struct {
	ARN string
	// ResourceType is the ARN form, for example sqs:queue.
	ResourceType string
	// Region is the region of the resource; "global" for global ones.
	Region string
}

// ExplorerReader reads Resource Explorer, the second way to find
// never-tagged resources (BR3.1). It never calls the API Search, which stops
// at 1,000 results, and never creates indexes or views (NFR3.4).
type ExplorerReader interface {
	// ExplorerIndexes returns the indexes of every region, reading all pages.
	ExplorerIndexes(ctx context.Context) ([]ExplorerIndex, error)
	// DefaultView returns the default view of the region and its filter. A
	// region without a default view is an AccessDenied with the reason
	// no-default-view.
	DefaultView(ctx context.Context, region string) (ExplorerView, error)
	// UntaggedResources returns every resource of the view without tags
	// (tag:none), reading all pages through ListResources.
	UntaggedResources(ctx context.Context, region, viewARN string) ([]ExplorerResource, error)
}

// TagWriter is the write interface of contract C6. U1 has no implementation
// and scan and plan never receive one; U6 and U7 will implement it.
type TagWriter interface {
	TagResources(ctx context.Context, region string, arns []string, tags map[string]string) error
	UntagResources(ctx context.Context, region string, arns []string, keys []string) error
}

// CallRecord describes one AWS API call for the --debug output. It carries no
// arguments, response content, error text or credentials (NFR8.2).
type CallRecord struct {
	Op       string
	Region   string
	Duration time.Duration
	// Retries is the number of retries, or -1 when it is not known.
	Retries int
	// ErrorKind is the error kind of a failed call, empty on success.
	ErrorKind string
	// CauseType is the Go type of the cause of an Internal error.
	CauseType string
}

// DebugSink receives one record per AWS API call when --debug is on.
type DebugSink interface {
	APICall(rec CallRecord)
}
