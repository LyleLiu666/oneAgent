package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	rtpkg "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/scope"
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

	var payload struct {
		Error string `json:"error"`
		Code  string `json:"code"`
		Hint  string `json:"hint"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Code != "workspace_chooser_unsupported" {
		t.Fatalf("expected code workspace_chooser_unsupported, got %q", payload.Code)
	}
	if payload.Error == "" || payload.Error == "服务器错误" {
		t.Fatalf("expected actionable error message, got %q", payload.Error)
	}
	if payload.Hint == "" {
		t.Fatalf("expected hint for manual workspace input")
	}
}

func TestBrowseWorkspace_Roots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	normalizedHome, err := scope.NormalizeWorkspaceRoot(home)
	if err != nil {
		t.Fatalf("normalize home: %v", err)
	}
	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		CurrentPath string `json:"current_path"`
		RootPath    string `json:"root_path"`
		ParentPath  string `json:"parent_path"`
		Entries     []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.CurrentPath != "" {
		t.Fatalf("expected empty current_path at root listing, got %q", payload.CurrentPath)
	}
	if payload.RootPath != "" {
		t.Fatalf("expected empty root_path at root listing, got %q", payload.RootPath)
	}
	if payload.ParentPath != "" {
		t.Fatalf("expected empty parent_path at root listing, got %q", payload.ParentPath)
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("expected 1 root entry, got %d", len(payload.Entries))
	}
	if payload.Entries[0].Path != normalizedHome {
		t.Fatalf("expected root path %q, got %q", normalizedHome, payload.Entries[0].Path)
	}
}

func TestBrowseWorkspace_DirectoryListing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	normalizedHome, err := scope.NormalizeWorkspaceRoot(home)
	if err != nil {
		t.Fatalf("normalize home: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse?path="+url.QueryEscape(home), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		CurrentPath string `json:"current_path"`
		RootPath    string `json:"root_path"`
		ParentPath  string `json:"parent_path"`
		Entries     []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.CurrentPath != normalizedHome {
		t.Fatalf("expected current_path %q, got %q", normalizedHome, payload.CurrentPath)
	}
	if payload.RootPath != normalizedHome {
		t.Fatalf("expected root_path %q, got %q", normalizedHome, payload.RootPath)
	}
	if payload.ParentPath != "" {
		t.Fatalf("expected empty parent_path at root, got %q", payload.ParentPath)
	}
	if len(payload.Entries) != 2 {
		t.Fatalf("expected 2 directory entries, got %d", len(payload.Entries))
	}
	if payload.Entries[0].Name != "alpha" || payload.Entries[1].Name != "beta" {
		t.Fatalf("expected sorted directory entries alpha,beta, got %+v", payload.Entries)
	}
}

func TestBrowseWorkspace_HidesInternalDirectoriesAndMarksHomeRootBrowseOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".oneagent", "config"), 0o755); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, "project"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, "bash-root"), 0o755); err != nil {
		t.Fatalf("mkdir bash-root: %v", err)
	}

	normalizedHome, err := scope.NormalizeWorkspaceRoot(home)
	if err != nil {
		t.Fatalf("normalize home: %v", err)
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home:                home,
			BashRootDir:         filepath.Join(home, "bash-root"),
			BashRootDirExplicit: true,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse?path="+url.QueryEscape(home), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		CurrentPath      string `json:"current_path"`
		RootPath         string `json:"root_path"`
		CanSelectCurrent bool   `json:"can_select_current"`
		Entries          []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.CurrentPath != normalizedHome {
		t.Fatalf("expected current_path %q, got %q", normalizedHome, payload.CurrentPath)
	}
	if payload.RootPath != normalizedHome {
		t.Fatalf("expected root_path %q, got %q", normalizedHome, payload.RootPath)
	}
	if payload.CanSelectCurrent {
		t.Fatalf("expected home root to be browse-only")
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("expected only project entry to remain visible, got %+v", payload.Entries)
	}
	if payload.Entries[0].Name != "project" {
		t.Fatalf("expected project entry, got %+v", payload.Entries)
	}
}

func TestBrowseWorkspace_RejectsProtectedInternalDirectories(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	internal := filepath.Join(home, ".oneagent")
	if err := os.MkdirAll(filepath.Join(internal, "config"), 0o755); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse?path="+url.QueryEscape(internal), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Code != "workspace_browser_path_denied" {
		t.Fatalf("expected workspace_browser_path_denied, got %q", payload.Code)
	}
}

func TestBrowseWorkspace_PrefersMostSpecificRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	bashRoot := filepath.Join(home, "bash-root")
	if err := os.MkdirAll(filepath.Join(bashRoot, "project"), 0o755); err != nil {
		t.Fatalf("mkdir bash root: %v", err)
	}

	normalizedBashRoot, err := scope.NormalizeWorkspaceRoot(bashRoot)
	if err != nil {
		t.Fatalf("normalize bash root: %v", err)
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home:                home,
			BashRootDir:         bashRoot,
			BashRootDirExplicit: true,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse?path="+url.QueryEscape(bashRoot), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		CurrentPath      string `json:"current_path"`
		RootPath         string `json:"root_path"`
		ParentPath       string `json:"parent_path"`
		CanSelectCurrent bool   `json:"can_select_current"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.CurrentPath != normalizedBashRoot {
		t.Fatalf("expected current_path %q, got %q", normalizedBashRoot, payload.CurrentPath)
	}
	if payload.RootPath != normalizedBashRoot {
		t.Fatalf("expected most specific root_path %q, got %q", normalizedBashRoot, payload.RootPath)
	}
	if payload.ParentPath != "" {
		t.Fatalf("expected no parent path at nested root, got %q", payload.ParentPath)
	}
	if !payload.CanSelectCurrent {
		t.Fatalf("expected nested bash root to remain selectable")
	}
}

func TestBrowseWorkspace_RejectsOutsideAllowedRoots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	outside := t.TempDir()

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse?path="+url.QueryEscape(outside), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Code != "workspace_browser_path_denied" {
		t.Fatalf("expected workspace_browser_path_denied, got %q", payload.Code)
	}
}

func TestBrowseWorkspace_NotSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{},
	}))
	router.GET("/api/workspace/browse", BrowseWorkspace)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace/browse", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected %d, got %d (%s)", http.StatusNotImplemented, rec.Code, rec.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
		Hint string `json:"hint"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Code != "workspace_browser_unsupported" {
		t.Fatalf("expected workspace_browser_unsupported, got %q", payload.Code)
	}
	if payload.Hint == "" {
		t.Fatalf("expected non-empty hint")
	}
}

func TestCreateWorkspaceDir_CreatesAndReturnsPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	normalizedHome, err := scope.NormalizeWorkspaceRoot(home)
	if err != nil {
		t.Fatalf("normalize home: %v", err)
	}

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.POST("/api/workspace/browse/create", CreateWorkspaceDir)

	body := strings.NewReader(`{"parent_path":` + fmt.Sprintf("%q", home) + `,"name":"project"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/workspace/browse/create", body)
	req.Header.Set("Content-Type", "application/json")
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
	expected := filepath.Join(normalizedHome, "project")
	if payload.Path != expected {
		t.Fatalf("expected path %q, got %q", expected, payload.Path)
	}
	info, err := os.Stat(expected)
	if err != nil {
		t.Fatalf("stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected created path to be directory")
	}
}

func TestCreateWorkspaceDir_RejectsInvalidName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()

	router := gin.New()
	router.Use(middleware.InjectRuntime(&rtpkg.Runtime{
		Config: &config.Config{
			Home: home,
		},
	}))
	router.POST("/api/workspace/browse/create", CreateWorkspaceDir)

	body := strings.NewReader(`{"parent_path":` + fmt.Sprintf("%q", home) + `,"name":"../escape"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/workspace/browse/create", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (%s)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Code != "workspace_browser_invalid_name" {
		t.Fatalf("expected workspace_browser_invalid_name, got %q", payload.Code)
	}
}
