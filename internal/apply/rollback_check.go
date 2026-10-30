package apply

import (
	"regexp"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/journal"
)

// The checks of the record before anything is read or written (U7 BR1.3,
// NFR3.15, security design §2). The record is input that others who can
// write its folder or bucket may have changed: every entry is checked, not
// only the targets, since one entry out of form is itself the sign of a
// record changed by hand.

// The limits of AWS tags (security design §2 (f)), counted in characters.
const (
	maxKeyLength   = 128
	maxValueLength = 256
)

// regionPattern is the form of a region name: lowercase letters, digits and
// hyphens, starting with a letter and ending with a digit.
var regionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[0-9]$`)

// checkRecord applies the checks (a) to (f) to the record read from where:
// the account (AccountMismatch) and the run ID, then every entry (InvalidInput
// naming the entry and the check).
func checkRecord(rec *journal.RunRecord, accountID, runID, where string) error {
	if rec.AccountID != accountID {
		return &errkind.Error{Kind: errkind.AccountMismatch, Subject: where,
			DetailID: "rollback.accountMismatch", DetailArgs: []any{where, rec.AccountID, accountID},
			HintID: "err.accountMismatchFix", HintArgs: []any{rec.AccountID}}
	}
	if rec.RunID != runID {
		return recordError(where, "rollback.recordRunID", rec.RunID, runID)
	}
	seen := make(map[string]bool, len(rec.Entries))
	for i := range rec.Entries {
		e := &rec.Entries[i]
		if seen[e.ARN] {
			return recordError(where, "rollback.recordDuplicate", e.ARN)
		}
		seen[e.ARN] = true
		if err := checkEntry(e, accountID, where); err != nil {
			return err
		}
	}
	return nil
}

func recordError(where, id string, args ...any) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Subject: where, DetailID: id, DetailArgs: append([]any{where}, args...),
		Detail: "the run record does not pass the checks of rollback", HintID: "rollback.recordFix"}
}

// checkEntry applies the checks (b) to (f) to one entry.
func checkEntry(e *journal.Entry, accountID, where string) error {
	arn, ok := awsaccess.ParseARN(e.ARN)
	if !ok || arn.Partition == "" || arn.Service == "" || arn.Resource == "" {
		return recordError(where, "rollback.recordARN", e.ARN)
	}
	if arn.Account != "" && arn.Account != accountID {
		return recordError(where, "rollback.recordAccount", e.ARN, accountID)
	}
	if !regionPattern.MatchString(e.Region) {
		return recordError(where, "rollback.recordRegion", e.ARN, e.Region)
	}
	if arn.Region != "" && arn.Region != e.Region {
		return recordError(where, "rollback.recordRegionMismatch", e.ARN, e.Region)
	}
	for _, k := range e.TouchedKeys {
		_, inBefore := e.BeforeTags[k]
		_, inPlanned := e.PlannedAfterTags[k]
		switch {
		case awsaccess.IsAWSKey(k):
			return recordError(where, "rollback.recordAWSKey", e.ARN, k)
		case !inBefore && !inPlanned || keysMatch([]string{k}, e.BeforeTags, e.PlannedAfterTags):
			return recordError(where, "rollback.recordTouched", e.ARN, k)
		}
	}
	for _, tags := range []map[string]string{e.BeforeTags, e.PlannedAfterTags} {
		for k, v := range tags {
			if utf8.RuneCountInString(k) > maxKeyLength || utf8.RuneCountInString(v) > maxValueLength {
				return recordError(where, "rollback.recordLength", e.ARN, maxKeyLength, maxValueLength)
			}
		}
	}
	if n := len(userTags(e.BeforeTags)); n > MaxTags {
		return recordError(where, "rollback.recordTooManyTags", e.ARN, n)
	}
	return nil
}
