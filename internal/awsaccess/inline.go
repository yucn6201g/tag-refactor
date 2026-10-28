package awsaccess

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Reading IAM users, roles and groups and their inline policies (U5 BR1.2,
// security design §1). Every call goes through invoke, so the time limits,
// the retries and the --debug lines apply.

// iamPageSize is the MaxItems of the IAM list APIs, their maximum (U5
// logical-components.md).
const iamPageSize = policyPageSize

// iamPage is one page of an IAM list: the items read and the marker of the
// next page.
type iamPage struct {
	items     []IAMPrincipal
	names     []string
	truncated bool
	marker    string
}

// readIAMPages reads every page of an IAM list. read asks for the page after
// marker ("" for the first one).
func (s *Session) readIAMPages(ctx context.Context, op string,
	read func(ctx context.Context, marker string, add apiOption) (iamPage, error)) (iamPage, error) {
	var all iamPage
	marker := ""
	for {
		var page iamPage
		err := s.invoke(ctx, op, s.region, func(ctx context.Context, add apiOption) error {
			var err error
			page, err = read(ctx, marker, add)
			return err
		})
		if err != nil {
			return iamPage{}, err
		}
		all.items = append(all.items, page.items...)
		all.names = append(all.names, page.names...)
		if !page.truncated {
			return all, nil
		}
		if page.marker == "" || page.marker == marker {
			return iamPage{}, &errkind.Error{Kind: errkind.Internal, Op: op, Region: s.region,
				Detail: "IAM returned a truncated list without a new marker"}
		}
		marker = page.marker
	}
}

// marker returns the Marker parameter of a request: nil on the first page.
func markerParam(marker string) *string {
	if marker == "" {
		return nil
	}
	return aws.String(marker)
}

// ListPrincipals reads every IAM user, role or group (ListUsers, ListRoles,
// ListGroups).
func (s *Session) ListPrincipals(ctx context.Context, kind PrincipalKind) ([]IAMPrincipal, error) {
	op, read, err := s.principalReader(kind)
	if err != nil {
		return nil, err
	}
	all, err := s.readIAMPages(ctx, op, read)
	return all.items, err
}

func (s *Session) principalReader(kind PrincipalKind) (string, func(context.Context, string, apiOption) (iamPage, error), error) {
	opts := func(add apiOption) func(*iam.Options) {
		return func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) }
	}
	switch kind {
	case PrincipalUser:
		return OpListUsers, func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListUsers(ctx, &iam.ListUsersInput{MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			page := iamPage{truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}
			for _, u := range out.Users {
				page.items = append(page.items, IAMPrincipal{Kind: kind, Name: aws.ToString(u.UserName), ARN: aws.ToString(u.Arn)})
			}
			return page, nil
		}, nil
	case PrincipalRole:
		return OpListRoles, func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListRoles(ctx, &iam.ListRolesInput{MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			page := iamPage{truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}
			for _, r := range out.Roles {
				page.items = append(page.items, IAMPrincipal{Kind: kind, Name: aws.ToString(r.RoleName), ARN: aws.ToString(r.Arn)})
			}
			return page, nil
		}, nil
	case PrincipalGroup:
		return OpListGroups, func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListGroups(ctx, &iam.ListGroupsInput{MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			page := iamPage{truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}
			for _, g := range out.Groups {
				page.items = append(page.items, IAMPrincipal{Kind: kind, Name: aws.ToString(g.GroupName), ARN: aws.ToString(g.Arn)})
			}
			return page, nil
		}, nil
	}
	return "", nil, unknownKind(kind)
}

func unknownKind(kind PrincipalKind) error {
	return &errkind.Error{Kind: errkind.Internal, Detail: fmt.Sprintf("unknown IAM principal kind %q", kind)}
}

// ListInlinePolicyNames reads the names of the inline policies of a user,
// role or group (ListUserPolicies, ListRolePolicies, ListGroupPolicies).
func (s *Session) ListInlinePolicyNames(ctx context.Context, p IAMPrincipal) ([]string, error) {
	opts := func(add apiOption) func(*iam.Options) {
		return func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) }
	}
	name := aws.String(p.Name)
	var op string
	var read func(context.Context, string, apiOption) (iamPage, error)
	switch p.Kind {
	case PrincipalUser:
		op = OpListUserPolicies
		read = func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{UserName: name, MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			return iamPage{names: out.PolicyNames, truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}, nil
		}
	case PrincipalRole:
		op = OpListRolePolicies
		read = func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: name, MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			return iamPage{names: out.PolicyNames, truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}, nil
		}
	case PrincipalGroup:
		op = OpListGroupPolicies
		read = func(ctx context.Context, marker string, add apiOption) (iamPage, error) {
			out, err := s.iam.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: name, MaxItems: aws.Int32(iamPageSize), Marker: markerParam(marker)}, opts(add))
			if err != nil {
				return iamPage{}, err
			}
			return iamPage{names: out.PolicyNames, truncated: out.IsTruncated, marker: aws.ToString(out.Marker)}, nil
		}
	default:
		return nil, unknownKind(p.Kind)
	}
	all, err := s.readIAMPages(ctx, op, read)
	return all.names, err
}

// GetInlinePolicyDocument reads one inline policy (GetUserPolicy,
// GetRolePolicy, GetGroupPolicy). The document is returned as IAM sends it:
// URL-encoded JSON.
func (s *Session) GetInlinePolicyDocument(ctx context.Context, p IAMPrincipal, policyName string) (string, error) {
	opts := func(add apiOption) func(*iam.Options) {
		return func(o *iam.Options) { o.APIOptions = append(o.APIOptions, add) }
	}
	principal, name := aws.String(p.Name), aws.String(policyName)
	var doc *string
	var op string
	var call func(ctx context.Context, add apiOption) error
	switch p.Kind {
	case PrincipalUser:
		op = OpGetUserPolicy
		call = func(ctx context.Context, add apiOption) error {
			out, err := s.iam.GetUserPolicy(ctx, &iam.GetUserPolicyInput{UserName: principal, PolicyName: name}, opts(add))
			if err == nil {
				doc = out.PolicyDocument
			}
			return err
		}
	case PrincipalRole:
		op = OpGetRolePolicy
		call = func(ctx context.Context, add apiOption) error {
			out, err := s.iam.GetRolePolicy(ctx, &iam.GetRolePolicyInput{RoleName: principal, PolicyName: name}, opts(add))
			if err == nil {
				doc = out.PolicyDocument
			}
			return err
		}
	case PrincipalGroup:
		op = OpGetGroupPolicy
		call = func(ctx context.Context, add apiOption) error {
			out, err := s.iam.GetGroupPolicy(ctx, &iam.GetGroupPolicyInput{GroupName: principal, PolicyName: name}, opts(add))
			if err == nil {
				doc = out.PolicyDocument
			}
			return err
		}
	default:
		return "", unknownKind(p.Kind)
	}
	if err := s.invoke(ctx, op, s.region, call); err != nil {
		return "", err
	}
	if doc == nil {
		return "", &errkind.Error{Kind: errkind.Internal, Op: op, Region: s.region, Detail: "IAM returned no policy document"}
	}
	return *doc, nil
}
