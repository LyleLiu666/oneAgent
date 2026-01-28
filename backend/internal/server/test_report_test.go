package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestGenerateTestReport_GoWorkspace_WritesReport(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	outDir := t.TempDir()
	report, reason := generateTestReport(context.Background(), testReportInput{
		WorkspaceRoot: ws,
		OutputDir:     outDir,
		Enable:        true,
		Timeout:       10 * time.Millisecond,
		RunBash: func(ctx context.Context, command string, timeout time.Duration) (tool.BashToolResult, error) {
			if strings.TrimSpace(command) != "go test ./..." {
				t.Fatalf("unexpected command: %q", command)
			}
			return tool.BashToolResult{
				Shell:      "bash",
				Stdout:     "ok\n",
				Stderr:     "",
				ExitCode:   0,
				DurationMs: 12,
			}, nil
		},
	})

	if reason != "" {
		t.Fatalf("expected empty reason, got %q", reason)
	}
	if report == "" {
		t.Fatalf("expected report path")
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("stat report: %v", err)
	}
	data, _ := os.ReadFile(report)
	if !strings.Contains(string(data), "# Test Report") || !strings.Contains(string(data), "go test ./...") {
		t.Fatalf("unexpected report content:\n%s", string(data))
	}
}

func TestGenerateTestReport_Disabled(t *testing.T) {
	got, reason := generateTestReport(context.Background(), testReportInput{Enable: false})
	if got != "" || reason != "disabled" {
		t.Fatalf("unexpected: %q %q", got, reason)
	}
}

