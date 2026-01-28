# Change: Add `edit_v2` tool (deterministic + explainable edits)

## Why
当前 `edit`（SBE）偏“容错成功率”，但缺少可解释性与风险分级，存在低概率 silent corruption 风险：改对文件但改错位置、或多处匹配时不确定替换。

为了支撑“数小时后台任务 + 强制证据”的交付定位，需要一个更确定、可证明的编辑工具：要么明确成功（含证据），要么明确失败并给出可操作诊断。

## What Changes
- 新增 `edit_v2` 工具（不替换 `edit`，避免破坏兼容）
- 支持 occurrence/anchor/期望替换次数等约束，降低误编辑
- 返回可解释的匹配证据（命中策略、行号范围、diff preview、替换计数）

## Impact
- Affected specs: `system-file-tools`
- Affected code: `backend/internal/tool/*`, `backend/internal/sbe/*`, `docs/tool-call-failures.md`（工具手册/提示词约束）

