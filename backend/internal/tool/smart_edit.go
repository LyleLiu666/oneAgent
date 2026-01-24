package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/sbe"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type SmartEditFileResult struct {
	FilePath     string `json:"file_path"`
	Replacements int    `json:"replacements,omitempty"`
	Bytes        int    `json:"bytes,omitempty"`
}

type SmartEditResult struct {
	FilePath     string                `json:"file_path,omitempty"`
	Replacements int                   `json:"replacements"`
	WrittenBytes int                   `json:"written_bytes,omitempty"`
	Files        []SmartEditFileResult `json:"files,omitempty"`
}

type editOperation struct {
	FilePath   string `json:"filePath"`
	OldString  string `json:"oldString"`
	NewString  string `json:"newString"`
	ReplaceAll bool   `json:"replaceAll,omitempty"`
}

type editToolRequest struct {
	Edits      []editOperation `json:"edits,omitempty"`
	ReplaceAll bool            `json:"replaceAll,omitempty"`
}

func smartEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "edit",
			Description: "对已有文件做“模糊替换”(fuzzy patch)。整文件写入/新建请用 write_file。强烈建议分段、小步、多次调用：单次 oldString/newString 建议 ≤3000 字、单次 edits ≤10、总字数建议 ≤12000，避免超出 LLM 最大 token 导致工具调用失败。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"edits": map[string]any{
						"type":        "array",
						"description": "要应用的编辑列表（建议每次只改 1-3 处，小步迭代）。",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"filePath": map[string]any{
									"type":        "string",
									"description": "要编辑的文件路径（相对沙箱根目录，或沙箱根目录内的绝对路径）。",
								},
								"oldString": map[string]any{
									"type":        "string",
									"description": "要搜索/匹配的原始片段（需要足够独特以定位；建议长度适中，过大会导致工具调用失败）。",
								},
								"newString": map[string]any{
									"type":        "string",
									"description": "替换后的片段（建议分段提交，避免一次性写入大段内容）。",
								},
								"replaceAll": map[string]any{
									"type":        "boolean",
									"description": "是否替换所有匹配（true=全部替换；false=仅替换第一个/最佳匹配）。",
								},
							},
							"required":             []string{"filePath", "oldString", "newString"},
							"additionalProperties": false,
						},
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "（可选）对本次 edits 全局生效的 replaceAll；true 时会覆盖每个 edit 的 replaceAll。",
					},
				},
				"required":             []string{"edits"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDEdit, spec, runSmartEditTool)
}

func runSmartEditTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req editToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	edits := req.Edits
	if len(edits) == 0 {
		var single editOperation
		if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single.FilePath) != "" {
			edits = []editOperation{single}
		}
	}
	if len(edits) == 0 {
		return nil, errors.New("edits 不能为空")
	}
	if len(edits) > maxEditOpsPerCall {
		return nil, fmt.Errorf("edits 数量过多（%d），请分段调用（每次最多 %d 条）", len(edits), maxEditOpsPerCall)
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	totalRunes := 0
	blocks := make([]sbe.EditBlock, 0, len(edits))
	for i, op := range edits {
		if strings.TrimSpace(op.FilePath) == "" {
			return nil, fmt.Errorf("edits[%d].filePath 不能为空", i)
		}
		if op.OldString == "" {
			return nil, fmt.Errorf("edits[%d].oldString 不能为空", i)
		}

		target, err := resolvePathWithinRoot(root, op.FilePath)
		if err != nil {
			return nil, fmt.Errorf("edits[%d]: %w", i, err)
		}

		// Ensure file exists
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在：%q。请先用 write_file 创建（或确认路径在沙箱根目录内）", op.FilePath)
		}

		oldRunes := runeCount(op.OldString)
		if oldRunes > maxEditSnippetRunes {
			return nil, fmt.Errorf("edits[%d].oldString 过长（%d 字），请分段（每段建议 <= %d 字）", i, oldRunes, maxEditSnippetRunes)
		}
		newRunes := runeCount(op.NewString)
		if newRunes > maxEditSnippetRunes {
			return nil, fmt.Errorf("edits[%d].newString 过长（%d 字），请分段（每段建议 <= %d 字）", i, newRunes, maxEditSnippetRunes)
		}
		totalRunes += oldRunes + newRunes

		replaceAll := op.ReplaceAll || req.ReplaceAll
		blocks = append(blocks, sbe.EditBlock{
			FilePath:   target,
			Search:     splitLines(op.OldString),
			Replace:    splitLines(op.NewString),
			ReplaceAll: replaceAll,
		})
	}
	if totalRunes > maxEditTotalRunesPerCall {
		return nil, fmt.Errorf("本次 edit 内容过长（%d 字），请分段多次调用（建议每次 <= %d 字）", totalRunes, maxEditTotalRunesPerCall)
	}

	replacementsByFile := make(map[string]int)
	totalReplacements := 0
	for _, block := range blocks {
		replacements, err := sbe.ApplyEditBlocks([]sbe.EditBlock{block})
		if err != nil {
			return nil, err
		}
		totalReplacements += replacements
		replacementsByFile[block.FilePath] += replacements
	}

	files := make([]SmartEditFileResult, 0, len(replacementsByFile))
	paths := make([]string, 0, len(replacementsByFile))
	for path := range replacementsByFile {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		files = append(files, SmartEditFileResult{
			FilePath:     path,
			Replacements: replacementsByFile[path],
		})
	}

	if len(files) == 1 {
		return SmartEditResult{
			FilePath:     files[0].FilePath,
			Replacements: files[0].Replacements,
			Files:        files,
		}, nil
	}

	return SmartEditResult{
		Replacements: totalReplacements,
		Files:        files,
	}, nil
}

// Helper to keep splitting consistent
func splitLines(value string) []string {
	return strings.Split(value, "\n")
}

func resolveSmartEditRoot() (string, error) {
	cfg := config.GetConfig()
	return shell.ResolveBashRoot(cfg.BashRootDir)
}

func resolvePathWithinRoot(root, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("file path is required")
	}

	var absPath string
	if filepath.IsAbs(trimmed) {
		absPath = filepath.Clean(trimmed)
	} else {
		absPath = filepath.Clean(filepath.Join(root, trimmed))
	}

	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", fmt.Errorf("resolve path error: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside sandbox root", path)
	}

	return absPath, nil
}
