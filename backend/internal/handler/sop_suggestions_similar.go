package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type similarSuggestion struct {
	SuggestionID string                    `json:"suggestion_id"`
	Title        string                    `json:"title"`
	Status       workledger.SuggestionStatus `json:"status"`
	Similarity   float64                   `json:"similarity"`
}

func GetSimilarSuggestions(c *gin.Context) {
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
	current, err := rt.WorkLedger.GetSuggestion(id)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "suggestion not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(current.PrincipalID) != principal {
		c.JSON(http.StatusNotFound, gin.H{"error": "suggestion not found"})
		return
	}

	limit := 5
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 20 {
		limit = 20
	}

	targetTokens := toTokenSet(strings.TrimSpace(current.Title) + " " + strings.TrimSpace(current.DraftSkill))
	if len(targetTokens) == 0 {
		c.JSON(http.StatusOK, []similarSuggestion{})
		return
	}

	// Best-effort: scan across days (DayKey="") and include parked.
	all, err := rt.WorkLedger.ListSuggestions(workledger.ListSuggestionsQuery{
		PrincipalID:   principal,
		IncludeParked: true,
		Limit:         200,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]similarSuggestion, 0, limit)
	for _, s := range all {
		if s.SuggestionID == current.SuggestionID {
			continue
		}
		candTokens := toTokenSet(strings.TrimSpace(s.Title) + " " + strings.TrimSpace(s.DraftSkill))
		sim := jaccard(targetTokens, candTokens)
		if sim <= 0 {
			continue
		}
		out = append(out, similarSuggestion{
			SuggestionID: s.SuggestionID,
			Title:        s.Title,
			Status:       s.Status,
			Similarity:   sim,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Similarity != out[j].Similarity {
			return out[i].Similarity > out[j].Similarity
		}
		return out[i].SuggestionID > out[j].SuggestionID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	c.JSON(http.StatusOK, out)
}
