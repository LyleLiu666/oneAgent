package workledger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_CreateAndGetReceipt(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	r, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusSucceeded,
		Summary:       "did a thing",
		Artifacts: ReceiptArtifacts{
			FindingsPath: "/tmp/findings.md",
			TraceLogPath: "/tmp/trace.jsonl",
		},
		Signals: ReceiptSignals{DurationMs: 123},
	})
	if err != nil {
		t.Fatalf("CreateReceipt: %v", err)
	}
	if r.ReceiptID == "" {
		t.Fatalf("expected receipt_id")
	}

	loaded, err := store.GetReceipt(r.ReceiptID)
	if err != nil {
		t.Fatalf("GetReceipt: %v", err)
	}
	if loaded.ReceiptID != r.ReceiptID {
		t.Fatalf("unexpected id: %q", loaded.ReceiptID)
	}
	if loaded.PrincipalID != "local" {
		t.Fatalf("unexpected principal: %q", loaded.PrincipalID)
	}

	if _, err := os.Stat(filepath.Join(store.ReceiptsDir(), r.ReceiptID, "receipt.json")); err != nil {
		t.Fatalf("stat receipt.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.ReceiptsDir(), r.ReceiptID, "receipt.md")); err != nil {
		t.Fatalf("stat receipt.md: %v", err)
	}
}

func TestStore_ListReceipts_FiltersAndSearches(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "ledger"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	old, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusFailed,
		StartedAt:     time.Now().Add(-2 * time.Hour),
		FinishedAt:    time.Now().Add(-2 * time.Hour),
		Summary:       "failed to do x",
	})
	if err != nil {
		t.Fatalf("CreateReceipt(old): %v", err)
	}
	_, _ = old, err

	newer, err := store.CreateReceipt(CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          ReceiptKindSubagentRun,
		Status:        ReceiptStatusSucceeded,
		StartedAt:     time.Now().Add(-1 * time.Hour),
		FinishedAt:    time.Now().Add(-1 * time.Hour),
		Summary:       "refactor auth module",
	})
	if err != nil {
		t.Fatalf("CreateReceipt(newer): %v", err)
	}

	list, err := store.ListReceipts(ListReceiptsQuery{
		PrincipalID: "local",
		Workspace:   "/tmp/ws",
		Status:      ReceiptStatusSucceeded,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListReceipts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if list[0].ReceiptID != newer.ReceiptID {
		t.Fatalf("unexpected receipt: %q", list[0].ReceiptID)
	}

	search, err := store.ListReceipts(ListReceiptsQuery{
		PrincipalID: "local",
		Q:           "auth module",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListReceipts(search): %v", err)
	}
	if len(search) != 1 || search[0].ReceiptID != newer.ReceiptID {
		t.Fatalf("unexpected search results: %+v", search)
	}
}

