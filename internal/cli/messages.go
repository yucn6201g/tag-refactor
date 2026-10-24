package cli

import (
	"fmt"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

const (
	helpHint        = `run "tag-refactor --help" to see the commands and options`
	permissionsHint = "docs/iam/readonly-policy.json lists the permissions tag-refactor needs"
	supportedHint   = "this version has scan (--rules, --profile, --region), plan (--rules, --profile, --region) and " +
		"rules suggest (--profile, --region, --output), plus --debug"
)

// describe turns an error into the two lines shown to the user: what
// happened and how to fix it (W3, NFR11.4). Only tool-authored fields of the
// error are used; the text of SDK errors is never shown (BR1.4).
func describe(e *errkind.Error) (what, fix string) {
	switch e.Kind {
	case errkind.InvalidInput:
		return describeInvalidInput(e)
	case errkind.UnsupportedVersion, errkind.NotFound, errkind.StorageFailed:
		return describeFileError(e)
	case errkind.CredentialsMissing:
		return describeCredentials(e)
	case errkind.AccessDenied:
		// The IAM action is what the user has to allow; it differs from the
		// API name for Resource Explorer's ListResources (security design §3).
		action := awsaccess.IAMAction(e.Op)
		return fmt.Sprintf("access denied for %s%s", action, inRegion(e.Region)),
			fmt.Sprintf("allow %s for the identity you use; %s", action, permissionsHint)
	case errkind.Transient:
		if e.Reason == errkind.ReasonTimeout {
			return fmt.Sprintf("AWS did not answer %s in time%s", e.Op, inRegion(e.Region)),
				"try again later; add --debug to see the time of each call"
		}
		return fmt.Sprintf("AWS kept failing on %s after retries%s", e.Op, inRegion(e.Region)),
			"try again later; add --debug to see each call and its retries"
	default:
		return describeInternal(e)
	}
}

func inRegion(region string) string {
	if region == "" {
		return ""
	}
	return " (region " + region + ")"
}

func describeInvalidInput(e *errkind.Error) (string, string) {
	switch e.Reason {
	case errkind.ReasonRuleFile:
		return describeFileError(e)
	case errkind.ReasonOptionNotAvailable:
		return fmt.Sprintf("%s is not available in this version", e.Subject), supportedHint
	case errkind.ReasonProfileNotFound:
		return fmt.Sprintf("the AWS profile %q was not found", e.Subject),
			`check the profile name in your AWS config file, or create it with "aws configure --profile <name>"`
	case errkind.ReasonRegionNotSet:
		return "no AWS region is set", "pass --region (for example --region us-east-1) or set AWS_REGION"
	case errkind.ReasonRegionInvalid:
		return fmt.Sprintf("%q is not an AWS region name", e.Region), "pass a region name such as us-east-1 with --region"
	case errkind.ReasonRegionUnavailable:
		return fmt.Sprintf("the region %s is not available for this account", e.Region),
			"pass an enabled region with --region or set AWS_REGION"
	}
	fix := e.Hint
	if fix == "" {
		fix = helpHint
	}
	return e.Detail, fix
}

// describeFileError formats rule file errors as "<file> line <n>: <detail>".
func describeFileError(e *errkind.Error) (string, string) {
	what := e.Subject
	if e.Line > 0 {
		what += fmt.Sprintf(" line %d", e.Line)
	}
	what += ": " + e.Detail
	fix := e.Hint
	if fix == "" {
		fix = "check the rule file"
	}
	return what, fix
}

func describeCredentials(e *errkind.Error) (string, string) {
	login := `"aws sso login"`
	if e.Subject != "" && e.Subject != "default" {
		login = fmt.Sprintf(`"aws sso login --profile %s"`, e.Subject)
	}
	switch e.Reason {
	case errkind.ReasonSSOSessionExpired:
		return "the AWS SSO session has expired or is not signed in", "run " + login + " and try again"
	case errkind.ReasonCredentialsExpired:
		return fmt.Sprintf("the AWS credentials have expired (%s)", e.Op),
			"sign in again, for example with " + login + ", or refresh the temporary credentials"
	case errkind.ReasonCredentialsRejected:
		return fmt.Sprintf("AWS rejected the credentials (%s)", e.Op),
			"check the access key of the profile or the environment, or sign in again"
	case errkind.ReasonCredentialsTimeout:
		return "AWS credentials could not be obtained in time",
			"check the profile settings and the network, or pass --profile"
	default:
		return "no AWS credentials were found",
			`run "aws configure" or "aws sso login", set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or pass --profile`
	}
}

// describeInternal names the operation only; the error text and its type are
// kept out of the message (security design §5).
func describeInternal(e *errkind.Error) (string, string) {
	where := e.Op
	if where == "" {
		where = "tag-refactor"
	}
	what := "an unexpected error occurred in " + where + inRegion(e.Region)
	if e.Detail != "" {
		what += ": " + e.Detail
	}
	return what, "run the command again with --debug to see the failing call"
}
