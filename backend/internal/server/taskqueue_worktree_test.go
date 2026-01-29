package server

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestTaskQueueRunner_WorktreeMode_CleansUpAndWritesReceiptEvidence(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	ctx := context.Background()
	userID := "local"

	// Use a local URL that fails fast (connection refused) so the test doesn't hang on network timeouts.
	baseURL := "http://127.0.0.1:1"
	if _, err := url.Parse(baseURL); err != nil {
		t.Fatalf("unexpected baseURL: %v", err)
	}

	_, err = rt.Settings.CreateProvider(ctx, settingsdb.Provider{
		ID:           "p1",
		UserID:       userID,
		Name:         "test",
		ProviderType: "openai",
		BaseURL:      baseURL,
		APIKey:       "test",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	_, err = rt.Settings.CreateModel(ctx, settingsdb.Model{
		ID:            "m1",
		ProviderID:    "p1",
		UserID:        userID,
		Name:          "test",
		Model:         "gpt-4o-mini",
		IsDefault:     true,
		EnableKVCache: false,
		Options:       map[string]any{},
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	if err := ensureTaskQueue(rt); err != nil {
		t.Fatalf("ensure task queue: %v", err)
	}

	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Test")
	writeFile(t, filepath.Join(workspace, "a.txt"), "hello\n")
	runGit(t, workspace, "add", ".")
	runGit(t, workspace, "commit", "-m", "init")

	if err := os.MkdirAll(filepath.Join(workspace, ".oneagent"), 0o700); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".oneagent", "project.json"), []byte(`{"attempt_execution_mode":"worktree"}`), 0o600); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	created, err := rt.Tasks.CreateTask(userID, workspace, "t", "noop", "m1", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if len(created.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(created.Attempts))
	}

	attempt := created.Attempts[0]
	_, execErr := rt.TaskRunner.ExecuteAttempt(ctx, created, attempt, nil)
	if execErr == nil {
		t.Fatalf("expected ExecuteAttempt error due to unreachable provider")
	}

	updated, err := rt.Tasks.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	latest := updated.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}
	if strings.TrimSpace(latest.WorktreeRoot) == "" {
		t.Fatalf("expected worktree_root to be recorded on attempt")
	}
	if len(strings.TrimSpace(latest.BaseCommitSHA)) < 8 {
		t.Fatalf("expected base_commit_sha to be recorded on attempt")
	}

	if _, statErr := os.Stat(latest.WorktreeRoot); statErr == nil {
		t.Fatalf("expected worktree_root to be cleaned up (ONEAGENT_KEEP_WORKTREES not set)")
	}

	receipts, err := rt.WorkLedger.ListReceipts(workledger.ListReceiptsQuery{
		PrincipalID: userID,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("list receipts: %v", err)
	}
	if len(receipts) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(receipts))
	}
	if strings.TrimSpace(receipts[0].Artifacts.WorktreeRoot) == "" {
		t.Fatalf("expected receipt.worktree_root")
	}
	if len(strings.TrimSpace(receipts[0].Artifacts.BaseCommitSHA)) < 8 {
		t.Fatalf("expected receipt.base_commit_sha")
	}
}
