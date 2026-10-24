package inventory

import "strings"

// The resource type tables of BR2.5 and BR3.3. AWS has no API that lists
// them, so the tool keeps them and its maintainers review them against the
// AWS documentation before each release (tech-stack-decisions.md).

// configToARNForm maps the AWS Config form of a resource type to the ARN form
// used everywhere else: "service:type" as in the resource part of the ARN, or
// the service alone when its ARNs have no type part, as ResourceTypeOf
// returns. Types not listed keep the AWS Config form (BR2.5).
var configToARNForm = map[string]string{
	"AWS::ACM::Certificate":                     "acm:certificate",
	"AWS::CloudFormation::Stack":                "cloudformation:stack",
	"AWS::CloudTrail::Trail":                    "cloudtrail:trail",
	"AWS::CloudWatch::Alarm":                    "cloudwatch:alarm",
	"AWS::CodeBuild::Project":                   "codebuild:project",
	"AWS::Config::ConformancePackCompliance":    "config:conformance-pack-compliance",
	"AWS::Config::ResourceCompliance":           "config:resource-compliance",
	"AWS::DynamoDB::Table":                      "dynamodb:table",
	"AWS::EC2::CustomerGateway":                 "ec2:customer-gateway",
	"AWS::EC2::DHCPOptions":                     "ec2:dhcp-options",
	"AWS::EC2::EIP":                             "ec2:elastic-ip",
	"AWS::EC2::Instance":                        "ec2:instance",
	"AWS::EC2::InternetGateway":                 "ec2:internet-gateway",
	"AWS::EC2::LaunchTemplate":                  "ec2:launch-template",
	"AWS::EC2::NatGateway":                      "ec2:natgateway",
	"AWS::EC2::NetworkAcl":                      "ec2:network-acl",
	"AWS::EC2::NetworkInterface":                "ec2:network-interface",
	"AWS::EC2::RouteTable":                      "ec2:route-table",
	"AWS::EC2::SecurityGroup":                   "ec2:security-group",
	"AWS::EC2::Subnet":                          "ec2:subnet",
	"AWS::EC2::Volume":                          "ec2:volume",
	"AWS::EC2::VPC":                             "ec2:vpc",
	"AWS::EC2::VPCEndpoint":                     "ec2:vpc-endpoint",
	"AWS::ECR::Repository":                      "ecr:repository",
	"AWS::ECS::Cluster":                         "ecs:cluster",
	"AWS::EKS::Cluster":                         "eks:cluster",
	"AWS::ElasticLoadBalancingV2::LoadBalancer": "elasticloadbalancing:loadbalancer",
	"AWS::Events::Rule":                         "events:rule",
	"AWS::IAM::Group":                           "iam:group",
	"AWS::IAM::Policy":                          "iam:policy",
	"AWS::IAM::Role":                            "iam:role",
	"AWS::IAM::User":                            "iam:user",
	"AWS::KMS::Key":                             "kms:key",
	"AWS::Lambda::Function":                     "lambda:function",
	"AWS::Logs::LogGroup":                       "logs:log-group",
	"AWS::RDS::DBCluster":                       "rds:cluster",
	"AWS::RDS::DBInstance":                      "rds:db",
	"AWS::RDS::DBSnapshot":                      "rds:snapshot",
	"AWS::S3::Bucket":                           "s3",
	"AWS::SNS::Topic":                           "sns",
	"AWS::SQS::Queue":                           "sqs",
	"AWS::SSM::AssociationCompliance":           "ssm:association-compliance",
	"AWS::SSM::ManagedInstanceInventory":        "ssm:managed-instance-inventory",
	"AWS::SSM::PatchCompliance":                 "ssm:patch-compliance",
	"AWS::SecretsManager::Secret":               "secretsmanager:secret",
	"AWS::StepFunctions::StateMachine":          "states:stateMachine",
}

// notTaggableTypes are the resource types (ARN form) known to have no tags
// that the tagging API can change (BR3.3): the compliance and inventory
// records of AWS Config and Systems Manager are not resources that carry
// tags. IAM types are not listed: the discovery leaves them out (isIAMType),
// so they never reach this check. Only types known to be untaggable are
// listed: a type missing from the list is treated as taggable, so a stale
// list can count a few required tags too many, never too few
// (tech-stack-decisions.md).
var notTaggableTypes = map[string]bool{
	"config:conformance-pack-compliance": true,
	"config:resource-compliance":         true,
	"ssm:association-compliance":         true,
	"ssm:managed-instance-inventory":     true,
	"ssm:patch-compliance":               true,
}

// ConfigTypeToARNForm returns the ARN form of an AWS Config resource type,
// or the type unchanged when the table does not know it (BR2.5).
func ConfigTypeToARNForm(configType string) string {
	if t, ok := configToARNForm[configType]; ok {
		return t
	}
	return configType
}

// ARNFormOf returns the ARN form of an AWS Config (CloudFormation) resource
// type and true, or "" and false when the table does not know the type. The
// rule file reader uses it to suggest the form to write in a condition (U3
// BR3.4); it is passed as rules.Options.TypeHint.
func ARNFormOf(configType string) (string, bool) {
	t, ok := configToARNForm[configType]
	return t, ok
}

// IsNotTaggable reports whether resources of the type (ARN form) cannot have
// their tags changed through the tagging API (BR3.3).
func IsNotTaggable(resourceType string) bool {
	return notTaggableTypes[resourceType]
}

// isIAMType reports whether the type is one of IAM's global types, which the
// discovery leaves out: AWS Config does not record them by default and
// Resource Explorer does not index their tags, so an IAM role with tags
// would look untagged (reliability design §3). That is why notTaggableTypes
// lists no IAM type (IAM groups, which have no tags, included).
func isIAMType(resourceType string) bool {
	return strings.HasPrefix(resourceType, "iam:") || strings.HasPrefix(resourceType, "AWS::IAM::")
}
