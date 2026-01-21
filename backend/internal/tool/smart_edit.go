package tool

import (
	"context"
	"encoding/json"
	"errors"
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

type SmartEditResult struct {
	FilePath     string `json:"file_path"`
	Replacements int    `json:"replacements"`
}

func smartEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "smart_edit",
			Description: "Apply a targeted edit to a file by replacing oldString with newString.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "Path to the file to edit.",
					},
					"oldString": map[string]any{
						"type":        "string",
						"description": "Exact text to search for (can be multi-line).",
					},
					"newString": map[string]any{
						"type":        "string",
						"description": "Replacement text (can be multi-line, empty to delete).",
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "Replace all matches instead of only the first.",
					},
				},
				"required":             []string{"filePath", "oldString", "newString"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDSmartEdit, spec, runSmartEditTool)
}

func runSmartEditTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	var req smartEditToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.FilePath) == "" {
		return nil, errors.New("filePath is required")
	}
	if req.OldString == "" {
		return nil, errors.New("oldString is required")
	}

	block := sbe.EditBlock{
		FilePath:   req.FilePath,
		Search:     splitLines(req.OldString),
		Replace:    splitLines(req.NewString),
		ReplaceAll: req.ReplaceAll,
	}

	replacements, err := sbe.ApplyEditBlocks([]sbe.EditBlock{block})
	if err != nil {
		return nil, err
	}

	return SmartEditResult{
		FilePath:     req.FilePath,
		Replacements: replacements,
	}, nil
}

func splitLines(value string) []string {
	return strings.Split(value, "\n")
}
