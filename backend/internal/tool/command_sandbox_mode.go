package tool

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

func commandToolSandboxMode(dec permissions.Decision) (shell.SandboxMode, error) {
	raw := strings.TrimSpace(dec.Constraints.SandboxMode)
	if raw != "" {
		return shell.ParseSandboxMode(raw)
	}

	// Default behavior for command tools:
	// - On macOS, prefer native sandbox when available (no Docker dependency).
	// - Otherwise, fail closed to sandbox_mode=none (command tools will degrade to readonly).
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("sandbox-exec"); err == nil {
			return shell.SandboxModeNative, nil
		}
	}
	return shell.SandboxModeNone, nil
}
