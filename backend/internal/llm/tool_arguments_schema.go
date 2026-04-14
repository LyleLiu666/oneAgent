package llm

import (
	"encoding/json"
	"strings"
)

func toolByName(tools []Tool) map[string]Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make(map[string]Tool, len(tools))
	for _, t := range tools {
		rawName := strings.TrimSpace(t.Function.Name)
		if rawName == "" {
			continue
		}
		out[rawName] = t

		sanitizedName := sanitizeProviderToolName(rawName)
		if sanitizedName == "" || sanitizedName == rawName {
			continue
		}
		cloned := t
		cloned.Function.Name = sanitizedName
		out[sanitizedName] = cloned
	}
	return out
}

func normalizeToolArgumentsJSONForTool(tool Tool, rawArgs string) string {
	args := SanitizeToolArgumentsJSON(normalizeToolArguments(rawArgs))
	required := requiredKeysFromToolSchema(tool.Function.Parameters)
	if len(required) == 0 {
		return args
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(args), &obj); err != nil || obj == nil {
		obj = map[string]any{"_raw": strings.TrimSpace(args)}
	}

	props, _ := tool.Function.Parameters["properties"].(map[string]any)
	changed := false

	for _, key := range required {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if v, ok := obj[key]; ok && v != nil {
			continue
		}
		obj[key] = defaultValueForJSONSchema(props[key])
		changed = true
	}

	if !changed {
		return args
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return args
	}
	return string(data)
}

func requiredKeysFromToolSchema(schema map[string]any) []string {
	if len(schema) == 0 {
		return nil
	}
	raw, ok := schema["required"]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, key := range v {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			out = append(out, key)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			key, ok := item.(string)
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			out = append(out, key)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func defaultValueForJSONSchema(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok || len(m) == 0 {
		return ""
	}
	if v, ok := m["default"]; ok && v != nil {
		return v
	}

	types := extractJSONSchemaTypes(m["type"])
	if len(types) == 0 {
		return ""
	}
	switch types[0] {
	case "string":
		return ""
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "object":
		return map[string]any{}
	case "array":
		return []any{}
	default:
		return ""
	}
}

func extractJSONSchemaTypes(raw any) []string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []string:
		out := make([]string, 0, len(v))
		for _, t := range v {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			out = append(out, t)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			t, ok := item.(string)
			if !ok {
				continue
			}
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			out = append(out, t)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}
