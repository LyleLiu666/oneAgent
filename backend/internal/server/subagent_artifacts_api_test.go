package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SubagentArtifactsAPI_Smoke(t *testing.T) {
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

	sessionID := "sess-1"
	if rt.Sessions == nil {
		t.Fatalf("sessions store not initialized")
	}
	if _, err := rt.Sessions.GetOrCreateSession(sessionID, "local", "chat", "t"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	runID := uuid.NewString()
	dayKey := time.Now().Format("2006-01-02")
	runDir := filepath.Join(rt.Layout.SubagentLogsDir, dayKey, sessionID, runID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "trace.jsonl"), []byte("{\"type\":\"start\"}\n"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "FINDINGS.md"), []byte("# Findings\n- ok\n"), 0o600); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	traceReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/subagent/sessions/"+sessionID+"/runs/"+runID+"/artifacts/trace", nil)
	traceRes, err := http.DefaultClient.Do(traceReq)
	if err != nil {
		t.Fatalf("GET trace: %v", err)
	}
	defer traceRes.Body.Close()
	if traceRes.StatusCode != http.StatusOK {
		t.Fatalf("GET trace status=%d", traceRes.StatusCode)
	}
	var traceOut map[string]any
	if err := json.NewDecoder(traceRes.Body).Decode(&traceOut); err != nil {
		t.Fatalf("decode trace response: %v", err)
	}
	traceContent, _ := traceOut["content"].(string)
	if !strings.Contains(traceContent, "\"type\"") {
		t.Fatalf("unexpected trace content: %+v", traceOut)
	}

	findReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/subagent/sessions/"+sessionID+"/runs/"+runID+"/artifacts/findings", nil)
	findRes, err := http.DefaultClient.Do(findReq)
	if err != nil {
		t.Fatalf("GET findings: %v", err)
	}
	defer findRes.Body.Close()
	if findRes.StatusCode != http.StatusOK {
		t.Fatalf("GET findings status=%d", findRes.StatusCode)
	}
	var findOut map[string]any
	if err := json.NewDecoder(findRes.Body).Decode(&findOut); err != nil {
		t.Fatalf("decode findings response: %v", err)
	}
	findContent, _ := findOut["content"].(string)
	if !strings.Contains(findContent, "# Findings") {
		t.Fatalf("unexpected findings content: %+v", findOut)
	}
}
