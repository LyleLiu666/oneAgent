package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
	"github.com/liu_y/oneAgent/backend/internal/llmlog"
	"github.com/liu_y/oneAgent/backend/internal/memorydb"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/workflow"
	"github.com/liu_y/oneAgent/backend/internal/workledger"

	"github.com/google/uuid"
)

type Runtime struct {
	Config *config.Config
	Layout *Layout

	AuthToken string

	Pairing *PairingService

	Settings     *settingsdb.DB
	Memory       *memorydb.DB
	Sessions     *sessionstore.Store
	LLMLog       *llmlog.Writer
	Skills       *skill.Manager
	FormalMemory *formalmemory.Service

	Tasks      *taskqueue.Store
	TaskRunner *taskqueue.TaskRunner

	WorkLedger *workledger.Store
	Workflows  *workflow.Store

	bgCtx    context.Context
	bgCancel context.CancelFunc
	bgWG     sync.WaitGroup
	bgMu     sync.Mutex
	bgOnce   map[string]*sync.Once

	secretaryRetryMu sync.RWMutex
	secretaryRetryFn func(context.Context, string)
}

func Init(cfg *config.Config) (*Runtime, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	// Ensure packages that rely on global config (e.g., skill discovery) see the same config instance.
	config.AppConfig = cfg

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

	memory, err := memorydb.Open(layout.MemoryDBPath)
	if err != nil {
		_ = settings.Close()
		return nil, err
	}

	sessions, err := sessionstore.New(layout.SessionsDir)
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		return nil, err
	}

	llmLogger, err := llmlog.New(layout.LLMLogsDir, cfg.LogRetentionDays)
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		return nil, err
	}

	formalMemory, err := formalmemory.Open(context.Background(), formalmemory.Config{
		PostgresDSN:     cfg.MemorySDKPostgresDSN,
		PreRecallPolicy: cfg.MemorySDKPreRecallPolicy,
	})
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		return nil, err
	}

	tasks, err := taskqueue.NewStore(layout.TasksDir)
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		_ = formalMemory.Close()
		return nil, err
	}

	ledger, err := workledger.NewStore(layout.LedgerDir)
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		_ = formalMemory.Close()
		return nil, err
	}

	workflows, err := workflow.NewStore(layout.WorkflowsDir)
	if err != nil {
		_ = settings.Close()
		_ = memory.Close()
		_ = formalMemory.Close()
		return nil, err
	}

	// Optional workflow artifacts root override (best-effort).
	if strings.TrimSpace(cfg.WorkflowArtifactsRoot) != "" {
		root := strings.TrimSpace(cfg.WorkflowArtifactsRoot)
		if err := os.MkdirAll(root, 0o700); err == nil {
			if s, err := workflow.NewStore(root); err == nil {
				workflows = s
			}
		}
	}

	rt := &Runtime{
		Config:       cfg,
		Layout:       layout,
		AuthToken:    token,
		Pairing:      NewPairingService(),
		Settings:     settings,
		Memory:       memory,
		Sessions:     sessions,
		LLMLog:       llmLogger,
		Skills:       skill.NewManager(30 * time.Second),
		FormalMemory: formalMemory,
		Tasks:        tasks,
		WorkLedger:   ledger,
		Workflows:    workflows,
	}
	rt.bgCtx, rt.bgCancel = context.WithCancel(context.Background())

	// Best-effort workflow run cleanup.
	_ = workflows.CleanupOldRuns(context.Background(), cfg.WorkflowArtifactsRetentionDays, now)
	return rt, nil
}

// Go runs a background goroutine tied to the runtime lifecycle.
// It is canceled when Runtime.Close is called.
func (r *Runtime) Go(fn func(ctx context.Context)) {
	if r == nil || fn == nil {
		return
	}
	r.bgWG.Add(1)
	go func() {
		defer r.bgWG.Done()
		fn(r.bgCtx)
	}()
}

// GoOnce starts a background goroutine only once per runtime instance.
func (r *Runtime) GoOnce(fn func(ctx context.Context)) {
	r.GoOnceKey("default", fn)
}

func (r *Runtime) GoOnceKey(key string, fn func(ctx context.Context)) {
	if r == nil || fn == nil {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "default"
	}

	r.bgMu.Lock()
	if r.bgOnce == nil {
		r.bgOnce = map[string]*sync.Once{}
	}
	once, ok := r.bgOnce[key]
	if !ok {
		once = &sync.Once{}
		r.bgOnce[key] = once
	}
	r.bgMu.Unlock()

	once.Do(func() { r.Go(fn) })
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.bgCancel != nil {
		r.bgCancel()
	}
	if r.TaskRunner != nil {
		r.TaskRunner.Stop()
	}
	r.bgWG.Wait()
	var firstErr error
	if r.Settings != nil {
		if err := r.Settings.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.Memory != nil {
		if err := r.Memory.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.FormalMemory != nil {
		if err := r.FormalMemory.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Runtime) SetSecretaryAutoTriageRetry(fn func(context.Context, string)) {
	if r == nil {
		return
	}
	r.secretaryRetryMu.Lock()
	r.secretaryRetryFn = fn
	r.secretaryRetryMu.Unlock()
}

func (r *Runtime) RetryPendingSecretaryAutoTriage(ctx context.Context, userID string) {
	if r == nil {
		return
	}
	r.secretaryRetryMu.RLock()
	fn := r.secretaryRetryFn
	r.secretaryRetryMu.RUnlock()
	if fn == nil {
		return
	}
	fn(ctx, userID)
}

type HealthStatus struct {
	Status                      string `json:"status"`
	SettingsDBOK                bool   `json:"settings_db"`
	MemoryDBOK                  bool   `json:"memory_db"`
	DataDirOK                   bool   `json:"data_dir"`
	LogsDirOK                   bool   `json:"logs_dir"`
	AuthMode                    string `json:"auth_mode"`
	OneAgentHome                string `json:"oneagent_home"`
	SettingsDB                  string `json:"settings_db_path"`
	MemoryDB                    string `json:"memory_db_path"`
	FormalMemoryEnabled         bool   `json:"formal_memory_enabled"`
	FormalMemoryConnected       bool   `json:"formal_memory_connected"`
	MemorySDKPreRecallPolicy    string `json:"memorysdk_prerecall_policy"`
	MemorySDKToolsEnabled       bool   `json:"memorysdk_enable_tools"`
	MemorySDKTurnEndJobsEnabled bool   `json:"memorysdk_enable_turn_end_jobs"`
}

func (r *Runtime) Health(ctx context.Context) (HealthStatus, error) {
	if r == nil {
		return HealthStatus{}, errors.New("runtime is nil")
	}
	status := HealthStatus{
		Status:                      "healthy",
		AuthMode:                    r.Config.AuthMode,
		OneAgentHome:                r.Layout.Home,
		SettingsDB:                  r.Layout.SettingsDBPath,
		MemoryDB:                    r.Layout.MemoryDBPath,
		FormalMemoryEnabled:         strings.TrimSpace(r.Config.MemorySDKPostgresDSN) != "",
		FormalMemoryConnected:       r.FormalMemory != nil,
		MemorySDKPreRecallPolicy:    effectiveMemorySDKPreRecallPolicy(r.Config),
		MemorySDKToolsEnabled:       r.Config.MemorySDKEnableTools,
		MemorySDKTurnEndJobsEnabled: r.Config.MemorySDKEnableTurnEndJobs,
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := r.Settings.HealthCheck(ctx); err != nil {
		status.Status = "degraded"
		status.SettingsDBOK = false
	} else {
		status.SettingsDBOK = true
	}

	if r.Memory == nil || r.Memory.HealthCheck(ctx) != nil {
		status.Status = "degraded"
		status.MemoryDBOK = false
	} else {
		status.MemoryDBOK = true
	}

	// Check directory writability by trying to create a temp file.
	status.DataDirOK = canWriteDir(r.Layout.DataDir)
	status.LogsDirOK = canWriteDir(r.Layout.LogsDir)
	if !status.DataDirOK || !status.LogsDirOK {
		status.Status = "degraded"
	}
	if status.FormalMemoryEnabled && !status.FormalMemoryConnected {
		status.Status = "degraded"
	}

	return status, nil
}

const settingsKeySecretarySessionID = "secretary_session_id"

// ResolveSecretarySessionID returns the canonical secretary session id for the given principal.
// It is persisted to Settings and MUST remain stable across reloads and restarts (best-effort).
func (r *Runtime) ResolveSecretarySessionID(ctx context.Context, principalID string) (string, error) {
	if r == nil || r.Settings == nil {
		return "", errors.New("runtime settings not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		principalID = "local"
	}

	val, err := r.Settings.GetUserSetting(ctx, principalID, settingsKeySecretarySessionID)
	if err == nil && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	id := uuid.NewString()
	if err := r.Settings.SetUserSetting(ctx, principalID, settingsKeySecretarySessionID, id); err != nil {
		return "", err
	}
	return id, nil
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

func effectiveMemorySDKPreRecallPolicy(cfg *config.Config) string {
	if cfg == nil {
		return "auto"
	}
	policy := strings.ToLower(strings.TrimSpace(cfg.MemorySDKPreRecallPolicy))
	if policy == "" {
		return "auto"
	}
	return policy
}
