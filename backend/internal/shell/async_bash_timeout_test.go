package shell

import (
	"context"
	"testing"
	"time"
)

func TestAsyncBash_TimesOutAndReportsTimedOutStatus(t *testing.T) {
	root := t.TempDir()

	jobID, err := StartBashAsync("sleep 5", 200*time.Millisecond, root)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var last AsyncBashPollResult
	for i := 0; i < 200; i++ {
		res, err := PollBashAsync(ctx, jobID, 100*time.Millisecond, 0, 0, 16384)
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		last = res
		if res.Status != AsyncBashStatusRunning && res.Status != AsyncBashStatusCanceling {
			break
		}
	}

	if last.Status != AsyncBashStatusTimedOut || !last.TimedOut {
		t.Fatalf("expected timed_out, got status=%s timed_out=%v exit_code=%d", last.Status, last.TimedOut, last.ExitCode)
	}
}
