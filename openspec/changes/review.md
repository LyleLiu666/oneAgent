# OpenSpec Changes: Priority & Progress

更新时间：2026-01-30

本文件只记录**当前 active changes** 的优先级与进度快照；历史内容不再维护。
已完成变更请看 `openspec/changes/archive/`，真实进度以 `openspec list` 为准。

## Priority（高 → 低）

### P0（核心体验 / 高 ROI）
- `add-live-llm-regression-suite`：真实跑一遍（读 settings.db 的 provider），产出大规模回归测试报告；对比 JSON vs XML（含 3000+ 字长文本入参）

### P1（治理与自动化 / 降低损耗）
- （暂无）

### P2（中长期愿景 / 上层形态）
- `add-workflow-orchestration-graph`：工作流编排（显式图）：节点=工作型 agent；交付物=文件集；Hard/Soft Gate

## Snapshot（来自 `openspec list`）
- `add-live-llm-regression-suite`：0/7 tasks
- `add-workflow-orchestration-graph`：12/16 tasks
