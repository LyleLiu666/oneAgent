package subagent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type ToolHandler func(ctx context.Context, raw json.RawMessage) (any, error)

type RunRequest struct {
	ParentSessionID string
	UserID          string

	SystemPrompt string

	Client llm.Client
	Tools  []llm.Tool

	Handlers map[string]ToolHandler

	WorkspaceRoot string
	WriteScope    []string

	LogsBaseDir string

	Task           string
	ContextSummary string
	SkillsSummary  string

	MaxSteps          int
	MaxRuntimeSeconds int
	MaxLogBytes       int
}

type RunResult struct {
	RunID        string `json:"run_id"`
	Summary      string `json:"summary"`
	FindingsPath string `json:"findings_path"`
	TraceLogPath string `json:"trace_log_path"`
	DurationMs   int64  `json:"duration_ms"`

	PlanMarkDone []PlanMarkDoneRecord `json:"plan_mark_done,omitempty"`
}

type PlanMarkDoneRecord struct {
	TaskID string `json:"task_id"`
	Pass   bool   `json:"pass"`
	Reason string `json:"reason,omitempty"`
}

type toolCaller interface {
	ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error)
}

type toolStreamingCaller interface {
	ChatCompletionStreamWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) (llm.ChatCompletionResult, error)
}

func Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if strings.TrimSpace(req.Task) == "" {
		return RunResult{}, errors.New("task is required")
	}
	if req.Client == nil {
		return RunResult{}, errors.New("llm client is required")
	}
	if len(req.Tools) == 0 || len(req.Handlers) == 0 {
		return RunResult{}, errors.New("tools are required")
	}
	if strings.TrimSpace(req.LogsBaseDir) == "" {
		return RunResult{}, errors.New("logs base dir is required")
	}

	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 200
	}
	if maxSteps > 2000 {
		maxSteps = 2000
	}
	maxRuntime := req.MaxRuntimeSeconds
	if maxRuntime <= 0 {
		maxRuntime = 3600
	}

	runID := uuid.NewString()
	now := time.Now()
	runDir := filepath.Join(req.LogsBaseDir, now.Format("2006-01-02"), req.ParentSessionID, runID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return RunResult{}, fmt.Errorf("create run dir: %w", err)
	}

	tracePath := filepath.Join(runDir, "trace.jsonl")
	traceFile, err := os.Create(tracePath)
	if err != nil {
		return RunResult{}, fmt.Errorf("create trace log: %w", err)
	}
	defer traceFile.Close()
	writer := bufio.NewWriter(traceFile)
	defer writer.Flush()

	started := time.Now()

	maxLogBytes := req.MaxLogBytes
	if maxLogBytes <= 0 {
		maxLogBytes = 64 * 1024 * 1024
	}
	if maxLogBytes > 512*1024*1024 {
		maxLogBytes = 512 * 1024 * 1024
	}

	limiter := &logLimiter{MaxBytes: maxLogBytes}

	logEvent(writer, limiter, true, map[string]any{
		"type":       "start",
		"run_id":     runID,
		"session_id": req.ParentSessionID,
		"user_id":    req.UserID,
		"task":       req.Task,
		"scope":      req.WriteScope,
		"tools":      toolNames(req.Tools),
	})

	messages := make([]llm.ChatMessage, 0, 4)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, llm.BuildSystemMessage(req.SystemPrompt))
	}

	var turnCtx strings.Builder
	turnCtx.WriteString("## 子 Agent 任务\n")
	turnCtx.WriteString(req.Task)
	turnCtx.WriteString("\n")
	if strings.TrimSpace(req.ContextSummary) != "" {
		turnCtx.WriteString("\n## 上下文引用\n")
		turnCtx.WriteString(req.ContextSummary)
		turnCtx.WriteString("\n")
	}
	if strings.TrimSpace(req.SkillsSummary) != "" {
		turnCtx.WriteString("\n## 技能建议（按步骤注入）\n")
		turnCtx.WriteString(req.SkillsSummary)
		turnCtx.WriteString("\n")
	}
	turnCtx.WriteString("\n## 交付要求\n")
	turnCtx.WriteString("你必须在最后输出一个 XML（不要额外包裹 JSON）：\n")
	turnCtx.WriteString("<subagent_handoff>\n")
	turnCtx.WriteString("  <summary>一句/几句短总结 + 下一步建议</summary>\n")
	turnCtx.WriteString("  <timeline>## 流水账\\n- ...</timeline>\n")
	turnCtx.WriteString("  <findings>## Findings\\n- ...</findings>\n")
	turnCtx.WriteString("  <changed_files>## 变更文件\\n- ...</changed_files>\n")
	turnCtx.WriteString("</subagent_handoff>\n")

	if msg, ok := llm.BuildTurnContextMessage(turnCtx.String()); ok {
		messages = append(messages, msg)
	}

	opts := &llm.ChatCompletionOptions{
		Tools: req.Tools,
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(maxRuntime)*time.Second)
	defer cancel()

	var (
		combined strings.Builder
		planDone []PlanMarkDoneRecord
		changed  = make(map[string]struct{})
		loopErr  error
	)

	normalizeChangedPath := func(p string) string {
		p = strings.TrimSpace(p)
		if p == "" {
			return ""
		}
		p = filepath.Clean(p)

		root := strings.TrimSpace(req.WorkspaceRoot)
		if root != "" && filepath.IsAbs(p) {
			if rel, err := filepath.Rel(root, p); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(rel)
			}
		}
		return filepath.ToSlash(p)
	}
	recordChanged := func(p string) {
		if normalized := normalizeChangedPath(p); normalized != "" {
			changed[normalized] = struct{}{}
		}
	}

	completed := false
	for step := 0; step < maxSteps; step++ {
		logEvent(writer, limiter, false, map[string]any{
			"type":  "llm_call",
			"step":  step,
			"input": messages,
		})

		result, err := callWithTools(ctx, req.Client, messages, opts, &combined)
		if err != nil {
			logEvent(writer, limiter, true, map[string]any{
				"type":  "error",
				"step":  step,
				"error": err.Error(),
			})
			loopErr = err
			break
		}

		logEvent(writer, limiter, false, map[string]any{
			"type":   "llm_result",
			"step":   step,
			"output": result,
		})

		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		if len(result.ToolCalls) == 0 {
			completed = true
			break
		}

		for _, call := range result.ToolCalls {
			toolName := strings.TrimSpace(call.Function.Name)
			rawArgs := json.RawMessage(call.Function.Arguments)

			logEvent(writer, limiter, false, map[string]any{
				"type":        "tool_call",
				"step":        step,
				"tool_name":   toolName,
				"tool_call_id": call.ID,
				"arguments":   call.Function.Arguments,
			})

			handler, ok := req.Handlers[toolName]
			var payload any
			var toolErr error
			if !ok {
				toolErr = fmt.Errorf("unknown tool: %s", toolName)
				payload = map[string]any{"error": toolErr.Error()}
			} else {
				payload, toolErr = handler(ctx, rawArgs)
				if toolErr != nil {
					payload = map[string]any{"error": fmt.Sprintf("Tool execution failed: %v", toolErr)}
				}
			}

			// Capture plan.mark_done results for handoff.
			if toolName == "plan" {
				var parsed struct {
					Action string `json:"action"`
					TaskID string `json:"task_id"`
				}
				_ = json.Unmarshal(rawArgs, &parsed)
				if strings.EqualFold(strings.TrimSpace(parsed.Action), "mark_done") {
					pass := false
					reason := ""
					if b, err := json.Marshal(payload); err == nil {
						var out struct {
							Pass   bool   `json:"pass"`
							Reason string `json:"reason"`
						}
						_ = json.Unmarshal(b, &out)
						pass = out.Pass
						reason = out.Reason
					}
					planDone = append(planDone, PlanMarkDoneRecord{
						TaskID: strings.TrimSpace(parsed.TaskID),
						Pass:   pass,
						Reason: strings.TrimSpace(reason),
					})
				}
			}

			// Best-effort changed file tracking for common file tools.
			switch toolName {
			case "write_file":
				var args struct {
					FilePath string `json:"filePath"`
				}
				_ = json.Unmarshal(rawArgs, &args)
				recordChanged(args.FilePath)
			case "edit", "multiedit":
				var batch struct {
					Edits []struct {
						FilePath string `json:"filePath"`
					} `json:"edits"`
				}
				_ = json.Unmarshal(rawArgs, &batch)
				if len(batch.Edits) > 0 {
					for _, e := range batch.Edits {
						recordChanged(e.FilePath)
					}
					break
				}
				var single struct {
					FilePath string `json:"filePath"`
				}
				_ = json.Unmarshal(rawArgs, &single)
				recordChanged(single.FilePath)
			}

			response, err := json.Marshal(payload)
			if err != nil {
				response = []byte(fmt.Sprintf(`{"error":"failed to marshal tool response: %v"}`, err))
			}

			logEvent(writer, limiter, false, map[string]any{
				"type":        "tool_result",
				"step":        step,
				"tool_name":   toolName,
				"tool_call_id": call.ID,
				"ok":          toolErr == nil,
				"output":      string(response),
				"error":       errorString(toolErr),
			})

			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				Content:    string(response),
				ToolCallID: call.ID,
				Name:       toolName,
			})
		}
	}
	if !completed && loopErr == nil {
		loopErr = errors.New("max steps reached")
	}

	finalText := combined.String()
	summary, timeline, findings, changedFiles := parseHandoffXML(finalText)
	if strings.TrimSpace(summary) == "" {
		summary = "subagent finished"
	}
	if loopErr != nil {
		if strings.TrimSpace(findings) == "" {
			findings = "## Findings\n- ERROR: " + loopErr.Error()
		} else {
			findings = strings.TrimSpace(findings) + "\n- ERROR: " + loopErr.Error()
		}
		if strings.TrimSpace(summary) == "" || summary == "subagent finished" {
			summary = "subagent failed: " + loopErr.Error()
		} else {
			summary = strings.TrimSpace(summary) + " (error: " + loopErr.Error() + ")"
		}
	}

	changedList := make([]string, 0, len(changed))
	for p := range changed {
		changedList = append(changedList, p)
	}
	sort.Strings(changedList)

	findingsPath := filepath.Join(runDir, "FINDINGS.md")
	if err := os.WriteFile(findingsPath, []byte(buildFindingsMarkdown(timeline, findings, changedFiles, changedList, planDone)), 0o644); err != nil {
		return RunResult{}, fmt.Errorf("write findings: %w", err)
	}

	logEvent(writer, limiter, true, map[string]any{
		"type":          "complete",
		"run_id":        runID,
		"summary":       summary,
		"findings_path": findingsPath,
		"trace_log_path": tracePath,
		"duration_ms":   time.Since(started).Milliseconds(),
		"error":         errorString(loopErr),
		"log_truncated": limiter.Truncated,
		"log_max_bytes": limiter.MaxBytes,
	})

	return RunResult{
		RunID:        runID,
		Summary:      truncateRunes(summary, 800),
		FindingsPath: findingsPath,
		TraceLogPath: tracePath,
		DurationMs:   time.Since(started).Milliseconds(),
		PlanMarkDone: planDone,
	}, loopErr
}

func callWithTools(ctx context.Context, client llm.Client, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, combined *strings.Builder) (llm.ChatCompletionResult, error) {
	if streamClient, ok := client.(toolStreamingCaller); ok {
		var stepContent strings.Builder
		result, err := streamClient.ChatCompletionStreamWithTools(ctx, messages, opts, func(chunk string) error {
			stepContent.WriteString(chunk)
			if combined != nil {
				combined.WriteString(chunk)
			}
			return nil
		})
		if err == nil && result.Content == "" && stepContent.Len() > 0 {
			result.Content = stepContent.String()
		}
		return result, err
	}
	c, ok := client.(toolCaller)
	if !ok {
		return llm.ChatCompletionResult{}, errors.New("tool calling not supported for this provider")
	}
	result, err := c.ChatCompletionWithTools(ctx, messages, opts)
	if err == nil && combined != nil && result.Content != "" {
		combined.WriteString(result.Content)
	}
	return result, err
}

type logLimiter struct {
	MaxBytes     int
	WrittenBytes int
	Truncated    bool
}

func logEvent(w *bufio.Writer, limiter *logLimiter, critical bool, payload map[string]any) {
	if w == nil {
		return
	}
	if limiter != nil && limiter.Truncated && !critical {
		return
	}
	payload["ts"] = time.Now().Format(time.RFC3339Nano)
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	n := len(b) + 1
	if limiter != nil && limiter.MaxBytes > 0 && !critical && limiter.WrittenBytes+n > limiter.MaxBytes {
		limiter.Truncated = true
		return
	}
	_, _ = w.Write(b)
	_, _ = w.WriteString("\n")
	_ = w.Flush()
	if limiter != nil {
		limiter.WrittenBytes += n
	}
}

func toolNames(tools []llm.Tool) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, strings.TrimSpace(t.Function.Name))
	}
	return out
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncateRunes(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
