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

	"github.com/liu_y/oneAgent/backend/internal/checkpoint"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/projectcfg"
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
		var attemptResult taskqueue.AttemptResult
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

		// Rollback boundary: snapshot workspace at attempt start (best-effort, fail-closed).
		checkpointDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "checkpoint")
		cp, err := checkpoint.CreateWorkspaceCheckpoint(ctx, task.Workspace, checkpointDir)
		if err != nil {
			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    task.ID,
				AttemptID: attempt.ID,
				Type:      "attempt.checkpoint.failed",
				Message:   "checkpoint creation failed",
				Data: map[string]any{
					"error": err.Error(),
				},
			})
			attemptResult.Summary = "checkpoint failed: " + err.Error()
			return attemptResult, err
		}
		if strings.TrimSpace(cp.ArchivePath) != "" && rt.Tasks != nil {
			cpPath := cp.ArchivePath
			_, _ = rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
				a := tk.LatestAttempt()
				if a == nil || a.ID != attempt.ID {
					return nil
				}
				a.CheckpointPath = strings.TrimSpace(cpPath)
				return nil
			})
			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    task.ID,
				AttemptID: attempt.ID,
				Type:      "attempt.checkpoint.created",
				Message:   "checkpoint created",
				Data: map[string]any{
					"checkpoint_path": cpPath,
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
		toolCtx = tool.ContextWithAttemptID(toolCtx, attempt.ID)
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

		// Optional workspace project scripts.
		projectCfg, projectCfgFound, err := projectcfg.Load(task.Workspace)
		if err != nil {
			attemptResult.Summary = "project config error: " + err.Error()
			return attemptResult, err
		}
		if projectCfgFound {
			attemptResult.ProjectConfigPath = filepath.Join(task.Workspace, ".oneagent", "project.json")

			scriptsDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "project_scripts")
			if err := os.MkdirAll(scriptsDir, 0o700); err != nil {
				wrap := fmt.Errorf("create attempt project scripts dir: %w", err)
				attemptResult.Summary = wrap.Error()
				return attemptResult, wrap
			}

			if strings.TrimSpace(projectCfg.CleanupScript) != "" {
				cleanupLogPath := filepath.Join(scriptsDir, "cleanup_script.log")
				attemptResult.CleanupScriptLogPath = cleanupLogPath
				defer func() {
					if err := runProjectScript(toolCtx, handlers, "cleanup_script", projectCfg.CleanupScript, cleanupLogPath); err != nil {
						_ = rt.Tasks.AppendEvent(taskqueue.Event{
							TaskID:    task.ID,
							AttemptID: attempt.ID,
							Type:      "attempt.cleanup_script.failed",
							Message:   "cleanup_script failed",
							Data: map[string]any{
								"log_path": cleanupLogPath,
								"error":    err.Error(),
							},
						})
					}
				}()
			}

			copyLogPath := filepath.Join(scriptsDir, "copy_files.log")
			attemptResult.CopyFilesLogPath = copyLogPath
			if err := runCopyFilesChecks(copyLogPath, task.Workspace, projectCfg.CopyFiles); err != nil {
				attemptResult.Summary = "copy_files failed: " + err.Error()
				return attemptResult, err
			}

			if strings.TrimSpace(projectCfg.SetupScript) != "" {
				setupLogPath := filepath.Join(scriptsDir, "setup_script.log")
				attemptResult.SetupScriptLogPath = setupLogPath
				if err := runProjectScript(toolCtx, handlers, "setup_script", projectCfg.SetupScript, setupLogPath); err != nil {
					attemptResult.Summary = "setup_script failed: " + err.Error()
					_ = rt.Tasks.AppendEvent(taskqueue.Event{
						TaskID:    task.ID,
						AttemptID: attempt.ID,
						Type:      "attempt.setup_script.failed",
						Message:   "setup_script failed",
						Data: map[string]any{
							"log_path": setupLogPath,
							"error":    err.Error(),
						},
					})
					return attemptResult, err
				}
				_ = rt.Tasks.AppendEvent(taskqueue.Event{
					TaskID:    task.ID,
					AttemptID: attempt.ID,
					Type:      "attempt.setup_script.succeeded",
					Message:   "setup_script succeeded",
					Data: map[string]any{
						"log_path": setupLogPath,
					},
				})
			}
		}

		contextSummary := buildResumeContextSummary(resumedFrom)
		if strings.TrimSpace(attempt.ReviewNotes) != "" {
			if contextSummary != "" {
				contextSummary += "\n\n"
			}
			contextSummary += "## Review notes\n"
			contextSummary += strings.TrimSpace(attempt.ReviewNotes)
		}
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
		attemptResult.RunID = res.RunID
		attemptResult.Summary = res.Summary
		attemptResult.FindingsPath = res.FindingsPath
		attemptResult.TraceLogPath = res.TraceLogPath
		attemptResult.Usage = res.Usage

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
		attemptResult.TestReportPath = testReportPath

		// Run project scripts in the finishing phase.
		if projectCfgFound {
			scriptsDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "project_scripts")
			if strings.TrimSpace(projectCfg.TestScript) != "" {
				testLogPath := filepath.Join(scriptsDir, "test_script.log")
				attemptResult.TestScriptLogPath = testLogPath
				if err := runProjectScript(toolCtx, handlers, "test_script", projectCfg.TestScript, testLogPath); err != nil {
					_ = rt.Tasks.AppendEvent(taskqueue.Event{
						TaskID:    task.ID,
						AttemptID: attempt.ID,
						Type:      "attempt.test_script.failed",
						Message:   "test_script failed",
						Data: map[string]any{
							"log_path": testLogPath,
							"error":    err.Error(),
						},
					})
					if runErr == nil {
						attemptResult.Summary = "test_script failed: " + err.Error()
						runErr = err
					}
				} else {
					_ = rt.Tasks.AppendEvent(taskqueue.Event{
						TaskID:    task.ID,
						AttemptID: attempt.ID,
						Type:      "attempt.test_script.succeeded",
						Message:   "test_script succeeded",
						Data: map[string]any{
							"log_path": testLogPath,
						},
					})
				}
			}
		}

		// Best-effort diff artifacts for review: changed files + git patch when possible.
		reviewDir := filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review")
		attemptResult.ReviewCommentsPath = filepath.Join(reviewDir, "review_comments.jsonl")
		// Pre-create the review comments file so the UI can reliably display/open it.
		if err := os.MkdirAll(filepath.Dir(attemptResult.ReviewCommentsPath), 0o700); err == nil {
			if f, err := os.OpenFile(attemptResult.ReviewCommentsPath, os.O_CREATE, 0o600); err == nil {
				_ = f.Close()
			}
		}
		if diff, err := generateDiffArtifacts(toolCtx, task.Workspace, res.FindingsPath, reviewDir); err != nil {
			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    task.ID,
				AttemptID: attempt.ID,
				Type:      "attempt.diff_artifacts.failed",
				Message:   "diff artifacts generation failed",
				Data: map[string]any{
					"error": err.Error(),
				},
			})
		} else {
			attemptResult.DiffPatchPath = diff.DiffPatchPath
			attemptResult.ChangedFilesPath = diff.ChangedFilesPath
			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    task.ID,
				AttemptID: attempt.ID,
				Type:      "attempt.diff_artifacts.created",
				Message:   "diff artifacts generated",
				Data: map[string]any{
					"git_workspace":      diff.IsGitWorkspace,
					"diff_patch_path":    diff.DiffPatchPath,
					"changed_files_path": diff.ChangedFilesPath,
					"note":               diff.Reason,
				},
			})
		}

		if rt.WorkLedger != nil {
			summary := strings.TrimSpace(res.Summary)
			if summary == "" {
				summary = "subagent finished"
			}
			if runErr != nil {
				summary = "attempt failed: " + runErr.Error()
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
					FindingsPath:       strings.TrimSpace(res.FindingsPath),
					TraceLogPath:       strings.TrimSpace(res.TraceLogPath),
					TestReportPath:     strings.TrimSpace(testReportPath),
					DiffPatchPath:      strings.TrimSpace(attemptResult.DiffPatchPath),
					ChangedFilesPath:   strings.TrimSpace(attemptResult.ChangedFilesPath),
					ReviewCommentsPath: strings.TrimSpace(attemptResult.ReviewCommentsPath),
				},
				Signals: signals,
			})
		}

		return attemptResult, runErr
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
			TaskID:         task.ID,
			AttemptID:      attempt.ID,
			WorkspaceRoot:  task.Workspace,
			Prompt:         task.Prompt,
			Summary:        attempt.Summary,
			FindingsPath:   attempt.FindingsPath,
			TraceLogPath:   attempt.TraceLogPath,
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
