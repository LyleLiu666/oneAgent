## Why

Work Ledger 需要支持“用户不在线也能看见进展”的核心场景（挂机交付/定期回来验收），否则用户必须反复点开各页面检查是否有新结果，体验差且容易错过关键状态。

本变更在不引入通知通道（IM/webhook）的前提下，先用 UI badge/入口把关键结果“可被看见”：
- 今日 digest 是否已生成（无副作用，只读判断）
- 今日 learning job 状态（无副作用，只读判断）
- SOP 建议（proposed）数量（无副作用，只读统计）

## What Changes
- 新增只读 API：`GET /api/ledger/status/today`，返回当日汇总状态（digest/learning/sop）。
- Work Ledger UI（`/ledger`）Tab 上展示 badge（dot/count），并在页面加载时轻量拉取一次状态。

## Impact
- **Backward-compatible**：新增 API；前端可逐步接入。
- **风险**：避免“状态接口”触发 digest/learning 的生成（必须只读）。

