package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_SOPSuggesionsAPI_Smoke(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)
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

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Proposed/rejected MUST NOT create personal skills.
	if _, err := os.Stat(filepath.Join(home, ".oneagent", "skills")); err == nil {
		t.Fatalf("expected no .oneagent/skills dir before approval")
	}

	// Create suggestion.
	body := map[string]any{
		"title":                "SOP: refactor auth",
		"description":          "Refactor with tests",
		"draft_skill":          "# Skill\n\n## SOP\n1. ...\n",
		"evidence_receipt_ids": []string{"r1", "r2"},
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST sop_suggestions: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d", res.StatusCode)
	}
	var created workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.SuggestionID == "" {
		t.Fatalf("expected suggestion_id")
	}

	// List proposed.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/sop_suggestions?status=proposed", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET sop_suggestions: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status=%d", res.StatusCode)
	}
	var list []workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected at least 1 suggestion")
	}

	// Update status.
	up := map[string]any{
		"status": "rejected",
	}
	b, _ = json.Marshal(up)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/sop_suggestions/"+created.SuggestionID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST status: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status code=%d", res.StatusCode)
	}
	var updated workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Status != workledger.SuggestionStatusRejected {
		t.Fatalf("expected rejected, got %q", updated.Status)
	}
	if _, err := os.Stat(filepath.Join(home, ".oneagent", "skills")); err == nil {
		t.Fatalf("expected no .oneagent/skills dir after rejection")
	}

	// Create another suggestion then merge it into the first.
	body2 := map[string]any{
		"title":                "SOP: refactor auth v2",
		"description":          "Another variant",
		"draft_skill":          "# Skill\n\n## SOP\n1. ...\n",
		"evidence_receipt_ids": []string{"r3"},
	}
	b, _ = json.Marshal(body2)
	res2, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST sop_suggestions 2: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("POST 2 status=%d", res2.StatusCode)
	}
	var created2 workledger.Suggestion
	if err := json.NewDecoder(res2.Body).Decode(&created2); err != nil {
		t.Fatalf("decode created2: %v", err)
	}

	mergeReq := map[string]any{
		"status":                    "merged",
		"merged_into_suggestion_id": created.SuggestionID,
	}
	b, _ = json.Marshal(mergeReq)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/sop_suggestions/"+created2.SuggestionID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST merge: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST merge status=%d", res.StatusCode)
	}
	var merged workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&merged); err != nil {
		t.Fatalf("decode merged: %v", err)
	}
	if merged.Status != workledger.SuggestionStatusMerged {
		t.Fatalf("expected merged status, got %q", merged.Status)
	}

	// Archived MUST NOT create personal skills.
	body3 := map[string]any{
		"title":                "SOP: archived",
		"description":          "Should not materialize",
		"draft_skill":          "# Skill\n\n## SOP\n1. ...\n",
		"evidence_receipt_ids": []string{"r4"},
	}
	b, _ = json.Marshal(body3)
	res3, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST sop_suggestions 3: %v", err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("POST 3 status=%d", res3.StatusCode)
	}
	var created3 workledger.Suggestion
	if err := json.NewDecoder(res3.Body).Decode(&created3); err != nil {
		t.Fatalf("decode created3: %v", err)
	}

	up = map[string]any{"status": "archived"}
	b, _ = json.Marshal(up)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/sop_suggestions/"+created3.SuggestionID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST archive: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST archive status=%d", res.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(home, ".oneagent", "skills")); err == nil {
		t.Fatalf("expected no .oneagent/skills dir after archive")
	}
}

func TestServer_SOPSuggesionsAPI_ApproveMaterializesSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)

	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	prevCfg := config.AppConfig
	config.AppConfig = cfg
	t.Cleanup(func() { config.AppConfig = prevCfg })

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

	// Create suggestion.
	body := map[string]any{
		"title":                "SOP: refactor auth",
		"description":          "Refactor with tests",
		"draft_skill":          "# Skill\n\n## SOP\n1. Add tests\n2. Change code\n",
		"evidence_receipt_ids": []string{"r1", "r2"},
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST sop_suggestions: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d", res.StatusCode)
	}
	var created workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// Approve it.
	up := map[string]any{"status": "approved"}
	b, _ = json.Marshal(up)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/sop_suggestions/"+created.SuggestionID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST approve: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST approve status=%d", res.StatusCode)
	}
	var approved workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved: %v", err)
	}
	if approved.Status != workledger.SuggestionStatusApproved {
		t.Fatalf("expected approved, got %q", approved.Status)
	}
	if approved.Meta.MaterializedSkillID == "" || approved.Meta.MaterializedSkillPath == "" {
		t.Fatalf("expected materialized meta, got %+v", approved.Meta)
	}
	if _, err := os.Stat(approved.Meta.MaterializedSkillPath); err != nil {
		t.Fatalf("expected SKILL.md to exist: %v", err)
	}

	// Ensure skill discovery picks it up.
	cat, err := skill.Discover(context.Background(), skill.DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	got, ok := cat.ByID(approved.Meta.MaterializedSkillID)
	if !ok {
		t.Fatalf("expected skill %q to be discovered", approved.Meta.MaterializedSkillID)
	}
	if got.Source != skill.SourceOneAgent {
		t.Fatalf("expected source=.oneagent, got %+v", got)
	}
	metaPath := approved.Meta.MaterializedSkillPath
	gotPath := got.Path
	if resolved, err := filepath.EvalSymlinks(metaPath); err == nil && resolved != "" {
		metaPath = resolved
	}
	if resolved, err := filepath.EvalSymlinks(gotPath); err == nil && resolved != "" {
		gotPath = resolved
	}
	if metaPath != gotPath {
		t.Fatalf("expected path=%q, got %q", approved.Meta.MaterializedSkillPath, got.Path)
	}
}

func TestServer_SOPSuggesionsAPI_EditThenApproveUsesEditedDraft(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)

	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	prevCfg := config.AppConfig
	config.AppConfig = cfg
	t.Cleanup(func() { config.AppConfig = prevCfg })

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

	body := map[string]any{
		"title":                "SOP: refactor auth",
		"description":          "Refactor with tests",
		"draft_skill":          "# Skill\n\n## SOP\n1. Old\n",
		"evidence_receipt_ids": []string{"r1", "r2"},
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST sop_suggestions: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d", res.StatusCode)
	}
	var created workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// Edit before approval.
	edit := map[string]any{
		"title":       "SOP: refactor auth (edited)",
		"draft_skill": "# Skill\n\n## SOP\n1. Edited\n",
	}
	b, _ = json.Marshal(edit)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/ledger/sop_suggestions/"+created.SuggestionID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT edit: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PUT edit status=%d", res.StatusCode)
	}
	var edited workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&edited); err != nil {
		t.Fatalf("decode edited: %v", err)
	}
	if edited.Title != "SOP: refactor auth (edited)" {
		t.Fatalf("expected edited title, got %q", edited.Title)
	}
	if edited.DraftSkill != "# Skill\n\n## SOP\n1. Edited" {
		t.Fatalf("expected edited draft_skill, got %q", edited.DraftSkill)
	}

	// Approve uses edited draft.
	up := map[string]any{"status": "approved"}
	b, _ = json.Marshal(up)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/ledger/sop_suggestions/"+created.SuggestionID+"/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST approve: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST approve status=%d", res.StatusCode)
	}
	var approved workledger.Suggestion
	if err := json.NewDecoder(res.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved: %v", err)
	}
	if approved.Meta.MaterializedSkillPath == "" {
		t.Fatalf("expected materialized path")
	}
	data, err := os.ReadFile(approved.Meta.MaterializedSkillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !bytes.Contains(data, []byte("1. Edited")) {
		t.Fatalf("expected materialized SKILL.md to contain edited draft, got:\n%s", string(data))
	}

	// Editing after approval is rejected (avoid drift vs materialized skill).
	edit2 := map[string]any{"draft_skill": "# Skill\n\n## SOP\n1. Should fail\n"}
	b, _ = json.Marshal(edit2)
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/ledger/sop_suggestions/"+created.SuggestionID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT edit after approve: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatalf("expected edit after approve to fail")
	}
}
