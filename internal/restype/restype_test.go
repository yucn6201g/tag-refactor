package restype

import (
	"slices"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Unit tests of the table of resource types (U8 NFR10.14), written after the
// implementation.

func TestTableHasEveryColumnOnEveryRow(t *testing.T) {
	if len(Rows) != 18 {
		t.Fatalf("%d rows, want 18", len(Rows))
	}
	seen := map[string]bool{}
	for _, r := range Rows {
		if r.ResourceType == "" || r.Service == "" || r.TerraformType == "" || len(r.CloudFormationTypes) == 0 {
			t.Errorf("a column is empty: %+v", r)
		}
		if seen[r.ResourceType] {
			t.Errorf("%s is listed twice", r.ResourceType)
		}
		seen[r.ResourceType] = true
		if service, _, _ := strings.Cut(r.ResourceType, ":"); service != r.Service {
			t.Errorf("%s has the service %s", r.ResourceType, r.Service)
		}
		for _, cf := range r.CloudFormationTypes {
			if !strings.HasPrefix(cf, "AWS::") {
				t.Errorf("%s is not a CloudFormation type", cf)
			}
		}
		if !strings.HasPrefix(r.TerraformType, "aws_") {
			t.Errorf("%s is not a Terraform type of AWS", r.TerraformType)
		}
	}
}

func TestEveryServiceIsOneApplyWrites(t *testing.T) {
	services := make([]string, 0, len(awsaccess.TaggingTypes))
	for _, tt := range awsaccess.TaggingTypes {
		services = append(services, tt.Service)
	}
	for _, r := range Rows {
		if !slices.Contains(services, r.Service) {
			t.Errorf("%s: the table of apply has no service %s", r.ResourceType, r.Service)
		}
	}
}

// The form of a resourceType is what scan writes in a plan: from the ARN, and
// from the AWS Config type for the resources it discovers.
func TestResourceTypesAreWrittenAsScanWritesThem(t *testing.T) {
	examples := map[string]string{
		"s3":                    "arn:aws:s3:::bucket",
		"sqs":                   "arn:aws:sqs:us-east-1:111122223333:queue",
		"sns":                   "arn:aws:sns:us-east-1:111122223333:topic",
		"secretsmanager:secret": "arn:aws:secretsmanager:us-east-1:111122223333:secret:name-AbCdEf",
		"kms:key":               "arn:aws:kms:us-east-1:111122223333:key/1234",
		"lambda:function":       "arn:aws:lambda:us-east-1:111122223333:function:name",
		"dynamodb:table":        "arn:aws:dynamodb:us-east-1:111122223333:table/name",
		"logs:log-group":        "arn:aws:logs:us-east-1:111122223333:log-group:name",
		"ec2:security-group":    "arn:aws:ec2:us-east-1:111122223333:security-group/sg-1",
		"ec2:vpc":               "arn:aws:ec2:us-east-1:111122223333:vpc/vpc-1",
		"ec2:subnet":            "arn:aws:ec2:us-east-1:111122223333:subnet/subnet-1",
		"ec2:instance":          "arn:aws:ec2:us-east-1:111122223333:instance/i-1",
		"ec2:volume":            "arn:aws:ec2:us-east-1:111122223333:volume/vol-1",
		"rds:db":                "arn:aws:rds:us-east-1:111122223333:db:name",
		"rds:cluster":           "arn:aws:rds:us-east-1:111122223333:cluster:name",
		"iam:role":              "arn:aws:iam::111122223333:role/name",
		"iam:user":              "arn:aws:iam::111122223333:user/name",
		"iam:policy":            "arn:aws:iam::111122223333:policy/name",
	}
	for _, r := range Rows {
		arn, ok := examples[r.ResourceType]
		if !ok {
			t.Errorf("no example for %s", r.ResourceType)
			continue
		}
		if got := inventory.ResourceTypeOf(arn); got != r.ResourceType {
			t.Errorf("scan writes %s for %s, the table %s", got, arn, r.ResourceType)
		}
		for _, cf := range r.CloudFormationTypes {
			if form, ok := inventory.ARNFormOf(cf); ok && form != r.ResourceType {
				t.Errorf("AWS Config gives %s for %s, the table %s", form, cf, r.ResourceType)
			}
		}
	}
}

func TestResourceTypeOf(t *testing.T) {
	for cf, want := range map[string]string{"AWS::S3::Bucket": "s3", "AWS::SQS::Queue": "sqs", "AWS::SNS::Topic": "sns",
		"AWS::Serverless::Function": "lambda:function", "AWS::Lambda::Function": "lambda:function", "AWS::EC2::VPC": "ec2:vpc",
		"AWS::IAM::ManagedPolicy": "iam:policy", "AWS::RDS::DBInstance": "rds:db"} {
		if got, ok := ResourceTypeOf(cf); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", cf, got, ok, want)
		}
	}
	for _, cf := range []string{"", "AWS::S3::BucketPolicy", "aws::s3::bucket", "AWS::IAM::Policy", "s3"} {
		if got, ok := ResourceTypeOf(cf); ok {
			t.Errorf("%q is not in the table: %q", cf, got)
		}
	}
}

// The package is what keeps iac away from the SDK: it takes nothing in but its
// own table (U8 NFR3.16). The test may import more than the package does.
func TestEveryCloudFormationTypeHasOneResourceType(t *testing.T) {
	seen := map[string]string{}
	for _, r := range Rows {
		for _, cf := range r.CloudFormationTypes {
			if other, dup := seen[cf]; dup {
				t.Errorf("%s belongs to %s and %s", cf, other, r.ResourceType)
			}
			seen[cf] = r.ResourceType
		}
	}
}
