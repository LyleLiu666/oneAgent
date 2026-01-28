package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type bashToolRequest struct {
	Command   string `json:"command"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

type BashToolResult struct {
	Shell           string `json:"shell"`
	SandboxMode     string `json:"sandbox_mode,omitempty"`
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
			Description: "在沙箱内执行 bash 命令（根目录为 $BASH_ROOT_DIR）。限制：不允许 heredoc(<<)；不允许 $/` 展开；文件路径必须在 $BASH_ROOT_DIR 内（/dev/null 例外）。不要用 bash 写文件：写文件用 write_file，改文件用 edit。命令尽量短小、分步执行，避免一次输出过长被截断。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "要执行的 bash 命令（在沙箱内运行）。",
					},
					"timeout_ms": map[string]any{
						"type":        "integer",
						"description": "（可选）超时时间（毫秒）。",
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

	dec, err := RequirePolicy(ctx, ToolIDBash)
	if err != nil {
		return nil, err
	}
	profile := CommandProfile(dec, "dev")
	if err := permissions.ValidateCommand(profile, req.Command, dec.Constraints.Allowlist); err != nil {
		return nil, err
	}
	mode := strings.ToLower(strings.TrimSpace(SandboxMode(dec, "none")))
	if mode == "" {
		mode = "none"
	}
	if mode != "none" && mode != "docker" {
		return nil, errors.New("unsupported sandbox_mode (expected none|docker)")
	}

	root, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	var result shell.Result
	if mode == "docker" {
		result, err = shell.RunBashDocker(ctx, req.Command, timeout, root)
	} else {
		result, err = shell.RunBash(ctx, req.Command, timeout, root, "")
	}
	if err != nil {
		return nil, err
	}

	return BashToolResult{
		Shell:           result.Shell,
		SandboxMode:     mode,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		ExitCode:        result.ExitCode,
		DurationMs:      result.Duration.Milliseconds(),
		TimedOut:        result.TimedOut,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
	}, nil
}
