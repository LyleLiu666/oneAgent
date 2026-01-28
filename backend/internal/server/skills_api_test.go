package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SkillsAPI_ListAndArchive(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Seed a personal oneAgent skill.
	skillPath := filepath.Join(home, ".oneagent", "skills", "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: demo-skill\ndescription: demo\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// List skills should include demo-skill and mark it archivable.
	res, err := http.Get(srv.URL + "/api/skills")
	if err != nil {
		t.Fatalf("GET /api/skills: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills status=%d", res.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, it := range list {
		if it["skill_id"] == "demo-skill" {
			found = true
			if it["archivable"] != true {
				t.Fatalf("expected archivable=true, got %+v", it)
			}
		}
	}
	if !found {
		t.Fatalf("expected demo-skill in list")
	}

	// Archive it.
	body := bytes.NewReader([]byte(`{}`))
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/skills/demo-skill/archive", body)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/skills/:id/archive: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/skills/:id/archive status=%d", res.StatusCode)
	}
	var archived map[string]any
	_ = json.NewDecoder(res.Body).Decode(&archived)
	if archived["ok"] != true {
		t.Fatalf("expected ok=true, got %+v", archived)
	}
	if _, err := os.Stat(skillPath); err == nil {
		t.Fatalf("expected original skill to be moved")
	}

	// List again should not include demo-skill.
	res, err = http.Get(srv.URL + "/api/skills")
	if err != nil {
		t.Fatalf("GET /api/skills(2): %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills(2) status=%d", res.StatusCode)
	}
	list = nil
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode list(2): %v", err)
	}
	for _, it := range list {
		if it["skill_id"] == "demo-skill" {
			t.Fatalf("expected demo-skill to be absent after archive")
		}
	}
}

func TestServer_SkillsAPI_GetAndUpdateWithOCC(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	skillPath := filepath.Join(home, ".oneagent", "skills", "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: demo-skill\ndescription: demo\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Get includes sha256 + skill_md.
	res, err := http.Get(srv.URL + "/api/skills/demo-skill")
	if err != nil {
		t.Fatalf("GET /api/skills/:id: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills/:id status=%d", res.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sha := got["sha256"].(string)
	if sha == "" {
		t.Fatalf("expected sha256")
	}

	// Update with wrong sha -> conflict.
	b, _ := json.Marshal(map[string]any{"skill_md": "# x\n", "expected_sha256": "deadbeef"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/skills/demo-skill", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/skills/:id: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", res2.StatusCode)
	}

	// Update with correct sha -> ok.
	b, _ = json.Marshal(map[string]any{"skill_md": "---\nname: demo-skill\n---\nupdated\n", "expected_sha256": sha})
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/skills/demo-skill", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/skills/:id(2): %v", err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res3.StatusCode)
	}
	var updated map[string]any
	_ = json.NewDecoder(res3.Body).Decode(&updated)
	if updated["sha256"] == sha {
		t.Fatalf("expected sha256 to change")
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected skill to exist: %v", err)
	}
}

func TestServer_SkillsAPI_Duplicates(t *testing.T) {
	userHome := t.TempDir()
	oneagentHome := t.TempDir()
	t.Setenv("HOME", userHome)

	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             oneagentHome,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Higher precedence: oneAgent personal.
	p1 := filepath.Join(oneagentHome, ".oneagent", "skills", "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p1), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p1, []byte("---\nname: demo-skill\ndescription: personal\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Lower precedence: ~/.claude.
	p2 := filepath.Join(userHome, ".claude", "skills", "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p2), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p2, []byte("---\nname: demo-skill\ndescription: claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/skills/duplicates")
	if err != nil {
		t.Fatalf("GET /api/skills/duplicates: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills/duplicates status=%d", res.StatusCode)
	}

	var groups []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&groups); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var demo map[string]any
	for _, g := range groups {
		if g["skill_id"] == "demo-skill" {
			demo = g
			break
		}
	}
	if demo == nil {
		t.Fatalf("expected demo-skill duplicates group")
	}

	cands, ok := demo["candidates"].([]any)
	if !ok || len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %#v", demo["candidates"])
	}

	// First candidate should be effective (personal, higher precedence).
	c0 := cands[0].(map[string]any)
	if c0["effective"] != true {
		t.Fatalf("expected first candidate effective=true, got %+v", c0)
	}
	if c0["archivable"] != true {
		t.Fatalf("expected personal candidate archivable=true, got %+v", c0)
	}
}
