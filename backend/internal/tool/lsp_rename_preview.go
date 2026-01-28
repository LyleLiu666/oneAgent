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
	"github.com/liu_y/oneAgent/backend/internal/lsp"
)

const (
	ToolIDLSPRenamePreview = "lsp_rename_preview"
)

type lspRenamePreviewRequest struct {
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`      // 1-based
	Character int    `json:"character"` // 1-based
	NewName   string `json:"new_name"`
}

type lspTextEditPreview struct {
	FilePath     string `json:"file_path"`
	Line         int    `json:"line"`          // 1-based
	Character    int    `json:"character"`     // 1-based
	EndLine      int    `json:"end_line"`      // 1-based
	EndCharacter int    `json:"end_character"` // 1-based
	NewText      string `json:"new_text"`
}

type lspRenamePreviewResult struct {
	OK bool `json:"ok"`

	Language  string `json:"language"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
	NewName   string `json:"new_name"`

	Edits []lspTextEditPreview `json:"edits"`

	Truncated       bool   `json:"truncated,omitempty"`
	TruncatedReason string `json:"truncated_reason,omitempty"`
	Hint            string `json:"hint,omitempty"`

	DroppedOutOfWorkspace int `json:"dropped_out_of_workspace,omitempty"`
}

func lspRenamePreviewDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "lsp.rename_preview",
			Description: "语义级重命名预览：返回将修改的位置列表（只读，不写文件）。要求 workspace 已启用，file_path 必须在 workspace 内。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
					"line":      map[string]any{"type": "integer"},
					"character": map[string]any{"type": "integer"},
					"new_name": map[string]any{
						"type":        "string",
						"description": "新名字。",
					},
				},
				"required":             []string{"file_path", "line", "character", "new_name"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDLSPRenamePreview, spec, runLSPRenamePreviewTool)
}

type workspaceEdit struct {
	Changes         map[string][]lsp.TextEdit `json:"changes,omitempty"`
	DocumentChanges []struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Edits []lsp.TextEdit `json:"edits"`
	} `json:"documentChanges,omitempty"`
}

func runLSPRenamePreviewTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req lspRenamePreviewRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	workspace := WorkspaceFromContext(ctx)
	if !workspace.Enabled || strings.TrimSpace(workspace.Root) == "" {
		return nil, errors.New("workspace is required for lsp tools")
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	req.NewName = strings.TrimSpace(req.NewName)
	if req.FilePath == "" || req.NewName == "" {
		return nil, errors.New("file_path and new_name are required")
	}
	if req.Line <= 0 || req.Character <= 0 {
		return nil, errors.New("line and character must be 1-based positive integers")
	}

	if _, err := RequirePolicy(ctx, ToolIDLSPRenamePreview); err != nil {
		return nil, err
	}

	absPath, err := resolveLSPInputPath(workspace.Root, req.FilePath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, err
	}

	before, _ := os.ReadFile(absPath)

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
		"newName": req.NewName,
	}

	var rawResult json.RawMessage
	if err := sess.Client.Call(callCtx, "textDocument/rename", params, &rawResult); err != nil {
		return nil, err
	}
	var we workspaceEdit
	if err := json.Unmarshal(rawResult, &we); err != nil {
		return nil, err
	}

	edits := make([]lspTextEditPreview, 0, 64)
	dropped := 0

	addEdits := func(uri string, list []lsp.TextEdit) {
		p, err := lsp.FileURIToPath(uri)
		if err != nil {
			return
		}
		if !withinWorkspace(workspace.Root, p) {
			dropped++
			return
		}
		rel := relToWorkspace(workspace.Root, p)
		for _, e := range list {
			edits = append(edits, lspTextEditPreview{
				FilePath:     rel,
				Line:         e.Range.Start.Line + 1,
				Character:    e.Range.Start.Character + 1,
				EndLine:      e.Range.End.Line + 1,
				EndCharacter: e.Range.End.Character + 1,
				NewText:      e.NewText,
			})
		}
	}

	for uri, list := range we.Changes {
		addEdits(uri, list)
	}
	for _, dc := range we.DocumentChanges {
		addEdits(dc.TextDocument.URI, dc.Edits)
	}

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].FilePath != edits[j].FilePath {
			return edits[i].FilePath < edits[j].FilePath
		}
		if edits[i].Line != edits[j].Line {
			return edits[i].Line < edits[j].Line
		}
		return edits[i].Character < edits[j].Character
	})

	const maxEdits = 500
	res := lspRenamePreviewResult{
		OK:                    true,
		Language:              string(lang),
		FilePath:              relToWorkspace(workspace.Root, absPath),
		Line:                  req.Line,
		Character:             req.Character,
		NewName:               req.NewName,
		Edits:                 edits,
		DroppedOutOfWorkspace: dropped,
	}
	if len(res.Edits) > maxEdits {
		res.Edits = res.Edits[:maxEdits]
		res.Truncated = true
		res.TruncatedReason = "max_edits"
		res.Hint = "Too many edits; consider narrowing scope or renaming in smaller steps."
	}

	// Safety check: the tool MUST NOT modify files.
	after, _ := os.ReadFile(absPath)
	if string(before) != string(after) {
		return nil, errors.New("lsp.rename_preview modified workspace files; this is not allowed")
	}

	// Best-effort: normalize file path separators for downstream consumers.
	for i := range res.Edits {
		res.Edits[i].FilePath = filepath.ToSlash(res.Edits[i].FilePath)
	}

	return res, nil
}

