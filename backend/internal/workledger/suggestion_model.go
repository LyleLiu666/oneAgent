package workledger

import "time"

type SuggestionStatus string

const (
	SuggestionStatusProposed   SuggestionStatus = "proposed"
	SuggestionStatusParked     SuggestionStatus = "parked"
	SuggestionStatusApproved   SuggestionStatus = "approved"
	SuggestionStatusRejected   SuggestionStatus = "rejected"
	SuggestionStatusMerged     SuggestionStatus = "merged"
	SuggestionStatusDeprecated SuggestionStatus = "deprecated"
)

type SuggestionScores struct {
	ScarcityScore float64 `json:"scarcity_score"`
	DepthScore    float64 `json:"depth_score"`
	EvidenceScore float64 `json:"evidence_score"`
	TotalScore    float64 `json:"total_score"`
}

type SuggestionMeta struct {
	DayKey string `json:"day_key,omitempty"`

	CompressionPrompt string   `json:"compression_prompt,omitempty"`
	SimilarSkillIDs   []string `json:"similar_skill_ids,omitempty"`
	DeltaVsTop1       string   `json:"delta_vs_top1,omitempty"`
}

type Suggestion struct {
	SuggestionID string `json:"suggestion_id"`

	PrincipalID   string `json:"principal_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	RiskNotes   string `json:"risk_notes,omitempty"`

	Status SuggestionStatus `json:"status"`

	EvidenceReceiptIDs []string `json:"evidence_receipt_ids"`
	EvidenceCount      int      `json:"evidence_count"`

	DraftSkill string `json:"draft_skill"`

	// Governance links.
	MergedIntoSuggestionID string `json:"merged_into_suggestion_id,omitempty"`

	Scores SuggestionScores `json:"scores,omitempty"`
	Meta   SuggestionMeta   `json:"meta,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

