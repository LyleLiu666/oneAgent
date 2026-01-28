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
	ToolIDLSPReferences = "lsp_references"
)

type lspReferencesRequest struct {
	FilePath           string `json:"file_path"`
	Line               int    `json:"line"`      // 1-based
	Character          int    `json:"character"` // 1-based
	IncludeDeclaration bool   `json:"include_declaration,omitempty"`
}

type lspReferencesResult struct {
	OK bool `json:"ok"`

	Language  string `json:"language"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`

	References []lspLocation `json:"references"`

	Truncated       bool   `json:"truncated,omitempty"`
	TruncatedReason string `json:"truncated_reason,omitempty"`
	Hint            string `json:"hint,omitempty"`

	DroppedOutOfWorkspace int `json:"dropped_out_of_workspace,omitempty"`
}

func lspReferencesDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "lsp.references",
			Description: "语义级导航：查找符号的引用位置（只读）。结果会按固定上限截断并给出 refine 提示。要求 workspace 已启用，file_path 必须在 workspace 内。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
					"line":      map[string]any{"type": "integer"},
					"character": map[string]any{"type": "integer"},
					"include_declaration": map[string]any{
						"type":        "boolean",
						"description": "是否包含定义位置（默认 false）。",
					},
				},
				"required":             []string{"file_path", "line", "character"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDLSPReferences, spec, runLSPReferencesTool)
}

func runLSPReferencesTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req lspReferencesRequest
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

	if _, err := RequirePolicy(ctx, ToolIDLSPReferences); err != nil {
		return nil, err
	}

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
		"context": map[string]any{"includeDeclaration": req.IncludeDeclaration},
	}

	var rawResult json.RawMessage
	if err := sess.Client.Call(callCtx, "textDocument/references", params, &rawResult); err != nil {
		return nil, err
	}
	var locs []lsp.Location
	if err := json.Unmarshal(rawResult, &locs); err != nil {
		return nil, err
	}

	const maxRefs = 200
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

	res := lspReferencesResult{
		OK:                   true,
		Language:             string(lang),
		FilePath:             relToWorkspace(workspace.Root, absPath),
		Line:                 req.Line,
		Character:            req.Character,
		References:           out,
		DroppedOutOfWorkspace: dropped,
	}

	if len(res.References) > maxRefs {
		res.References = res.References[:maxRefs]
		res.Truncated = true
		res.TruncatedReason = "max_results"
		res.Hint = "Too many references; refine by narrowing scope (e.g., limit to current file or a smaller folder)."
	}

	return res, nil
}

