package cli

import (
	"fmt"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// describe turns an error into the two lines shown to the user, in lang:
// what happened and how to fix it (W3, NFR11.4). Only tool-authored fields of
// the error are used; the text of SDK errors is never shown (BR1.4). The
// Detail and Hint that other packages write (rule file and plan file
// checks) are in English and are shown as they are inside the translated
// lines.
func describe(e *errkind.Error, lang display.Lang) (what, fix string) {
	t := func(id string, args ...any) string { return display.T(lang, id, args...) }
	switch e.Kind {
	case errkind.InvalidInput:
		return describeInvalidInput(e, lang)
	case errkind.UnsupportedVersion, errkind.NotFound:
		return describeFileError(e, lang)
	case errkind.StorageFailed:
		if e.Reason == errkind.ReasonOutputFailed {
			return describeOutput(e, lang)
		}
		return describeFileError(e, lang)
	case errkind.CredentialsMissing:
		return describeCredentials(e, lang)
	case errkind.AccessDenied:
		// The IAM action is what the user has to allow; it differs from the
		// API name for Resource Explorer's ListResources (security design §3).
		action := awsaccess.IAMAction(e.Op)
		return t("err.accessDenied", action, inRegion(e.Region, lang)), t("err.accessDeniedFix", action, t("err.permissions"))
	case errkind.Transient:
		if e.Reason == errkind.ReasonTimeout {
			return t("err.timeout", e.Op, inRegion(e.Region, lang)), t("err.timeoutFix")
		}
		return t("err.transient", e.Op, inRegion(e.Region, lang)), t("err.transientFix")
	default:
		return describeInternal(e, lang)
	}
}

func inRegion(region string, lang display.Lang) string {
	if region == "" {
		return ""
	}
	return display.T(lang, "err.inRegion", region)
}

func describeInvalidInput(e *errkind.Error, lang display.Lang) (string, string) {
	t := func(id string, args ...any) string { return display.T(lang, id, args...) }
	switch e.Reason {
	case errkind.ReasonRuleFile, errkind.ReasonPlanFile:
		return describeFileError(e, lang)
	case errkind.ReasonOutputExists, errkind.ReasonOutputFolderMissing, errkind.ReasonOutputTooLarge:
		return describeOutput(e, lang)
	case errkind.ReasonOptionNotAvailable:
		return t("err.notAvailable", e.Subject), t("err.supported")
	case errkind.ReasonProfileNotFound:
		return t("err.profileNotFound", e.Subject), t("err.profileFix")
	case errkind.ReasonRegionNotSet:
		return t("err.regionNotSet"), t("err.regionNotSetFix")
	case errkind.ReasonRegionInvalid:
		return t("err.regionInvalid", e.Region), t("err.regionInvalidFix")
	case errkind.ReasonRegionUnavailable:
		return t("err.regionNA", e.Region), t("err.regionNAFix")
	}
	fix := e.Hint
	if fix == "" {
		fix = t("err.help")
	}
	return e.Detail, fix
}

// describeOutput formats the errors of the files tag-refactor writes; Key is
// the option (--out or --output).
func describeOutput(e *errkind.Error, lang display.Lang) (string, string) {
	t := func(id string, args ...any) string { return display.T(lang, id, args...) }
	option := e.Key
	if option == "" {
		option = "--out"
	}
	switch e.Reason {
	case errkind.ReasonOutputExists:
		return t("err.outExists", e.Subject), t("err.outExistsFix", option)
	case errkind.ReasonOutputFolderMissing:
		return t("err.outFolder", e.Subject), t("err.outFolderFix", option)
	case errkind.ReasonOutputTooLarge:
		return t("err.outTooLarge", e.Subject), t("err.outTooLargeFix")
	}
	return t("err.outFailed", e.Subject), t("err.outFailedFix")
}

// describeFileError formats rule file and plan file errors as "<file> line
// <n>: <detail>".
func describeFileError(e *errkind.Error, lang display.Lang) (string, string) {
	what := display.T(lang, "err.file", e.Subject, e.Detail)
	if e.Line > 0 {
		what = display.T(lang, "err.fileLine", e.Subject, e.Line, e.Detail)
	}
	fix := e.Hint
	if fix == "" {
		fix = display.T(lang, "err.checkRules")
	}
	return what, fix
}

func describeCredentials(e *errkind.Error, lang display.Lang) (string, string) {
	t := func(id string, args ...any) string { return display.T(lang, id, args...) }
	login := `"aws sso login"`
	if e.Subject != "" && e.Subject != "default" {
		login = fmt.Sprintf(`"aws sso login --profile %s"`, e.Subject)
	}
	switch e.Reason {
	case errkind.ReasonSSOSessionExpired:
		return t("err.ssoExpired"), t("err.ssoExpiredFix", login)
	case errkind.ReasonCredentialsExpired:
		return t("err.credExpired", e.Op), t("err.credExpiredFix", login)
	case errkind.ReasonCredentialsRejected:
		return t("err.credRejected", e.Op), t("err.credRejectedFix")
	case errkind.ReasonCredentialsTimeout:
		return t("err.credTimeout"), t("err.credTimeoutFix")
	default:
		return t("err.credMissing"), t("err.credMissingFix")
	}
}

// describeInternal names the operation only; the error text and its type are
// kept out of the message (security design §5).
func describeInternal(e *errkind.Error, lang display.Lang) (string, string) {
	where := e.Op
	if where == "" {
		where = "tag-refactor"
	}
	what := display.T(lang, "err.internal", where, inRegion(e.Region, lang))
	if e.Detail != "" {
		what = display.T(lang, "err.internalDetail", where, inRegion(e.Region, lang), e.Detail)
	}
	return what, display.T(lang, "err.internalFix")
}
