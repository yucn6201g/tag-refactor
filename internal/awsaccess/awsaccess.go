// Package awsaccess is the only part of tag-refactor that talks to AWS
// (contract C6). It builds the SDK configuration, applies time limits and the
// SDK's standard retries, converts SDK errors into error kinds, and exposes
// read-only interfaces. The write interface is a separate type and has no
// implementation in U1, so scan and plan cannot be given one (NFR3.1).
package awsaccess

import (
	"context"
	"time"
)

// Operation names, written as IAM actions.
const (
	OpGetCallerIdentity = "sts:GetCallerIdentity"
	OpDescribeRegions   = "ec2:DescribeRegions"
	OpGetResources      = "tag:GetResources"
	OpListPolicies      = "iam:ListPolicies"
	OpGetPolicyVersion  = "iam:GetPolicyVersion"
)

// Operation is one AWS operation tag-refactor calls.
type Operation struct {
	// Name is the IAM action, for example "tag:GetResources".
	Name string
	// NeedsPermission is false for operations that work without any IAM
	// permission (sts:GetCallerIdentity).
	NeedsPermission bool
}

// Operations lists every AWS operation U1 calls. The example read-only policy
// docs/iam/readonly-policy.json must allow exactly the operations that need a
// permission; a test compares the two so they cannot drift apart.
var Operations = []Operation{
	{Name: OpGetCallerIdentity, NeedsPermission: false},
	{Name: OpDescribeRegions, NeedsPermission: true},
	{Name: OpGetResources, NeedsPermission: true},
	{Name: OpListPolicies, NeedsPermission: true},
	{Name: OpGetPolicyVersion, NeedsPermission: true},
}

// RequiredPermissions returns the IAM actions U1 needs, in table order.
func RequiredPermissions() []string {
	var actions []string
	for _, op := range Operations {
		if op.NeedsPermission {
			actions = append(actions, op.Name)
		}
	}
	return actions
}

// AccountContext is the account and region a command works on.
type AccountContext struct {
	AccountID string
	// CallerARN identifies the caller. It contains no credentials.
	CallerARN string
	// Profile is the profile name, empty for the SDK's default resolution.
	Profile string
	Region  string
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

// TagReader reads tags (C6, minimal U1 version).
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
