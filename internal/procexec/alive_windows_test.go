//go:build windows

package procexec

import "golang.org/x/sys/windows"

// stillActive is the exit code of a process that runs (STILL_ACTIVE).
const stillActive = 259

func alive(pid int) bool {
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

func processGroup(int) int { return 0 }

func hasProcessGroups() bool { return false }
