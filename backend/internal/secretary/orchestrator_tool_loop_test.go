package secretary

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type scriptedStreamClient struct {
	responses []string

	callCount int
	calls     [][]llm.ChatMessage
}

func (c *scriptedStreamClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("unexpected ChatCompletion call")
}

func (c *scriptedStreamClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	c.callCount++
	c.calls = append(c.calls, messages)

	if len(c.responses) == 0 {
		return errors.New("no more scripted responses")
	}
	out := c.responses[0]
	c.responses = c.responses[1:]

	if err := callback(out); err != nil {
		return err
	}
	return nil
}

func (c *scriptedStreamClient) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	return llm.ChatCompletionResult{}, errors.New("unexpected ChatCompletionWithTools call")
}

func TestTriage_ToolLoop_XML_FeedsToolErrorAndDoesNotWrite(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	client := &scriptedStreamClient{
		responses: []string{
			`<tool_data>
<call>
  <tool_name>write_file</tool_name>
  <filePath>should_not_exist.txt</filePath>
  <content>hi</content>
</call>
</tool_data>`,
			`<secretary_triage_plan>
  <intent>progress</intent>
  <summary_message>ok</summary_message>
  <tasks></tasks>
  <questions></questions>
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

	if _, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "帮我拆解一下接下来要做什么", ws); err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	triaged, err := o.Triage(context.Background(), "local", "session-1", nil)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if strings.TrimSpace(triaged.SummaryMessage) != "ok" {
		t.Fatalf("expected summary ok, got %q", triaged.SummaryMessage)
	}

	if _, err := os.Stat(filepath.Join(ws, "should_not_exist.txt")); err == nil {
		t.Fatalf("expected tool loop to not write any files in workspace")
	}

	if client.callCount != 2 {
		t.Fatalf("expected 2 streaming calls, got %d", client.callCount)
	}
	if len(client.calls) < 2 {
		t.Fatalf("expected 2 calls recorded")
	}

	// Tool errors should be fed back to the model for retry via a <tool_result> message.
	foundToolResult := false
	for _, m := range client.calls[1] {
		if m.Role != model.MessageRoleUser {
			continue
		}
		if strings.Contains(m.Content, "<tool_result>") && strings.Contains(m.Content, "unknown tool") && strings.Contains(m.Content, "write_file") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected second call to include tool_result with unknown tool error, got %+v", client.calls[1])
	}
}
