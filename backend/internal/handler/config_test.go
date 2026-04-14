package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	rtpkg "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestGetRuntimeConfig_IncludesWorkspaceChooserCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := getWorkspaceChooserCapability
	getWorkspaceChooserCapability = func() workspaceChooserCapability {
		return workspaceChooserCapability{
			Supported: false,
			Reason:    "Native folder chooser is unavailable in this server environment.",
		}
	}
	t.Cleanup(func() { getWorkspaceChooserCapability = prev })

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Bind:             "0.0.0.0",
			Port:             "8080",
			DefaultWorkspace: "/tmp/workspace",
		},
	}))
	router.GET("/api/config", GetRuntimeConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		DefaultWorkspace          string   `json:"default_workspace"`
		BaseURL                   string   `json:"base_url"`
		Warnings                  []string `json:"warnings"`
		WorkspaceChooserSupported bool     `json:"workspace_chooser_supported"`
		WorkspaceChooserReason    string   `json:"workspace_chooser_reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.DefaultWorkspace != "/tmp/workspace" {
		t.Fatalf("expected default_workspace to be returned, got %q", payload.DefaultWorkspace)
	}
	if payload.BaseURL != "http://localhost:8080" {
		t.Fatalf("expected base_url to be returned, got %q", payload.BaseURL)
	}
	if payload.WorkspaceChooserSupported {
		t.Fatalf("expected workspace chooser to be marked unsupported")
	}
	if payload.WorkspaceChooserReason == "" {
		t.Fatalf("expected workspace chooser reason")
	}
	if len(payload.Warnings) == 0 {
		t.Fatalf("expected non-loopback bind warning")
	}
}
