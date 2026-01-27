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
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_LedgerStatusTodayAPI_ReadOnlySummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

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

	dayKey := workledger.DayKey(time.Now())
	digestPath := rt.WorkLedger.DigestPath("local", dayKey)

	// Before any data exists: endpoint must be read-only and must not create digest.
	{
		res, err := http.Get(srv.URL + "/api/ledger/status/today")
		if err != nil {
			t.Fatalf("GET status/today: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET status=%d", res.StatusCode)
		}

		var out map[string]any
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if out["day_key"] != dayKey {
			t.Fatalf("expected day_key=%q, got %v", dayKey, out["day_key"])
		}
		if out["digest_exists"] != false {
			t.Fatalf("expected digest_exists=false, got %v", out["digest_exists"])
		}
		if out["learning_job_status"] != "none" {
			t.Fatalf("expected learning_job_status=none, got %v", out["learning_job_status"])
		}
		if out["sop_proposed_count"] != float64(0) {
			t.Fatalf("expected sop_proposed_count=0, got %v", out["sop_proposed_count"])
		}

		if _, err := os.Stat(digestPath); err == nil {
			t.Fatalf("expected digest to NOT be created by status endpoint")
		}
	}

	// Seed data and verify summary reflects it.
	if _, err := rt.WorkLedger.CreateSuggestion(workledger.CreateSuggestionInput{
		PrincipalID:         "local",
		WorkspaceRoot:       "/tmp/ws",
		Title:               "SOP: do X",
		Description:         "desc",
		EvidenceReceiptIDs:  []string{"r1"},
		DraftSkill:          "# Skill\n\n1. ...\n",
		Scores:              workledger.SuggestionScores{TotalScore: 1},
		Meta:                workledger.SuggestionMeta{DayKey: dayKey},
	}); err != nil {
		t.Fatalf("create suggestion: %v", err)
	}

	// Ensure digest file exists (simulates user having generated it previously).
	if err := os.MkdirAll(filepath.Dir(digestPath), 0o700); err != nil {
		t.Fatalf("mkdir digest dir: %v", err)
	}
	if err := os.WriteFile(digestPath, []byte("# digest\n"), 0o644); err != nil {
		t.Fatalf("write digest: %v", err)
	}

	if err := rt.WorkLedger.WriteLearningJob(workledger.LearningJob{
		JobID:       "j1",
		PrincipalID: "local",
		DayKey:      dayKey,
		Status:      workledger.LearningJobStatusRunning,
		StartedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write job: %v", err)
	}

	res, err := http.Get(srv.URL + "/api/ledger/status/today")
	if err != nil {
		t.Fatalf("GET status/today 2: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET2 status=%d", res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response2: %v", err)
	}
	if out["digest_exists"] != true {
		t.Fatalf("expected digest_exists=true, got %v", out["digest_exists"])
	}
	if out["learning_job_status"] != string(workledger.LearningJobStatusRunning) {
		t.Fatalf("expected learning_job_status=%q, got %v", workledger.LearningJobStatusRunning, out["learning_job_status"])
	}
	if out["sop_proposed_count"] != float64(1) {
		t.Fatalf("expected sop_proposed_count=1, got %v", out["sop_proposed_count"])
	}
}
