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
	"github.com/liu_y/oneAgent/backend/internal/sopskill"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type createSuggestionRequest struct {
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	RiskNotes     string   `json:"risk_notes,omitempty"`
	DraftSkill    string   `json:"draft_skill"`
	EvidenceIDs   []string `json:"evidence_receipt_ids"`

	DayKey string `json:"day_key,omitempty"`
}

func CreateSuggestion(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req createSuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	dayKey := strings.TrimSpace(req.DayKey)
	if dayKey != "" {
		if _, err := time.ParseInLocation("2006-01-02", dayKey, time.Local); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid day_key"})
			return
		}
	}

	sug, err := rt.WorkLedger.CreateSuggestion(workledger.CreateSuggestionInput{
		PrincipalID:        principal,
		WorkspaceRoot:      strings.TrimSpace(req.WorkspaceRoot),
		Title:              strings.TrimSpace(req.Title),
		Description:        strings.TrimSpace(req.Description),
		RiskNotes:          strings.TrimSpace(req.RiskNotes),
		EvidenceReceiptIDs: req.EvidenceIDs,
		DraftSkill:         strings.TrimSpace(req.DraftSkill),
		Scores:             workledger.ComputeSuggestionScores(rt.WorkLedger, principal, dayKey, req.Title, req.DraftSkill, req.EvidenceIDs),
		Meta:               workledger.SuggestionMeta{DayKey: dayKey},
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	_ = rt.WorkLedger.ApplyInboxCap(principal, sug.Meta.DayKey, 10)
	c.JSON(http.StatusOK, sug)
}

func ListSuggestions(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	dayKey := strings.TrimSpace(c.Query("day"))
	status := strings.TrimSpace(c.Query("status"))
	includeParked := strings.TrimSpace(c.Query("include_parked")) == "1"
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	list, err := rt.WorkLedger.ListSuggestions(workledger.ListSuggestionsQuery{
		PrincipalID:   principal,
		DayKey:        dayKey,
		Status:        workledger.SuggestionStatus(status),
		IncludeParked: includeParked,
		Limit:         limit,
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func GetSuggestion(c *gin.Context) {
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
	sug, err := rt.WorkLedger.GetSuggestion(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "suggestion not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(sug.PrincipalID) != principal {
		c.JSON(http.StatusNotFound, gin.H{"error": "suggestion not found"})
		return
	}
	c.JSON(http.StatusOK, sug)
}

type updateSuggestionRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	RiskNotes   *string `json:"risk_notes,omitempty"`
	DraftSkill  *string `json:"draft_skill,omitempty"`
}

func UpdateSuggestion(c *gin.Context) {
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
		if errors.Is(err, os.ErrNotExist) {
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

	switch current.Status {
	case workledger.SuggestionStatusProposed, workledger.SuggestionStatusParked:
		// ok
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "only proposed/parked suggestions can be edited"})
		return
	}

	var req updateSuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Title == nil && req.Description == nil && req.RiskNotes == nil && req.DraftSkill == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	updated, err := rt.WorkLedger.UpdateSuggestion(id, func(sug *workledger.Suggestion) error {
		if req.Title != nil {
			v := strings.TrimSpace(*req.Title)
			if v == "" {
				return errors.New("title cannot be empty")
			}
			sug.Title = v
		}
		if req.Description != nil {
			sug.Description = strings.TrimSpace(*req.Description)
		}
		if req.RiskNotes != nil {
			sug.RiskNotes = strings.TrimSpace(*req.RiskNotes)
		}
		if req.DraftSkill != nil {
			v := strings.TrimSpace(*req.DraftSkill)
			if v == "" {
				return errors.New("draft_skill cannot be empty")
			}
			sug.DraftSkill = v
		}

		dayKey := strings.TrimSpace(sug.Meta.DayKey)
		sug.Scores = workledger.ComputeSuggestionScores(rt.WorkLedger, principal, dayKey, sug.Title, sug.DraftSkill, sug.EvidenceReceiptIDs)
		return nil
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	_ = rt.WorkLedger.ApplyInboxCap(principal, updated.Meta.DayKey, 10)
	c.JSON(http.StatusOK, updated)
}

type updateStatusRequest struct {
	Status       string `json:"status"`
	MergedIntoID string `json:"merged_into_suggestion_id,omitempty"`
}

func UpdateSuggestionStatus(c *gin.Context) {
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
		if errors.Is(err, os.ErrNotExist) {
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

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	status := workledger.SuggestionStatus(strings.TrimSpace(req.Status))
	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	if status == workledger.SuggestionStatusMerged {
		intoID := strings.TrimSpace(req.MergedIntoID)
		if intoID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "merged_into_suggestion_id is required for merged"})
			return
		}
		merged, _, err := rt.WorkLedger.MergeSuggestions(id, intoID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, merged)
		return
	}

	if status == workledger.SuggestionStatusApproved {
		// Approve is strong-consistent: materialize first, then update status.
		if strings.TrimSpace(current.Meta.MaterializedSkillID) == "" || strings.TrimSpace(current.Meta.MaterializedSkillPath) == "" {
			m, err := sopskill.MaterializeSuggestion(rt.Config.Home, current)
			if err != nil {
				RespondError(c, http.StatusBadRequest, err)
				return
			}
			updated, err := rt.WorkLedger.UpdateSuggestion(id, func(sug *workledger.Suggestion) error {
				sug.Status = workledger.SuggestionStatusApproved
				sug.MergedIntoSuggestionID = ""
				sug.Meta.MaterializedSkillID = m.SkillID
				sug.Meta.MaterializedSkillPath = m.SkillPath
				return nil
			})
			if err != nil {
				RespondError(c, http.StatusBadRequest, err)
				return
			}
			c.JSON(http.StatusOK, updated)
			return
		}
		updated, err := rt.WorkLedger.UpdateSuggestion(id, func(sug *workledger.Suggestion) error {
			sug.Status = workledger.SuggestionStatusApproved
			sug.MergedIntoSuggestionID = ""
			return nil
		})
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, updated)
		return
	}

	updated, err := rt.WorkLedger.UpdateSuggestionStatus(id, status, "")
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

type loadMoreRequest struct {
	DayKey string `json:"day_key,omitempty"`
	Count  int    `json:"count,omitempty"`
}

func LoadMoreSuggestions(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req loadMoreRequest
	_ = c.ShouldBindJSON(&req) // optional

	dayKey := strings.TrimSpace(req.DayKey)
	if dayKey == "" {
		dayKey = workledger.DayKey(time.Now())
	}

	list, err := rt.WorkLedger.LoadMoreParked(principal, dayKey, req.Count)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, list)
}
