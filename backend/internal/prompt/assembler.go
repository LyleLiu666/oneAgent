package prompt

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed assets/**
var assetsFS embed.FS

type AssembleInput struct {
	// BaseOverride, if non-empty, replaces the built-in base persona module.
	BaseOverride string

	// ToolNames are LLM tool function names (e.g. "bash", "write_file", "edit_v2", "lsp_definition").
	// The assembler will include matching tool manual modules when available.
	ToolNames []string
}

type AssembleOutput struct {
	StablePrefix string
	Modules      []string
}

func AssembleStablePrefix(input AssembleInput) (AssembleOutput, error) {
	baseOverride := strings.TrimSpace(input.BaseOverride)

	modules := make([]string, 0, 8)
	used := make([]string, 0, 8)

	if baseOverride != "" {
		modules = append(modules, strings.TrimSpace(baseOverride))
		used = append(used, "base_override")
	} else {
		base, err := readAsset("assets/base_persona.md")
		if err != nil {
			return AssembleOutput{}, err
		}
		modules = append(modules, base)
		used = append(used, "assets/base_persona.md")
	}

	constraints, err := readAsset("assets/constraints.md")
	if err != nil {
		return AssembleOutput{}, err
	}
	modules = append(modules, constraints)
	used = append(used, "assets/constraints.md")

	// Tool manuals: stable order, include only if tool is enabled.
	keys := uniqueSortedToolKeys(input.ToolNames)
	for _, key := range keys {
		rel := "assets/tools/" + key + ".md"
		body, err := readAssetOptional(rel)
		if err != nil {
			return AssembleOutput{}, err
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		modules = append(modules, body)
		used = append(used, rel)
	}

	var b strings.Builder
	for i, m := range modules {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m)
	}

	out := strings.TrimSpace(b.String())
	if out == "" {
		return AssembleOutput{}, errors.New("assembled prompt is empty")
	}
	return AssembleOutput{StablePrefix: out, Modules: used}, nil
}

func uniqueSortedToolKeys(toolNames []string) []string {
	seen := make(map[string]bool, len(toolNames))
	keys := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := sanitizeToolName(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sanitizeToolName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.Trim(name, "_")
	return name
}

func readAsset(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = path.Clean(rel)
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid asset path: %q", rel)
	}
	data, err := assetsFS.ReadFile(rel)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readAssetOptional(rel string) (string, error) {
	body, err := readAsset(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return body, nil
}
