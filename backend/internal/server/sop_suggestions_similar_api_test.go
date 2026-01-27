package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SOPSuggesionsSimilarAPI_Smoke(t *testing.T) {
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

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Create 2 similar suggestions.
	body := map[string]any{
		"title":               "SOP: refactor auth",
		"draft_skill":         "# Skill\n\n## SOP\n1. refactor auth\n2. run tests\n",
		"evidence_receipt_ids": []string{"r1"},
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST s1: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST s1 status=%d", res.StatusCode)
	}
	var s1 map[string]any
	_ = json.NewDecoder(res.Body).Decode(&s1)
	id1, _ := s1["suggestion_id"].(string)
	if id1 == "" {
		t.Fatalf("expected suggestion_id")
	}

	body2 := map[string]any{
		"title":               "SOP: refactor auth flow",
		"draft_skill":         "# Skill\n\n## SOP\n1. refactor auth\n2. add tests\n",
		"evidence_receipt_ids": []string{"r2"},
	}
	b, _ = json.Marshal(body2)
	res2, err := http.Post(srv.URL+"/api/ledger/sop_suggestions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST s2: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("POST s2 status=%d", res2.StatusCode)
	}

	// Similar endpoint should return at least one match.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/ledger/sop_suggestions/"+id1+"/similar?limit=5", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET similar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET similar status=%d", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) < 1 {
		t.Fatalf("expected at least 1 similar suggestion")
	}
}

