package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type lsToolRequest struct {
	Path string `json:"path,omitempty"`
}

type LsEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type LsToolResult struct {
	Root    string    `json:"root"`
	Path    string    `json:"path"`
	Entries []LsEntry `json:"entries"`
}

func lsDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "ls",
			Description: "List directory entries within the sandbox root (defaults to '.').",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Directory or file path (relative to $BASH_ROOT_DIR unless absolute within it).",
					},
				},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDLs, spec, runLsTool)
}

func runLsTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req lsToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	pathValue := strings.TrimSpace(req.Path)
	if pathValue == "" {
		pathValue = "."
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	target, err := resolvePathWithinRoot(root, pathValue)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("path does not exist")
		}
		return nil, err
	}

	entries := []LsEntry{}
	if !info.IsDir() {
		entries = append(entries, LsEntry{
			Name:  filepath.Base(target),
			Path:  target,
			IsDir: false,
		})
		return LsToolResult{
			Root:    root,
			Path:    target,
			Entries: entries,
		}, nil
	}

	dirEntries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	for _, entry := range dirEntries {
		entries = append(entries, LsEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(target, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return LsToolResult{
		Root:    root,
		Path:    target,
		Entries: entries,
	}, nil
}
