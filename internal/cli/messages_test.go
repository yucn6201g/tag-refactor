package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

func TestEveryErrorKindHasAWhatAndAFix(t *testing.T) {
	secret := errors.New("AKIAFAKEFAKEFAKE0000 secret text from the SDK")
	cases := []struct {
		err     *errkind.Error
		what    string
		fixPart string
	}{
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: "bad flag"}, "bad flag", "--help"},
		{&errkind.Error{Kind: errkind.InvalidInput, Detail: "plain detail", Hint: "do this"}, "plain detail", "do this"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOptionNotAvailable, Subject: "--out"}, "--out is not available in this version", "scan"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonProfileNotFound, Subject: "dev"}, `the AWS profile "dev" was not found`, "aws configure --profile"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionNotSet}, "no AWS region is set", "AWS_REGION"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionInvalid, Region: "x!"}, `"x!" is not an AWS region name`, "--region"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionUnavailable, Region: "me-south-1"}, "the region me-south-1 is not available", "enabled region"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRuleFile, Subject: "r.yaml", Line: 3, Detail: "unknown key", Hint: "fix it"}, "r.yaml line 3: unknown key", "fix it"},
		{&errkind.Error{Kind: errkind.UnsupportedVersion, Subject: "r.yaml", Line: 1, Detail: "version 2"}, "r.yaml line 1: version 2", "check the rule file"},
		{&errkind.Error{Kind: errkind.NotFound, Subject: "r.yaml", Detail: "missing", Hint: "check --rules"}, "r.yaml: missing", "check --rules"},
		{&errkind.Error{Kind: errkind.StorageFailed, Subject: "r.yaml", Detail: "unreadable", Cause: secret}, "r.yaml: unreadable", "check the rule file"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Cause: secret}, "no AWS credentials were found", "aws configure"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonSSOSessionExpired, Subject: "default"}, "SSO session has expired", `"aws sso login" and`},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsExpired, Op: "sts:GetCallerIdentity", Subject: "dev"}, "credentials have expired", "aws sso login --profile dev"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsRejected, Op: "sts:GetCallerIdentity"}, "AWS rejected the credentials", "access key"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsTimeout}, "could not be obtained in time", "network"},
		{&errkind.Error{Kind: errkind.AccessDenied, Op: "iam:ListPolicies", Cause: secret}, "access denied for iam:ListPolicies", "docs/iam/readonly-policy.json"},
		{&errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout, Op: "tag:GetResources", Region: "us-east-1"}, "did not answer tag:GetResources in time (region us-east-1)", "--debug"},
		{&errkind.Error{Kind: errkind.Transient, Op: "tag:GetResources"}, "kept failing on tag:GetResources after retries", "--debug"},
		{&errkind.Error{Kind: errkind.Internal, Cause: secret}, "an unexpected error occurred in tag-refactor", "--debug"},
		{&errkind.Error{Kind: errkind.Internal, Op: "tag:GetResources", Detail: "repeated token"}, "in tag:GetResources: repeated token", "--debug"},
		{&errkind.Error{Kind: errkind.Conflict, Op: "x"}, "an unexpected error occurred in x", "--debug"},
	}
	for _, tc := range cases {
		what, fix := describe(tc.err)
		if !strings.Contains(what, tc.what) || !strings.Contains(fix, tc.fixPart) {
			t.Errorf("describe(%s/%s) = %q / %q, want %q / %q", tc.err.Kind, tc.err.Reason, what, fix, tc.what, tc.fixPart)
		}
		if strings.Contains(what+fix, "AKIA") || strings.Contains(what+fix, "secret text") {
			t.Errorf("describe leaks the cause: %q / %q", what, fix)
		}
	}
}

// BR6.3: an error or an incomplete result is 1, anything to review is 2,
// otherwise 0; incomplete wins over review.
func TestExitCodeFollowsBR6_3(t *testing.T) {
	cases := []struct {
		o    outcome
		code int
		mean string
	}{
		{outcome{command: "plan", incomplete: true, needsReview: true}, 1, "impact analysis incomplete"},
		{outcome{command: "scan", needsReview: true}, 2, "issues found"},
		{outcome{command: "plan", needsReview: true}, 2, "review needed"},
		{outcome{command: "scan"}, 0, "no issues"},
		{outcome{command: "plan"}, 0, "no changes needed"},
	}
	for _, tc := range cases {
		code, meaning := tc.o.exitCode()
		if code != tc.code || meaning != tc.mean {
			t.Errorf("%+v -> %d %q, want %d %q", tc.o, code, meaning, tc.code, tc.mean)
		}
	}
	if ensureKind(nil) != nil || errkind.KindOf(ensureKind(errors.New("x"))) != errkind.Internal {
		t.Fatal("ensureKind must keep nil and classify plain errors as Internal")
	}
}
