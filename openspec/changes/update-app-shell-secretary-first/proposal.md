# Change: Secretary-first app shell（默认隐藏侧边栏与管理菜单）

## Why
当前前端已具备 Chat “秘书模式”（隐藏历史/模型/工具/trace/任务面板等），但登录后全局侧边栏仍然始终展示（`frontend/src/App.vue`），导致：
- 默认体验仍像“管理系统/工作台”，与“像微信里和秘书聊天”的心智冲突
- 用户为了“只说事儿”仍需要过滤大量低频入口

我们希望把 oneAgent 的默认入口变成：**只保留秘书模式**；所有菜单/工作台只在“回归完全模式”时再出现（渐进式披露）。

## What Changes
- 引入“全局 UI 模式（secretary/full）”作为 app shell 的一等公民，并在刷新/重启后保持一致（持久化）。
- 在 **秘书模式** 下：
  - 全局 Sidebar（含移动端菜单按钮）不渲染
  - 默认入口指向秘书模式（例如首次打开即为 secretary，或 `/` 自动进入 `/secretary`；实现方案见 design）
  - deep-link 到高级页面时提供“切换到完全模式”的恢复路径（best-effort）
- 在 **完全模式** 下：
  - 恢复 Sidebar 与全量页面入口（Tasks/Governance/Ledger/Workflows…）
- 调整 Work Ledger “全局可见”badge 规则，使其与“秘书模式隐藏侧边栏”不冲突（见 spec deltas）。

## Impact
- Affected specs:
  - `app-shell-ux`（NEW）
  - `work-ledger-ux`（MODIFIED）
- Affected code (expected):
  - `frontend/src/App.vue`
  - `frontend/src/components/Sidebar.vue`
  - `frontend/src/components/ChatBox.vue`
  - `frontend/src/router/index.ts`
  - 新增：全局 UI mode store（Pinia）或等价机制

