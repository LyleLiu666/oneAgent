package taskqueue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type stubLLMClient struct {
	out string
	err error

	lastMessages []llm.ChatMessage
}

func (c *stubLLMClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	c.lastMessages = messages
	return c.out, c.err
}

func (c *stubLLMClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	return errors.New("streaming not implemented in stub")
}

func TestOutcomeObserver_Decide_ParsesJSON(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "FINDINGS.md")
	trace := filepath.Join(dir, "trace.jsonl")
	report := filepath.Join(dir, "TEST_REPORT.md")
	if err := os.WriteFile(findings, []byte("# ok\n"), 0o600); err != nil {
		t.Fatalf("write findings: %v", err)
	}
	if err := os.WriteFile(trace, []byte("{\"type\":\"start\"}\n"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	if err := os.WriteFile(report, []byte("TEST_REPORT_MARKER\n"), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	client := &stubLLMClient{
		out: `{"pass":true,"reason":"done","evidence":["FINDINGS.md"],"next_steps":"","questions_for_user":[]}`,
	}

	obs := &OutcomeObserver{Client: client}
	got, err := obs.Decide(context.Background(), ObserveInput{
		TaskID:         "task-1",
		AttemptID:      "attempt-1",
		WorkspaceRoot:  "/tmp/ws",
		Prompt:         "do the thing",
		Summary:        "finished",
		FindingsPath:   findings,
		TraceLogPath:   trace,
		TestReportPath: report,
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !got.Pass || got.Reason != "done" || len(got.Evidence) != 1 {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) != "" {
		t.Fatalf("expected empty next_steps for pass decision, got %q", got.NextSteps)
	}
	if len(client.lastMessages) != 2 || client.lastMessages[0].Role != "system" || client.lastMessages[1].Role != "user" {
		t.Fatalf("unexpected messages: %+v", client.lastMessages)
	}
	if !strings.Contains(client.lastMessages[1].Content, "TEST_REPORT_MARKER") {
		t.Fatalf("expected test report excerpt to be included in observer input")
	}
}

func TestOutcomeObserver_Decide_ParsesJSONFromWrappedOutput(t *testing.T) {
	client := &stubLLMClient{
		out: "OK\n```json\n{\"pass\":false,\"reason\":\"missing evidence\",\"evidence\":[],\"next_steps\":\"run tests and fix issues\",\"questions_for_user\":[\"Should we prioritize speed or correctness?\"]}\n```\n",
	}
	obs := &OutcomeObserver{Client: client}
	got, err := obs.Decide(context.Background(), ObserveInput{
		TaskID:        "task-1",
		AttemptID:     "attempt-1",
		WorkspaceRoot: "/tmp/ws",
		Prompt:        "do the thing",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if got.Pass || got.Reason != "missing evidence" {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) == "" {
		t.Fatalf("expected next_steps for fail decision")
	}
	if len(got.QuestionsForUser) != 1 {
		t.Fatalf("expected questions_for_user, got %+v", got.QuestionsForUser)
	}
}

func TestOutcomeObserver_Decide_RequiresClient(t *testing.T) {
	obs := &OutcomeObserver{}
	_, err := obs.Decide(context.Background(), ObserveInput{
		TaskID:        "task-1",
		AttemptID:     "attempt-1",
		WorkspaceRoot: "/tmp/ws",
		Prompt:        "do the thing",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}
