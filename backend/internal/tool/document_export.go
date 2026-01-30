package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

const (
	ToolIDDocumentExport = "document_export"
)

type documentExportRequest struct {
	InputPath    string `json:"input_path"`
	Format       string `json:"format"` // docx | pptx
	OutputPath   string `json:"output_path,omitempty"`
	TemplatePath string `json:"template_path,omitempty"`
}

type documentExportResult struct {
	OK bool `json:"ok"`

	InputPath  string `json:"input_path,omitempty"`
	OutputPath string `json:"output_path,omitempty"`
	Format     string `json:"format,omitempty"`

	PandocCommand string `json:"pandoc_command,omitempty"`

	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

func documentExportDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "document_export",
			Description: "导出文档：将 workspace 内的 Markdown 导出为 Office（docx/pptx）。依赖 pandoc；输出路径必须在 workspace 内。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input_path": map[string]any{
						"type":        "string",
						"description": "输入 Markdown 文件路径（相对 workspace）。",
					},
					"format": map[string]any{
						"type":        "string",
						"description": "输出格式：docx | pptx。",
						"enum":        []string{"docx", "pptx"},
					},
					"output_path": map[string]any{
						"type":        "string",
						"description": "（可选）输出文件路径（相对 workspace）。默认与 input_path 同目录同名不同扩展。",
					},
					"template_path": map[string]any{
						"type":        "string",
						"description": "（可选）模板文件路径（相对 workspace）。docx 使用 reference-doc；pptx best-effort。",
					},
				},
				"required":             []string{"input_path", "format"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDDocumentExport, spec, runDocumentExportTool)
}

func runDocumentExportTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req documentExportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	req.InputPath = strings.TrimSpace(req.InputPath)
	req.Format = strings.ToLower(strings.TrimSpace(req.Format))
	req.OutputPath = strings.TrimSpace(req.OutputPath)
	req.TemplatePath = strings.TrimSpace(req.TemplatePath)

	if req.InputPath == "" {
		return nil, errors.New("input_path is required")
	}
	if req.Format != "docx" && req.Format != "pptx" {
		return nil, errors.New("format must be one of: docx, pptx")
	}

	dec, err := RequirePolicy(ctx, ToolIDDocumentExport)
	if err != nil {
		return nil, err
	}

	root, inputAbs, err := resolvePathForWrite(ctx, req.InputPath)
	if err != nil {
		return nil, err
	}
	if root != "" {
		if rel, err := filepath.Rel(root, inputAbs); err == nil {
			relSlash := filepath.ToSlash(rel)
			relSlash = strings.TrimPrefix(relSlash, "./")
			if err := EnforceFileScope(root, relSlash, dec); err != nil {
				return nil, err
			}
		}
	}
	if _, err := os.Stat(inputAbs); err != nil {
		return nil, err
	}

	outputRel := req.OutputPath
	if outputRel == "" {
		dir := filepath.Dir(req.InputPath)
		base := strings.TrimSuffix(filepath.Base(req.InputPath), filepath.Ext(req.InputPath))
		outputRel = filepath.Join(dir, base+"."+req.Format)
	}

	_, outputAbs, err := resolvePathForWrite(ctx, outputRel)
	if err != nil {
		return nil, err
	}
	if root != "" {
		if rel, err := filepath.Rel(root, outputAbs); err == nil {
			relSlash := filepath.ToSlash(rel)
			relSlash = strings.TrimPrefix(relSlash, "./")
			if err := EnforceFileScope(root, relSlash, dec); err != nil {
				return nil, err
			}
		}
	}

	templateAbs := ""
	if req.TemplatePath != "" {
		_, abs, err := resolvePathForWrite(ctx, req.TemplatePath)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			return nil, err
		}
		templateAbs = abs
	}

	if err := os.MkdirAll(filepath.Dir(outputAbs), 0o755); err != nil {
		return nil, err
	}

	pandocCmd := strings.TrimSpace(os.Getenv("ONEAGENT_PANDOC_CMD"))
	if pandocCmd == "" {
		pandocCmd = "pandoc"
	}
	prefixArgs := strings.Fields(strings.TrimSpace(os.Getenv("ONEAGENT_PANDOC_ARGS")))

	if !filepath.IsAbs(pandocCmd) {
		if _, err := exec.LookPath(pandocCmd); err != nil {
			return nil, fmt.Errorf("pandoc not found: %s (run `oneagent doctor` for diagnostics; install hint: %s)", pandocCmd, pandocInstallHint())
		}
	}

	args := make([]string, 0, 12)
	args = append(args, prefixArgs...)
	args = append(args, inputAbs, "-o", outputAbs)
	if templateAbs != "" {
		args = append(args, "--reference-doc="+templateAbs)
	}

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(runCtx, pandocCmd, args...)
	if strings.TrimSpace(root) != "" {
		cmd.Dir = root
	}
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		out := string(outBytes)
		if len(out) > 16*1024 {
			out = out[:16*1024] + "\n...(truncated)\n"
		}
		return nil, fmt.Errorf("pandoc failed: %v\n%s", err, strings.TrimSpace(out))
	}

	if _, err := os.Stat(outputAbs); err != nil {
		return nil, fmt.Errorf("export succeeded but output file missing: %v", err)
	}

	relOut := outputRel
	if root != "" && filepath.IsAbs(outputAbs) {
		if rel, err := filepath.Rel(root, outputAbs); err == nil {
			relOut = filepath.ToSlash(rel)
		}
	}
	relIn := req.InputPath
	if root != "" && filepath.IsAbs(inputAbs) {
		if rel, err := filepath.Rel(root, inputAbs); err == nil {
			relIn = filepath.ToSlash(rel)
		}
	}

	return documentExportResult{
		OK:           true,
		InputPath:     relIn,
		OutputPath:    relOut,
		Format:        req.Format,
		PandocCommand: pandocCmd,
		Stdout:        strings.TrimSpace(string(outBytes)),
	}, nil
}

func pandocInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install pandoc"
	case "linux":
		return "install pandoc via your package manager (apt/yum/pacman)"
	case "windows":
		return "choco install pandoc (or winget install Pandoc)"
	default:
		return "install pandoc"
	}
}
