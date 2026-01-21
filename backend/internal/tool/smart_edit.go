package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/sbe"
)

type smartEditToolRequest struct {
	FilePath   string `json:"filePath"`
	OldString  string `json:"oldString"`
	NewString  string `json:"newString"`
	ReplaceAll bool   `json:"replaceAll,omitempty"`
}

type smartEditCommandRequest struct {
	Command    json.RawMessage `json:"command"`
	ReplaceAll bool            `json:"replaceAll,omitempty"`
}

type SmartEditFileResult struct {
	FilePath     string `json:"file_path"`
	Replacements int    `json:"replacements"`
}

type SmartEditResult struct {
	FilePath     string                `json:"file_path,omitempty"`
	Replacements int                   `json:"replacements"`
	Files        []SmartEditFileResult `json:"files,omitempty"`
}

func smartEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "smart_edit",
			Description: "Apply one or more fuzzy edits using a shell-style smart-edit script (recommended) or legacy {filePath, oldString, newString}.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "array",
						"description": "Smart-edit script lines. Example: [\"apply_smart_edit <<'EOF'\", \"file: path\", \"<<<< SEARCH\", \"...\", \"==== REPLACE\", \"...\", \">>>>\", \"EOF\"].",
						"items": map[string]any{
							"type": "string",
						},
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "If true, replace all matches for each edit block.",
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDSmartEdit, spec, runSmartEditTool)
}

func runSmartEditTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	blocks, replaceAll, err := parseSmartEditBlocks(raw)
	if err != nil {
		return nil, err
	}
	if replaceAll {
		for i := range blocks {
			blocks[i].ReplaceAll = true
		}
	}

	replacementsByFile := make(map[string]int)
	total := 0
	for _, block := range blocks {
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
		}, nil
	}

	return SmartEditResult{
		Replacements: total,
		Files:        files,
	}, nil
}

func splitLines(value string) []string {
	return strings.Split(value, "\n")
}

func parseSmartEditBlocks(raw json.RawMessage) ([]sbe.EditBlock, bool, error) {
	if len(raw) == 0 {
		return nil, false, errors.New("missing tool arguments")
	}

	var commandReq smartEditCommandRequest
	if err := json.Unmarshal(raw, &commandReq); err == nil && len(commandReq.Command) > 0 {
		command, err := decodeSmartEditCommand(commandReq.Command)
		if err != nil {
			return nil, false, err
		}
		blocks, err := sbe.ParseSmartEditCommand(command)
		if err != nil {
			return nil, false, err
		}
		return blocks, commandReq.ReplaceAll, nil
	}

	var legacy smartEditToolRequest
	if err := json.Unmarshal(raw, &legacy); err == nil {
		if strings.TrimSpace(legacy.FilePath) == "" {
			return nil, false, errors.New("filePath is required")
		}
		if legacy.OldString == "" {
			return nil, false, errors.New("oldString is required")
		}
		block := sbe.EditBlock{
			FilePath:   legacy.FilePath,
			Search:     splitLines(legacy.OldString),
			Replace:    splitLines(legacy.NewString),
			ReplaceAll: legacy.ReplaceAll,
		}
		return []sbe.EditBlock{block}, false, nil
	}

	var command string
	if err := json.Unmarshal(raw, &command); err == nil {
		blocks, err := sbe.ParseSmartEditCommand(command)
		if err != nil {
			return nil, false, err
		}
		return blocks, false, nil
	}

	// Last resort: treat the raw bytes as a plain script (lenient input).
	blocks, err := sbe.ParseSmartEditCommand(string(raw))
	if err != nil {
		return nil, false, fmt.Errorf("invalid smart_edit arguments: %w", err)
	}
	return blocks, false, nil
}

func decodeSmartEditCommand(raw json.RawMessage) (string, error) {
	var lines []string
	if err := json.Unmarshal(raw, &lines); err == nil && len(lines) > 0 {
		return strings.Join(lines, "\n"), nil
	}

	var command string
	if err := json.Unmarshal(raw, &command); err == nil && strings.TrimSpace(command) != "" {
		return command, nil
	}

	return "", errors.New("command must be a non-empty string or string array")
}
