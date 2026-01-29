package toolxml

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func buildPlainText(minRunes int) string {
	if minRunes <= 0 {
		return ""
	}
	return strings.Repeat("空", minRunes)
}

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

func TestToolProtocols_LongText_JSONAndXML_AreLosslessAndMeasurable(t *testing.T) {
	const minRunes = 3000

	cases := []struct {
		name                string
		content             string
		expectXMLSmaller    bool
		expectInvalidRawJSON bool
	}{
		{
			name:             "plainText",
			content:          buildPlainText(minRunes),
			expectXMLSmaller: false,
		},
		{
			name:                 "codeLike",
			content:              buildLongText(minRunes),
			expectXMLSmaller:     true,
			expectInvalidRawJSON: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if utf8.RuneCountInString(tc.content) < minRunes {
				t.Fatalf("expected >=%d runes, got %d", minRunes, utf8.RuneCountInString(tc.content))
			}

			xmlBlock := "<tool_data>\n" +
				"  <call>\n" +
				"    <tool_name>write_file</tool_name>\n" +
				"    <filePath>a.txt</filePath>\n" +
				"    <content><![CDATA[" + tc.content + "]]></content>\n" +
				"  </call>\n" +
				"</tool_data>\n"

			calls, err := ParseToolData(xmlBlock)
			if err != nil {
				t.Fatalf("ParseToolData: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}
			if calls[0].Fields["content"] != tc.content {
				t.Fatalf("expected xml content to roundtrip losslessly")
			}

			xmlArgs, _, err := buildToolArgs("write_file", calls[0].Fields)
			if err != nil {
				t.Fatalf("buildToolArgs(xml): %v", err)
			}
			var xmlDecoded struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(xmlArgs, &xmlDecoded); err != nil {
				t.Fatalf("unmarshal xml args: %v", err)
			}
			if xmlDecoded.Content != tc.content {
				t.Fatalf("expected xml args content to roundtrip losslessly")
			}

			jsonArgsBytes, err := json.Marshal(map[string]any{
				"filePath": "a.txt",
				"content":  tc.content,
			})
			if err != nil {
				t.Fatalf("marshal json args: %v", err)
			}
			var jsonDecoded struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(jsonArgsBytes, &jsonDecoded); err != nil {
				t.Fatalf("unmarshal json args: %v", err)
			}
			if jsonDecoded.Content != tc.content {
				t.Fatalf("expected json args content to roundtrip losslessly")
			}

			t.Logf("sizes: xml_block=%d json_args=%d", len(xmlBlock), len(jsonArgsBytes))

			if tc.expectXMLSmaller && len(xmlBlock) >= len(jsonArgsBytes) {
				t.Fatalf("expected xml_block < json_args for code-like long text; got xml=%d json=%d", len(xmlBlock), len(jsonArgsBytes))
			}

			rawJSON := `{"filePath":"a.txt","content":"` + tc.content + `"}`
			isRawValid := json.Valid([]byte(rawJSON))
			if tc.expectInvalidRawJSON && isRawValid {
				t.Fatalf("expected raw/unescaped json to be invalid")
			}
			if !tc.expectInvalidRawJSON && !isRawValid {
				t.Fatalf("expected raw json to be valid for plain text")
			}
		})
	}
}
