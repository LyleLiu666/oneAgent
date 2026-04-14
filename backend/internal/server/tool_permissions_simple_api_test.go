package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SimpleToolPermissionsAPI_GetAndUpdate(t *testing.T) {
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

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tool_permissions/simple", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET simple tool permissions: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET simple tool permissions status=%d", res.StatusCode)
	}

	var getBody struct {
		CurrentMode         string `json:"current_mode"`
		CommandApprovalMode string `json:"command_approval_mode"`
	}
	if err := json.NewDecoder(res.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode GET simple tool permissions: %v", err)
	}
	if getBody.CurrentMode == "" {
		t.Fatalf("expected current_mode to be returned")
	}
	if getBody.CommandApprovalMode != "auto" {
		t.Fatalf("expected default command approval mode=auto, got %q", getBody.CommandApprovalMode)
	}

	payload, _ := json.Marshal(map[string]any{
		"mode":                  "readonly",
		"command_approval_mode": "manual",
		"source":                "chat_header",
	})
	updateReq, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/tool_permissions/simple", bytes.NewReader(payload))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRes, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("PUT simple tool permissions: %v", err)
	}
	defer updateRes.Body.Close()
	if updateRes.StatusCode != http.StatusOK {
		t.Fatalf("PUT simple tool permissions status=%d", updateRes.StatusCode)
	}

	var updateBody struct {
		CurrentMode         string `json:"current_mode"`
		CommandApprovalMode string `json:"command_approval_mode"`
	}
	if err := json.NewDecoder(updateRes.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode PUT simple tool permissions: %v", err)
	}
	if updateBody.CurrentMode != "readonly" {
		t.Fatalf("expected readonly current_mode after update, got %q", updateBody.CurrentMode)
	}
	if updateBody.CommandApprovalMode != "manual" {
		t.Fatalf("expected manual command approval mode after update, got %q", updateBody.CommandApprovalMode)
	}
}
