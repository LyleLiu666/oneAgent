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
	"github.com/liu_y/oneAgent/backend/internal/toolcalling"
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
	userID string,
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

	// Inject userID into context for tool handlers to access user-specific settings
	if userID != "" {
		ctx = tool.ContextWithUserID(ctx, userID)
	}

	var combined strings.Builder

	maxSteps := toolcalling.ChatToolMaxSteps()
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
			if indexCaseInsensitive(StripThinking(raw.String()), toolDataStart) != -1 {
				protoErr := errors.New("truncated <tool_data> block")
				if onTrace != nil {
					onTrace(fmt.Sprintf("Tool protocol error: %v", protoErr))
				}

				toolName := "tool_protocol"
				toolCallID := fmt.Sprintf("xml_%d_protocol", step)
				result := ToolResult{
					ToolName:   toolName,
					ToolCallID: toolCallID,
					OK:         false,
					Error:      protoErr.Error(),
					OutputJSON: fmt.Sprintf(`{"error":%q}`, protoErr.Error()),
				}
				recordedCall := llm.ToolCall{
					ID:   toolCallID,
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      toolName,
						Arguments: fmt.Sprintf(`{"error":%q}`, protoErr.Error()),
					},
				}

				toolResultMsg := buildToolResultMessage([]ToolResult{result})
				if recordFailure != nil {
					recordFailure(toolName, toolCallID, assistantForHistory, protoErr)
				}
				if observeStep != nil {
					observeStep(StepRecord{
						VisibleContent:    stepVisible,
						AssistantContent:  assistantForHistory,
						ToolCalls:         []llm.ToolCall{recordedCall},
						ToolResults:       []ToolResult{result},
						ToolResultMessage: toolResultMsg,
					})
				}

				messages = append(messages, llm.ChatMessage{
					Role:    "assistant",
					Content: assistantForHistory,
				})
				messages = append(messages, llm.ChatMessage{
					Role:    "user",
					Content: toolResultMsg,
				})
				continue
			}
			if observeFinal != nil {
				observeFinal(stepVisible, assistantForHistory)
			}
			return combined.String(), nil
		}

		calls, err := ParseToolData(toolBlock)
		if err != nil {
			if onTrace != nil {
				onTrace(fmt.Sprintf("Tool protocol parse error: %v", err))
			}

			toolName := "tool_protocol"
			toolCallID := fmt.Sprintf("xml_%d_protocol", step)
			result := ToolResult{
				ToolName:   toolName,
				ToolCallID: toolCallID,
				OK:         false,
				Error:      err.Error(),
				OutputJSON: fmt.Sprintf(`{"error":%q}`, err.Error()),
			}
			recordedCall := llm.ToolCall{
				ID:   toolCallID,
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      toolName,
					Arguments: fmt.Sprintf(`{"error":%q}`, err.Error()),
				},
			}

			toolResultMsg := buildToolResultMessage([]ToolResult{result})
			if recordFailure != nil {
				recordFailure(toolName, toolCallID, toolBlock, err)
			}
			if observeStep != nil {
				observeStep(StepRecord{
					VisibleContent:    stepVisible,
					AssistantContent:  assistantForHistory,
					ToolCalls:         []llm.ToolCall{recordedCall},
					ToolResults:       []ToolResult{result},
					ToolResultMessage: toolResultMsg,
				})
			}

			messages = append(messages, llm.ChatMessage{
				Role:    "assistant",
				Content: assistantForHistory,
			})
			messages = append(messages, llm.ChatMessage{
				Role:    "user",
				Content: toolResultMsg,
			})
			continue
		}
		messages = append(messages, llm.ChatMessage{
			Role:    "assistant",
			Content: assistantForHistory,
		})

		results := make([]ToolResult, 0, len(calls))
		recordedCalls := make([]llm.ToolCall, 0, len(calls))
		for idx, call := range calls {
			toolName := strings.TrimSpace(call.ToolName)
			canonicalName := tool.CanonicalToolName(toolName)
			toolCallID := fmt.Sprintf("xml_%d_%d", step, idx)
			recordedCalls = append(recordedCalls, llm.ToolCall{
				ID:   toolCallID,
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      toolName,
					Arguments: "",
				},
			})

			handler, ok := handlers[canonicalName]
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

			args, argsString, err := buildToolArgs(canonicalName, call.Fields)
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
				var approvalRequired *tool.ApprovalRequiredError
				var approvalDenied *tool.ApprovalDeniedError
				var invalidArgs *tool.InvalidArgumentsError
				if errors.As(handlerErr, &approvalRequired) {
					payload = map[string]any{
						"error":             "approval_required",
						"approval_required": true,
						"approval_id":       approvalRequired.ApprovalID,
						"tool_id":           approvalRequired.ToolID,
						"scope_id":          approvalRequired.ScopeID,
						"arguments":         argsString,
					}
				} else if errors.As(handlerErr, &approvalDenied) {
					payload = map[string]any{
						"error":           "approval_denied",
						"approval_denied": true,
						"approval_id":     approvalDenied.ApprovalID,
						"tool_id":         approvalDenied.ToolID,
						"scope_id":        approvalDenied.ScopeID,
						"reason":          approvalDenied.Reason,
						"arguments":       argsString,
					}
				} else if errors.As(handlerErr, &invalidArgs) {
					payload = map[string]any{
						"error":            "invalid_arguments",
						"invalid_args":     true,
						"message":          strings.TrimSpace(handlerErr.Error()),
						"missing_fields":   invalidArgs.MissingFields,
						"raw_arguments":    argsString,
						"tool_call_id":     toolCallID,
						"tool_name":        toolName,
						"tool_schema_hint": "Ensure all required fields are present and XML fields are correctly filled.",
					}
				} else {
					payload = map[string]string{
						"error": fmt.Sprintf("Tool execution failed: %v", handlerErr),
					}
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

	err := fmt.Errorf("xml tool call limit reached (max_steps=%d)", maxSteps)
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
			b.WriteString("    <error>")
			b.WriteString(escapeXMLText(r.Error))
			b.WriteString("</error>\n")
		}
		if r.OutputJSON != "" {
			b.WriteString("    <output>")
			b.WriteString(escapeXMLText(r.OutputJSON))
			b.WriteString("</output>\n")
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

	case "run_command":
		action := strings.ToLower(strings.TrimSpace(fields["action"]))
		command := strings.TrimSpace(fields["command"])
		jobID := strings.TrimSpace(fields["job_id"])

		if action == "" {
			if command != "" {
				action = "start"
			} else if jobID != "" {
				action = "poll"
			}
		}
		if action == "" {
			return nil, "", errors.New("missing action")
		}

		payload := map[string]any{
			"action": action,
		}
		if command != "" {
			payload["command"] = command
		}
		if jobID != "" {
			payload["job_id"] = jobID
		}

		if rawWait := strings.TrimSpace(fields["wait_seconds"]); rawWait != "" {
			if wait, err := strconv.Atoi(rawWait); err == nil && wait >= 0 {
				payload["wait_seconds"] = wait
			}
		} else if rawWait := strings.TrimSpace(fields["wait_ms"]); rawWait != "" {
			if waitMs, err := strconv.Atoi(rawWait); err == nil && waitMs >= 0 {
				payload["wait_seconds"] = (waitMs + 999) / 1000
			}
		}
		if rawMax := strings.TrimSpace(fields["max_runtime_seconds"]); rawMax != "" {
			if maxRuntime, err := strconv.Atoi(rawMax); err == nil && maxRuntime > 0 {
				payload["max_runtime_seconds"] = maxRuntime
			}
		} else if rawMax := strings.TrimSpace(fields["max_runtime_ms"]); rawMax != "" {
			if maxRuntimeMs, err := strconv.Atoi(rawMax); err == nil && maxRuntimeMs > 0 {
				payload["max_runtime_seconds"] = (maxRuntimeMs + 999) / 1000
			}
		}
		if rawStdout := strings.TrimSpace(fields["stdout_offset"]); rawStdout != "" {
			if offset, err := strconv.Atoi(rawStdout); err == nil && offset >= 0 {
				payload["stdout_offset"] = offset
			}
		}
		if rawStderr := strings.TrimSpace(fields["stderr_offset"]); rawStderr != "" {
			if offset, err := strconv.Atoi(rawStderr); err == nil && offset >= 0 {
				payload["stderr_offset"] = offset
			}
		}
		if rawMax := strings.TrimSpace(fields["max_delta_bytes"]); rawMax != "" {
			if maxDeltaBytes, err := strconv.Atoi(rawMax); err == nil && maxDeltaBytes > 0 {
				if maxDeltaBytes > 65536 {
					maxDeltaBytes = 65536
				}
				payload["max_delta_bytes"] = maxDeltaBytes
			}
		}

		switch action {
		case "start":
			if command == "" {
				return nil, "", errors.New("missing command")
			}
		case "poll", "cancel":
			if jobID == "" {
				return nil, "", errors.New("missing job_id")
			}
		default:
			return nil, "", errors.New("unsupported action")
		}

		data, err := json.Marshal(payload)
		return data, string(data), err

	case "edit":
		filePath := strings.TrimSpace(fields["filePath"])
		replaceAll := parseBool(fields["replaceAll"])

		if command := fields["command"]; strings.TrimSpace(command) != "" {
			return nil, "", errors.New("edit 不再支持 command/apply_edit heredoc；请使用 filePath + oldcontent/newcontent，并分段小步多次调用")
		}
		if filePath != "" && strings.TrimSpace(fields["content"]) != "" {
			return nil, "", errors.New("整文件写入/新建请使用 write_file（必要时可 append=true 分段写入）")
		}

		oldContent := fields["oldcontent"]
		newContent := fields["newcontent"]
		if filePath == "" || oldContent == "" {
			return nil, "", errors.New("edit 需要 filePath + oldcontent/newcontent")
		}

		payload := map[string]any{
			"edits": []map[string]any{
				{
					"filePath":   filePath,
					"oldString":  oldContent,
					"newString":  newContent,
					"replaceAll": replaceAll,
				},
			},
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "write_file":
		filePath := strings.TrimSpace(fields["filePath"])
		if filePath == "" {
			return nil, "", errors.New("write_file 缺少 filePath")
		}
		appendMode := parseBool(fields["append"])
		payload := map[string]any{
			"filePath": filePath,
			"content":  fields["content"],
		}
		if appendMode {
			payload["append"] = true
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "glob":
		pattern := strings.TrimSpace(fields["pattern"])
		if pattern == "" {
			return nil, "", errors.New("missing pattern")
		}
		payload := map[string]any{
			"pattern": pattern,
		}
		data, err := json.Marshal(payload)
		return data, string(data), err
	case "ls":
		pathValue := strings.TrimSpace(fields["path"])
		payload := map[string]any{}
		if pathValue != "" {
			payload["path"] = pathValue
		}
		data, err := json.Marshal(payload)
		return data, string(data), err
	case "multiedit":
		editsRaw := strings.TrimSpace(fields["edits"])
		if editsRaw == "" {
			return nil, "", errors.New("missing edits")
		}
		var edits []any
		if err := json.Unmarshal([]byte(editsRaw), &edits); err != nil {
			return nil, "", errors.New("edits must be valid JSON array")
		}
		if len(edits) == 0 {
			return nil, "", errors.New("edits must be a non-empty JSON array")
		}
		payload := map[string]any{
			"edits": edits,
		}
		if replaceAll := parseBool(fields["replaceAll"]); replaceAll {
			payload["replaceAll"] = true
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "search":
		query := strings.TrimSpace(fields["query"])
		if query == "" {
			return nil, "", errors.New("missing query")
		}
		payload := map[string]any{
			"query": query,
		}
		if rawCount := strings.TrimSpace(fields["count"]); rawCount != "" {
			if count, err := strconv.Atoi(rawCount); err == nil && count > 0 {
				payload["count"] = count
			}
		}
		if freshness := strings.TrimSpace(fields["freshness"]); freshness != "" {
			payload["freshness"] = freshness
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "rg":
		pattern := strings.TrimSpace(fields["pattern"])
		if pattern == "" {
			return nil, "", errors.New("missing pattern")
		}
		pathValue := strings.TrimSpace(fields["path"])
		payload := map[string]any{
			"pattern": pattern,
		}
		if pathValue != "" {
			payload["path"] = pathValue
		}
		if rawMax := strings.TrimSpace(fields["max_results"]); rawMax != "" {
			if maxResults, err := strconv.Atoi(rawMax); err == nil && maxResults > 0 {
				payload["max_results"] = maxResults
			}
		}
		if fixed := parseBool(fields["fixed_strings"]); fixed {
			payload["fixed_strings"] = true
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "plan":
		action := strings.ToLower(strings.TrimSpace(fields["action"]))
		if action == "" {
			return nil, "", errors.New("missing action")
		}
		payload := map[string]any{
			"action": action,
		}
		if taskID := strings.TrimSpace(fields["task_id"]); taskID != "" {
			payload["task_id"] = taskID
		} else if taskID := strings.TrimSpace(fields["taskId"]); taskID != "" {
			payload["task_id"] = taskID
		}
		if template := fields["template"]; strings.TrimSpace(template) != "" {
			payload["template"] = template
		}
		if overwrite := parseBool(fields["overwrite"]); overwrite {
			payload["overwrite"] = true
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "skill_read":
		payload := map[string]any{}
		if name := strings.TrimSpace(fields["name"]); name != "" {
			payload["name"] = name
		}
		if id := strings.TrimSpace(fields["skill_id"]); id != "" {
			payload["skill_id"] = id
		} else if id := strings.TrimSpace(fields["id"]); id != "" {
			payload["skill_id"] = id
		}
		data, err := json.Marshal(payload)
		return data, string(data), err

	case "subagent":
		task := strings.TrimSpace(fields["task"])
		if task == "" {
			return nil, "", errors.New("missing task")
		}

		payload := map[string]any{
			"task": task,
		}

		if summary := strings.TrimSpace(fields["context_summary"]); summary != "" {
			payload["context_summary"] = summary
		}
		if rawToolIDs := strings.TrimSpace(fields["tool_ids"]); rawToolIDs != "" {
			ids, err := parseStringList(rawToolIDs)
			if err != nil {
				return nil, "", fmt.Errorf("tool_ids: %w", err)
			}
			if len(ids) > 0 {
				payload["tool_ids"] = ids
			}
		}
		if rawScope := strings.TrimSpace(fields["scope"]); rawScope != "" {
			scopePatterns, err := parseStringList(rawScope)
			if err != nil {
				return nil, "", fmt.Errorf("scope: %w", err)
			}
			if len(scopePatterns) > 0 {
				payload["scope"] = scopePatterns
			}
		}
		if rawSkillIDs := strings.TrimSpace(fields["skill_ids"]); rawSkillIDs != "" {
			skillIDs, err := parseStringList(rawSkillIDs)
			if err != nil {
				return nil, "", fmt.Errorf("skill_ids: %w", err)
			}
			if len(skillIDs) > 0 {
				payload["skill_ids"] = skillIDs
			}
		}
		if rawMaxSteps := strings.TrimSpace(fields["max_steps"]); rawMaxSteps != "" {
			if maxSteps, err := strconv.Atoi(rawMaxSteps); err == nil && maxSteps > 0 {
				payload["max_steps"] = maxSteps
			}
		}
		if rawMaxRuntime := strings.TrimSpace(fields["max_runtime_seconds"]); rawMaxRuntime != "" {
			if maxRuntime, err := strconv.Atoi(rawMaxRuntime); err == nil && maxRuntime > 0 {
				payload["max_runtime_seconds"] = maxRuntime
			}
		}
		if rawMaxLogBytes := strings.TrimSpace(fields["max_log_bytes"]); rawMaxLogBytes != "" {
			if maxLogBytes, err := strconv.Atoi(rawMaxLogBytes); err == nil && maxLogBytes > 0 {
				payload["max_log_bytes"] = maxLogBytes
			}
		}
		if rawKSkills := strings.TrimSpace(fields["k_skills"]); rawKSkills != "" {
			if kSkills, err := strconv.Atoi(rawKSkills); err == nil && kSkills >= 0 {
				payload["k_skills"] = kSkills
			}
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

func parseStringList(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var out []string
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, errors.New("must be a JSON array of strings or a comma-separated list")
		}
		return out, nil
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out, nil
}
