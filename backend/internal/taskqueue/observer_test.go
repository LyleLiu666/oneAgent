package taskqueue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

type sequenceLLMClient struct {
	outs []string
	errs []error

	callCount int

	lastMessages []llm.ChatMessage
}

func (c *sequenceLLMClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	c.callCount++
	c.lastMessages = messages

	if len(c.outs) == 0 {
		return "", errors.New("no more outputs in sequenceLLMClient")
	}

	out := c.outs[0]
	c.outs = c.outs[1:]

	var err error
	if len(c.errs) > 0 {
		err = c.errs[0]
		c.errs = c.errs[1:]
	}
	return out, err
}

func (c *sequenceLLMClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	return errors.New("streaming not implemented in sequenceLLMClient")
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

func TestOutcomeObserver_Decide_ParsesFirstJSONObjectWhenTrailingBracesExist(t *testing.T) {
	client := &stubLLMClient{
		out: "```json\n{\"pass\":true,\"reason\":\"done\",\"evidence\":[],\"next_steps\":\"\",\"questions_for_user\":[]}\n```\n\nextra note {not json}\n",
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
	if !got.Pass || got.Reason != "done" {
		t.Fatalf("unexpected decision: %+v", got)
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

func TestParseObserverDecision_FallbackMarkdown_PassTrue(t *testing.T) {
	raw := "## 评估结果\n\n**Pass: true**\n\n### 理由\n\n这个 Attempt 合理完成了阶段目标。\n"
	got, err := parseObserverDecision(raw)
	if err != nil {
		t.Fatalf("parseObserverDecision: %v", err)
	}
	if !got.Pass {
		t.Fatalf("expected pass=true, got %+v", got)
	}
	if strings.TrimSpace(got.Reason) == "" {
		t.Fatalf("expected reason to be parsed, got %+v", got)
	}
}

func TestParseObserverDecision_FallbackMarkdown_PassFalse(t *testing.T) {
	raw := "## 评估结果\n\n**Pass: false**\n\n### 理由\n\n缺少关键交付物。\n\n### 下一步\n\n1. 补齐交付物\n2. 重新运行测试\n"
	got, err := parseObserverDecision(raw)
	if err != nil {
		t.Fatalf("parseObserverDecision: %v", err)
	}
	if got.Pass {
		t.Fatalf("expected pass=false, got %+v", got)
	}
	if strings.TrimSpace(got.Reason) == "" {
		t.Fatalf("expected reason to be parsed, got %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) == "" {
		t.Fatalf("expected next_steps to be parsed, got %+v", got)
	}
}

func TestParseObserverDecision_AcceptsXML(t *testing.T) {
	raw := `
some preface
<observer_decision>
  <pass>false</pass>
  <reason>missing evidence</reason>
  <evidence>
    <item>FINDINGS.md: missing</item>
    <item>trace.jsonl: empty</item>
  </evidence>
  <next_steps>run tests</next_steps>
  <questions_for_user>
    <item>Which branch should we target?</item>
  </questions_for_user>
</observer_decision>
`

	got, err := parseObserverDecision(raw)
	if err != nil {
		t.Fatalf("parseObserverDecision: %v", err)
	}
	if got.Pass {
		t.Fatalf("expected pass=false, got %+v", got)
	}
	if strings.TrimSpace(got.Reason) != "missing evidence" {
		t.Fatalf("expected reason to be parsed, got %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) != "run tests" {
		t.Fatalf("expected next_steps to be parsed, got %+v", got)
	}
	if len(got.Evidence) != 2 {
		t.Fatalf("expected evidence items, got %+v", got.Evidence)
	}
	if len(got.QuestionsForUser) != 1 {
		t.Fatalf("expected questions_for_user, got %+v", got.QuestionsForUser)
	}
}

func TestParseObserverDecision_AcceptsLooselyFormattedXMLWhenAngleBracketsUnescaped(t *testing.T) {
	raw := `
<observer_decision>
  <pass>false</pass>
  <reason>the expression 1 < 2 & 2 > 1 appears in text</reason>
  <evidence>
    <item>FINDINGS.md: contains "<div>"</item>
  </evidence>
  <next_steps>escape &lt; &amp; &gt; or switch to JSON</next_steps>
  <questions_for_user>
    <item>Is "a < b" expected?</item>
  </questions_for_user>
</observer_decision>
`

	got, err := parseObserverDecision(raw)
	if err != nil {
		t.Fatalf("parseObserverDecision: %v", err)
	}
	if got.Pass {
		t.Fatalf("expected pass=false, got %+v", got)
	}
	if !strings.Contains(got.Reason, "1 < 2") {
		t.Fatalf("expected reason to include raw angle brackets, got %+v", got)
	}
	if len(got.Evidence) != 1 || !strings.Contains(got.Evidence[0], "<div>") {
		t.Fatalf("expected evidence item to preserve raw text, got %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) == "" {
		t.Fatalf("expected next_steps to be parsed, got %+v", got)
	}
	if len(got.QuestionsForUser) != 1 || !strings.Contains(got.QuestionsForUser[0], "a < b") {
		t.Fatalf("expected questions_for_user item to preserve raw text, got %+v", got)
	}
}

func TestParseObserverDecision_RepairsMissingClosingObserverDecisionTag(t *testing.T) {
	raw := "<observer_decision>\n<pass>false</pass>\n<reason>missing evidence</reason>\n<next_steps>run tests</next_steps>\n"
	got, err := parseObserverDecision(raw)
	if err != nil {
		t.Fatalf("parseObserverDecision: %v", err)
	}
	if got.Pass {
		t.Fatalf("expected pass=false, got %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) != "run tests" {
		t.Fatalf("expected next_steps to be parsed, got %+v", got)
	}
}

func TestOutcomeObserver_Decide_RetriesWhenObserverOutputInvalid(t *testing.T) {
	client := &sequenceLLMClient{
		outs: []string{
			"我需要更多信息才能判断。",
			"<observer_decision><pass>false</pass><reason>missing evidence</reason><next_steps>run tests</next_steps></observer_decision>",
		},
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
	if got.Pass {
		t.Fatalf("expected pass=false, got %+v", got)
	}
	if strings.TrimSpace(got.NextSteps) != "run tests" {
		t.Fatalf("expected next_steps to be parsed, got %+v", got)
	}
	if client.callCount != 2 {
		t.Fatalf("expected 2 ChatCompletion calls, got %d", client.callCount)
	}
}
