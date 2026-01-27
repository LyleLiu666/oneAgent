# Change: Add SOP governance workbench

## Why
SOP suggestions 是“从工作过程学习”的核心资产入口，但目前分散在 Ledger/Settings 中，难以作为一个专门的“待治理资产工作台”来使用。

我们需要一个明确入口，让用户可以：
- 快速浏览 *proposed/parked* 的建议（默认 inbox 上限 10）
- 以“稀缺性/know-how 深度/证据强度”为主线做筛选与排序
- 人工确认后再 approve（materialize 成 `SKILL.md`），或 merge/reject/park/archive

## What Changes
- 新增一个 UI 页面：SOP Governance Workbench（治理工作台）
  - 展示 SOP suggestions 列表，默认按 `total_score` 降序
  - 展示评分维度（scarcity/depth/evidence/total）与 compressibility 结果（若存在）
  - 提供治理动作：approve / reject / park / archive / merge / edit draft / similar
- 不新增 webhook/外部通知；仅作为 UI 内治理入口。

## Impact
- Affected specs: `work-ledger-ux`
- Affected code: `frontend/src/views/*`, `frontend/src/router/index.ts`, `frontend/src/components/Sidebar.vue`

