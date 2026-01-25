package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestRunCommandTool_StartThenPoll_ReturnsDeltas(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	startArgs := map[string]any{
		"action":              "start",
		"command":             "echo A; sleep 2; echo B",
		"wait_seconds":        1,
		"max_runtime_seconds": 5,
	}
	startRaw, _ := json.Marshal(startArgs)

	startAny, err := runCommandTool(ctx, startRaw)
	if err != nil {
		t.Fatalf("start: expected no error, got %v", err)
	}
	startRes, ok := startAny.(RunCommandResult)
	if !ok {
		t.Fatalf("start: expected RunCommandResult, got %T", startAny)
	}
	if startRes.JobID == "" {
		t.Fatalf("start: expected job_id to be set")
	}
	if startRes.Status != RunCommandStatusRunning {
		t.Fatalf("start: expected status=%q, got %q", RunCommandStatusRunning, startRes.Status)
	}
	if !strings.Contains(startRes.StdoutDelta, "A") {
		t.Fatalf("start: expected stdout_delta to contain %q, got %q", "A", startRes.StdoutDelta)
	}
	if strings.Contains(startRes.StdoutDelta, "B") {
		t.Fatalf("start: did not expect stdout_delta to contain %q, got %q", "B", startRes.StdoutDelta)
	}
	if startRes.StdoutOffset <= 0 {
		t.Fatalf("start: expected stdout_offset > 0, got %d", startRes.StdoutOffset)
	}
	if startRes.StderrOffset != 0 {
		t.Fatalf("start: expected stderr_offset=0, got %d", startRes.StderrOffset)
	}

	pollArgs := map[string]any{
		"action":        "poll",
		"job_id":        startRes.JobID,
		"wait_seconds":  2,
		"stdout_offset": startRes.StdoutOffset,
		"stderr_offset": startRes.StderrOffset,
	}
	pollRaw, _ := json.Marshal(pollArgs)

	pollAny, err := runCommandTool(ctx, pollRaw)
	if err != nil {
		t.Fatalf("poll: expected no error, got %v", err)
	}
	pollRes, ok := pollAny.(RunCommandResult)
	if !ok {
		t.Fatalf("poll: expected RunCommandResult, got %T", pollAny)
	}
	if pollRes.JobID != startRes.JobID {
		t.Fatalf("poll: expected job_id=%q, got %q", startRes.JobID, pollRes.JobID)
	}
	if pollRes.Status != RunCommandStatusRunning && pollRes.Status != RunCommandStatusCompleted {
		t.Fatalf("poll: expected status running|completed, got %q", pollRes.Status)
	}
	if pollRes.StdoutOffset < startRes.StdoutOffset {
		t.Fatalf("poll: expected stdout_offset >= %d, got %d", startRes.StdoutOffset, pollRes.StdoutOffset)
	}

	seenB := strings.Contains(pollRes.StdoutDelta, "B")
	final := pollRes
	for i := 0; i < 5 && final.Status == RunCommandStatusRunning; i++ {
		nextRaw, _ := json.Marshal(map[string]any{
			"action":        "poll",
			"job_id":        startRes.JobID,
			"wait_seconds":  2,
			"stdout_offset": final.StdoutOffset,
			"stderr_offset": final.StderrOffset,
		})
		nextAny, err := runCommandTool(ctx, nextRaw)
		if err != nil {
			t.Fatalf("poll again: expected no error, got %v", err)
		}
		nextRes, ok := nextAny.(RunCommandResult)
		if !ok {
			t.Fatalf("poll again: expected RunCommandResult, got %T", nextAny)
		}
		final = nextRes
		if strings.Contains(final.StdoutDelta, "B") {
			seenB = true
		}
	}

	if !seenB {
		t.Fatalf("poll: expected stdout_delta to contain %q at least once", "B")
	}
	if final.StdoutOffset <= startRes.StdoutOffset {
		t.Fatalf("poll: expected stdout_offset to advance from %d, got %d", startRes.StdoutOffset, final.StdoutOffset)
	}
	if final.Status != RunCommandStatusCompleted {
		t.Fatalf("poll: expected status=%q, got %q", RunCommandStatusCompleted, final.Status)
	}
	if final.ExitCode != 0 {
		t.Fatalf("poll: expected exit_code=0, got %d", final.ExitCode)
	}
	if final.TimedOut {
		t.Fatalf("poll: expected timed_out=false")
	}
	if final.DurationMs <= 0 {
		t.Fatalf("poll: expected duration_ms > 0, got %d", final.DurationMs)
	}
	if final.ElapsedMs < final.DurationMs {
		t.Fatalf("poll: expected elapsed_ms >= duration_ms, got elapsed=%d duration=%d", final.ElapsedMs, final.DurationMs)
	}
}

func TestRunCommandTool_PollUnknownJob_ReturnsError(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	raw, _ := json.Marshal(map[string]any{
		"action":       "poll",
		"job_id":       "missing",
		"wait_seconds": 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := runCommandTool(ctx, raw); err == nil {
		t.Fatalf("expected error for missing job")
	}
}
