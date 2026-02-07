# system-secretary-orchestration Spec Delta

## MODIFIED Requirements

### Requirement: The system MUST provide a triage API that batches messages into a single secretary report and dispatches workers (best-effort)
系统必须 (MUST) 提供一个 triage（归并/派工）机制（best-effort），用于对“自上次 triage 以来的消息集合”生成一条低噪声的秘书汇报，并在需要时将可执行工作派发为后台 worker（Task Queue tasks）。

系统应该 (SHOULD) 优先走“最短可闭环路径”（best-effort）：
- 若 SU **判断/预计**大概率可在约 `<=5` 次只读工具调用内闭环（该数字仅用于决策阈值，**非硬限制**），则系统不应 (SHOULD NOT) 创建/入队任何后台 tasks（best-effort），而应直接写入一条用户可读的汇报消息（best-effort）。
- 仅当需求涉及写/改/跑（对用户资产产生变更）或预计超出简单 tool loop 预算时，系统才应派发后台 tasks（best-effort）。
- SU 若选择自办，可根据需要继续使用只读工具；实际 tool loop 的上限由系统全局 max-steps 控制（best-effort）。

系统必须 (MUST) 在连续追问场景中继承上一轮上下文（best-effort）：
- triage prompt 必须包含上一轮可追溯锚点（如最近 triage summary / pending questions / recent semantic anchors），避免“失忆式”重问（best-effort）。
- 当检测到“短回复 + 存在 pending questions”时，系统应该 (SHOULD) 优先按“回答上轮问题”解释（best-effort）。

系统必须 (MUST) 对 triage prompt 上下文执行预算控制（best-effort）：
- 预算上限为 `80,000 runes`（与会话压缩阈值一致）。
- 超限时优先压缩历史锚点并保留关键上下文；并在上下文中标注省略计数（best-effort）。

#### Scenario: Triage directly replies without dispatch for a small read-only request (best-effort)
- **GIVEN** 用户提出一个**预计**可在约 `<=5` 次只读工具调用内闭环的查询/解释请求（best-effort；决策阈值，非硬限制）
- **WHEN** 客户端调用 triage API（best-effort）
- **THEN** 系统写入一条 `role=assistant,type=text` 的秘书回复消息（best-effort）
- **AND** 系统不创建/不 enqueue 任何后台 tasks（best-effort）
- **AND** 系统更新 triage cursor，使后续 triage 仅处理新增消息（best-effort）

#### Scenario: Follow-up reply inherits previous triage context (best-effort)
- **GIVEN** 上一轮 triage 产生了 `questions[]`（best-effort）
- **WHEN** 用户下一轮给出短回复（例如“在 user home 下面找一下”）（best-effort）
- **THEN** triage prompt 包含上一轮 pending question 与语义锚点（best-effort）
- **AND** 系统不应先给出泛化反问（best-effort）

#### Scenario: Triage prompt budget is capped with omission markers (best-effort)
- **GIVEN** 历史上下文较长，若全量注入会超过 `80,000 runes`（best-effort）
- **WHEN** 系统构建 triage prompt（best-effort）
- **THEN** 系统压缩历史锚点并保留关键上下文（best-effort）
- **AND** prompt 中包含省略计数标记（best-effort）

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
- **GIVEN** SU 通过只读工具完成直答闭环（best-effort）
- **WHEN** triage 完成（best-effort）
- **THEN** SU 对用户的回复可见（best-effort）
- **AND** SW 不产生额外的 chat messages（best-effort）
- **AND** 系统仍保留可追溯证据（例如 triage run 记录、policy snapshot hash 等；best-effort）

### Requirement: Secretary MUST provide home-rooted readonly search first, then layered expansion when needed (best-effort)
当会话未绑定 workspace 且用户意图是“查找目录/文件/仓库路径”时，系统必须 (MUST) 采用分层只读搜索策略（best-effort）：
- 默认搜索根为当前用户 `home`（best-effort）。
- home 未命中时，自动按阶段扩搜：`home -> common-dev -> whitelist`（best-effort）。
- 每个阶段都应有超时与遍历预算上限，避免长时间阻塞（best-effort）。
- 默认排除系统/隐藏目录与高噪声目录（如 `.git`、`node_modules`）；仅在用户明确要求时才放开系统深搜（best-effort）。

#### Scenario: Home-first search injects default readonly root (best-effort)
- **GIVEN** 会话未绑定 workspace，且用户请求“找项目目录”（best-effort）
- **WHEN** 系统构建 triage prompt（best-effort）
- **THEN** prompt 包含 `default_readonly_search_root=<user_home>`（best-effort）

#### Scenario: Search expands beyond home when no home candidate (best-effort)
- **GIVEN** home 阶段没有命中候选目录（best-effort）
- **WHEN** 系统继续执行分层搜索（best-effort）
- **THEN** 系统自动进入 `common-dev` 或 `whitelist` 阶段（best-effort）
- **AND** 返回候选路径而不是先反问用户（best-effort）

#### Scenario: Hidden/system directories are excluded by default (best-effort)
- **GIVEN** 默认搜索策略启用（best-effort）
- **WHEN** 系统执行目录遍历（best-effort）
- **THEN** 以 `.` 开头的隐藏目录与系统目录默认不进入候选扫描（best-effort）
