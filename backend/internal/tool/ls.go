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
			Description: "列出沙箱根目录内的目录/文件信息（默认 path='.'）。路径相对 $BASH_ROOT_DIR，或为其内部的绝对路径。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "目录或文件路径（相对 $BASH_ROOT_DIR，或为其内部的绝对路径）。",
					},
				},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDLs, spec, runLsTool)
}

func runLsTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req lsToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	pathValue := strings.TrimSpace(req.Path)
	if pathValue == "" {
		pathValue = "."
	}

	root, target, err := resolvePathForRead(ctx, pathValue)
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
