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

	streamCalls [][]llm.ChatMessage
	chatCalls   [][]llm.ChatMessage
}

func (c *suReportStreamClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = opts
	c.chatCalls = append(c.chatCalls, messages)
	return "", nil
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

func TestTriage_BuildsSummaryFromPlan_WhenQuestionsExist(t *testing.T) {
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
	}

	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   &taskqueue.TaskRunner{},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	if _, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "我有点忙，你先帮我整理下需要我确认的信息", ws); err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	got, err := o.Triage(context.Background(), "local", "session-1", nil)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if !strings.Contains(got.SummaryMessage, "sw_summary") {
		t.Fatalf("expected summary to include plan summary, got %q", got.SummaryMessage)
	}
	if !strings.Contains(got.SummaryMessage, "请把仓库根目录路径发我") {
		t.Fatalf("expected summary to include question, got %q", got.SummaryMessage)
	}
	if len(client.chatCalls) != 0 {
		t.Fatalf("expected triage to not require non-streaming ChatCompletion, got %d calls", len(client.chatCalls))
	}
}
