package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type globToolRequest struct {
	Pattern string `json:"pattern"`
}

type GlobToolResult struct {
	Root    string   `json:"root"`
	Pattern string   `json:"pattern"`
	Matches []string `json:"matches"`
}

func globDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "glob",
			Description: "List files matching a glob pattern within the sandbox root (relative to $BASH_ROOT_DIR unless absolute within it).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern to match (e.g. **/*.go).",
					},
				},
				"required":             []string{"pattern"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDGlob, spec, runGlobTool)
}

func runGlobTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req globToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return nil, errors.New("pattern is required")
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	matches, err := globMatches(root, pattern)
	if err != nil {
		return nil, err
	}

	return GlobToolResult{
		Root:    root,
		Pattern: pattern,
		Matches: matches,
	}, nil
}

func globMatches(root, pattern string) ([]string, error) {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return nil, errors.New("pattern is required")
	}

	if !hasGlobMeta(trimmed) {
		target, err := resolvePathWithinRoot(root, trimmed)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(target); err != nil {
			if os.IsNotExist(err) {
				return []string{}, nil
			}
			return nil, err
		}
		return []string{target}, nil
	}

	baseDir := globBaseDir(trimmed)
	if baseDir != "" {
		if _, err := resolvePathWithinRoot(root, baseDir); err != nil {
			return nil, err
		}
	}

	absPattern := trimmed
	if !filepath.IsAbs(trimmed) {
		absPattern = filepath.Join(root, trimmed)
	}
	relPattern, err := filepath.Rel(root, absPattern)
	if err != nil {
		return nil, fmt.Errorf("resolve pattern error: %w", err)
	}
	if relPattern == ".." || strings.HasPrefix(relPattern, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("pattern %q is outside sandbox root", pattern)
	}
	relPattern = filepath.ToSlash(relPattern)
	relPattern = strings.TrimPrefix(relPattern, "./")

	baseAbs := root
	if baseDir != "" {
		if filepath.IsAbs(baseDir) {
			baseAbs = baseDir
		} else {
			baseAbs = filepath.Join(root, baseDir)
		}
	}
	baseAbs = filepath.Clean(baseAbs)

	info, err := os.Stat(baseAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		baseAbs = filepath.Dir(baseAbs)
	}

	matches := []string{}
	err = filepath.WalkDir(baseAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == baseAbs {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimPrefix(rel, "./")
		matched, matchErr := matchDoubleStar(relPattern, rel)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(matches)
	return matches, nil
}

func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func globBaseDir(pattern string) string {
	metaIndex := strings.IndexAny(pattern, "*?[")
	if metaIndex == -1 {
		return pattern
	}
	prefix := pattern[:metaIndex]
	if prefix == "" {
		return ""
	}
	return filepath.Dir(prefix)
}

func matchDoubleStar(pattern, name string) (bool, error) {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	pattern = strings.Trim(pattern, "/")
	name = strings.Trim(name, "/")

	patSegs := splitSegments(pattern)
	nameSegs := splitSegments(name)
	return matchSegments(patSegs, nameSegs)
}

func splitSegments(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "/")
}

func matchSegments(pattern, name []string) (bool, error) {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			for len(pattern) > 1 && pattern[1] == "**" {
				pattern = pattern[1:]
			}
			if len(pattern) == 1 {
				return true, nil
			}
			for i := 0; i <= len(name); i++ {
				ok, err := matchSegments(pattern[1:], name[i:])
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
			}
			return false, nil
		}

		if len(name) == 0 {
			return false, nil
		}

		matched, err := path.Match(pattern[0], name[0])
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
		pattern = pattern[1:]
		name = name[1:]
	}
	return len(name) == 0, nil
}
