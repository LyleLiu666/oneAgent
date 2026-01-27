package workledger

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStore_GetOrCreateDigest_WritesAndReuses(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now()
	dayKey := DayKey(now)

	_, err = store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusSucceeded,
		FinishedAt:    now,
		Summary:       "did work",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "/tmp/findings.md",
			TraceLogPath: "/tmp/trace.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt: %v", err)
	}

	d1, err := store.GetOrCreateDigest("local", now, true)
	if err != nil {
		t.Fatalf("GetOrCreateDigest(refresh): %v", err)
	}
	if !strings.Contains(d1.Markdown, "## Completed") {
		t.Fatalf("unexpected digest: %s", d1.Markdown)
	}

	d2, err := store.GetOrCreateDigest("local", now, false)
	if err != nil {
		t.Fatalf("GetOrCreateDigest(cache): %v", err)
	}
	if d2.DayKey != dayKey {
		t.Fatalf("unexpected dayKey: %q", d2.DayKey)
	}
	if d2.GeneratedAt != (time.Time{}) {
		t.Fatalf("expected cached digest GeneratedAt to be zero")
	}
	if strings.TrimSpace(d2.Markdown) == "" {
		t.Fatalf("expected markdown")
	}
}

func TestStore_GenerateDigest_IncludesEvidenceAndFailures(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now()

	ok, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusSucceeded,
		FinishedAt:    now,
		Summary:       "refactor auth",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "findings.md",
			TraceLogPath: "trace.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(ok): %v", err)
	}

	bad, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusFailed,
		FinishedAt:    now,
		Summary:       "add tests (failed)",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "findings2.md",
			TraceLogPath: "trace2.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(bad): %v", err)
	}

	d, err := store.GenerateDigest("local", now)
	if err != nil {
		t.Fatalf("GenerateDigest: %v", err)
	}

	if !strings.Contains(d.Markdown, "## Completed") || !strings.Contains(d.Markdown, ok.ReceiptID) {
		t.Fatalf("expected completed section to include receipt_id, got:\n%s", d.Markdown)
	}
	if !strings.Contains(d.Markdown, "## Needs Attention") || !strings.Contains(d.Markdown, bad.ReceiptID) {
		t.Fatalf("expected needs attention section to include failed receipt_id, got:\n%s", d.Markdown)
	}
	if !strings.Contains(d.Markdown, "## Evidence") ||
		!strings.Contains(d.Markdown, "findings_path: findings.md") ||
		!strings.Contains(d.Markdown, "trace_log_path: trace.jsonl") {
		t.Fatalf("expected evidence section to include artifacts, got:\n%s", d.Markdown)
	}
}
