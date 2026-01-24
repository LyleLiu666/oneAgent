package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

func writeFileDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "write_file",
			Description: "写入文件内容（创建/覆盖或追加）。路径必须在沙箱根目录内。强烈建议分段、小步：单次 content 建议 ≤3000 字；超过上限会自动截断写入并在结果里标记 truncated/continue_append，后续用 append=true 继续追加。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "文件路径（相对沙箱根目录，或沙箱根目录内的绝对路径）。",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "要写入的内容（建议单次 ≤3000 字；超过上限会自动截断；大文件用 append=true 分段写入）。",
					},
					"append": map[string]any{
						"type":        "boolean",
						"description": "（可选）是否以追加方式写入。true=追加；false=覆盖（默认）。用于分段写入大文件。",
					},
				},
				"required":             []string{"filePath", "content"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDWriteFile, spec, runWriteFileTool)
}

type writeFileRequest struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
	Append   bool   `json:"append,omitempty"`
}

type writeFileResult struct {
	FilePath       string `json:"file_path"`
	WrittenBytes   int    `json:"written_bytes"`
	Truncated      bool   `json:"truncated,omitempty"`
	OriginalRunes  int    `json:"original_runes,omitempty"`
	WrittenRunes   int    `json:"written_runes,omitempty"`
	ContinueAppend bool   `json:"continue_append,omitempty"`
}

func runWriteFileTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req writeFileRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.FilePath == "" {
		return nil, fmt.Errorf("filePath 不能为空")
	}
	if runeCount(req.FilePath) > maxWriteFilePathRunesLimit {
		return nil, fmt.Errorf("filePath 过长，请缩短路径（建议 <= %d 字）", maxWriteFilePathRunesLimit)
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	target, err := resolvePathWithinRoot(root, req.FilePath)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	content := req.Content
	truncatedContent, truncated, originalRunes := truncateToRunes(content, maxWriteFileRunesPerCall)
	if truncated {
		if cut := strings.LastIndexByte(truncatedContent, '\n'); cut > 0 {
			truncatedContent = truncatedContent[:cut+1]
		}
		content = truncatedContent
	}
	writtenRunes := runeCount(content)
	contentBytes := []byte(content)
	if req.Append {
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("write file error: %w", err)
		}
		defer f.Close()
		if _, err := f.Write(contentBytes); err != nil {
			return nil, fmt.Errorf("write file error: %w", err)
		}
	} else {
		if err := os.WriteFile(target, contentBytes, 0o644); err != nil {
			return nil, fmt.Errorf("write file error: %w", err)
		}
	}

	result := writeFileResult{
		FilePath:     target,
		WrittenBytes: len(contentBytes),
	}
	if truncated {
		result.Truncated = true
		result.OriginalRunes = originalRunes
		result.WrittenRunes = writtenRunes
		result.ContinueAppend = true
	}
	return result, nil
}
