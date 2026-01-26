# system-llm-prompt-caching Specification

## Purpose
TBD - created by archiving change optimize-kv-cache. Update Purpose after archive.
## Requirements
### Requirement: Prompt 结构必须缓存友好
系统必须 (MUST) 将 prompt 构建为“稳定前缀 (Stable Prefix)”与“动态上下文 (Volatile Context)”两段，并保证：
1) **稳定前缀不可变**：在同一会话内，稳定前缀的任意内容一旦生成，不得在后续轮次被“编辑/拼接/重写”（除非发生压缩/重建等结构性事件，见 `prompt_cache_key` 的 epoch）。
2) **动态上下文只追加不回写**：每轮变化的内容必须以“追加新消息”的方式加入，并放在稳定前缀之后。
3) **动态上下文不可缓存**：当 provider 需要显式标记 cacheable 段时，动态上下文不得被选入 cacheable 集合。

经验法则：**不要把每轮变化的内容写进任何可能被缓存的 system message**（不仅仅是第一条）。

#### Scenario: 动态注入不破坏稳定前缀
- **GIVEN** 会话基础 system prompt 与工具定义保持不变（稳定前缀）
- **AND** 每轮都会产生变化的动态上下文（如技能推荐/计划状态/observer 结果）
- **WHEN** 系统为多轮对话构建 prompt
- **THEN** 稳定前缀的内容在多轮之间保持字节级一致
- **AND** 动态上下文仅出现在稳定前缀之后
- **AND** 动态上下文不会被 cache selector 选为 cacheable（当 provider 需要显式标记时）

### Requirement: 动态模块必须使用 TurnContext（避免破坏 Stable Prefix）
系统必须 (MUST) 提供一个统一的 TurnContext 注入位置/格式，用于承载所有“每轮变化的模块化信息”，包括但不限于：
- skills 推荐结果
- plan 状态/当前任务 scope
- observer 验收结果
- subagent handoff 摘要（注意：只允许摘要 + 引用，不允许把完整 FINDINGS/trace 回灌到 prompt）

系统必须 (MUST) 确保这些信息不会以“修改稳定前缀”的方式注入（例如拼接到 system prompt 头部），以避免打碎 prefix cache。

#### Scenario: skills 推荐不打碎缓存
- **GIVEN** 两轮对话的 stable prefix 相同
- **AND** 两轮对话的 skills 推荐结果不同
- **WHEN** 系统构建两轮 prompt
- **THEN** skills 推荐信息仅出现在 TurnContext（volatile）中
- **AND** stable prefix 仍保持字节级一致

#### Scenario: subagent 不回灌完整 findings
- **GIVEN** subagent 产生了较大的 `FINDINGS.md`
- **WHEN** 主 Agent 在下一轮 prompt 中引用 subagent 结果
- **THEN** prompt 中只包含 subagent 的短总结与 `findings_path/trace_log_path` 引用
- **THEN** prompt 不包含完整 `FINDINGS.md` 正文（避免巨大且高度动态的内容进入前缀）

### Requirement: 显式标记型 provider 必须覆盖稳定长段（含压缩摘要）
当 provider 需要显式标记缓存段（如 `cache_control` / `cachePoint` / Anthropic prompt-caching）时，系统必须 (MUST) 确保“会话压缩摘要”等稳定且可能很长的段落进入 cacheable 集合。

#### Scenario: 压缩后摘要可缓存
- **GIVEN** 会话发生压缩且生成摘要消息 `summary`
- **WHEN** 下一轮请求使用显式标记型 provider 且启用缓存
- **THEN** 系统在请求中对摘要对应段落注入缓存标记（例如 `cache_control: {type:"ephemeral"}` 或 `cachePoint: {type:"ephemeral"}`）

### Requirement: Anthropic 工具块参与缓存（可配置）
当使用 Anthropic 且启用缓存时，系统必须 (MUST) 在被选为 cacheable 的消息上，对其内容块（包括 `tool_use` 与 `tool_result`）注入 `cache_control: {type:"ephemeral"}`（或在 provider 不支持该字段时保持兼容并记录降级原因）。

#### Scenario: 工具循环的末尾可缓存
- **GIVEN** Anthropic 工具调用产生 `tool_use` 与 `tool_result` 记录
- **WHEN** 系统选择“最近 N 条消息”为 cacheable 且启用缓存
- **THEN** `tool_use` 与 `tool_result` 对应内容块也被注入 cache_control（或被明确降级并可观测）

### Requirement: 缓存能力矩阵与回退策略
系统必须 (MUST) 维护显式的 provider 缓存能力矩阵，并覆盖系统已集成的所有 provider（不得只在部分 provider 下生效）。

系统必须 (MUST) 仅在 provider 声明支持时注入对应字段/头；当上游返回“不支持缓存字段/参数”错误时，系统必须 (MUST) 自动回退并向用户/日志提示。

#### Scenario: 不支持字段时自动降级
- **GIVEN** 某 provider 不支持 `prompt_cache_key`
- **WHEN** 用户启用缓存并触发一次 LLM 请求
- **THEN** 系统不会注入 `prompt_cache_key` 字段
- **OR** 若上游返回“不支持字段”错误，则系统自动关闭该字段并记录降级原因

### Requirement: 缓存指标必须可观测
系统必须 (MUST) 为每次 LLM 调用记录缓存相关指标，并至少包含：是否启用缓存、使用的 cache key（可脱敏/哈希）、以及在 provider 可提供时的 cached tokens/hit 信息；这些信息必须 (MUST) 能在 trace/log/UI 中被查看。

系统必须 (MUST) 将每次 LLM 调用的完整 request/response（含 messages）写入日志文件，并在 trace 中记录该日志文件路径指针（trace 只保存“摘要 + 指针”，避免回灌大体量 payload）。

#### Scenario: 用户可确认缓存是否生效
- **GIVEN** 用户启用 `enable_kv_cache`
- **WHEN** 触发一次对话请求并完成 LLM 调用
- **THEN** 用户可在 trace/UI 中看到本次调用的 `prompt_cache_enabled` 与缓存命中相关字段（若可得）

#### Scenario: trace 包含 LLM payload 日志指针
- **GIVEN** 一次 LLM 调用产生了完整 request/response（含 messages）
- **WHEN** 系统记录本次调用的 trace
- **THEN** trace 中包含该次调用对应的日志文件路径指针

### Requirement: prompt_cache_key 必须与会话稳定段绑定并支持 epoch
当 provider 支持 `prompt_cache_key` 时，系统必须 (MUST) 为同一会话在多轮之间复用 cache key，并在“稳定前缀发生结构性变化”（如模型切换、工具协议切换、会话压缩）时支持引入 `epoch` 以更新 cache key，避免不可解释的 miss 或潜在碰撞。

系统应该 (SHOULD) 将 cache key 与 stable signature 绑定（例如对 stable prefix 的规范化表示求哈希），以便在“工具集合/系统提示词结构变化”时可解释地更新 cache key。

#### Scenario: 模型切换触发 cache key 更新
- **GIVEN** 同一会话先后选择不同模型
- **WHEN** 发起新的 LLM 请求且启用 `prompt_cache_key`
- **THEN** 系统为新模型生成不同的 cache key（例如通过 `epoch` 或 stable signature 变化体现）

#### Scenario: 工具集合变化触发 cache key 更新
- **GIVEN** 同一会话的可用工具集合发生变化（例如启用/禁用某个工具或工具 schema 变化）
- **WHEN** 发起新的 LLM 请求且启用 `prompt_cache_key`
- **THEN** 系统更新 cache key（例如 `epoch` 递增或 stable signature 变化），避免沿用旧 key 导致不可解释的 miss

