package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestServer_LearningJobsAPI_Smoke(t *testing.T) {
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

	// Best-effort: job may or may not exist depending on scheduler timing.
	res, err := http.Get(srv.URL + "/api/ledger/learning/jobs/today")
	if err != nil {
		t.Fatalf("GET today: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected GET status=%d", res.StatusCode)
	}

	// Manual run always returns OK and a job payload.
	res, err = http.Post(srv.URL+"/api/ledger/learning/jobs/run_today", "application/json", nil)
	if err != nil {
		t.Fatalf("POST run_today: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d", res.StatusCode)
	}

	// Verify job exists on disk (source of truth).
	dayKey := workledger.DayKey(time.Now())
	_, err = rt.WorkLedger.GetLearningJob("local", dayKey)
	if err != nil {
		t.Fatalf("GetLearningJob: %v", err)
	}
}
