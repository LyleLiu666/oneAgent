package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestTaskQueueRunner_ProjectScripts_SetupFailureWritesLogs(t *testing.T) {
	home := t.TempDir()
	layout, err := runtime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("ensure layout: %v", err)
	}

	settings, err := settingsdb.Open(layout.SettingsDBPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = settings.Close() })

	ctx := context.Background()
	userID := "local"

	_, err = settings.CreateProvider(ctx, settingsdb.Provider{
		ID:           "p1",
		UserID:       userID,
		Name:         "test",
		ProviderType: "openai",
		BaseURL:      "http://example.invalid",
		APIKey:       "test",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = settings.CreateModel(ctx, settingsdb.Model{
		ID:            "m1",
		ProviderID:    "p1",
		UserID:        userID,
		Name:          "test",
		Model:         "gpt-4o-mini",
		IsDefault:     true,
		EnableKVCache: true,
		Options:       map[string]any{},
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	rt := &runtime.Runtime{
		Layout:   layout,
		Settings: settings,
	}
	if err := ensureTaskQueue(rt); err != nil {
		t.Fatalf("ensure task queue: %v", err)
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".oneagent"), 0o700); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".oneagent", "project.json"), []byte(`{"setup_script":"ls does-not-exist"}`), 0o600); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	task := taskqueue.Task{
		ID:        "t1",
		UserID:    userID,
		Workspace: workspace,
		Prompt:    "noop",
	}
	attempt := taskqueue.Attempt{ID: "a1"}

	res, execErr := rt.TaskRunner.ExecuteAttempt(ctx, task, attempt, nil)
	if execErr == nil {
		t.Fatalf("expected error")
	}

	if strings.TrimSpace(res.ProjectConfigPath) == "" {
		t.Fatalf("expected project_config_path")
	}
	if strings.TrimSpace(res.SetupScriptLogPath) == "" {
		t.Fatalf("expected setup_script_log_path")
	}
	if strings.TrimSpace(res.CopyFilesLogPath) == "" {
		t.Fatalf("expected copy_files_log_path")
	}

	if _, err := os.Stat(res.SetupScriptLogPath); err != nil {
		t.Fatalf("setup log missing: %v", err)
	}
	data, err := os.ReadFile(res.SetupScriptLogPath)
	if err != nil {
		t.Fatalf("read setup log: %v", err)
	}
	if !strings.Contains(string(data), "# setup_script") {
		t.Fatalf("expected setup log header, got: %q", string(data))
	}
}

func TestTaskQueueRunner_ProjectScripts_CopyFilesMissingFailsAttempt(t *testing.T) {
	home := t.TempDir()
	layout, err := runtime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("ensure layout: %v", err)
	}

	settings, err := settingsdb.Open(layout.SettingsDBPath)
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = settings.Close() })

	ctx := context.Background()
	userID := "local"

	_, err = settings.CreateProvider(ctx, settingsdb.Provider{
		ID:           "p1",
		UserID:       userID,
		Name:         "test",
		ProviderType: "openai",
		BaseURL:      "http://example.invalid",
		APIKey:       "test",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = settings.CreateModel(ctx, settingsdb.Model{
		ID:            "m1",
		ProviderID:    "p1",
		UserID:        userID,
		Name:          "test",
		Model:         "gpt-4o-mini",
		IsDefault:     true,
		EnableKVCache: true,
		Options:       map[string]any{},
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	rt := &runtime.Runtime{
		Layout:   layout,
		Settings: settings,
	}
	if err := ensureTaskQueue(rt); err != nil {
		t.Fatalf("ensure task queue: %v", err)
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".oneagent"), 0o700); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".oneagent", "project.json"), []byte(`{"copy_files":[".env"],"setup_script":"ls ."}`), 0o600); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	task := taskqueue.Task{
		ID:        "t1",
		UserID:    userID,
		Workspace: workspace,
		Prompt:    "noop",
	}
	attempt := taskqueue.Attempt{ID: "a1"}

	res, execErr := rt.TaskRunner.ExecuteAttempt(ctx, task, attempt, nil)
	if execErr == nil {
		t.Fatalf("expected error")
	}
	if strings.TrimSpace(res.CopyFilesLogPath) == "" {
		t.Fatalf("expected copy_files_log_path")
	}
	if _, err := os.Stat(res.CopyFilesLogPath); err != nil {
		t.Fatalf("copy_files log missing: %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Summary), "copy_files") {
		t.Fatalf("expected summary to mention copy_files, got: %q", res.Summary)
	}
}

