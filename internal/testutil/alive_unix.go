//go:build !windows

package testutil

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ProcessAlive reports a running process (v0.2.0 U3 NFR3.16): the tests of
// synth look with it that no grandchild of cdk is left. A zombie that its
// parent has not waited for counts as gone. It looks as the tests of
// internal/procexec do.
func ProcessAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	return !zombie(pid)
}

// zombie reads the state of the process on Linux; elsewhere it says no.
func zombie(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// The state follows the name in brackets, which can hold spaces.
	rest := string(data[strings.LastIndexByte(string(data), ')')+1:])
	return strings.HasPrefix(strings.TrimSpace(rest), "Z")
}
