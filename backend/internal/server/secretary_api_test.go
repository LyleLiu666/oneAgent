package server

import (
	"context"
	"encoding/json"
	"fmt"
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
		lastUser := ""
		for _, m := range req.Messages {
			if m.Role == "system" && system == "" {
				system = m.Content
			}
			if m.Role == "user" {
				lastUser = m.Content
			}
		}

		var content string
		switch {
		case strings.Contains(system, "ONEAGENT_SECRETARY_ACK"):
			snippet := []rune(strings.TrimSpace(lastUser))
			if len(snippet) > 12 {
				snippet = snippet[:12]
			}
			content = fmt.Sprintf("已记下：%s。", string(snippet))
		case strings.Contains(system, "ONEAGENT_SECRETARY_TRIAGE"):
			content = `{"summary_message":"我理解为 2 件事：①整理一份报告；②跑 backend 测试。我已分别安排 worker。","tasks":[{"title":"整理报告","prompt":"整理一份报告，输出 report.md，并确保内容结构清晰。","workspace_strategy":"new"},{"title":"跑 backend 测试","prompt":"在 repo 内运行 go test ./...，如失败请修复并补齐测试。","workspace_strategy":"session"}],"questions":[]}`
		default:
			http.Error(w, "unexpected system prompt", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunk := map[string]any{
			"id": "cmpl-test",
			"choices": []any{
				map[string]any{
					"delta": map[string]any{"content": content},
				},
			},
		}
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
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
	if strings.TrimSpace(r1.SessionID) == "" || r1.MessageID == 0 || r1.AckMessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", r1)
	}
	if !strings.Contains(r1.AckText, "已记下") || !strings.Contains(r1.AckText, "整理") {
		t.Fatalf("expected ack to reference message, got %q", r1.AckText)
	}

	var r2 inboxResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"session_id": r1.SessionID,
		"content":    "另外也跑一下 backend 测试",
	}, &r2)
	if r2.SessionID != r1.SessionID || r2.MessageID == 0 || r2.AckMessageID == 0 {
		t.Fatalf("unexpected inbox response 2: %+v", r2)
	}

	// Verify messages are persisted and ack is linked by parent_id.
	sessionBody := mustGet(t, srv.URL, rt.AuthToken, "/api/sessions/"+r1.SessionID)
	var sess struct {
		ID       string `json:"id"`
		Messages []struct {
			ID       uint  `json:"id"`
			Role     string `json:"role"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			ParentID *uint  `json:"parent_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(sessionBody, &sess); err != nil {
		t.Fatalf("unmarshal session: %v", err)
	}
	foundAck := false
	for _, m := range sess.Messages {
		if m.ID == r1.AckMessageID {
			foundAck = true
			if m.Role != "assistant" || m.Type != "text" {
				t.Fatalf("unexpected ack message: %+v", m)
			}
			if m.ParentID == nil || *m.ParentID != r1.MessageID {
				t.Fatalf("expected ack parent_id=%d, got %+v", r1.MessageID, m.ParentID)
			}
		}
	}
	if !foundAck {
		t.Fatalf("missing ack message id=%d", r1.AckMessageID)
	}

	type triageResp struct {
		SummaryMessage   string   `json:"summary_message"`
		SummaryMessageID uint     `json:"summary_message_id"`
		CursorMessageID  uint     `json:"cursor_message_id"`
		CreatedTaskIDs   []string `json:"created_task_ids"`
		Questions        []string `json:"questions"`
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
		"session_id":         r1.SessionID,
		"cursor_message_id":  0,
	}, &tr2)
	if len(tr2.CreatedTaskIDs) != len(tr1.CreatedTaskIDs) {
		t.Fatalf("expected same tasks on retry, got %+v vs %+v", tr2.CreatedTaskIDs, tr1.CreatedTaskIDs)
	}
	after, _ := rt.Tasks.ListTasks("", "")
	if len(after) != len(before) {
		t.Fatalf("expected no new tasks on retry, got before=%d after=%d", len(before), len(after))
	}

	// Summary message should be appended once.
	body2 := mustGet(t, srv.URL, rt.AuthToken, "/api/sessions/"+r1.SessionID)
	var sess2 struct {
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
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/secretary/state?session_id="+r1.SessionID, nil)
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
