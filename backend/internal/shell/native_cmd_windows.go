//go:build windows

package shell

import (
	"os"
	"os/exec"
	"syscall"
)

func newNativeBashCommand(command string, root string, tmpDir string, _ string) (*exec.Cmd, func(), error) {
	shellPath, err := ResolveBashPath()
	if err != nil {
		return nil, nil, err
	}

	token, cleanupToken, err := createWindowsNativeSandboxToken(root)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(shellPath, "--noprofile", "--norc", "-lc", command)
	cmd.Dir = root
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"HOME":          root,
		"PWD":           root,
		"BASH_ROOT_DIR": root,
		"TMPDIR":        tmpDir,
		"TMP":           tmpDir,
		"TEMP":          tmpDir,
		"BASH_ENV":      "",
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{Token: token}
	setupCmdForProcessGroup(cmd)

	cleanupAfterStart := func() { cleanupToken() }
	return cmd, cleanupAfterStart, nil
}

