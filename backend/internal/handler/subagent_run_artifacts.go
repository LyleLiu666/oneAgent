package handler

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

func GetSubagentRunArtifact(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Layout == nil || strings.TrimSpace(rt.Layout.SubagentLogsDir) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := strings.TrimSpace(middleware.GetUserID(c))
	if userID == "" {
		userID = "local"
	}

	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}
	if rt.Sessions != nil {
		if _, _, err := rt.Sessions.GetSessionWithMessages(sessionID, userID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	}

	runID := strings.TrimSpace(c.Param("run_id"))
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := uuid.Parse(runID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
		return
	}

	kind := strings.TrimSpace(c.Param("kind"))
	if kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind is required"})
		return
	}

	runDir, err := findSubagentRunDir(rt.Layout.SubagentLogsDir, sessionID, runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	path := ""
	switch kind {
	case "trace":
		path = filepath.Join(runDir, "trace.jsonl")
	case "findings":
		path = filepath.Join(runDir, "FINDINGS.md")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported artifact kind"})
		return
	}
	path = filepath.Clean(path)

	tail := strings.TrimSpace(c.Query("tail"))
	useTail := tail == "1" || strings.EqualFold(tail, "true")
	var content string
	var truncated bool
	var readErr error
	if useTail {
		content, truncated, readErr = readFileTailLimited(path, maxArtifactBytes)
	} else {
		content, truncated, readErr = readFileLimited(path, maxArtifactBytes)
	}
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, readErr)
		return
	}

	c.JSON(http.StatusOK, artifactContentResponse{
		Path:      path,
		Content:   content,
		Truncated: truncated,
	})
}

func findSubagentRunDir(baseDir, sessionID, runID string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	sessionID = strings.TrimSpace(sessionID)
	runID = strings.TrimSpace(runID)
	if baseDir == "" {
		return "", errors.New("baseDir is required")
	}
	if sessionID == "" {
		return "", errors.New("sessionID is required")
	}
	if runID == "" {
		return "", errors.New("runID is required")
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dayDir := filepath.Join(baseDir, e.Name())
		candidate := filepath.Join(dayDir, sessionID, runID)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

