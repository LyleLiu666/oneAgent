//go:build linux

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	if strings.TrimSpace(os.Getenv(nativeSandboxHelperEnv)) == "" {
		return
	}
	os.Exit(runNativeSandboxHelperLinux())
}

func runNativeSandboxHelperLinux() int {
	// Avoid recursion.
	_ = os.Unsetenv(nativeSandboxHelperEnv)

	rootRaw := strings.TrimSpace(os.Getenv(nativeSandboxRootEnv))
	command := os.Getenv(nativeSandboxCommandEnv)
	startDir := strings.TrimSpace(os.Getenv(nativeSandboxStartDirEnv))
	if rootRaw == "" {
		fmt.Fprintln(os.Stderr, "native sandbox helper: missing root")
		return 2
	}
	if strings.TrimSpace(command) == "" {
		fmt.Fprintln(os.Stderr, "native sandbox helper: missing command")
		return 2
	}

	root, err := ResolveBashRoot(rootRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "native sandbox helper:", err.Error())
		return 2
	}
	if startDir == "" {
		startDir = root
	}
	if err := ensurePathWithinRoot(root, startDir); err != nil {
		fmt.Fprintln(os.Stderr, "native sandbox helper: invalid working directory:", err.Error())
		return 2
	}

	if err := applyLandlockWriteSandbox(root); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	shellPath, err := ResolveBashPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "native sandbox helper:", err.Error())
		return 2
	}

	cmd := exec.Command(shellPath, "--noprofile", "--norc", "-lc", command)
	cmd.Dir = startDir
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()
	if runErr == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		code := exitErr.ExitCode()
		if code == 0 {
			return 1
		}
		return code
	}
	fmt.Fprintln(os.Stderr, "native sandbox helper:", runErr.Error())
	return 2
}

