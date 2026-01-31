package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/trash"
)

type trashFileRequest struct {
	FilePath string `json:"filePath"`
}

type trashFileResult struct {
	OK bool `json:"ok"`

	TrashID string `json:"trash_id"`

	OriginalPath string `json:"original_path"`
	TrashedPath  string `json:"trashed_path"`
	ManifestPath string `json:"manifest_path"`

	IsDir bool `json:"is_dir"`
}

func trashFileDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "trash_file",
			Description: "软删除（move-to-trash）：将 workspace 内的文件/目录移动到 `<workspace>/.oneagent/trash/` 下（可人工恢复）。回收站条目会在保留期后被自动清理（best-effort）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "要软删除的文件/目录路径（相对 workspace 根目录，或 workspace 内的绝对路径）。",
					},
				},
				"required":             []string{"filePath"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDTrashFile, spec, runTrashFileTool)
}

func runTrashFileTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req trashFileRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.FilePath == "" {
		return nil, fmt.Errorf("filePath 不能为空")
	}

	dec, err := RequirePolicy(ctx, ToolIDTrashFile)
	if err != nil {
		return nil, err
	}
	if runeCount(req.FilePath) > maxWriteFilePathRunesLimit {
		return nil, fmt.Errorf("filePath 过长，请缩短路径（建议 <= %d 字）", maxWriteFilePathRunesLimit)
	}

	root, target, err := resolvePathForWrite(ctx, req.FilePath)
	if err != nil {
		return nil, err
	}
	if root == "" {
		return nil, errors.New("workspace 未设置")
	}

	if rel, err := filepath.Rel(root, target); err == nil {
		relSlash := filepath.ToSlash(rel)
		relSlash = strings.TrimPrefix(relSlash, "./")
		if err := EnforceFileScope(root, relSlash, dec); err != nil {
			return nil, err
		}
	}

	info, err := os.Lstat(target)
	if err != nil {
		return nil, err
	}
	isDir := info.IsDir()

	trashID := uuid.NewString()
	entryDir := trash.EntryDir(root, trashID)
	payloadPath := trash.PayloadPath(root, trashID)

	if err := os.MkdirAll(entryDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create trash entry directory: %w", err)
	}

	if err := os.Rename(target, payloadPath); err != nil {
		// Best-effort rollback: remove the entry dir if move fails.
		_ = os.RemoveAll(entryDir)
		return nil, err
	}

	manifest := trash.Manifest{
		TrashID:      trashID,
		CreatedAtUTC: time.Now().UTC().Format(time.RFC3339Nano),
		OriginalPath: target,
		TrashedPath:  payloadPath,
		IsDir:        isDir,
	}
	manifestPath, err := trash.WriteManifest(root, trashID, manifest)
	if err != nil {
		// Move succeeded; keep the payload but return a clear error.
		return nil, fmt.Errorf("trashed but failed to write manifest: %w", err)
	}

	// Opportunistic cleanup (best-effort).
	_, _ = trash.Cleanup(ctx, root, trash.DefaultRetention, time.Now())

	return trashFileResult{
		OK: true,

		TrashID: trashID,

		OriginalPath: target,
		TrashedPath:  payloadPath,
		ManifestPath: manifestPath,

		IsDir: isDir,
	}, nil
}
