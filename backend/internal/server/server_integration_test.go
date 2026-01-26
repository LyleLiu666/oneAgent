package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SmokeE2E(t *testing.T) {
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

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/health", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status=%d", res.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		t.Fatalf("decode /health: %v", err)
	}
	if health["status"] == "" {
		t.Fatalf("expected health.status, got %+v", health)
	}

	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / status=%d", res.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(res.Body)
	body := string(bodyBytes)
	if !strings.Contains(body, "<!doctype html") && !strings.Contains(body, "<!DOCTYPE html") {
		t.Fatalf("expected index.html, got body prefix: %q", body[:min(len(body), 120)])
	}

	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/tools", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/tools: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tools status=%d", res.StatusCode)
	}
	var tools []any
	if err := json.NewDecoder(res.Body).Decode(&tools); err != nil {
		t.Fatalf("decode /api/tools: %v", err)
	}
	if len(tools) == 0 {
		t.Fatalf("expected tools list")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
