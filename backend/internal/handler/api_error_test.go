package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestRedactErrorString_RedactsBearerToken(t *testing.T) {
	out := redactErrorString("Authorization: Bearer abc.def.ghi")
	if strings.Contains(out, "abc.def.ghi") {
		t.Fatalf("expected token to be redacted, got %q", out)
	}
	if !strings.Contains(out, "Bearer <redacted>") {
		t.Fatalf("expected redaction marker, got %q", out)
	}
}

func TestRedactErrorString_RedactsUnixAbsolutePaths(t *testing.T) {
	out := redactErrorString("open /Users/alice/secrets/token.txt: permission denied")
	if strings.Contains(out, "/Users/alice/secrets/token.txt") {
		t.Fatalf("expected unix path to be redacted, got %q", out)
	}
	if !strings.Contains(out, "<path>") {
		t.Fatalf("expected <path> marker, got %q", out)
	}
}

func TestRedactErrorString_RedactsWindowsAbsolutePaths(t *testing.T) {
	out := redactErrorString(`failed to read C:\Users\alice\secrets\token.txt`)
	if strings.Contains(out, `C:\Users\alice\secrets\token.txt`) {
		t.Fatalf("expected windows path to be redacted, got %q", out)
	}
	if !strings.Contains(out, "<path>") {
		t.Fatalf("expected <path> marker, got %q", out)
	}
}

func TestClassifyAPIError_ToolPermissionDeniedIsUserSafe(t *testing.T) {
	code, msg, hint := classifyAPIError(http.StatusBadRequest, errors.New("command not allowed by profile=readonly: rm -rf /Users/alice"))
	if code != "tool_permission_denied" {
		t.Fatalf("expected code=tool_permission_denied, got %q", code)
	}
	if strings.Contains(strings.ToLower(msg), "profile=readonly") || strings.Contains(strings.ToLower(msg), "not allowed") {
		t.Fatalf("expected user-safe msg, got %q", msg)
	}
	if strings.TrimSpace(hint) == "" {
		t.Fatalf("expected non-empty hint")
	}
}
