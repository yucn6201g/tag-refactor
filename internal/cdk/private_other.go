//go:build !windows

package cdk

import "os"

// privateDir makes a temporary folder only the user can enter (0700, security
// design §5).
func privateDir() (string, error) {
	dir, err := os.MkdirTemp("", "tag-refactor-helper-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
