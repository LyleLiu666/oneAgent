# Design: Document export

## Scope (v1)
- Input: a markdown file in workspace (e.g. `report.md`)
- Output: `report.docx` / `slides.pptx` written under workspace output dir (or user-specified)
- Engine: pandoc (external dependency)

## Safety
- No network required; conversion is local
- Tool output must include generated file path and any stderr summary
- Hard cap on pandoc runtime/output size to avoid runaway

