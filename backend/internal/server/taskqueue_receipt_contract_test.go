package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestTaskQueueRunner_AttemptReceipts_ManifestAndCompleteness_AcrossSamples(t *testing.T) {
	home := t.TempDir()
	layout, err := runtime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("ensure layout: %v", err)
	}

	tasks, err := taskqueue.NewStore(layout.TasksDir)
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	ledger, err := workledger.NewStore(layout.LedgerDir)
	if err != nil {
		t.Fatalf("new ledger store: %v", err)
	}

	rt := &runtime.Runtime{
		Layout:     layout,
		Tasks:      tasks,
		WorkLedger: ledger,
	}
	if err := ensureTaskQueue(rt); err != nil {
		t.Fatalf("ensure task queue: %v", err)
	}
	t.Cleanup(rt.TaskRunner.Stop)

	userID := "local"
	workspace := t.TempDir()

	type sample struct {
		status   taskqueue.AttemptStatus
		findings bool
		trace    bool
	}
	samples := []sample{
		{status: taskqueue.AttemptSucceeded, findings: true, trace: true},
		{status: taskqueue.AttemptFailed, findings: true, trace: true},
		{status: taskqueue.AttemptCanceled, findings: true, trace: false},
		{status: taskqueue.AttemptTimedOut, findings: false, trace: true},
		{status: taskqueue.AttemptInterrupted, findings: false, trace: false},
	}

	for i, s := range samples {
		task, err := rt.Tasks.CreateTask(userID, workspace, fmt.Sprintf("T%d", i+1), "noop", "", taskqueue.Limits{})
		if err != nil {
			t.Fatalf("CreateTask(%d): %v", i, err)
		}
		attemptID := task.Attempts[0].ID

		now := time.Now().UTC()
		started := now.Add(-200 * time.Millisecond)
		finished := now

		artifactDir := filepath.Join(t.TempDir(), task.ID, attemptID)
		if err := os.MkdirAll(artifactDir, 0o700); err != nil {
			t.Fatalf("mkdir artifacts(%d): %v", i, err)
		}

		findingsPath := ""
		if s.findings {
			findingsPath = filepath.Join(artifactDir, "FINDINGS.md")
			if err := os.WriteFile(findingsPath, []byte("# Findings\n- ok\n"), 0o600); err != nil {
				t.Fatalf("write findings(%d): %v", i, err)
			}
		}
		tracePath := ""
		if s.trace {
			tracePath = filepath.Join(artifactDir, "trace.jsonl")
			if err := os.WriteFile(tracePath, []byte("{\"type\":\"complete\"}\n"), 0o600); err != nil {
				t.Fatalf("write trace(%d): %v", i, err)
			}
		}

		updated, err := rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
			a := tk.LatestAttempt()
			if a == nil || a.ID != attemptID {
				return fmt.Errorf("missing attempt")
			}
			a.Status = s.status
			a.StartedAt = &started
			a.FinishedAt = &finished
			a.Summary = fmt.Sprintf("sample-%d", i+1)
			a.FindingsPath = findingsPath
			a.TraceLogPath = tracePath
			a.Error = ""
			return taskqueue.EnsureArtifactManifestV1(layout.TasksDir, *tk, a)
		})
		if err != nil {
			t.Fatalf("UpdateTask(%d): %v", i, err)
		}

		finalAttempt := updated.LatestAttempt()
		if finalAttempt == nil || finalAttempt.ID != attemptID {
			t.Fatalf("missing final attempt(%d)", i)
		}
		if strings.TrimSpace(finalAttempt.ArtifactManifestVersion) != taskqueue.ArtifactManifestVersionV1 {
			t.Fatalf("expected artifact_manifest_version=%q, got %q", taskqueue.ArtifactManifestVersionV1, finalAttempt.ArtifactManifestVersion)
		}
		if strings.TrimSpace(finalAttempt.ArtifactManifestPath) == "" {
			t.Fatalf("expected artifact_manifest_path to be set(%d)", i)
		}
		if _, err := os.Stat(finalAttempt.ArtifactManifestPath); err != nil {
			t.Fatalf("artifact_manifest_path missing(%d): %v", i, err)
		}

		if rt.TaskRunner == nil || rt.TaskRunner.OnAttemptFinished == nil {
			t.Fatalf("expected OnAttemptFinished to be configured")
		}
		rt.TaskRunner.OnAttemptFinished(context.Background(), updated, *finalAttempt)

		receiptID := fmt.Sprintf("task_%s_attempt_%s", strings.TrimSpace(task.ID), strings.TrimSpace(attemptID))
		r, err := rt.WorkLedger.GetReceipt(receiptID)
		if err != nil {
			t.Fatalf("GetReceipt(%d): %v", i, err)
		}
		if strings.TrimSpace(r.ArtifactManifestVersion) != taskqueue.ArtifactManifestVersionV1 {
			t.Fatalf("expected receipt.artifact_manifest_version=%q, got %q", taskqueue.ArtifactManifestVersionV1, r.ArtifactManifestVersion)
		}
		if strings.TrimSpace(r.ArtifactManifestPath) != strings.TrimSpace(finalAttempt.ArtifactManifestPath) {
			t.Fatalf("expected receipt.artifact_manifest_path=%q, got %q", finalAttempt.ArtifactManifestPath, r.ArtifactManifestPath)
		}
		if strings.TrimSpace(string(r.EvidenceCompleteness)) == "" {
			t.Fatalf("expected evidence_completeness to be set(%d)", i)
		}
	}
}
