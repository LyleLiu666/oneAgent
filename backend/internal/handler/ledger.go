package handler

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func ListReceipts(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	status := strings.TrimSpace(c.Query("status"))
	workspace := strings.TrimSpace(c.Query("workspace"))
	q := strings.TrimSpace(c.Query("q"))

	list, err := rt.WorkLedger.ListReceipts(workledger.ListReceiptsQuery{
		PrincipalID: principal,
		Workspace:   workspace,
		Status:      workledger.ReceiptStatus(status),
		Q:           q,
		Limit:       limit,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}

func GetReceipt(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	id := strings.TrimSpace(c.Param("id"))
	r, err := rt.WorkLedger.GetReceipt(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "receipt not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(r.PrincipalID) != principal {
		c.JSON(http.StatusNotFound, gin.H{"error": "receipt not found"})
		return
	}

	c.JSON(http.StatusOK, r)
}

func GetDigestToday(c *gin.Context) {
	GetDigestByParam(c, workledger.DayKey(time.Now()))
}

func GetDigest(c *gin.Context) {
	GetDigestByParam(c, c.Param("day"))
}

func GetDigestByParam(c *gin.Context, dayKey string) {
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
	day, err := time.ParseInLocation("2006-01-02", dayKey, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid day"})
		return
	}

	refresh := strings.TrimSpace(c.Query("refresh")) == "1"
	d, err := rt.WorkLedger.GetOrCreateDigest(principal, day, refresh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"principal_id": d.PrincipalID,
		"day_key":      d.DayKey,
		"generated_at": d.GeneratedAt,
		"markdown":     d.Markdown,
	})
}
