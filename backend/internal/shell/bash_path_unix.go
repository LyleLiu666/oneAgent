//go:build !windows

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ResolveBashPath finds a usable bash binary on macOS/Linux.
func ResolveBashPath() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("LYLE_BASH_PATH")); explicit != "" {
		if err := assertShellNotBlacklisted(explicit); err != nil {
			return "", err
		}
		if !isExecutable(explicit) {
			return "", fmt.Errorf("bash not found at: %s", explicit)
		}
		return explicit, nil
	}

	if bashPath, err := exec.LookPath("bash"); err == nil {
		return bashPath, nil
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/bin/bash", "/usr/bin/bash", "/opt/homebrew/bin/bash"}
	default:
		candidates = []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash"}
	}

	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	return "", errors.New("bash not found on this system")
}
