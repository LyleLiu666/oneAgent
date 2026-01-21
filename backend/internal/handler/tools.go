package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

// ListTools returns available tool metadata.
func ListTools(c *gin.Context) {
	c.JSON(http.StatusOK, tool.Infos())
}
