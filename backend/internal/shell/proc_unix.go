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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(proc *os.Process) {
	if proc == nil || proc.Pid <= 0 {
		return
	}
	_ = syscall.Kill(-proc.Pid, syscall.SIGKILL)
}
