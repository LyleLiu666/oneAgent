package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/subagent"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/handler"
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

	tools, handlers, err := buildSubagentToolset()
	if err != nil {
		return err
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

		toolCtx := ctx
		toolCtx = tool.ContextWithUserID(toolCtx, userID)
		toolCtx = tool.ContextWithSettingsDB(toolCtx, rt.Settings)
		toolCtx = tool.ContextWithSkillManager(toolCtx, rt.Skills)
		toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{
			Enabled: true,
			Root:    task.Workspace,
		})

		maxSteps := task.Limits.MaxSteps
		if maxSteps <= 0 {
			maxSteps = 2000
		}

		maxRuntime := task.Limits.MaxRuntimeSeconds
		if maxRuntime <= 0 {
			maxRuntime = int((6 * time.Hour).Seconds())
		}

		contextSummary := buildResumeContextSummary(resumedFrom)
		req := subagent.RunRequest{
			ParentSessionID: task.ID,
			UserID:          userID,
			SystemPrompt:    handler.DefaultSystemPrompt,
			Client:          client,
			Tools:           tools,
			Handlers:        handlers,
			WorkspaceRoot:   task.Workspace,
			WriteScope:      nil,
			LogsBaseDir:     rt.Layout.SubagentLogsDir,
			Task:            task.Prompt,
			ContextSummary:  contextSummary,
			MaxSteps:        maxSteps,
			MaxRuntimeSeconds: maxRuntime,
		}

		res, runErr := subagent.Run(toolCtx, req)
		_ = modelName // reserved for future observer/executor tuning

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
				},
				Signals: workledger.ReceiptSignals{DurationMs: res.DurationMs},
			})
		}

		return taskqueue.AttemptResult{
			RunID:        res.RunID,
			Summary:      res.Summary,
			FindingsPath: res.FindingsPath,
			TraceLogPath: res.TraceLogPath,
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

func buildSubagentToolset() ([]llm.Tool, map[string]subagent.ToolHandler, error) {
	ids := make([]string, 0, 16)
	for _, def := range tool.All() {
		if def.ID == tool.ToolIDSubagent {
			continue
		}
		ids = append(ids, def.ID)
	}
	defs, err := tool.Mount(ids)
	if err != nil {
		return nil, nil, err
	}

	handlers := make(map[string]subagent.ToolHandler, len(defs))
	for _, def := range defs {
		handlers[def.Spec.Function.Name] = subagent.ToolHandler(def.Handler)
	}

	return tool.ToolsForLLM(defs), handlers, nil
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
