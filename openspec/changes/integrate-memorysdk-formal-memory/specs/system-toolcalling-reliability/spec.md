## ADDED Requirements
### Requirement: Formal memory tools MUST only be exposed on chat JSON toolcalling, not XML fallback
当宿主启用了 formal memory tools 时，系统必须 (MUST) 仅在 chat 主 Agent 且实际使用 JSON/native tool calling 时向模型暴露 `memory.recall`、`memory.remember`、`memory.forget`。

系统不得 (MUST NOT) 在以下 surface 暴露这些工具：
- XML tool protocol fallback
- secretary 默认工具面
- subagent 继承工具面
- taskqueue 默认工具面
- 全局 `/tools` 列表或全局 registry

#### Scenario: JSON chat gets formal memory tools
- **GIVEN** formal memory store 已启用
- **AND** `MEMORYSDK_ENABLE_TOOLS=1`
- **AND** 当前 chat 回合最终协议为 JSON/native tool calling
- **WHEN** 系统组装本轮工具列表
- **THEN** 本轮可用 tools 包含 `memory.recall`、`memory.remember`、`memory.forget`

#### Scenario: XML fallback does not get formal memory tools
- **GIVEN** formal memory store 已启用
- **AND** `MEMORYSDK_ENABLE_TOOLS=1`
- **AND** 当前 chat 回合因 provider 能力不足回退到 XML tool protocol
- **WHEN** 系统组装本轮工具列表
- **THEN** 本轮不会暴露 `memory.recall`、`memory.remember`、`memory.forget`
- **AND** chat 主流程仍继续执行

### Requirement: Formal memory remember MUST require a stable tool call id
当模型通过 `memory.remember` 主动写入 candidate memory 时，系统必须 (MUST) 将稳定的 JSON tool call ID 作为 invocation id 传给 external bridge。

系统不得 (MUST NOT) 使用随机值、时间戳、payload hash 或递增计数器替代该 invocation id。

如果当前调用拿不到稳定的 tool call id，系统必须 (MUST) hard-fail，而不是降级生成弱幂等键。

#### Scenario: remember without stable tool call id hard-fails
- **GIVEN** formal memory tools 已启用
- **AND** 当前回合为 JSON/native tool calling
- **WHEN** `memory.remember` 执行时拿不到稳定的 tool call id
- **THEN** 系统返回明确错误
- **AND** 不写入 candidate memory

### Requirement: Formal memory tool semantics MUST be delegated to the external bridge
系统必须 (MUST) 直接复用 external `memorySdk/bridge/agentsdk` 的 formal memory tool 语义，而不是在宿主中复制一套 remember / forget / scope 校验逻辑。

宿主允许做的事情仅限于：
- 提供 host memory context
- 提供 invocation meta
- 决定当前 surface 是否挂载工具
- 记录日志、trace、health 与 doctor 状态

#### Scenario: host wrapper delegates to external bridge
- **GIVEN** 宿主已配置 formal memory
- **WHEN** chat 执行 `memory.remember` 或 `memory.forget`
- **THEN** 请求由 external bridge 统一处理
- **AND** 宿主不自行生成独立的 remember / forget 校验规则
