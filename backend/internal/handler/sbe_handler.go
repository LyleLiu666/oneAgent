package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/liu_y/oneAgent/backend/internal/sbe"
)

type smartEditRequest struct {
	Command string `json:"command" binding:"required"`
}

type smartEditResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// HandleSmartEdit processes smart edit commands
func HandleSmartEdit(c *gin.Context) {
	var req smartEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}

	err := sbe.ApplyEdits(req.Command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, smartEditResponse{
		Success: true,
		Message: "Smart edit applied successfully",
	})
}
