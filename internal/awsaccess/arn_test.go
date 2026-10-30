package awsaccess

import (
	"slices"
	"testing"
)

// Unit tests of the ARN parts the checks of rollback use (U7 security design
// §2) and of the permissions rollback needs (NFR3.13).

func TestParseARNSplitsSixParts(t *testing.T) {
	for _, c := range []struct {
		arn  string
		want ARN
	}{
		{"arn:aws:sqs:us-east-1:123456789012:qa", ARN{"aws", "sqs", "us-east-1", "123456789012", "qa"}},
		{"arn:aws:s3:::bucket-b1", ARN{"aws", "s3", "", "", "bucket-b1"}},
		{"arn:aws:iam::123456789012:role/path/r", ARN{"aws", "iam", "", "123456789012", "role/path/r"}},
		{"arn:aws:ec2:us-east-1::snapshot/snap-1", ARN{"aws", "ec2", "us-east-1", "", "snapshot/snap-1"}},
		{"arn:aws:logs:us-east-1:123456789012:log-group:/a/b:*", ARN{"aws", "logs", "us-east-1", "123456789012", "log-group:/a/b:*"}},
		{"arn:aws-cn:sns:cn-north-1:123456789012:t", ARN{"aws-cn", "sns", "cn-north-1", "123456789012", "t"}},
	} {
		got, ok := ParseARN(c.arn)
		if !ok || got != c.want {
			t.Errorf("%s: %+v %v", c.arn, got, ok)
		}
	}
}

func TestParseARNRefusesWhatIsNotAnARN(t *testing.T) {
	for _, s := range []string{"", "arn", "arn:aws:sqs:us-east-1:123456789012", "ARN:aws:sqs:us-east-1:1:q", "urn:aws:sqs:us-east-1:1:q", "sqs:q"} {
		if got, ok := ParseARN(s); ok {
			t.Errorf("%q parsed as %+v", s, got)
		}
	}
}

// An empty part is kept empty: the checks of rollback refuse an empty
// partition, service or resource themselves.
func TestParseARNKeepsEmptyParts(t *testing.T) {
	got, ok := ParseARN("arn::::::")
	if !ok || got != (ARN{Resource: ":"}) {
		t.Fatalf("%+v %v", got, ok)
	}
}

// NFR3.13: every operation rollback calls is allowed by the example policy
// of apply, which rollback uses as it is (sts:GetCallerIdentity needs no
// permission).
func TestTheOperationsOfRollbackAreInTheApplyPolicy(t *testing.T) {
	allowed := ApplyPermissions()
	for _, op := range []string{OpGetResources, OpTagResources, OpUntagResources, OpGetObject, OpPutObject,
		OpDeleteObject, OpListBucket} {
		if !slices.Contains(allowed, IAMAction(op)) {
			t.Errorf("%s is not in docs/iam/apply-policy.json", op)
		}
	}
}

// Every resource type apply can write has both a tag and an untag action,
// since rollback sets back values and removes keys apply added.
func TestEveryTypeCanBeRestoredBothWays(t *testing.T) {
	for _, row := range TaggingTypes {
		if len(row.TagActions) == 0 || len(row.UntagActions) == 0 {
			t.Errorf("%s cannot be restored both ways", row.Name)
		}
		if a := tagActionFor(OpUntagResources, "arn:aws:"+row.Service+":"); !slices.Contains(row.UntagActions, a) {
			t.Errorf("%s: the untag action %s", row.Name, a)
		}
	}
}
