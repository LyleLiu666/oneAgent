# Design: `edit_v2`

## Goals
- Deterministic: 多候选/低置信度时宁可失败，不做不确定替换
- Explainable: 每次替换返回“为什么命中/命中哪里/改了多少”
- Compatible: 保留现有 `edit` 行为；新能力通过 `edit_v2` 提供

## Proposed tool contract (v1)
Inputs:
- `filePath` (string)
- `oldcontent` (string)
- `newcontent` (string)
- `occurrence` (int, optional; default=1)
- `before` / `after` (string, optional anchors; used to disambiguate)
- `expected_replacements` (int, optional; if set and mismatched -> fail)
- `expected_sha256` (string, optional; OCC precondition)

Outputs:
- `ok` (bool)
- `replacements` (int)
- `matched_strategy` (string)
- `matched_range` (`start_line`, `end_line`)
- `diff_preview` (string, capped)
- `precondition_failed` (bool)
- `diagnostics` (string, human readable)

## Safety notes
- Preserve newline style (LF/CRLF) when writing back.
- Use atomic replace for writeback (reuse existing fsutil atomic write).
- Cap output size (diff preview, diagnostics) to avoid token blowups.

