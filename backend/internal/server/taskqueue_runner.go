package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/prompt"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/subagent"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func ensureTaskQueue(rt *runtime.Runtime) error {
	if rt == nil || rt.Layout == nil {
		return fmt.Errorf("runtime is required")
	}
	if rt.Tasks == nil {
		store, err := taskqueue.NewStore(rt.Layout.TasksDir)
		if err != nil {
			return err
		}
		rt.Tasks = store
	}
	if rt.TaskRunner != nil {
		return rt.TaskRunner.Start()
	}

	exec := func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
		userID := strings.TrimSpace(task.UserID)
		if userID == "" {
			userID = "local"
		}

		client, modelName, err := resolveLLMClient(ctx, rt.Settings, userID, task.ModelID)
		if err != nil {
			return taskqueue.AttemptResult{}, err
		}

		policySnap, err := rt.ResolveToolPolicySnapshot(ctx, userID)
		if err != nil {
			return taskqueue.AttemptResult{}, err
		}
		if rt.Tasks != nil && strings.TrimSpace(attempt.ID) != "" {
			snapCopy := policySnap
			_, _ = rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
				a := tk.LatestAttempt()
				if a == nil || a.ID != attempt.ID {
					return nil
				}
				if strings.TrimSpace(a.PrincipalID) == "" {
					a.PrincipalID = userID
				}
				if a.PolicySnapshot == nil {
					a.PolicySnapshot = &snapCopy
				}
				return nil
			})
			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    task.ID,
				AttemptID: attempt.ID,
				Type:      "attempt.policy_snapshot",
				Message:   "Policy snapshot resolved",
				Data: map[string]any{
					"principal_id": userID,
					"policy_id":    policySnap.Policy.ID,
					"policy_hash":  policySnap.PolicyHash,
				},
			})
		}

		toolIDs := make([]string, 0, 16)
		for _, def := range tool.All() {
			if def.ID == tool.ToolIDSubagent {
				continue
			}
			toolIDs = append(toolIDs, def.ID)
		}
		defs, err := tool.MountWithSnapshot(toolIDs, policySnap)
		if err != nil {
			return taskqueue.AttemptResult{}, err
		}

		toolNames := make([]string, 0, len(defs))
		for _, def := range defs {
			toolNames = append(toolNames, def.Spec.Function.Name)
		}
		assembled, err := prompt.AssembleStablePrefix(prompt.AssembleInput{ToolNames: toolNames})
		if err != nil {
			return taskqueue.AttemptResult{}, err
		}
		systemPrompt := assembled.StablePrefix

		tools := tool.ToolsForLLM(defs)
		handlers := make(map[string]subagent.ToolHandler, len(defs))
		for _, def := range defs {
			handlers[def.Spec.Function.Name] = subagent.ToolHandler(def.Handler)
		}

		toolCtx := ctx
		toolCtx = tool.ContextWithUserID(toolCtx, userID)
		toolCtx = tool.ContextWithPolicySnapshot(toolCtx, policySnap)
		toolCtx = tool.ContextWithSettingsDB(toolCtx, rt.Settings)
		toolCtx = tool.ContextWithSkillManager(toolCtx, rt.Skills)
		toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{
			Enabled: true,
			Root:    task.Workspace,
		})
		toolCtx = tool.ContextWithOCC(toolCtx, strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_OCC")) != "1")

		limits := taskqueue.ResolveLimits(task.Limits)

		maxSteps := limits.MaxSteps
		if maxSteps <= 0 {
			maxSteps = 2000
		}

		maxRuntime := limits.MaxRuntimeSeconds
		if maxRuntime <= 0 {
			maxRuntime = int((6 * time.Hour).Seconds())
		}

		contextSummary := buildResumeContextSummary(resumedFrom)
		req := subagent.RunRequest{
			ParentSessionID:   task.ID,
			UserID:            userID,
			SystemPrompt:      systemPrompt,
			Client:            client,
			Tools:             tools,
			Handlers:          handlers,
			WorkspaceRoot:     task.Workspace,
			WriteScope:        nil,
			LogsBaseDir:       rt.Layout.SubagentLogsDir,
			Task:              task.Prompt,
			ContextSummary:    contextSummary,
			MaxSteps:          maxSteps,
			MaxRuntimeSeconds: maxRuntime,
			MaxTotalTokens:    limits.MaxTotalTokens,
			MaxCostUSD:        limits.MaxCostUSD,
		}

		res, runErr := subagent.Run(toolCtx, req)
		_ = modelName // reserved for future observer/executor tuning

		// Best-effort test report generation (evidence), written next to findings/trace when possible.
		testReportPath := ""
		if strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_TEST_REPORT")) != "1" {
			outDir := ""
			if strings.TrimSpace(res.FindingsPath) != "" {
				outDir = filepath.Dir(res.FindingsPath)
			} else if strings.TrimSpace(res.TraceLogPath) != "" {
				outDir = filepath.Dir(res.TraceLogPath)
			}

			if outDir != "" {
				report, _ := generateTestReport(toolCtx, testReportInput{
					WorkspaceRoot: task.Workspace,
					OutputDir:     outDir,
					Enable:        true,
					RunBash: func(ctx context.Context, command string, timeout time.Duration) (tool.BashToolResult, error) {
						raw := []byte(fmt.Sprintf(`{"command":%q,"timeout_ms":%d}`, command, int(timeout.Milliseconds())))
						out, err := handlers["bash"](ctx, raw)
						if err != nil {
							return tool.BashToolResult{}, err
						}
						if br, ok := out.(tool.BashToolResult); ok {
							return br, nil
						}
						// Tolerate map[string]any output shapes.
						b, _ := json.Marshal(out)
						var br tool.BashToolResult
						_ = json.Unmarshal(b, &br)
						return br, nil
					},
				})
				testReportPath = strings.TrimSpace(report)
			}
		}

		if rt.WorkLedger != nil {
			summary := strings.TrimSpace(res.Summary)
			if summary == "" {
				if runErr != nil {
					summary = "subagent failed: " + runErr.Error()
				} else {
					summary = "subagent finished"
				}
			}
			status := workledger.ReceiptStatusSucceeded
			if runErr != nil {
				status = workledger.ReceiptStatusFailed
			}
			finished := time.Now()
			started := finished.Add(-time.Duration(maxInt64(0, res.DurationMs)) * time.Millisecond)
			signals := workledger.ReceiptSignals{DurationMs: res.DurationMs}
			if res.Usage != nil {
				signals.Calls = res.Usage.Calls
				signals.PromptTokens = res.Usage.PromptTokens
				signals.CompletionTokens = res.Usage.CompletionTokens
				signals.TotalTokens = res.Usage.TotalTokens
				signals.CostUSD = res.Usage.CostUSD
			}
			_, _ = rt.WorkLedger.CreateReceipt(workledger.CreateReceiptInput{
				PrincipalID:   userID,
				WorkspaceRoot: task.Workspace,
				Kind:          workledger.ReceiptKindSubagentRun,
				Status:        status,
				StartedAt:     started,
				FinishedAt:    finished,
				Summary:       summary,
				Artifacts: workledger.ReceiptArtifacts{
					FindingsPath: strings.TrimSpace(res.FindingsPath),
					TraceLogPath: strings.TrimSpace(res.TraceLogPath),
					TestReportPath: strings.TrimSpace(testReportPath),
				},
				Signals: signals,
			})
		}

		return taskqueue.AttemptResult{
			RunID:        res.RunID,
			Summary:      res.Summary,
			FindingsPath: res.FindingsPath,
			TraceLogPath: res.TraceLogPath,
			TestReportPath: testReportPath,
			Usage:        res.Usage,
		}, runErr
	}

	decideOutcome := func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
		userID := strings.TrimSpace(task.UserID)
		if userID == "" {
			userID = "local"
		}
		client, modelName, err := resolveLLMClient(ctx, rt.Settings, userID, task.ModelID)
		if err != nil {
			return taskqueue.ObserverDecision{}, err
		}

		observer := &taskqueue.OutcomeObserver{
			Client: client,
			Model:  modelName,
		}

		return observer.Decide(ctx, taskqueue.ObserveInput{
			TaskID:        task.ID,
			AttemptID:     attempt.ID,
			WorkspaceRoot: task.Workspace,
			Prompt:        task.Prompt,
			Summary:       attempt.Summary,
			FindingsPath:  attempt.FindingsPath,
			TraceLogPath:  attempt.TraceLogPath,
			TestReportPath: attempt.TestReportPath,
		})
	}

	rt.TaskRunner = &taskqueue.TaskRunner{
		Store:          rt.Tasks,
		ExecuteAttempt: exec,
		DecideOutcome:  decideOutcome,
	}
	return rt.TaskRunner.Start()
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func buildResumeContextSummary(resumedFrom *taskqueue.Attempt) string {
	if resumedFrom == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Previous attempt context\n")
	if strings.TrimSpace(resumedFrom.Summary) != "" {
		b.WriteString("\n### Summary\n")
		b.WriteString(strings.TrimSpace(resumedFrom.Summary))
		b.WriteString("\n")
	}
	if strings.TrimSpace(resumedFrom.FindingsPath) != "" {
		b.WriteString("\n### Findings\n")
		b.WriteString(resumedFrom.FindingsPath)
		b.WriteString("\n")
	}
	if strings.TrimSpace(resumedFrom.TraceLogPath) != "" {
		b.WriteString("\n### Trace\n")
		b.WriteString(resumedFrom.TraceLogPath)
		b.WriteString("\n")
	}
	if strings.TrimSpace(resumedFrom.Error) != "" {
		b.WriteString("\n### Error\n")
		b.WriteString(strings.TrimSpace(resumedFrom.Error))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func resolveLLMClient(ctx context.Context, settings *settingsdb.DB, userID, modelID string) (llm.Client, string, error) {
	if settings == nil {
		return nil, "", fmt.Errorf("settings db is required")
	}

	modelID = strings.TrimSpace(modelID)
	var m settingsdb.Model
	if modelID != "" {
		got, err := settings.GetModel(ctx, userID, modelID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, "", fmt.Errorf("model not found")
			}
			return nil, "", err
		}
		m = got
	} else {
		models, err := settings.ListModels(ctx, userID, "")
		if err != nil {
			return nil, "", err
		}
		for _, candidate := range models {
			if candidate.IsDefault {
				m = candidate
				break
			}
		}
		if m.ID == "" {
			return nil, "", fmt.Errorf("no LLM model configured")
		}
	}

	provider, err := settings.GetProvider(ctx, userID, m.ProviderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("provider not found")
		}
		return nil, "", err
	}

	if strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.APIKey) == "" {
		return nil, "", fmt.Errorf("provider base_url or api_key is missing")
	}

	client, err := llm.NewClientForProvider(llm.ProviderConfig{
		ProviderType: provider.ProviderType,
		Endpoint:     provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        m.Model,
	})
	if err != nil {
		return nil, "", err
	}

	return client, m.Model, nil
}
