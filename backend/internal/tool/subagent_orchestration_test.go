package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

type recordingToolClient struct {
	mu      sync.Mutex
	calls   [][]llm.ChatMessage
	results []llm.ChatCompletionResult
	index   int
}

func (c *recordingToolClient) ChatCompletion(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("not implemented")
}

func (c *recordingToolClient) ChatCompletionStream(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) error {
	return errors.New("not implemented")
}

func (c *recordingToolClient) ChatCompletionWithTools(_ context.Context, messages []llm.ChatMessage, _ *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls = append(c.calls, append([]llm.ChatMessage(nil), messages...))
	if c.index >= len(c.results) {
		return llm.ChatCompletionResult{}, errors.New("no scripted result available")
	}
	out := c.results[c.index]
	c.index++
	return out, nil
}

func TestSubagent_ContextSlicing_Step3OnlyCarriesSummariesAndReferences(t *testing.T) {
	home := t.TempDir()
	layout, err := oneruntime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	workspaceRoot := t.TempDir()

	client := &recordingToolClient{
		results: []llm.ChatCompletionResult{
			{Content: `<subagent_handoff><summary>S1</summary><timeline>## 流水账
- DETAIL1</timeline></subagent_handoff>`},
			{Content: `<subagent_handoff><summary>S2</summary><timeline>## 流水账
- DETAIL2</timeline></subagent_handoff>`},
			{Content: `<subagent_handoff><summary>S3</summary></subagent_handoff>`},
		},
	}

	base := context.Background()
	base = ContextWithSessionID(base, "sess-ctx")
	base = ContextWithUserID(base, "u1")
	base = ContextWithRuntimeLayout(base, layout)
	base = ContextWithLLMClient(base, client)
	base = ContextWithSystemPrompt(base, "sys")
	base = ContextWithMountedToolIDs(base, []string{ToolIDSubagent, ToolIDReadFile})
	base = ContextWithWorkspace(base, WorkspaceConfig{Enabled: true, Root: workspaceRoot})

	step1Any, err := runSubagentTool(base, json.RawMessage(`{"task":"step1","k_skills":0}`))
	if err != nil {
		t.Fatalf("step1: %v", err)
	}
	step1, ok := step1Any.(subagentToolResult)
	if !ok || !step1.OK {
		t.Fatalf("expected step1 ok, got %#v", step1Any)
	}

	step2Ctx := "step1: " + step1.Summary + "\nfindings: " + step1.FindingsPath + "\ntrace: " + step1.TraceLogPath + "\n"
	step2Any, err := runSubagentTool(base, json.RawMessage(`{"task":"step2","context_summary":`+jsonString(step2Ctx)+`, "k_skills":0}`))
	if err != nil {
		t.Fatalf("step2: %v", err)
	}
	step2, ok := step2Any.(subagentToolResult)
	if !ok || !step2.OK {
		t.Fatalf("expected step2 ok, got %#v", step2Any)
	}

	step3Ctx := strings.Join([]string{
		"step1: " + step1.Summary,
		"findings: " + step1.FindingsPath,
		"trace: " + step1.TraceLogPath,
		"",
		"step2: " + step2.Summary,
		"findings: " + step2.FindingsPath,
		"trace: " + step2.TraceLogPath,
		"",
	}, "\n")

	_, err = runSubagentTool(base, json.RawMessage(`{"task":"step3","context_summary":`+jsonString(step3Ctx)+`, "k_skills":0}`))
	if err != nil {
		t.Fatalf("step3: %v", err)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.calls) != 3 {
		t.Fatalf("expected 3 subagent LLM calls, got %d", len(client.calls))
	}

	var combined strings.Builder
	for _, msg := range client.calls[2] {
		combined.WriteString(msg.Role)
		combined.WriteString(": ")
		combined.WriteString(msg.Content)
		combined.WriteString("\n")
	}
	text := combined.String()

	if !strings.Contains(text, step1.Summary) || !strings.Contains(text, step2.Summary) {
		t.Fatalf("expected step3 input to carry step1/step2 summaries, got:\n%s", text)
	}
	if !strings.Contains(text, step1.FindingsPath) || !strings.Contains(text, step2.FindingsPath) {
		t.Fatalf("expected step3 input to carry step1/step2 findings references, got:\n%s", text)
	}
	if strings.Contains(text, "DETAIL1") || strings.Contains(text, "DETAIL2") {
		t.Fatalf("expected step3 input to not include step1/step2 details, got:\n%s", text)
	}
}

func jsonString(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}
