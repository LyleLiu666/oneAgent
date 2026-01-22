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
	Action       string `json:"action,omitempty"`
	Command      string `json:"command,omitempty"`
	JobID        string `json:"job_id,omitempty"`
	WaitMs       int    `json:"wait_ms,omitempty"`
	MaxRuntimeMs int    `json:"max_runtime_ms,omitempty"`
	StdoutOffset int    `json:"stdout_offset,omitempty"`
	StderrOffset int    `json:"stderr_offset,omitempty"`
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
			Description: "Run a bash command asynchronously to avoid timeouts. Use action=start with {command, wait_ms?, max_runtime_ms?}; it returns {job_id, status, stdout_delta/stderr_delta, stdout_offset/stderr_offset}. Then call action=poll with {job_id, wait_ms?, stdout_offset?, stderr_offset?} to fetch new output until status becomes completed/failed/timed_out/canceled.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "start|poll|cancel. If omitted, defaults to start when command is provided, otherwise poll when job_id is provided.",
						"enum":        []string{"start", "poll", "cancel"},
					},
					"command": map[string]any{
						"type":        "string",
						"description": "Bash command to run (required for action=start).",
					},
					"job_id": map[string]any{
						"type":        "string",
						"description": "Job ID returned from action=start (required for action=poll|cancel).",
					},
					"wait_ms": map[string]any{
						"type":        "integer",
						"description": "Optional: wait up to this many milliseconds for completion before returning. Max 30000ms.",
						"minimum":     0,
						"maximum":     30000,
					},
					"max_runtime_ms": map[string]any{
						"type":        "integer",
						"description": "Optional: maximum runtime for the started job (milliseconds). Default 600000ms; max 1800000ms.",
						"minimum":     1,
						"maximum":     1800000,
					},
					"stdout_offset": map[string]any{
						"type":        "integer",
						"description": "Optional: byte offset into stdout; tool returns stdout_delta since this offset, plus updated stdout_offset.",
						"minimum":     0,
					},
					"stderr_offset": map[string]any{
						"type":        "integer",
						"description": "Optional: byte offset into stderr; tool returns stderr_delta since this offset, plus updated stderr_offset.",
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

	wait := time.Duration(req.WaitMs) * time.Millisecond
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
		maxRuntime := time.Duration(req.MaxRuntimeMs) * time.Millisecond
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

