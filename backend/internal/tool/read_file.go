package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

func readFileDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "read_file",
			Description: "读取文件内容（支持按行分页与输出限流）。当传入相对路径时，要求 workspace 已启用且路径在 workspace 内（防 .. 与 symlink 越界）；绝对路径允许只读读取。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "文件路径（workspace 启用时允许相对路径；绝对路径始终只读允许）。",
					},
					"offset_lines": map[string]any{
						"type":        "integer",
						"description": "（可选）从第几行开始读取（0-based）。默认 0。",
					},
					"limit_lines": map[string]any{
						"type":        "integer",
						"description": "（可选）最多读取多少行。默认 200，最大 2000。",
					},
					"max_bytes": map[string]any{
						"type":        "integer",
						"description": "（可选）输出内容最大字节数（UTF-8 安全截断）。默认 65536，最大 1048576。",
					},
				},
				"required":             []string{"filePath"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDReadFile, spec, runReadFileTool)
}

type readFileRequest struct {
	FilePath    string `json:"filePath"`
	OffsetLines int    `json:"offset_lines,omitempty"`
	LimitLines  int    `json:"limit_lines,omitempty"`
	MaxBytes    int    `json:"max_bytes,omitempty"`
}

type readFileResult struct {
	OK bool `json:"ok"`

	FilePath string `json:"file_path"`

	OffsetLines int `json:"offset_lines"`
	LimitLines  int `json:"limit_lines"`
	MaxBytes    int `json:"max_bytes"`

	StartLine int `json:"start_line"` // 1-based; 0 when empty
	EndLine   int `json:"end_line"`   // 1-based; 0 when empty

	Content string `json:"content"`

	Truncated bool `json:"truncated,omitempty"`

	TotalBytes int64 `json:"total_bytes,omitempty"`
	TotalLines int64 `json:"total_lines,omitempty"`
}

func runReadFileTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req readFileRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.FilePath == "" {
		return nil, fmt.Errorf("filePath 不能为空")
	}

	dec, err := RequirePolicy(ctx, ToolIDReadFile)
	if err != nil {
		return nil, err
	}
	if IsReadOutsideWorkspaceDenied(dec) && filepath.IsAbs(req.FilePath) {
		return nil, fmt.Errorf("read outside workspace is denied by policy")
	}
	if runeCount(req.FilePath) > maxWriteFilePathRunesLimit {
		return nil, fmt.Errorf("filePath 过长，请缩短路径（建议 <= %d 字）", maxWriteFilePathRunesLimit)
	}

	offset := req.OffsetLines
	if offset < 0 {
		offset = 0
	}
	limit := req.LimitLines
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	if maxBytes > 1024*1024 {
		maxBytes = 1024 * 1024
	}

	root := ""
	if r, err := resolveWorkspaceRootBestEffort(ctx); err == nil {
		root = r
	} else if !errors.Is(err, scope.ErrWorkspaceNotSet) {
		return nil, err
	}

	target, err := scope.ResolveReadPath(root, req.FilePath)
	if err != nil {
		if errors.Is(err, scope.ErrWorkspaceNotSet) {
			return nil, fmt.Errorf("workspace 未设置：相对路径不可用，请启用 workspace 或使用绝对路径")
		}
		if errors.Is(err, scope.ErrPathOutsideWorkspace) {
			return nil, fmt.Errorf("path is outside workspace")
		}
		return nil, err
	}

	f, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var totalBytes int64
	sizeKnown := false
	if st, err := f.Stat(); err == nil {
		totalBytes = st.Size()
		sizeKnown = true
	}

	r := bufio.NewReaderSize(f, 64*1024)

	// Skip offset lines.
	for skipped := 0; skipped < offset; skipped++ {
		_, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return readFileResult{
					OK: true,

					FilePath: target,

					OffsetLines: offset,
					LimitLines:  limit,
					MaxBytes:    maxBytes,

					StartLine: 0,
					EndLine:   0,

					Content: "",
				}, nil
			}
			return nil, err
		}
	}

	var b strings.Builder
	linesRead := 0
	writtenBytes := 0
	truncated := false

	for linesRead < limit && writtenBytes < maxBytes {
		line, err := r.ReadString('\n')
		if line != "" {
			remaining := maxBytes - writtenBytes
			if len(line) <= remaining {
				b.WriteString(line)
				writtenBytes += len(line)
				linesRead++
			} else {
				prefix := truncateUTF8ToBytes(line, remaining)
				if prefix != "" {
					b.WriteString(prefix)
					writtenBytes += len(prefix)
					linesRead++
				}
				truncated = true
				break
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}

	content := b.String()
	startLine := 0
	endLine := 0
	if linesRead > 0 {
		startLine = offset + 1
		endLine = offset + linesRead
	}

	// If we exactly hit maxBytes, treat as truncated unless we are at EOF.
	if !truncated && writtenBytes >= maxBytes {
		if _, err := r.Peek(1); err == nil {
			truncated = true
		}
	}

	// Record OCC fingerprint for small files to prevent read->write drift.
	// We use the file's full content sha256 (independent of pagination/truncation).
	const maxOCCTrackedFileBytes = 8 * 1024 * 1024
	if OCCFromContext(ctx) != nil && sizeKnown && totalBytes <= maxOCCTrackedFileBytes {
		if sha, err := fileSHA256Hex(target); err == nil {
			occRecordSHA256(ctx, target, sha)
		}
	}

	return readFileResult{
		OK: true,

		FilePath: target,

		OffsetLines: offset,
		LimitLines:  limit,
		MaxBytes:    maxBytes,

		StartLine: startLine,
		EndLine:   endLine,

		Content: content,

		Truncated: truncated,

		TotalBytes: totalBytes,
		TotalLines: -1,
	}, nil
}

func resolveWorkspaceRootBestEffort(ctx context.Context) (string, error) {
	cfg := config.GetConfig()
	if cfg.BashRootDirExplicit && strings.TrimSpace(cfg.BashRootDir) != "" {
		// Keep consistent with other tools: explicit bash root acts as an implicit workspace root.
		return resolveWorkspaceRoot(ctx)
	}
	ws := WorkspaceFromContext(ctx)
	if ws.Enabled && strings.TrimSpace(ws.Root) != "" {
		return scope.NormalizeWorkspaceRoot(ws.Root)
	}
	return "", scope.ErrWorkspaceNotSet
}

func truncateUTF8ToBytes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.ValidString(s[:cut]) {
		cut--
	}
	if cut <= 0 {
		return ""
	}
	return s[:cut]
}
