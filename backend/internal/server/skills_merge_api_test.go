package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_Skills_PinAndArchiveShadowed(t *testing.T) {
	oneAgentHome := t.TempDir()
	userHome := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("ONEAGENT_HOME", oneAgentHome)

	// Create a Claude candidate skill under the workspace (workspaceRoot/.claude).
	claudeSkillDir := filepath.Join(workspaceRoot, ".claude", "skills", "foo")
	if err := os.MkdirAll(claudeSkillDir, 0o700); err != nil {
		t.Fatalf("mkdir claude skill dir: %v", err)
	}
	claudeSkillPath := filepath.Join(claudeSkillDir, "SKILL.md")
	claudeContent := []byte(`---
name: foo
description: from claude
---

# foo
`)
	if err := os.WriteFile(claudeSkillPath, claudeContent, 0o600); err != nil {
		t.Fatalf("write claude SKILL.md: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(claudeSkillPath); err == nil && resolved != "" {
		claudeSkillPath = resolved
	}

	// Create a shadowed personal duplicate (different folder, same skill id via frontmatter).
	shadowDir := filepath.Join(oneAgentHome, ".oneagent", "skills", "foo-old")
	if err := os.MkdirAll(shadowDir, 0o700); err != nil {
		t.Fatalf("mkdir shadow dir: %v", err)
	}
	shadowPath := filepath.Join(shadowDir, "SKILL.md")
	shadowContent := []byte(`---
name: foo
description: old personal duplicate
---

# foo old
`)
	if err := os.WriteFile(shadowPath, shadowContent, 0o600); err != nil {
		t.Fatalf("write shadow SKILL.md: %v", err)
	}

	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             oneAgentHome,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	rt, err := runtime.Init(cfg)
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

	// Discover duplicates and pin the claude candidate -> personal canonical.
	resp, err := http.Get(srv.URL + "/api/skills/duplicates?workspace=" + url.QueryEscape(workspaceRoot))
	if err != nil {
		t.Fatalf("GET /api/skills/duplicates: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills/duplicates status=%d", resp.StatusCode)
	}
	var groups []struct {
		SkillID    string `json:"skill_id"`
		Candidates []struct {
			SkillID string `json:"skill_id"`
			Source  string `json:"source"`
			Path    string `json:"path"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		t.Fatalf("decode duplicates: %v", err)
	}
	var pickedPath string
	for _, g := range groups {
		if g.SkillID != "foo" {
			continue
		}
		for _, c := range g.Candidates {
			if c.Source == ".claude" && samePath(c.Path, claudeSkillPath) {
				pickedPath = c.Path
				break
			}
		}
	}
	if pickedPath == "" {
		t.Logf("groups=%+v", groups)
		t.Fatalf("expected to find claude candidate in duplicates")
	}

	body := map[string]any{
		"source":                    ".claude",
		"path":                      pickedPath,
		"workspace_root":            workspaceRoot,
		"archive_shadowed_personal": true,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/skills/foo/pin", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/skills/foo/pin: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/skills/foo/pin status=%d", res.StatusCode)
	}
	var pinResp struct {
		OK            bool     `json:"ok"`
		CanonicalPath string   `json:"canonical_path"`
		ArchivedPaths []string `json:"archived_paths"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pinResp); err != nil {
		t.Fatalf("decode pin resp: %v", err)
	}
	if !pinResp.OK || pinResp.CanonicalPath == "" {
		t.Fatalf("unexpected pin resp: %+v", pinResp)
	}
	// Canonical content should match pinned candidate.
	data, err := os.ReadFile(pinResp.CanonicalPath)
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	if string(data) != string(claudeContent) {
		t.Fatalf("canonical content mismatch")
	}
	// Shadowed personal duplicate should be archived (moved).
	if _, err := os.Stat(shadowDir); err == nil {
		t.Fatalf("expected shadow dir to be moved/archived")
	}
	if len(pinResp.ArchivedPaths) == 0 {
		t.Fatalf("expected archived_paths")
	}

	// Pin again should be idempotent (still OK, same canonical path).
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/api/skills/foo/pin", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST /api/skills/foo/pin (2): %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/skills/foo/pin (2) status=%d", res2.StatusCode)
	}
	var pinResp2 struct {
		CanonicalPath string `json:"canonical_path"`
	}
	_ = json.NewDecoder(res2.Body).Decode(&pinResp2)
	if pinResp2.CanonicalPath != pinResp.CanonicalPath {
		t.Fatalf("expected canonical path stable, got %q vs %q", pinResp2.CanonicalPath, pinResp.CanonicalPath)
	}
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	if ra, err := filepath.EvalSymlinks(a); err == nil && ra != "" {
		a = filepath.Clean(ra)
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil && rb != "" {
		b = filepath.Clean(rb)
	}
	return a == b
}
