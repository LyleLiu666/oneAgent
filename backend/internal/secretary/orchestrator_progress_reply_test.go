package secretary

import (
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestLooksLikeProgressQuery_IncludesFollowUpQuestions(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: "在运行了吗", want: true},
		{in: "现在有几个任务在进行", want: true},
		{in: "已完成的那个是什么", want: true},
		{in: "完成的那个是什么？", want: true},
		{in: "哪个完成了", want: true},
		{in: "帮我写十万字小说", want: false},
	}

	for _, tc := range cases {
		if got := looksLikeProgressQuery(tc.in); got != tc.want {
			t.Fatalf("looksLikeProgressQuery(%q)=%v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestBuildProgressReply_IncludesTaskDetails(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	running, err := store.CreateTask("local", ws, "写武侠小说", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask running: %v", err)
	}
	succeeded, err := store.CreateTask("local", ws, "生成大纲", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask succeeded: %v", err)
	}
	failed, err := store.CreateTask("local", ws, "补齐设定表", "prompt", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	now := time.Now().UTC()

	if _, err := store.UpdateTask(running.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptRunning
		a.StartedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask running: %v", err)
	}

	if _, err := store.UpdateTask(succeeded.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptSucceeded
		a.Summary = "完成大纲并输出到文件"
		a.FindingsPath = "/tmp/findings.md"
		a.FinishedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask succeeded: %v", err)
	}

	if _, err := store.UpdateTask(failed.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		a.Status = taskqueue.AttemptFailed
		a.Error = "boom"
		a.FinishedAt = &now
		return nil
	}); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	o := &Orchestrator{Tasks: store}
	st := State{
		TriageRuns: []TriageRun{{
			CreatedTaskIDs: []string{running.ID, succeeded.ID, failed.ID},
		}},
	}

	out, _, err := o.buildProgressReply("local", ws, st)
	if err != nil {
		t.Fatalf("buildProgressReply: %v", err)
	}

	if !strings.Contains(out, "运行") || !strings.Contains(out, "已完成") {
		t.Fatalf("expected status counts in output, got %q", out)
	}

	for _, title := range []string{running.Title, succeeded.Title, failed.Title} {
		if !strings.Contains(out, title) {
			t.Fatalf("expected output to include title %q, got %q", title, out)
		}
	}
	if !strings.Contains(out, "findings") && !strings.Contains(out, "Findings") && !strings.Contains(out, "产出") {
		t.Fatalf("expected output to mention artifacts for succeeded task, got %q", out)
	}
}
