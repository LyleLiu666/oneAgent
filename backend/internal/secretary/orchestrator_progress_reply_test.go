package secretary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type promptAssertingClient struct {
	wantUserPromptSubstrings  []string
	wantTurnContextSubstrings []string
	called                    bool
}

func (c *promptAssertingClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("unexpected ChatCompletion call (expected tool calling)")
}

func (c *promptAssertingClient) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	_ = ctx
	_ = opts

	foundUserPrompt := false
	foundTurnContext := false
	for _, m := range messages {
		if m.Role != model.MessageRoleUser {
			continue
		}
		if strings.HasPrefix(m.Content, "【TurnContext（每轮变化") {
			for _, want := range c.wantTurnContextSubstrings {
				if strings.TrimSpace(want) == "" {
					continue
				}
				if !strings.Contains(m.Content, want) {
					return llm.ChatCompletionResult{}, fmt.Errorf("expected turn context to include %q, got %q", want, m.Content)
				}
			}
			foundTurnContext = true
			continue
		}
		if !strings.Contains(m.Content, "session_workspace_root:") {
			continue
		}
		for _, want := range c.wantUserPromptSubstrings {
			if strings.TrimSpace(want) == "" {
				continue
			}
			if !strings.Contains(m.Content, want) {
				return llm.ChatCompletionResult{}, fmt.Errorf("expected SW prompt to include %q, got %q", want, m.Content)
			}
		}
		foundUserPrompt = true
	}
	if !foundUserPrompt {
		return llm.ChatCompletionResult{}, errors.New("missing SW user prompt")
	}
	if len(c.wantTurnContextSubstrings) > 0 && !foundTurnContext {
		return llm.ChatCompletionResult{}, errors.New("missing turn context message")
	}

	c.called = true
	return llm.ChatCompletionResult{
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      "secretary_triage_plan",
					Arguments: `{"intent":"progress","summary_message":"ok","tasks":[],"task_actions":[],"questions":[]}`,
				},
			},
		},
	}, nil
}

func (c *promptAssertingClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestTriage_ProgressQuestion_UsesTaskSnapshot_AndUsesLLMSummary(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	running, err := tasks.CreateTask("local", ws, "写武侠小说", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	now := time.Now().UTC()
	if _, err := tasks.UpdateTask(running.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptRunning
		a.StartedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	client := &promptAssertingClient{
		wantUserPromptSubstrings: []string{
			"session_workspace_root:",
			"现在有几个任务在进行",
		},
		wantTurnContextSubstrings: []string{
			"## 任务看板快照",
			"我查了下：运行",
			"写武侠小说",
		},
	}

	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   &taskqueue.TaskRunner{},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	res, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "现在有几个任务在进行", ws)
	if err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}
	if res.MessageID == 0 {
		t.Fatalf("expected message id to be set")
	}

	triaged, err := o.Triage(context.Background(), "local", "session-1", nil)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if got := strings.TrimSpace(triaged.SummaryMessage); got == "" {
		t.Fatalf("expected triage summary to be non-empty")
	}
	if !strings.Contains(triaged.SummaryMessage, "ok") {
		t.Fatalf("expected triage summary to come from LLM plan, got %q", triaged.SummaryMessage)
	}
	if !client.called {
		t.Fatalf("expected SW client to be called")
	}
}

func TestTriage_WhenLLMUnavailable_ReturnsError(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	running, err := tasks.CreateTask("local", ws, "写武侠小说", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	now := time.Now().UTC()
	if _, err := tasks.UpdateTask(running.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptRunning
		a.StartedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   &taskqueue.TaskRunner{},
		// No ResolveModel: simulate deployments/tests without LLM configured.
	}

	if _, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "任务完成得怎么样", ws); err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	triaged, err := o.Triage(context.Background(), "local", "session-1", nil)
	if err == nil {
		t.Fatalf("expected triage to fail without ResolveModel/LLM")
	}
	if triaged.SummaryMessage != "" {
		t.Fatalf("expected no summary message on error, got %q", triaged.SummaryMessage)
	}
}

func TestSecretaryDefaultToolIDs_UsesPolicyAllowlist(t *testing.T) {
	snap := permissions.ResolveSnapshot("local", secretaryReadOnlyPolicy(), time.Now())
	ids := secretaryDefaultToolIDs(snap)
	if len(ids) == 0 {
		t.Fatalf("expected tool ids to be non-empty")
	}

	want := []string{
		tool.ToolIDSearch,
		tool.ToolIDReadFile,
		tool.ToolIDLs,
		tool.ToolIDGlob,
		tool.ToolIDRg,
		tool.ToolIDSkillRead,
	}
	for _, id := range want {
		found := false
		for _, got := range ids {
			if got == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected tool ids to include %q, got %v", id, ids)
		}
	}

	deny := []string{
		tool.ToolIDWriteFile,
		tool.ToolIDEdit,
		tool.ToolIDEditV2,
		tool.ToolIDMultiEdit,
		tool.ToolIDTrashFile,
		tool.ToolIDDocumentExport,
		tool.ToolIDPlan,
		tool.ToolIDBash,
		tool.ToolIDRunCommand,
		tool.ToolIDSubagent,
	}
	for _, id := range deny {
		for _, got := range ids {
			if got == id {
				t.Fatalf("expected tool ids to exclude %q, got %v", id, ids)
			}
		}
	}
}

func TestBuildProgressReply_IncludesTaskDetails(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	running, err := store.CreateTask("local", ws, "写武侠小说", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask running: %v", err)
	}
	succeeded, err := store.CreateTask("local", ws, "生成大纲", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask succeeded: %v", err)
	}
	failed, err := store.CreateTask("local", ws, "补齐设定表", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	now := time.Now().UTC()

	if _, err := store.UpdateTask(running.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptRunning
		a.StartedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask running: %v", err)
	}

	if _, err := store.UpdateTask(succeeded.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptSucceeded
		a.Summary = "完成大纲并输出到文件"
		a.FindingsPath = "/tmp/findings.md"
		a.FinishedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask succeeded: %v", err)
	}

	if _, err := store.UpdateTask(failed.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptFailed
		a.Error = "boom"
		a.FinishedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	o := &Orchestrator{Tasks: store}
	st := State{
		TriageRuns: []TriageRun{{
			CreatedTaskIDs: []string{running.ID, succeeded.ID, failed.ID},
		}},
	}

	out, _, err := o.buildProgressReply("local", ws, st)
	if err != nil {
		t.Fatalf("buildProgressReply: %v", err)
	}

	if !strings.Contains(out, "运行") || !strings.Contains(out, "已完成") {
		t.Fatalf("expected status counts in output, got %q", out)
	}

	for _, title := range []string{running.Title, succeeded.Title, failed.Title} {
		if !strings.Contains(out, title) {
			t.Fatalf("expected output to include title %q, got %q", title, out)
		}
	}
	if !strings.Contains(out, "findings") && !strings.Contains(out, "Findings") && !strings.Contains(out, "产出") {
		t.Fatalf("expected output to mention artifacts for succeeded task, got %q", out)
	}
}
