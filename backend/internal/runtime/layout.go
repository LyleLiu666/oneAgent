package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Layout struct {
	Home string

	OneAgentDir string

	ConfigDir string
	DataDir   string
	LogsDir   string
	TmpDir    string

	SettingsDBPath string
	AuthTokenPath  string

	SessionsDir string
	TasksDir    string

	TraceLogsDir   string
	LLMLogsDir     string
	SubagentLogsDir string
}

func EnsureLayout(home string) (*Layout, error) {
	if home == "" {
		return nil, errors.New("home is required")
	}

	oneagentDir := filepath.Join(home, ".oneagent")
	layout := &Layout{
		Home:          home,
		OneAgentDir:   oneagentDir,
		ConfigDir:     filepath.Join(oneagentDir, "config"),
		DataDir:       filepath.Join(oneagentDir, "data"),
		LogsDir:       filepath.Join(oneagentDir, "logs"),
		TmpDir:        filepath.Join(oneagentDir, "tmp"),
		SettingsDBPath: filepath.Join(oneagentDir, "settings.db"),
		AuthTokenPath:  filepath.Join(oneagentDir, "config", "auth_token"),
		SessionsDir:    filepath.Join(oneagentDir, "data", "sessions"),
		TasksDir:       filepath.Join(oneagentDir, "data", "tasks"),
		TraceLogsDir:   filepath.Join(oneagentDir, "logs", "trace"),
		LLMLogsDir:     filepath.Join(oneagentDir, "logs", "llm"),
		SubagentLogsDir: filepath.Join(oneagentDir, "logs", "subagent"),
	}

	dirs := []struct {
		path string
		mode os.FileMode
	}{
		{path: home, mode: 0o700},
		{path: oneagentDir, mode: 0o700},
		{path: layout.ConfigDir, mode: 0o700},
		{path: layout.DataDir, mode: 0o700},
		{path: layout.LogsDir, mode: 0o700},
		{path: layout.TmpDir, mode: 0o700},
		{path: layout.SessionsDir, mode: 0o700},
		{path: layout.TasksDir, mode: 0o700},
		{path: layout.TraceLogsDir, mode: 0o700},
		{path: layout.LLMLogsDir, mode: 0o700},
		{path: layout.SubagentLogsDir, mode: 0o700},
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d.path, err)
		}
	}

	return layout, nil
}

// CleanupOldLogs removes dated log directories older than retentionDays.
// It is best-effort and never returns an error for individual deletion failures.
func CleanupOldLogs(baseDir string, retentionDays int, now time.Time) error {
	if baseDir == "" {
		return errors.New("baseDir is required")
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		parsed, err := time.Parse("2006-01-02", name)
		if err != nil {
			continue
		}
		if parsed.Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(baseDir, name))
		}
	}

	return nil
}
