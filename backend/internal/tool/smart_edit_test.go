package tool

import (
	"encoding/json"
	"testing"
)

func TestSmartEdit_ParseCommand_StringEncodedJSONArray(t *testing.T) {
	raw := json.RawMessage(`{"command":"[\"cat > foo.txt <<'EOF'\",\"hello\",\"EOF\"]"}`)

	blocks, replaceAll, writes, err := parseSmartEditInput(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if replaceAll {
		t.Fatalf("expected replaceAll=false")
	}
	if len(blocks) != 0 {
		t.Fatalf("expected no edit blocks, got %d", len(blocks))
	}
	if len(writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(writes))
	}
	if writes[0].FilePath != "foo.txt" {
		t.Fatalf("expected filePath %q, got %q", "foo.txt", writes[0].FilePath)
	}
	if writes[0].Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", writes[0].Content)
	}
}
