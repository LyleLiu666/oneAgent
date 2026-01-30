package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SkillsAPI_ListStale(t *testing.T) {
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

	oldSkill := filepath.Join(home, ".oneagent", "skills", "old-skill", "SKILL.md")
	newSkill := filepath.Join(home, ".oneagent", "skills", "new-skill", "SKILL.md")

	for _, p := range []string{oldSkill, newSkill} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("---\nname: "+filepath.Base(filepath.Dir(p))+"\ndescription: demo\n---\nbody\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Mark old-skill as very old via file mtime.
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldSkill, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/skills/stale?days=1")
	if err != nil {
		t.Fatalf("GET /api/skills/stale: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/skills/stale status=%d", res.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}

	foundOld := false
	foundNew := false
	for _, it := range list {
		if it["skill_id"] == "old-skill" {
			foundOld = true
		}
		if it["skill_id"] == "new-skill" {
			foundNew = true
		}
	}
	if !foundOld {
		t.Fatalf("expected old-skill to be listed as stale")
	}
	if foundNew {
		t.Fatalf("expected new-skill to not be listed as stale")
	}
}

