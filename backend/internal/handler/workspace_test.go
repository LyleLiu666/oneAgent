package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestChooseWorkspace_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	prev := chooseWorkspaceDir
	chooseWorkspaceDir = func(_ context.Context) (string, bool, error) {
		return dir, false, nil
	}
	t.Cleanup(func() { chooseWorkspaceDir = prev })

	router := gin.New()
	router.POST("/api/workspace/choose", ChooseWorkspace)

	req := httptest.NewRequest(http.MethodPost, "/api/workspace/choose", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Path == "" {
		t.Fatalf("expected non-empty path, got %q", payload.Path)
	}
}

func TestChooseWorkspace_Canceled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := chooseWorkspaceDir
	chooseWorkspaceDir = func(_ context.Context) (string, bool, error) {
		return "", true, nil
	}
	t.Cleanup(func() { chooseWorkspaceDir = prev })

	router := gin.New()
	router.POST("/api/workspace/choose", ChooseWorkspace)

	req := httptest.NewRequest(http.MethodPost, "/api/workspace/choose", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Canceled bool `json:"canceled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Canceled {
		t.Fatalf("expected canceled=true, got false")
	}
}

func TestChooseWorkspace_NotSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := chooseWorkspaceDir
	chooseWorkspaceDir = func(_ context.Context) (string, bool, error) {
		return "", false, fmt.Errorf("%w: boom", ErrWorkspaceChooserNotSupported)
	}
	t.Cleanup(func() { chooseWorkspaceDir = prev })

	router := gin.New()
	router.POST("/api/workspace/choose", ChooseWorkspace)

	req := httptest.NewRequest(http.MethodPost, "/api/workspace/choose", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected %d, got %d (%s)", http.StatusNotImplemented, rec.Code, rec.Body.String())
	}
}
