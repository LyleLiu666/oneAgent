package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type scriptedToolClient struct {
	results []llm.ChatCompletionResult
	index   int
}

func (c *scriptedToolClient) ChatCompletion(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("not implemented")
}

func (c *scriptedToolClient) ChatCompletionStream(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) error {
	return errors.New("not implemented")
}

func (c *scriptedToolClient) ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	if c.index >= len(c.results) {
		return llm.ChatCompletionResult{}, errors.New("no scripted result available")
	}
	out := c.results[c.index]
	c.index++
	return out, nil
}

func TestRun_WritesFindingsAndTrace_WithChangedFilesAndPlanMarkDone(t *testing.T) {
	logsBase := t.TempDir()
	workspaceRoot := t.TempDir()

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write_file",
							Arguments: `{"filePath":"a.txt","content":"hello"}`,
						},
					},
					{
						ID:   "call_2",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "plan",
							Arguments: `{"action":"mark_done","task_id":"T1"}`,
						},
					},
				},
			},
			{
				Content: `<subagent_handoff>
  <summary>done</summary>
  <timeline>## 流水账
- wrote file</timeline>
  <findings>## Findings
- ok</findings>
</subagent_handoff>`,
			},
		},
	}

	tools := []llm.Tool{
		{Type: "function", Function: llm.ToolFunction{Name: "write_file"}},
		{Type: "function", Function: llm.ToolFunction{Name: "plan"}},
	}

	handlers := map[string]ToolHandler{
		"write_file": func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
		"plan": func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{"pass": false, "reason": "nope"}, nil
		},
	}

	got, err := Run(context.Background(), RunRequest{
		ParentSessionID:   "sess-123",
		UserID:            "u1",
		SystemPrompt:      "sys",
		Client:            client,
		Tools:             tools,
		Handlers:          handlers,
		WorkspaceRoot:     workspaceRoot,
		WriteScope:        []string{"**"},
		LogsBaseDir:       logsBase,
		Task:              "step",
		MaxSteps:          10,
		MaxRuntimeSeconds: 60,
		MaxLogBytes:       1024 * 1024,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.RunID == "" {
		t.Fatalf("expected run_id")
	}
	if got.TraceLogPath == "" {
		t.Fatalf("expected trace_log_path")
	}
	if got.FindingsPath == "" {
		t.Fatalf("expected findings_path")
	}
	if got.Usage == nil || got.Usage.Calls == 0 || got.Usage.TotalTokens == 0 {
		t.Fatalf("expected usage totals, got %+v", got.Usage)
	}

	if _, err := os.Stat(got.TraceLogPath); err != nil {
		t.Fatalf("stat trace log: %v", err)
	}
	if _, err := os.Stat(got.FindingsPath); err != nil {
		t.Fatalf("stat findings: %v", err)
	}

	if filepath.Dir(got.TraceLogPath) != filepath.Dir(got.FindingsPath) {
		t.Fatalf("expected trace and findings in same dir")
	}

	relTrace, err := filepath.Rel(logsBase, got.TraceLogPath)
	if err != nil {
		t.Fatalf("rel trace: %v", err)
	}
	parts := strings.Split(relTrace, string(os.PathSeparator))
	if len(parts) < 4 {
		t.Fatalf("unexpected trace path: %q", got.TraceLogPath)
	}
	if ok, _ := regexp.MatchString(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`, parts[0]); !ok {
		t.Fatalf("expected date dir, got %q", parts[0])
	}
	if parts[1] != "sess-123" {
		t.Fatalf("expected session dir %q, got %q", "sess-123", parts[1])
	}
	if parts[2] != got.RunID {
		t.Fatalf("expected run dir %q, got %q", got.RunID, parts[2])
	}
	if parts[3] != "trace.jsonl" {
		t.Fatalf("expected trace filename, got %q", parts[3])
	}

	if filepath.Base(got.FindingsPath) != "FINDINGS.md" {
		t.Fatalf("expected FINDINGS.md, got %q", filepath.Base(got.FindingsPath))
	}

	content, err := os.ReadFile(got.FindingsPath)
	if err != nil {
		t.Fatalf("read findings: %v", err)
	}
	text := string(content)
	for _, want := range []string{"## 流水账", "## Findings", "## 变更文件"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected findings to contain %q", want)
		}
	}
	if !strings.Contains(text, "- a.txt") {
		t.Fatalf("expected changed files to include a.txt, got:\n%s", text)
	}
	if !strings.Contains(text, "## Plan Mark Done") {
		t.Fatalf("expected plan mark done section, got:\n%s", text)
	}
	if !strings.Contains(text, "task_id=T1 pass=false") || !strings.Contains(text, "reason=nope") {
		t.Fatalf("expected plan mark done record, got:\n%s", text)
	}
}

func TestRun_TruncatesSummary(t *testing.T) {
	logsBase := t.TempDir()
	long := strings.Repeat("a", 2000)

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				Content: "<subagent_handoff><summary>" + long + "</summary></subagent_handoff>",
			},
		},
	}

	tools := []llm.Tool{
		{Type: "function", Function: llm.ToolFunction{Name: "write_file"}},
	}
	handlers := map[string]ToolHandler{
		"write_file": func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}

	got, err := Run(context.Background(), RunRequest{
		ParentSessionID:   "sess-123",
		UserID:            "u1",
		SystemPrompt:      "sys",
		Client:            client,
		Tools:             tools,
		Handlers:          handlers,
		WorkspaceRoot:     t.TempDir(),
		LogsBaseDir:       logsBase,
		Task:              "step",
		MaxSteps:          3,
		MaxRuntimeSeconds: 60,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if utf8.RuneCountInString(got.Summary) > 800 {
		t.Fatalf("expected summary to be truncated, got %d runes", utf8.RuneCountInString(got.Summary))
	}
}

func TestParseHandoffXML_ToleratesRawTextAndMalformedXML(t *testing.T) {
	t.Run("raw angle brackets and ampersands", func(t *testing.T) {
		input := `<subagent_handoff>
  <summary>ok</summary>
  <timeline>## 流水账
- emitted <tool_data> and & chars</timeline>
  <findings>## Findings
- ok</findings>
</subagent_handoff>`

		summary, timeline, findings, changed := parseHandoffXML(input)
		if summary != "ok" {
			t.Fatalf("expected summary ok, got %q", summary)
		}
		if !strings.Contains(timeline, "<tool_data>") || !strings.Contains(timeline, "&") {
			t.Fatalf("expected timeline to preserve raw text, got %q", timeline)
		}
		if !strings.Contains(findings, "## Findings") {
			t.Fatalf("expected findings to contain header, got %q", findings)
		}
		if changed != "" {
			t.Fatalf("expected changed_files empty, got %q", changed)
		}
	})

	t.Run("missing closing tags", func(t *testing.T) {
		input := `<subagent_handoff><summary>done</summary><findings>## Findings
- ok`

		summary, _, findings, _ := parseHandoffXML(input)
		if summary != "done" {
			t.Fatalf("expected summary done, got %q", summary)
		}
		if !strings.Contains(findings, "## Findings") || !strings.Contains(findings, "- ok") {
			t.Fatalf("expected findings to be extracted, got %q", findings)
		}
	})
}
