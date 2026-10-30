// Package journal keeps the run records of apply and rollback (contracts C4
// and C10) and the lock that stops two of them from running for the same
// account at once (U6 BR3.1, BR3.8, BR5.1 to BR5.5).
//
// A record is saved so that a stop at any point leaves either the previous
// version or the new one: a local record is written to a temporary file in
// the same folder, synced to disk and renamed over the record; an S3 record
// is replaced by one PutObject (NFR6.1). While apply writes, a goroutine
// saves the record every five seconds without making the writes wait
// (NFR6.2). The lock is a local file created only if it does not exist and,
// when the records go to S3, an object written only if it does not exist.
//
// The package reaches S3 only through the ObjectStore interface of
// awsaccess, and never prints: it reports warnings through a callback with
// the identifiers of the display's message catalog.
package journal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Version is the version of the run record this tool writes and reads.
const Version = 1

// Status is the status of an entry of the record (contract C4).
type Status string

// The statuses of contract C4.
const (
	StatusPending Status = "pending"
	StatusApplied Status = "applied"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
)

// Entry is one resource of the record (contract C4 entries). Its maps are
// never changed once they are stored, so a copy of the slice of entries is a
// copy of the record.
type Entry struct {
	ARN              string            `json:"arn"`
	Region           string            `json:"region"`
	TouchedKeys      []string          `json:"touchedKeys"`
	BeforeTags       map[string]string `json:"beforeTags"`
	PlannedAfterTags map[string]string `json:"plannedAfterTags"`
	AfterTags        map[string]string `json:"afterTags,omitempty"`
	Status           Status            `json:"status"`
	Reason           string            `json:"reason,omitempty"`
}

// RollbackResult is the result of one resource of a rollback (contract C4;
// U7 writes it). Status is restored, skipped, failed or alreadyRestored;
// Reason is the name of the reason, never an AWS error text.
type RollbackResult struct {
	ARN    string `json:"arn"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// RollbackRecord is one rollback in the rollbacks of a run record (contract
// C4; U7 writes it while a RollbackRun runs).
type RollbackRecord struct {
	RollbackID string           `json:"rollbackId"`
	StartedAt  string           `json:"startedAt"`
	Completed  bool             `json:"completed"`
	Results    []RollbackResult `json:"results"`
}

// RunRecord is the run record of contract C4.
type RunRecord struct {
	Version     int              `json:"version"`
	ToolVersion string           `json:"toolVersion"`
	RunID       string           `json:"runId"`
	AccountID   string           `json:"accountId"`
	PlanID      string           `json:"planId"`
	StartedAt   string           `json:"startedAt"`
	Completed   bool             `json:"completed"`
	Entries     []Entry          `json:"entries"`
	Rollbacks   []RollbackRecord `json:"rollbacks,omitempty"`
}

// Purpose says what holds the lock.
type Purpose string

// The purposes of a lock (U6 entities LockInfo).
const (
	PurposeApply    Purpose = "apply"
	PurposeRollback Purpose = "rollback"
)

// LockInfo is the content of a lock: enough for a person to tell whether the
// run that made it is still running (U6 BR3.2, NFR8.23: nothing else).
type LockInfo struct {
	AccountID   string  `json:"accountId"`
	Purpose     Purpose `json:"purpose"`
	RunID       string  `json:"runId"`
	CreatedAt   string  `json:"createdAt"`
	Host        string  `json:"host"`
	PID         int     `json:"pid"`
	ToolVersion string  `json:"toolVersion"`
}

// Warning identifiers: the messages of the display's catalog the journal
// reports through Options.Warn.
const (
	// WarnFolderReadable: the record folder %s can be read or written by
	// other users.
	WarnFolderReadable = "journal.folderReadable"
	// WarnLockReplaced: the lock %s holds another run (%s), so it was kept.
	WarnLockReplaced = "journal.lockReplaced"
)

// WarningIDs lists every warning identifier, for the catalog test of the
// display.
var WarningIDs = []string{WarnFolderReadable, WarnLockReplaced}

// Options configure a Journal.
type Options struct {
	// Home is the user's home folder; the default records and the local lock
	// go to <Home>/.tag-refactor/runs/<accountId>/ (BR5.1).
	Home string
	// Store reaches S3; it is needed only for S3 locations.
	Store awsaccess.ObjectStore
	// ToolVersion is written into records and locks.
	ToolVersion string
	// Now gives the times of records and locks; nil means time.Now.
	Now func() time.Time
	// Host and PID go into locks; empty and 0 mean the real ones.
	Host string
	PID  int
	// Warn receives warnings as catalog identifiers with their values; nil
	// drops them.
	Warn func(id string, args ...any)
}

// Journal keeps run records and locks.
type Journal struct {
	opts Options
}

// New returns a journal.
func New(opts Options) *Journal {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Host == "" {
		opts.Host, _ = os.Hostname()
	}
	if opts.PID == 0 {
		opts.PID = os.Getpid()
	}
	if opts.Warn == nil {
		opts.Warn = func(string, ...any) {}
	}
	return &Journal{opts: opts}
}

// DefaultDir is the folder of the local lock, the log files and, without
// --record-to, the records of an account (BR5.1).
func DefaultDir(home, accountID string) string {
	return filepath.Join(home, ".tag-refactor", "runs", accountID)
}

func (j *Journal) defaultDir(accountID string) string { return DefaultDir(j.opts.Home, accountID) }

// timeFormat writes times in UTC to the second.
const timeFormat = "2006-01-02T15:04:05Z"

func (j *Journal) now() string { return j.opts.Now().UTC().Format(timeFormat) }

// runIDPattern is the form of a run ID (BR5.4).
var runIDPattern = regexp.MustCompile(`^\d{8}T\d{6}Z-[0-9a-f]{4}$`)

// NewRunID returns <yyyymmdd>T<hhmmss>Z-<4 hexadecimal digits> for now in
// UTC (BR5.4). random nil means crypto/rand.
func NewRunID(now time.Time, random io.Reader) string {
	if random == nil {
		random = rand.Reader
	}
	var b [2]byte
	if _, err := io.ReadFull(random, b[:]); err != nil {
		b = [2]byte{}
	}
	return now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// ValidRunID reports whether id has the form of a run ID.
func ValidRunID(id string) bool { return runIDPattern.MatchString(id) }

// storageError is a StorageFailed about subject (a path or an S3 URI), with
// the English detail and its catalog message.
func storageError(subject, detailID, detail string, cause error) *errkind.Error {
	return &errkind.Error{Kind: errkind.StorageFailed, Subject: subject, Detail: detail, DetailID: detailID,
		Hint: "check that the folder or the bucket can be written and has free space", HintID: "journal.hintStorage", Cause: cause}
}

// marshal writes a record or a lock as indented JSON.
func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// cleanField keeps a field of the log on its line: tabs and line breaks
// become spaces.
func cleanField(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}
