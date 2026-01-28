package handler

import (
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
	meta := make([]string, 0, 4)
	toolName = strings.TrimSpace(toolName)
	toolCallID = strings.TrimSpace(toolCallID)
	if toolName != "" {
		meta = append(meta, "tool="+toolName)
	}
	if toolCallID != "" {
		meta = append(meta, "tool_call_id="+toolCallID)
	}
	sourceLine := ""
	if len(meta) > 0 {
		sourceLine = "source: " + strings.Join(meta, " ") + "\n"
	}

	return strings.TrimSpace(fmt.Sprintf("%s\n%scontent:\n%s\n%s", untrustedContentBegin, sourceLine, raw, untrustedContentEnd))
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

