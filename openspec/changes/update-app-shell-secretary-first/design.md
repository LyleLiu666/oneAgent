## Context
我们已经有 Chat 层面的 `secretary/full` 两种 UI 呈现，但 app shell 仍按“登录后总是展示 Sidebar”设计，导致“低噪声秘书模式”无法成为真正的默认入口。

本变更聚焦 **UI 壳层（App Shell）**：把“秘书/完全”从 ChatBox 局部状态升级为全局状态，并让 Sidebar/导航遵循该状态。

## Goals / Non-Goals
- Goals
  - 默认只看到秘书模式（像微信聊天窗）
  - 只有切换到完全模式，才看到 Sidebar 与管理菜单
  - 模式切换可逆、可持久化、全局一致
  - deep-link 到高级页面时不“死路”：给出可操作恢复（best-effort）
- Non-Goals
  - 不在本变更中重做 Tasks/Governance/Ledger 页面本身的内容与信息架构
  - 不在本变更中实现“聊天驱动的任务化执行”（见后续 Phase S2）

## Decisions
- Decision: 全局 `ui_mode` 的来源与一致性
  - 建议：新增 `ui` Pinia store：`uiMode: 'secretary' | 'full'`
  - 与现有 `ChatBox` localStorage key `oneagent-chat-ui-mode` 保持兼容（迁移策略：读取旧 key → 写入新 store → 后续统一由 store 驱动）
- Decision: 路由策略（默认入口）
  - 方案 A（推荐）：`/` 根据 `ui_mode` 重定向：
    - secretary → `/secretary`
    - full → `/chat`（或现有 `/`）
  - 方案 B：不改路由，只把 ChatBox 默认 mode 改为 secretary，并让 App.vue 按 mode 隐藏 Sidebar
  - 取舍：A 更显式且符合“入口简单”，B 改动更小但语义更隐含
- Decision: deep-link 行为（secretary 下访问 full 页面）
  - 方案：保留 URL 可访问，但页面顶部展示一条低噪声提示：“此页属于完全模式”，并提供一键切换（best-effort）
  - 备选：直接重定向回 `/secretary`（更强硬，但可能让用户困惑“我点了链接怎么回去了”）

## Risks / Trade-offs
- 过度隐藏导致“找不到高级能力”：需要一个可发现但不打扰的“进入完全模式”入口（例如 Chat header 按钮、更多菜单、快捷键）。
- 兼容性：旧 localStorage key 已被使用；需要迁移而不破坏现有行为。

## Migration Plan
1) 引入全局 `ui_mode` store，并读取/迁移旧 key（无破坏）
2) App.vue 根据 `ui_mode` 渲染 Sidebar（秘书模式不渲染）
3) ChatBox 改为读写全局 store（不再各自维护）
4) 路由层补齐默认入口策略（如选择方案 A）
5) 补齐测试与 openspec validate

## Open Questions
- 是否要强制禁止 secretary 模式进入 `/tasks` 等页面（重定向），还是允许但提示？
- “进入完全模式”入口希望多隐蔽？（按钮是否需要放到“⋯更多”里更像微信？）

