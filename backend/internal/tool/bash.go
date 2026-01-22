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

type bashToolRequest struct {
	Command   string `json:"command"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

type BashToolResult struct {
	Shell           string `json:"shell"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExitCode        int    `json:"exit_code"`
	DurationMs      int64  `json:"duration_ms"`
	TimedOut        bool   `json:"timed_out"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
}

func bashDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "bash",
			Description: "Execute a bash command inside the sandbox rooted at $BASH_ROOT_DIR. Restrictions: no heredoc redirection (<<), no $/` expansions, and file paths must stay within $BASH_ROOT_DIR (except /dev/null). Prefer smart_edit for file writes/edits.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "Bash command to run inside the sandbox.",
					},
					"timeout_ms": map[string]any{
						"type":        "integer",
						"description": "Optional timeout in milliseconds.",
						"minimum":     1,
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDBash, spec, runBashTool)
}

func runBashTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req bashToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Command) == "" {
		return nil, errors.New("command is required")
	}

	cfg := config.GetConfig()
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	result, err := shell.RunBash(ctx, req.Command, timeout, cfg.BashRootDir, "")
	if err != nil {
		return nil, err
	}

	return BashToolResult{
		Shell:           result.Shell,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		ExitCode:        result.ExitCode,
		DurationMs:      result.Duration.Milliseconds(),
		TimedOut:        result.TimedOut,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
	}, nil
}
