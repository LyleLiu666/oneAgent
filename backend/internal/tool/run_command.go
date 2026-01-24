package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type runCommandToolRequest struct {
	Action            string `json:"action,omitempty"`
	Command           string `json:"command,omitempty"`
	JobID             string `json:"job_id,omitempty"`
	WaitSeconds       int    `json:"wait_seconds,omitempty"`
	MaxRuntimeSeconds int    `json:"max_runtime_seconds,omitempty"`
	StdoutOffset      int    `json:"stdout_offset,omitempty"`
	StderrOffset      int    `json:"stderr_offset,omitempty"`
}

type RunCommandStatus string

const (
	RunCommandStatusRunning   RunCommandStatus = "running"
	RunCommandStatusCompleted RunCommandStatus = "completed"
	RunCommandStatusFailed    RunCommandStatus = "failed"
	RunCommandStatusCanceled  RunCommandStatus = "canceled"
	RunCommandStatusTimedOut  RunCommandStatus = "timed_out"
)

type RunCommandResult struct {
	JobID           string           `json:"job_id"`
	Status          RunCommandStatus `json:"status"`
	StdoutDelta     string           `json:"stdout_delta,omitempty"`
	StderrDelta     string           `json:"stderr_delta,omitempty"`
	StdoutOffset    int              `json:"stdout_offset"`
	StderrOffset    int              `json:"stderr_offset"`
	ExitCode        int              `json:"exit_code,omitempty"`
	TimedOut        bool             `json:"timed_out,omitempty"`
	Canceled        bool             `json:"canceled,omitempty"`
	DurationMs      int64            `json:"duration_ms,omitempty"`
	ElapsedMs       int64            `json:"elapsed_ms"`
	StdoutTruncated bool             `json:"stdout_truncated"`
	StderrTruncated bool             `json:"stderr_truncated"`
}

func runCommandDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "run_command",
			Description: "异步执行 bash 命令（用于长任务，避免超时）。用 action=start 启动，返回 job_id；再用 action=poll 分段拉取 stdout/stderr（通过 stdout_offset/stderr_offset），避免一次输出过大。注意同 bash 沙箱限制：不要写文件；写文件用 write_file，改文件用 edit。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "start|poll|cancel。可省略：有 command 默认 start；有 job_id 默认 poll。",
						"enum":        []string{"start", "poll", "cancel"},
					},
					"command": map[string]any{
						"type":        "string",
						"description": "要运行的 bash 命令（action=start 必填）。",
					},
					"job_id": map[string]any{
						"type":        "string",
						"description": "action=start 返回的任务 ID（action=poll/cancel 必填）。",
					},
					"wait_seconds": map[string]any{
						"type":        "integer",
						"description": "（可选）等待任务完成的最长时间（秒），最多 30s。",
						"minimum":     0,
						"maximum":     30,
					},
					"max_runtime_seconds": map[string]any{
						"type":        "integer",
						"description": "（可选）任务最长运行时间（秒），默认 600s，最大 1800s。",
						"minimum":     1,
						"maximum":     1800,
					},
					"stdout_offset": map[string]any{
						"type":        "integer",
						"description": "（可选）stdout 的字节偏移；返回从该偏移之后的 stdout_delta 以及新的 stdout_offset。",
						"minimum":     0,
					},
					"stderr_offset": map[string]any{
						"type":        "integer",
						"description": "（可选）stderr 的字节偏移；返回从该偏移之后的 stderr_delta 以及新的 stderr_offset。",
						"minimum":     0,
					},
				},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDRunCommand, spec, runCommandTool)
}

func runCommandTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req runCommandToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	command := strings.TrimSpace(req.Command)
	jobID := strings.TrimSpace(req.JobID)

	if action == "" {
		if command != "" {
			action = "start"
		} else if jobID != "" {
			action = "poll"
		}
	}

	wait := time.Duration(req.WaitSeconds) * time.Second
	if wait < 0 {
		wait = 0
	}
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}

	cfg := config.GetConfig()

	switch action {
	case "start":
		if command == "" {
			return nil, errors.New("command is required")
		}
		maxRuntime := time.Duration(req.MaxRuntimeSeconds) * time.Second
		jobID, err := shell.StartBashAsync(command, maxRuntime, cfg.BashRootDir)
		if err != nil {
			return nil, err
		}
		poll, err := shell.PollBashAsync(ctx, jobID, wait, req.StdoutOffset, req.StderrOffset)
		if err != nil {
			return nil, err
		}
		return toRunCommandResult(poll), nil
	case "poll":
		if jobID == "" {
			return nil, errors.New("job_id is required")
		}
		poll, err := shell.PollBashAsync(ctx, jobID, wait, req.StdoutOffset, req.StderrOffset)
		if err != nil {
			return nil, err
		}
		return toRunCommandResult(poll), nil
	case "cancel":
		if jobID == "" {
			return nil, errors.New("job_id is required")
		}
		if err := shell.CancelBashAsync(jobID); err != nil {
			return nil, err
		}
		poll, err := shell.PollBashAsync(ctx, jobID, 0, req.StdoutOffset, req.StderrOffset)
		if err != nil {
			return nil, err
		}
		return toRunCommandResult(poll), nil
	default:
		return nil, errors.New("action must be start|poll|cancel")
	}
}

func toRunCommandResult(poll shell.AsyncBashPollResult) RunCommandResult {
	return RunCommandResult{
		JobID:           poll.JobID,
		Status:          RunCommandStatus(poll.Status),
		StdoutDelta:     poll.StdoutDelta,
		StderrDelta:     poll.StderrDelta,
		StdoutOffset:    poll.StdoutOffset,
		StderrOffset:    poll.StderrOffset,
		ExitCode:        poll.ExitCode,
		TimedOut:        poll.TimedOut,
		Canceled:        poll.Canceled,
		DurationMs:      poll.DurationMs,
		ElapsedMs:       poll.ElapsedMs,
		StdoutTruncated: poll.StdoutTruncated,
		StderrTruncated: poll.StderrTruncated,
	}
}
