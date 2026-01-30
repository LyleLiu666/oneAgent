package taskqueue

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type QueueGlobalPolicy struct {
	// MaxRunningWorkspaces limits the number of distinct workspaces that can be running tasks concurrently.
	// 0 means unlimited (default behavior).
	MaxRunningWorkspaces int `json:"max_running_workspaces,omitempty"`
}

type WorkspacePolicy struct {
	Paused   bool `json:"paused,omitempty"`
	Priority int  `json:"priority,omitempty"`
}

type Schedule struct {
	ID string `json:"id"`

	Enabled bool `json:"enabled"`

	UserID    string `json:"user_id,omitempty"`
	Workspace string `json:"workspace"`
	Title     string `json:"title,omitempty"`
	Prompt    string `json:"prompt"`
	ModelID   string `json:"model_id,omitempty"`
	Limits    Limits `json:"limits,omitempty"`

	EverySeconds int       `json:"every_seconds"`
	NextRunAt    time.Time `json:"next_run_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type QueueGovernance struct {
	Global     QueueGlobalPolicy            `json:"global"`
	Workspaces map[string]WorkspacePolicy   `json:"workspaces,omitempty"`
	Schedules  []Schedule                   `json:"schedules,omitempty"`
	UpdatedAt  time.Time                    `json:"updated_at"`
}

func defaultGovernance() QueueGovernance {
	return QueueGovernance{
		Global:     QueueGlobalPolicy{MaxRunningWorkspaces: 0},
		Workspaces: map[string]WorkspacePolicy{},
		Schedules:  nil,
		UpdatedAt:  time.Time{},
	}
}

func (s *Store) governancePath() string {
	return filepath.Join(s.tasksDir, "governance.json")
}

func (s *Store) GetGovernance() (QueueGovernance, error) {
	if s == nil {
		return QueueGovernance{}, errors.New("store is nil")
	}
	s.govMu.Lock()
	defer s.govMu.Unlock()

	return s.getGovernanceLocked()
}

func (s *Store) UpdateGovernance(fn func(g *QueueGovernance) error) (QueueGovernance, error) {
	if s == nil {
		return QueueGovernance{}, errors.New("store is nil")
	}
	if fn == nil {
		return QueueGovernance{}, errors.New("fn is nil")
	}

	s.govMu.Lock()
	defer s.govMu.Unlock()

	g, err := s.getGovernanceLocked()
	if err != nil {
		return QueueGovernance{}, err
	}

	if g.Workspaces == nil {
		g.Workspaces = map[string]WorkspacePolicy{}
	}

	if err := fn(&g); err != nil {
		return QueueGovernance{}, err
	}
	g.UpdatedAt = time.Now().UTC()

	if err := writeJSONAtomic(s.governancePath(), g, 0o644); err != nil {
		return QueueGovernance{}, err
	}

	s.govCache = g
	s.govCacheAt = time.Now()
	return g, nil
}

func (s *Store) getGovernanceLocked() (QueueGovernance, error) {
	// Simple cache to avoid reading the JSON on every scheduler tick.
	if !s.govCacheAt.IsZero() && time.Since(s.govCacheAt) < 2*time.Second {
		return s.govCache, nil
	}

	path := s.governancePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			g := defaultGovernance()
			s.govCache = g
			s.govCacheAt = time.Now()
			return g, nil
		}
		return QueueGovernance{}, err
	}

	var g QueueGovernance
	if err := json.Unmarshal(data, &g); err != nil {
		// If governance file is corrupted, fail closed to default behavior (best-effort).
		g = defaultGovernance()
	}
	if g.Workspaces == nil {
		g.Workspaces = map[string]WorkspacePolicy{}
	}
	s.govCache = g
	s.govCacheAt = time.Now()
	return g, nil
}

func (s *Store) TakeDueSchedules(now time.Time) ([]Schedule, error) {
	now = now.UTC()
	var due []Schedule
	_, err := s.UpdateGovernance(func(g *QueueGovernance) error {
		if len(g.Schedules) == 0 {
			return nil
		}

		for i := range g.Schedules {
			sc := g.Schedules[i]
			if !sc.Enabled {
				continue
			}
			if strings.TrimSpace(sc.Workspace) == "" || strings.TrimSpace(sc.Prompt) == "" {
				continue
			}
			if sc.EverySeconds <= 0 {
				continue
			}

			next := sc.NextRunAt
			if next.IsZero() || !now.Before(next.UTC()) {
				due = append(due, sc)
				sc.NextRunAt = now.Add(time.Duration(sc.EverySeconds) * time.Second)
				sc.UpdatedAt = now
				g.Schedules[i] = sc
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return due, nil
}
