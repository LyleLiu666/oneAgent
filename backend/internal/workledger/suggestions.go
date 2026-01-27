package workledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CreateSuggestionInput struct {
	PrincipalID   string
	WorkspaceRoot string

	Title       string
	Description string
	RiskNotes   string

	EvidenceReceiptIDs []string

	DraftSkill string

	Scores SuggestionScores
	Meta   SuggestionMeta
}

func (s *Store) suggestionDir(id string) string {
	return filepath.Join(s.SuggestionsDir(), id)
}

func (s *Store) suggestionJSONPath(id string) string {
	return filepath.Join(s.suggestionDir(id), "suggestion.json")
}

func (s *Store) CreateSuggestion(in CreateSuggestionInput) (Suggestion, error) {
	if s == nil {
		return Suggestion{}, errors.New("store is nil")
	}
	if strings.TrimSpace(in.PrincipalID) == "" {
		return Suggestion{}, errors.New("principal_id is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return Suggestion{}, errors.New("title is required")
	}
	if strings.TrimSpace(in.DraftSkill) == "" {
		return Suggestion{}, errors.New("draft_skill is required")
	}
	if len(in.EvidenceReceiptIDs) < 1 {
		return Suggestion{}, errors.New("evidence_receipt_ids is required")
	}

	now := time.Now().UTC()
	id := uuid.NewString()
	dayKey := strings.TrimSpace(in.Meta.DayKey)
	if dayKey == "" {
		dayKey = DayKey(now)
	}

	sug := Suggestion{
		SuggestionID: id,
		PrincipalID:  strings.TrimSpace(in.PrincipalID),

		WorkspaceRoot: strings.TrimSpace(in.WorkspaceRoot),
		Title:         strings.TrimSpace(in.Title),
		Description:   strings.TrimSpace(in.Description),
		RiskNotes:     strings.TrimSpace(in.RiskNotes),

		Status: SuggestionStatusProposed,

		EvidenceReceiptIDs: dedupStrings(in.EvidenceReceiptIDs),
		DraftSkill:         strings.TrimSpace(in.DraftSkill),

		Scores: in.Scores,
		Meta: SuggestionMeta{
			DayKey:             dayKey,
			CompressionPrompt:  strings.TrimSpace(in.Meta.CompressionPrompt),
			SimilarSkillIDs:    dedupStrings(in.Meta.SimilarSkillIDs),
			DeltaVsTop1:        strings.TrimSpace(in.Meta.DeltaVsTop1),
		},

		CreatedAt: now,
		UpdatedAt: now,
	}
	sug.EvidenceCount = len(sug.EvidenceReceiptIDs)

	mu := s.suggestionLock(id)
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(sugDirRoot(sug, s.SuggestionsDir()), 0o700); err != nil {
		return Suggestion{}, fmt.Errorf("create suggestions dir: %w", err)
	}
	if err := os.MkdirAll(s.suggestionDir(id), 0o700); err != nil {
		return Suggestion{}, fmt.Errorf("create suggestion dir: %w", err)
	}
	if err := writeJSONAtomic(s.suggestionJSONPath(id), sug, 0o600); err != nil {
		return Suggestion{}, err
	}
	return sug, nil
}

func sugDirRoot(_ Suggestion, root string) string {
	return root
}

func (s *Store) GetSuggestion(id string) (Suggestion, error) {
	if s == nil {
		return Suggestion{}, errors.New("store is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Suggestion{}, errors.New("suggestion_id is required")
	}

	mu := s.suggestionLock(id)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(s.suggestionJSONPath(id))
	if err != nil {
		return Suggestion{}, err
	}
	var sug Suggestion
	if err := json.Unmarshal(data, &sug); err != nil {
		return Suggestion{}, fmt.Errorf("decode suggestion: %w", err)
	}
	return sug, nil
}

type ListSuggestionsQuery struct {
	PrincipalID string
	DayKey      string
	Status      SuggestionStatus

	IncludeParked bool
	Limit         int
}

func (s *Store) ListSuggestions(q ListSuggestionsQuery) ([]Suggestion, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	if strings.TrimSpace(q.PrincipalID) == "" {
		return nil, errors.New("principal_id is required")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	root := s.SuggestionsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Suggestion{}, nil
		}
		return nil, err
	}

	dayKey := strings.TrimSpace(q.DayKey)
	status := strings.TrimSpace(string(q.Status))

	out := make([]Suggestion, 0, minInt(limit, len(entries)))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		data, err := os.ReadFile(s.suggestionJSONPath(id))
		if err != nil {
			continue
		}
		var sug Suggestion
		if err := json.Unmarshal(data, &sug); err != nil {
			continue
		}
		if strings.TrimSpace(sug.PrincipalID) != strings.TrimSpace(q.PrincipalID) {
			continue
		}
		if dayKey != "" && strings.TrimSpace(sug.Meta.DayKey) != dayKey {
			continue
		}
		if !q.IncludeParked && sug.Status == SuggestionStatusParked {
			continue
		}
		if status != "" && strings.TrimSpace(string(sug.Status)) != status {
			continue
		}

		out = append(out, sug)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Scores.TotalScore != out[j].Scores.TotalScore {
			return out[i].Scores.TotalScore > out[j].Scores.TotalScore
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) UpdateSuggestionStatus(id string, status SuggestionStatus, mergedInto string) (Suggestion, error) {
	if s == nil {
		return Suggestion{}, errors.New("store is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Suggestion{}, errors.New("suggestion_id is required")
	}
	status = SuggestionStatus(strings.TrimSpace(string(status)))
	if strings.TrimSpace(string(status)) == "" {
		return Suggestion{}, errors.New("status is required")
	}
	if !isValidSuggestionStatus(status) {
		return Suggestion{}, errors.New("invalid status")
	}
	if status == SuggestionStatusMerged {
		return Suggestion{}, errors.New("use MergeSuggestions for merged status")
	}

	mu := s.suggestionLock(id)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(s.suggestionJSONPath(id))
	if err != nil {
		return Suggestion{}, err
	}
	var sug Suggestion
	if err := json.Unmarshal(data, &sug); err != nil {
		return Suggestion{}, fmt.Errorf("decode suggestion: %w", err)
	}

	sug.Status = status
	sug.MergedIntoSuggestionID = ""
	sug.UpdatedAt = time.Now().UTC()

	if err := writeJSONAtomic(s.suggestionJSONPath(id), sug, 0o600); err != nil {
		return Suggestion{}, err
	}
	return sug, nil
}

func (s *Store) MergeSuggestions(fromID, intoID string) (Suggestion, Suggestion, error) {
	if s == nil {
		return Suggestion{}, Suggestion{}, errors.New("store is nil")
	}
	fromID = strings.TrimSpace(fromID)
	intoID = strings.TrimSpace(intoID)
	if fromID == "" || intoID == "" {
		return Suggestion{}, Suggestion{}, errors.New("from_id and into_id are required")
	}
	if fromID == intoID {
		return Suggestion{}, Suggestion{}, errors.New("cannot merge into itself")
	}

	first, second := fromID, intoID
	if second < first {
		first, second = second, first
	}

	m1 := s.suggestionLock(first)
	m2 := s.suggestionLock(second)
	m1.Lock()
	m2.Lock()
	defer m2.Unlock()
	defer m1.Unlock()

	from, err := s.readSuggestionLocked(fromID)
	if err != nil {
		return Suggestion{}, Suggestion{}, err
	}
	into, err := s.readSuggestionLocked(intoID)
	if err != nil {
		return Suggestion{}, Suggestion{}, err
	}
	if strings.TrimSpace(from.PrincipalID) != strings.TrimSpace(into.PrincipalID) {
		return Suggestion{}, Suggestion{}, errors.New("principal_id mismatch")
	}

	now := time.Now().UTC()

	from.Status = SuggestionStatusMerged
	from.MergedIntoSuggestionID = intoID
	from.UpdatedAt = now

	into.EvidenceReceiptIDs = dedupStrings(append(into.EvidenceReceiptIDs, from.EvidenceReceiptIDs...))
	into.EvidenceCount = len(into.EvidenceReceiptIDs)
	into.UpdatedAt = now

	if err := writeJSONAtomic(s.suggestionJSONPath(fromID), from, 0o600); err != nil {
		return Suggestion{}, Suggestion{}, err
	}
	if err := writeJSONAtomic(s.suggestionJSONPath(intoID), into, 0o600); err != nil {
		return Suggestion{}, Suggestion{}, err
	}
	return from, into, nil
}

func (s *Store) readSuggestionLocked(id string) (Suggestion, error) {
	data, err := os.ReadFile(s.suggestionJSONPath(id))
	if err != nil {
		return Suggestion{}, err
	}
	var sug Suggestion
	if err := json.Unmarshal(data, &sug); err != nil {
		return Suggestion{}, fmt.Errorf("decode suggestion: %w", err)
	}
	return sug, nil
}

func isValidSuggestionStatus(sug SuggestionStatus) bool {
	switch sug {
	case SuggestionStatusProposed,
		SuggestionStatusParked,
		SuggestionStatusApproved,
		SuggestionStatusRejected,
		SuggestionStatusMerged,
		SuggestionStatusDeprecated,
		SuggestionStatusArchived:
		return true
	default:
		return false
	}
}

// LoadMoreParked moves the highest ranked parked suggestions back into the inbox (proposed),
// then reapplies the inbox cap (10) for the day.
func (s *Store) LoadMoreParked(principalID, dayKey string, count int) ([]Suggestion, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, errors.New("principal_id is required")
	}
	dayKey = strings.TrimSpace(dayKey)
	if dayKey == "" {
		dayKey = DayKey(time.Now())
	}
	if count <= 0 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	parked, err := s.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:    principalID,
		DayKey:         dayKey,
		Status:         SuggestionStatusParked,
		IncludeParked:  true,
		Limit:          1000,
	})
	if err != nil {
		return nil, err
	}

	// Move top-ranked parked to proposed.
	moved := 0
	for _, sug := range parked {
		if moved >= count {
			break
		}
		if _, err := s.UpdateSuggestionStatus(sug.SuggestionID, SuggestionStatusProposed, ""); err == nil {
			moved++
		}
	}

	// Reapply cap and return updated inbox (proposed).
	if err := s.ApplyInboxCap(principalID, dayKey, 10); err != nil {
		return nil, err
	}
	return s.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:   principalID,
		DayKey:        dayKey,
		Status:        SuggestionStatusProposed,
		IncludeParked: false,
		Limit:         100,
	})
}

func (s *Store) ApplyInboxCap(principalID, dayKey string, cap int) error {
	if s == nil {
		return errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return errors.New("principal_id is required")
	}
	dayKey = strings.TrimSpace(dayKey)
	if dayKey == "" {
		dayKey = DayKey(time.Now())
	}
	if cap <= 0 {
		cap = 10
	}

	all, err := s.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:   principalID,
		DayKey:        dayKey,
		IncludeParked: true,
		Limit:         2000,
	})
	if err != nil {
		return err
	}

	// Eligible for inbox competition: proposed + parked.
	cands := make([]Suggestion, 0, len(all))
	for _, sug := range all {
		if sug.Status != SuggestionStatusProposed && sug.Status != SuggestionStatusParked {
			continue
		}
		cands = append(cands, sug)
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Scores.TotalScore != cands[j].Scores.TotalScore {
			return cands[i].Scores.TotalScore > cands[j].Scores.TotalScore
		}
		return cands[i].CreatedAt.After(cands[j].CreatedAt)
	})

	for i, sug := range cands {
		want := SuggestionStatusParked
		if i < cap {
			want = SuggestionStatusProposed
		}
		if sug.Status == want {
			continue
		}
		_, _ = s.UpdateSuggestionStatus(sug.SuggestionID, want, "")
	}
	return nil
}

func dedupStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
