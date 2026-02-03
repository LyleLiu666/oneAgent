package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessioncompress"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
)

const (
	defaultSessionCompressionMaxContextRunes = sessioncompress.DefaultMaxContextRunes
	sessionCompressionKeepTextMsgs           = sessioncompress.DefaultKeepTextMessages // last 2 rounds (user+assistant)*2

	sessionCompressionMaxSummaryInputRunes = sessioncompress.DefaultMaxSummaryInputRunes
	sessionCompressionMaxMsgRunesForInput  = sessioncompress.DefaultMaxMsgRunesForInput

	sessionCompressionSummaryPrefix = sessioncompress.DefaultSummaryPrefix
)

// sessionCompressionMaxContextRunes returns the threshold after which the system will attempt to compress
// the chat history. This is intentionally fixed (80k).
func sessionCompressionMaxContextRunes() int {
	return defaultSessionCompressionMaxContextRunes
}

func approximateContextRunes(messages []llm.ChatMessage) int {
	return sessioncompress.ApproximateContextRunes(messages)
}

func compressionOptions() sessioncompress.Options {
	return sessioncompress.Options{
		MaxContextRunes:      sessionCompressionMaxContextRunes(),
		KeepTextMessages:     sessionCompressionKeepTextMsgs,
		MaxSummaryInputRunes: sessionCompressionMaxSummaryInputRunes,
		MaxMsgRunesForInput:  sessionCompressionMaxMsgRunesForInput,
		SummaryPrefix:        sessionCompressionSummaryPrefix,
		FormatMessageContent: formatMessageContentForCompression,
	}
}

func formatMessageContentForCompression(msg model.ChatMessage) string {
	content := strings.TrimSpace(msg.Content)
	switch msg.Type {
	case model.MessageTypeToolCall:
		if payload, ok := parsePersistedToolCall(msg.Content); ok {
			var lines []string
			if strings.TrimSpace(payload.Content) != "" {
				lines = append(lines, "assistant_visible: "+strings.TrimSpace(payload.Content))
			}
			if len(payload.ToolCalls) > 0 {
				for _, call := range payload.ToolCalls {
					args := strings.TrimSpace(call.Function.Arguments)
					if args != "" {
						args = truncateString(args, 800)
					}
					lines = append(lines, fmt.Sprintf("tool_call: %s args=%s", call.Function.Name, args))
				}
			}
			content = strings.Join(lines, "\n")
		}

	case model.MessageTypeToolResult:
		if payload, ok := parsePersistedToolResult(msg.Content); ok {
			if strings.TrimSpace(payload.Name) != "" {
				content = fmt.Sprintf("tool_result: %s\n%s", payload.Name, strings.TrimSpace(payload.Content))
			} else {
				content = strings.TrimSpace(payload.Content)
			}
		}
	}

	return strings.TrimSpace(content)
}

func compressSessionIfNeeded(
	ctx context.Context,
	sessions *sessionstore.Store,
	sessionID string,
	persistedMessages []model.ChatMessage,
	llmMessages []llm.ChatMessage,
	client llm.Client,
) (bool, []llm.ChatMessage, error) {
	return sessioncompress.CompressSessionIfNeeded(ctx, sessions, sessionID, persistedMessages, llmMessages, client, compressionOptions())
}

func buildCompressionFallbackMessages(persistedMessages []model.ChatMessage, llmMessages []llm.ChatMessage, err error) []llm.ChatMessage {
	return sessioncompress.BuildFallbackMessages(persistedMessages, llmMessages, err, compressionOptions())
}
