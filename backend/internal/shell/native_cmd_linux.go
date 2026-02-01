//go:build linux

package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func newNativeBashCommand(command string, root string, tmpDir string, _ string) (*exec.Cmd, func(), error) {
	if _, err := landlockABIVersion(); err != nil {
		return nil, nil, fmt.Errorf("native sandbox unavailable: %w (use sandbox_mode=docker)", err)
	}

	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return nil, nil, fmt.Errorf("native sandbox unavailable: resolve executable: %w", err)
	}

	cmd := exec.Command(exe)
	cmd.Dir = root
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		nativeSandboxHelperEnv:   "1",
		nativeSandboxRootEnv:     root,
		nativeSandboxStartDirEnv: root,
		nativeSandboxCommandEnv:  command,
		"HOME":                   root,
		"PWD":                    root,
		"BASH_ROOT_DIR":          root,
		"TMPDIR":                 tmpDir,
		"TMP":                    tmpDir,
		"TEMP":                   tmpDir,
		"BASH_ENV":               "",
	})
	setupCmdForProcessGroup(cmd)
	return cmd, func() {}, nil
}

