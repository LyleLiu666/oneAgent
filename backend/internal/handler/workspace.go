package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/scope"
)

var ErrWorkspaceChooserNotSupported = errors.New("workspace chooser not supported")

// chooseWorkspaceDir is overridden in tests.
var chooseWorkspaceDir = defaultChooseWorkspaceDir

// ChooseWorkspace opens a native folder picker on the server machine and returns the selected path.
// This is primarily intended for local-tool mode where the server runs on the same machine as the UI.
func ChooseWorkspace(c *gin.Context) {
	selected, canceled, err := chooseWorkspaceDir(c.Request.Context())
	if err != nil {
		if errors.Is(err, ErrWorkspaceChooserNotSupported) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if canceled || strings.TrimSpace(selected) == "" {
		c.JSON(http.StatusOK, gin.H{"canceled": true})
		return
	}

	normalized, err := scope.NormalizeWorkspaceRoot(selected)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"path": normalized})
}

func defaultChooseWorkspaceDir(ctx context.Context) (string, bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return chooseWorkspaceDarwin(ctx)
	default:
		return "", false, fmt.Errorf("%w: %s", ErrWorkspaceChooserNotSupported, runtime.GOOS)
	}
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
