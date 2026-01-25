package handler

import (
	"encoding/json"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

type persistedToolCallMessage struct {
	Protocol   string         `json:"protocol,omitempty"`    // "json" or "xml"
	Content    string         `json:"content,omitempty"`     // UI-facing assistant content (e.g., without <tool_data> for xml)
	LLMContent string         `json:"llm_content,omitempty"` // Content to feed back into the LLM history (may include <tool_data> for xml)
	ToolCalls  []llm.ToolCall `json:"tool_calls,omitempty"`  // Structured tool calls (json protocol)
}

type persistedToolResult struct {
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	OK         bool   `json:"ok,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

type persistedToolResultMessage struct {
	Protocol   string                `json:"protocol,omitempty"`     // "json" or "xml"
	ToolCallID string                `json:"tool_call_id,omitempty"` // Only for json protocol
	Name       string                `json:"name,omitempty"`         // Only for json protocol
	Arguments  string                `json:"arguments,omitempty"`    // Only for json protocol
	Content    string                `json:"content,omitempty"`      // Tool output (json protocol) or <tool_result> block (xml protocol)
	Results    []persistedToolResult `json:"results,omitempty"`      // Optional structured results for UI
}

func marshalPersistedToolCall(protocol, content, llmContent string, toolCalls []llm.ToolCall) (string, error) {
	if strings.TrimSpace(llmContent) == "" {
		llmContent = content
	}
	payload := persistedToolCallMessage{
		Protocol:   strings.ToLower(strings.TrimSpace(protocol)),
		Content:    content,
		LLMContent: llmContent,
		ToolCalls:  toolCalls,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func marshalPersistedToolResult(protocol, toolCallID, name, arguments, content string, results []persistedToolResult) (string, error) {
	payload := persistedToolResultMessage{
		Protocol:   strings.ToLower(strings.TrimSpace(protocol)),
		ToolCallID: toolCallID,
		Name:       name,
		Arguments:  arguments,
		Content:    content,
		Results:    results,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parsePersistedToolCall(raw string) (persistedToolCallMessage, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return persistedToolCallMessage{}, false
	}
	var payload persistedToolCallMessage
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return persistedToolCallMessage{}, false
	}
	return payload, true
}

func parsePersistedToolResult(raw string) (persistedToolResultMessage, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return persistedToolResultMessage{}, false
	}
	var payload persistedToolResultMessage
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return persistedToolResultMessage{}, false
	}
	return payload, true
}

func buildLLMHistoryFromMessages(messages []model.ChatMessage, toolProtocol string) []llm.ChatMessage {
	protocol := strings.ToLower(strings.TrimSpace(toolProtocol))
	out := make([]llm.ChatMessage, 0, len(messages))

	for _, msg := range messages {
		switch msg.Type {
		case model.MessageTypeToolCall:
			payload, ok := parsePersistedToolCall(msg.Content)
			if ok && payload.Protocol != "" && protocol != "" && payload.Protocol != protocol {
				continue
			}

			content := msg.Content
			var toolCalls []llm.ToolCall
			if ok {
				content = payload.LLMContent
				if strings.TrimSpace(content) == "" {
					content = payload.Content
				}
				toolCalls = payload.ToolCalls
			}

			llmMsg := llm.ChatMessage{Role: msg.Role, Content: content}
			if protocol == "json" && len(toolCalls) > 0 {
				llmMsg.ToolCalls = toolCalls
			}
			out = append(out, llmMsg)

		case model.MessageTypeToolResult:
			payload, ok := parsePersistedToolResult(msg.Content)
			if ok && payload.Protocol != "" && protocol != "" && payload.Protocol != protocol {
				continue
			}

			content := msg.Content
			if ok {
				content = payload.Content
			}

			llmMsg := llm.ChatMessage{Role: msg.Role, Content: content}
			if protocol == "json" && ok {
				llmMsg.ToolCallID = payload.ToolCallID
				llmMsg.Name = payload.Name
			}
			out = append(out, llmMsg)

		default:
			out = append(out, llm.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return out
}
