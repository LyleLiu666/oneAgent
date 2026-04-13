package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type toolcallingRegressionReport struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`

	Summary toolcallingRegressionSummary `json:"summary"`
	Tasks   []toolcallingRegressionTask  `json:"tasks"`
}

type toolcallingRegressionSummary struct {
	TotalTasks int `json:"total_tasks"`

	ToolSelectionAccuracy float64 `json:"tool_selection_accuracy"`

	TotalToolCalls       int                `json:"total_tool_calls"`
	ToolArgumentsValid   int                `json:"tool_arguments_valid"`
	ToolArgumentsInvalid int                `json:"tool_arguments_invalid"`
	ToolCallsOK          int                `json:"tool_calls_ok"`
	ToolCallsFailed      int                `json:"tool_calls_failed"`
	ToolCallsFailedBy    map[string]int     `json:"tool_calls_failed_by_class"`
	StepsPerTaskAvg      float64            `json:"steps_per_task_avg"`
	StepsPerTaskByTask   map[string]float64 `json:"steps_per_task_by_task"`
}

type toolcallingRegressionTask struct {
	Name string `json:"name"`

	ExpectedTools []string `json:"expected_tools"`

	ObservedTools []string `json:"observed_tools"`
	Steps         int      `json:"steps"`

	ToolCalls       int            `json:"tool_calls"`
	ToolCallsOK     int            `json:"tool_calls_ok"`
	ToolCallsFailed int            `json:"tool_calls_failed"`
	FailuresByClass map[string]int `json:"failures_by_class"`
}

func TestToolcallingMetrics_ScriptedRegressionReport(t *testing.T) {
	tasks := []struct {
		name          string
		expectedTools []string
		defs          []tool.Definition
		results       []llm.ChatCompletionResult
	}{
		{
			name:          "ok",
			expectedTools: []string{"dummy"},
			defs: []tool.Definition{
				{
					ID: "dummy",
					Spec: llm.Tool{
						Type: "function",
						Function: llm.ToolFunction{
							Name: "dummy",
							Parameters: map[string]any{
								"type":                 "object",
								"properties":           map[string]any{"x": map[string]any{"type": "string"}},
								"required":             []string{"x"},
								"additionalProperties": false,
							},
						},
					},
					Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
						return map[string]any{"ok": true}, nil
					},
				},
			},
			results: []llm.ChatCompletionResult{
				{
					Content: "calling tool",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.ToolCallFunction{
								Name:      "dummy",
								Arguments: `{"x":"ok"}`,
							},
						},
					},
				},
				{Content: "done"},
			},
		},
		{
			name:          "invalid_args_missing_required",
			expectedTools: []string{"dummy"},
			defs: []tool.Definition{
				{
					ID: "dummy",
					Spec: llm.Tool{
						Type: "function",
						Function: llm.ToolFunction{
							Name: "dummy",
							Parameters: map[string]any{
								"type":                 "object",
								"properties":           map[string]any{"x": map[string]any{"type": "string"}},
								"required":             []string{"x"},
								"additionalProperties": false,
							},
						},
					},
					Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
						t.Fatalf("handler must not run for invalid args")
						return nil, nil
					},
				},
			},
			results: []llm.ChatCompletionResult{
				{
					Content: "calling tool",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.ToolCallFunction{
								Name:      "dummy",
								Arguments: `{}`,
							},
						},
					},
				},
				{Content: "done"},
			},
		},
		{
			name:          "approval_required",
			expectedTools: []string{"dummy"},
			defs: []tool.Definition{
				{
					ID: "dummy",
					Spec: llm.Tool{
						Type: "function",
						Function: llm.ToolFunction{
							Name: "dummy",
							Parameters: map[string]any{
								"type":                 "object",
								"properties":           map[string]any{"x": map[string]any{"type": "string"}},
								"required":             []string{"x"},
								"additionalProperties": false,
							},
						},
					},
					Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
						return nil, &tool.ApprovalRequiredError{ApprovalID: "a_1", ToolID: "dummy", ScopeID: "s_1"}
					},
				},
			},
			results: []llm.ChatCompletionResult{
				{
					Content: "calling tool",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.ToolCallFunction{
								Name:      "dummy",
								Arguments: `{"x":"ok"}`,
							},
						},
					},
				},
				{Content: "done"},
			},
		},
	}

	report := toolcallingRegressionReport{
		Version:     1,
		GeneratedAt: time.Now().UTC(),
		Summary: toolcallingRegressionSummary{
			ToolCallsFailedBy:  map[string]int{},
			StepsPerTaskByTask: map[string]float64{},
		},
		Tasks: make([]toolcallingRegressionTask, 0, len(tasks)),
	}

	totalExpected := 0
	totalCorrect := 0
	totalSteps := 0

	for _, tc := range tasks {
		taskResult := runToolcallingRegressionTask(t, tc.name, tc.expectedTools, tc.defs, tc.results)
		report.Tasks = append(report.Tasks, taskResult)
		report.Summary.TotalTasks++

		totalSteps += taskResult.Steps
		report.Summary.StepsPerTaskByTask[tc.name] = float64(taskResult.Steps)

		report.Summary.TotalToolCalls += taskResult.ToolCalls
		report.Summary.ToolCallsOK += taskResult.ToolCallsOK
		report.Summary.ToolCallsFailed += taskResult.ToolCallsFailed

		for cls, n := range taskResult.FailuresByClass {
			report.Summary.ToolCallsFailedBy[cls] += n
		}

		totalExpected += len(taskResult.ExpectedTools)
		for idx := range taskResult.ExpectedTools {
			if idx < len(taskResult.ObservedTools) && taskResult.ObservedTools[idx] == taskResult.ExpectedTools[idx] {
				totalCorrect++
			}
		}
	}

	// Compute validity + accuracy.
	report.Summary.ToolSelectionAccuracy = 1.0
	if totalExpected > 0 {
		report.Summary.ToolSelectionAccuracy = float64(totalCorrect) / float64(totalExpected)
	}
	report.Summary.ToolArgumentsValid = report.Summary.TotalToolCalls
	report.Summary.ToolArgumentsInvalid = 0
	if n, ok := report.Summary.ToolCallsFailedBy["invalid_arguments"]; ok && n > 0 {
		report.Summary.ToolArgumentsInvalid = n
		report.Summary.ToolArgumentsValid = report.Summary.TotalToolCalls - n
	}
	if report.Summary.TotalTasks > 0 {
		report.Summary.StepsPerTaskAvg = float64(totalSteps) / float64(report.Summary.TotalTasks)
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "toolcalling_metrics_report.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("stat report: %v", err)
	}

	// Basic assertions for regressions:
	if report.Summary.TotalToolCalls == 0 {
		t.Fatalf("expected tool calls > 0")
	}
	if report.Summary.ToolCallsOK == 0 {
		t.Fatalf("expected some tool calls ok")
	}
	if report.Summary.ToolCallsFailed == 0 {
		t.Fatalf("expected some tool calls failed")
	}
	if report.Summary.ToolCallsFailedBy["invalid_arguments"] == 0 {
		t.Fatalf("expected invalid_arguments failures to be recorded")
	}
	if report.Summary.ToolCallsFailedBy["policy_denied"] == 0 {
		t.Fatalf("expected policy_denied failures to be recorded")
	}

	t.Logf("toolcalling metrics report: %s", outPath)
}

func runToolcallingRegressionTask(
	t *testing.T,
	name string,
	expectedTools []string,
	defs []tool.Definition,
	results []llm.ChatCompletionResult,
) toolcallingRegressionTask {
	t.Helper()

	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("sessionstore.New: %v", err)
	}

	sessionID := uuid.NewString()
	userID := "u_1"
	if _, err := sessions.GetOrCreateSession(sessionID, userID, "", ""); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	client := &fakeToolCaller{results: results}
	broadcaster := &StreamBroadcaster{clients: map[chan StreamEvent]bool{}}

	_, _, _, err = runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "go"}},
		&llm.ChatCompletionOptions{},
		defs,
		broadcaster,
		sessionID,
		userID,
		nil,
		"test-model",
		false,
		sessions,
	)
	if err != nil {
		t.Fatalf("runToolLoop error: %v", err)
	}

	_, msgs, err := sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		t.Fatalf("GetSessionWithMessages: %v", err)
	}

	var observedTools []string
	steps := 0
	failuresBy := map[string]int{}
	toolCalls := 0
	toolCallsOK := 0
	toolCallsFailed := 0

	for _, msg := range msgs {
		switch msg.Type {
		case model.MessageTypeToolCall:
			steps++
			call, ok := parsePersistedToolCall(msg.Content)
			if ok {
				for _, c := range call.ToolCalls {
					observedTools = append(observedTools, strings.TrimSpace(c.Function.Name))
				}
			}

		case model.MessageTypeToolResult:
			payload, ok := parsePersistedToolResult(msg.Content)
			if !ok {
				continue
			}
			for _, r := range payload.Results {
				toolCalls++
				if r.OK {
					toolCallsOK++
					continue
				}
				toolCallsFailed++
				cls := classifyToolFailure(payload.Content, r)
				failuresBy[cls]++
			}
		}
	}

	return toolcallingRegressionTask{
		Name:            name,
		ExpectedTools:   expectedTools,
		ObservedTools:   observedTools,
		Steps:           steps,
		ToolCalls:       toolCalls,
		ToolCallsOK:     toolCallsOK,
		ToolCallsFailed: toolCallsFailed,
		FailuresByClass: failuresBy,
	}
}

func classifyToolFailure(output string, r persistedToolResult) string {
	output = strings.TrimSpace(output)
	if output != "" && json.Valid([]byte(output)) {
		var obj map[string]any
		if err := json.Unmarshal([]byte(output), &obj); err == nil {
			if code, ok := obj["error"].(string); ok {
				code = strings.TrimSpace(code)
				switch code {
				case "invalid_arguments":
					return "invalid_arguments"
				case "approval_required", "approval_denied":
					return "policy_denied"
				}
				if strings.Contains(strings.ToLower(code), "unknown tool") {
					return "unknown"
				}
			}
		}
	}
	// Fallback heuristics based on structured error.
	msg := strings.ToLower(strings.TrimSpace(r.Error))
	switch {
	case strings.Contains(msg, "missing required fields") || strings.Contains(msg, "arguments must be valid json"):
		return "invalid_arguments"
	case strings.Contains(msg, "approval required") || strings.Contains(msg, "approval denied"):
		return "policy_denied"
	case strings.Contains(msg, "unknown tool"):
		return "unknown"
	default:
		return "tool_error"
	}
}
