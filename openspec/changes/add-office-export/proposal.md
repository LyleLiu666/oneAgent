# Change: Add document export (Markdown → Office)

## Why
对于非开发者用户，Markdown 往往不是最终交付物。若用户仍需手动复制粘贴到 Word/PPT 并调格式，交付闭环不完整。

提供“最后一公里”的导出能力（至少 Markdown → DOCX/PPTX）可以显著提升可交付性与部门推广价值。

## What Changes
- 新增导出能力：`.md` → `.docx` / `.pptx`（v1）
- 支持可选模板（公司模板）与输出路径
- 明确依赖可诊断（pandoc/模板缺失时给出可操作提示）

## Impact
- Affected specs: `system-document-export` (new), `platform-support` (diagnostics)
- Affected code: `backend/internal/tool/*` (new tool), `backend/internal/doctor/*` (optional)

