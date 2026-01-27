package workledger

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_Suggestions_InboxCapAndLoadMore(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	dayKey := DayKey(time.Now())

	// Create 12 suggestions with increasing total score.
	for i := 0; i < 12; i++ {
		_, err := store.CreateSuggestion(CreateSuggestionInput{
			PrincipalID: "local",
			Title:       "S" + string(rune('A'+i)),
			DraftSkill:  "# Skill\n\ncontent",
			EvidenceReceiptIDs: []string{
				"r1", "r2",
			},
			Scores: SuggestionScores{
				TotalScore: float64(i),
			},
			Meta: SuggestionMeta{DayKey: dayKey},
		})
		if err != nil {
			t.Fatalf("CreateSuggestion: %v", err)
		}
	}

	// Apply cap=10; lowest 2 should be parked.
	if err := store.ApplyInboxCap("local", dayKey, 10); err != nil {
		t.Fatalf("ApplyInboxCap: %v", err)
	}

	inbox, err := store.ListSuggestions(ListSuggestionsQuery{
		PrincipalID: "local",
		DayKey:      dayKey,
		Status:      SuggestionStatusProposed,
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("ListSuggestions(inbox): %v", err)
	}
	if len(inbox) != 10 {
		t.Fatalf("expected 10 proposed, got %d", len(inbox))
	}

	parked, err := store.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:   "local",
		DayKey:        dayKey,
		Status:        SuggestionStatusParked,
		IncludeParked: true,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListSuggestions(parked): %v", err)
	}
	if len(parked) != 2 {
		t.Fatalf("expected 2 parked, got %d", len(parked))
	}

	// Load more one parked back into inbox; still capped at 10.
	updated, err := store.LoadMoreParked("local", dayKey, 1)
	if err != nil {
		t.Fatalf("LoadMoreParked: %v", err)
	}
	if len(updated) != 10 {
		t.Fatalf("expected 10 proposed after load more, got %d", len(updated))
	}
}

func TestStore_Suggestions_Merge(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	dayKey := DayKey(time.Now())

	s1, err := store.CreateSuggestion(CreateSuggestionInput{
		PrincipalID:        "local",
		Title:              "S1",
		DraftSkill:         "# Skill\n\ncontent",
		EvidenceReceiptIDs: []string{"r1", "r2"},
		Scores:             SuggestionScores{TotalScore: 0.9},
		Meta:              SuggestionMeta{DayKey: dayKey},
	})
	if err != nil {
		t.Fatalf("CreateSuggestion(s1): %v", err)
	}
	s2, err := store.CreateSuggestion(CreateSuggestionInput{
		PrincipalID:        "local",
		Title:              "S2",
		DraftSkill:         "# Skill\n\ncontent2",
		EvidenceReceiptIDs: []string{"r2", "r3"},
		Scores:             SuggestionScores{TotalScore: 0.8},
		Meta:              SuggestionMeta{DayKey: dayKey},
	})
	if err != nil {
		t.Fatalf("CreateSuggestion(s2): %v", err)
	}

	merged, canonical, err := store.MergeSuggestions(s2.SuggestionID, s1.SuggestionID)
	if err != nil {
		t.Fatalf("MergeSuggestions: %v", err)
	}
	if merged.Status != SuggestionStatusMerged {
		t.Fatalf("expected merged status, got %q", merged.Status)
	}
	if merged.MergedIntoSuggestionID != s1.SuggestionID {
		t.Fatalf("expected merged_into=%q, got %q", s1.SuggestionID, merged.MergedIntoSuggestionID)
	}
	if canonical.SuggestionID != s1.SuggestionID {
		t.Fatalf("expected canonical id %q, got %q", s1.SuggestionID, canonical.SuggestionID)
	}
	if canonical.EvidenceCount != 3 {
		t.Fatalf("expected canonical evidence_count=3, got %d", canonical.EvidenceCount)
	}
}
