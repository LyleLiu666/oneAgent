package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_SOPSuggesionsAPI_Smoke(t *testing.T) {
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

	// Create suggestion.
	body := map[string]any{
		"title":               "SOP: refactor auth",
		"description":         "Refactor with tests",
		"draft_skill":         "# Skill\n\n## SOP\n1. ...\n",
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

	// Create another suggestion then merge it into the first.
	body2 := map[string]any{
		"title":               "SOP: refactor auth v2",
		"description":         "Another variant",
		"draft_skill":         "# Skill\n\n## SOP\n1. ...\n",
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
		"status":                      "merged",
		"merged_into_suggestion_id":   created.SuggestionID,
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
}
