package fileout

import (
	"errors"
	"syscall"
)

// The Windows errors of a rename while another program has the file open.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

// TransientRename reports whether a rename failed only because another program
// has the file open at the moment (U8 security design §7): on Windows a sharing
// violation, a lock violation or an access denied.
func TransientRename(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorAccessDenied || errno == errorSharingViolation || errno == errorLockViolation
}
