package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestServer_TaskQueue_EndToEnd_FIFO_Cancel_Resume(t *testing.T) {
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

	artifactsDir := t.TempDir()
	blockA := make(chan struct{})
	started := struct {
		mu    sync.Mutex
		tasks map[string]int
	}{tasks: map[string]int{}}

	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			started.mu.Lock()
			started.tasks[task.Title]++
			started.mu.Unlock()

			if task.Title == "A" {
				select {
				case <-blockA:
				case <-ctx.Done():
					return taskqueue.AttemptResult{}, ctx.Err()
				}
			}

			dir := filepath.Join(artifactsDir, task.ID, attempt.ID)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			findings := filepath.Join(dir, "FINDINGS.md")
			trace := filepath.Join(dir, "trace.jsonl")
			_ = os.WriteFile(findings, []byte("# Findings\n- ok\n"), 0o600)
			_ = os.WriteFile(trace, []byte("{\"type\":\"complete\"}\n"), 0o600)

			if task.Title == "C" && resumedFrom == nil {
				return taskqueue.AttemptResult{
					RunID:        "run-" + attempt.ID,
					Summary:      "failed",
					FindingsPath: findings,
					TraceLogPath: trace,
				}, errors.New("simulated failure")
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

	taskA := postTask(t, srv.URL, workspace, "A", "task A")
	taskB := postTask(t, srv.URL, workspace, "B", "task B")

	waitForTaskAttemptStatusHTTP(t, srv.URL, taskA.ID, taskqueue.AttemptRunning, 2*time.Second)

	gotB := getTaskHTTP(t, srv.URL, taskB.ID)
	if gotB.LatestAttempt() == nil || gotB.LatestAttempt().Status != taskqueue.AttemptQueued {
		t.Fatalf("expected B queued while A running, got %+v", gotB.LatestAttempt())
	}

	cancelTaskHTTP(t, srv.URL, taskB.ID)
	close(blockA)

	waitForTaskAttemptStatusHTTP(t, srv.URL, taskA.ID, taskqueue.AttemptSucceeded, 2*time.Second)
	waitForTaskAttemptStatusHTTP(t, srv.URL, taskB.ID, taskqueue.AttemptCanceled, 2*time.Second)

	started.mu.Lock()
	bRuns := started.tasks["B"]
	started.mu.Unlock()
	if bRuns != 0 {
		t.Fatalf("expected task B never executed after cancel, got runs=%d", bRuns)
	}

	taskC := postTask(t, srv.URL, workspace, "C", "task C")
	waitForTaskAttemptStatusHTTP(t, srv.URL, taskC.ID, taskqueue.AttemptFailed, 2*time.Second)

	resumeTaskHTTP(t, srv.URL, taskC.ID)
	waitForTaskAttemptStatusHTTP(t, srv.URL, taskC.ID, taskqueue.AttemptSucceeded, 2*time.Second)

	finalC := getTaskHTTP(t, srv.URL, taskC.ID)
	if len(finalC.Attempts) != 2 {
		t.Fatalf("expected 2 attempts for resumed task, got %d", len(finalC.Attempts))
	}
}

func postTask(t *testing.T, baseURL, workspace, title, prompt string) taskqueue.Task {
	t.Helper()

	payload := map[string]any{
		"workspace": workspace,
		"title":     title,
		"prompt":    prompt,
	}
	b, _ := json.Marshal(payload)
	res, err := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(b))
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
	return created
}

func getTaskHTTP(t *testing.T, baseURL, taskID string) taskqueue.Task {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/tasks/"+taskID, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/tasks/:id: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tasks/:id status=%d", res.StatusCode)
	}
	var got taskqueue.Task
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	return got
}

func cancelTaskHTTP(t *testing.T, baseURL, taskID string) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/tasks/"+taskID+"/cancel", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/tasks/:id/cancel: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/tasks/:id/cancel status=%d", res.StatusCode)
	}
}

func resumeTaskHTTP(t *testing.T, baseURL, taskID string) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/tasks/"+taskID+"/resume", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/tasks/:id/resume: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/tasks/:id/resume status=%d", res.StatusCode)
	}
}

func waitForTaskAttemptStatusHTTP(t *testing.T, baseURL, taskID string, want taskqueue.AttemptStatus, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got := getTaskHTTP(t, baseURL, taskID)
		if a := got.LatestAttempt(); a != nil && a.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got := getTaskHTTP(t, baseURL, taskID)
	t.Fatalf("timeout waiting for status %q; got %+v", want, got.LatestAttempt())
}
