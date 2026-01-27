# Change: Add sidebar ledger status badges

## Why
当前 SOP 提示主要出现在 Ledger 页面内部，但用户可能主要停留在 Chat/Tasks/Governance。为了支持“长时交付 + 定期来收成果”的使用方式，需要在全局导航中也能看到关键状态（尤其是 SOP inbox 数量）。

## What Changes
- Sidebar 在 `Governance`（或 Ledger）导航项上展示 SOP `proposed` 数量 badge（来自 `GET /api/ledger/status/today`）
- 轮询刷新（低频）以保持状态不过时

## Impact
- Affected specs: `work-ledger-ux`
- Affected code: `frontend/src/components/Sidebar.vue` (+ tests)

