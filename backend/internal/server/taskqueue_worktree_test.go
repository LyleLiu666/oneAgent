package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func waitForTaskAttemptTerminal(t *testing.T, rt *runtime.Runtime, taskID string) taskqueue.Task {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tk, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		a := tk.LatestAttempt()
		if a != nil && a.Status != taskqueue.AttemptQueued && a.Status != taskqueue.AttemptRunning {
			return tk
		}
		time.Sleep(25 * time.Millisecond)
	}

	tk, err := rt.Tasks.GetTask(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	latest := tk.LatestAttempt()
	if latest != nil && (latest.Status == taskqueue.AttemptQueued || latest.Status == taskqueue.AttemptRunning) {
		t.Fatalf("expected attempt to reach terminal status, got %q", latest.Status)
	}
	return tk
}

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

	if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	updated := waitForTaskAttemptTerminal(t, rt, created.ID)
	latest := updated.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}
	if latest.Status == taskqueue.AttemptQueued || latest.Status == taskqueue.AttemptRunning {
		t.Fatalf("expected attempt to reach terminal status, got %q", latest.Status)
	}
	if strings.TrimSpace(latest.WorktreeRoot) == "" {
		t.Fatalf("expected worktree_root to be recorded on attempt")
	}
	if len(strings.TrimSpace(latest.BaseCommitSHA)) < 8 {
		t.Fatalf("expected base_commit_sha to be recorded on attempt")
	}
	if got := strings.TrimSpace(latest.WorktreeMode); got != "worktree" {
		t.Fatalf("expected worktree_mode=%q, got %q", "worktree", got)
	}
	if got := strings.TrimSpace(latest.WorktreeCleanupStatus); got != "cleaned" {
		t.Fatalf("expected worktree_cleanup_status=%q, got %q", "cleaned", got)
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
	if strings.TrimSpace(receipts[0].ArtifactManifestVersion) == "" || strings.TrimSpace(receipts[0].ArtifactManifestPath) == "" {
		t.Fatalf("expected receipt to include artifact manifest reference, got %+v", receipts[0])
	}
	if _, err := os.Stat(receipts[0].ArtifactManifestPath); err != nil {
		t.Fatalf("artifact manifest path missing: %v", err)
	}
	if got := strings.TrimSpace(string(receipts[0].EvidenceCompleteness)); got == "" {
		t.Fatalf("expected evidence_completeness to be set")
	} else if got != "complete" && got != "partial" && got != "insufficient" {
		t.Fatalf("unexpected evidence_completeness: %q", got)
	}
	if strings.TrimSpace(receipts[0].Artifacts.WorktreeRoot) == "" {
		t.Fatalf("expected receipt.worktree_root")
	}
	if len(strings.TrimSpace(receipts[0].Artifacts.BaseCommitSHA)) < 8 {
		t.Fatalf("expected receipt.base_commit_sha")
	}
	if strings.TrimSpace(receipts[0].Artifacts.CheckpointPath) == "" {
		t.Fatalf("expected receipt.checkpoint_path")
	}
	if got := strings.TrimSpace(receipts[0].Artifacts.WorktreeMode); got != "worktree" {
		t.Fatalf("expected receipt.worktree_mode=%q, got %q", "worktree", got)
	}
	if got := strings.TrimSpace(receipts[0].Artifacts.WorktreeCleanupStatus); got != "cleaned" {
		t.Fatalf("expected receipt.worktree_cleanup_status=%q, got %q", "cleaned", got)
	}
}

func TestTaskQueueRunner_WorktreeMode_NonGitWorkspace_FailsClosed(t *testing.T) {
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
	if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	updated := waitForTaskAttemptTerminal(t, rt, created.ID)
	latest := updated.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}
	if latest.Status == taskqueue.AttemptQueued || latest.Status == taskqueue.AttemptRunning {
		t.Fatalf("expected attempt to reach terminal status, got %q", latest.Status)
	}
	if got := strings.TrimSpace(latest.WorktreeMode); got != "worktree" {
		t.Fatalf("expected worktree_mode=%q, got %q", "worktree", got)
	}
	if strings.TrimSpace(latest.WorktreeRoot) != "" {
		t.Fatalf("expected no worktree_root for non-git workspace")
	}
	if strings.TrimSpace(latest.BaseCommitSHA) != "" {
		t.Fatalf("expected no base_commit_sha for non-git workspace")
	}
	if got := strings.ToLower(strings.TrimSpace(latest.Error)); !strings.Contains(got, "not a git") {
		t.Fatalf("expected non-git error, got %q", latest.Error)
	}
	worktreeRoot := filepath.Join(rt.Layout.TasksDir, updated.ID, "attempts", latest.ID, "worktree")
	if _, err := os.Stat(worktreeRoot); err == nil {
		t.Fatalf("expected worktree dir to not exist")
	}
}

func TestTaskQueueRunner_WorktreeMode_CleanupFailure_IsRecorded(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	orig := worktreeRemoveFn
	worktreeRemoveFn = func(ctx context.Context, workspaceRoot, worktreeRoot string) error {
		return errors.New("file is in use")
	}
	t.Cleanup(func() { worktreeRemoveFn = orig })

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
	if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	updated := waitForTaskAttemptTerminal(t, rt, created.ID)
	latest := updated.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}
	if strings.TrimSpace(latest.WorktreeRoot) == "" {
		t.Fatalf("expected worktree_root to be recorded on attempt")
	}
	if got := strings.TrimSpace(latest.WorktreeCleanupStatus); got != "cleanup_failed" {
		t.Fatalf("expected worktree_cleanup_status=%q, got %q", "cleanup_failed", got)
	}
	if strings.TrimSpace(latest.WorktreeCleanupError) == "" {
		t.Fatalf("expected worktree_cleanup_error to be recorded")
	}
	if strings.TrimSpace(latest.WorktreeCleanupHint) == "" {
		t.Fatalf("expected worktree_cleanup_hint to be recorded")
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
	if got := strings.TrimSpace(receipts[0].Artifacts.WorktreeCleanupStatus); got != "cleanup_failed" {
		t.Fatalf("expected receipt.worktree_cleanup_status=%q, got %q", "cleanup_failed", got)
	}
	if strings.TrimSpace(receipts[0].Artifacts.WorktreeCleanupError) == "" {
		t.Fatalf("expected receipt.worktree_cleanup_error")
	}
	if strings.TrimSpace(receipts[0].Artifacts.WorktreeCleanupHint) == "" {
		t.Fatalf("expected receipt.worktree_cleanup_hint")
	}
}

func TestTaskQueueRunner_WorktreeMode_ConcurrentWorkspaces_CleansUpWorktrees(t *testing.T) {
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

	workspaces := []string{t.TempDir(), t.TempDir()}
	for _, workspace := range workspaces {
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
	}

	taskIDs := make([]string, 0, len(workspaces))
	for i, workspace := range workspaces {
		created, err := rt.Tasks.CreateTask(userID, workspace, "t", fmt.Sprintf("noop-%d", i+1), "m1", taskqueue.Limits{})
		if err != nil {
			t.Fatalf("create task: %v", err)
		}
		taskIDs = append(taskIDs, created.ID)
	}

	for _, id := range taskIDs {
		if err := rt.TaskRunner.Enqueue(id); err != nil {
			t.Fatalf("enqueue task: %v", err)
		}
	}

	for _, id := range taskIDs {
		updated := waitForTaskAttemptTerminal(t, rt, id)
		latest := updated.LatestAttempt()
		if latest == nil {
			t.Fatalf("expected latest attempt")
		}
		if strings.TrimSpace(latest.WorktreeRoot) == "" {
			t.Fatalf("expected worktree_root to be recorded on attempt")
		}
		if got := strings.TrimSpace(latest.WorktreeCleanupStatus); got != "cleaned" {
			t.Fatalf("expected worktree_cleanup_status=%q, got %q", "cleaned", got)
		}
		if _, statErr := os.Stat(latest.WorktreeRoot); statErr == nil {
			t.Fatalf("expected worktree_root to be cleaned up (ONEAGENT_KEEP_WORKTREES not set)")
		}
	}

	receipts, err := rt.WorkLedger.ListReceipts(workledger.ListReceiptsQuery{
		PrincipalID: userID,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("list receipts: %v", err)
	}
	if len(receipts) != len(taskIDs) {
		t.Fatalf("expected %d receipts, got %d", len(taskIDs), len(receipts))
	}
}

func TestTaskQueueRunner_WorktreeMode_OrphanSweeper_CleansRetainedWorktrees(t *testing.T) {
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

	t.Setenv("ONEAGENT_KEEP_WORKTREES", "1")

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
	if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	updated := waitForTaskAttemptTerminal(t, rt, created.ID)
	latest := updated.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected latest attempt")
	}
	if strings.TrimSpace(latest.WorktreeRoot) == "" {
		t.Fatalf("expected worktree_root")
	}
	if got := strings.TrimSpace(latest.WorktreeCleanupStatus); got != "retained" {
		t.Fatalf("expected worktree_cleanup_status=%q, got %q", "retained", got)
	}
	if _, err := os.Stat(latest.WorktreeRoot); err != nil {
		t.Fatalf("expected worktree_root to exist when retained: %v", err)
	}

	// Disable keep and run sweeper.
	os.Unsetenv("ONEAGENT_KEEP_WORKTREES")
	cleanupOrphanAttemptWorktrees(context.Background(), rt)

	if _, err := os.Stat(latest.WorktreeRoot); err == nil {
		t.Fatalf("expected retained worktree_root to be removed by sweeper")
	}
}
