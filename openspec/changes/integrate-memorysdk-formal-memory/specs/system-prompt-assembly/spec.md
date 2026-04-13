## ADDED Requirements
### Requirement: Formal memory pre-recall MUST be injected as volatile TurnContext when enabled (best-effort)
当宿主启用了 `memorySdk` formal memory 能力时，系统必须 (MUST) 在聊天主循环的每轮开始前执行 pre-recall（best-effort），并把 recall 结果作为 volatile `TurnContext` 注入，而不是并入 stable prefix。

该注入必须 (MUST) 满足：
- recall 结果只影响本轮上下文
- recall 文本不进入 stable prefix
- recall scope 由宿主提供 host context，再交由 `memorySdk` bridge 统一处理

#### Scenario: pre-recall recall items are appended as TurnContext
- **GIVEN** 当前会话启用了 formal memory pre-recall（best-effort）
- **AND** `memorySdk` 为当前 host context 返回了 recall items（best-effort）
- **WHEN** 系统组装本轮 prompt
- **THEN** recall 摘要只出现在本轮 `TurnContext` 中（best-effort）
- **AND** stable prefix 不包含这些 recall 摘要

### Requirement: Formal memory pre-recall degradation MUST NOT break the main chat turn (best-effort)
当 `memorySdk` pre-recall 在单轮执行时失败或降级时，系统必须 (MUST) 继续主聊天流程（best-effort），而不是因为 formal memory 不可用就中断本轮回复。

#### Scenario: pre-recall degrade does not fail the chat turn
- **GIVEN** 当前会话启用了 formal memory pre-recall（best-effort）
- **AND** `memorySdk` 在本轮 pre-recall 期间返回 degraded 结果或内部错误（best-effort）
- **WHEN** 系统继续组装并发送本轮 prompt
- **THEN** 本轮聊天仍继续执行（best-effort）
- **AND** recall 失败不会被并入 stable prefix 或变成启动级硬错误（best-effort）
