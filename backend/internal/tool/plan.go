package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/plan"
)

type planToolRequest struct {
	Action     string           `json:"action,omitempty"`
	TaskID     string           `json:"task_id,omitempty"`
	Title      string           `json:"title,omitempty"`
	Status     string           `json:"status,omitempty"`
	Scope      []string         `json:"scope,omitempty"`
	Acceptance *plan.Acceptance `json:"acceptance,omitempty"`
	Template   string           `json:"template,omitempty"`
	Overwrite  bool             `json:"overwrite,omitempty"`
}

type planTaskDTO struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Status     string          `json:"status"`
	Scope      []string        `json:"scope,omitempty"`
	Acceptance plan.Acceptance `json:"acceptance,omitempty"`
}

type planToolResult struct {
	OK bool `json:"ok"`

	Action   string `json:"action"`
	PlanPath string `json:"plan_path"`

	Existed bool `json:"existed,omitempty"`
	Updated bool `json:"updated,omitempty"`

	TaskID  string        `json:"task_id,omitempty"`
	Pass    bool          `json:"pass,omitempty"`
	Message string        `json:"message,omitempty"`
	Reason  string        `json:"reason,omitempty"`
	Tasks   []planTaskDTO `json:"tasks,omitempty"`
}

func planDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "plan",
			Description: "计划模块：init|get|upsert_task|delete_task|mark_done（兼容旧 action：start→init、update→get、complete→mark_done）。计划文件固定为 $WORKSPACE_ROOT/.oneagent/PLAN.md。upsert_task 用于新增/更新任务（推荐先写 acceptance/scope）；mark_done 会触发 observer 只读验收（仅文件/内容校验，不执行命令），通过后才会把任务写为 done；失败会返回 `【plan中某个任务标记done失败】` 与原因且不写回。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "init|get|upsert_task|delete_task|mark_done（兼容 start|update|complete）",
						"enum":        []string{"init", "get", "upsert_task", "delete_task", "mark_done", "start", "update", "complete"},
					},
					"task_id": map[string]any{
						"type":        "string",
						"description": "action=mark_done/delete_task 时必填；action=upsert_task 时可选（不填则从 title 生成）。",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "action=upsert_task 时必填：任务标题。",
					},
					"status": map[string]any{
						"type":        "string",
						"description": "action=upsert_task 时可选：todo|doing|done（done 请优先用 mark_done，以触发验收）。",
						"enum":        []string{"todo", "doing", "done"},
					},
					"scope": map[string]any{
						"type":        "array",
						"description": "action=upsert_task 时可选：可写范围 glob 列表（相对 workspace 根目录）。",
						"items": map[string]any{
							"type": "string",
						},
					},
					"acceptance": map[string]any{
						"type":        "object",
						"description": "action=upsert_task 时可选：验收标准（observer 仅做只读校验）。",
						"properties": map[string]any{
							"files": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "string"},
							},
							"must_contain": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"path": map[string]any{"type": "string"},
										"text": map[string]any{"type": "string"},
									},
									"required":             []string{"path", "text"},
									"additionalProperties": false,
								},
							},
						},
						"additionalProperties": false,
					},
					"template": map[string]any{
						"type":        "string",
						"description": "action=init 时可选：自定义 PLAN.md 内容（为空则使用内置模板）。",
					},
					"overwrite": map[string]any{
						"type":        "boolean",
						"description": "action=init 时可选：是否覆盖已存在的 PLAN.md（默认 false）。",
					},
				},
				"required":             []string{"action"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDPlan, spec, runPlanTool)
}

func runPlanTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req planToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		return nil, errors.New("action is required")
	}

	switch action {
	case "start":
		action = "init"
	case "update":
		action = "get"
	case "complete":
		action = "mark_done"
	}

	root, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		return nil, err
	}

	switch action {
	case "init":
		planPath, existed, err := plan.EnsurePlanFile(root, req.Template, req.Overwrite)
		if err != nil {
			return nil, err
		}
		return planToolResult{
			OK:       true,
			Action:   action,
			PlanPath: planPath,
			Existed:  existed,
		}, nil

	case "get":
		parsed, planPath, _, err := plan.Load(root)
		if err != nil {
			if errors.Is(err, plan.ErrPlanNotFound) {
				return planToolResult{
					OK:       false,
					Action:   action,
					PlanPath: planPath,
					Message:  "PLAN.md not found; run plan init first",
				}, nil
			}
			return nil, err
		}

		tasks := make([]planTaskDTO, 0, len(parsed.Tasks))
		for _, task := range parsed.Tasks {
			tasks = append(tasks, planTaskDTO{
				ID:         task.ID,
				Title:      task.Title,
				Status:     task.Status,
				Scope:      task.Scope,
				Acceptance: task.Acceptance,
			})
		}

		return planToolResult{
			OK:       true,
			Action:   action,
			PlanPath: planPath,
			Tasks:    tasks,
		}, nil

	case "mark_done":
		taskID := strings.TrimSpace(req.TaskID)
		if taskID == "" {
			return nil, errors.New("task_id is required")
		}

		result, err := plan.MarkDone(root, taskID)
		if err != nil {
			if errors.Is(err, plan.ErrPlanNotFound) || errors.Is(err, plan.ErrTaskNotFound) {
				var tasks []planTaskDTO
				if parsed, _, _, loadErr := plan.Load(root); loadErr == nil {
					tasks = make([]planTaskDTO, 0, len(parsed.Tasks))
					for _, task := range parsed.Tasks {
						tasks = append(tasks, planTaskDTO{
							ID:         task.ID,
							Title:      task.Title,
							Status:     task.Status,
							Scope:      task.Scope,
							Acceptance: task.Acceptance,
						})
					}
				}
				return planToolResult{
					OK:       false,
					Action:   action,
					PlanPath: plan.DefaultPlanPath(root),
					TaskID:   taskID,
					Message:  fmt.Sprintf("plan mark_done failed: %v", err),
					Tasks:    tasks,
				}, nil
			}
			return nil, err
		}

		return planToolResult{
			OK:       result.Pass,
			Action:   action,
			PlanPath: result.PlanPath,
			TaskID:   result.TaskID,
			Pass:     result.Pass,
			Updated:  result.Updated,
			Message:  result.Message,
			Reason:   result.Reason,
		}, nil

	case "upsert_task":
		title := strings.TrimSpace(req.Title)
		if title == "" {
			return nil, errors.New("title is required")
		}

		parsed, planPath, _, err := plan.Load(root)
		if err != nil {
			if errors.Is(err, plan.ErrPlanNotFound) {
				return planToolResult{
					OK:       false,
					Action:   action,
					PlanPath: planPath,
					Message:  "PLAN.md not found; run plan init first",
				}, nil
			}
			return nil, err
		}

		taskID := strings.TrimSpace(req.TaskID)
		if taskID == "" {
			taskID = plan.SuggestTaskID(parsed, title)
		}

		status := strings.ToLower(strings.TrimSpace(req.Status))
		if status == "" {
			status = "todo"
		}
		if status == "done" {
			return nil, errors.New("status=done is not allowed in upsert_task; use mark_done to trigger validation")
		}

		acc := plan.Acceptance{}
		if req.Acceptance != nil {
			acc = *req.Acceptance
		}

		upsert, err := plan.UpsertTask(root, plan.Task{
			ID:         taskID,
			Title:      title,
			Status:     status,
			Scope:      req.Scope,
			Acceptance: acc,
		})
		if err != nil {
			return nil, err
		}

		return planToolResult{
			OK:       true,
			Action:   action,
			PlanPath: upsert.PlanPath,
			TaskID:   upsert.TaskID,
			Existed:  upsert.Existed,
			Updated:  upsert.Updated,
		}, nil

	case "delete_task":
		taskID := strings.TrimSpace(req.TaskID)
		if taskID == "" {
			return nil, errors.New("task_id is required")
		}

		del, err := plan.DeleteTask(root, taskID)
		if err != nil {
			if errors.Is(err, plan.ErrPlanNotFound) || errors.Is(err, plan.ErrTaskNotFound) {
				return planToolResult{
					OK:       false,
					Action:   action,
					PlanPath: plan.DefaultPlanPath(root),
					TaskID:   taskID,
					Message:  fmt.Sprintf("plan delete_task failed: %v", err),
				}, nil
			}
			return nil, err
		}

		return planToolResult{
			OK:       true,
			Action:   action,
			PlanPath: del.PlanPath,
			TaskID:   del.TaskID,
			Updated:  del.Updated,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}
