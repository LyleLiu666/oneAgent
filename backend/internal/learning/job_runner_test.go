package learning

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type fakeEvaluator struct {
	err error
}

func (f fakeEvaluator) Evaluate(ctx context.Context, principalID string, sug workledger.Suggestion) (CompressibilityResult, error) {
	if f.err != nil {
		return CompressibilityResult{}, f.err
	}
	return CompressibilityResult{
		CompressionPrompt: "simple prompt",
		Verdict:           CompressibilityVerdictNotEquivalent,
		Reason:            "needs SOP",
	}, nil
}

func TestRunDailyJobWithEvaluator_EvaluatorErrorsDoNotFailJob(t *testing.T) {
	base := t.TempDir()
	store, err := workledger.NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()

	// Seed enough usable receipts to create suggestions.
	for i := 0; i < 2; i++ {
		_, err := store.CreateReceipt(workledger.CreateReceiptInput{
			PrincipalID: "local",
			Kind:        workledger.ReceiptKindSubagentRun,
			Status:      workledger.ReceiptStatusSucceeded,
			FinishedAt:  now.Add(-time.Duration(i) * time.Minute),
			Summary:     "usable",
			Artifacts: workledger.ReceiptArtifacts{
				FindingsPath: "f.md",
				TraceLogPath: "t.jsonl",
			},
		})
		if err != nil {
			t.Fatalf("CreateReceipt(%d): %v", i, err)
		}
	}

	job, err := RunDailyJobWithEvaluator(context.Background(), store, "local", now, fakeEvaluator{err: errors.New("boom")})
	if err != nil {
		// The job itself still succeeds; caller should not treat this as fatal.
		t.Fatalf("RunDailyJobWithEvaluator returned error: %v", err)
	}
	if job.Status != workledger.LearningJobStatusSucceeded {
		t.Fatalf("expected succeeded, got %q (job=%+v)", job.Status, job)
	}
	if job.Stats.SuggestionsCreated == 0 {
		t.Fatalf("expected suggestions created stats to be set")
	}
	if len(job.Warnings) == 0 {
		t.Fatalf("expected warnings when evaluator errors")
	}
}

func TestRunDailyJobWithEvaluator_IdempotentAfterSuccess(t *testing.T) {
	base := t.TempDir()
	store, err := workledger.NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		_, err := store.CreateReceipt(workledger.CreateReceiptInput{
			PrincipalID: "local",
			Kind:        workledger.ReceiptKindSubagentRun,
			Status:      workledger.ReceiptStatusSucceeded,
			FinishedAt:  now.Add(-time.Duration(i) * time.Minute),
			Summary:     "usable",
			Artifacts: workledger.ReceiptArtifacts{
				FindingsPath: "f.md",
				TraceLogPath: "t.jsonl",
			},
		})
		if err != nil {
			t.Fatalf("CreateReceipt(%d): %v", i, err)
		}
	}

	j1, err := RunDailyJobWithEvaluator(context.Background(), store, "local", now, fakeEvaluator{})
	if err != nil {
		t.Fatalf("RunDailyJobWithEvaluator(1): %v", err)
	}
	if j1.Status != workledger.LearningJobStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", j1.Status)
	}

	j2, err := RunDailyJobWithEvaluator(context.Background(), store, "local", now, fakeEvaluator{err: errors.New("should not be called")})
	if err != nil {
		t.Fatalf("RunDailyJobWithEvaluator(2): %v", err)
	}
	if j2.Status != workledger.LearningJobStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", j2.Status)
	}
	if j2.JobID != j1.JobID {
		t.Fatalf("expected same job_id, got %q vs %q", j2.JobID, j1.JobID)
	}
}
