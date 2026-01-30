# Change: Make tool function names OpenAI-compatible (no dots) with aliases

## Why
部分 OpenAI-compatible provider 对 tool function name 有严格约束（常见为只允许字母数字、下划线、短横线；不允许 `.`）。当前项目内存在多处带点号的 tool name：
- `skill.read`
- `document.export`
- `lsp.definition` / `lsp.references` / `lsp.rename_preview`

这会造成：
- provider 注册 tools 失败或行为不确定（成功率低但难以定位）
- prompt/manual 文件名与 tool 名不一致，增加维护成本

## What Changes
- 将上述工具的“canonical name”统一为下划线形式：
  - `skill_read`
  - `document_export`
  - `lsp_definition`, `lsp_references`, `lsp_rename_preview`
- 为兼容历史 prompt/skills/前端，系统必须保留旧名字作为 alias（过渡期）：
  - 调用旧名字仍能执行（best-effort）
  - 但文档/手册/示例统一使用新名字

## Impact
- Affected specs: `system-skill-management`, `system-document-export`, `system-code-intelligence`
- Affected code (expected): `backend/internal/tool/*`, `backend/internal/tool/registry.go`（alias map）, `backend/internal/prompt/assets/tools/*`
- Tests (expected): tool registry + e2e tool loop tests

