// Package restype holds the table of the resource types iac reflects: for each
// resourceType of a plan, its service, its CloudFormation types and its
// Terraform type (U8 logical-components.md, NFR10.14). It depends on nothing,
// so that iac does not take in the AWS SDK for it.
package restype

// Row is one resource type.
type Row struct {
	// ResourceType is written as scan writes it: the service and type part of
	// the ARN, or the service alone when its ARNs have no type part (S3
	// buckets, SQS queues and SNS topics), as inventory.ResourceTypeOf returns.
	ResourceType string
	// Service is the service part of the ARN, for example "sqs".
	Service string
	// CloudFormationTypes are the Type values of the templates; at least one.
	CloudFormationTypes []string
	// TerraformType is the type of the resource block.
	TerraformType string
}

// Rows is the table: the services apply writes, for the types often made with
// IaC. The IAM types are not collected by scan today, so they do not reach a
// plan yet; they wait here for the work that collects them.
var Rows = []Row{
	{"s3", "s3", []string{"AWS::S3::Bucket"}, "aws_s3_bucket"},
	{"sqs", "sqs", []string{"AWS::SQS::Queue"}, "aws_sqs_queue"},
	{"sns", "sns", []string{"AWS::SNS::Topic"}, "aws_sns_topic"},
	{"secretsmanager:secret", "secretsmanager", []string{"AWS::SecretsManager::Secret"}, "aws_secretsmanager_secret"},
	{"kms:key", "kms", []string{"AWS::KMS::Key"}, "aws_kms_key"},
	{"lambda:function", "lambda", []string{"AWS::Lambda::Function", "AWS::Serverless::Function"}, "aws_lambda_function"},
	{"dynamodb:table", "dynamodb", []string{"AWS::DynamoDB::Table"}, "aws_dynamodb_table"},
	{"logs:log-group", "logs", []string{"AWS::Logs::LogGroup"}, "aws_cloudwatch_log_group"},
	{"ec2:security-group", "ec2", []string{"AWS::EC2::SecurityGroup"}, "aws_security_group"},
	{"ec2:vpc", "ec2", []string{"AWS::EC2::VPC"}, "aws_vpc"},
	{"ec2:subnet", "ec2", []string{"AWS::EC2::Subnet"}, "aws_subnet"},
	{"ec2:instance", "ec2", []string{"AWS::EC2::Instance"}, "aws_instance"},
	{"ec2:volume", "ec2", []string{"AWS::EC2::Volume"}, "aws_ebs_volume"},
	{"rds:db", "rds", []string{"AWS::RDS::DBInstance"}, "aws_db_instance"},
	{"rds:cluster", "rds", []string{"AWS::RDS::DBCluster"}, "aws_rds_cluster"},
	{"iam:role", "iam", []string{"AWS::IAM::Role"}, "aws_iam_role"},
	{"iam:user", "iam", []string{"AWS::IAM::User"}, "aws_iam_user"},
	{"iam:policy", "iam", []string{"AWS::IAM::ManagedPolicy"}, "aws_iam_policy"},
}

// byCloudFormationType finds the resourceType of a CloudFormation Type.
var byCloudFormationType = func() map[string]string {
	m := make(map[string]string, len(Rows))
	for _, r := range Rows {
		for _, t := range r.CloudFormationTypes {
			m[t] = r.ResourceType
		}
	}
	return m
}()

// ResourceTypeOf returns the resourceType of a CloudFormation Type, and false
// for a Type the table does not have (U8 BR3.2).
func ResourceTypeOf(cloudFormationType string) (string, bool) {
	t, ok := byCloudFormationType[cloudFormationType]
	return t, ok
}
