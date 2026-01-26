package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llmlog"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

type Runtime struct {
	Config *config.Config
	Layout *Layout

	AuthToken string

	Settings *settingsdb.DB
	Sessions *sessionstore.Store
	LLMLog   *llmlog.Writer
	Skills   *skill.Manager
}

func Init(cfg *config.Config) (*Runtime, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	layout, err := EnsureLayout(cfg.Home)
	if err != nil {
		return nil, err
	}

	// Best-effort log cleanup for all log buckets we manage.
	now := time.Now()
	_ = CleanupOldLogs(layout.LLMLogsDir, cfg.LogRetentionDays, now)
	_ = CleanupOldLogs(layout.TraceLogsDir, cfg.LogRetentionDays, now)
	_ = CleanupOldLogs(layout.SubagentLogsDir, cfg.LogRetentionDays, now)

	var token string
	if cfg.AuthMode == "token" {
		tok, created, err := EnsureAuthToken(layout.AuthTokenPath)
		if err != nil {
			return nil, err
		}
		token = tok
		if created {
			log.Printf("Generated local auth token at %s", layout.AuthTokenPath)
		}
	} else if cfg.AuthMode == "none" {
		log.Printf("WARNING: AUTH_MODE=none (authentication disabled). Use only in trusted environments.")
	}

	settings, err := settingsdb.Open(layout.SettingsDBPath)
	if err != nil {
		return nil, err
	}

	sessions, err := sessionstore.New(layout.SessionsDir)
	if err != nil {
		_ = settings.Close()
		return nil, err
	}

	llmLogger, err := llmlog.New(layout.LLMLogsDir, cfg.LogRetentionDays)
	if err != nil {
		_ = settings.Close()
		return nil, err
	}

	rt := &Runtime{
		Config:    cfg,
		Layout:    layout,
		AuthToken: token,
		Settings:  settings,
		Sessions:  sessions,
		LLMLog:    llmLogger,
		Skills:    skill.NewManager(30 * time.Second),
	}
	return rt, nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var firstErr error
	if r.Settings != nil {
		if err := r.Settings.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type HealthStatus struct {
	Status        string `json:"status"`
	SettingsDBOK  bool   `json:"settings_db"`
	DataDirOK     bool   `json:"data_dir"`
	LogsDirOK     bool   `json:"logs_dir"`
	AuthMode      string `json:"auth_mode"`
	OneAgentHome  string `json:"oneagent_home"`
	SettingsDB    string `json:"settings_db_path"`
}

func (r *Runtime) Health(ctx context.Context) (HealthStatus, error) {
	if r == nil {
		return HealthStatus{}, errors.New("runtime is nil")
	}
	status := HealthStatus{
		Status:       "healthy",
		AuthMode:     r.Config.AuthMode,
		OneAgentHome: r.Layout.Home,
		SettingsDB:   r.Layout.SettingsDBPath,
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := r.Settings.HealthCheck(ctx); err != nil {
		status.Status = "degraded"
		status.SettingsDBOK = false
	} else {
		status.SettingsDBOK = true
	}

	// Check directory writability by trying to create a temp file.
	status.DataDirOK = canWriteDir(r.Layout.DataDir)
	status.LogsDirOK = canWriteDir(r.Layout.LogsDir)
	if !status.DataDirOK || !status.LogsDirOK {
		status.Status = "degraded"
	}

	return status, nil
}

func canWriteDir(dir string) bool {
	if dir == "" {
		return false
	}
	path := fmt.Sprintf("%s/.writecheck-%d", dir, time.Now().UnixNano())
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(path)
	return true
}
