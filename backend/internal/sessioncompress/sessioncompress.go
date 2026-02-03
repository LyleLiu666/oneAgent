package sessioncompress

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
)

const (
	DefaultMaxContextRunes = 80_000

	// Keep last 2 rounds (user+assistant)*2.
	DefaultKeepTextMessages = 4

	DefaultMaxSummaryInputRunes = 120_000
	DefaultMaxMsgRunesForInput  = 4_000

	DefaultSummaryPrefix = "【会话压缩】"
)

type FormatMessageContentFunc func(msg model.ChatMessage) string

type Options struct {
	MaxContextRunes      int
	KeepTextMessages     int
	MaxSummaryInputRunes int
	MaxMsgRunesForInput  int
	SummaryPrefix        string
	FormatMessageContent FormatMessageContentFunc
}

func DefaultOptions() Options {
	return Options{
		MaxContextRunes:      DefaultMaxContextRunes,
		KeepTextMessages:     DefaultKeepTextMessages,
		MaxSummaryInputRunes: DefaultMaxSummaryInputRunes,
		MaxMsgRunesForInput:  DefaultMaxMsgRunesForInput,
		SummaryPrefix:        DefaultSummaryPrefix,
		FormatMessageContent: func(msg model.ChatMessage) string {
			return strings.TrimSpace(msg.Content)
		},
	}
}

func normalizeOptions(opts Options) Options {
	def := DefaultOptions()
	if opts.MaxContextRunes <= 0 {
		opts.MaxContextRunes = def.MaxContextRunes
	}
	if opts.KeepTextMessages <= 0 {
		opts.KeepTextMessages = def.KeepTextMessages
	}
	if opts.MaxSummaryInputRunes <= 0 {
		opts.MaxSummaryInputRunes = def.MaxSummaryInputRunes
	}
	if opts.MaxMsgRunesForInput <= 0 {
		opts.MaxMsgRunesForInput = def.MaxMsgRunesForInput
	}
	if strings.TrimSpace(opts.SummaryPrefix) == "" {
		opts.SummaryPrefix = def.SummaryPrefix
	}
	if opts.FormatMessageContent == nil {
		opts.FormatMessageContent = def.FormatMessageContent
	}
	return opts
}

func ApproximateContextRunes(messages []llm.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += utf8.RuneCountInString(msg.Role)
		total += utf8.RuneCountInString(msg.Content)
		total += utf8.RuneCountInString(msg.Name)
		total += utf8.RuneCountInString(msg.ToolCallID)

		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				total += utf8.RuneCountInString(call.ID)
				total += utf8.RuneCountInString(call.Type)
				total += utf8.RuneCountInString(call.Function.Name)
				total += utf8.RuneCountInString(call.Function.Arguments)
			}
		}
	}
	return total
}

func SplitForCompression(dbMessages []model.ChatMessage, keepTextMessages int) (toSummarize []model.ChatMessage, toKeep []model.ChatMessage) {
	if keepTextMessages <= 0 || len(dbMessages) == 0 {
		return dbMessages, nil
	}

	keepIDs := make(map[uint]struct{}, keepTextMessages)
	for i := len(dbMessages) - 1; i >= 0 && len(keepIDs) < keepTextMessages; i-- {
		msg := dbMessages[i]
		if msg.Type != model.MessageTypeText {
			continue
		}
		if msg.Role != model.MessageRoleUser && msg.Role != model.MessageRoleAssistant {
			continue
		}
		keepIDs[msg.ID] = struct{}{}
	}

	toKeep = make([]model.ChatMessage, 0, len(keepIDs))
	toSummarize = make([]model.ChatMessage, 0, len(dbMessages)-len(keepIDs))

	for _, msg := range dbMessages {
		if _, ok := keepIDs[msg.ID]; ok {
			toKeep = append(toKeep, msg)
			continue
		}
		toSummarize = append(toSummarize, msg)
	}

	return toSummarize, toKeep
}

func FormatForSummaryInput(dbMessages []model.ChatMessage, opts Options) string {
	opts = normalizeOptions(opts)
	var b strings.Builder
	for _, msg := range dbMessages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		msgType := strings.TrimSpace(msg.Type)
		if msgType == "" {
			msgType = "text"
		}

		content := strings.TrimSpace(opts.FormatMessageContent(msg))
		content = truncateString(content, opts.MaxMsgRunesForInput)
		if content == "" {
			continue
		}

		fmt.Fprintf(&b, "[%d][%s][%s] %s\n", msg.ID, role, msgType, content)
	}

	out := b.String()
	if out == "" {
		return out
	}

	// Hard cap for safety (avoid blowing up summary prompt).
	if utf8.RuneCountInString(out) > opts.MaxSummaryInputRunes {
		out = truncateString(out, opts.MaxSummaryInputRunes)
	}
	return out
}

func BuildCompressionSummary(ctx context.Context, client llm.Client, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	maxTokens := 8192
	temperature := 0.0
	opts := &llm.ChatCompletionOptions{
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	}

	prompt := strings.TrimSpace(`
你是一个“会话压缩器”，目标是把一段对话历史压缩成可继续对话的摘要。
请只输出两段内容：
1) 流水账：按时间顺序的关键动作/事件（用条目列出）
2) Findings：重要结论、决定、约束、待办、已知事实（用条目列出）
要求：客观、可追溯到原对话，不要编造细节；尽量短但信息密度高；不要输出除这两段外的其他内容。
`)

	userMsg := "需要压缩的历史如下（按顺序）：\n\n```text\n" + input + "\n```\n"

	out, err := client.ChatCompletion(ctx, []llm.ChatMessage{
		llm.BuildSystemMessage(prompt),
		llm.BuildUserMessage(userMsg),
	}, opts)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func CompressSessionIfNeeded(
	ctx context.Context,
	sessions *sessionstore.Store,
	sessionID string,
	persistedMessages []model.ChatMessage,
	llmMessages []llm.ChatMessage,
	client llm.Client,
	opts Options,
) (bool, []llm.ChatMessage, error) {
	opts = normalizeOptions(opts)
	if sessions == nil || client == nil {
		return false, llmMessages, nil
	}

	if ApproximateContextRunes(llmMessages) <= opts.MaxContextRunes {
		return false, llmMessages, nil
	}

	toSummarize, toKeep := SplitForCompression(persistedMessages, opts.KeepTextMessages)
	if len(toSummarize) == 0 {
		return false, llmMessages, nil
	}

	input := FormatForSummaryInput(toSummarize, opts)
	summary, err := BuildCompressionSummary(ctx, client, input)
	if err != nil {
		return false, llmMessages, err
	}
	if strings.TrimSpace(summary) == "" {
		summary = "流水账:\n- （摘要生成为空）\n\nFindings:\n- （摘要生成为空）"
	}

	summaryHeader := fmt.Sprintf("%s已压缩 %d 条历史消息\n\n", opts.SummaryPrefix, len(toSummarize))
	summaryContent := summaryHeader + summary

	summaryCreatedAt := time.Now()
	if len(toKeep) > 0 && !toKeep[0].CreatedAt.IsZero() {
		summaryCreatedAt = toKeep[0].CreatedAt.Add(-1 * time.Millisecond)
	}

	summaryMsg := model.ChatMessage{
		SessionID: sessionID,
		Role:      model.MessageRoleAssistant,
		Type:      model.MessageTypeText,
		Content:   summaryContent,
		CreatedAt: summaryCreatedAt,
	}

	newStored := make([]model.ChatMessage, 0, 1+len(toKeep))
	newStored = append(newStored, summaryMsg)
	newStored = append(newStored, toKeep...)
	for i := range newStored {
		newStored[i].ID = uint(i + 1)
		newStored[i].ParentID = nil
		newStored[i].SessionID = sessionID
	}

	if err := sessions.ReplaceMessages(sessionID, newStored, time.Now(), uint(len(newStored)+1)); err != nil {
		return false, llmMessages, err
	}

	// Rebuild prompt: keep current system + summary + tail text messages + current user message.
	if len(llmMessages) == 0 {
		return true, llmMessages, nil
	}

	newMessages := make([]llm.ChatMessage, 0, 2+len(toKeep)+1)
	newMessages = append(newMessages, llmMessages[0]) // system (already includes tool prompt additions)
	newMessages = append(newMessages, llm.BuildSessionSummaryMessage(summaryContent))

	for _, msg := range toKeep {
		newMessages = append(newMessages, llm.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	// Preserve the current user message (last message in llmMessages).
	newMessages = append(newMessages, llmMessages[len(llmMessages)-1])

	return true, newMessages, nil
}

func BuildFallbackMessages(persistedMessages []model.ChatMessage, llmMessages []llm.ChatMessage, err error, opts Options) []llm.ChatMessage {
	opts = normalizeOptions(opts)
	if len(llmMessages) == 0 {
		return llmMessages
	}

	_, toKeep := SplitForCompression(persistedMessages, opts.KeepTextMessages)
	placeholder := fmt.Sprintf("%s摘要生成失败（%v），已仅保留最近两轮对话。", opts.SummaryPrefix, err)

	fallback := make([]llm.ChatMessage, 0, 2+len(toKeep)+1)
	fallback = append(fallback, llmMessages[0])
	fallback = append(fallback, llm.BuildAssistantMessage(placeholder))
	for _, msg := range toKeep {
		fallback = append(fallback, llm.ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	fallback = append(fallback, llmMessages[len(llmMessages)-1])

	return fallback
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		return ""
	}
	return string(runes[:maxLen])
}
