package handler

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

const (
	sessionCompressionMaxContextRunes = 80_000
	sessionCompressionKeepTextMsgs    = 4 // last 2 rounds (user+assistant)*2

	sessionCompressionMaxSummaryInputRunes = 120_000
	sessionCompressionMaxMsgRunesForInput  = 4_000
)

func approximateContextRunes(messages []llm.ChatMessage) int {
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

func splitForCompression(dbMessages []model.ChatMessage, keepTextMessages int) (toSummarize []model.ChatMessage, toKeep []model.ChatMessage) {
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

func formatForSummaryInput(dbMessages []model.ChatMessage) string {
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

		content = truncateString(content, sessionCompressionMaxMsgRunesForInput)
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
	if utf8.RuneCountInString(out) > sessionCompressionMaxSummaryInputRunes {
		out = truncateString(out, sessionCompressionMaxSummaryInputRunes)
	}
	return out
}

func buildCompressionSummary(ctx context.Context, client llm.Client, input string) (string, error) {
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

func compressSessionIfNeeded(
	ctx context.Context,
	db *gorm.DB,
	sessionID string,
	dbMessages []model.ChatMessage,
	llmMessages []llm.ChatMessage,
	client llm.Client,
) (bool, []llm.ChatMessage, error) {
	if db == nil || client == nil {
		return false, llmMessages, nil
	}

	if approximateContextRunes(llmMessages) <= sessionCompressionMaxContextRunes {
		return false, llmMessages, nil
	}

	toSummarize, toKeep := splitForCompression(dbMessages, sessionCompressionKeepTextMsgs)
	if len(toSummarize) == 0 {
		return false, llmMessages, nil
	}

	input := formatForSummaryInput(toSummarize)
	summary, err := buildCompressionSummary(ctx, client, input)
	if err != nil {
		return false, llmMessages, err
	}
	if strings.TrimSpace(summary) == "" {
		summary = "流水账:\n- （摘要生成为空）\n\nFindings:\n- （摘要生成为空）"
	}

	summaryHeader := fmt.Sprintf("【会话压缩】已压缩 %d 条历史消息\n\n", len(toSummarize))
	summaryContent := summaryHeader + summary

	summaryCreatedAt := time.Now()
	if len(toKeep) > 0 && !toKeep[0].CreatedAt.IsZero() {
		summaryCreatedAt = toKeep[0].CreatedAt.Add(-1 * time.Millisecond)
	}

	keepIDs := make([]uint, 0, len(toKeep))
	for _, msg := range toKeep {
		keepIDs = append(keepIDs, msg.ID)
	}

	tx := db.Begin()
	if tx.Error != nil {
		return false, llmMessages, tx.Error
	}

	if len(keepIDs) > 0 {
		if err := tx.Where("session_id = ? AND id NOT IN ?", sessionID, keepIDs).Delete(&model.ChatMessage{}).Error; err != nil {
			tx.Rollback()
			return false, llmMessages, err
		}
	} else {
		if err := tx.Where("session_id = ?", sessionID).Delete(&model.ChatMessage{}).Error; err != nil {
			tx.Rollback()
			return false, llmMessages, err
		}
	}

	summaryMsg := model.ChatMessage{
		SessionID: sessionID,
		Role:      model.MessageRoleAssistant,
		Type:      model.MessageTypeText,
		Content:   summaryContent,
		CreatedAt: summaryCreatedAt,
	}
	if err := tx.Create(&summaryMsg).Error; err != nil {
		tx.Rollback()
		return false, llmMessages, err
	}

	if err := tx.Model(&model.ChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now()).Error; err != nil {
		tx.Rollback()
		return false, llmMessages, err
	}

	if err := tx.Commit().Error; err != nil {
		return false, llmMessages, err
	}

	// Rebuild prompt: keep current system + summary + tail text messages + current user message.
	if len(llmMessages) == 0 {
		return true, llmMessages, nil
	}

	newMessages := make([]llm.ChatMessage, 0, 2+len(toKeep)+1)
	newMessages = append(newMessages, llmMessages[0]) // system (already includes tool prompt additions)
	newMessages = append(newMessages, llm.BuildAssistantMessage(summaryContent))

	for _, msg := range toKeep {
		newMessages = append(newMessages, llm.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	// Preserve the current user message (last message in llmMessages).
	newMessages = append(newMessages, llmMessages[len(llmMessages)-1])

	return true, newMessages, nil
}
