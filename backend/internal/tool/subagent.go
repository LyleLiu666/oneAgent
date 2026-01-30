package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/skillrecall"
	"github.com/liu_y/oneAgent/backend/internal/subagent"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type subagentToolRequest struct {
	Task           string   `json:"task"`
	ContextSummary string   `json:"context_summary,omitempty"`
	ToolIDs        []string `json:"tool_ids,omitempty"`
	Scope          []string `json:"scope,omitempty"`

	MaxSteps          int `json:"max_steps,omitempty"`
	MaxRuntimeSeconds int `json:"max_runtime_seconds,omitempty"`
	MaxLogBytes       int `json:"max_log_bytes,omitempty"`

	SkillIDs []string `json:"skill_ids,omitempty"`

	KSkills *int `json:"k_skills,omitempty"`
}

type subagentToolResult struct {
	OK bool `json:"ok"`

	Summary      string `json:"summary,omitempty"`
	FindingsPath string `json:"findings_path,omitempty"`
	TraceLogPath string `json:"trace_log_path,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`

	PlanMarkDone []subagent.PlanMarkDoneRecord `json:"plan_mark_done,omitempty"`

	Error string `json:"error,omitempty"`
}

func subagentDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "subagent",
			Description: "启动一个隔离上下文的子 Agent 执行一个独立步骤。输入 task（必填）+ 可选 context_summary/scope/tool_ids/max_steps/max_runtime_seconds/k_skills；输出短总结 summary + findings_path/trace_log_path 指针。默认禁止递归（子 Agent 不挂载 subagent 工具本身）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "本步骤要完成的任务描述（必填）。",
					},
					"context_summary": map[string]any{
						"type":        "string",
						"description": "（可选）前序步骤短总结与引用（路径）。",
					},
					"tool_ids": map[string]any{
						"type":        "array",
						"description": "（可选）子 Agent 允许使用的工具 ID 列表；为空则默认使用主工具集（排除 subagent 本身）。",
						"items":       map[string]any{"type": "string"},
					},
					"scope": map[string]any{
						"type":        "array",
						"description": "（可选）子 Agent 可写范围（glob，基于 workspace 根目录的相对路径）。越界写/改/删会被拒绝。",
						"items":       map[string]any{"type": "string"},
					},
					"max_steps": map[string]any{
						"type":        "integer",
						"description": "（可选）最大工具调用步数（默认 200）。",
						"minimum":     1,
					},
					"max_runtime_seconds": map[string]any{
						"type":        "integer",
						"description": "（可选）最长运行时间秒（默认 3600）。",
						"minimum":     1,
					},
					"max_log_bytes": map[string]any{
						"type":        "integer",
						"description": "（可选）子 Agent trace.jsonl 最大字节数软上限（默认 64MiB；超过后将停止记录中间事件，但仍会写入结束事件）。",
						"minimum":     1,
					},
					"k_skills": map[string]any{
						"type":        "integer",
						"description": "（可选）按步骤自动注入的技能 Top-K（默认 3；0=不注入）。",
						"minimum":     0,
						"maximum":     8,
					},
					"skill_ids": map[string]any{
						"type":        "array",
						"description": "（可选）显式指定要注入的技能（优先基于 skill_id 解析，找不到则按 name 尝试）。会以“摘要块”写入子 Agent TurnContext（volatile），并提示子 Agent 需要时自行调用 `skill_read` 读取完整 SKILL.md（兼容旧名：`skill.read`）。",
						"items":       map[string]any{"type": "string"},
					},
				},
				"required":             []string{"task"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDSubagent, spec, runSubagentTool)
}

func runSubagentTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req subagentToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, errors.New("task is required")
	}

	sessionID := strings.TrimSpace(SessionIDFromContext(ctx))
	if sessionID == "" {
		return nil, errors.New("missing session context")
	}

	userID := strings.TrimSpace(UserIDFromContext(ctx))
	if userID == "" {
		userID = "local"
	}

	layout := RuntimeLayoutFromContext(ctx)
	if layout == nil {
		return nil, errors.New("missing runtime layout")
	}

	client := LLMClientFromContext(ctx)
	if client == nil {
		return nil, errors.New("missing llm client")
	}

	systemPrompt := SystemPromptFromContext(ctx)

	ws := WorkspaceFromContext(ctx)
	workspaceRoot := ""
	if ws.Enabled {
		workspaceRoot = ws.Root
	}

	// Build tool set for subagent (exclude subagent itself to prevent recursion).
	ids := req.ToolIDs
	if len(ids) == 0 {
		for _, def := range All() {
			if def.ID == ToolIDSubagent {
				continue
			}
			ids = append(ids, def.ID)
		}
	} else {
		filtered := make([]string, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || id == ToolIDSubagent {
				continue
			}
			filtered = append(filtered, id)
		}
		ids = filtered
	}

	snap, ok := PolicySnapshotFromContext(ctx)
	if !ok {
		snap = permissions.ResolveSnapshot(userID, permissions.DefaultPolicy(), time.Now())
	}
	defs, err := MountWithSnapshot(ids, snap)
	if err != nil {
		return nil, err
	}

	handlers := make(map[string]subagent.ToolHandler, len(defs))
	for _, def := range defs {
		handlers[def.Spec.Function.Name] = subagent.ToolHandler(def.Handler)
	}

	subCtx := ctx
	subCtx = ContextWithWorkspace(subCtx, WorkspaceConfig{
		Enabled:    ws.Enabled,
		Root:       ws.Root,
		WriteScope: req.Scope,
	})
	subCtx = ContextWithPolicySnapshot(subCtx, snap)

	skillsSummary := ""
	k := 3
	if req.KSkills != nil {
		k = *req.KSkills
	}
	if k < 0 {
		k = 0
	}
	if k > 8 {
		k = 8
	}

	manager := SkillManagerFromContext(ctx)
	if manager != nil {
		loadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		cat, err := manager.Load(loadCtx, workspaceRoot)
		cancel()
		if err == nil && cat != nil && len(cat.Skills) > 0 {
			seenSkills := make(map[string]bool)
			var b strings.Builder

			for _, raw := range req.SkillIDs {
				name := strings.TrimSpace(raw)
				if name == "" {
					continue
				}
				if seenSkills[strings.ToLower(name)] {
					continue
				}
				seenSkills[strings.ToLower(name)] = true

				s, ok := cat.ByID(name)
				if !ok {
					s, ok = cat.ByName(name)
				}
				if ok {
					b.WriteString("- ")
					b.WriteString(strings.TrimSpace(s.Name))
					b.WriteString(": ")
					if strings.TrimSpace(s.Description) != "" {
						b.WriteString(strings.TrimSpace(s.Description))
					} else {
						b.WriteString("（无描述）")
					}
					b.WriteString(" (")
					b.WriteString(string(s.Source))
					b.WriteString(")\n")
					continue
				}
				b.WriteString("- ")
				b.WriteString(name)
				b.WriteString(": （未找到该 skill）\n")
			}

			if k > 0 {
				eligibleCat := skill.FilterEligibleCatalog(cat, nil)
				res, err := skillrecall.Search(ctx, eligibleCat, task, skillrecall.Options{MaxResults: k, Timeout: 2 * time.Second}, nil)
				if err == nil && len(res.Candidates) > 0 {
					for _, cand := range res.Candidates {
						if cand.Score <= 0 {
							continue
						}
						key := strings.ToLower(strings.TrimSpace(cand.Skill.ID))
						if key == "" {
							key = strings.ToLower(strings.TrimSpace(cand.Skill.Name))
						}
						if key != "" && seenSkills[key] {
							continue
						}
						if key != "" {
							seenSkills[key] = true
						}

						b.WriteString("- ")
						b.WriteString(strings.TrimSpace(cand.Skill.Name))
						b.WriteString(": ")
						if strings.TrimSpace(cand.Skill.Description) != "" {
							b.WriteString(strings.TrimSpace(cand.Skill.Description))
						} else {
							b.WriteString("（无描述）")
						}
						b.WriteString(" (")
						b.WriteString(string(cand.Skill.Source))
						b.WriteString(")\n")
					}
				}
			}

			if b.Len() > 0 {
				b.WriteString("如需使用某个技能，请先调用 `skill_read`（按技能名称）读取该技能的 `SKILL.md`（兼容旧名：`skill.read`）。\n")
				skillsSummary = b.String()
			}
		}
	} else if len(req.SkillIDs) > 0 {
		var b strings.Builder
		for _, raw := range req.SkillIDs {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
		if b.Len() > 0 {
			b.WriteString("（注意：当前环境无法加载技能目录；如可用请调用 `skill_read` 读取技能详情；兼容旧名：`skill.read`。）\n")
			skillsSummary = b.String()
		}
	}

	result, runErr := subagent.Run(subCtx, subagent.RunRequest{
		ParentSessionID:   sessionID,
		UserID:            userID,
		SystemPrompt:      systemPrompt,
		Client:            client,
		Tools:             ToolsForLLM(defs),
		Handlers:          handlers,
		WorkspaceRoot:     workspaceRoot,
		WriteScope:        req.Scope,
		LogsBaseDir:       layout.SubagentLogsDir,
		Task:              task,
		ContextSummary:    strings.TrimSpace(req.ContextSummary),
		SkillsSummary:     skillsSummary,
		MaxSteps:          req.MaxSteps,
		MaxRuntimeSeconds: req.MaxRuntimeSeconds,
		MaxLogBytes:       req.MaxLogBytes,
	})

	out := subagentToolResult{
		OK:           runErr == nil,
		Summary:      result.Summary,
		FindingsPath: result.FindingsPath,
		TraceLogPath: result.TraceLogPath,
		RunID:        result.RunID,
		DurationMs:   result.DurationMs,
		PlanMarkDone: result.PlanMarkDone,
	}
	if runErr != nil {
		out.Error = runErr.Error()
	}

	if ledger := WorkLedgerFromContext(ctx); ledger != nil {
		summary := strings.TrimSpace(out.Summary)
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
		started := finished.Add(-time.Duration(maxInt64(0, result.DurationMs)) * time.Millisecond)
		_, _ = ledger.CreateReceipt(workledger.CreateReceiptInput{
			PrincipalID:   userID,
			WorkspaceRoot: workspaceRoot,
			Kind:          workledger.ReceiptKindSubagentRun,
			Status:        status,
			StartedAt:     started,
			FinishedAt:    finished,
			Summary:       summary,
			Artifacts: workledger.ReceiptArtifacts{
				FindingsPath: strings.TrimSpace(out.FindingsPath),
				TraceLogPath: strings.TrimSpace(out.TraceLogPath),
			},
			Signals: workledger.ReceiptSignals{DurationMs: result.DurationMs},
		})
	}

	return out, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
