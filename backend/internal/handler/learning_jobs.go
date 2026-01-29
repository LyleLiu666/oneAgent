package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/learning"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func GetLearningJobToday(c *gin.Context) {
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
	job, err := rt.WorkLedger.GetLearningJob(principal, dayKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func RunLearningJobToday(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	eval := &learning.LLMCompressibilityEvaluator{Settings: rt.Settings}
	job, err := learning.RunDailyJobWithEvaluator(c.Request.Context(), rt.WorkLedger, principal, time.Now(), eval)
	// Return the job even if it failed, so users can inspect status/evidence.
	if err != nil {
		code, msg, hint := classifyAPIError(http.StatusInternalServerError, err)
		c.JSON(http.StatusOK, gin.H{
			"job":        job,
			"error":      msg,
			"code":       code,
			"request_id": strings.TrimSpace(middleware.GetRequestID(c)),
			"hint":       hint,
		})
		return
	}
	c.JSON(http.StatusOK, job)
}
