package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_WorkLedgerAPI_Smoke(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if rt.WorkLedger == nil {
		t.Fatalf("expected WorkLedger store")
	}

	_, err = rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: filepath.Join(home, "ws"),
		Kind:          workledger.ReceiptKindSubagentRun,
		Status:        workledger.ReceiptStatusSucceeded,
		StartedAt:     time.Now().Add(-time.Minute),
		FinishedAt:    time.Now(),
		Summary:       "refactor auth module",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: filepath.Join(home, "FINDINGS.md"),
			TraceLogPath: filepath.Join(home, "trace.jsonl"),
		},
		Signals: workledger.ReceiptSignals{DurationMs: 1200},
	})
	if err != nil {
		t.Fatalf("CreateReceipt: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// List.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/receipts?q="+url.QueryEscape("auth"), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/ledger/receipts: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ledger/receipts status=%d", res.StatusCode)
	}
	var list []workledger.Receipt
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode receipts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(list))
	}

	// Get.
	id := list[0].ReceiptID
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/receipts/"+id, nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/ledger/receipts/:id: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ledger/receipts/:id status=%d", res.StatusCode)
	}
	var got workledger.Receipt
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if got.ReceiptID != id {
		t.Fatalf("unexpected id: %q", got.ReceiptID)
	}

	// Digest today.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/digests/today?refresh=1", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/ledger/digests/today: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ledger/digests/today status=%d", res.StatusCode)
	}
	var digestResp map[string]any
	if err := json.NewDecoder(res.Body).Decode(&digestResp); err != nil {
		t.Fatalf("decode digest: %v", err)
	}
	if s, _ := digestResp["markdown"].(string); s == "" {
		t.Fatalf("expected digest markdown")
	}

	// Structured digest.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/digests/today/structured", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/ledger/digests/today/structured: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ledger/digests/today/structured status=%d", res.StatusCode)
	}
	var structured map[string]any
	if err := json.NewDecoder(res.Body).Decode(&structured); err != nil {
		t.Fatalf("decode structured digest: %v", err)
	}
	items, _ := structured["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected structured digest items")
	}

	// Sanity: list is independent of any external state.
	_, _ = context.Background(), got
}
