package handler

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type similarSuggestion struct {
	SuggestionID string                      `json:"suggestion_id"`
	Title        string                      `json:"title"`
	Status       workledger.SuggestionStatus `json:"status"`
	Similarity   float64                     `json:"similarity"`
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
		RespondError(c, http.StatusInternalServerError, err)
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
		RespondError(c, http.StatusInternalServerError, err)
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

func toTokenSet(s string) map[string]struct{} {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	parts := strings.Fields(b.String())
	out := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		out[p] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
