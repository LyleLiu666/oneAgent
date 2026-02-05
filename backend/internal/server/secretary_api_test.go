package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestServer_SecretaryInboxAndTriage_SmokeAndIdempotency(t *testing.T) {
	poolRoot := t.TempDir()
	t.Setenv("ONEAGENT_WORKSPACE_POOL_DIR", poolRoot)

	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Stub task runner so tests don't require a real subagent/tool environment.
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

	// Mock OpenAI-compatible SSE endpoint for ack/triage.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		system := ""
		for _, m := range req.Messages {
			if m.Role == "system" && system == "" {
				system = m.Content
			}
		}

		switch {
		case strings.Contains(system, "ONEAGENT_SECRETARY_ACK"):
			http.Error(w, "unexpected ack request (quick-ack is disabled)", http.StatusBadRequest)
			return
		case strings.Contains(system, "ONEAGENT_SECRETARY_TRIAGE"):
			args := `{"intent":"dispatch","summary_message":"我理解为 2 件事：①整理一份报告；②跑 backend 测试。我已分别安排 worker。","tasks":[{"title":"整理报告","prompt":"整理一份报告，输出 report.md，并确保内容结构清晰。","workspace_strategy":"new"},{"title":"跑 backend 测试","prompt":"在 repo 内运行 go test ./...，如失败请修复并补齐测试。","workspace_strategy":"session"}],"questions":[]}`
			resp := map[string]any{
				"id": "cmpl-test",
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
							"tool_calls": []any{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "secretary_triage_plan",
										"arguments": args,
									},
								},
							},
						},
						"finish_reason": "stop",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "unexpected system prompt", http.StatusBadRequest)
			return
		}
	}))
	t.Cleanup(mock.Close)

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)

	workspace := t.TempDir()

	type inboxResp struct {
		SessionID     string `json:"session_id"`
		MessageID     uint   `json:"message_id"`
		AckMessageID  uint   `json:"ack_message_id"`
		AckText       string `json:"ack_text"`
		Error         string `json:"error"`
		AckMessageRaw any    `json:"-"`
	}

	var r1 inboxResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content":   "帮我整理一份报告",
		"workspace": workspace,
	}, &r1)
	if strings.TrimSpace(r1.SessionID) == "" || r1.MessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", r1)
	}
	if r1.AckMessageID != 0 || strings.TrimSpace(r1.AckText) != "" {
		t.Fatalf("expected empty ack, got ack_message_id=%d ack_text=%q", r1.AckMessageID, r1.AckText)
	}

	var r2 inboxResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"session_id": r1.SessionID,
		"content":    "另外也跑一下 backend 测试",
	}, &r2)
	if r2.SessionID != r1.SessionID || r2.MessageID == 0 {
		t.Fatalf("unexpected inbox response 2: %+v", r2)
	}
	if r2.AckMessageID != 0 || strings.TrimSpace(r2.AckText) != "" {
		t.Fatalf("expected empty ack, got ack_message_id=%d ack_text=%q", r2.AckMessageID, r2.AckText)
	}

	// Verify messages are persisted and no quick-ack assistant message is inserted.
	sessionBody := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/session")
	var sess struct {
		ID       string `json:"id"`
		Messages []struct {
			ID       uint   `json:"id"`
			Role     string `json:"role"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			ParentID *uint  `json:"parent_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(sessionBody, &sess); err != nil {
		t.Fatalf("unmarshal session: %v", err)
	}
	if sess.ID != r1.SessionID {
		t.Fatalf("expected secretary session id=%q, got %+v", r1.SessionID, sess)
	}
	userCount := 0
	assistantCount := 0
	for _, m := range sess.Messages {
		switch m.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		}
	}
	if userCount != 2 || assistantCount != 0 {
		t.Fatalf("expected 2 user messages and 0 assistant messages before triage, got user=%d assistant=%d", userCount, assistantCount)
	}

	type triageResp struct {
		SummaryMessage    string   `json:"summary_message"`
		SummaryMessageID  uint     `json:"summary_message_id"`
		CursorMessageID   uint     `json:"cursor_message_id"`
		CreatedTaskIDs    []string `json:"created_task_ids"`
		Questions         []string `json:"questions"`
		WorkspacesCreated []string `json:"workspaces_created"`
	}

	var tr1 triageResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/triage", map[string]any{
		"session_id": r1.SessionID,
	}, &tr1)
	if strings.TrimSpace(tr1.SummaryMessage) == "" || tr1.SummaryMessageID == 0 {
		t.Fatalf("unexpected triage response: %+v", tr1)
	}
	if tr1.CursorMessageID == 0 {
		t.Fatalf("expected updated cursor, got %+v", tr1)
	}
	if len(tr1.CreatedTaskIDs) != 2 {
		t.Fatalf("expected 2 created tasks, got %+v", tr1.CreatedTaskIDs)
	}
	if len(tr1.WorkspacesCreated) != 1 {
		t.Fatalf("expected 1 workspace created, got %+v", tr1.WorkspacesCreated)
	}
	poolReal, _ := filepath.EvalSymlinks(poolRoot)
	wsReal, _ := filepath.EvalSymlinks(tr1.WorkspacesCreated[0])
	poolPrefix := filepath.Clean(poolReal) + string(os.PathSeparator)
	wsPath := filepath.Clean(wsReal) + string(os.PathSeparator)
	if !strings.HasPrefix(wsPath, poolPrefix) {
		t.Fatalf("expected workspace under pool root=%s (real=%s), got %s (real=%s)", poolRoot, poolReal, tr1.WorkspacesCreated[0], wsReal)
	}

	// Idempotency: re-triage the same range should not create more tasks.
	before, _ := rt.Tasks.ListTasks("", "")

	var tr2 triageResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/triage", map[string]any{
		"session_id":        r1.SessionID,
		"cursor_message_id": 0,
	}, &tr2)
	if len(tr2.CreatedTaskIDs) != len(tr1.CreatedTaskIDs) {
		t.Fatalf("expected same tasks on retry, got %+v vs %+v", tr2.CreatedTaskIDs, tr1.CreatedTaskIDs)
	}
	after, _ := rt.Tasks.ListTasks("", "")
	if len(after) != len(before) {
		t.Fatalf("expected no new tasks on retry, got before=%d after=%d", len(before), len(after))
	}

	// Summary message should be appended once.
	body2 := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/session")
	var sess2 struct {
		ID       string `json:"id"`
		Messages []struct {
			ID      uint   `json:"id"`
			Role    string `json:"role"`
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body2, &sess2); err != nil {
		t.Fatalf("unmarshal session2: %v", err)
	}
	if sess2.ID != r1.SessionID {
		t.Fatalf("expected secretary session id=%q, got %+v", r1.SessionID, sess2)
	}
	summaryCount := 0
	for _, m := range sess2.Messages {
		if m.Role == "assistant" && m.Type == "text" && strings.TrimSpace(m.Content) == strings.TrimSpace(tr1.SummaryMessage) {
			summaryCount++
		}
	}
	if summaryCount != 1 {
		t.Fatalf("expected 1 summary message, got %d", summaryCount)
	}

	// State endpoint should restore cursor and evidence.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/secretary/state", nil)
	if err != nil {
		t.Fatalf("new state request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET state: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("GET state status=%d body=%s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	var st struct {
		CursorMessageID uint `json:"cursor_message_id"`
		TriageRuns      []struct {
			FromCursor     uint     `json:"from_cursor"`
			ToMessageID    uint     `json:"to_message_id"`
			SummaryMessage string   `json:"summary_message"`
			CreatedTaskIDs []string `json:"created_task_ids"`
		} `json:"triage_runs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if st.CursorMessageID != tr1.CursorMessageID {
		t.Fatalf("expected cursor=%d, got %d", tr1.CursorMessageID, st.CursorMessageID)
	}
	if len(st.TriageRuns) == 0 {
		t.Fatalf("expected triage_runs in state")
	}
}

func TestServer_SecretaryTriage_ProgressQuery_ReturnsWorkspaceStats(t *testing.T) {
	poolRoot := t.TempDir()
	t.Setenv("ONEAGENT_WORKSPACE_POOL_DIR", poolRoot)

	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Minimal task runner to satisfy secretary triage dependencies.
	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, _ *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			return taskqueue.AttemptResult{}, nil
		},
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
	}
	if err := rt.TaskRunner.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}

	// Prepare an ephemeral workspace under pool root so progress query can scan files.
	workspace := filepath.Join(poolRoot, "ws-progress")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "chapter1.md"), []byte(strings.Repeat("a", 12000)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Create a queued task in the same workspace.
	if _, err := rt.Tasks.CreateTask("local", workspace, "write", "p", "", taskqueue.Limits{}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Mock OpenAI-compatible endpoint and configure a default model, since secretary triage must be LLM-driven.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		system := ""
		all := strings.Builder{}
		for _, m := range req.Messages {
			if m.Role == "system" && system == "" {
				system = m.Content
			}
			all.WriteString(m.Content)
			all.WriteString("\n")
		}

		switch {
		case strings.Contains(system, "ONEAGENT_SECRETARY_ACK"):
			http.Error(w, "unexpected ack request (quick-ack is disabled)", http.StatusBadRequest)
			return
		case strings.Contains(system, "ONEAGENT_SECRETARY_TRIAGE"):
			combined := all.String()
			if !strings.Contains(combined, "排队 1") {
				http.Error(w, "missing queued-count snapshot", http.StatusBadRequest)
				return
			}
			if !strings.Contains(combined, "一共有") || !strings.Contains(combined, "文件") || !strings.Contains(combined, "字") {
				http.Error(w, "missing workspace stats snapshot", http.StatusBadRequest)
				return
			}

			args := `{"intent":"progress","summary_message":"排队 1。这个文件夹一共有 1 个文件，约 12000 字。","tasks":[],"task_actions":[],"questions":[]}`
			resp := map[string]any{
				"id": "cmpl-test",
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
							"tool_calls": []any{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "secretary_triage_plan",
										"arguments": args,
									},
								},
							},
						},
						"finish_reason": "stop",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "unexpected system prompt", http.StatusBadRequest)
			return
		}
	}))
	t.Cleanup(mock.Close)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)

	var inboxResp struct {
		SessionID string `json:"session_id"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content":   "写了多少了？",
		"workspace": workspace,
	}, &inboxResp)
	if strings.TrimSpace(inboxResp.SessionID) == "" {
		t.Fatalf("expected session id")
	}

	var triageResp struct {
		SummaryMessage    string   `json:"summary_message"`
		CreatedTaskIDs    []string `json:"created_task_ids"`
		WorkspacesCreated []string `json:"workspaces_created"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/triage", map[string]any{
		"session_id": inboxResp.SessionID,
	}, &triageResp)

	if len(triageResp.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no tasks dispatched for progress query, got %+v", triageResp.CreatedTaskIDs)
	}
	if len(triageResp.WorkspacesCreated) != 0 {
		t.Fatalf("expected no workspaces created for progress query, got %+v", triageResp.WorkspacesCreated)
	}
	if !strings.Contains(triageResp.SummaryMessage, "排队 1") {
		t.Fatalf("expected summary to include queued count, got %q", triageResp.SummaryMessage)
	}
	if !strings.Contains(triageResp.SummaryMessage, "一共有") || !strings.Contains(triageResp.SummaryMessage, "文件") || !strings.Contains(triageResp.SummaryMessage, "字") {
		t.Fatalf("expected summary to include workspace stats, got %q", triageResp.SummaryMessage)
	}
	if strings.Contains(strings.ToLower(triageResp.SummaryMessage), "workspace") {
		t.Fatalf("expected summary to avoid internal term 'workspace', got %q", triageResp.SummaryMessage)
	}
}

func TestServer_SecretaryTriage_ProgressQuery_NoWorkspace_StillReturnsStats(t *testing.T) {
	poolRoot := t.TempDir()
	t.Setenv("ONEAGENT_WORKSPACE_POOL_DIR", poolRoot)

	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Minimal task runner to satisfy secretary triage dependencies.
	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, _ *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			return taskqueue.AttemptResult{}, nil
		},
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
	}
	if err := rt.TaskRunner.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}

	// Prepare an ephemeral workspace under pool root so progress query can scan files.
	workspace := filepath.Join(poolRoot, "ws-progress-no-workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "chapter1.md"), []byte(strings.Repeat("a", 12000)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Create a queued task in the same workspace.
	if _, err := rt.Tasks.CreateTask("local", workspace, "write", "p", "", taskqueue.Limits{}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Mock OpenAI-compatible endpoint and configure a default model, since secretary triage must be LLM-driven.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		system := ""
		all := strings.Builder{}
		for _, m := range req.Messages {
			if m.Role == "system" && system == "" {
				system = m.Content
			}
			all.WriteString(m.Content)
			all.WriteString("\n")
		}

		switch {
		case strings.Contains(system, "ONEAGENT_SECRETARY_ACK"):
			http.Error(w, "unexpected ack request (quick-ack is disabled)", http.StatusBadRequest)
			return
		case strings.Contains(system, "ONEAGENT_SECRETARY_TRIAGE"):
			combined := all.String()
			if !strings.Contains(combined, "排队 1") {
				http.Error(w, "missing queued-count snapshot", http.StatusBadRequest)
				return
			}
			if !strings.Contains(combined, "一共有") || !strings.Contains(combined, "文件") || !strings.Contains(combined, "字") {
				http.Error(w, "missing workspace stats snapshot", http.StatusBadRequest)
				return
			}

			args := `{"intent":"progress","summary_message":"排队 1。这个文件夹一共有 1 个文件，约 12000 字。","tasks":[],"task_actions":[],"questions":[]}`
			resp := map[string]any{
				"id": "cmpl-test",
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
							"tool_calls": []any{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "secretary_triage_plan",
										"arguments": args,
									},
								},
							},
						},
						"finish_reason": "stop",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "unexpected system prompt", http.StatusBadRequest)
			return
		}
	}))
	t.Cleanup(mock.Close)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)

	var inboxResp struct {
		SessionID string `json:"session_id"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "写了多少了？",
	}, &inboxResp)
	if strings.TrimSpace(inboxResp.SessionID) == "" {
		t.Fatalf("expected session id")
	}

	var triageResp struct {
		SummaryMessage    string   `json:"summary_message"`
		CreatedTaskIDs    []string `json:"created_task_ids"`
		WorkspacesCreated []string `json:"workspaces_created"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/triage", map[string]any{
		"session_id": inboxResp.SessionID,
	}, &triageResp)

	if len(triageResp.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no tasks dispatched for progress query, got %+v", triageResp.CreatedTaskIDs)
	}
	if len(triageResp.WorkspacesCreated) != 0 {
		t.Fatalf("expected no workspaces created for progress query, got %+v", triageResp.WorkspacesCreated)
	}
	if !strings.Contains(triageResp.SummaryMessage, "排队 1") {
		t.Fatalf("expected summary to include queued count, got %q", triageResp.SummaryMessage)
	}
	if !strings.Contains(triageResp.SummaryMessage, "一共有") || !strings.Contains(triageResp.SummaryMessage, "文件") || !strings.Contains(triageResp.SummaryMessage, "字") {
		t.Fatalf("expected summary to include workspace stats, got %q", triageResp.SummaryMessage)
	}
	if strings.Contains(strings.ToLower(triageResp.SummaryMessage), "workspace") {
		t.Fatalf("expected summary to avoid internal term 'workspace', got %q", triageResp.SummaryMessage)
	}
}
