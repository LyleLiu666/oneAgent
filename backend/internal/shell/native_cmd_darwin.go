//go:build darwin

package shell

import (
	"fmt"
	"os"
	"os/exec"
)

func newNativeBashCommand(command string, root string, tmpDir string, profileDir string) (*exec.Cmd, string, error) {
	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return nil, "", fmt.Errorf("native sandbox unavailable: sandbox-exec not found (expected /usr/bin/sandbox-exec)")
	}

	shellPath, err := ResolveBashPath()
	if err != nil {
		return nil, "", err
	}

	profilePath, err := writeNativeSandboxProfile(profileDir, root)
	if err != nil {
		return nil, "", err
	}

	cmd := exec.Command(sandboxExecPath, "-f", profilePath, shellPath, "--noprofile", "--norc", "-lc", command)
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
	setupCmdForProcessGroup(cmd)
	return cmd, profilePath, nil
}

