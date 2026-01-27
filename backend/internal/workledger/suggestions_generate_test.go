package workledger

import (
	"context"
	"path/filepath"
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
