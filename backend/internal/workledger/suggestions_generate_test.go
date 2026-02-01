package workledger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStore_GenerateSuggestionsV1_EvidenceGating(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()

	// Not usable: failed even with artifacts.
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusFailed,
		FinishedAt:  now.Add(-10 * time.Minute),
		Summary:     "failed receipt",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f.md",
			TraceLogPath: "t.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(failed): %v", err)
	}

	// Not usable: succeeded but missing artifacts.
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		FinishedAt:  now.Add(-9 * time.Minute),
		Summary:     "missing artifacts",
		Artifacts:   ReceiptArtifacts{},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(missing artifacts): %v", err)
	}

	// Usable #1.
	r1, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		FinishedAt:  now.Add(-2 * time.Minute),
		Summary:     "usable 1",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f1.md",
			TraceLogPath: "t1.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r1): %v", err)
	}

	// Usable #2.
	r2, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		FinishedAt:  now.Add(-1 * time.Minute),
		Summary:     "usable 2",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f2.md",
			TraceLogPath: "t2.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r2): %v", err)
	}

	dayKey := DayKey(now)
	created, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       dayKey,
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 created suggestion, got %d", len(created))
	}
	if created[0].Status != SuggestionStatusProposed {
		t.Fatalf("expected proposed, got %q", created[0].Status)
	}
	if created[0].EvidenceCount != 2 {
		t.Fatalf("expected 2 evidence, got %d", created[0].EvidenceCount)
	}

	ids := map[string]bool{}
	for _, id := range created[0].EvidenceReceiptIDs {
		ids[id] = true
	}
	if !ids[r1.ReceiptID] || !ids[r2.ReceiptID] {
		t.Fatalf("expected evidence to be the two usable receipts, got %+v", created[0].EvidenceReceiptIDs)
	}
}

func TestStore_GenerateSuggestionsV1_DedupWithinDay(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		_, err := store.CreateReceipt(CreateReceiptInput{
			PrincipalID: "local",
			Kind:        ReceiptKindSubagentRun,
			Status:      ReceiptStatusSucceeded,
			FinishedAt:  now.Add(-time.Duration(i) * time.Minute),
			Summary:     "usable",
			Artifacts: ReceiptArtifacts{
				FindingsPath: "f.md",
				TraceLogPath: "t.jsonl",
			},
		})
		if err != nil {
			t.Fatalf("CreateReceipt(%d): %v", i, err)
		}
	}

	dayKey := DayKey(now)

	created1, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       dayKey,
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1(1): %v", err)
	}
	if len(created1) != 1 {
		t.Fatalf("expected 1 created suggestion, got %d", len(created1))
	}

	created2, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       dayKey,
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1(2): %v", err)
	}
	if len(created2) != 0 {
		t.Fatalf("expected 0 new suggestions due to dedupe, got %d", len(created2))
	}
}

func TestStore_GenerateSuggestionsV1_DefaultLookbackIsOneDay(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		StartedAt:   now.Add(-2*24*time.Hour - 2*time.Minute),
		FinishedAt:  now.Add(-2 * 24 * time.Hour),
		Summary:     "usable",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f-old.md",
			TraceLogPath: "t-old.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(old): %v", err)
	}
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		StartedAt:   now.Add(-1*time.Hour - 2*time.Minute),
		FinishedAt:  now.Add(-1 * time.Hour),
		Summary:     "usable",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f-new.md",
			TraceLogPath: "t-new.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(new): %v", err)
	}

	created, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID: "local",
		DayKey:      DayKey(now),
		// LookbackDays omitted: should default to the daily cadence (1 day)
		Count: 1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected 0 suggestions with default 1-day lookback, got %d", len(created))
	}
}

func TestStore_GenerateSuggestionsV1_AttachesGovernanceHints(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()
	dayKey := DayKey(now)

	// Seed an existing suggestion that should be similar to the generated one.
	existing, err := store.CreateSuggestion(CreateSuggestionInput{
		PrincipalID:        "local",
		WorkspaceRoot:      "",
		Title:              "SOP: usable 1",
		Description:        "seed",
		EvidenceReceiptIDs: []string{"r-old-1", "r-old-2"},
		DraftSkill:         "This SOP is about usable 1 receipts and review.",
		Scores:             SuggestionScores{TotalScore: 0.9},
		Meta:               SuggestionMeta{DayKey: "2026-01-01"},
	})
	if err != nil {
		t.Fatalf("CreateSuggestion(existing): %v", err)
	}

	// Create 2 usable receipts that will generate a new suggestion titled "SOP: usable 1".
	for i := 0; i < 2; i++ {
		_, err := store.CreateReceipt(CreateReceiptInput{
			PrincipalID: "local",
			Kind:        ReceiptKindSubagentRun,
			Status:      ReceiptStatusSucceeded,
			FinishedAt:  now.Add(-time.Duration(i) * time.Minute),
			Summary:     "usable 1",
			Artifacts: ReceiptArtifacts{
				FindingsPath: "f.md",
				TraceLogPath: "t.jsonl",
			},
		})
		if err != nil {
			t.Fatalf("CreateReceipt(%d): %v", i, err)
		}
	}

	created, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       dayKey,
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 created suggestion, got %d", len(created))
	}

	got, err := store.GetSuggestion(created[0].SuggestionID)
	if err != nil {
		t.Fatalf("GetSuggestion: %v", err)
	}
	if len(got.Meta.SimilarSuggestionIDs) == 0 {
		t.Fatalf("expected similar_suggestion_ids to be populated")
	}
	found := false
	for _, id := range got.Meta.SimilarSuggestionIDs {
		if id == existing.SuggestionID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected similar_suggestion_ids to include %q, got %+v", existing.SuggestionID, got.Meta.SimilarSuggestionIDs)
	}
}

func TestStore_GenerateSuggestionsV1_FillsDraftFromFindings(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	findings1 := filepath.Join(base, "findings1.md")
	if err := os.WriteFile(findings1, []byte("# FINDINGS\n\n## 流水账\n- step A unique\n\n## Findings\n- pitfall A\n"), 0o600); err != nil {
		t.Fatalf("write findings1: %v", err)
	}
	findings2 := filepath.Join(base, "findings2.md")
	if err := os.WriteFile(findings2, []byte("# FINDINGS\n\n## 流水账\n- step B unique\n"), 0o600); err != nil {
		t.Fatalf("write findings2: %v", err)
	}

	now := time.Now().UTC()
	for i, fp := range []string{findings1, findings2} {
		_, err := store.CreateReceipt(CreateReceiptInput{
			PrincipalID: "local",
			Kind:        ReceiptKindSubagentRun,
			Status:      ReceiptStatusSucceeded,
			FinishedAt:  now.Add(-time.Duration(2-i) * time.Minute),
			Summary:     "auth flow tests",
			Artifacts: ReceiptArtifacts{
				FindingsPath: fp,
				TraceLogPath: "t.jsonl",
			},
		})
		if err != nil {
			t.Fatalf("CreateReceipt(%d): %v", i, err)
		}
	}

	dayKey := DayKey(now)
	created, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       dayKey,
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 created suggestion, got %d", len(created))
	}
	draft := created[0].DraftSkill
	if !strings.Contains(draft, "step A unique") {
		t.Fatalf("expected draft to include extracted timeline step A, got:\n%s", draft)
	}
	if !strings.Contains(draft, "step B unique") {
		t.Fatalf("expected draft to include extracted timeline step B, got:\n%s", draft)
	}
}

func TestStore_GenerateSuggestionsV1_SkipsDissimilarReceipts(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		FinishedAt:  now.Add(-2 * time.Minute),
		Summary:     "Refactor auth flow",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f1.md",
			TraceLogPath: "t1.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r1): %v", err)
	}
	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID: "local",
		Kind:        ReceiptKindSubagentRun,
		Status:      ReceiptStatusSucceeded,
		FinishedAt:  now.Add(-1 * time.Minute),
		Summary:     "Write brand plan budget",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "f2.md",
			TraceLogPath: "t2.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r2): %v", err)
	}

	created, err := store.GenerateSuggestionsV1(context.Background(), GenerateSuggestionsInput{
		PrincipalID:  "local",
		DayKey:       DayKey(now),
		LookbackDays: 1,
		Count:        1,
	})
	if err != nil {
		t.Fatalf("GenerateSuggestionsV1: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected 0 suggestions for dissimilar receipts, got %d", len(created))
	}
}
