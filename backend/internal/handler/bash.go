package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type bashRunRequest struct {
	Command   string `json:"command" binding:"required"`
	TimeoutMs int    `json:"timeout_ms"`
	WorkDir   string `json:"work_dir"`
}

type bashRunResponse struct {
	Shell           string `json:"shell"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExitCode        int    `json:"exit_code"`
	DurationMs      int64  `json:"duration_ms"`
	TimedOut        bool   `json:"timed_out"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	CWD             string `json:"cwd"`
}

// RunBash executes a bash command for the current user.
func RunBash(c *gin.Context) {
	var req bashRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}

	cfg := config.GetConfig()
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	result, err := shell.RunBash(c.Request.Context(), req.Command, timeout, cfg.BashRootDir, req.WorkDir)
	if err != nil {
		var unsafeErr *shell.UnsafeCommandError
		if errors.As(err, &unsafeErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": unsafeErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bashRunResponse{
		Shell:           result.Shell,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		ExitCode:        result.ExitCode,
		DurationMs:      result.Duration.Milliseconds(),
		TimedOut:        result.TimedOut,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
		CWD:             result.CWD,
	})
}
