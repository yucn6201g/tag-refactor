//go:build !windows

package procexec

import (
	"os/exec"
	"syscall"
)

// processTree is the group of processes of one program on Unix: a process
// group of its own when it is not interactive, so that a stop reaches its
// children too; the foreground group of tag-refactor when it is, so that it
// can read the terminal.
type processTree struct {
	cmd         *exec.Cmd
	interactive bool
}

func newProcessTree(cmd *exec.Cmd, interactive bool) *processTree {
	t := &processTree{cmd: cmd, interactive: interactive}
	if !interactive {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	cmd.Cancel = t.stop
	return t
}

func (t *processTree) start() error { return t.cmd.Start() }

// stop kills the group, or only the process of an interactive program.
func (t *processTree) stop() error {
	if t.cmd.Process == nil {
		return nil
	}
	if !t.interactive {
		if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL); err == nil {
			return nil
		}
	}
	return t.cmd.Process.Kill()
}

// release kills what is left of the group of a program that is not
// interactive, as the job object does on Windows. A group with nobody left
// answers ESRCH, which is fine.
func (t *processTree) release() {
	if !t.interactive && t.cmd.Process != nil && t.cmd.ProcessState != nil {
		_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	}
}
