package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func newBlockingTestTaskRunner(store *taskqueue.Store) *taskqueue.TaskRunner {
	return &taskqueue.TaskRunner{
		Store: store,
		DecideOutcome: func(context.Context, taskqueue.Task, taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, _ taskqueue.Task, _ taskqueue.Attempt, _ *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			<-ctx.Done()
			return taskqueue.AttemptResult{}, ctx.Err()
		},
	}
}

func TestMCP_ToolsList_Succeeds(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %+v", resp.Error)
	}
	if len(resp.Result.Tools) < 3 {
		t.Fatalf("expected tools, got %d", len(resp.Result.Tools))
	}
	found := false
	for _, tool := range resp.Result.Tools {
		if tool.Name == "tasks.list" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tasks.list tool")
	}
}

func TestMCP_ToolsCall_TasksList_ReturnsTasks(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	workspace := t.TempDir()
	created, err := rt.Tasks.CreateTask("local", workspace, "T1", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tasks.list","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %+v", resp.Error)
	}
	if len(resp.Result.Content) == 0 || strings.TrimSpace(resp.Result.Content[0].Text) == "" {
		t.Fatalf("expected content text")
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(resp.Result.Content[0].Text), &tasks); err != nil {
		t.Fatalf("parse tasks json: %v", err)
	}
	found := false
	for _, tsk := range tasks {
		if tsk["id"] == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created task id %q in tasks", created.ID)
	}
}

func TestMCP_AuthRequired_WhenTokenMode(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "token",
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCP_LocalOnly_DeniesNonLoopback(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
		MCPAllowRemote:   false,
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCP_PolicyDeny_RejectsCall(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := rt.Settings.SetToolPolicy(ctx, "local", permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{{
			ID:     "deny-mcp-tools-list",
			Effect: permissions.EffectDeny,
			ToolID: "mcp.tools.list",
		}},
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCP_WriteTool_DefaultsToDenyWithoutExplicitAllow(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	workspace := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tasks.create",
			"arguments": map[string]any{
				"request_id": "req-1",
				"workspace":  workspace,
				"prompt":     "hi",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error struct {
			Data struct {
				RequestID string `json:"request_id"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error.Data.RequestID != "req-1" {
		t.Fatalf("expected request_id req-1, got %q", resp.Error.Data.RequestID)
	}
}

func TestMCP_WriteTool_AllowsWithExplicitRule(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	rt.TaskRunner = newBlockingTestTaskRunner(rt.Tasks)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := rt.Settings.SetToolPolicy(ctx, "local", permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{{
			ID:     "allow-mcp-tasks-create",
			Effect: permissions.EffectAllow,
			ToolID: "mcp.tool.tasks.create",
		}},
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	workspace := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tasks.create",
			"arguments": map[string]any{
				"request_id": "req-1",
				"workspace":  workspace,
				"prompt":     "hi",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %+v", resp.Error)
	}
	if len(resp.Result.Content) == 0 || strings.TrimSpace(resp.Result.Content[0].Text) == "" {
		t.Fatalf("expected content text")
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(resp.Result.Content[0].Text), &task); err != nil {
		t.Fatalf("parse task json: %v", err)
	}
	taskID, _ := task["id"].(string)
	if strings.TrimSpace(taskID) == "" {
		t.Fatalf("expected created task id")
	}
	created, err := rt.Tasks.GetTask(taskID)
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	evs, err := rt.Tasks.ReadEvents(taskID)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	found := false
	for _, ev := range evs {
		if ev.Type == "mcp.action.tasks.create" {
			found = true
			if ev.Data["principal_id"] != created.UserID {
				t.Fatalf("expected principal_id %q in event, got %#v", created.UserID, ev.Data["principal_id"])
			}
			if ev.Data["request_id"] != "req-1" {
				t.Fatalf("expected request_id req-1 in event, got %#v", ev.Data["request_id"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected mcp.action.tasks.create event")
	}

	tracePath := filepath.Join(rt.Layout.TraceLogsDir, "mcp_calls.jsonl")
	traceData, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read mcp trace: %v", err)
	}
	if !bytes.Contains(traceData, []byte(`"tool_id":"mcp.tool.tasks.create"`)) {
		t.Fatalf("expected mcp trace to include tool_id mcp.tool.tasks.create")
	}
	if !bytes.Contains(traceData, []byte(`"request_id":"req-1"`)) {
		t.Fatalf("expected mcp trace to include request_id req-1")
	}
}

func TestMCP_WriteTool_ApprovalRequiredFlow_IsNonReplayable(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	rt.TaskRunner = newBlockingTestTaskRunner(rt.Tasks)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := rt.Settings.SetToolPolicy(ctx, "local", permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{{
			ID:     "allow-mcp-tasks-create-with-approval",
			Effect: permissions.EffectAllow,
			ToolID: "mcp.tool.tasks.create",
			Constraints: permissions.Constraints{
				Approval: "required",
			},
		}},
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	workspace := t.TempDir()
	payload := func() []byte {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "tasks.create",
				"arguments": map[string]any{
					"request_id": "req-1",
					"workspace":  workspace,
					"prompt":     "hi",
				},
			},
		})
		return body
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var requiredResp struct {
		Error struct {
			Code int `json:"code"`
			Data struct {
				Code       string `json:"code"`
				ApprovalID string `json:"approval_id"`
				ToolID     string `json:"tool_id"`
				ScopeID    string `json:"scope_id"`
				RequestID  string `json:"request_id"`
				ArgsHash   string `json:"args_hash"`
			} `json:"data"`
		} `json:"error"`
		Result any `json:"result"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&requiredResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if requiredResp.Error.Code != -32001 || requiredResp.Error.Data.Code != "approval_required" {
		t.Fatalf("expected approval_required, got %+v", requiredResp)
	}
	if requiredResp.Error.Data.ToolID != "mcp.tool.tasks.create" {
		t.Fatalf("expected tool_id mcp.tool.tasks.create, got %q", requiredResp.Error.Data.ToolID)
	}
	if requiredResp.Error.Data.ScopeID != "mcp:req-1" {
		t.Fatalf("expected scope_id mcp:req-1, got %q", requiredResp.Error.Data.ScopeID)
	}
	if requiredResp.Error.Data.RequestID != "req-1" {
		t.Fatalf("expected request_id req-1, got %q", requiredResp.Error.Data.RequestID)
	}
	if len(requiredResp.Error.Data.ApprovalID) == 0 {
		t.Fatalf("expected approval_id")
	}
	if len(requiredResp.Error.Data.ArgsHash) != 64 {
		t.Fatalf("expected args_hash sha256 hex, got %q", requiredResp.Error.Data.ArgsHash)
	}

	approveBody, _ := json.Marshal(map[string]any{"reason": "ok"})
	approveReq := httptest.NewRequest(http.MethodPost, "/api/approvals/"+requiredResp.Error.Data.ApprovalID+"/approve", bytes.NewReader(approveBody))
	approveReq.Header.Set("Content-Type", "application/json")
	approveReq.RemoteAddr = "127.0.0.1:1234"
	approveRec := httptest.NewRecorder()
	router.ServeHTTP(approveRec, approveReq)
	if approveRec.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", approveRec.Code, approveRec.Body.String())
	}

	// Retry after approval should succeed.
	retryReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(payload))
	retryReq.Header.Set("Content-Type", "application/json")
	retryReq.RemoteAddr = "127.0.0.1:1234"
	retryRec := httptest.NewRecorder()
	router.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", retryRec.Code, retryRec.Body.String())
	}

	var okResp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(retryRec.Body.Bytes())).Decode(&okResp); err != nil {
		t.Fatalf("decode retry: %v", err)
	}
	if okResp.Error != nil {
		t.Fatalf("expected no error, got %+v", okResp.Error)
	}
	if len(okResp.Result.Content) == 0 || strings.TrimSpace(okResp.Result.Content[0].Text) == "" {
		t.Fatalf("expected content text")
	}

	// A second identical call must require a new approval (non-replayable).
	secondReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(payload))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.RemoteAddr = "127.0.0.1:1234"
	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", secondRec.Code, secondRec.Body.String())
	}
	var secondRequired struct {
		Error struct {
			Data struct {
				Code       string `json:"code"`
				ApprovalID string `json:"approval_id"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(secondRec.Body.Bytes())).Decode(&secondRequired); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if secondRequired.Error.Data.Code != "approval_required" {
		t.Fatalf("expected approval_required, got %s body=%s", secondRequired.Error.Data.Code, secondRec.Body.String())
	}
	if secondRequired.Error.Data.ApprovalID == requiredResp.Error.Data.ApprovalID {
		t.Fatalf("expected a new approval_id, got same %q", secondRequired.Error.Data.ApprovalID)
	}
}

func TestMCP_WriteTool_ApprovalDeniedFlow_ReturnsStableError(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	rt.TaskRunner = newBlockingTestTaskRunner(rt.Tasks)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := rt.Settings.SetToolPolicy(ctx, "local", permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{{
			ID:     "allow-mcp-tasks-cancel-with-approval",
			Effect: permissions.EffectAllow,
			ToolID: "mcp.tool.tasks.cancel",
			Constraints: permissions.Constraints{
				Approval: "required",
			},
		}},
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	workspace := t.TempDir()
	task, err := rt.Tasks.CreateTask("local", workspace, "T1", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tasks.cancel",
			"arguments": map[string]any{
				"request_id": "req-2",
				"task_id":    task.ID,
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var required struct {
		Error struct {
			Code int `json:"code"`
			Data struct {
				Code       string `json:"code"`
				ApprovalID string `json:"approval_id"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&required); err != nil {
		t.Fatalf("decode required: %v", err)
	}
	if required.Error.Code != -32001 || required.Error.Data.Code != "approval_required" {
		t.Fatalf("expected approval_required, got %s body=%s", required.Error.Data.Code, rec.Body.String())
	}

	denyBody, _ := json.Marshal(map[string]any{"reason": "no"})
	denyReq := httptest.NewRequest(http.MethodPost, "/api/approvals/"+required.Error.Data.ApprovalID+"/deny", bytes.NewReader(denyBody))
	denyReq.Header.Set("Content-Type", "application/json")
	denyReq.RemoteAddr = "127.0.0.1:1234"
	denyRec := httptest.NewRecorder()
	router.ServeHTTP(denyRec, denyReq)
	if denyRec.Code != http.StatusOK {
		t.Fatalf("deny status=%d body=%s", denyRec.Code, denyRec.Body.String())
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(body))
	retryReq.Header.Set("Content-Type", "application/json")
	retryReq.RemoteAddr = "127.0.0.1:1234"
	retryRec := httptest.NewRecorder()
	router.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", retryRec.Code, retryRec.Body.String())
	}

	var denied struct {
		Error struct {
			Code int `json:"code"`
			Data struct {
				Code       string `json:"code"`
				ApprovalID string `json:"approval_id"`
				Reason     string `json:"reason"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(retryRec.Body.Bytes())).Decode(&denied); err != nil {
		t.Fatalf("decode denied: %v", err)
	}
	if denied.Error.Code != -32002 || denied.Error.Data.Code != "approval_denied" {
		t.Fatalf("expected approval_denied, got %s body=%s", denied.Error.Data.Code, retryRec.Body.String())
	}
	if denied.Error.Data.ApprovalID != required.Error.Data.ApprovalID {
		t.Fatalf("expected approval_id %q, got %q", required.Error.Data.ApprovalID, denied.Error.Data.ApprovalID)
	}
	if denied.Error.Data.Reason != "no" {
		t.Fatalf("expected reason no, got %q", denied.Error.Data.Reason)
	}
}

func TestMCP_ActionStateMachine_MatchesHTTP_CancelAndResume(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")
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

	rt.TaskRunner = newBlockingTestTaskRunner(rt.Tasks)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := rt.Settings.SetToolPolicy(ctx, "local", permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-mcp-tasks-cancel",
				Effect: permissions.EffectAllow,
				ToolID: "mcp.tool.tasks.cancel",
			},
			{
				ID:     "allow-mcp-tasks-resume",
				Effect: permissions.EffectAllow,
				ToolID: "mcp.tool.tasks.resume",
			},
		},
	}); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	workspace := t.TempDir()
	httpTask, err := rt.Tasks.CreateTask("local", workspace, "T-http", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create http task: %v", err)
	}
	mcpTask, err := rt.Tasks.CreateTask("local", workspace, "T-mcp", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create mcp task: %v", err)
	}

	// Cancel via HTTP.
	httpReq := httptest.NewRequest(http.MethodPost, "/api/tasks/"+httpTask.ID+"/cancel", nil)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.RemoteAddr = "127.0.0.1:1234"
	httpRec := httptest.NewRecorder()
	router.ServeHTTP(httpRec, httpReq)
	if httpRec.Code != http.StatusOK {
		t.Fatalf("http cancel status=%d body=%s", httpRec.Code, httpRec.Body.String())
	}

	// Cancel via MCP.
	cancelBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tasks.cancel",
			"arguments": map[string]any{
				"request_id": "req-cancel-1",
				"task_id":    mcpTask.ID,
			},
		},
	})
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(cancelBody))
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelReq.RemoteAddr = "127.0.0.1:1234"
	cancelRec := httptest.NewRecorder()
	router.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("mcp cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}

	gotHTTP, err := rt.Tasks.GetTask(httpTask.ID)
	if err != nil {
		t.Fatalf("get http task: %v", err)
	}
	gotMCP, err := rt.Tasks.GetTask(mcpTask.ID)
	if err != nil {
		t.Fatalf("get mcp task: %v", err)
	}
	if gotHTTP.LatestAttempt() == nil || gotMCP.LatestAttempt() == nil {
		t.Fatalf("expected attempts")
	}
	if gotHTTP.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected http cancel status %q, got %q", taskqueue.AttemptCanceled, gotHTTP.LatestAttempt().Status)
	}
	if gotMCP.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected mcp cancel status %q, got %q", taskqueue.AttemptCanceled, gotMCP.LatestAttempt().Status)
	}

	// Resume: force both tasks into terminal status first.
	now := time.Now()
	resumeHTTP, err := rt.Tasks.CreateTask("local", workspace, "R-http", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create resume http task: %v", err)
	}
	resumeMCP, err := rt.Tasks.CreateTask("local", workspace, "R-mcp", "do it", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create resume mcp task: %v", err)
	}
	for _, id := range []string{resumeHTTP.ID, resumeMCP.ID} {
		if _, err := rt.Tasks.UpdateTask(id, func(tk *taskqueue.Task) error {
			a := tk.LatestAttempt()
			a.Status = taskqueue.AttemptSucceeded
			a.FinishedAt = &now
			return nil
		}); err != nil {
			t.Fatalf("force terminal: %v", err)
		}
	}

	// Resume via HTTP.
	resumeHTTPBody, _ := json.Marshal(map[string]any{"review_notes": "ok", "source": "mcp"})
	resumeHTTPReq := httptest.NewRequest(http.MethodPost, "/api/tasks/"+resumeHTTP.ID+"/resume", bytes.NewReader(resumeHTTPBody))
	resumeHTTPReq.Header.Set("Content-Type", "application/json")
	resumeHTTPReq.RemoteAddr = "127.0.0.1:1234"
	resumeHTTPRec := httptest.NewRecorder()
	router.ServeHTTP(resumeHTTPRec, resumeHTTPReq)
	if resumeHTTPRec.Code != http.StatusOK {
		t.Fatalf("http resume status=%d body=%s", resumeHTTPRec.Code, resumeHTTPRec.Body.String())
	}

	// Resume via MCP.
	resumeBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tasks.resume",
			"arguments": map[string]any{
				"request_id":   "req-resume-1",
				"task_id":      resumeMCP.ID,
				"review_notes": "ok",
			},
		},
	})
	resumeReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(resumeBody))
	resumeReq.Header.Set("Content-Type", "application/json")
	resumeReq.RemoteAddr = "127.0.0.1:1234"
	resumeRec := httptest.NewRecorder()
	router.ServeHTTP(resumeRec, resumeReq)
	if resumeRec.Code != http.StatusOK {
		t.Fatalf("mcp resume status=%d body=%s", resumeRec.Code, resumeRec.Body.String())
	}

	gotResumeHTTP, err := rt.Tasks.GetTask(resumeHTTP.ID)
	if err != nil {
		t.Fatalf("get resume http task: %v", err)
	}
	gotResumeMCP, err := rt.Tasks.GetTask(resumeMCP.ID)
	if err != nil {
		t.Fatalf("get resume mcp task: %v", err)
	}
	if len(gotResumeHTTP.Attempts) != 2 {
		t.Fatalf("expected 2 attempts for http resume, got %d", len(gotResumeHTTP.Attempts))
	}
	if len(gotResumeMCP.Attempts) != 2 {
		t.Fatalf("expected 2 attempts for mcp resume, got %d", len(gotResumeMCP.Attempts))
	}
	if strings.TrimSpace(gotResumeHTTP.LatestAttempt().ResumedFromAttemptID) == "" {
		t.Fatalf("expected resumed_from_attempt_id for http resume")
	}
	if strings.TrimSpace(gotResumeMCP.LatestAttempt().ResumedFromAttemptID) == "" {
		t.Fatalf("expected resumed_from_attempt_id for mcp resume")
	}

	evs, err := rt.Tasks.ReadEvents(resumeMCP.ID)
	if err != nil {
		t.Fatalf("read resume mcp events: %v", err)
	}
	found := false
	for _, ev := range evs {
		if ev.Type == "mcp.action.tasks.resume" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mcp.action.tasks.resume event")
	}
}
