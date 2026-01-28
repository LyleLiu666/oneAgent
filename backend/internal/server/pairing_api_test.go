package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_PairingAPI_ExchangeCreatesToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)

	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "token",
		LogRetentionDays: 1,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	now := time.Date(2026, 1, 28, 0, 0, 0, 0, time.UTC)
	rt.Pairing.SetNow(func() time.Time { return now })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Admin creates pairing code for alice.
	createBody, _ := json.Marshal(map[string]any{
		"principal_id": "alice",
		"ttl_seconds":  60,
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/admin/pairing_codes", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/admin/pairing_codes: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d", res.StatusCode)
	}
	var pairing struct {
		Code        string `json:"code"`
		PrincipalID string `json:"principal_id"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pairing); err != nil {
		t.Fatalf("decode pairing: %v", err)
	}
	if strings.TrimSpace(pairing.Code) == "" || pairing.PrincipalID != "alice" {
		t.Fatalf("unexpected pairing response: %+v", pairing)
	}

	// New device exchanges code for long-lived token (public endpoint).
	exBody, _ := json.Marshal(map[string]any{"code": pairing.Code})
	res, err = http.Post(srv.URL+"/api/auth/pair/exchange", "application/json", bytes.NewReader(exBody))
	if err != nil {
		t.Fatalf("POST /api/auth/pair/exchange: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("exchange status=%d", res.StatusCode)
	}
	var exchanged struct {
		Token       string `json:"token"`
		PrincipalID string `json:"principal_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&exchanged); err != nil {
		t.Fatalf("decode exchange: %v", err)
	}
	if strings.TrimSpace(exchanged.Token) == "" || exchanged.PrincipalID != "alice" {
		t.Fatalf("unexpected exchange response: %+v", exchanged)
	}

	// Token should authenticate as alice.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+exchanged.Token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status=%d", res.StatusCode)
	}
	var me struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.UserID != "alice" {
		t.Fatalf("expected user_id=alice, got %q", me.UserID)
	}

	// Pairing code cannot be reused.
	res, err = http.Post(srv.URL+"/api/auth/pair/exchange", "application/json", bytes.NewReader(exBody))
	if err != nil {
		t.Fatalf("POST exchange reuse: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	var errPayload map[string]any
	_ = json.NewDecoder(res.Body).Decode(&errPayload)
	msg := strings.ToLower(strings.TrimSpace(errPayload["error"].(string)))
	if !strings.Contains(msg, "already used") {
		t.Fatalf("expected reuse error, got: %v", errPayload)
	}

	// Expired code should be rejected.
	now = now.Add(10 * time.Second)
	createBody, _ = json.Marshal(map[string]any{
		"principal_id": "bob",
		"ttl_seconds":  1,
	})
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/admin/pairing_codes", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST create 2: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("create2 status=%d", res.StatusCode)
	}
	var pairing2 struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pairing2); err != nil {
		t.Fatalf("decode pairing2: %v", err)
	}

	now = now.Add(2 * time.Second)
	exBody2, _ := json.Marshal(map[string]any{"code": pairing2.Code})
	res, err = http.Post(srv.URL+"/api/auth/pair/exchange", "application/json", bytes.NewReader(exBody2))
	if err != nil {
		t.Fatalf("POST exchange expired: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 expired, got %d", res.StatusCode)
	}
	errPayload = map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&errPayload)
	msg = strings.ToLower(strings.TrimSpace(errPayload["error"].(string)))
	if !strings.Contains(msg, "expired") {
		t.Fatalf("expected expired error, got: %v", errPayload)
	}
}

