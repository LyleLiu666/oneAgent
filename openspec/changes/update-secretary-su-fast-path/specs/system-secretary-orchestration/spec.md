# system-secretary-orchestration Spec Delta

## MODIFIED Requirements

### Requirement: The system MUST provide a triage API that batches messages into a single secretary report and dispatches workers (best-effort)
系统必须 (MUST) 提供一个 triage（归并/派工）机制（best-effort），用于对“自上次 triage 以来的消息集合”生成一条低噪声的秘书汇报，并在需要时将可执行工作派发为后台 worker（Task Queue tasks）。

系统应该 (SHOULD) 优先走“最短可闭环路径”（best-effort）：
- 若 SU 在 `<=5` 次只读工具调用预算内可以直接拿到证据并完成回复，则系统不应 (SHOULD NOT) 创建/入队任何后台 tasks（best-effort），而应直接写入一条用户可读的汇报消息（best-effort）。
- 仅当需求涉及写/改/跑（对用户资产产生变更）或预计超出简单 tool loop 预算时，系统才应派发后台 tasks（best-effort）。

#### Scenario: Triage directly replies without dispatch for a small read-only request (best-effort)
- **GIVEN** 用户提出一个可在 `<=5` 次只读工具调用内闭环的查询/解释请求（best-effort）
- **WHEN** 客户端调用 triage API（best-effort）
- **THEN** 系统写入一条 `role=assistant,type=text` 的秘书回复消息（best-effort）
- **AND** 系统不创建/不 enqueue 任何后台 tasks（best-effort）
- **AND** 系统更新 triage cursor，使后续 triage 仅处理新增消息（best-effort）

#### Scenario: Triage dispatches workers when mutation or long-running work is required (best-effort)
- **GIVEN** 用户提出需要写/改/跑或长时交付物的需求（best-effort）
- **WHEN** 客户端调用 triage API（best-effort）
- **THEN** 系统写入一条低噪声的秘书汇报消息（best-effort）
- **AND** 系统创建并 enqueue 一个或多个后台 tasks 作为 worker（best-effort）

### Requirement: Secretary orchestration MUST be split into SU/SW channels with distinct responsibilities (best-effort)
系统必须 (MUST) 将秘书编排划分为两个逻辑通道（双分身；best-effort）：
- `Secretary(User)`（SU）：面向用户对话与汇报；默认只读；可在预算内使用只读工具并直接完成小请求（best-effort）
- `Secretary(Work)`（SW）：面向执行层协调 worker 的事件/恢复/答疑；默认不面向用户（best-effort）

系统必须 (MUST) 将用户输入路由给 SU，将 worker 的事件/提问路由给 SW（best-effort）。

当 SU 已经完成直答闭环（无派工）时，系统不应 (SHOULD NOT) 将该次过程性信息写入 SW 的 chat message list（best-effort）；系统应通过系统留痕（例如 triage run 记录与可追溯元数据）保证可归因与可追溯（best-effort）。

#### Scenario: SU direct answer does not write into SW chat message list (best-effort)
- **GIVEN** SU 通过只读工具在预算内完成直答闭环（best-effort）
- **WHEN** triage 完成（best-effort）
- **THEN** SU 对用户的回复可见（best-effort）
- **AND** SW 不产生额外的 chat messages（best-effort）
- **AND** 系统仍保留可追溯证据（例如 triage run 记录、policy snapshot hash 等；best-effort）

