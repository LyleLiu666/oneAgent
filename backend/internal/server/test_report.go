package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type bashRunner func(ctx context.Context, command string, timeout time.Duration) (tool.BashToolResult, error)

type testReportInput struct {
	WorkspaceRoot string
	OutputDir     string

	Enable bool

	Timeout  time.Duration
	MaxBytes int

	RunBash bashRunner
}

func generateTestReport(ctx context.Context, in testReportInput) (string, string) {
	if !in.Enable {
		return "", "disabled"
	}
	if strings.TrimSpace(in.WorkspaceRoot) == "" {
		return "", "missing_workspace"
	}
	if strings.TrimSpace(in.OutputDir) == "" {
		return "", "missing_output_dir"
	}
	if in.RunBash == nil {
		return "", "missing_bash_runner"
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	maxBytes := in.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	if maxBytes > 2*1024*1024 {
		maxBytes = 2 * 1024 * 1024
	}

	cmd, kind, reason := detectTestCommand(in.WorkspaceRoot)
	if cmd == "" {
		return "", reason
	}

	res, err := in.RunBash(ctx, cmd, timeout)
	if err != nil {
		reason = "bash_error: " + err.Error()
	}

	reportPath := filepath.Join(in.OutputDir, "TEST_REPORT.md")
	content := buildTestReportMarkdown(kind, cmd, res, err)
	if len(content) > maxBytes {
		content = content[:maxBytes] + "\n\n...(truncated)\n"
	}
	_ = os.MkdirAll(filepath.Dir(reportPath), 0o700)
	if writeErr := os.WriteFile(reportPath, []byte(content), 0o600); writeErr != nil {
		return "", "write_error: " + writeErr.Error()
	}

	return reportPath, reason
}

func detectTestCommand(workspaceRoot string) (command string, kind string, reason string) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return "", "", "missing_workspace"
	}

	if fileExists(filepath.Join(workspaceRoot, "go.mod")) {
		return "go test ./...", "go", ""
	}
	if fileExists(filepath.Join(workspaceRoot, "package.json")) {
		// Best-effort; supports Vite/Vitest setups where `npm test -- --run` works.
		return "npm test -- --run", "node", ""
	}
	if fileExists(filepath.Join(workspaceRoot, "pyproject.toml")) || fileExists(filepath.Join(workspaceRoot, "requirements.txt")) {
		return "pytest -q", "python", ""
	}

	return "", "", "no_tests_detected"
}

func buildTestReportMarkdown(kind, command string, res tool.BashToolResult, runErr error) string {
	var b strings.Builder
	b.WriteString("# Test Report\n\n")
	if strings.TrimSpace(kind) != "" {
		b.WriteString("- kind: ")
		b.WriteString(strings.TrimSpace(kind))
		b.WriteString("\n")
	}
	b.WriteString("- command: `")
	b.WriteString(strings.TrimSpace(command))
	b.WriteString("`\n")
	b.WriteString(fmt.Sprintf("- exit_code: %d\n", res.ExitCode))
	b.WriteString(fmt.Sprintf("- duration_ms: %d\n", res.DurationMs))
	if res.TimedOut {
		b.WriteString("- timed_out: true\n")
	}
	if runErr != nil {
		b.WriteString("- error: ")
		b.WriteString(runErr.Error())
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if strings.TrimSpace(res.Stdout) != "" {
		b.WriteString("## Stdout\n\n```text\n")
		b.WriteString(res.Stdout)
		if !strings.HasSuffix(res.Stdout, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}
	if strings.TrimSpace(res.Stderr) != "" {
		b.WriteString("## Stderr\n\n```text\n")
		b.WriteString(res.Stderr)
		if !strings.HasSuffix(res.Stderr, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}
	if strings.TrimSpace(res.Stdout) == "" && strings.TrimSpace(res.Stderr) == "" {
		b.WriteString("## Output\n\n(no output)\n\n")
	}

	return b.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
