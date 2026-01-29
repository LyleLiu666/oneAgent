package projectcfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/scope"
)

// ProjectConfig is loaded from <workspace>/.oneagent/project.json.
//
// All fields are optional.
type ProjectConfig struct {
	SetupScript     string   `json:"setup_script,omitempty"`
	TestScript      string   `json:"test_script,omitempty"`
	CleanupScript   string   `json:"cleanup_script,omitempty"`
	DevServerScript string   `json:"dev_server_script,omitempty"`
	CopyFiles       []string `json:"copy_files,omitempty"`
}

// Load discovers and parses <workspace>/.oneagent/project.json.
//
// found is true when the config file exists (even if parsing fails).
func Load(workspaceRoot string) (cfg ProjectConfig, found bool, err error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return ProjectConfig{}, false, errors.New("workspace_root is required")
	}

	path := filepath.Join(workspaceRoot, ".oneagent", "project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, false, nil
		}
		return ProjectConfig{}, false, fmt.Errorf("read project config %s: %w", path, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var parsed ProjectConfig
	if err := dec.Decode(&parsed); err != nil {
		return ProjectConfig{}, true, fmt.Errorf("parse project config %s: %w", path, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected extra JSON content")
		}
		return ProjectConfig{}, true, fmt.Errorf("parse project config %s: %w", path, err)
	}

	parsed.SetupScript = strings.TrimSpace(parsed.SetupScript)
	parsed.TestScript = strings.TrimSpace(parsed.TestScript)
	parsed.CleanupScript = strings.TrimSpace(parsed.CleanupScript)
	parsed.DevServerScript = strings.TrimSpace(parsed.DevServerScript)

	if len(parsed.CopyFiles) > 0 {
		normalized := make([]string, 0, len(parsed.CopyFiles))
		for i, raw := range parsed.CopyFiles {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return ProjectConfig{}, true, fmt.Errorf("parse project config %s: copy_files[%d] is empty", path, i)
			}
			if filepath.IsAbs(raw) {
				return ProjectConfig{}, true, fmt.Errorf("parse project config %s: copy_files[%d] must be relative (got absolute path): %s", path, i, raw)
			}
			cleaned := filepath.Clean(raw)
			if cleaned == "." || cleaned == string(filepath.Separator) {
				return ProjectConfig{}, true, fmt.Errorf("parse project config %s: copy_files[%d] is invalid: %s", path, i, raw)
			}
			abs, err := scope.ResolveWritePath(workspaceRoot, cleaned, nil)
			if err != nil {
				return ProjectConfig{}, true, fmt.Errorf("parse project config %s: copy_files[%d] is outside workspace: %s", path, i, raw)
			}
			rel, err := filepath.Rel(workspaceRoot, abs)
			if err != nil {
				return ProjectConfig{}, true, fmt.Errorf("parse project config %s: copy_files[%d] is invalid: %s", path, i, raw)
			}
			rel = filepath.ToSlash(rel)
			rel = strings.TrimPrefix(rel, "./")
			rel = strings.TrimPrefix(rel, "/")
			normalized = append(normalized, rel)
		}
		parsed.CopyFiles = normalized
	}

	return parsed, true, nil
}
