package llmlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type Writer struct {
	baseDir       string
	retentionDays int
}

type CallRecord struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Model     string    `json:"model"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`

	Request any `json:"request"`

	Response any            `json:"response,omitempty"`
	Error    string         `json:"error,omitempty"`
	Usage    *llm.UsageInfo `json:"usage,omitempty"`

	PromptCacheEnabled bool   `json:"prompt_cache_enabled"`
	PromptCacheKeyHash string `json:"prompt_cache_key_hash,omitempty"`
	PromptCacheEpoch   int    `json:"prompt_cache_epoch,omitempty"`

	PromptCacheDowngraded      bool   `json:"prompt_cache_downgraded,omitempty"`
	PromptCacheDowngradeReason string `json:"prompt_cache_downgrade_reason,omitempty"`
}

func New(baseDir string, retentionDays int) (*Writer, error) {
	if baseDir == "" {
		return nil, errors.New("baseDir is required")
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create llm log dir: %w", err)
	}
	return &Writer{baseDir: baseDir, retentionDays: retentionDays}, nil
}

func (w *Writer) PathForCall(now time.Time, sessionID, callID string) string {
	date := now.Format("2006-01-02")
	return filepath.Join(w.baseDir, date, sessionID, callID+".json")
}

func (w *Writer) WriteCall(path string, rec CallRecord) error {
	if path == "" {
		return errors.New("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create call log dir: %w", err)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
