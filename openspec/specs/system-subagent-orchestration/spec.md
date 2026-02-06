# system-subagent-orchestration Specification

## Purpose
Defines subagent orchestration patterns to reduce context pollution, including invocation decisions, context slicing, structured metadata, and findings-based handoffs.

## Requirements
### Requirement: agentic 子 Agent 启动决策 (Agentic Invocation Decision)
系统必须 (MUST) 允许主 Agent 自主判断某个工作是否适合拆分并启用子 Agent；系统不得 (MUST NOT) 强制任何任务必须使用子 Agent。

#### Scenario: 主 Agent 不启用子 Agent 仍可正常推进
- **GIVEN** 一个任务强耦合、需要频繁回看大量上下文
- **WHEN** 主 Agent 选择不启用子 Agent，直接在主上下文内完成工作
- **THEN** 系统不要求调用子 Agent（不会因为未调用而失败或阻塞）

#### Scenario: 主 Agent 启用子 Agent 执行独立步骤
- **GIVEN** 一个任务可以拆分为边界清晰、相对独立的步骤
- **WHEN** 主 Agent 选择启用子 Agent 执行其中一个步骤
- **THEN** 系统启动子 Agent 并返回该步骤的交付引用（见下述要求）

### Requirement: 子 Agent 调用 (Sub-Agent Invocation)
系统必须 (MUST) 提供一种机制，使主 Agent 可以为某个步骤启动一个子 Agent 执行，并获得“人类式短总结 + 可追溯引用”作为返回结果：
- `summary`：一句/几句短总结 + 下一步建议
- `findings_path`：指向子 Agent 交付件（findings 文件）
- `trace_log_path`：指向子 Agent 完整执行痕迹日志

#### Scenario: 主 Agent 启动子 Agent 并获得交付引用
- **GIVEN** 主 Agent 需要执行一个明确步骤（例如“实现后端 API 并补齐单测”）
- **WHEN** 主 Agent 调用子 Agent 执行该步骤
- **THEN** 系统返回的结果中包含 `summary`
- **THEN** 系统返回的结果中包含 `findings_path` 与 `trace_log_path`

### Requirement: Findings 文件交付 (Findings File Delivery)
系统必须 (MUST) 将子 Agent 的详细交付件写入一个 findings 文件（例如 `FINDINGS.md`）。该文件必须包含：
1) `## 流水账`
2) `## Findings`

并且该文件应该 (SHOULD) 包含：
- `## 变更文件`：列出新增/修改的文件路径（用于可追溯）

#### Scenario: 成功生成 findings 文件
- **GIVEN** 子 Agent 成功完成一次步骤执行
- **WHEN** 系统返回 `findings_path`
- **THEN** `findings_path` 指向的文件存在
- **THEN** 该文件包含 `## 流水账` 与 `## Findings`

#### Scenario: 返回给主 Agent 的汇报不回灌全量细节
- **GIVEN** 子 Agent 在执行过程中产生了大量中间过程与日志
- **WHEN** 系统向主 Agent 返回子 Agent 的结果
- **THEN** 返回内容只包含 `summary` 与必要的路径引用（例如 `findings_path`/`trace_log_path`）
- **THEN** 返回内容不包含子 Agent 的全量过程/日志或完整 findings 文件全文

### Requirement: 上下文切割与引用传递 (Context Slicing & Reference Carryover)
系统必须 (MUST) 将子 Agent 的输入上下文与主 Agent 隔离；默认仅向子 Agent 提供：
- 本步骤详细目标/约束/验收标准
- 前序步骤的“短总结 + findings 引用（路径）”

系统不得 (MUST NOT) 默认注入前序步骤的全量对话与工具过程。

#### Scenario: 第三步仅携带前两步摘要
- **GIVEN** 一个任务被分成 4 个步骤，且步骤 1/2 已完成并产生了短总结与 findings 引用
- **WHEN** 系统启动步骤 3 的子 Agent
- **THEN** 子 Agent 输入中包含步骤 1/2 的短总结与 findings 引用（路径）
- **THEN** 子 Agent 输入中不包含步骤 1/2 的完整对话与工具执行细节

### Requirement: 结构化元信息使用 XML（避免 JSON 强校验） (XML Structured Metadata)
系统不应 (SHOULD NOT) 要求子 Agent 以 JSON 作为交接输出/汇报格式（避免转义与强校验失败）。当确实需要结构化元信息（例如 `summary/findings_path/trace_log_path/run_id`）时，系统必须 (MUST) 支持使用 XML 进行表达。

系统应该 (SHOULD) 以“默认 CDATA”的方式处理 XML 节点内容：即系统不要求显式包裹 `<![CDATA[...]]>`，也能正确处理节点内的文本内容。

#### Scenario: XML 中无需显式 CDATA
- **GIVEN** 系统需要返回结构化元信息给主 Agent
- **WHEN** 系统返回 XML 交接（例如包含 `<subagent_handoff>`）
- **THEN** 系统返回的 XML 不要求主 Agent 或子 Agent 显式使用 `<![CDATA[...]]>` 才能正确解析/使用节点内容

### Requirement: 按步骤注入技能 (Per-Step Skill Injection)
系统应该 (SHOULD) 支持按子 Agent 步骤任务自动挑选相关技能（Top-K skills 的中文摘要），并将其写入子 Agent TurnContext（volatile）（不得回写稳定 system prompt，以保持 KV cache 友好）；同时系统必须 (MUST) 支持显式指定要注入的 skill（当自动召回不可用或用户有明确偏好时）。

#### Scenario: 翻译步骤注入 Translator 技能
- **GIVEN** 一个子 Agent 步骤任务是“将英文 README 翻译为中文”
- **WHEN** 系统为该步骤构建子 Agent TurnContext（volatile）的技能注入消息
- **THEN** TurnContext 中包含与翻译相关的技能摘要（例如包含 Translator skill 的名称与描述）
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

### Requirement: Toolkit-as-skill 选择子 Agent 工具集 (Toolkits via Skills)
系统必须 (MUST) 支持将 skill 作为“工具包技能（toolkit skill）”来决定子 Agent 的工具集合，以避免主上下文频繁变更 tool_ids 破坏 KV-cache，同时也避免子 Agent 默认挂载全量工具导致的上下文爆炸。

工具集选择顺序（best-effort）：
1) **显式 tool_ids**：若调用方提供 `tool_ids`，系统按其挂载（受 policy/approval gate 限制）。
2) **toolkit skill 推导**：若 `tool_ids` 为空但提供了 `skill_ids`，且这些 skills 的 frontmatter 声明了 `tool_ids`，系统应使用这些 `tool_ids` 作为子 Agent 工具集（受 policy/approval gate 限制）。
3) **继承父工具集**：若以上都为空，系统应默认继承父上下文已挂载的工具集合（排除 `subagent` 本身，防递归），而不是隐式扩张为全量工具。

#### Scenario: Toolkit skill infers tool_ids when omitted
- **GIVEN** 用户/主 Agent 调用子 Agent，未显式传 `tool_ids`
- **AND** 该调用显式传入 `skill_ids=["repo-toolkit"]`
- **AND** skill `repo-toolkit` 的 frontmatter 声明 `tool_ids=["rg","read_file"]`
- **WHEN** 系统启动子 Agent
- **THEN** 子 Agent 的工具集合包含 `rg` 与 `read_file`（best-effort，仍受 policy 限制）

#### Scenario: Omitted tool_ids inherits parent's mounted tools
- **GIVEN** 用户/主 Agent 调用子 Agent，未显式传 `tool_ids` 且未传入任何 toolkit skill
- **AND** 父上下文当前挂载工具集合为 `["read_file","rg","subagent"]`
- **WHEN** 系统启动子 Agent
- **THEN** 子 Agent 默认工具集合为 `["read_file","rg"]`（排除 subagent 本身；best-effort）

### Requirement: 限制与递归控制 (Limits & Recursion Control)
系统必须 (MUST) 对子 Agent 的执行施加限制，包括但不限于：最大递归深度、最大工具执行步数/时长、交接输出大小上限；并且默认禁止子 Agent 再次启动子 Agent（或限制在允许的最大深度内）。

同时系统应该 (SHOULD) 将“最大步数/时长/日志规模”的默认值设置得**偏大**（目标是长时间运行并交付可用结果，而非 demo），并允许通过配置覆盖。

建议默认值（可配置）：
- `max_steps`：>= 200
- `max_runtime_seconds`：>= 3600（1 小时）

#### Scenario: 子 Agent 尝试再启动子 Agent 被拒绝
- **GIVEN** 系统默认最大递归深度为 1
- **WHEN** 子 Agent 尝试启动另一个子 Agent
- **THEN** 系统拒绝该请求并返回清晰的错误原因（例如“subagent recursion is disabled”）

### Requirement: 默认工具权限与文件作用域 (Default Tools & File Scope)
系统必须 (MUST) 默认允许子 Agent 使用与主 Agent 相同的工具集合（包含文件读写/编辑/删除等工具），以支持生产级交付。

系统必须 (MUST) 支持对子 Agent 的“可写文件范围”进行限制（scope，glob 规则，基于 `<workspace>/` 的相对路径），并在文件工具层强制执行：超出 scope 的写/改/删请求必须被拒绝并返回可理解错误。

#### Scenario: 子 Agent 越界修改文件被拒绝
- **GIVEN** 当前会话启用 workspace，根目录为 `<workspace>/`，并且子 Agent scope 被限制为 `backend/**`
- **WHEN** 子 Agent 尝试修改 `frontend/App.vue`
- **THEN** 系统拒绝该写/改/删操作，并返回清晰错误（例如 “path is outside subagent scope”）

### Requirement: 子 Agent 可观测性与完整痕迹落盘 (Sub-Agent Observability & Full Trace Logging)
系统必须 (MUST) 记录子 Agent 执行的可观测数据，包括 parent→child 关联、输入摘要、输出短总结、耗时与模型信息，并以 `subagent` 类型的 trace 条目暴露。

系统必须 (MUST) 将子 Agent 的完整执行痕迹（消息、工具调用、工具结果、错误等）记录到文件中，并按日期与 session_id 分类存放；同时在 trace 中包含可定位到该文件的指针（例如 `trace_log_path`）。

#### Scenario: Trace 中包含 subagent 条目并关联 parent
- **GIVEN** 主 Agent 成功启动并完成一次子 Agent 执行
- **WHEN** 系统持久化本轮执行的 trace
- **THEN** trace 中存在 `type=subagent` 的条目
- **THEN** 该条目包含 parent 会话/调用的关联信息（例如 parent_session_id 或等价字段）

#### Scenario: 日志文件按日期与 session_id 分类可定位
- **GIVEN** 主 Agent 触发一次子 Agent 执行
- **WHEN** 子 Agent 执行结束
- **THEN** 系统在 `ONEAGENT_HOME/.oneagent/logs/subagent/YYYY-MM-DD/<session_id>/`（或等价可配置目录）下生成可定位的日志文件（jsonl）
- **THEN** 系统返回的 `trace_log_path` 指向该日志文件

### Requirement: plan.mark_done 结果写入 handoff (Plan Mark Done in Handoff)
系统必须 (MUST) 在子 Agent 调用 `plan.mark_done`（无论成功或失败）时，将“任务 id + done 成功/失败 + 失败原因（若有）”自动体现在其 handoff（summary/findings）中，便于主 Agent 决策下一步。

#### Scenario: handoff 包含标记 done 的结果
- **GIVEN** 子 Agent 执行完任务并尝试 `plan.mark_done`
- **WHEN** 子 Agent 返回 handoff 给主 Agent
- **THEN** handoff 中包含“任务 id + done 成功/失败 + 失败原因（若有）”的信息

### Requirement: Subagent MUST inherit and not exceed parent permissions
系统必须 (MUST) 确保子 Agent 的工具权限不超过父上下文；子 Agent 仅可使用父级允许的工具集合与约束。

#### Scenario: Subagent cannot escalate permissions
- **GIVEN** 父上下文不允许 `bash`
- **WHEN** 子 Agent 尝试调用 `bash`
- **THEN** 系统拒绝并返回权限错误
