package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestServer_TaskQueueAPI_Smoke(t *testing.T) {
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_TOTAL_TOKENS", "1000")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_COST_USD", "1.25")

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

	// Stub runner so API tests don't require a real LLM/provider.
	artifactsDir := t.TempDir()
	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, _ *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			dir := filepath.Join(artifactsDir, task.ID, attempt.ID)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			findings := filepath.Join(dir, "FINDINGS.md")
			trace := filepath.Join(dir, "trace.jsonl")
			if err := os.WriteFile(findings, []byte("# Findings\n- ok\n"), 0o600); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			if err := os.WriteFile(trace, []byte("{\"type\":\"complete\"}\n"), 0o600); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			return taskqueue.AttemptResult{
				RunID:        "run-" + attempt.ID,
				Summary:      "done",
				FindingsPath: findings,
				TraceLogPath: trace,
			}, nil
		},
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
	}
	if err := rt.TaskRunner.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	workspace := t.TempDir()

	body := map[string]any{
		"workspace": workspace,
		"title":     "T1",
		"prompt":    "do the thing",
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/tasks", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /api/tasks: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/tasks status=%d", res.StatusCode)
	}

	var created taskqueue.Task
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create task: %v", err)
	}
	if created.ID == "" || created.Workspace == "" {
		t.Fatalf("unexpected task: %+v", created)
	}
	if created.Limits.MaxSteps <= 0 || created.Limits.MaxRuntimeSeconds <= 0 {
		t.Fatalf("expected default limits to be applied, got %+v", created.Limits)
	}
	if created.Limits.MaxTotalTokens != 1000 || created.Limits.MaxCostUSD != 1.25 {
		t.Fatalf("expected default budgets to be applied, got %+v", created.Limits)
	}

	// Wait for background run to finish (runner is async).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/tasks/:id: %v", err)
		}
		var got taskqueue.Task
		_ = json.NewDecoder(resp.Body).Decode(&got)
		_ = resp.Body.Close()
		if a := got.LatestAttempt(); a != nil && a.Status == taskqueue.AttemptSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// List by workspace.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks?workspace="+url.QueryEscape(workspace), nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/tasks: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tasks status=%d", res.StatusCode)
	}
	var list []taskqueue.Task
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode list tasks: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected at least 1 task")
	}

	// Events.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID+"/events", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/tasks/:id/events: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tasks/:id/events status=%d", res.StatusCode)
	}
	var evs []taskqueue.Event
	if err := json.NewDecoder(res.Body).Decode(&evs); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(evs) == 0 {
		t.Fatalf("expected events")
	}

	// Create task with explicit budgets.
	body = map[string]any{
		"workspace": workspace,
		"title":     "T2",
		"prompt":    "do the other thing",
		"limits": map[string]any{
			"max_total_tokens": 55,
			"max_cost_usd":     0.75,
		},
	}
	b, _ = json.Marshal(body)
	res, err = http.Post(srv.URL+"/api/tasks", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /api/tasks (explicit budgets): %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/tasks (explicit budgets) status=%d", res.StatusCode)
	}
	var created2 taskqueue.Task
	if err := json.NewDecoder(res.Body).Decode(&created2); err != nil {
		t.Fatalf("decode create task (explicit budgets): %v", err)
	}
	if created2.Limits.MaxTotalTokens != 55 || created2.Limits.MaxCostUSD != 0.75 {
		t.Fatalf("expected explicit budgets to roundtrip, got %+v", created2.Limits)
	}
}
