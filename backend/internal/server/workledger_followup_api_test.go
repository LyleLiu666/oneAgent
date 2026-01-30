package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_WorkLedgerFollowUpAPI_CreatesTaskFromReceipts(t *testing.T) {
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

	ws := filepath.Join(home, "ws")
	if err := os.MkdirAll(ws, 0o700); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	normalizedWS, err := scope.NormalizeWorkspaceRoot(ws)
	if err != nil {
		t.Fatalf("normalize ws: %v", err)
	}
	now := time.Now()

	r1, err := rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: ws,
		Kind:          workledger.ReceiptKindSubagentRun,
		Status:        workledger.ReceiptStatusFailed,
		StartedAt:     now.Add(-2 * time.Minute),
		FinishedAt:    now.Add(-time.Minute),
		Summary:       "tool failed",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: filepath.Join(home, "FINDINGS1.md"),
			TraceLogPath: filepath.Join(home, "trace1.jsonl"),
		},
	})
	if err != nil {
		t.Fatalf("CreateReceipt(r1): %v", err)
	}
	r2, err := rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
		PrincipalID:   "local",
		WorkspaceRoot: ws,
		Kind:          workledger.ReceiptKindSubagentRun,
		Status:        workledger.ReceiptStatusInterrupted,
		StartedAt:     now.Add(-time.Minute),
		FinishedAt:    now,
		Summary:       "server restarted",
		Artifacts: workledger.ReceiptArtifacts{
			FindingsPath: filepath.Join(home, "FINDINGS2.md"),
			TraceLogPath: filepath.Join(home, "trace2.jsonl"),
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

	body, _ := json.Marshal(map[string]any{
		"receipt_ids": []string{r1.ReceiptID, r2.ReceiptID},
		"instruction": "把这两个失败点修复并补齐测试",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/followups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/ledger/followups: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/ledger/followups status=%d", res.StatusCode)
	}

	var task map[string]any
	if err := json.NewDecoder(res.Body).Decode(&task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if got := task["workspace"]; got != normalizedWS {
		t.Fatalf("unexpected workspace: %v", got)
	}
	prompt, _ := task["prompt"].(string)
	if prompt == "" || !bytes.Contains([]byte(prompt), []byte(r1.ReceiptID)) {
		t.Fatalf("expected prompt to include receipt id")
	}
}
