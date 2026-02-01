package toolxml

import "testing"

func TestParseToolData_UnclosedCDATA_StripsPrefix(t *testing.T) {
	input := `<tool_data>
  <call>
    <tool_name>bash</tool_name>
    <command><![CDATA[ls -la</command>
  </call>
</tool_data>`

	calls, err := ParseToolData(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ToolName != "bash" {
		t.Fatalf("expected tool 'bash', got %q", calls[0].ToolName)
	}
	if got := calls[0].Fields["command"]; got != "ls -la" {
		t.Fatalf("expected command %q, got %q", "ls -la", got)
	}
}

func TestParseToolData_ToleratesToolNameClosingTagTypo(t *testing.T) {
	input := `<tool_data>
	  <call>
	    <tool_name>write_file</toolName>
	    <filePath>a.txt</filePath>
	    <content>hi</content>
	  </call>
	</tool_data>`

	calls, err := ParseToolData(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ToolName != "write_file" {
		t.Fatalf("expected tool 'write_file', got %q", calls[0].ToolName)
	}
	if got := calls[0].Fields["filePath"]; got != "a.txt" {
		t.Fatalf("expected filePath %q, got %q", "a.txt", got)
	}
	if got := calls[0].Fields["content"]; got != "hi" {
		t.Fatalf("expected content %q, got %q", "hi", got)
	}
}

func TestParseToolData_RepairsMissingFilePathOpenTagAfterToolNameClose(t *testing.T) {
	input := `<tool_data>
	  <tool_name>edit</toolName>/Users/liu_y/code/pyProject/testPro/caipu/src/generator.py</filePath>
	  <oldcontent>old</oldcontent>
	  <newcontent>new</newcontent>
	</tool_data>`

	calls, err := ParseToolData(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ToolName != "edit" {
		t.Fatalf("expected tool 'edit', got %q", calls[0].ToolName)
	}
	if got := calls[0].Fields["filePath"]; got != "/Users/liu_y/code/pyProject/testPro/caipu/src/generator.py" {
		t.Fatalf("expected filePath %q, got %q", "/Users/liu_y/code/pyProject/testPro/caipu/src/generator.py", got)
	}
}

func TestParseToolData_RepairsToolNameClosedByFilePathTag(t *testing.T) {
	input := `<tool_data>
	  <tool_name>read_file</filePath>src/generator.py</filePath>
	</tool_data>`

	calls, err := ParseToolData(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ToolName != "read_file" {
		t.Fatalf("expected tool 'read_file', got %q", calls[0].ToolName)
	}
	if got := calls[0].Fields["filePath"]; got != "src/generator.py" {
		t.Fatalf("expected filePath %q, got %q", "src/generator.py", got)
	}
}
