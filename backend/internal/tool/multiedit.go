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

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/sbe"
)

type multiEditToolRequest struct {
	Edits      []smartEditToolRequest `json:"edits"`
	ReplaceAll bool                   `json:"replaceAll,omitempty"`
}

func multiEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "multiedit",
			Description: "Apply multiple fuzzy edits across files. Provide edits as an array of {filePath, oldString, newString, replaceAll?}.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"edits": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"filePath": map[string]any{
									"type": "string",
								},
								"oldString": map[string]any{
									"type": "string",
								},
								"newString": map[string]any{
									"type": "string",
								},
								"replaceAll": map[string]any{
									"type": "boolean",
								},
							},
							"required":             []string{"filePath", "oldString", "newString"},
							"additionalProperties": false,
						},
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "If true, replace all matches for each edit block.",
					},
				},
				"required":             []string{"edits"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDMultiEdit, spec, runMultiEditTool)
}

func runMultiEditTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req multiEditToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	if len(req.Edits) == 0 {
		return nil, errors.New("edits is required")
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	blocks := make([]sbe.EditBlock, 0, len(req.Edits))
	for i, edit := range req.Edits {
		if strings.TrimSpace(edit.FilePath) == "" {
			return nil, fmt.Errorf("edits[%d].filePath is required", i)
		}
		if edit.OldString == "" {
			return nil, fmt.Errorf("edits[%d].oldString is required", i)
		}
		block := sbe.EditBlock{
			FilePath:   edit.FilePath,
			Search:     splitLines(edit.OldString),
			Replace:    splitLines(edit.NewString),
			ReplaceAll: edit.ReplaceAll,
		}
		if req.ReplaceAll {
			block.ReplaceAll = true
		}
		blocks = append(blocks, block)
	}

	replacementsByFile := make(map[string]int)
	total := 0
	for _, block := range blocks {
		target, err := resolvePathWithinRoot(root, block.FilePath)
		if err != nil {
			return nil, err
		}
		block.FilePath = target

		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}
			if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
				return nil, fmt.Errorf("failed to create new file: %w", err)
			}
		}

		replacements, err := sbe.ApplyEditBlocks([]sbe.EditBlock{block})
		if err != nil {
			return nil, err
		}
		total += replacements
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
		Replacements: total,
		Files:        files,
	}, nil
}
