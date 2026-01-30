package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type staleSkillInfo struct {
	SkillID     string       `json:"skill_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Source      skill.Source `json:"source"`
	Path        string       `json:"path"`
	Archivable  bool         `json:"archivable"`

	UsedCount      int64     `json:"used_count"`
	LastUsedAt     time.Time `json:"last_used_at"`
	LastActivityAt time.Time `json:"last_activity_at"`

	StaleReason        string `json:"stale_reason"`
	RecommendedAction  string `json:"recommended_action"`
	StaleThresholdDays int    `json:"stale_threshold_days"`
}

func ListStaleSkills(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	days := 30
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be a positive integer"})
			return
		}
		days = n
	}

	userID := strings.TrimSpace(middleware.GetUserID(c))
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	db := rt.Settings

	threshold := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	out := make([]staleSkillInfo, 0, 16)
	for _, s := range cat.Skills {
		// We only govern personal skills for now (best-effort).
		if s.Source != skill.SourceOneAgent {
			continue
		}

		usage := settingsdb.SkillUsage{}
		if db != nil {
			u, ok, err := db.GetSkillUsage(c.Request.Context(), userID, s.ID)
			if err == nil && ok {
				usage = u
			}
		}

		var fileActivity time.Time
		if fi, err := os.Stat(s.Path); err == nil {
			fileActivity = fi.ModTime()
		}

		lastUsedAt := usage.LastUsedAt
		lastActivity := lastUsedAt
		if fileActivity.After(lastActivity) {
			lastActivity = fileActivity
		}

		if lastActivity.IsZero() || !lastActivity.Before(threshold) {
			continue
		}

		reason := "inactive"
		if usage.UsedCount <= 0 && !fileActivity.IsZero() {
			reason = "never_used_recently"
		}

		recommended := "archive"
		if usage.UsedCount > 0 {
			recommended = "review_then_archive"
		}

		out = append(out, staleSkillInfo{
			SkillID:            s.ID,
			Name:               s.Name,
			Description:        s.Description,
			Source:             s.Source,
			Path:               s.Path,
			Archivable:         canArchiveSkill(home, s),
			UsedCount:          usage.UsedCount,
			LastUsedAt:         lastUsedAt,
			LastActivityAt:     lastActivity,
			StaleReason:        reason,
			RecommendedAction:  recommended,
			StaleThresholdDays: days,
		})
	}

	c.JSON(http.StatusOK, out)
}

