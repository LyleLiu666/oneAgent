package handler

import (
	"errors"
	"net/http"
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
