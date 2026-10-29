package journal

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// LocationKind says where the records go.
type LocationKind string

// The kinds of location (U6 entities RecordLocation).
const (
	LocationLocal LocationKind = "local"
	LocationS3    LocationKind = "s3"
)

// Location is where the records of a run go (BR5.1, BR5.2). The lock and
// the log file stay in the default folder whatever the location is.
type Location struct {
	Kind LocationKind
	// Dir is the folder given to --record-to, or empty for the default
	// folder; the records go to <Dir>/<accountId>/.
	Dir string
	// Bucket and Prefix are the parts of s3://<bucket>/<prefix>; Prefix has
	// no slash at either end and may be empty.
	Bucket, Prefix string
	// Value is --record-to as given, for --record-from of the rollback
	// command (BR6.3); empty for the default.
	Value string
}

// bucketPattern is the form of an S3 bucket name: 3 to 63 lowercase letters,
// digits, dots and hyphens, starting and ending with a letter or a digit.
var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// ParseLocation reads a --record-to value (BR5.2): "" is the default local
// folder, s3://<bucket>/<prefix> is S3, and anything else is a local
// folder. A wrong S3 value is a usage error.
func ParseLocation(value string) (Location, error) {
	if value == "" {
		return Location{Kind: LocationLocal}, nil
	}
	rest, isS3 := strings.CutPrefix(value, "s3://")
	if !isS3 {
		dir, err := filepath.Abs(value)
		if err != nil {
			return Location{}, badLocation(value)
		}
		return Location{Kind: LocationLocal, Dir: dir, Value: value}, nil
	}
	bucket, prefix, _ := strings.Cut(rest, "/")
	prefix = strings.Trim(prefix, "/")
	if !bucketPattern.MatchString(bucket) || strings.Contains(bucket, "..") || strings.Contains(prefix, "//") ||
		strings.IndexFunc(prefix, unicode.IsControl) >= 0 {
		return Location{}, badLocation(value)
	}
	return Location{Kind: LocationS3, Bucket: bucket, Prefix: prefix, Value: value}, nil
}

func badLocation(value string) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--record-to",
		Detail: `--record-to "` + value + `" is not a folder or s3://<bucket>/<prefix>`, DetailID: "journal.badLocation", DetailArgs: []any{value},
		Hint: "pass a folder, or s3://<bucket>/<prefix> with a valid bucket name", HintID: "journal.badLocationFix"}
}

// s3Key is the key of name under the location's prefix and the account.
func (l Location) s3Key(accountID, name string) string {
	if l.Prefix == "" {
		return accountID + "/" + name
	}
	return l.Prefix + "/" + accountID + "/" + name
}

// s3URI is the URI of name under the location's prefix and the account.
func (l Location) s3URI(accountID, name string) string {
	return "s3://" + l.Bucket + "/" + l.s3Key(accountID, name)
}

// RecordDir is the folder (or S3 URI ending in /) of the records of the
// account at the location, as the confirmation shows it (BR2.1).
func (l Location) RecordDir(home, accountID string) string {
	switch {
	case l.Kind == LocationS3:
		return l.s3URI(accountID, "")
	case l.Dir != "":
		return filepath.Join(l.Dir, accountID) + string(filepath.Separator)
	}
	return DefaultDir(home, accountID) + string(filepath.Separator)
}

// localDir is the folder of the records of a local location.
func (l Location) localDir(home, accountID string) string {
	if l.Dir != "" {
		return filepath.Join(l.Dir, accountID)
	}
	return DefaultDir(home, accountID)
}
