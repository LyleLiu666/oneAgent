# Change: Add core tool manuals (prompt/assets/tools)

## Why
`system-prompt-assembly` 会根据启用的 tools 注入对应 tool manual，但当前 `backend/internal/prompt/assets/tools/` 仅有：
- `bash.md`
- `write_file.md`

缺少高频工具（`read_file/edit/edit_v2/rg/glob/ls/run_command/...`）的手册会导致模型：
- 不知道应该优先用更稳定的文件/搜索工具
- 退化为 bash（而 bash 受权限限制，进一步拉低成功率）

专家建议清单见：`docs/oneAgent_toolcall_advice/docs/07_建议补齐的工具手册清单_模板.md`。

## What Changes
- 为核心内置工具补齐 tool manuals（遵循 assembler 的 `sanitizeToolName` 规则：`.`→`_`），并保证启用工具时会被注入 stable prefix。
- 扩展 prompt unit tests，确保关键工具的手册存在且可被注入（防止回归）。

## Impact
- Affected specs: `system-prompt-assembly`
- Affected code (expected): `backend/internal/prompt/assets/tools/*.md`, `backend/internal/prompt/assembler_test.go`
- Reference docs: `docs/oneAgent_toolcall_advice/docs/07_建议补齐的工具手册清单_模板.md`

