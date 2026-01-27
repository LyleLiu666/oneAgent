package workledger

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// DigestExists reports whether a digest markdown file exists for the given principal/dayKey.
// It is intentionally read-only (no generation/refresh side effects).
func (s *Store) DigestExists(principalID, dayKey string) (bool, error) {
	if s == nil {
		return false, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	dayKey = strings.TrimSpace(dayKey)
	if principalID == "" || dayKey == "" {
		return false, errors.New("principal_id and day_key are required")
	}

	path := s.DigestPath(principalID, dayKey)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.Size() > 0, nil
}

type CountSuggestionsQuery struct {
	PrincipalID    string
	Status         SuggestionStatus
	IncludeParked  bool
	WorkspaceRoot  string
}

func (s *Store) CountSuggestions(q CountSuggestionsQuery) (int, error) {
	if s == nil {
		return 0, errors.New("store is nil")
	}
	if strings.TrimSpace(q.PrincipalID) == "" {
		return 0, errors.New("principal_id is required")
	}

	root := s.SuggestionsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	wantStatus := strings.TrimSpace(string(q.Status))
	wantWS := strings.TrimSpace(q.WorkspaceRoot)
	n := 0

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		data, err := os.ReadFile(s.suggestionJSONPath(id))
		if err != nil {
			continue
		}
		var sug Suggestion
		if err := json.Unmarshal(data, &sug); err != nil {
			continue
		}
		if strings.TrimSpace(sug.PrincipalID) != strings.TrimSpace(q.PrincipalID) {
			continue
		}
		if wantWS != "" && strings.TrimSpace(sug.WorkspaceRoot) != wantWS {
			continue
		}
		if !q.IncludeParked && sug.Status == SuggestionStatusParked {
			continue
		}
		if wantStatus != "" && strings.TrimSpace(string(sug.Status)) != wantStatus {
			continue
		}
		n++
	}

	return n, nil
}

