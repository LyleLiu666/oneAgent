## ADDED Requirements
### Requirement: Runtime and doctor MUST report formal memory enablement and gating state
当宿主接入 optional formal memory 时，系统必须 (MUST) 在 runtime health 与 doctor 诊断中暴露最小可用状态，以便排查“SDK 已接入但能力未生效”的问题。

至少必须包含：
- formal memory 是否启用
- formal memory store 是否已连接
- 当前 `MEMORYSDK_PRE_RECALL_POLICY`
- `MEMORYSDK_ENABLE_TOOLS` 是否启用
- `MEMORYSDK_ENABLE_TURN_END_JOBS` 是否启用

系统不得 (MUST NOT) 在 doctor 输出中泄露原始 DSN 或其他敏感凭据。

#### Scenario: doctor shows formal memory state without leaking DSN
- **GIVEN** 用户启用了 formal memory
- **WHEN** 用户执行 `oneagent doctor`
- **THEN** 输出包含 formal memory 是否启用、是否连接、pre-recall policy、tools 开关、turn-end jobs 开关
- **AND** 输出中不包含原始 Postgres DSN

### Requirement: Formal memory tools and turn-end jobs MUST have independent kill switches
系统必须 (MUST) 提供独立的 kill switch，允许在不关闭整个 formal memory store 的前提下，分别关闭主动 memory tools 与 turn-end jobs。

至少必须支持：
- `MEMORYSDK_ENABLE_TOOLS`
- `MEMORYSDK_ENABLE_TURN_END_JOBS`

#### Scenario: tools can be disabled while prerecall remains enabled
- **GIVEN** formal memory store 已启用
- **AND** `MEMORYSDK_PRE_RECALL_POLICY=auto`
- **AND** `MEMORYSDK_ENABLE_TOOLS=0`
- **WHEN** 系统处理一个 chat 回合
- **THEN** pre-recall 仍可执行
- **AND** 本轮不暴露 `memory.recall`、`memory.remember`、`memory.forget`

### Requirement: Formal memory host context MUST use stable opaque project identifiers
当宿主向 external formal memory 暴露 `project` scope 时，系统必须 (MUST) 使用稳定 opaque 的 project identifier，而不是直接把宿主 `workspace_root` 绝对路径作为 external scope ID。

系统必须 (MUST)：
- 保持同一 workspace 在同一宿主上的 project ID 稳定
- 默认不把宿主机绝对路径写入 external formal memory 的 project scope ID

#### Scenario: prerecall uses opaque project scope id
- **GIVEN** chat 当前 workspace 已设置
- **WHEN** 系统构建 formal memory prerecall host context
- **THEN** external `project` scope 使用稳定 opaque ID
- **AND** 该 scope ID 不等于原始 `workspace_root` 绝对路径

### Requirement: Pre-recall degradation MUST remain observable without breaking chat
当 formal memory pre-recall 失败或降级时，系统必须 (MUST) 保持 chat 主流程继续执行；如果当前 chat 开启 trace，系统还必须 (MUST) 对当前回合暴露降级提示。

#### Scenario: prerecall degradation surfaces a trace hint
- **GIVEN** formal memory prerecall 在当前回合发生降级
- **AND** 当前 chat trace 已开启
- **WHEN** 系统继续处理该 chat 回合
- **THEN** 用户仍收到正常的聊天结果
- **AND** 当前回合 trace 中包含 pre-recall 降级提示

### Requirement: Chat turn-end MUST enqueue formal memory extract jobs only after durable turn persistence
当宿主启用 formal memory turn-end jobs 时，系统必须 (MUST) 在 chat 回合已经完成 durable persistence 之后，才向 external `memorySdk` enqueue extract job。

系统必须 (MUST)：
- 在当前回合未被取消时触发
- 使用当前 session 作为 thread scope
- 提供稳定的 `turn_ref` 与 `runlog_ref`
- 默认不向 external formal memory store 持久化宿主机绝对路径
- 将 enqueue 失败视为 degrade，而不是让 chat 主流程失败

系统不得 (MUST NOT)：
- 在主聊天链路内同步执行重型 extract / consolidation
- 因 enqueue 失败而丢弃本轮已经生成并落盘的聊天结果

#### Scenario: successful chat turn enqueues extract job after persistence
- **GIVEN** formal memory store 已启用
- **AND** `MEMORYSDK_ENABLE_TURN_END_JOBS=1`
- **WHEN** chat 回合完成且本轮消息已落盘
- **THEN** 系统为当前 session/thread enqueue 一个 `extract_turn_candidates` job
- **AND** 该 job payload 至少包含稳定的 `turn_ref` 与 `runlog_ref`

#### Scenario: enqueue failure degrades without breaking chat
- **GIVEN** formal memory turn-end jobs 已启用
- **AND** external enqueue 在当前回合返回错误
- **WHEN** chat 回合已经生成并持久化 assistant 结果
- **THEN** 用户仍收到正常的聊天结果
- **AND** 系统仅记录 turn-end enqueue 失败的降级信息
