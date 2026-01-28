package tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestLSPTools_DefinitionReferencesRenamePreview(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	workspaceRoot := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: workspaceRoot})

	// Prepare a file inside workspace.
	targetPath := filepath.Join(workspaceRoot, "main.go")
	if err := os.WriteFile(targetPath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	// Point Go LSP command to the test helper server.
	t.Setenv("ONEAGENT_LSP_GO_CMD", os.Args[0])
	t.Setenv("ONEAGENT_LSP_GO_ARGS", "-test.run=TestHelperLSPServer")
	t.Setenv("ONEAGENT_LSP_TEST_SERVER", "1")

	// lsp.definition
	defRaw, _ := json.Marshal(map[string]any{"file_path": "main.go", "line": 3, "character": 6})
	gotAny, err := runLSPDefinitionTool(ctx, defRaw)
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	def, ok := gotAny.(lspDefinitionResult)
	if !ok || !def.OK {
		t.Fatalf("definition unexpected: %#v (%T)", gotAny, gotAny)
	}
	if len(def.Locations) == 0 {
		t.Fatalf("expected at least one location")
	}
	if def.DroppedOutOfWorkspace == 0 {
		t.Fatalf("expected dropped_out_of_workspace > 0 (server returns one out-of-workspace loc)")
	}

	// lsp.references should be capped.
	refRaw, _ := json.Marshal(map[string]any{"file_path": "main.go", "line": 3, "character": 6})
	gotAny, err = runLSPReferencesTool(ctx, refRaw)
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	refs, ok := gotAny.(lspReferencesResult)
	if !ok || !refs.OK {
		t.Fatalf("references unexpected: %#v (%T)", gotAny, gotAny)
	}
	if !refs.Truncated || refs.TruncatedReason == "" {
		t.Fatalf("expected truncated references with reason, got truncated=%v reason=%q len=%d", refs.Truncated, refs.TruncatedReason, len(refs.References))
	}
	if len(refs.References) != 200 {
		t.Fatalf("expected capped references=200, got %d", len(refs.References))
	}

	// lsp.rename_preview MUST NOT modify the file.
	before, _ := os.ReadFile(targetPath)
	renameRaw, _ := json.Marshal(map[string]any{"file_path": "main.go", "line": 3, "character": 6, "new_name": "main2"})
	gotAny, err = runLSPRenamePreviewTool(ctx, renameRaw)
	if err != nil {
		t.Fatalf("rename_preview: %v", err)
	}
	ren, ok := gotAny.(lspRenamePreviewResult)
	if !ok || !ren.OK {
		t.Fatalf("rename_preview unexpected: %#v (%T)", gotAny, gotAny)
	}
	after, _ := os.ReadFile(targetPath)
	if string(before) != string(after) {
		t.Fatalf("expected file unchanged by rename_preview")
	}
	if len(ren.Edits) == 0 {
		t.Fatalf("expected at least one edit preview")
	}
}

// TestHelperLSPServer runs as a subprocess LSP stdio server for tests.
// It is invoked by exec'ing the current test binary with -test.run=TestHelperLSPServer.
func TestHelperLSPServer(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ONEAGENT_LSP_TEST_SERVER")) != "1" {
		return
	}

	// Act as an LSP server over stdio. Do not write anything except JSON-RPC frames.
	rootURI := ""
	br := bufio.NewReader(os.Stdin)
	for {
		msg, err := readLSPFrame(br)
		if err != nil {
			return
		}
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int            `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			var p struct {
				RootURI string `json:"rootUri"`
			}
			_ = json.Unmarshal(req.Params, &p)
			rootURI = p.RootURI
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, map[string]any{
				"capabilities": map[string]any{
					"definitionProvider": true,
					"referencesProvider": true,
					"renameProvider":     true,
				},
				})
			}
		case "initialized":
			// notification
		case "shutdown":
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, nil)
			}
		case "exit":
			return
		case "textDocument/definition":
			// Return one location inside workspace and one outside to test filtering.
			in := rootURI + "/def.go"
			out := "file:///tmp/outside.go"
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, []map[string]any{
					{"uri": in, "range": mkRange(1, 1, 1, 4)},
					{"uri": out, "range": mkRange(1, 1, 1, 4)},
				})
			}
		case "textDocument/references":
			// Return 250 references inside workspace to trigger cap=200.
			locs := make([]map[string]any, 0, 250)
			for i := 0; i < 250; i++ {
				uri := fmt.Sprintf("%s/%d.go", rootURI, i)
				locs = append(locs, map[string]any{"uri": uri, "range": mkRange(0, 0, 0, 1)})
			}
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, locs)
			}
		case "textDocument/rename":
			// Return a workspace edit (changes) that would rename in two files.
			edit := map[string]any{
				"changes": map[string]any{
					rootURI + "/main.go": []map[string]any{
						{"range": mkRange(2, 0, 2, 4), "newText": "main2"},
					},
					rootURI + "/other.go": []map[string]any{
						{"range": mkRange(1, 0, 1, 4), "newText": "main2"},
					},
				},
			}
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, edit)
			}
		default:
			// Unknown request: return null.
			if req.ID != nil {
				writeLSPResponse(os.Stdout, *req.ID, nil)
			}
		}
	}
}

func mkRange(sl, sc, el, ec int) map[string]any {
	return map[string]any{
		"start": map[string]any{"line": sl, "character": sc},
		"end":   map[string]any{"line": el, "character": ec},
	}
}

func readLSPFrame(br *bufio.Reader) ([]byte, error) {
	var contentLength int
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[0])) != "content-length" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
		contentLength = n
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(br, buf); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf), nil
}

func writeLSPResponse(w io.Writer, id int, result any) {
	resp := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data))
	_, _ = w.Write(data)
}
