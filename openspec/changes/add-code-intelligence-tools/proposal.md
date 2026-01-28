# Change: Add code intelligence tools (LSP/AST)

## Why
目前对代码的理解主要依赖 `rg/glob/read_file + edit` 的文本级操作，缺少语义级能力（definition/references/rename 预览），容易出现“漏改引用/改错位置/跨文件不一致”。

引入最小语义工具集能显著提升“稳定性交付”的上限，并减少对提示词与上下文拼接的依赖。

## What Changes
- 新增语义级只读工具：
  - `lsp.definition`
  - `lsp.references`
  - `lsp.rename_preview`（输出改动清单，不直接写文件）
- 提供可诊断的降级策略（缺少语言服务器/不支持语言时明确报错 + doctor 提示）

## Impact
- Affected specs: `system-code-intelligence` (new)
- Affected code: `backend/internal/tool/*`, `backend/internal/lsp/*` (new), `backend/internal/tool/preconditions.go` (scope integration)

