package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestGetSkill_IncludesFilesForReadOnlySkill(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: x\ndescription: d\n---\n# Main\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "notes.md"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	cfg := &config.Config{Home: home}
	prevCfg := config.AppConfig
	config.AppConfig = cfg
	t.Cleanup(func() { config.AppConfig = prevCfg })

	rt := &runtime.Runtime{Config: cfg}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.GET("/api/skills/:id", GetSkill)

	req := httptest.NewRequest(http.MethodGet, "/api/skills/x", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		SkillID    string   `json:"skill_id"`
		Archivable bool     `json:"archivable"`
		Files      []string `json:"files"`
		SkillMD    string   `json:"skill_md"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.SkillID != "x" {
		t.Fatalf("expected skill_id=x, got %q", payload.SkillID)
	}
	if payload.Archivable {
		t.Fatalf("expected archivable=false")
	}
	if !strings.Contains(payload.SkillMD, "# Main") {
		t.Fatalf("expected SKILL.md content to include '# Main', got %q", payload.SkillMD)
	}

	seen := make(map[string]bool, len(payload.Files))
	for _, f := range payload.Files {
		seen[f] = true
	}
	if !seen["SKILL.md"] {
		t.Fatalf("expected files to include SKILL.md, got %v", payload.Files)
	}
	if !seen["references/notes.md"] {
		t.Fatalf("expected files to include references/notes.md, got %v", payload.Files)
	}
}

func TestReadSkillFile_ReadsFileWithinSkillRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "x")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: x\ndescription: d\n---\n# Main\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "notes.md"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	cfg := &config.Config{Home: home}
	prevCfg := config.AppConfig
	config.AppConfig = cfg
	t.Cleanup(func() { config.AppConfig = prevCfg })

	rt := &runtime.Runtime{Config: cfg}

	router := gin.New()
	router.Use(middleware.InjectRuntime(rt))
	router.GET("/api/skills/:id/file", ReadSkillFile)

	req := httptest.NewRequest(http.MethodGet, "/api/skills/x/file?path=references/notes.md", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Path != "references/notes.md" {
		t.Fatalf("expected path=references/notes.md, got %q", payload.Path)
	}
	if payload.Content != "notes\n" {
		t.Fatalf("expected content 'notes', got %q", payload.Content)
	}
}
