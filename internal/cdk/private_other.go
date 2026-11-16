//go:build !windows

package cdk

import "os"

// privateDir makes a temporary folder only the user can enter (0700, security
// design §5), in base (empty: the temporary folder of the system).
func privateDir(base string) (string, error) {
	dir, err := os.MkdirTemp(base, "tag-refactor-helper-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
