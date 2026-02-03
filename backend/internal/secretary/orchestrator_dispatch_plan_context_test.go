package secretary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
)

type contextAssertingClient struct {
	wantWorkspace string
}

func (c contextAssertingClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	for _, m := range messages {
		if m.Role != model.MessageRoleUser {
			continue
		}
		if !strings.Contains(m.Content, "session_workspace_root:") {
			continue
		}
		if !strings.Contains(m.Content, c.wantWorkspace) {
			return "", fmt.Errorf("expected SW prompt to include session workspace %q, got %q", c.wantWorkspace, m.Content)
		}
		return `{"summary_message":"ok","tasks":[],"questions":[]}`, nil
	}
	return "", errors.New("missing session_workspace_root context in SW prompt")
}

func (c contextAssertingClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestGenerateDispatchPlan_IncludesSessionWorkspaceRootContext(t *testing.T) {
	store, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	const wantWorkspace = "/tmp/repo"
	o := &Orchestrator{
		Sessions: store,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return contextAssertingClient{wantWorkspace: wantWorkspace}, "mock", nil
		},
	}

	plan, _, err := o.generateDispatchPlan(context.Background(), "local", "su", wantWorkspace, "", model.JSONB{}, []model.ChatMessage{
		{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "do it"},
	})
	if err != nil {
		t.Fatalf("generateDispatchPlan: %v", err)
	}
	if strings.TrimSpace(plan.SummaryMessage) != "ok" {
		t.Fatalf("expected plan summary ok, got %q", plan.SummaryMessage)
	}
}
