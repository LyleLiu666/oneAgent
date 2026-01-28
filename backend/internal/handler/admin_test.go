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

func TestAdminTokensAndPolicies_LocalOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
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
	api.POST("/admin/tokens", CreateAuthToken)
	api.GET("/admin/tokens", ListAuthTokens)
	api.POST("/admin/tokens/revoke", RevokeAuthToken)
	api.PUT("/admin/tool_policies/:principal_id", SetToolPolicy)
	api.GET("/admin/tool_policies/:principal_id", GetToolPolicy)

	createReq := createAuthTokenRequest{PrincipalID: "alice"}
	createBody, _ := json.Marshal(createReq)
	createHTTP := httptest.NewRequest(http.MethodPost, "/api/admin/tokens", bytes.NewReader(createBody))
	createHTTP.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createHTTP)
	if createW.Code != http.StatusOK {
		t.Fatalf("create token: expected 200, got %d body=%s", createW.Code, createW.Body.String())
	}
	var created authTokenResponse
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if created.PrincipalID != "alice" || created.Token == "" {
		t.Fatalf("unexpected create response: %+v", created)
	}

	listHTTP := httptest.NewRequest(http.MethodGet, "/api/admin/tokens", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listHTTP)
	if listW.Code != http.StatusOK {
		t.Fatalf("list tokens: expected 200, got %d body=%s", listW.Code, listW.Body.String())
	}
	var list []authTokenResponse
	if err := json.Unmarshal(listW.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	found := false
	for _, item := range list {
		if item.Token == created.Token && item.PrincipalID == "alice" && item.RevokedAt == nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected created token in list, got %+v", list)
	}

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{
			{ID: "deny-bash", Effect: permissions.EffectDeny, ToolID: "bash"},
		},
	}
	policyBody, _ := json.Marshal(policy)
	setPolicyHTTP := httptest.NewRequest(http.MethodPut, "/api/admin/tool_policies/alice", bytes.NewReader(policyBody))
	setPolicyHTTP.Header.Set("Content-Type", "application/json")
	setPolicyW := httptest.NewRecorder()
	router.ServeHTTP(setPolicyW, setPolicyHTTP)
	if setPolicyW.Code != http.StatusOK {
		t.Fatalf("set policy: expected 200, got %d body=%s", setPolicyW.Code, setPolicyW.Body.String())
	}

	getPolicyHTTP := httptest.NewRequest(http.MethodGet, "/api/admin/tool_policies/alice", nil)
	getPolicyW := httptest.NewRecorder()
	router.ServeHTTP(getPolicyW, getPolicyHTTP)
	if getPolicyW.Code != http.StatusOK {
		t.Fatalf("get policy: expected 200, got %d body=%s", getPolicyW.Code, getPolicyW.Body.String())
	}
	var getRes toolPolicyResponse
	if err := json.Unmarshal(getPolicyW.Body.Bytes(), &getRes); err != nil {
		t.Fatalf("unmarshal get policy: %v", err)
	}
	if getRes.PrincipalID != "alice" || !getRes.Exists || getRes.Policy.ID != "p1" {
		t.Fatalf("unexpected get policy response: %+v", getRes)
	}

	revokeBody, _ := json.Marshal(revokeAuthTokenRequest{Token: created.Token})
	revokeHTTP := httptest.NewRequest(http.MethodPost, "/api/admin/tokens/revoke", bytes.NewReader(revokeBody))
	revokeHTTP.Header.Set("Content-Type", "application/json")
	revokeW := httptest.NewRecorder()
	router.ServeHTTP(revokeW, revokeHTTP)
	if revokeW.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d body=%s", revokeW.Code, revokeW.Body.String())
	}
}

func TestAdminEndpoints_ForbiddenForNonLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	bob, err := db.CreateAuthToken(ctx, "bob")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "token"},
		Settings: db,
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.APIAuth(rt))
	api := router.Group("/api")
	api.GET("/admin/tokens", ListAuthTokens)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+bob.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}
