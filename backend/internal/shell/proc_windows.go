//go:build windows

package shell

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func setupCmdForProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	// Best-effort process grouping for later tree termination.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcessTree(proc *os.Process) {
	if proc == nil || proc.Pid <= 0 {
		return
	}
	// Best-effort: terminate the process tree. This is important because bash typically spawns child processes.
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(proc.Pid)).Run()
	_ = proc.Kill()
}
