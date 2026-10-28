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
	tagPageSize     = 100  // tag:GetResources ResourcesPerPage maximum
	detailsPageSize = 1000 // iam:GetAccountAuthorizationDetails MaxItems maximum
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

// detailsFilter asks iam:GetAccountAuthorizationDetails for the users, roles
// and groups with their inline policies and for the customer managed
// policies, whether they are used or not: the caller chooses by the counts,
// since a policy used only as a permissions boundary has no attachment (U5
// BR1.1). AWS managed policies are not read.
var detailsFilter = []iamtypes.EntityType{
	iamtypes.EntityTypeUser, iamtypes.EntityTypeRole, iamtypes.EntityTypeGroup, iamtypes.EntityTypeLocalManagedPolicy,
}

// ReadIAMPolicies reads every customer managed policy and every inline
// policy of the account with iam:GetAccountAuthorizationDetails (U5 BR1.1,
// BR1.2). IAM may give fewer items than asked for while more follow, so it
// reads until a page is not truncated. Every page goes through invoke, so
// the time limits, the retries and the --debug lines apply.
func (s *Session) ReadIAMPolicies(ctx context.Context) (IAMPolicies, error) {
	var all IAMPolicies
	marker := ""
	for {
		in := &iam.GetAccountAuthorizationDetailsInput{Filter: detailsFilter, MaxItems: aws.Int32(detailsPageSize)}
		if marker != "" {
			in.Marker = aws.String(marker)
		}
		var out *iam.GetAccountAuthorizationDetailsOutput
		err := s.invoke(ctx, OpGetAccountAuthorizationDetails, s.region, func(ctx context.Context, add apiOption) error {
			var err error
			out, err = s.iam.GetAccountAuthorizationDetails(ctx, in, func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) })
			return err
		})
		if err != nil {
			return IAMPolicies{}, err
		}
		all.add(out)
		if !out.IsTruncated {
			return all, nil
		}
		next := aws.ToString(out.Marker)
		if next == "" || next == marker {
			return IAMPolicies{}, &errkind.Error{Kind: errkind.Internal, Op: OpGetAccountAuthorizationDetails, Region: s.region,
				Detail: "IAM returned a truncated page without a new marker"}
		}
		marker = next
	}
}

// add appends the policies of one page.
func (p *IAMPolicies) add(out *iam.GetAccountAuthorizationDetailsOutput) {
	for _, m := range out.Policies {
		p.Managed = append(p.Managed, managedPolicy(m))
	}
	for _, u := range out.UserDetailList {
		p.addInline(IAMPrincipal{Kind: PrincipalUser, Name: aws.ToString(u.UserName), ARN: aws.ToString(u.Arn)}, u.UserPolicyList)
	}
	for _, r := range out.RoleDetailList {
		p.addInline(IAMPrincipal{Kind: PrincipalRole, Name: aws.ToString(r.RoleName), ARN: aws.ToString(r.Arn)}, r.RolePolicyList)
	}
	for _, g := range out.GroupDetailList {
		p.addInline(IAMPrincipal{Kind: PrincipalGroup, Name: aws.ToString(g.GroupName), ARN: aws.ToString(g.Arn)}, g.GroupPolicyList)
	}
}

func (p *IAMPolicies) addInline(principal IAMPrincipal, policies []iamtypes.PolicyDetail) {
	for _, d := range policies {
		p.Inline = append(p.Inline, InlinePolicy{Principal: principal, Name: aws.ToString(d.PolicyName),
			Document: aws.ToString(d.PolicyDocument), DocumentMissing: d.PolicyDocument == nil})
	}
}

// managedPolicy takes the document of the default version from the
// versions of the answer; the older versions are not read.
func managedPolicy(m iamtypes.ManagedPolicyDetail) ManagedPolicy {
	p := ManagedPolicy{
		ARN:             aws.ToString(m.Arn),
		AttachmentCount: int(aws.ToInt32(m.AttachmentCount)),
		BoundaryCount:   int(aws.ToInt32(m.PermissionsBoundaryUsageCount)),
		DocumentMissing: true,
	}
	for _, v := range m.PolicyVersionList {
		if v.IsDefaultVersion && v.Document != nil {
			p.Document, p.DocumentMissing = *v.Document, false
			break
		}
	}
	return p
}
