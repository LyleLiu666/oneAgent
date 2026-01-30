## ADDED Requirements

### Requirement: Core built-in tools MUST have tool manuals
系统必须 (MUST) 为核心内置工具提供 tool manuals（prompt assets），并确保在工具启用时会被注入 stable prefix。tool manuals 的文件名必须 (MUST) 与 assembler 的 `sanitizeToolName` 规则一致（例如 `lsp.definition` → `lsp_definition.md`）。

至少应覆盖（best-effort）：
- `read_file`, `write_file`
- `edit`, `edit_v2`, `multiedit`
- `rg`, `glob`, `ls`, `search`
- `run_command`, `plan`, `subagent`
- `skill.read`（manual 文件名应为 `skill_read.md`）
- `document.export`（manual 文件名应为 `document_export.md`）
- `lsp.*`（manual 文件名应为 `lsp_definition.md` / `lsp_references.md` / `lsp_rename_preview.md`，best-effort）

#### Scenario: Assembler injects manual for a core tool
- **GIVEN** 本轮启用了 `read_file` 与 `rg`
- **WHEN** 系统构建 stable prefix
- **THEN** stable prefix 包含 `read_file` 与 `rg` 的 tool manuals（best-effort）

