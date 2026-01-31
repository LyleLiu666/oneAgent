package toolxml

import (
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

// SystemPrompt returns the system prompt suffix that enables XML-wrapped tool calling.
// The caller should append this to the base system prompt when XML tools are enabled.
func SystemPrompt(defs []tool.Definition) string {
	if len(defs) == 0 {
		return ""
	}

	defs, _ = FilterSupportedDefinitions(defs)
	if len(defs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("你可以通过输出 XML 的 <tool_data>...</tool_data> 来调用工具。\n")
	b.WriteString("当你需要调用工具时：只输出 <tool_data> 块（不要输出其它文本）。\n")
	b.WriteString("同一步内需要调用多个工具：在一个 <tool_data> 里放多个 <call>...</call>。\n")
	b.WriteString("重要：请小步、分段、多次调用；编辑/写入内容建议每段 ≤6000 字（write_file 单次最多 200000 字，超出会失败且不会写入），避免一次性输出过长（可能超出 LLM 最大 token，导致工具调用失败）。\n")
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
		name := tool.CanonicalToolName(def.Spec.Function.Name)
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
			b.WriteString("    <max_delta_bytes>16384</max_delta_bytes>（可选；本次最多返回多少字节的 stdout/stderr 增量）\n")
			b.WriteString("  轮询(poll)：\n")
			b.WriteString("    <tool_name>run_command</tool_name>\n")
			b.WriteString("    <action>poll</action>\n")
			b.WriteString("    <job_id>...</job_id>\n")
			b.WriteString("    <wait_seconds>2</wait_seconds>（可选，最大 30）\n")
			b.WriteString("    <max_delta_bytes>16384</max_delta_bytes>（可选）\n")
			b.WriteString("    <stdout_offset>0</stdout_offset>（可选）\n")
			b.WriteString("    <stderr_offset>0</stderr_offset>（可选）\n")
			b.WriteString("  说明：与 bash 同样的沙箱限制；stdout/stderr 由工具内部写入沙箱内临时文件并做“保留最近一段”的截断；用 offset + max_delta_bytes 分段拉取输出，避免单次输出过大。\n")
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
			b.WriteString("  <edits>\n")
			b.WriteString("  [\n")
			b.WriteString("    {\"filePath\":\"path/to/file\",\"oldString\":\"...\",\"newString\":\"...\",\"replaceAll\":true}\n")
			b.WriteString("  ]\n")
			b.WriteString("  </edits>\n")
			b.WriteString("  <replaceAll>true</replaceAll>（可选；对所有 edits 生效）\n")
		case "search":
			b.WriteString("- search（联网搜索）：\n")
			b.WriteString("  <tool_name>search</tool_name>\n")
			b.WriteString("  <query>...</query>\n")
			b.WriteString("  <count>5</count>（可选；默认 5）\n")
			b.WriteString("  <freshness>day|week|month</freshness>（可选）\n")
		case "plan":
			b.WriteString("- plan（计划管理）：\n")
			b.WriteString("  <tool_name>plan</tool_name>\n")
			b.WriteString("  <action>start|update|complete</action>\n")
			b.WriteString("  <task_id>...</task_id>（可选）\n")
			b.WriteString("  <template>...</template>（可选）\n")
			b.WriteString("  <overwrite>true</overwrite>（可选）\n")
		case "skill_read":
			b.WriteString("- skill_read（读取技能说明）：\n")
			b.WriteString("  <tool_name>skill_read</tool_name>\n")
			b.WriteString("  <name>...</name>（可选）\n")
			b.WriteString("  <skill_id>...</skill_id>（可选）\n")
		case "rg":
			b.WriteString("- rg（ripgrep，本地高速搜索，支持正则）：\n")
			b.WriteString("  <tool_name>rg</tool_name>\n")
			b.WriteString("  <pattern>...</pattern>\n")
			b.WriteString("  <path>path/to/dir</path>（可选，默认 '.'）\n")
			b.WriteString("  <max_results>50</max_results>（可选，默认 50，最大 200；达到上限会提前终止并标记 truncated=true）\n")
			b.WriteString("  <fixed_strings>true</fixed_strings>（可选；true=字面量搜索）\n")
			b.WriteString("  说明：若环境未安装 rg，将返回 available=false（工具调用 ok=true），你需要自行决定下一步（例如改用 bash）。\n")
		case "subagent":
			b.WriteString("- subagent（启动一个隔离上下文的子 Agent 执行独立步骤；返回短总结 + 引用路径）：\n")
			b.WriteString("  <tool_name>subagent</tool_name>\n")
			b.WriteString("  <task>...</task>\n")
			b.WriteString("  <context_summary>...</context_summary>（可选；前序步骤短总结 + findings/trace 引用）\n")
			b.WriteString("  <scope>backend/**</scope>（可选；可写范围 glob；支持逗号分隔多个）\n")
			b.WriteString("  <tool_ids>edit,write_file,rg</tool_ids>（可选；允许工具 ID 列表；默认继承主工具集但不含 subagent）\n")
			b.WriteString("  <skill_ids>translator</skill_ids>（可选；显式注入的技能 ID 列表；支持逗号分隔多个）\n")
			b.WriteString("  <k_skills>3</k_skills>（可选；自动召回并注入的技能 Top-K，0=不注入，默认 3）\n")
			b.WriteString("  <max_steps>200</max_steps>（可选；默认 200）\n")
			b.WriteString("  <max_runtime_seconds>3600</max_runtime_seconds>（可选；默认 3600）\n")
			b.WriteString("  说明：默认禁止递归（子 Agent 不挂载 subagent 工具本身）。\n")
		default:
			// XML protocol only exposes tools supported by the XML engine.
			// Keep this prompt fail-closed: do not advertise unknown tools.
			continue
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
