package toolxml

import (
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

// SystemPrompt returns the system prompt suffix that enables XML-wrapped tool calling.
// The caller should append this to the base system prompt when XML tools are enabled.
func SystemPrompt(defs []tool.Definition) string {
	if len(defs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("You can call tools by outputting XML wrapped in <tool_data>...</tool_data>.\n")
	b.WriteString("When you need to call tools, output ONLY the <tool_data> block (no other text).\n")
	b.WriteString("For multiple tool calls in one step, put multiple <call>...</call> blocks inside a single <tool_data>.\n")
	b.WriteString("For multi-line fields, wrap content in <![CDATA[ ... ]]>.\n\n")
	b.WriteString("XML schema:\n")
	b.WriteString("<tool_data>\n")
	b.WriteString("  <call>\n")
	b.WriteString("    <tool_name>...</tool_name>\n")
	b.WriteString("    <!-- tool-specific fields -->\n")
	b.WriteString("  </call>\n")
	b.WriteString("</tool_data>\n\n")
	b.WriteString("Supported tools:\n")
	for _, def := range defs {
		name := def.Spec.Function.Name
		switch name {
		case "bash":
			b.WriteString("- bash:\n")
			b.WriteString("  <tool_name>bash</tool_name>\n")
			b.WriteString("  <command><![CDATA[...]]></command>\n")
			b.WriteString("  <timeout_ms>10000</timeout_ms> (optional)\n")
		case "smart_edit":
			b.WriteString("- smart_edit (preferred for file edits and writes):\n")
			b.WriteString("  1) Fuzzy replace:\n")
			b.WriteString("     <tool_name>smart_edit</tool_name>\n")
			b.WriteString("     <filePath>path/to/file</filePath>\n")
			b.WriteString("     <oldcontent><![CDATA[...]]></oldcontent>\n")
			b.WriteString("     <newcontent><![CDATA[...]]></newcontent>\n")
			b.WriteString("     <replaceAll>true</replaceAll> (optional)\n")
			b.WriteString("  2) Write full file:\n")
			b.WriteString("     <tool_name>smart_edit</tool_name>\n")
			b.WriteString("     <filePath>path/to/file</filePath>\n")
			b.WriteString("     <content><![CDATA[...]]></content>\n")
			b.WriteString("  3) Advanced script (optional):\n")
			b.WriteString("     <tool_name>smart_edit</tool_name>\n")
			b.WriteString("     <command><![CDATA[\n")
			b.WriteString("apply_smart_edit <<'EOF'\n")
			b.WriteString("file: path/to/file\n")
			b.WriteString("<<<< SEARCH\n")
			b.WriteString("...\n")
			b.WriteString("==== REPLACE\n")
			b.WriteString("...\n")
			b.WriteString(">>>>\n")
			b.WriteString("EOF\n")
			b.WriteString("]]></command>\n")
		default:
			if strings.TrimSpace(name) != "" {
				b.WriteString(fmt.Sprintf("- %s: not documented\n", name))
			}
		}
	}

	b.WriteString("\nTool result schema:\n")
	b.WriteString("<tool_result>\n")
	b.WriteString("  <call>\n")
	b.WriteString("    <tool_name>...</tool_name>\n")
	b.WriteString("    <tool_call_id>...</tool_call_id>\n")
	b.WriteString("    <ok>true|false</ok>\n")
	b.WriteString("    <output><![CDATA[{...json...}]]></output>\n")
	b.WriteString("    <error><![CDATA[...]]></error>\n")
	b.WriteString("  </call>\n")
	b.WriteString("</tool_result>\n")
	b.WriteString("If <ok> is false, adjust the next <tool_data> call and try again.\n")
	return b.String()
}
