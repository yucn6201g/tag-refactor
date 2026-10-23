// Package errkind defines the error kinds shared by every part of tag-refactor
// (contract C12). Parts return errors with a kind; only the cli package turns
// a kind into an exit code and a two-line message.
//
// An *Error never exposes the text of the error it wraps: AWS SDK messages can
// echo request details, so only tool-authored fields (Detail, Hint, Op,
// Region, Subject, Key, Line) are meant to be shown.
package errkind

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Kind is the kind of an error (contract C12).
type Kind int

// The error kinds of contract C12. Internal is the zero value so an error that
// was never classified is treated as unexpected.
const (
	Internal Kind = iota
	InvalidInput
	UnsupportedVersion
	NotFound
	AccessDenied
	CredentialsMissing
	Transient
	StorageFailed
	AccountMismatch
	Conflict
	Unavailable
)

var kindNames = [...]string{
	Internal:           "Internal",
	InvalidInput:       "InvalidInput",
	UnsupportedVersion: "UnsupportedVersion",
	NotFound:           "NotFound",
	AccessDenied:       "AccessDenied",
	CredentialsMissing: "CredentialsMissing",
	Transient:          "Transient",
	StorageFailed:      "StorageFailed",
	AccountMismatch:    "AccountMismatch",
	Conflict:           "Conflict",
	Unavailable:        "Unavailable",
}

// String returns the contract name of the kind, for example "AccessDenied".
func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return fmt.Sprintf("Kind(%d)", int(k))
	}
	return kindNames[k]
}

// Reason refines a kind so the cli package can choose the message. It is a
// clue for the wording, not a separate kind.
type Reason string

// Reasons used in U1.
const (
	ReasonNone                Reason = ""
	ReasonCredentialsNotFound Reason = "credentials-not-found"
	ReasonCredentialsExpired  Reason = "credentials-expired"
	ReasonSSOSessionExpired   Reason = "sso-session-expired"
	ReasonCredentialsRejected Reason = "credentials-rejected"
	ReasonCredentialsTimeout  Reason = "credentials-timeout"
	ReasonProfileNotFound     Reason = "profile-not-found"
	ReasonRegionNotSet        Reason = "region-not-set"
	ReasonRegionInvalid       Reason = "region-invalid"
	ReasonRegionUnavailable   Reason = "region-unavailable"
	ReasonTimeout             Reason = "timeout"
	ReasonRetriesExhausted    Reason = "retries-exhausted"
	ReasonUsage               Reason = "usage"
	ReasonOptionNotAvailable  Reason = "option-not-available"
	ReasonRuleFile            Reason = "rule-file"
	// ReasonNoDefaultView is an AccessDenied of Resource Explorer: the region
	// has no default view, so ListResources cannot be used (U2, C12).
	ReasonNoDefaultView Reason = "no-default-view"
)

// Error is an error with a kind. Only the tool-authored fields are safe to
// print; Cause is kept for errors.Is and errors.As but its text is never shown.
type Error struct {
	Kind   Kind
	Reason Reason
	// Op is the operation that failed, for example "tag:GetResources".
	Op string
	// Region is the AWS region of the failed call, if any.
	Region string
	// Subject is what the error is about: a profile name, a file path or an
	// option name.
	Subject string
	// Line and Key locate an error in the rule file. Line is 1-based; 0 means
	// no line.
	Line int
	Key  string
	// Detail says what happened, written by the tool.
	Detail string
	// Hint says how to fix it, written by the tool.
	Hint string
	// Cause is the underlying error. Its text is never printed.
	Cause error
}

// Error returns a summary built only from tool-authored fields.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Kind.String())
	if e.Op != "" {
		b.WriteString(" (")
		b.WriteString(e.Op)
		b.WriteString(")")
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error { return e.Cause }

// New returns an error of the given kind with a tool-authored detail.
func New(kind Kind, detail string) *Error {
	return &Error{Kind: kind, Detail: detail}
}

// As returns the first *Error in err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// KindOf returns the kind of err. Errors without a kind are Internal.
func KindOf(err error) Kind {
	if e, ok := As(err); ok {
		return e.Kind
	}
	return Internal
}

// Ensure returns err as an *Error, wrapping an unclassified error as Internal
// for the given operation. It returns nil for nil.
func Ensure(err error, op string) *Error {
	if err == nil {
		return nil
	}
	if e, ok := As(err); ok {
		return e
	}
	return &Error{Kind: Internal, Op: op, Cause: err}
}

// CauseType returns the Go type name of the innermost cause of err, for
// example "*net.OpError". It is shown in --debug for Internal errors instead
// of the error text.
func CauseType(err error) string {
	if err == nil {
		return ""
	}
	inner := err
	for {
		next := errors.Unwrap(inner)
		if next == nil {
			break
		}
		inner = next
	}
	return reflect.TypeOf(inner).String()
}
