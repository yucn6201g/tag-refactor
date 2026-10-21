package errkind

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestKindStringUsesContractNames(t *testing.T) {
	want := map[Kind]string{
		Internal: "Internal", InvalidInput: "InvalidInput", UnsupportedVersion: "UnsupportedVersion",
		NotFound: "NotFound", AccessDenied: "AccessDenied", CredentialsMissing: "CredentialsMissing",
		Transient: "Transient", StorageFailed: "StorageFailed", AccountMismatch: "AccountMismatch",
		Conflict: "Conflict", Unavailable: "Unavailable",
	}
	for k, name := range want {
		if got := k.String(); got != name {
			t.Errorf("%d.String() = %q, want %q", int(k), got, name)
		}
	}
	if got := Kind(99).String(); got != "Kind(99)" {
		t.Errorf("unknown kind = %q", got)
	}
	if got := Kind(-1).String(); got != "Kind(-1)" {
		t.Errorf("negative kind = %q", got)
	}
}

func TestErrorTextNeverIncludesTheCause(t *testing.T) {
	secret := "AKIAFAKEFAKEFAKE0000 is not authorized"
	cause := fmt.Errorf("sdk: %s", secret)
	e := &Error{Kind: AccessDenied, Op: "tag:GetResources", Detail: "access denied", Cause: cause}

	if strings.Contains(e.Error(), "AKIA") {
		t.Fatalf("Error() leaks the cause: %q", e.Error())
	}
	if e.Error() != "AccessDenied (tag:GetResources): access denied" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Fatal("cause is not reachable with errors.Is")
	}
	if got := New(NotFound, "missing").Error(); got != "NotFound: missing" {
		t.Fatalf("New().Error() = %q", got)
	}
	if got := (&Error{Kind: Transient}).Error(); got != "Transient" {
		t.Fatalf("bare Error() = %q", got)
	}
}

func TestKindOfAsAndEnsure(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", &Error{Kind: Transient})
	if KindOf(wrapped) != Transient {
		t.Fatalf("KindOf(wrapped) = %v", KindOf(wrapped))
	}
	if KindOf(errors.New("plain")) != Internal {
		t.Fatal("an unclassified error must be Internal")
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Fatal("As found a kind in a plain error")
	}
	if Ensure(nil, "op") != nil {
		t.Fatal("Ensure(nil) must be nil")
	}
	kept := &Error{Kind: NotFound}
	if Ensure(fmt.Errorf("x: %w", kept), "op") != kept {
		t.Fatal("Ensure must return the existing *Error")
	}
	plain := errors.New("boom")
	got := Ensure(plain, "load")
	if got.Kind != Internal || got.Op != "load" || !errors.Is(got, plain) {
		t.Fatalf("Ensure(plain) = %+v", got)
	}
}

func TestCauseTypeNamesTheInnermostError(t *testing.T) {
	err := &Error{Kind: Internal, Cause: fmt.Errorf("send: %w", &net.DNSError{Name: "x"})}
	if got := CauseType(err); got != "*net.DNSError" {
		t.Fatalf("CauseType = %q", got)
	}
	if got := CauseType(nil); got != "" {
		t.Fatalf("CauseType(nil) = %q", got)
	}
}
