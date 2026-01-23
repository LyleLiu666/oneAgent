package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/sbe"
	"github.com/liu_y/oneAgent/backend/internal/shell"
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
	Replacements int    `json:"replacements,omitempty"`
	Bytes        int    `json:"bytes,omitempty"`
}

type SmartEditResult struct {
	FilePath     string                `json:"file_path,omitempty"`
	Replacements int                   `json:"replacements"`
	WrittenBytes int                   `json:"written_bytes,omitempty"`
	Files        []SmartEditFileResult `json:"files,omitempty"`
}

func smartEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "edit",
			Description: "Apply edits via a shell-style script. Supports apply_edit blocks for fuzzy replace, or `cat >path <<'EOF' ... EOF` blocks to write a full file. Prefer {filePath, content} for full-file writes when possible. Legacy {filePath, oldString, newString} is also supported. 一次新增或替换的字符串长度不可超过3000字，小步迭代，分批提交，禁止一次性提交过多字数。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"description": "edit script, as an array of lines or a single multi-line string. Example: [\"apply_edit <<'EOF'\", \"file: path\", \"<<<< SEARCH\", \"...\", \"==== REPLACE\", \"...\", \">>>>\", \"EOF\"].",
						"oneOf": []any{
							map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
							},
							map[string]any{
								"type": "string",
							},
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

	return newDefinition(ToolIDEdit, spec, runSmartEditTool)
}

func runSmartEditTool(ctx context.Context, raw json.RawMessage) (any, error) {
	_ = ctx

	blocks, replaceAll, fileWrites, err := parseSmartEditInput(raw)
	if err != nil {
		return nil, err
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	if len(fileWrites) > 0 {
		writtenByFile := make(map[string]int)
		totalBytes := 0
		for _, write := range fileWrites {
			target, err := resolvePathWithinRoot(root, write.FilePath)
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}
			contentBytes := []byte(write.Content)
			if err := os.WriteFile(target, contentBytes, 0644); err != nil {
				return nil, fmt.Errorf("write file error: %w", err)
			}
			writtenByFile[target] = len(contentBytes)
			totalBytes += len(contentBytes)
		}

		files := make([]SmartEditFileResult, 0, len(writtenByFile))
		paths := make([]string, 0, len(writtenByFile))
		for path := range writtenByFile {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			files = append(files, SmartEditFileResult{
				FilePath: path,
				Bytes:    writtenByFile[path],
			})
		}

		if len(files) == 1 {
			return SmartEditResult{
				FilePath:     files[0].FilePath,
				WrittenBytes: files[0].Bytes,
				Files:        files,
			}, nil
		}

		return SmartEditResult{
			WrittenBytes: totalBytes,
			Files:        files,
		}, nil
	}

	if replaceAll {
		for i := range blocks {
			blocks[i].ReplaceAll = true
		}
	}

	replacementsByFile := make(map[string]int)
	total := 0
	for _, block := range blocks {
		target, err := resolvePathWithinRoot(root, block.FilePath)
		if err != nil {
			return nil, err
		}
		block.FilePath = target

		// Check if file exists, if not create it
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}
			if err := os.WriteFile(target, []byte{}, 0644); err != nil {
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

func splitLines(value string) []string {
	return strings.Split(value, "\n")
}

type smartEditFileWrite struct {
	FilePath string
	Content  string
}

func parseSmartEditInput(raw json.RawMessage) ([]sbe.EditBlock, bool, []smartEditFileWrite, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, nil, errors.New("missing tool arguments")
	}

	raw = normalizeConcatenatedJSON(raw)

	var commandReq smartEditCommandRequest
	if err := json.Unmarshal(raw, &commandReq); err == nil && len(commandReq.Command) > 0 {
		lines, err := decodeCommandLines(commandReq.Command)
		if err != nil {
			return nil, false, nil, err
		}

		joined := strings.Join(lines, "\n")
		blocks, parseErr := sbe.ParseSmartEditCommand(joined)
		if parseErr == nil {
			return blocks, commandReq.ReplaceAll, nil, nil
		}

		fileWrites, writeErr := parseCatHeredocWrites(lines)
		if writeErr == nil {
			return nil, commandReq.ReplaceAll, fileWrites, nil
		}

		return nil, false, nil, fmt.Errorf("invalid smart_edit command: %w (cat heredoc parse: %v)", parseErr, writeErr)
	}

	var legacy smartEditToolRequest
	if err := json.Unmarshal(raw, &legacy); err == nil {
		if strings.TrimSpace(legacy.FilePath) == "" {
			return nil, false, nil, errors.New("filePath is required")
		}
		if legacy.OldString == "" {
			return nil, false, nil, errors.New("oldString is required")
		}
		block := sbe.EditBlock{
			FilePath:   legacy.FilePath,
			Search:     splitLines(legacy.OldString),
			Replace:    splitLines(legacy.NewString),
			ReplaceAll: legacy.ReplaceAll,
		}
		return []sbe.EditBlock{block}, false, nil, nil
	}

	var command string
	if err := json.Unmarshal(raw, &command); err == nil {
		command = strings.TrimSpace(command)
		if command == "" {
			return nil, false, nil, errors.New("command is required")
		}

		trimmed := strings.TrimSpace(command)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && json.Valid([]byte(trimmed)) {
			var decoded []string
			if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil && len(decoded) > 0 {
				joined := strings.Join(decoded, "\n")
				blocks, parseErr := sbe.ParseSmartEditCommand(joined)
				if parseErr == nil {
					return blocks, false, nil, nil
				}
				fileWrites, writeErr := parseCatHeredocWrites(decoded)
				if writeErr == nil {
					return nil, false, fileWrites, nil
				}
				return nil, false, nil, fmt.Errorf("invalid smart_edit command: %w (cat heredoc parse: %v)", parseErr, writeErr)
			}
		}

		lines := strings.Split(command, "\n")
		blocks, parseErr := sbe.ParseSmartEditCommand(command)
		if parseErr == nil {
			return blocks, false, nil, nil
		}
		fileWrites, writeErr := parseCatHeredocWrites(lines)
		if writeErr == nil {
			return nil, false, fileWrites, nil
		}
		return nil, false, nil, fmt.Errorf("invalid smart_edit command: %w (cat heredoc parse: %v)", parseErr, writeErr)
	}

	// Last resort: treat the raw bytes as a plain script (lenient input).
	script := strings.TrimSpace(string(raw))
	if script == "" {
		return nil, false, nil, errors.New("command is required")
	}
	lines := strings.Split(script, "\n")
	blocks, parseErr := sbe.ParseSmartEditCommand(script)
	if parseErr == nil {
		return blocks, false, nil, nil
	}
	fileWrites, writeErr := parseCatHeredocWrites(lines)
	if writeErr == nil {
		return nil, false, fileWrites, nil
	}
	return nil, false, nil, fmt.Errorf("invalid smart_edit arguments: %w (cat heredoc parse: %v)", parseErr, writeErr)
}

func decodeCommandLines(raw json.RawMessage) ([]string, error) {
	var lines []string
	if err := json.Unmarshal(raw, &lines); err == nil && len(lines) > 0 {
		return lines, nil
	}

	var command string
	if err := json.Unmarshal(raw, &command); err == nil && strings.TrimSpace(command) != "" {
		command = strings.TrimRight(command, "\n")
		trimmed := strings.TrimSpace(command)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && json.Valid([]byte(trimmed)) {
			var decoded []string
			if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil && len(decoded) > 0 {
				return decoded, nil
			}
		}
		return strings.Split(command, "\n"), nil
	}

	return nil, errors.New("command must be a non-empty string or string array")
}

func normalizeConcatenatedJSON(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || json.Valid(trimmed) {
		return trimmed
	}

	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] != '}' {
			continue
		}
		j := i + 1
		for j < len(trimmed) && (trimmed[j] == ' ' || trimmed[j] == '\n' || trimmed[j] == '\r' || trimmed[j] == '\t') {
			j++
		}
		if j >= len(trimmed) || trimmed[j] != '{' {
			continue
		}
		candidate := bytes.TrimSpace(trimmed[j:])
		if json.Valid(candidate) {
			return candidate
		}
	}

	return trimmed
}

var catHeredocHeader = regexp.MustCompile(`^cat\s*>\s*(?P<path>(?:'[^']+'|"[^"]+"|[^\s]+))\s*<<\s*(?P<marker>(?:'[^']+'|"[^"]+"|[^\s]+))\s*$`)

func parseCatHeredocWrites(lines []string) ([]smartEditFileWrite, error) {
	var writes []smartEditFileWrite

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		matches := catHeredocHeader.FindStringSubmatch(line)
		if matches == nil {
			return nil, fmt.Errorf("unsupported command: %s", line)
		}

		path := stripOptionalQuotes(matches[catHeredocHeader.SubexpIndex("path")])
		marker := stripOptionalQuotes(matches[catHeredocHeader.SubexpIndex("marker")])
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("cat heredoc missing file path")
		}
		if strings.TrimSpace(marker) == "" {
			return nil, errors.New("cat heredoc missing EOF marker")
		}

		i++
		var contentLines []string
		for i < len(lines) {
			rawLine := lines[i]
			trimmedLine := strings.TrimSpace(rawLine)
			if trimmedLine == marker || stripOptionalQuotes(trimmedLine) == marker {
				break
			}
			contentLines = append(contentLines, rawLine)
			i++
		}
		if i < len(lines) {
			i++ // consume marker line
		} // else: leniently accept EOF as end-of-input

		writes = append(writes, smartEditFileWrite{
			FilePath: path,
			Content:  strings.Join(contentLines, "\n"),
		})
	}

	if len(writes) == 0 {
		return nil, errors.New("no write blocks found")
	}

	return writes, nil
}

func stripOptionalQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
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
