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
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/checkpoint"
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
			testReport := filepath.Join(dir, "TEST_REPORT.txt")
			if err := os.WriteFile(findings, []byte("# Findings\n- ok\n"), 0o600); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			if err := os.WriteFile(trace, []byte("{\"type\":\"complete\"}\n"), 0o600); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			if err := os.WriteFile(testReport, []byte("ok\n"), 0o600); err != nil {
				return taskqueue.AttemptResult{}, err
			}
			return taskqueue.AttemptResult{
				RunID:          "run-" + attempt.ID,
				Summary:        "done",
				FindingsPath:   findings,
				TraceLogPath:   trace,
				TestReportPath: testReport,
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
	var last taskqueue.Task
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/tasks/:id: %v", err)
		}
		var got taskqueue.Task
		_ = json.NewDecoder(resp.Body).Decode(&got)
		_ = resp.Body.Close()
		last = got
		if a := got.LatestAttempt(); a != nil && a.Status == taskqueue.AttemptSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if last.LatestAttempt() == nil || last.LatestAttempt().Status != taskqueue.AttemptSucceeded {
		t.Fatalf("expected task to finish, got %+v", last.LatestAttempt())
	}
	if strings.TrimSpace(last.LatestAttempt().TestReportPath) == "" {
		t.Fatalf("expected test_report_path after attempt finished, got %+v", last.LatestAttempt())
	}

	latest := last.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}

	// Seed diff artifacts so the artifact endpoints can serve content (stub runner does not generate them).
	reviewDir := filepath.Join(rt.Layout.TasksDir, created.ID, "attempts", latest.ID, "review")
	if err := os.MkdirAll(reviewDir, 0o700); err != nil {
		t.Fatalf("mkdir reviewDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "diff.patch"), []byte("diff --git a/a b/a\n"), 0o600); err != nil {
		t.Fatalf("write diff.patch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "changed_files.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatalf("write changed_files.txt: %v", err)
	}

	// Artifact endpoints.
	artifactReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/diff_patch", nil)
	artifactRes, err := http.DefaultClient.Do(artifactReq)
	if err != nil {
		t.Fatalf("GET diff_patch: %v", err)
	}
	defer artifactRes.Body.Close()
	if artifactRes.StatusCode != http.StatusOK {
		t.Fatalf("GET diff_patch status=%d", artifactRes.StatusCode)
	}
	var diffResp map[string]any
	if err := json.NewDecoder(artifactRes.Body).Decode(&diffResp); err != nil {
		t.Fatalf("decode diff_patch: %v", err)
	}
	if !strings.Contains(diffResp["content"].(string), "diff --git") {
		t.Fatalf("unexpected diff_patch content: %+v", diffResp)
	}

	changedReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/changed_files", nil)
	changedRes, err := http.DefaultClient.Do(changedReq)
	if err != nil {
		t.Fatalf("GET changed_files: %v", err)
	}
	defer changedRes.Body.Close()
	if changedRes.StatusCode != http.StatusOK {
		t.Fatalf("GET changed_files status=%d", changedRes.StatusCode)
	}
	var changedResp map[string]any
	if err := json.NewDecoder(changedRes.Body).Decode(&changedResp); err != nil {
		t.Fatalf("decode changed_files: %v", err)
	}
	if strings.TrimSpace(changedResp["content"].(string)) == "" {
		t.Fatalf("expected changed_files content, got %+v", changedResp)
	}

	// Review comments (append-only).
	commentBody := map[string]any{"comment": "looks good, please add one more test"}
	commentJSON, _ := json.Marshal(commentBody)
	res, err = http.Post(srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/review_comments", "application/json", bytes.NewReader(commentJSON))
	if err != nil {
		t.Fatalf("POST review_comments: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST review_comments status=%d", res.StatusCode)
	}
	var posted map[string]any
	if err := json.NewDecoder(res.Body).Decode(&posted); err != nil {
		t.Fatalf("decode posted review comment: %v", err)
	}
	if strings.TrimSpace(posted["comment"].(string)) == "" {
		t.Fatalf("expected comment in response, got %+v", posted)
	}

	commentsReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/review_comments", nil)
	res, err = http.DefaultClient.Do(commentsReq)
	if err != nil {
		t.Fatalf("GET review_comments: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET review_comments status=%d", res.StatusCode)
	}
	var comments []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&comments); err != nil {
		t.Fatalf("decode review comments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 review comment, got %d", len(comments))
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

	// Rollback endpoint (best-effort): create a checkpoint, mutate workspace, restore.
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("before\n"), 0o600); err != nil {
		t.Fatalf("write a.txt baseline: %v", err)
	}

	checkpointDir := filepath.Join(rt.Layout.TasksDir, created.ID, "attempts", latest.ID, "checkpoint_test")
	cp, err := checkpoint.CreateWorkspaceCheckpoint(context.Background(), workspace, checkpointDir)
	if err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	_, _ = rt.Tasks.UpdateTask(created.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		if a == nil || a.ID != latest.ID {
			return nil
		}
		a.CheckpointPath = cp.ArchivePath
		return nil
	})

	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("after\n"), 0o600); err != nil {
		t.Fatalf("mutate a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "new.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("write new.txt: %v", err)
	}

	rollbackReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/rollback", nil)
	rollbackRes, err := http.DefaultClient.Do(rollbackReq)
	if err != nil {
		t.Fatalf("POST rollback: %v", err)
	}
	defer rollbackRes.Body.Close()
	if rollbackRes.StatusCode != http.StatusOK {
		t.Fatalf("POST rollback status=%d", rollbackRes.StatusCode)
	}

	gotA, err := os.ReadFile(filepath.Join(workspace, "a.txt"))
	if err != nil {
		t.Fatalf("read restored a.txt: %v", err)
	}
	if string(gotA) != "before\n" {
		t.Fatalf("expected workspace restored by checkpoint, got a.txt=%q", string(gotA))
	}
	if _, err := os.Stat(filepath.Join(workspace, "new.txt")); err == nil {
		t.Fatalf("expected new.txt to be removed on rollback")
	}

	rollbackReq2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/tasks/"+created.ID+"/attempts/"+latest.ID+"/rollback", nil)
	rollbackRes2, err := http.DefaultClient.Do(rollbackReq2)
	if err != nil {
		t.Fatalf("POST rollback (idempotent): %v", err)
	}
	defer rollbackRes2.Body.Close()
	if rollbackRes2.StatusCode != http.StatusOK {
		t.Fatalf("POST rollback (idempotent) status=%d", rollbackRes2.StatusCode)
	}

	eventsAfterRollbackReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tasks/"+created.ID+"/events", nil)
	eventsAfterRollbackRes, err := http.DefaultClient.Do(eventsAfterRollbackReq)
	if err != nil {
		t.Fatalf("GET events after rollback: %v", err)
	}
	defer eventsAfterRollbackRes.Body.Close()
	if eventsAfterRollbackRes.StatusCode != http.StatusOK {
		t.Fatalf("GET events after rollback status=%d", eventsAfterRollbackRes.StatusCode)
	}
	var eventsAfterRollback []taskqueue.Event
	if err := json.NewDecoder(eventsAfterRollbackRes.Body).Decode(&eventsAfterRollback); err != nil {
		t.Fatalf("decode events after rollback: %v", err)
	}
	var sawRollback, sawRepeat bool
	for _, ev := range eventsAfterRollback {
		if ev.AttemptID != latest.ID {
			continue
		}
		switch ev.Type {
		case "attempt.rollback.succeeded":
			sawRollback = true
		case "attempt.rollback.repeat":
			sawRepeat = true
		}
	}
	if !sawRollback || !sawRepeat {
		t.Fatalf("expected rollback audit events, got rollback=%t repeat=%t", sawRollback, sawRepeat)
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
