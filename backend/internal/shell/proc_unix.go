//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"syscall"
)

func setupCmdForProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcessTree(proc *os.Process) {
	if proc == nil || proc.Pid <= 0 {
		return
	}
	_ = syscall.Kill(-proc.Pid, syscall.SIGKILL)
}
