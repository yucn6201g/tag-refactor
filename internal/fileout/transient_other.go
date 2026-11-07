//go:build !windows

package fileout

// TransientRename reports whether a rename failed only for a moment. Outside
// Windows an open file does not stop a rename, so no error is (U8 security
// design §7).
func TransientRename(error) bool { return false }
