package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

type failingToolClient struct{}

func (c *failingToolClient) ChatCompletion(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("not implemented")
}

func (c *failingToolClient) ChatCompletionStream(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) error {
	return errors.New("not implemented")
}

func (c *failingToolClient) ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	return llm.ChatCompletionResult{}, errors.New("fetch failed")
}

func TestSubagentTool_Failure_ReturnsErrorAndArtifacts(t *testing.T) {
	home := t.TempDir()
	layout, err := oneruntime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	workspaceRoot := t.TempDir()

	base := context.Background()
	base = ContextWithSessionID(base, "sess-1")
	base = ContextWithUserID(base, "u1")
	base = ContextWithRuntimeLayout(base, layout)
	base = ContextWithLLMClient(base, &failingToolClient{})
	base = ContextWithSystemPrompt(base, "sys")
	base = ContextWithMountedToolIDs(base, []string{ToolIDSubagent, ToolIDReadFile})
	base = ContextWithWorkspace(base, WorkspaceConfig{Enabled: true, Root: workspaceRoot})

	anyOut, err := runSubagentTool(base, json.RawMessage(`{"task":"step1","k_skills":0}`))
	if err != nil {
		t.Fatalf("runSubagentTool: %v", err)
	}
	out, ok := anyOut.(subagentToolResult)
	if !ok {
		t.Fatalf("expected subagentToolResult, got %T", anyOut)
	}
	if out.OK {
		t.Fatalf("expected ok=false")
	}
	if strings.TrimSpace(out.Error) == "" || !strings.Contains(out.Error, "fetch failed") {
		t.Fatalf("expected error to include %q, got %q", "fetch failed", out.Error)
	}
	if out.FindingsPath == "" || out.TraceLogPath == "" {
		t.Fatalf("expected findings/trace paths, got findings=%q trace=%q", out.FindingsPath, out.TraceLogPath)
	}
	if _, err := os.Stat(out.FindingsPath); err != nil {
		t.Fatalf("stat findings: %v", err)
	}
	if _, err := os.Stat(out.TraceLogPath); err != nil {
		t.Fatalf("stat trace: %v", err)
	}
	if filepath.Dir(out.FindingsPath) != filepath.Dir(out.TraceLogPath) {
		t.Fatalf("expected artifacts in same dir")
	}
}
