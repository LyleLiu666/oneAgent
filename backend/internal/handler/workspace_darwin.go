//go:build darwin

package handler

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func defaultChooseWorkspaceDir(ctx context.Context) (string, bool, error) {
	return chooseWorkspaceDarwin(ctx)
}

func chooseWorkspaceDarwin(ctx context.Context) (string, bool, error) {
	// AppleScript: show folder picker and return POSIX path.
	const script = `POSIX path of (choose folder with prompt "Select workspace folder")`
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "user canceled") || strings.Contains(lower, "cancelled") {
			return "", true, nil
		}
		if text == "" {
			return "", false, fmt.Errorf("choose folder failed: %w", err)
		}
		return "", false, fmt.Errorf("choose folder failed: %w: %s", err, text)
	}
	if text == "" {
		return "", true, nil
	}
	return text, false, nil
}
