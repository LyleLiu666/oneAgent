package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

func TestAPIAuth_TokenMode_SupportsSettingsDBTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	token, err := db.CreateAuthToken(ctx, "alice")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	rt := &runtime.Runtime{
		Config:    &config.Config{AuthMode: "token"},
		AuthToken: "local-token-file",
		Settings:  db,
	}

	router := gin.New()
	router.Use(APIAuth(rt))
	router.GET("/api/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": GetUserID(c)})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res["user_id"] != "alice" {
		t.Fatalf("expected user_id=alice, got %q", res["user_id"])
	}
}

func TestAPIAuth_TokenMode_FallsBackToRuntimeTokenFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rt := &runtime.Runtime{
		Config:    &config.Config{AuthMode: "token"},
		AuthToken: "local-token-file",
	}

	router := gin.New()
	router.Use(APIAuth(rt))
	router.GET("/api/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": GetUserID(c)})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer local-token-file")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res["user_id"] != "local" {
		t.Fatalf("expected user_id=local, got %q", res["user_id"])
	}
}

func TestAPIAuth_TokenMode_RejectsRevokedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := settingsdb.Open(dbPath)
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	token, err := db.CreateAuthToken(ctx, "alice")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := db.RevokeAuthToken(ctx, token.Token); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	rt := &runtime.Runtime{
		Config:   &config.Config{AuthMode: "token"},
		Settings: db,
	}

	router := gin.New()
	router.Use(APIAuth(rt))
	router.GET("/api/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": GetUserID(c)})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}
