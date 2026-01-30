package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/model"
)

const (
	untrustedContentBegin = "BEGIN_UNTRUSTED_CONTENT"
	untrustedContentEnd   = "END_UNTRUSTED_CONTENT"
)

func wrapUntrustedToolOutput(toolName, toolCallID, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	envelope := buildUntrustedToolEnvelope(toolName, toolCallID, raw)
	if strings.TrimSpace(envelope) == "" {
		return raw
	}
	return strings.TrimSpace(fmt.Sprintf("%s\n%s\n%s", untrustedContentBegin, envelope, untrustedContentEnd))
}

type suspiciousMatch struct {
	RuleID string
	Kind   string
	Snippet string
}

var suspiciousRules = []struct {
	id   string
	kind string
	re   *regexp.Regexp
}{
	{
		id:   "injection_ignore_previous",
		kind: "prompt_injection",
		re:   regexp.MustCompile(`(?i)\b(ignore|disregard)\b[\s\S]{0,120}\b(previous|prior)\b[\s\S]{0,80}\b(instruction|message|system)\b`),
	},
	{
		id:   "injection_reveal_system_prompt",
		kind: "prompt_injection",
		re:   regexp.MustCompile(`(?i)\b(reveal|show|print|leak)\b[\s\S]{0,80}\b(system|developer)\b[\s\S]{0,40}\bprompt\b`),
	},
}

func detectSuspiciousPatterns(text string) []suspiciousMatch {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	out := make([]suspiciousMatch, 0, 2)
	for _, rule := range suspiciousRules {
		loc := rule.re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		snippet := strings.TrimSpace(text[loc[0]:loc[1]])
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		out = append(out, suspiciousMatch{
			RuleID:  rule.id,
			Kind:    rule.kind,
			Snippet: snippet,
		})
	}
	return out
}

type untrustedToolEnvelope struct {
	Untrusted bool `json:"_untrusted"`
	Source    struct {
		Kind       string `json:"kind"`
		Tool       string `json:"tool,omitempty"`
		ToolCallID string `json:"tool_call_id,omitempty"`
	} `json:"source"`

	OK bool `json:"ok"`

	Output json.RawMessage `json:"output,omitempty"`
	Error  *struct {
		ErrorCode string `json:"error_code,omitempty"`
		Retryable bool   `json:"retryable,omitempty"`
		Message   string `json:"message,omitempty"`
		Hint      string `json:"hint,omitempty"`
	} `json:"error,omitempty"`
}

func buildUntrustedToolEnvelope(toolName, toolCallID, raw string) string {
	toolName = strings.TrimSpace(toolName)
	toolCallID = strings.TrimSpace(toolCallID)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	out := untrustedToolEnvelope{
		Untrusted: true,
		OK:        true,
	}
	out.Source.Kind = "tool"
	out.Source.Tool = toolName
	out.Source.ToolCallID = toolCallID

	// Keep tool output machine-parseable: embed as JSON when possible, otherwise as JSON string.
	if json.Valid([]byte(raw)) {
		out.Output = json.RawMessage(raw)
	} else {
		b, _ := json.Marshal(raw)
		out.Output = b
	}

	// Best-effort "ok"/error extraction.
	if strings.HasPrefix(strings.TrimSpace(raw), "{") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			if v, ok := obj["ok"].(bool); ok {
				out.OK = v
			} else if _, ok := obj["error"]; ok {
				out.OK = false
			}
			if !out.OK {
				errMsg := ""
				if s, ok := obj["error"].(string); ok {
					errMsg = strings.TrimSpace(s)
				}
				code, hint, retryable := classifyToolOutputError(errMsg)
				out.Error = &struct {
					ErrorCode string `json:"error_code,omitempty"`
					Retryable bool   `json:"retryable,omitempty"`
					Message   string `json:"message,omitempty"`
					Hint      string `json:"hint,omitempty"`
				}{
					ErrorCode: code,
					Retryable: retryable,
					Message:   errMsg,
					Hint:      hint,
				}
			}
		}
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func classifyToolOutputError(message string) (code string, hint string, retryable bool) {
	msg := strings.ToLower(strings.TrimSpace(message))
	switch {
	case msg == "":
		return "tool_error", "", true
	case strings.Contains(msg, "approval_required"):
		return "approval_required", "该工具需要审批；请在“审批”中允许后重试", true
	case strings.Contains(msg, "approval_denied"):
		return "approval_denied", "该工具审批被拒绝；如需继续请调整权限策略后重试", false
	case strings.Contains(msg, "unknown tool"):
		return "unknown_tool", "模型调用了不存在的工具；请让模型改用可用工具或重试", false
	case strings.Contains(msg, "not allowed") || strings.Contains(msg, "denied"):
		return "policy_denied", "操作被策略拒绝；请调整工具权限/policy/profile 后重试", false
	case strings.Contains(msg, "cannot unmarshal") || strings.Contains(msg, "invalid character") || strings.Contains(msg, "unexpected end of json"):
		return "invalid_arguments", "工具参数不是合法 JSON；请只返回一个 JSON 对象，不要解释文本", true
	default:
		return "tool_error", "", true
	}
}

func recordSuspiciousMatches(
	broadcaster *StreamBroadcaster,
	entries *[]model.TraceEntry,
	sessionID string,
	userID string,
	source string,
	toolName string,
	toolCallID string,
	content string,
) {
	matches := detectSuspiciousPatterns(content)
	if len(matches) == 0 {
		return
	}

	for _, m := range matches {
		log.Printf("security_alert session_id=%s user_id=%s kind=%s rule=%s source=%s tool=%s tool_call_id=%s snippet=%q",
			strings.TrimSpace(sessionID),
			strings.TrimSpace(userID),
			m.Kind,
			m.RuleID,
			strings.TrimSpace(source),
			strings.TrimSpace(toolName),
			strings.TrimSpace(toolCallID),
			m.Snippet,
		)

		if entries != nil {
			e := model.NewTraceEntry(model.TraceTypeCustom, "SecurityAlert")
			e.Input = map[string]any{
				"source":        source,
				"tool_name":     toolName,
				"tool_call_id":  toolCallID,
				"rule_id":       m.RuleID,
				"kind":          m.Kind,
			}
			e.Output = map[string]any{
				"snippet": m.Snippet,
			}
			e.Metadata["rule_id"] = m.RuleID
			e.Metadata["kind"] = m.Kind
			e.Metadata["source"] = source
			if strings.TrimSpace(toolName) != "" {
				e.Metadata["tool"] = strings.TrimSpace(toolName)
			}
			if strings.TrimSpace(toolCallID) != "" {
				e.Metadata["tool_call_id"] = strings.TrimSpace(toolCallID)
			}
			e.Complete()
			*entries = append(*entries, e)
		}
	}

	if broadcaster != nil {
		broadcaster.Broadcast(StreamEvent{
			Type: "trace",
			Data: fmt.Sprintf("Security alert: suspicious untrusted content detected (%d match)", len(matches)),
		})
	}
}
