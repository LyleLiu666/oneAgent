# Design: Code intelligence (LSP/AST)

## Principles
- Read-only by default: `rename_preview` 输出 edits，不直接写入文件
- Workspace-scoped: 所有 file paths 必须在 workspace 内（防越界）
- Deterministic output: 返回结果稳定排序，避免抖动

## Tool surface (v1)
- `lsp.definition(filePath, line, column)` → definitions[]
- `lsp.references(filePath, line, column, include_declaration?)` → references[]
- `lsp.rename_preview(filePath, line, column, new_name)` → edits[] (file + ranges + newText)

## Implementation sketch
- 首先仅支持 Go / TS 两类常见语言（后续可扩展）
- 采用进程级 language server（可复用/缓存），并为每个 workspace 维护 session
- 输出做上限保护（最多 N 个 reference；超过则提示 refine）

