package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestServer_ToolApprovalsAPI_ApproveAndConsume(t *testing.T) {
	cfg := &config.Config{
		Profile:  "local",
		Bind:     "127.0.0.1",
		Port:     "0",
		Home:     t.TempDir(),
		AuthMode: "none",
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	approvalID, err := rt.Settings.EnsurePendingToolApproval(context.Background(), "local", "scope-1", tool.ToolIDWriteFile, "hash-1")
	if err != nil {
		t.Fatalf("create pending approval: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"reason": "ok"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/approvals/"+approvalID+"/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST approve: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST approve status=%d", res.StatusCode)
	}

	consumed, err := rt.Settings.ConsumeApprovedToolApproval(context.Background(), "local", "scope-1", tool.ToolIDWriteFile, "hash-1")
	if err != nil {
		t.Fatalf("consume approved: %v", err)
	}
	if !consumed {
		t.Fatalf("expected approval to be consumable after approval")
	}
}

func TestServer_ToolApprovalsAPI_Deny(t *testing.T) {
	cfg := &config.Config{
		Profile:  "local",
		Bind:     "127.0.0.1",
		Port:     "0",
		Home:     t.TempDir(),
		AuthMode: "none",
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	approvalID, err := rt.Settings.EnsurePendingToolApproval(context.Background(), "local", "scope-1", tool.ToolIDWriteFile, "hash-2")
	if err != nil {
		t.Fatalf("create pending approval: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"reason": "no"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/approvals/"+approvalID+"/deny", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST deny: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST deny status=%d", res.StatusCode)
	}
}
