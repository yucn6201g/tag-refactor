package awsaccess

import (
	"slices"
	"strings"
)

// The operations only apply (and later rollback) calls (U6 security design
// §1, §2). They are kept apart from Operations, the operations of scan and
// plan, so the read-only example policy keeps matching Operations alone.
const (
	OpTagResources   = "tag:TagResources"
	OpUntagResources = "tag:UntagResources"
	OpPutObject      = "s3:PutObject"
	OpGetObject      = "s3:GetObject"
	OpDeleteObject   = "s3:DeleteObject"
	// OpListBucket is not called: the permission makes S3 answer NotFound
	// rather than AccessDenied for a record that does not exist.
	OpListBucket = "s3:ListBucket"
)

// WriteOperations lists the operations apply calls besides Operations, and
// s3:ListBucket, which it needs without calling (U6 security design §2). The
// tagging API and the record operations keep their API names as IAM actions.
var WriteOperations = []Operation{
	{API: OpTagResources, IAMAction: OpTagResources, NeedsPermission: true},
	{API: OpUntagResources, IAMAction: OpUntagResources, NeedsPermission: true},
	{API: OpPutObject, IAMAction: OpPutObject, NeedsPermission: true},
	{API: OpGetObject, IAMAction: OpGetObject, NeedsPermission: true},
	{API: OpDeleteObject, IAMAction: OpDeleteObject, NeedsPermission: true},
	{API: OpListBucket, IAMAction: OpListBucket, NeedsPermission: true},
}

// TaggingType is one row of the table of the resource types apply can
// rewrite with the example policy (U6 security design §2, "書き換えの対象の
// 種類の表"). The tagging API checks, for each resource, the tagging actions
// of the resource's own service: TagActions for TagResources, UntagActions
// for UntagResources.
type TaggingType struct {
	// Name says which resources the row is for.
	Name string
	// Service is the service part of the resources' ARNs, for example "sqs".
	Service      string
	TagActions   []string
	UntagActions []string
	// Resources are the forms of the Resource element, with <account-id>
	// standing for the account as in the read-only example policy.
	Resources []string
}

// accountPlaceholder stands for the account ID in the Resources of the
// example policies.
const accountPlaceholder = "<account-id>"

// TaggingTypes is the table of the resource types (U6 security design §2).
// The action names follow the AWS managed policy
// ResourceGroupsTaggingAPITagUntagSupportedResources; the resource forms are
// the ARN forms of each service in the Service Authorization Reference.
// docs/iam/apply-policy.json is checked against it by a test.
var TaggingTypes = []TaggingType{
	{Name: "EC2 resources (instances, volumes, snapshots, AMIs, security groups and others)", Service: "ec2",
		TagActions: []string{"ec2:CreateTags"}, UntagActions: []string{"ec2:DeleteTags"},
		Resources: []string{"arn:aws:ec2:*:" + accountPlaceholder + ":*", "arn:aws:ec2:*::snapshot/*", "arn:aws:ec2:*::image/*"}},
	{Name: "S3 buckets", Service: "s3",
		TagActions:   []string{"s3:GetBucketTagging", "s3:PutBucketTagging", "s3:TagResource"},
		UntagActions: []string{"s3:GetBucketTagging", "s3:PutBucketTagging", "s3:UntagResource"},
		Resources:    []string{"arn:aws:s3:::*"}},
	{Name: "SQS queues", Service: "sqs", TagActions: []string{"sqs:TagQueue"}, UntagActions: []string{"sqs:UntagQueue"},
		Resources: []string{"arn:aws:sqs:*:" + accountPlaceholder + ":*"}},
	{Name: "SNS topics", Service: "sns", TagActions: []string{"sns:TagResource"}, UntagActions: []string{"sns:UntagResource"},
		Resources: []string{"arn:aws:sns:*:" + accountPlaceholder + ":*"}},
	{Name: "Secrets Manager secrets", Service: "secretsmanager",
		TagActions: []string{"secretsmanager:TagResource"}, UntagActions: []string{"secretsmanager:UntagResource"},
		Resources: []string{"arn:aws:secretsmanager:*:" + accountPlaceholder + ":secret:*"}},
	{Name: "IAM roles, users and managed policies", Service: "iam",
		TagActions:   []string{"iam:TagRole", "iam:TagUser", "iam:TagPolicy"},
		UntagActions: []string{"iam:UntagRole", "iam:UntagUser", "iam:UntagPolicy"},
		Resources: []string{"arn:aws:iam::" + accountPlaceholder + ":role/*", "arn:aws:iam::" + accountPlaceholder + ":user/*",
			"arn:aws:iam::" + accountPlaceholder + ":policy/*"}},
	{Name: "KMS keys", Service: "kms", TagActions: []string{"kms:TagResource"}, UntagActions: []string{"kms:UntagResource"},
		Resources: []string{"arn:aws:kms:*:" + accountPlaceholder + ":key/*"}},
	{Name: "Lambda functions", Service: "lambda", TagActions: []string{"lambda:TagResource"}, UntagActions: []string{"lambda:UntagResource"},
		Resources: []string{"arn:aws:lambda:*:" + accountPlaceholder + ":function:*"}},
	{Name: "DynamoDB tables", Service: "dynamodb", TagActions: []string{"dynamodb:TagResource"}, UntagActions: []string{"dynamodb:UntagResource"},
		Resources: []string{"arn:aws:dynamodb:*:" + accountPlaceholder + ":table/*"}},
	{Name: "CloudWatch Logs log groups", Service: "logs",
		TagActions:   []string{"logs:TagResource", "logs:TagLogGroup"},
		UntagActions: []string{"logs:UntagResource", "logs:UntagLogGroup"},
		Resources:    []string{"arn:aws:logs:*:" + accountPlaceholder + ":log-group:*"}},
	{Name: "RDS resources (DB instances, clusters, snapshots and others)", Service: "rds",
		TagActions: []string{"rds:AddTagsToResource"}, UntagActions: []string{"rds:RemoveTagsFromResource"},
		Resources: []string{"arn:aws:rds:*:" + accountPlaceholder + ":*"}},
}

// ApplyPermissions returns every IAM action apply needs, sorted and without
// repeats: the reads of scan and plan, the tagging API, the actions of the
// table of resource types, and the record in S3 (U6 NFR3.11).
func ApplyPermissions() []string {
	actions := RequiredPermissions()
	for _, op := range WriteOperations {
		actions = append(actions, op.IAMAction)
	}
	for _, t := range TaggingTypes {
		actions = append(actions, t.TagActions...)
		actions = append(actions, t.UntagActions...)
	}
	slices.Sort(actions)
	return slices.Compact(actions)
}

// TaggingTypeOf returns the row of the table for the service of an ARN.
func TaggingTypeOf(arn string) (TaggingType, bool) {
	parts := strings.SplitN(arn, ":", 4)
	if len(parts) < 3 {
		return TaggingType{}, false
	}
	for _, t := range TaggingTypes {
		if t.Service == parts[2] {
			return t, true
		}
	}
	return TaggingType{}, false
}

// tagActionFor names the action AWS checks for writing the tags of arn with
// the tagging API operation op, for an AccessDenied of that resource: the
// first tagging action of its service, or op when the table has no row for it
// (U6 security design §5).
func tagActionFor(op, arn string) string {
	t, ok := TaggingTypeOf(arn)
	if !ok {
		return op
	}
	actions := t.TagActions
	if op == OpUntagResources {
		actions = t.UntagActions
	}
	// For S3 the first actions are the bucket tagging reads and writes;
	// PutBucketTagging is the one that writes.
	for _, a := range actions {
		if !strings.Contains(a, ":Get") {
			return a
		}
	}
	return actions[0]
}

// IsAWSKey reports whether a tag key starts with aws:, in any case: AWS keeps
// those keys, and tag-refactor never writes or removes them.
func IsAWSKey(key string) bool {
	return strings.HasPrefix(strings.ToLower(key), "aws:")
}
