# system-agent-factory Spec Delta

## ADDED Requirements

### Requirement: The system MUST provide an Agent Factory to build agents from config (best-effort)
系统必须 (MUST) 提供一套可复用的 **Agent Factory（build-agent）**（best-effort），用于用声明式 config 构建不同职责的 agent（例如 worker chat / secretary / subagent / worker task agent），以避免每个 agent 各自手写“拼 prompt/选协议/parse 输出”的重复逻辑。

Agent Factory 的 config 至少应覆盖（best-effort）：
- `system_prompt_assets`（稳定前缀的模块化资产）
- `tool_protocol`（`none|json|xml`）
- `tool_ids`（挂载哪些 tools；含是否挂载 `subagent`）
- `policy_snapshot`（工具权限策略快照：允许哪些工具/动作；默认 fail-closed）
- `workspace_config`（是否提供 workspace、是否允许文件变更；best-effort）
- `skills`（是否启用 skills/recall/read-gating；best-effort）
- `persistence_policy`（message list 留痕策略：text/tool_call/tool_result/trace pointers；append-only，best-effort）

#### Scenario: Factory builds different agents with different tool protocols
- **GIVEN** 一个 worker chat agent 需要启用 tools 且 provider 支持原生 tools（best-effort）
- **WHEN** 系统用 Agent Factory 构建该 agent（best-effort）
- **THEN** 该 agent 使用 `tool_protocol=json` 并进入 tool loop（best-effort）
- **GIVEN** 一个 secretary agent 选择使用 XML `<tool_data>`（best-effort）
- **WHEN** 系统用同一套 Factory 构建该 agent（best-effort）
- **THEN** 该 agent 使用 `tool_protocol=xml` 并复用 XML engine（best-effort）

#### Scenario: Secretary agent is configured as “no file mutation” via policy (best-effort)
- **GIVEN** Secretary agent 被配置为“可以查询/排障，但禁止对用户目录做增删改”（best-effort）
- **WHEN** 系统用 Agent Factory 构建该 agent（best-effort）
- **THEN** tool permissions 以 `policy_snapshot` 强制执行（fail-closed，best-effort）
- **AND** 当模型尝试调用被禁止的文件变更工具时，系统以 `tool_result` 将失败原因反馈给模型，以便其自行换方案或改为派工（best-effort）

### Requirement: Agent Factory MUST preserve KV-cache friendly stable prefix (best-effort)
系统必须 (MUST) 确保由 Agent Factory 构建的 agent 在启用 KV-cache 时满足“稳定前缀可缓存”的约束（best-effort）：
- stable prefix 仅由 system prompt assets 与稳定配置决定（best-effort）
- 易变内容（skills 推荐摘要、observer snapshot、subagent handoff summary 等）不得污染 stable prefix（best-effort）
- 易变内容应进入 TurnContext（volatile）并可被 cache selector 排除（best-effort）

#### Scenario: Volatile context does not change prompt_cache_key
- **GIVEN** 两轮对话的 agent config（system prompt assets / tool_protocol / tools schema）相同（best-effort）
- **AND** 两轮对话的技能召回结果不同（best-effort）
- **WHEN** Agent Factory 为两轮构建 prompt 并启用 KV-cache（best-effort）
- **THEN** 两轮的 `prompt_cache_key`（或等价稳定签名）保持一致（best-effort）
- **AND** skills 推荐信息仅出现在 TurnContext（volatile）中（best-effort）

### Requirement: Agent Factory MUST support structured outputs without forcing plain-text JSON (best-effort)
系统必须 (MUST) 支持 agent 产出结构化决策/计划时的可靠通道（best-effort），并不得 (MUST NOT) 通过“要求模型仅输出纯文本 JSON”作为唯一方式（该方式易导致降智与 parse 失败）。

结构化输出应优先使用：
1) 原生 tool calling（function call）返回结构化字段（best-effort），或
2) 宽松 tags 协议（例如 XML-like tags；best-effort）并允许 best-effort 修复/容错解析。

#### Scenario: Secretary triage plan uses a structured-output channel
- **GIVEN** Secretary SW 需要产出 triage plan（summary/tasks/questions；best-effort）
- **WHEN** 系统调用该 agent（best-effort）
- **THEN** 结构化输出通过 tool-call 或宽松 tags 返回（best-effort）
- **AND** 系统能将其解析为内部 plan 结构并继续流程（best-effort）
