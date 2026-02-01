package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
)

type fakeSummaryClient struct {
	out string
	err error
}

func (c fakeSummaryClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	return c.out, c.err
}

func (c fakeSummaryClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestCompressSessionIfNeeded_RewritesSessionAndPreservesCurrentUserMessage(t *testing.T) {
	t.Parallel()

	t.Setenv("ONEAGENT_SESSION_COMPRESSION_MAX_CONTEXT_RUNES", "200")

	store, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	sessionID := "s1"
	if _, err := store.GetOrCreateSession(sessionID, "u1", "chat", "t"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	now := time.Now()
	persisted := []model.ChatMessage{
		{ID: 1, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u1", CreatedAt: now.Add(-6 * time.Minute)},
		{ID: 2, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a1", CreatedAt: now.Add(-5 * time.Minute)},
		{ID: 3, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u2", CreatedAt: now.Add(-4 * time.Minute)},
		{ID: 4, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a2", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: 5, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u3", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: 6, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a3", CreatedAt: now.Add(-1 * time.Minute)},
	}

	currentUser := strings.Repeat("a", sessionCompressionMaxContextRunes()+1)
	llmMessages := []llm.ChatMessage{
		llm.BuildSystemMessage("sys"),
		llm.BuildUserMessage(currentUser),
	}

	client := fakeSummaryClient{out: "流水账:\n- did x\n\nFindings:\n- decided y"}

	compressed, newMsgs, err := compressSessionIfNeeded(context.Background(), store, sessionID, persisted, llmMessages, client)
	if err != nil {
		t.Fatalf("compressSessionIfNeeded: %v", err)
	}
	if !compressed {
		t.Fatalf("expected compressed=true")
	}

	if len(newMsgs) != 7 {
		t.Fatalf("expected 7 prompt messages, got %d", len(newMsgs))
	}

	if newMsgs[0].Role != model.MessageRoleSystem {
		t.Fatalf("expected first prompt message to be system, got %q", newMsgs[0].Role)
	}
	if !strings.HasPrefix(strings.TrimSpace(newMsgs[1].Content), sessionCompressionSummaryPrefix) {
		t.Fatalf("expected summary message to start with %q", sessionCompressionSummaryPrefix)
	}
	if !strings.Contains(newMsgs[1].Content, "已压缩 2 条历史消息") {
		t.Fatalf("expected summary header to mention compressed count")
	}
	if !strings.Contains(newMsgs[1].Content, "流水账") || !strings.Contains(newMsgs[1].Content, "Findings") {
		t.Fatalf("expected summary to contain timeline/findings sections")
	}

	last := newMsgs[len(newMsgs)-1]
	if last.Role != model.MessageRoleUser {
		t.Fatalf("expected last prompt message to be user, got %q", last.Role)
	}
	if last.Content != currentUser {
		t.Fatalf("expected current user message to be preserved")
	}

	_, storedMsgs, err := store.GetSessionWithMessages(sessionID, "")
	if err != nil {
		t.Fatalf("load stored messages: %v", err)
	}

	if len(storedMsgs) != 5 {
		t.Fatalf("expected 5 stored messages after rewrite (summary + 4 tail), got %d", len(storedMsgs))
	}
	if !strings.HasPrefix(strings.TrimSpace(storedMsgs[0].Content), sessionCompressionSummaryPrefix) {
		t.Fatalf("expected stored summary to start with %q", sessionCompressionSummaryPrefix)
	}
}

func TestBuildCompressionFallbackMessages_PreservesCurrentUserMessageAndTail(t *testing.T) {
	t.Parallel()

	t.Setenv("ONEAGENT_SESSION_COMPRESSION_MAX_CONTEXT_RUNES", "200")

	sessionID := "s1"
	now := time.Now()
	persisted := []model.ChatMessage{
		{ID: 1, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u1", CreatedAt: now.Add(-6 * time.Minute)},
		{ID: 2, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a1", CreatedAt: now.Add(-5 * time.Minute)},
		{ID: 3, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u2", CreatedAt: now.Add(-4 * time.Minute)},
		{ID: 4, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a2", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: 5, SessionID: sessionID, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u3", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: 6, SessionID: sessionID, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a3", CreatedAt: now.Add(-1 * time.Minute)},
	}

	currentUser := strings.Repeat("b", sessionCompressionMaxContextRunes()+1)
	llmMessages := []llm.ChatMessage{
		llm.BuildSystemMessage("sys"),
		llm.BuildUserMessage(currentUser),
	}

	out := buildCompressionFallbackMessages(persisted, llmMessages, errors.New("boom"))
	if len(out) != 7 {
		t.Fatalf("expected 7 fallback prompt messages, got %d", len(out))
	}

	if out[0].Role != model.MessageRoleSystem {
		t.Fatalf("expected first fallback message to be system, got %q", out[0].Role)
	}
	if out[1].Role != model.MessageRoleAssistant {
		t.Fatalf("expected second fallback message to be assistant, got %q", out[1].Role)
	}
	if !strings.Contains(out[1].Content, "摘要生成失败") || !strings.Contains(out[1].Content, "boom") {
		t.Fatalf("expected fallback placeholder to include failure reason")
	}

	if out[len(out)-1].Role != model.MessageRoleUser || out[len(out)-1].Content != currentUser {
		t.Fatalf("expected fallback to preserve current user message")
	}
}
