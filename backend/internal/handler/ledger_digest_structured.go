package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

func GetStructuredDigestToday(c *gin.Context) {
	GetStructuredDigestByParam(c, time.Now().In(time.Local).Format("2006-01-02"))
}

func GetStructuredDigest(c *gin.Context) {
	GetStructuredDigestByParam(c, c.Param("day"))
}

func GetStructuredDigestByParam(c *gin.Context, dayKey string) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	dayKey = strings.TrimSpace(dayKey)
	if _, err := time.ParseInLocation("2006-01-02", dayKey, time.Local); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid day"})
		return
	}

	d, err := rt.WorkLedger.GetStructuredDigest(principal, dayKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "digest not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, d)
}

