package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func GetLedgerStatusToday(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	dayKey := workledger.DayKey(time.Now())

	digestExists, err := rt.WorkLedger.DigestExists(principal, dayKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	learningStatus := "none"
	if job, err := rt.WorkLedger.GetLearningJob(principal, dayKey); err == nil {
		learningStatus = string(job.Status)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	proposedCount, err := rt.WorkLedger.CountSuggestions(workledger.CountSuggestionsQuery{
		PrincipalID: principal,
		Status:      workledger.SuggestionStatusProposed,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"day_key":            dayKey,
		"digest_exists":      digestExists,
		"learning_job_status": learningStatus,
		"sop_proposed_count": proposedCount,
	})
}

