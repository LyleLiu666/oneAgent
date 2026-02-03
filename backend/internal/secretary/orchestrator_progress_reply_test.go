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
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type promptAssertingClient struct {
	wantSubstrings []string
	called         bool
}

func (c *promptAssertingClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = opts
	for _, m := range messages {
		if m.Role != model.MessageRoleUser {
			continue
		}
		if !strings.Contains(m.Content, "session_workspace_root:") {
			continue
		}
		for _, want := range c.wantSubstrings {
			if want == "" {
				continue
			}
			if !strings.Contains(m.Content, want) {
				return "", fmt.Errorf("expected SW prompt to include %q, got %q", want, m.Content)
			}
		}
		c.called = true
		return `{"summary_message":"ok","tasks":[],"questions":[]}`, nil
	}
	return "", errors.New("missing SW user prompt")
}

func (c *promptAssertingClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestTriage_ProgressQuestion_UsesSWPlanAndIncludesTaskSnapshot(t *testing.T) {
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
		wantSubstrings: []string{
			"现在有几个任务在进行",
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
	if strings.TrimSpace(triaged.SummaryMessage) != "ok" {
		t.Fatalf("expected triage summary ok, got %q", triaged.SummaryMessage)
	}
	if !client.called {
		t.Fatalf("expected SW client to be called")
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
