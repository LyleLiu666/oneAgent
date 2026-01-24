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
	b.WriteString("你可以通过输出 XML 的 <tool_data>...</tool_data> 来调用工具。\n")
	b.WriteString("当你需要调用工具时：只输出 <tool_data> 块（不要输出其它文本）。\n")
	b.WriteString("同一步内需要调用多个工具：在一个 <tool_data> 里放多个 <call>...</call>。\n")
	b.WriteString("重要：请小步、分段、多次调用；编辑/写入内容建议每段 ≤3000 字，避免一次性输出过长（可能超出 LLM 最大 token，导致工具调用失败）。\n")
	b.WriteString("\n")
	b.WriteString("XML 结构：\n")
	b.WriteString("<tool_data>\n")
	b.WriteString("  <call>\n")
	b.WriteString("    <tool_name>...</tool_name>\n")
	b.WriteString("    <!-- 不同工具有不同字段 -->\n")
	b.WriteString("  </call>\n")
	b.WriteString("</tool_data>\n\n")
	b.WriteString("支持的工具：\n")
	for _, def := range defs {
		name := def.Spec.Function.Name
		switch name {
		case "bash":
			b.WriteString("- bash:\n")
			b.WriteString("  <tool_name>bash</tool_name>\n")
			b.WriteString("  <command>...</command>\n")
			b.WriteString("  <timeout_ms>10000</timeout_ms>（可选）\n")
			b.WriteString("  说明：sudo/apt-get/pip/node/npm/find/touch 等命令会被阻止；路径必须在 $BASH_ROOT_DIR 内。\n")
		case "run_command":
			b.WriteString("- run_command（异步执行 bash，适用于长任务）：\n")
			b.WriteString("  启动(start)：\n")
			b.WriteString("    <tool_name>run_command</tool_name>\n")
			b.WriteString("    <action>start</action>\n")
			b.WriteString("    <command>...</command>\n")
			b.WriteString("    <wait_seconds>10</wait_seconds>（可选，最大 30）\n")
			b.WriteString("    <max_runtime_seconds>600</max_runtime_seconds>（可选）\n")
			b.WriteString("  轮询(poll)：\n")
			b.WriteString("    <tool_name>run_command</tool_name>\n")
			b.WriteString("    <action>poll</action>\n")
			b.WriteString("    <job_id>...</job_id>\n")
			b.WriteString("    <wait_seconds>2</wait_seconds>（可选，最大 30）\n")
			b.WriteString("    <stdout_offset>0</stdout_offset>（可选）\n")
			b.WriteString("    <stderr_offset>0</stderr_offset>（可选）\n")
			b.WriteString("  说明：与 bash 同样的沙箱限制；用 stdout/stderr 的 offset 分段拉取输出，避免单次输出过大。\n")
		case "edit":
			b.WriteString("- edit（用于对已有文件做分段替换，强烈推荐小步多次）：\n")
			b.WriteString("  整文件写入/新建请用 write_file。\n")
			b.WriteString("  模糊替换示例：\n")
			b.WriteString("     <tool_name>edit</tool_name>\n")
			b.WriteString("     <filePath>path/to/file</filePath>\n")
			b.WriteString("     <oldcontent>...</oldcontent>\n")
			b.WriteString("     <newcontent>...</newcontent>\n")
			b.WriteString("     <replaceAll>true</replaceAll>（可选）\n")
		case "write_file":
			b.WriteString("- write_file（用于写文件/新建文件，建议分段写入避免过长）：\n")
			b.WriteString("     <tool_name>write_file</tool_name>\n")
			b.WriteString("     <filePath>path/to/file</filePath>\n")
			b.WriteString("     <content>...</content>\n")
			b.WriteString("     <append>true</append>（可选；true=追加写入，用于分段写入大文件）\n")
		case "glob":
			b.WriteString("- glob:\n")
			b.WriteString("  <tool_name>glob</tool_name>\n")
			b.WriteString("  <pattern>**/*.go</pattern>\n")
			b.WriteString("  说明：pattern 相对 $BASH_ROOT_DIR（或为其内部的绝对路径）。\n")
		case "ls":
			b.WriteString("- ls:\n")
			b.WriteString("  <tool_name>ls</tool_name>\n")
			b.WriteString("  <path>path/to/dir</path>（可选，默认 '.'）\n")
		case "multiedit":
			b.WriteString("- multiedit（edit 的别名：批量模糊替换，兼容旧提示词）：\n")
			b.WriteString("  <tool_name>multiedit</tool_name>\n")
			b.WriteString("  <edits><![CDATA[\n")
			b.WriteString("  [\n")
			b.WriteString("    {\"filePath\":\"path/to/file\",\"oldString\":\"...\",\"newString\":\"...\",\"replaceAll\":true}\n")
			b.WriteString("  ]\n")
			b.WriteString("  ]]></edits>\n")
			b.WriteString("  <replaceAll>true</replaceAll>（可选；对所有 edits 生效）\n")
		default:
			if strings.TrimSpace(name) != "" {
				b.WriteString(fmt.Sprintf("- %s: not documented\n", name))
			}
		}
	}

	b.WriteString("\n工具返回结构：\n")
	b.WriteString("<tool_result>\n")
	b.WriteString("  <call>\n")
	b.WriteString("    <tool_name>...</tool_name>\n")
	b.WriteString("    <tool_call_id>...</tool_call_id>\n")
	b.WriteString("    <ok>true|false</ok>\n")
	b.WriteString("    <output>{...json...}</output>\n")
	b.WriteString("    <error>...</error>\n")
	b.WriteString("  </call>\n")
	b.WriteString("</tool_result>\n")
	b.WriteString("如果 <ok> 为 false，请根据错误信息调整下一次 <tool_data> 的参数重试。\n")
	return b.String()
}
