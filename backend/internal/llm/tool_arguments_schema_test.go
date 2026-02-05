package llm

import (
	"encoding/json"
	"testing"
)

func TestNormalizeToolArgumentsJSONForTool_AddsMissingRequiredKeys(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "write_file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
					"content":  map[string]any{"type": "string"},
				},
				"required": []any{"filePath", "content"},
			},
		},
	}

	got := normalizeToolArgumentsJSONForTool(tool, "{}")
	if !json.Valid([]byte(got)) {
		t.Fatalf("expected JSON, got %q", got)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["filePath"] != "" {
		t.Fatalf("expected filePath to default to empty string, got %#v", parsed["filePath"])
	}
	if parsed["content"] != "" {
		t.Fatalf("expected content to default to empty string, got %#v", parsed["content"])
	}
}

func TestNormalizeToolArgumentsJSONForTool_PreservesExistingRequiredKeys(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "write_file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
					"content":  map[string]any{"type": "string"},
				},
				"required": []any{"filePath", "content"},
			},
		},
	}

	got := normalizeToolArgumentsJSONForTool(tool, `{"filePath":"a.txt","content":"hi"}`)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["filePath"] != "a.txt" || parsed["content"] != "hi" {
		t.Fatalf("unexpected args after normalization: %q", got)
	}
}

func TestNormalizeToolArgumentsJSONForTool_AddsRequiredKeysWhenArgsInvalidJSON(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "write_file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
					"content":  map[string]any{"type": "string"},
				},
				"required": []any{"filePath", "content"},
			},
		},
	}

	got := normalizeToolArgumentsJSONForTool(tool, "{not json")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["_raw"]; !ok {
		t.Fatalf("expected _raw to be preserved for invalid JSON args, got %q", got)
	}
	if parsed["filePath"] != "" || parsed["content"] != "" {
		t.Fatalf("expected required keys to be defaulted, got %q", got)
	}
}

