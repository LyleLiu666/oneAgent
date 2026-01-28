package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/lsp"
)

const (
	ToolIDLSPDefinition = "lsp_definition"
)

type lspDefinitionRequest struct {
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`      // 1-based
	Character int    `json:"character"` // 1-based
}

type lspDefinitionResult struct {
	OK bool `json:"ok"`

	Language string `json:"language"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Character int   `json:"character"`

	Locations []lspLocation `json:"locations"`

	DroppedOutOfWorkspace int `json:"dropped_out_of_workspace,omitempty"`
}

func lspDefinitionDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "lsp.definition",
			Description: "语义级导航：查找符号的定义位置（只读）。要求 workspace 已启用，file_path 必须在 workspace 内。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string", "description": "文件路径（相对或绝对，但必须在 workspace 内）。"},
					"line": map[string]any{
						"type":        "integer",
						"description": "光标行（1-based）。",
					},
					"character": map[string]any{
						"type":        "integer",
						"description": "光标列（1-based）。",
					},
				},
				"required":             []string{"file_path", "line", "character"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDLSPDefinition, spec, runLSPDefinitionTool)
}

func runLSPDefinitionTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req lspDefinitionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	workspace := WorkspaceFromContext(ctx)
	if !workspace.Enabled || strings.TrimSpace(workspace.Root) == "" {
		return nil, errors.New("workspace is required for lsp tools")
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.FilePath == "" {
		return nil, errors.New("file_path is required")
	}
	if req.Line <= 0 || req.Character <= 0 {
		return nil, errors.New("line and character must be 1-based positive integers")
	}

	dec, err := RequirePolicy(ctx, ToolIDLSPDefinition)
	if err != nil {
		return nil, err
	}
	_ = dec // read-only today; reserved for future constraints.

	absPath, err := resolveLSPInputPath(workspace.Root, req.FilePath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, err
	}

	lang, err := lspLanguageForPath(absPath)
	if err != nil {
		return nil, err
	}
	server := lspServerForLanguage(lang)
	if err := ensureServerAvailable(server); err != nil {
		return nil, err
	}

	callCtx, cancel := withToolTimeout(ctx)
	defer cancel()

	sess, err := lsp.StartSession(callCtx, server.Command, server.Args, workspace.Root, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sess.Close(context.Background()) }()

	if err := lsp.Initialize(callCtx, sess.Client, workspace.Root); err != nil {
		return nil, err
	}

	uri := lsp.PathToFileURI(absPath)
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position": map[string]any{
			"line":      req.Line - 1,
			"character": req.Character - 1,
		},
	}

	var rawResult json.RawMessage
	if err := sess.Client.Call(callCtx, "textDocument/definition", params, &rawResult); err != nil {
		return nil, err
	}
	locs, err := parseLocations(rawResult)
	if err != nil {
		return nil, err
	}

	out := make([]lspLocation, 0, len(locs))
	dropped := 0
	for _, loc := range locs {
		p, err := lsp.FileURIToPath(loc.URI)
		if err != nil {
			continue
		}
		if !withinWorkspace(workspace.Root, p) {
			dropped++
			continue
		}
		out = append(out, lspLocation{
			FilePath:     relToWorkspace(workspace.Root, p),
			Line:         loc.Range.Start.Line + 1,
			Character:    loc.Range.Start.Character + 1,
			EndLine:      loc.Range.End.Line + 1,
			EndCharacter: loc.Range.End.Character + 1,
		})
	}
	sortLocations(out)

	return lspDefinitionResult{
		OK:                   true,
		Language:             string(lang),
		FilePath:             relToWorkspace(workspace.Root, absPath),
		Line:                 req.Line,
		Character:            req.Character,
		Locations:            out,
		DroppedOutOfWorkspace: dropped,
	}, nil
}
