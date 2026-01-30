# Change: Improve SOP/skills learning pipeline (evidence, dedupe/merge, staleness retirement)

## Why
随着 Work Ledger 持续积累，系统会不断产生 SOP suggestions 与个人 skills。如果缺少“生命周期治理”，会出现：
- suggestions/skills 越积越多，召回噪音变大，成功率下降
- duplicates 增多但缺少自动化线索，治理成本上升
- 旧技能过时但仍被召回，导致执行失败/风格偏离

因此需要把“自动学习”从一次性生成，升级为**管线化**：强制证据 → 去重合并 → 过时淘汰。

## What Changes
- 扩展 `system-work-ledger`（学习管线）：
  - learning job 产出更结构化的治理线索（similar/duplicates hints、建议 merge targets、证据摘要）
  - 在生成 suggestions 时强制证据与去重策略（best-effort）
- 扩展 `system-skill-management`（生命周期）：
  - 记录技能使用信号（last_used_at/used_count best-effort）
  - 提供 staleness 检测与治理动作（deprecate/archive with reason）（best-effort）

## Impact
- Affected specs: `system-work-ledger`, `system-skill-management`
- Affected code (expected): `backend/internal/workledger/*`, `backend/internal/skill/*`, `frontend/src/views/Governance*`
- Related docs: `docs/roadmap-longterm.md`

