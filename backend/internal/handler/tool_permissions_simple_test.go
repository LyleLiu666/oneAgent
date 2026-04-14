package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

func TestSimpleToolPermissions_GetUsesCurrentPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	bobToken, err := db.CreateAuthToken(ctx, "bob")
	if err != nil {
		t.Fatalf("create auth token: %v", err)
	}
	if err := db.SetToolPolicy(ctx, "bob", permissions.Policy{
		ID:                "simple_host_full",
		DefaultEffect:     permissions.EffectAllow,
		DefaultCmdProfile: "full",
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "token"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.GET("/tool_permissions/simple", GetSimpleToolPermissions)

	req := httptest.NewRequest(http.MethodGet, "/api/tool_permissions/simple", nil)
	req.Header.Set("Authorization", "Bearer "+bobToken.Token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body simpleToolPermissionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PrincipalID != "bob" {
		t.Fatalf("expected principal_id=bob, got %+v", body)
	}
	if body.AdvancedSettingsAvailable {
		t.Fatalf("expected advanced settings to be unavailable for bob")
	}
}

func TestSimpleToolPermissions_UpdateRejectsExplicitPrincipalID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "none"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.PUT("/tool_permissions/simple", UpdateSimpleToolPermissions)

	payload, _ := json.Marshal(map[string]any{
		"principal_id": "bob",
		"mode":         "readonly",
		"source":       "chat_header",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tool_permissions/simple", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSimpleToolPermissions_UpdateAppliesPresetForCurrentPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	bobToken, err := db.CreateAuthToken(ctx, "bob")
	if err != nil {
		t.Fatalf("create auth token: %v", err)
	}
	if err := db.SetToolPolicy(ctx, "alice", permissions.Policy{
		ID:                "simple_readonly",
		DefaultEffect:     permissions.EffectAllow,
		DefaultCmdProfile: "readonly",
	}); err != nil {
		t.Fatalf("seed alice policy: %v", err)
	}

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "token"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.PUT("/tool_permissions/simple", UpdateSimpleToolPermissions)

	payload, _ := json.Marshal(map[string]any{
		"mode":                  "host_full",
		"command_approval_mode": "manual",
		"source":                "chat_header",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tool_permissions/simple", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+bobToken.Token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	bobPolicy, ok, err := db.GetToolPolicy(ctx, "bob")
	if err != nil {
		t.Fatalf("get bob policy: %v", err)
	}
	if !ok || bobPolicy.ID != "simple_host_full" {
		t.Fatalf("expected bob to receive host_full policy, got ok=%v policy=%+v", ok, bobPolicy)
	}

	alicePolicy, ok, err := db.GetToolPolicy(ctx, "alice")
	if err != nil {
		t.Fatalf("get alice policy: %v", err)
	}
	if !ok || alicePolicy.ID != "simple_readonly" {
		t.Fatalf("expected alice policy unchanged, got ok=%v policy=%+v", ok, alicePolicy)
	}

	commandApprovalMode, err := db.GetUserSetting(ctx, "bob", settingsdb.SettingKeyCommandApprovalMode)
	if err != nil {
		t.Fatalf("get bob approval mode: %v", err)
	}
	if commandApprovalMode != "manual" {
		t.Fatalf("expected bob approval mode=manual, got %q", commandApprovalMode)
	}
}

func TestSimpleToolPermissions_UpdateApprovalOnlyDoesNotPersistImplicitDefaultPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "none"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.PUT("/tool_permissions/simple", UpdateSimpleToolPermissions)

	payload, _ := json.Marshal(map[string]any{
		"command_approval_mode": "manual",
		"source":                "chat_header",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tool_permissions/simple", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	if _, ok, err := db.GetToolPolicy(ctx, "local"); err != nil {
		t.Fatalf("get local policy: %v", err)
	} else if ok {
		t.Fatalf("expected approval-only update to keep local without an explicit tool policy")
	}

	commandApprovalMode, err := db.GetUserSetting(ctx, "local", settingsdb.SettingKeyCommandApprovalMode)
	if err != nil {
		t.Fatalf("get local approval mode: %v", err)
	}
	if commandApprovalMode != "manual" {
		t.Fatalf("expected local approval mode=manual, got %q", commandApprovalMode)
	}
}

func TestSimpleToolPermissions_UpdateRejectsInvalidCommandApprovalMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "none"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.PUT("/tool_permissions/simple", UpdateSimpleToolPermissions)

	payload, _ := json.Marshal(map[string]any{
		"command_approval_mode": "later",
		"source":                "chat_header",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tool_permissions/simple", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSimpleToolPermissions_UpdateRejectsInvalidSource(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "none"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.PUT("/tool_permissions/simple", UpdateSimpleToolPermissions)

	payload, _ := json.Marshal(map[string]any{
		"mode":   "readonly",
		"source": "spreadsheet_import",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tool_permissions/simple", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
