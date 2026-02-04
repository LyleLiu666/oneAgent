package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SecretaryRecoveryFocus_PersistsInState(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
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

	body := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/state")
	var st struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if strings.TrimSpace(st.SessionID) == "" {
		t.Fatalf("expected session_id, got %+v", st)
	}

	var focusResp struct {
		RecoveryFocus *struct {
			TaskID    string `json:"task_id"`
			AttemptID string `json:"attempt_id"`
		} `json:"recovery_focus"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/recovery/focus", map[string]any{
		"task_id":    "t1",
		"attempt_id": "a1",
	}, &focusResp)
	if focusResp.RecoveryFocus == nil || focusResp.RecoveryFocus.TaskID != "t1" || focusResp.RecoveryFocus.AttemptID != "a1" {
		t.Fatalf("expected recovery focus to be set, got %+v", focusResp)
	}

	body2 := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/state")
	var st2 struct {
		RecoveryFocus *struct {
			TaskID    string `json:"task_id"`
			AttemptID string `json:"attempt_id"`
		} `json:"recovery_focus"`
	}
	if err := json.Unmarshal(body2, &st2); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if st2.RecoveryFocus == nil || st2.RecoveryFocus.TaskID != "t1" || st2.RecoveryFocus.AttemptID != "a1" {
		t.Fatalf("expected recovery focus to persist in state, got %+v", st2)
	}

	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/recovery/focus", map[string]any{}, &focusResp)
	if focusResp.RecoveryFocus != nil {
		t.Fatalf("expected recovery focus to clear, got %+v", focusResp)
	}
}
