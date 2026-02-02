package workflowexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workflow"
)

type fakeResolver struct {
	client llm.Client
}

func (r fakeResolver) ResolveClient(_ context.Context, _ string, _ string) (llm.Client, string, error) {
	return r.client, "fake", nil
}

type fakeToolClient struct {
	content string
}

func (c *fakeToolClient) ChatCompletion(_ context.Context, _ []llm.ChatMessage, _ *llm.ChatCompletionOptions) (string, error) {
	return c.content, nil
}

func (c *fakeToolClient) ChatCompletionStream(_ context.Context, _ []llm.ChatMessage, _ *llm.ChatCompletionOptions, _ llm.StreamCallback) error {
	return nil
}

func (c *fakeToolClient) ChatCompletionWithTools(_ context.Context, _ []llm.ChatMessage, _ *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	return llm.ChatCompletionResult{Content: c.content}, nil
}

func TestExecutor_ExportsDeliverablesAndWritesManifest(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	store, err := workflow.NewStore(filepath.Join(t.TempDir(), "workflows"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	src := filepath.Join(ws, "out", "article.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	ctx := context.Background()
	wf, err := store.CreateWorkflow(ctx, ws, "Demo")
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	ver, err := store.PublishVersion(ctx, ws, wf.WorkflowID, workflow.Graph{
		Nodes: []workflow.Node{{
			NodeID: "writer",
			Prompt: "write an article",
		}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, ver.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	handOff := strings.TrimSpace(`
<subagent_handoff>
  <summary>ok</summary>
  <timeline>## 流水账
- wrote file</timeline>
  <findings>## Findings
- done</findings>
  <changed_files>## 变更文件
- out/article.md</changed_files>
</subagent_handoff>
`)

	exec := &Executor{
		Store:              store,
		Runtime:            rt,
		Resolver:           fakeResolver{client: &fakeToolClient{content: handOff}},
		DefaultPrincipalID: "local",
		WorkspaceRoot:      ws,
		ExecutionRoot:      ws,
	}

	final, err := store.RunWorkflow(ctx, ws, wf.WorkflowID, run.RunID, exec, workflow.RunOptions{Concurrency: 1})
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if final.Status != workflow.RunStatusSucceeded {
		t.Fatalf("expected succeeded, got %+v", final)
	}

	nr := final.NodeRuns["writer"]
	if nr.Status != workflow.NodeStatusSucceeded {
		t.Fatalf("expected writer succeeded, got %+v", nr)
	}
	if len(nr.Artifacts.Artifacts) == 0 {
		t.Fatalf("expected artifacts, got %+v", nr.Artifacts)
	}

	foundDeliverable := ""
	foundManifest := ""
	for _, a := range nr.Artifacts.Artifacts {
		if strings.TrimSpace(a.Kind) == "deliverable" && strings.Contains(a.Path, string(filepath.Separator)+"deliverables"+string(filepath.Separator)) {
			foundDeliverable = a.Path
		}
		if strings.TrimSpace(a.Kind) == "manifest" {
			foundManifest = a.Path
		}
	}
	if strings.TrimSpace(foundDeliverable) == "" {
		t.Fatalf("expected deliverable artifact, got %+v", nr.Artifacts.Artifacts)
	}
	if strings.TrimSpace(foundManifest) == "" {
		t.Fatalf("expected manifest artifact, got %+v", nr.Artifacts.Artifacts)
	}
	if _, err := os.Stat(foundDeliverable); err != nil {
		t.Fatalf("expected exported deliverable to exist: %v", err)
	}
	if _, err := os.Stat(foundManifest); err != nil {
		t.Fatalf("expected manifest file to exist: %v", err)
	}
}
