# Change: Improve Digest / notifications / batch follow-up (“harvest mode”)

## Why
oneAgent 的核心价值是“用户把任务丢给 agent 后可以离开，回来能收割成果”。当前系统已具备 Work Ledger + Digest，但 Digest 主要是 markdown 展示，缺少：
- **可操作的聚合视图**：失败原因聚类、按 workspace/task/tool 分类、可筛选/检索
- **批处理 follow-up**：把一组失败/待决策条目一次性转成下一轮任务
- **低打扰的通知信号**：用户不持续盯屏也能知道“该回来看结果了”

这会导致用户在收割阶段仍然需要逐条点开、手动整理，损耗大、耐心消耗快。

## What Changes
- 扩展 `system-work-ledger`：Digest 产出结构化数据（除 markdown 外），支持失败聚类与批处理 follow-up 的数据基础
- 扩展 `work-ledger-ux`：提供“Harvest mode”的 Digest 视图（聚合、聚类、筛选、批处理 follow-up）
- 通知策略（best-effort）：至少提供 in-app badge/提示；后续可扩展 desktop/webhook/email

## Impact
- Affected specs: `system-work-ledger`, `work-ledger-ux`
- Affected code (expected): `backend/internal/workledger/*`, `backend/internal/handler/ledger*.go`, `frontend/src/views/Ledger*.vue`
- Related docs: `openspec/roadmap-longterm.md`, `docs/ux-vision.md`
