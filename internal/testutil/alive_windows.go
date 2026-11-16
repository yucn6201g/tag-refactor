//go:build windows

package testutil

import "golang.org/x/sys/windows"

// stillActive is the exit code of a process that runs (STILL_ACTIVE).
const stillActive = 259

// ProcessAlive reports a running process (v0.2.0 U3 NFR3.16): the tests of
// synth look with it that no grandchild of cdk is left. It looks as the tests
// of internal/procexec do.
func ProcessAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
