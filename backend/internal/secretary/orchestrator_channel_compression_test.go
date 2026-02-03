package secretary

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
)

type scriptedClient struct{}

func (c scriptedClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	sys := ""
	for _, m := range messages {
		if m.Role == model.MessageRoleSystem {
			sys = m.Content
			break
		}
	}

	switch {
	case strings.Contains(sys, "会话压缩器"):
		return "流水账:\n- 压缩测试\n\nFindings:\n- ok", nil
	case strings.Contains(sys, "ONEAGENT_SECRETARY_TRIAGE"):
		return `{"summary_message":"ok","tasks":[],"questions":[]}`, nil
	case strings.Contains(sys, "ONEAGENT_SECRETARY_ACK"):
		return "收到", nil
	default:
		return "ok", nil
	}
}

func (c scriptedClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestOrchestrator_CompressSU_DoesNotTouchSW(t *testing.T) {
	store, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	o := &Orchestrator{
		Sessions: store,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return scriptedClient{}, "mock", nil
		},
	}

	const suSessionID = "su"
	if _, err := store.GetOrCreateSession(suSessionID, "local", secretaryModuleSU, "t"); err != nil {
		t.Fatalf("create su session: %v", err)
	}

	// Preload enough history to exceed the fixed 80k compression threshold.
	big := strings.Repeat("好", 20_000)
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})

	swSessionID := deriveSWSessionID(suSessionID)
	if _, err := store.GetOrCreateSession(swSessionID, "local", secretaryModuleSW, "sw"); err != nil {
		t.Fatalf("create sw session: %v", err)
	}
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "small"})

	ctx := context.Background()
	_, err = o.AppendInboxMessage(ctx, "local", suSessionID, "hello", "")
	if err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	_, suMsgs, err := store.GetSessionWithMessages(suSessionID, "local")
	if err != nil {
		t.Fatalf("load su messages: %v", err)
	}
	if len(suMsgs) < 1 || !strings.HasPrefix(strings.TrimSpace(suMsgs[0].Content), sessioncompress.DefaultSummaryPrefix) {
		t.Fatalf("expected SU session to be compressed with prefix %q", sessioncompress.DefaultSummaryPrefix)
	}

	_, swMsgs, err := store.GetSessionWithMessages(swSessionID, "local")
	if err != nil {
		t.Fatalf("load sw messages: %v", err)
	}
	if len(swMsgs) != 1 {
		t.Fatalf("expected SW session to remain unchanged, got %d messages", len(swMsgs))
	}
	if strings.TrimSpace(swMsgs[0].Content) != "small" {
		t.Fatalf("expected SW content unchanged, got %q", swMsgs[0].Content)
	}
}

func TestOrchestrator_CompressSW_DoesNotTouchSU(t *testing.T) {
	store, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	o := &Orchestrator{
		Sessions: store,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return scriptedClient{}, "mock", nil
		},
	}

	const suSessionID = "su"
	if _, err := store.GetOrCreateSession(suSessionID, "local", secretaryModuleSU, "t"); err != nil {
		t.Fatalf("create su session: %v", err)
	}
	_, _ = store.AppendMessage(suSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "small"})

	swSessionID := deriveSWSessionID(suSessionID)
	if _, err := store.GetOrCreateSession(swSessionID, "local", secretaryModuleSW, "sw"); err != nil {
		t.Fatalf("create sw session: %v", err)
	}

	// Preload enough history to exceed the fixed 80k compression threshold.
	big := strings.Repeat("好", 20_000)
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: big})
	_, _ = store.AppendMessage(swSessionID, model.ChatMessage{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: big})

	plan, _, err := o.generateDispatchPlan(context.Background(), "local", suSessionID, model.JSONB{}, []model.ChatMessage{
		{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "do it"},
	})
	if err != nil {
		t.Fatalf("generateDispatchPlan: %v", err)
	}
	if strings.TrimSpace(plan.SummaryMessage) != "ok" {
		t.Fatalf("expected plan summary ok, got %q", plan.SummaryMessage)
	}

	_, suMsgs, err := store.GetSessionWithMessages(suSessionID, "local")
	if err != nil {
		t.Fatalf("load su messages: %v", err)
	}
	if len(suMsgs) != 1 || strings.TrimSpace(suMsgs[0].Content) != "small" {
		t.Fatalf("expected SU session unchanged, got %+v", suMsgs)
	}

	_, swMsgs, err := store.GetSessionWithMessages(swSessionID, "local")
	if err != nil {
		t.Fatalf("load sw messages: %v", err)
	}
	if len(swMsgs) < 1 || !strings.HasPrefix(strings.TrimSpace(swMsgs[0].Content), sessioncompress.DefaultSummaryPrefix) {
		t.Fatalf("expected SW session to be compressed with prefix %q", sessioncompress.DefaultSummaryPrefix)
	}
}
