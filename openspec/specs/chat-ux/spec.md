# chat-ux Specification

## Purpose
TBD - created by archiving change add-secretary-mode-chat. Update Purpose after archive.
## Requirements
### Requirement: Chat UI MUST provide “Secretary Mode” (low-noise)
系统必须 (MUST) 在 Chat UI 中提供一种“秘书模式”以降低默认信息噪声：当用户只想与助手对话时，界面仅保留对话所需的最小要素，其余低频/高级内容默认折叠但可恢复。

秘书模式至少满足：
- 仍可正常发送消息并接收回复
- 默认隐藏低频/高级区域（例如历史侧栏、模型/工具选择、trace/工具细节、任务面板等，best-effort）
- 提供可发现的一键入口切换回完整模式
- 支持通过独立路由直接进入秘书模式（例如 `/secretary`，best-effort），且完整模式仍可从全局菜单进入（best-effort）

#### Scenario: Secretary mode remains usable for chat
- **GIVEN** 用户已进入 Chat 的秘书模式
- **WHEN** 用户发送一条消息
- **THEN** 系统返回助手回复（best-effort）
- **AND** 发送流程不依赖任何高级区域处于展开状态

#### Scenario: User can switch between secretary mode and full mode
- **GIVEN** 用户在秘书模式中
- **WHEN** 用户选择切换到完整模式
- **THEN** 之前被折叠的低频/高级区域变为可见（best-effort）
- **WHEN** 用户再次切换回秘书模式
- **THEN** 低频/高级区域再次折叠（best-effort）

#### Scenario: Secretary mode choice persists across reload
- **GIVEN** 用户已开启秘书模式
- **WHEN** 用户刷新页面或重新打开应用
- **THEN** Chat 仍以秘书模式呈现（best-effort）

#### Scenario: Secretary mode can be entered via dedicated route
- **GIVEN** 系统提供秘书模式独立路由（例如 `/secretary`）
- **WHEN** 用户访问该路由
- **THEN** 页面以秘书模式呈现（best-effort）
- **AND** 用户仍能通过全局菜单进入完整模式（best-effort）

### Requirement: Secretary mode MUST support multi-message sending with quick acks and batched triage replies (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下支持“微信式连续发送”：用户可以连续发送多条消息，而不被“assistant 正在生成中”的状态阻塞；系统应该 (SHOULD) 在用户每次发送后尽快给出一个低噪声的快速确认（quick ack，**由 LLM 生成的短句**，best-effort）；系统应该 (SHOULD) 在短暂静默窗口后对“自上次归并以来的消息集合”输出一条低噪声的秘书式汇报（best-effort），而不是逐条进行完整分析回复。

#### Scenario: User can send multiple messages without being blocked
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户在短时间内连续发送多条消息
- **THEN** UI 允许每条消息都被发送并显示在对话中（best-effort）
- **AND** 系统不会因为 assistant 正在生成而阻止用户继续发送（best-effort）

#### Scenario: Each message gets a quick ack before triage
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户发送一条消息
- **THEN** UI 在短时间内展示一条 quick ack（由 LLM 生成的短句，best-effort）
- **AND** 该确认不要求等待后续 triage 汇总完成（best-effort）

#### Scenario: Quick ack is replayable after refresh (traceable)
- **GIVEN** 用户在秘书模式下发送了一条消息并收到 quick ack（best-effort）
- **WHEN** 用户刷新页面或重新进入会话（best-effort）
- **THEN** 对话中仍可看到同样的 quick ack 内容（best-effort）

#### Scenario: Secretary produces a single batched reply for a message burst
- **GIVEN** 用户处于秘书模式并连续发送了多条消息（best-effort）
- **WHEN** 用户停止输入并产生短暂静默窗口（best-effort）
- **THEN** 系统输出一条秘书式汇报，覆盖这批消息的归并理解（best-effort）
- **AND** 汇报可包含“正在推进的工作线/已派发的后台任务/需要用户确认的问题”（best-effort）

### Requirement: Secretary mode MUST notify in chat when a background task completes (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，在后台任务完成/失败时以低噪声方式通知用户（best-effort），以强化“微信心智”的确定性，并减少用户去任务工作台查看的心智负担。

通知策略至少包括（best-effort）：
- 仅对“本次进入页面后发生的状态跃迁”提示（避免首次加载刷屏）
- 当任务 latest attempt 从 `queued/running` 进入终态时，追加一条助手消息提示“已完成/已失败 + 交付已更新”

#### Scenario: No history replay on first load
- **GIVEN** 用户进入秘书模式并加载任务列表
- **AND** 存在一些历史已完成任务
- **WHEN** 首次渲染完成
- **THEN** 系统不回放历史完成通知（best-effort）

#### Scenario: Chat notifies when a running task finishes
- **GIVEN** 某任务 latest attempt 处于 `running`
- **WHEN** 下一次轮询中该任务进入终态（例如 `succeeded` 或 `failed`）
- **THEN** 对话区追加一条助手通知（best-effort）

### Requirement: Secretary mode MUST suggest background task handoff for long-running requests (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，对“明显是长任务/需要交付物”的消息给出低噪声引导（best-effort），优先建议交给后台任务队列执行，并允许用户选择“仍作为即时聊天发送”（best-effort）。

#### Scenario: Secretary suggests handoff on send for long tasks
- **GIVEN** 用户处于秘书模式（best-effort）
- **AND** 用户输入一条“看起来是长任务”的消息（best-effort）
- **WHEN** 用户点击 Send
- **THEN** 系统展示一个低噪声确认，让用户选择 `交给后台` 或 `作为聊天发送`（best-effort）

### Requirement: Secretary mode MUST provide a chat receipt after successful task handoff (best-effort)
系统必须 (MUST) 在秘书模式下，当用户将消息 handoff 成后台任务且创建成功时，在对话区追加一个“回执”以留痕（best-effort），至少包括：
- 用户消息（原始输入）
- 助手回执（确认已交给后台，交付物会出现在交付区）

#### Scenario: Handoff appends a receipt to the chat history
- **GIVEN** 用户处于秘书模式且 handoff 创建任务成功（best-effort）
- **WHEN** 系统完成创建
- **THEN** 对话区出现一条用户消息与一条助手回执（best-effort）

### Requirement: Secretary mode MUST surface low-noise recovery actions for failed tasks (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示“需要处理”的失败任务，并提供可恢复动作（best-effort），避免用户在“只聊天”心智下错过失败与下一步。

至少包括（best-effort）：
- 失败任务的提示（基于 Task Queue 中终态且非 `succeeded` 的 attempt）
- 一键继续（`POST /api/tasks/:id/resume`）
- 一键进入完全模式排障（例如跳转任务工作台）

#### Scenario: Failed task is surfaced in secretary mode
- **GIVEN** `GET /api/tasks` 返回至少 1 个任务，其 latest attempt 处于失败终态（非 `queued/running` 且非 `succeeded`）
- **WHEN** 用户处于秘书模式并停留在对话主界面
- **THEN** 页面展示该任务的低噪声“需要处理”提示（best-effort）

#### Scenario: User can resume a failed task from secretary mode
- **GIVEN** 失败任务提示已展示（best-effort）
- **WHEN** 用户点击“继续”
- **THEN** 系统调用 `POST /api/tasks/:id/resume` 并将该任务重新入队（best-effort）

#### Scenario: User can enter full mode troubleshooting from secretary mode
- **GIVEN** 失败任务提示已展示（best-effort）
- **WHEN** 用户点击“排障”
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到任务工作台页面（例如 `/tasks`，best-effort）

### Requirement: Secretary mode MUST surface low-noise deliverable cards for completed task artifacts (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式呈现 Task Queue 已完成任务的“可交付产物入口”（best-effort），且交付卡片范围仅覆盖任务产物（Task artifacts），不引入额外的管理系统信息结构。

至少包括（best-effort）：
- 展示最近完成的 task attempt（终态）对应的交付卡片（限量 N 个以控制噪声）
- 每张卡片提供 artifacts 的可点击入口（例如 findings/diff/test_report/trace）
- 用户点击 artifact 后可在秘书模式内预览其内容（best-effort），无需切换到完全模式

#### Scenario: Deliverable cards appear for completed tasks
- **GIVEN** `GET /api/tasks` 返回至少 1 个任务，其 latest attempt 处于终态（非 `queued/running`）
- **AND** 该 attempt 存在至少一个可用 artifact（best-effort）
- **WHEN** 用户处于秘书模式并停留在对话主界面
- **THEN** 页面展示对应的 deliverable card（best-effort）

#### Scenario: User can preview a task artifact from a deliverable card
- **GIVEN** deliverable card 已渲染且包含 findings artifact（best-effort）
- **WHEN** 用户点击 findings 入口
- **THEN** 系统调用 task artifact 预览接口并展示内容（best-effort）
- **AND** 不切换到完全模式（best-effort）

### Requirement: Secretary mode MUST support handing off a message to Task Queue (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下提供一种低噪声方式，将“长任务”从即时对话中手动交给后台任务队列执行（best-effort），避免用户为了入队而切回管理系统外观。

至少包括（best-effort）：
- 在输入区可发现的 handoff 动作（仅在秘书模式出现）
- handoff 使用当前输入作为 task `prompt` 创建任务（`POST /api/tasks`）
- 成功后清空输入并给出低噪声确认；失败时保留输入并给出可解释错误

#### Scenario: Handoff action is visible in secretary mode
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户在输入框中输入非空内容
- **THEN** 页面展示一个“交给后台/入队任务”的低噪声动作（best-effort）

#### Scenario: Handoff creates a task without sending chat
- **GIVEN** 用户处于秘书模式且输入框非空（best-effort）
- **WHEN** 用户触发 handoff 动作
- **THEN** 系统创建一个后台任务，其 `prompt` 等于输入内容（best-effort）
- **AND** 系统不发起 chat stream（best-effort）

### Requirement: Secretary mode MUST surface low-noise task queue hints (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式提示 Task Queue 的关键运行状态（best-effort），避免用户在“只聊天”时遗漏后台执行进度。

至少包括（best-effort）：
- 活跃任务数量提示（`GET /api/tasks` 中 `attempt.status in {queued,running}` 的任务数）

#### Scenario: Task hint is visible in secretary mode when active tasks > 0
- **GIVEN** `GET /api/tasks` 返回至少 1 个活跃任务（`queued` 或 `running`）
- **AND** 用户处于秘书模式（best-effort）
- **WHEN** 用户停留在对话主界面
- **THEN** 页面展示一个低噪声 Task badge（best-effort）

#### Scenario: User can enter full mode from the task hint
- **GIVEN** 用户处于秘书模式且 Task badge 可见（best-effort）
- **WHEN** 用户点击该 badge
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到任务工作台页面（例如 `/tasks`，best-effort）

### Requirement: Secretary mode MUST surface low-noise status hints (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下，以低噪声方式展示关键状态提示（best-effort），避免用户在“只聊天”时错过可治理/可收割的事项。

至少包括（best-effort）：
- SOP 待治理数量（`GET /api/ledger/status/today.sop_proposed_count`）

#### Scenario: SOP badge is visible in secretary mode when proposed count > 0
- **GIVEN** `GET /api/ledger/status/today` 返回 `sop_proposed_count > 0`
- **AND** 用户处于秘书模式（best-effort）
- **WHEN** 用户停留在对话主界面
- **THEN** 页面展示一个低噪声 SOP badge（best-effort）

#### Scenario: User can enter full mode from the hint
- **GIVEN** 用户处于秘书模式且 SOP badge 可见（best-effort）
- **WHEN** 用户点击该 badge
- **THEN** 系统切换到完全模式（best-effort）
- **AND** 跳转到 SOP 治理页面（例如 `/governance/sop`，best-effort）

### Requirement: Chat streaming MUST be resilient to reload (best-effort)
当用户刷新页面或网络短暂断开导致 SSE 中断时，系统必须 (MUST) 确保本次回复生成不会因此被动停止（best-effort）；系统应该 (SHOULD) 允许用户重新打开页面后继续观察该 session 的流式输出（best-effort）。

#### Scenario: Refresh does not stop generation
- **GIVEN** 用户在 session S 中触发了一次 assistant 回复生成（处于流式返回中）
- **WHEN** 用户刷新页面导致 SSE 连接断开
- **THEN** 后端生成过程不应因为 SSE 断开而被动停止（best-effort）
- **AND** 该回复最终应落盘到该 session 的历史消息中（best-effort）

#### Scenario: Client can re-attach after reload (best-effort)
- **GIVEN** session S 存在进行中的流式输出
- **WHEN** 用户刷新页面后重新进入 Chat UI
- **THEN** UI 应自动尝试重新 attach 到 session S 的进行中 stream（best-effort）
- **AND** UI 应继续展示该回复的流式输出（best-effort）

### Requirement: Chat UI MUST provide an explicit Stop button that discards the current reply
当 assistant 正在生成回复时，Chat UI 必须 (MUST) 提供可发现的“停止”按钮；停止不应依赖刷新页面或网络波动等副作用。

当用户主动停止时，系统应将本次 assistant 回复视为被丢弃（discard；best-effort）：不应写入会话历史。

#### Scenario: Stop cancels generation and discards assistant reply
- **GIVEN** assistant 正在为当前 session 生成回复（流式中）
- **WHEN** 用户点击 “停止”
- **THEN** 前端向后端发起 stop/cancel 请求（best-effort）
- **AND** 后端停止进一步生成（best-effort）
- **AND** 系统不应将本次 assistant 回复写入会话历史（discard；best-effort）
- **AND** UI 进入非流式状态，允许用户继续发送新消息（best-effort）

#### Scenario: Stop affects all viewers of the same session
- **GIVEN** 同一 session 在多个浏览器标签页/窗口中被同时打开且正在生成回复
- **WHEN** 任意一个页面点击 “停止”
- **THEN** 该 session 的本次回复生成应被停止（best-effort）
- **AND** 其它页面的流式输出也应尽快停止并进入非流式状态（best-effort）

### Requirement: Secretary mode MUST surface in-flight tool progress
When a tool call is running, Chat UI MUST show a compact “working” indicator even in secretary mode, so users can see the LLM is still working.

#### Scenario: Secretary mode shows tool progress while tool call is in-flight
- **GIVEN** 用户处于秘书模式
- **WHEN** 系统正在执行至少一个 tool call
- **THEN** UI 必须显示一个紧凑的“执行中”提示
- **AND** 提示至少包含工具名或工具数量（best-effort）

#### Scenario: Progress indicator shows streaming response token count
- **GIVEN** 系统正在流式返回 assistant 消息
- **WHEN** 前端收到 response token 统计
- **THEN** “执行中”提示应展示 token 计数（best-effort）

