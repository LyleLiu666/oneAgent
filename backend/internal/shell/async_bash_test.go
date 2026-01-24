package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAsyncBash_LargeStdout_CanDrainWithOffsets(t *testing.T) {
	root := t.TempDir()

	// Produce >64KB output to validate we can still drain it via offsets.
	jobID, err := StartBashAsync("yes a | head -c 70000", 30*time.Second, root)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdoutOffset, stderrOffset := 0, 0
	var stdout strings.Builder
	var stderr strings.Builder

	var last AsyncBashPollResult
	for i := 0; i < 500; i++ {
		res, err := PollBashAsync(ctx, jobID, 200*time.Millisecond, stdoutOffset, stderrOffset, 8192)
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		last = res

		stdout.WriteString(res.StdoutDelta)
		stderr.WriteString(res.StderrDelta)
		stdoutOffset = res.StdoutOffset
		stderrOffset = res.StderrOffset

		done := res.Status != AsyncBashStatusRunning && res.Status != AsyncBashStatusCanceling
		if done && res.StdoutDelta == "" && res.StderrDelta == "" {
			break
		}
	}

	if last.Status == AsyncBashStatusRunning || last.Status == AsyncBashStatusCanceling {
		t.Fatalf("expected job to finish, got status=%s", last.Status)
	}
	if last.StdoutTruncated {
		t.Fatalf("expected stdout not truncated, got stdout_base_offset=%d", last.StdoutBaseOffset)
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if stdout.Len() != 70000 {
		t.Fatalf("expected stdout bytes=%d, got %d", 70000, stdout.Len())
	}
}

func TestAsyncBash_PollReturnsOnOutputNotOnlyCompletion(t *testing.T) {
	root := t.TempDir()

	jobID, err := StartBashAsync("echo hi; sleep 5", 30*time.Second, root)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = CancelBashAsync(jobID) })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := PollBashAsync(ctx, jobID, 30*time.Second, 0, 0, 16384)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !strings.Contains(res.StdoutDelta, "hi") {
		t.Fatalf("expected stdout to contain %q, got %q", "hi", res.StdoutDelta)
	}
	if res.Status != AsyncBashStatusRunning && res.Status != AsyncBashStatusCanceling {
		t.Fatalf("expected running/canceling, got status=%s", res.Status)
	}
}

func TestAsyncBash_Cancel_UsesCancelingStatus(t *testing.T) {
	root := t.TempDir()

	jobID, err := StartBashAsync("sleep 30", 30*time.Second, root)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := CancelBashAsync(jobID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := PollBashAsync(ctx, jobID, 1*time.Second, 0, 0, 16384)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Canceled && res.Status == AsyncBashStatusRunning {
		t.Fatalf("expected status!=running when canceled=true, got status=%s", res.Status)
	}
	if res.Status != AsyncBashStatusCanceling && res.Status != AsyncBashStatusCanceled {
		t.Fatalf("expected status canceling/canceled, got %s", res.Status)
	}
}
