package toolxml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type FailureRecorder func(toolName, toolCallID, args string, err error)

type StepRecord struct {
	VisibleContent    string
	AssistantContent  string
	ToolCalls         []llm.ToolCall
	ToolResults       []ToolResult
	ToolResultMessage string
}

type StepObserver func(StepRecord)

type ToolResult struct {
	ToolName   string
	ToolCallID string
	OK         bool
	OutputJSON string
	Error      string
}

func RunLoop(
	ctx context.Context,
	client llm.Client,
	messages []llm.ChatMessage,
	opts *llm.ChatCompletionOptions,
	defs []tool.Definition,
	onContent llm.StreamCallback,
	onTrace func(string),
	onError func(string),
	recordFailure FailureRecorder,
	observeStep StepObserver,
	observeFinal func(visibleContent, assistantContent string),
	onStepStart func(step int),
) (string, error) {
	if client == nil {
		return "", errors.New("missing llm client")
	}
	if len(defs) == 0 {
		return "", errors.New("no tools configured")
	}

	handlers := make(map[string]tool.Handler, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Spec.Function.Name)
		if name == "" {
			continue
		}
		handlers[name] = def.Handler
	}
	if len(handlers) == 0 {
		return "", errors.New("no tool handlers registered")
	}

	var combined strings.Builder

	const maxSteps = 20
	for step := 0; step < maxSteps; step++ {
		if onStepStart != nil {
			onStepStart(step)
		}
		var raw strings.Builder
		var visible strings.Builder
		filter := &streamFilter{}

		err := client.ChatCompletionStream(ctx, messages, opts, func(chunk string) error {
			raw.WriteString(chunk)
			emitted := filter.Feed(chunk)
			if emitted == "" {
				return nil
			}
			visible.WriteString(emitted)
			if onContent != nil {
				return onContent(emitted)
			}
			return nil
		})
		if tail := filter.Flush(); tail != "" {
			visible.WriteString(tail)
			if onContent != nil {
				if cbErr := onContent(tail); cbErr != nil && err == nil {
					err = cbErr
				}
			}
		}

		stepVisible := visible.String()
		if stepVisible != "" {
			combined.WriteString(stepVisible)
		}

		if err != nil {
			return combined.String(), err
		}

		assistantForHistory := strings.TrimSpace(StripThinking(raw.String()))
		if assistantForHistory == "" {
			assistantForHistory = stepVisible
		}

		toolBlock, ok := ExtractLatestToolData(raw.String())
		if !ok {
			if observeFinal != nil {
				observeFinal(stepVisible, assistantForHistory)
			}
			return combined.String(), nil
		}

		calls, err := ParseToolData(toolBlock)
		if err != nil {
			if onError != nil {
				onError(fmt.Sprintf("Tool protocol parse error: %v", err))
			}
			return combined.String(), err
		}
		messages = append(messages, llm.ChatMessage{
			Role:    "assistant",
			Content: assistantForHistory,
		})

		results := make([]ToolResult, 0, len(calls))
		recordedCalls := make([]llm.ToolCall, 0, len(calls))
		for idx, call := range calls {
			toolName := strings.TrimSpace(call.ToolName)
			toolCallID := fmt.Sprintf("xml_%d_%d", step, idx)
			recordedCalls = append(recordedCalls, llm.ToolCall{
				ID:   toolCallID,
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      toolName,
					Arguments: "",
				},
			})

			handler, ok := handlers[toolName]
			if !ok {
				err := fmt.Errorf("unknown tool: %s", toolName)
				if onTrace != nil {
					onTrace(fmt.Sprintf("Tool error: %v", err))
				}
				if recordFailure != nil {
					recordFailure(toolName, toolCallID, call.Raw, err)
				}
				results = append(results, ToolResult{
					ToolName:   toolName,
					ToolCallID: toolCallID,
					OK:         false,
					Error:      err.Error(),
					OutputJSON: `{"error":"unknown tool"}`,
				})
				continue
			}

			if onTrace != nil {
				onTrace(fmt.Sprintf("Running tool: %s", toolName))
			}

			args, argsString, err := buildToolArgs(toolName, call.Fields)
			if err != nil {
				if onTrace != nil {
					onTrace(fmt.Sprintf("Tool %s args error: %v", toolName, err))
				}
				if recordFailure != nil {
					recordFailure(toolName, toolCallID, call.Raw, err)
				}
				results = append(results, ToolResult{
					ToolName:   toolName,
					ToolCallID: toolCallID,
					OK:         false,
					Error:      err.Error(),
					OutputJSON: fmt.Sprintf(`{"error":%q}`, err.Error()),
				})
				continue
			}
			recordedCalls[len(recordedCalls)-1].Function.Arguments = argsString

			payload, handlerErr := handler(ctx, args)
			if handlerErr != nil {
				if onTrace != nil {
					onTrace(fmt.Sprintf("Tool %s failed: %v", toolName, handlerErr))
				}
				if recordFailure != nil {
					recordFailure(toolName, toolCallID, formatFailureArgs(call.Raw, argsString), handlerErr)
				}
				payload = map[string]string{
					"error": fmt.Sprintf("Tool execution failed: %v", handlerErr),
				}
			}

			response, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				if onTrace != nil {
					onTrace(fmt.Sprintf("Tool %s response error: %v", toolName, marshalErr))
				}
				if recordFailure != nil {
					recordFailure(toolName, toolCallID, formatFailureArgs(call.Raw, argsString), marshalErr)
				}
				response = []byte(fmt.Sprintf(`{"error":"failed to marshal tool response: %v"}`, marshalErr))
			}

			result := ToolResult{
				ToolName:   toolName,
				ToolCallID: toolCallID,
				OK:         handlerErr == nil && marshalErr == nil,
				OutputJSON: string(response),
			}
			if handlerErr != nil {
				result.Error = handlerErr.Error()
			}
			if marshalErr != nil {
				if result.Error == "" {
					result.Error = marshalErr.Error()
				} else {
					result.Error = result.Error + "; " + marshalErr.Error()
				}
			}

			results = append(results, result)
		}

		toolResultMsg := buildToolResultMessage(results)
		if observeStep != nil {
			observeStep(StepRecord{
				VisibleContent:    stepVisible,
				AssistantContent:  assistantForHistory,
				ToolCalls:         recordedCalls,
				ToolResults:       results,
				ToolResultMessage: toolResultMsg,
			})
		}
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: toolResultMsg,
		})
	}

	err := errors.New("xml tool call limit reached")
	if recordFailure != nil {
		recordFailure("", "", "", err)
	}
	if onError != nil {
		onError(err.Error())
	}
	return combined.String(), err
}

func formatFailureArgs(xmlCall, normalized string) string {
	xmlCall = strings.TrimSpace(xmlCall)
	normalized = strings.TrimSpace(normalized)
	if xmlCall == "" {
		return normalized
	}
	if normalized == "" {
		return xmlCall
	}
	return xmlCall + "\n\n--normalized_args--\n" + normalized
}

func buildToolResultMessage(results []ToolResult) string {
	var b strings.Builder
	b.WriteString("<tool_result>\n")
	for _, r := range results {
		b.WriteString("  <call>\n")
		b.WriteString("    <tool_name>")
		b.WriteString(escapeXMLText(r.ToolName))
		b.WriteString("</tool_name>\n")
		b.WriteString("    <tool_call_id>")
		b.WriteString(escapeXMLText(r.ToolCallID))
		b.WriteString("</tool_call_id>\n")
		b.WriteString("    <ok>")
		if r.OK {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString("</ok>\n")
		if r.Error != "" {
			b.WriteString("    <error><![CDATA[")
			b.WriteString(r.Error)
			b.WriteString("]]></error>\n")
		}
		if r.OutputJSON != "" {
			b.WriteString("    <output><![CDATA[")
			b.WriteString(r.OutputJSON)
			b.WriteString("]]></output>\n")
		}
		b.WriteString("  </call>\n")
	}
	b.WriteString("</tool_result>\n")
	return b.String()
}

func escapeXMLText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func buildToolArgs(toolName string, fields map[string]string) (json.RawMessage, string, error) {
	switch toolName {
	case "bash":
		command := strings.TrimSpace(fields["command"])
		if command == "" {
			return nil, "", errors.New("missing command")
		}
		payload := map[string]any{
			"command": command,
		}
		if rawTimeout := strings.TrimSpace(fields["timeout_ms"]); rawTimeout != "" {
			if timeout, err := strconv.Atoi(rawTimeout); err == nil && timeout > 0 {
				payload["timeout_ms"] = timeout
			}
		}

		data, err := json.Marshal(payload)
		return data, string(data), err

	case "edit":
		filePath := strings.TrimSpace(fields["filePath"])
		replaceAll := parseBool(fields["replaceAll"])

		if command := fields["command"]; strings.TrimSpace(command) != "" {
			payload := map[string]any{
				"command":    command,
				"replaceAll": replaceAll,
			}
			data, err := json.Marshal(payload)
			return data, string(data), err
		}

		if filePath != "" && strings.TrimSpace(fields["content"]) != "" {
			content := fields["content"]
			lines := make([]string, 0, 4+strings.Count(content, "\n"))
			lines = append(lines, fmt.Sprintf("cat >%s <<'EOF'", quoteHeredocPath(filePath)))
			lines = append(lines, strings.Split(content, "\n")...)
			lines = append(lines, "EOF")

			payload := map[string]any{
				"command": lines,
			}
			data, err := json.Marshal(payload)
			return data, string(data), err
		}

		oldContent := fields["oldcontent"]
		newContent := fields["newcontent"]
		if filePath == "" || oldContent == "" {
			return nil, "", errors.New("edit requires filePath + oldcontent/newcontent, or filePath + content, or command")
		}

		payload := map[string]any{
			"filePath":   filePath,
			"oldString":  oldContent,
			"newString":  newContent,
			"replaceAll": replaceAll,
		}
		data, err := json.Marshal(payload)
		return data, string(data), err
	default:
		return nil, "", fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func parseBool(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	b, err := strconv.ParseBool(trimmed)
	return err == nil && b
}

func quoteHeredocPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.ContainsAny(trimmed, " \t\r\n\"'") {
		escaped := strings.ReplaceAll(trimmed, "'", `'\''`)
		return "'" + escaped + "'"
	}
	return trimmed
}
