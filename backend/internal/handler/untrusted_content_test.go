package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func extractUntrustedJSONPayload(t *testing.T, wrapped string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(wrapped), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %q", len(lines), wrapped)
	}
	if strings.TrimSpace(lines[0]) != untrustedContentBegin {
		t.Fatalf("expected begin marker %q, got %q", untrustedContentBegin, lines[0])
	}
	if strings.TrimSpace(lines[len(lines)-1]) != untrustedContentEnd {
		t.Fatalf("expected end marker %q, got %q", untrustedContentEnd, lines[len(lines)-1])
	}

	payload := strings.Join(lines[1:len(lines)-1], "\n")
	var out map[string]any
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		t.Fatalf("payload must be valid JSON: %v\npayload=%q", err, payload)
	}
	return out
}

func TestWrapUntrustedToolOutput_JSONEnvelope(t *testing.T) {
	t.Run("okPayloadEmbedsJSONOutput", func(t *testing.T) {
		raw := `{"ok":true,"content":"x"}`
		wrapped := wrapUntrustedToolOutput("rg", "call_1", raw)

		env := extractUntrustedJSONPayload(t, wrapped)
		if env["_untrusted"] != true {
			t.Fatalf("expected _untrusted=true, got %#v", env["_untrusted"])
		}
		source, _ := env["source"].(map[string]any)
		if source["kind"] != "tool" || source["tool"] != "rg" || source["tool_call_id"] != "call_1" {
			t.Fatalf("unexpected source: %#v", source)
		}
		if env["ok"] != true {
			t.Fatalf("expected ok=true, got %#v", env["ok"])
		}
		if _, ok := env["output"].(map[string]any); !ok {
			t.Fatalf("expected output to be a JSON object, got %#v", env["output"])
		}
		if _, hasErr := env["error"]; hasErr {
			t.Fatalf("expected no error, got %#v", env["error"])
		}
	})

	t.Run("errorPayloadIncludesErrorFields", func(t *testing.T) {
		raw := `{"error":"command not allowed by profile=dev: find"}`
		wrapped := wrapUntrustedToolOutput("bash", "call_2", raw)

		env := extractUntrustedJSONPayload(t, wrapped)
		if env["ok"] != false {
			t.Fatalf("expected ok=false, got %#v", env["ok"])
		}
		errObj, _ := env["error"].(map[string]any)
		code, _ := errObj["error_code"].(string)
		hint, _ := errObj["hint"].(string)
		if strings.TrimSpace(code) == "" {
			t.Fatalf("expected error_code, got %#v", errObj)
		}
		if strings.TrimSpace(hint) == "" {
			t.Fatalf("expected hint, got %#v", errObj)
		}
	})

	t.Run("nonJSONOutputFallsBackToJSONString", func(t *testing.T) {
		wrapped := wrapUntrustedToolOutput("tool", "call_3", "hello")

		env := extractUntrustedJSONPayload(t, wrapped)
		if env["ok"] != true {
			t.Fatalf("expected ok=true, got %#v", env["ok"])
		}
		if _, ok := env["output"].(string); !ok {
			t.Fatalf("expected output to be a string, got %#v", env["output"])
		}
	})
}
