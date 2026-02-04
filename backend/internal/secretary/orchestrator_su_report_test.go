package secretary

import (
	"context"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type suReportStreamClient struct {
	streamResponses []string
	chatResponses   []string

	streamCalls [][]llm.ChatMessage
	chatCalls   [][]llm.ChatMessage
}

func (c *suReportStreamClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = opts
	c.chatCalls = append(c.chatCalls, messages)
	if len(c.chatResponses) == 0 {
		return "", nil
	}
	out := c.chatResponses[0]
	c.chatResponses = c.chatResponses[1:]
	return out, nil
}

func (c *suReportStreamClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	_ = ctx
	_ = opts
	c.streamCalls = append(c.streamCalls, messages)
	if len(c.streamResponses) == 0 {
		return nil
	}
	out := c.streamResponses[0]
	c.streamResponses = c.streamResponses[1:]
	return callback(out)
}

func TestTriage_UsesSUReportWhenAvailable(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	client := &suReportStreamClient{
		streamResponses: []string{
			`<secretary_triage_plan>
  <intent>clarify</intent>
  <summary_message>sw_summary</summary_message>
  <tasks></tasks>
  <questions>
    <item>请把仓库根目录路径发我（例如：/path/to/repo）</item>
  </questions>
</secretary_triage_plan>`,
		},
		chatResponses: []string{"SU_OK"},
	}

	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   &taskqueue.TaskRunner{},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	if _, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "我有点忙，你先帮我看下进度", ws); err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	got, err := o.Triage(context.Background(), "local", "session-1", nil)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if strings.TrimSpace(got.SummaryMessage) != "SU_OK" {
		t.Fatalf("expected SU report to be used, got %q", got.SummaryMessage)
	}
	if len(client.chatCalls) == 0 {
		t.Fatalf("expected ChatCompletion to be called for SU report")
	}
	if len(client.chatCalls[0]) == 0 || client.chatCalls[0][0].Role != "system" {
		t.Fatalf("expected first SU report message to be system prompt, got %+v", client.chatCalls[0])
	}
	if !strings.Contains(client.chatCalls[0][0].Content, "ONEAGENT_SECRETARY_SU_REPORT") {
		t.Fatalf("expected SU system prompt to include marker, got %q", client.chatCalls[0][0].Content)
	}
}
