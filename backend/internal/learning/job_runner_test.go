package learning

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestRunDailyJob_Idempotent(t *testing.T) {
	store, err := workledger.NewStore(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Seed 2 receipts with required evidence pointers.
	_, err = store.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID: "local",
		Kind:        workledger.ReceiptKindSubagentRun,
		Status:      workledger.ReceiptStatusSucceeded,
		Summary:     "A",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: "f1.md",
			TraceLogPath: "t1.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt 1: %v", err)
	}
	_, err = store.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID: "local",
		Kind:        workledger.ReceiptKindSubagentRun,
		Status:      workledger.ReceiptStatusSucceeded,
		Summary:     "B",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: "f2.md",
			TraceLogPath: "t2.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt 2: %v", err)
	}

	now := time.Now()
	job1, err := RunDailyJob(context.Background(), store, "local", now)
	if err != nil {
		t.Fatalf("RunDailyJob 1: %v", err)
	}
	if job1.Status != workledger.LearningJobStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", job1.Status)
	}

	job2, err := RunDailyJob(context.Background(), store, "local", now)
	if err != nil {
		t.Fatalf("RunDailyJob 2: %v", err)
	}
	if job2.Status != workledger.LearningJobStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", job2.Status)
	}

	dayKey := workledger.DayKey(now)
	sugs, err := store.ListSuggestions(workledger.ListSuggestionsQuery{
		PrincipalID:   "local",
		DayKey:        dayKey,
		IncludeParked: true,
		Limit:         200,
	})
	if err != nil {
		t.Fatalf("ListSuggestions: %v", err)
	}
	if len(sugs) != job1.Stats.SuggestionsCreated {
		t.Fatalf("expected %d suggestions, got %d", job1.Stats.SuggestionsCreated, len(sugs))
	}
}
