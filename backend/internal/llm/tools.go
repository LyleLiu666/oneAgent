package llm

import (
	"encoding/json"
	"sort"
	"strings"
)

func normalizeTools(tools []Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}

	ordered := make([]Tool, len(tools))
	copy(ordered, tools)
	for i := range ordered {
		ordered[i].Function.Name = sanitizeProviderToolName(ordered[i].Function.Name)
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		leftID := ordered[i].ID
		rightID := ordered[j].ID
		if leftID == "" || rightID == "" {
			return ordered[i].Function.Name < ordered[j].Function.Name
		}
		if leftID == rightID {
			return ordered[i].Function.Name < ordered[j].Function.Name
		}
		return leftID < rightID
	})

	return ordered
}

func sanitizeProviderToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(name))

	lastUnderscore := false
	for _, r := range name {
		allowed := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' ||
			r == '-'
		if allowed {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if lastUnderscore {
			continue
		}
		b.WriteByte('_')
		lastUnderscore = true
	}

	return strings.Trim(b.String(), "_")
}

func sanitizeProviderToolChoice(choice any) any {
	if choice == nil {
		return nil
	}

	if s, ok := choice.(string); ok {
		return strings.TrimSpace(s)
	}

	data, err := json.Marshal(choice)
	if err != nil {
		return choice
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return choice
	}
	if len(decoded) == 0 {
		return decoded
	}

	typeName, _ := decoded["type"].(string)
	typeName = strings.ToLower(strings.TrimSpace(typeName))

	if fnRaw, ok := decoded["function"].(map[string]any); ok {
		if name, ok := fnRaw["name"].(string); ok {
			fnRaw["name"] = sanitizeProviderToolName(name)
		}
		decoded["function"] = fnRaw
	}

	if typeName == "function" || typeName == "tool" {
		if name, ok := decoded["name"].(string); ok {
			decoded["name"] = sanitizeProviderToolName(name)
		}
	}

	return decoded
}

func ToolNamesEquivalent(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return sanitizeProviderToolName(left) == sanitizeProviderToolName(right)
}
