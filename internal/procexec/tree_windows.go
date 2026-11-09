//go:build windows

package procexec

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processTree is the job object of one program on Windows. The program is
// created suspended, put in a job object that kills everything in it when it
// is closed, and only then resumed, so that no child it makes can start
// outside the job (reliability design §1).
type processTree struct {
	cmd     *exec.Cmd
	job     windows.Handle
	process windows.Handle
}

var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

func newProcessTree(cmd *exec.Cmd, _ bool) *processTree {
	t := &processTree{cmd: cmd}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	cmd.Cancel = t.stop
	return t
}

func (t *processTree) start() error {
	job, err := newKillingJob()
	if err != nil {
		return err
	}
	t.job = job
	if err := t.cmd.Start(); err != nil {
		return err
	}
	if err := t.adopt(); err != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
		return err
	}
	return nil
}

// adopt puts the suspended process in the job and resumes it.
func (t *processTree) adopt() error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME|
		windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(t.cmd.Process.Pid))
	if err != nil {
		return err
	}
	t.process = h
	if err := windows.AssignProcessToJobObject(t.job, h); err != nil {
		return err
	}
	if status, _, _ := ntResumeProcess.Call(uintptr(h)); status != 0 {
		return errors.New("NtResumeProcess failed")
	}
	return nil
}

func newKillingJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// stop ends every process of the job.
func (t *processTree) stop() error {
	if t.job != 0 {
		return windows.TerminateJobObject(t.job, 1)
	}
	if t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}
	return nil
}

// release closes the handles; closing the job ends what is left in it.
func (t *processTree) release() {
	if t.process != 0 {
		_ = windows.CloseHandle(t.process)
	}
	if t.job != 0 {
		_ = windows.CloseHandle(t.job)
	}
}
