package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_SOPSuggesionsGenerateAPI_Smoke(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	// Seed receipts that qualify as evidence.
	_, err = rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          workledger.ReceiptKindSubagentRun,
		Status:        workledger.ReceiptStatusSucceeded,
		Summary:       "Refactor auth flow",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: "findings.md",
			TraceLogPath: "trace.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r1): %v", err)
	}
	_, err = rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: "/tmp/ws",
		Kind:          workledger.ReceiptKindSubagentRun,
		Status:        workledger.ReceiptStatusSucceeded,
		Summary:       "Add tests for auth",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: "findings2.md",
			TraceLogPath: "trace2.jsonl",
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r2): %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body := map[string]any{
		"count": 1,
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/ledger/sop_suggestions/generate", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST generate: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d", res.StatusCode)
	}
	var created []workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(created))
	}
	if created[0].SuggestionID == "" {
		t.Fatalf("expected suggestion_id")
	}
	if created[0].Scores.TotalScore <= 0 {
		t.Fatalf("expected non-zero total score")
	}
	if len(created[0].EvidenceReceiptIDs) != 2 {
		t.Fatalf("expected 2 evidence ids")
	}
	uniq := map[string]bool{}
	for _, id := range created[0].EvidenceReceiptIDs {
		uniq[id] = true
	}
	if len(uniq) != 2 {
		t.Fatalf("expected two different receipts")
	}

	// Second call should avoid creating duplicates for the same evidence pair.
	res2, err := http.Post(srv.URL+"/api/ledger/sop_suggestions/generate", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST generate again: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("POST again status=%d", res2.StatusCode)
	}
	var created2 []workledger.Suggestion
	if err := json.NewDecoder(res2.Body).Decode(&created2); err != nil {
		t.Fatalf("decode created2: %v", err)
	}
	if len(created2) != 0 {
		t.Fatalf("expected 0 new suggestions, got %d", len(created2))
	}
}
