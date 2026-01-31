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

func TestServer_CommandApprovalSettingsAPI_GetAndUpdate(t *testing.T) {
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

	getMode := func() string {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/command_approvals/settings", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET settings: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET settings status=%d", res.StatusCode)
		}
		var body struct {
			CommandApprovalMode string `json:"command_approval_mode"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode GET: %v", err)
		}
		return body.CommandApprovalMode
	}

	if got := getMode(); got != "auto" {
		t.Fatalf("expected default mode=auto, got %q", got)
	}

	payload, _ := json.Marshal(map[string]any{"command_approval_mode": "manual"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/command_approvals/settings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT settings: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PUT settings status=%d", res.StatusCode)
	}

	if got := getMode(); got != "manual" {
		t.Fatalf("expected updated mode=manual, got %q", got)
	}
}

