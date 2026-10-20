package awsaccess

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	rgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Page sizes: the largest values the APIs accept (NFR1.3).
const (
	tagPageSize    = 100  // tag:GetResources ResourcesPerPage maximum
	policyPageSize = 1000 // iam:ListPolicies MaxItems maximum
)

// ListTaggedResources reads every tagged resource of the region. It keeps
// reading while the response carries a pagination token, even after an empty
// page (NFR1.3).
func (s *Session) ListTaggedResources(ctx context.Context, region string) ([]TaggedResource, error) {
	client := rgt.NewFromConfig(s.cfg, func(o *rgt.Options) { o.Region = region })
	var resources []TaggedResource
	seen := map[string]bool{}
	token := ""
	for {
		in := &rgt.GetResourcesInput{ResourcesPerPage: aws.Int32(tagPageSize)}
		if token != "" {
			in.PaginationToken = aws.String(token)
		}
		var out *rgt.GetResourcesOutput
		err := s.invoke(ctx, OpGetResources, region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = client.GetResources(ctx, in, func(o *rgt.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, m := range out.ResourceTagMappingList {
			tags := make(map[string]string, len(m.Tags))
			for _, t := range m.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			resources = append(resources, TaggedResource{ARN: aws.ToString(m.ResourceARN), Tags: tags})
		}
		token = aws.ToString(out.PaginationToken)
		if token == "" {
			return resources, nil
		}
		if seen[token] {
			return nil, &errkind.Error{Kind: errkind.Internal, Op: OpGetResources, Region: region,
				Detail: "the tagging API repeated a pagination token"}
		}
		seen[token] = true
	}
}

// ListAttachedCustomerPolicies reads the customer managed policies attached
// to a user, role or group (BR5.1). OnlyAttached makes a separate read of the
// attachments unnecessary.
func (s *Session) ListAttachedCustomerPolicies(ctx context.Context) ([]IAMPolicyRef, error) {
	var refs []IAMPolicyRef
	marker := ""
	for {
		in := &iam.ListPoliciesInput{
			Scope:        iamtypes.PolicyScopeTypeLocal,
			OnlyAttached: true,
			MaxItems:     aws.Int32(policyPageSize),
		}
		if marker != "" {
			in.Marker = aws.String(marker)
		}
		var out *iam.ListPoliciesOutput
		err := s.invoke(ctx, OpListPolicies, s.region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = s.iam.ListPolicies(ctx, in, func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, p := range out.Policies {
			refs = append(refs, IAMPolicyRef{ARN: aws.ToString(p.Arn), DefaultVersionID: aws.ToString(p.DefaultVersionId)})
		}
		if !out.IsTruncated {
			return refs, nil
		}
		next := aws.ToString(out.Marker)
		if next == "" || next == marker {
			return nil, &errkind.Error{Kind: errkind.Internal, Op: OpListPolicies, Region: s.region,
				Detail: "IAM returned a truncated policy list without a new marker"}
		}
		marker = next
	}
}

// GetPolicyDocument reads the default version of a policy. The document is
// returned as IAM sends it: URL-encoded JSON.
func (s *Session) GetPolicyDocument(ctx context.Context, ref IAMPolicyRef) (string, error) {
	var out *iam.GetPolicyVersionOutput
	err := s.invoke(ctx, OpGetPolicyVersion, s.region, func(ctx context.Context, add apiOption) error {
		var err error
		out, err = s.iam.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
			PolicyArn: aws.String(ref.ARN),
			VersionId: aws.String(ref.DefaultVersionID),
		}, func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return "", err
	}
	if out.PolicyVersion == nil {
		return "", &errkind.Error{Kind: errkind.Internal, Op: OpGetPolicyVersion, Region: s.region,
			Detail: "IAM returned no policy version"}
	}
	return aws.ToString(out.PolicyVersion.Document), nil
}
