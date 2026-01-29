package toolxml

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func buildLongText(minRunes int) string {
	pattern := `第1段: "quoted" \\ path \n lineA
lineB
<tag>hello</tag> </div> end
（中文）`
	per := utf8.RuneCountInString(pattern)
	repeats := 1
	if per > 0 && minRunes > per {
		repeats = (minRunes + per - 1) / per
	}
	out := strings.Repeat(pattern, repeats)
	// Defensive: CDATA cannot contain "]]>".
	out = strings.ReplaceAll(out, "]]>", "]] ]>")
	return out
}

func TestParseToolData_LongTextCDATA_RoundTripToArgs(t *testing.T) {
	const minRunes = 3000
	content := buildLongText(minRunes)
	if utf8.RuneCountInString(content) < minRunes {
		t.Fatalf("expected >=%d runes, got %d", minRunes, utf8.RuneCountInString(content))
	}

	toolData := "<tool_data>\n" +
		"  <call>\n" +
		"    <tool_name>write_file</tool_name>\n" +
		"    <filePath>a.txt</filePath>\n" +
		"    <content><![CDATA[" + content + "]]></content>\n" +
		"  </call>\n" +
		"</tool_data>\n"

	calls, err := ParseToolData(toolData)
	if err != nil {
		t.Fatalf("ParseToolData: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ToolName != "write_file" {
		t.Fatalf("expected tool=%q, got %q", "write_file", calls[0].ToolName)
	}
	if calls[0].Fields["filePath"] != "a.txt" {
		t.Fatalf("expected filePath=%q, got %q", "a.txt", calls[0].Fields["filePath"])
	}
	if calls[0].Fields["content"] != content {
		t.Fatalf("expected content to roundtrip losslessly")
	}

	args, argsString, err := buildToolArgs("write_file", calls[0].Fields)
	if err != nil {
		t.Fatalf("buildToolArgs: %v", err)
	}
	if strings.TrimSpace(argsString) == "" {
		t.Fatalf("expected argsString")
	}

	var decoded struct {
		FilePath string `json:"filePath"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if decoded.FilePath != "a.txt" {
		t.Fatalf("expected decoded filePath=%q, got %q", "a.txt", decoded.FilePath)
	}
	if decoded.Content != content {
		t.Fatalf("expected decoded content to roundtrip losslessly")
	}
}
