//go:build !windows

package procexec

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

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

// alive reports a running process; a zombie that its parent has not waited for
// counts as gone.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	return !zombie(pid)
}

func processGroup(pid int) int {
	g, err := syscall.Getpgid(pid)
	if err != nil {
		return -1
	}
	return g
}

func hasProcessGroups() bool { return true }
