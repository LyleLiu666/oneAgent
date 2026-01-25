# 设计：KV Cache（Prompt Caching）优化

> 本文档是对仓库现状的 review + 优化设计说明，重点回答：
> 1) 当前项目是否能充分利用 KV cache？
> 2) 哪些情况下会“用不好/用不上”？
> 3) 如何优化（面向未来能力也不退化）？

## 1. 术语与目标
### 1.1 术语
- **KV cache**：推理阶段对 attention key/value 的缓存，避免每个 token 反复重算。
- **Prompt caching / prefix caching**：将“相同前缀 prompt”的推理结果缓存，并在后续请求复用（本项目语境下把它统称为 KV cache）。

### 1.2 目标
- 提升缓存命中：减少重复 prompt 的算力消耗与首 token 延迟。
- 保持正确性：缓存仅是性能优化，不能改变模型输入语义。
- 可观测：能看到“缓存是否生效”，并能定位为什么没命中。
- 可维护：后续 skills/plan/subagent 引入动态注入时，不破坏缓存。

## 2. 现状实现（代码级 review）
### 2.1 开关与注入位置
- 模型配置：`enable_kv_cache`（Settings UI → backend model）
- chat handler：在生成请求前设置 `llm.ChatCompletionOptions.EnablePromptCache=true`，并在部分 provider 下设置 `PromptCacheKey=session_id`。

### 2.2 cache 选择策略
`backend/internal/llm/cache.go`：
- `cacheMessageIndexes()`：默认选择“前两条 system message + 最近两条 message”作为 cacheable。
- OpenRouter/Bedrock：在 message 上注入 `cache_control` / `cachePoint`。
- Anthropic：在 text content block 上注入 `cache_control`，并设置 `anthropic-beta` header。

### 2.3 关键正向点（已经做得比较好）
- **会话内稳定 key**：使用 `session_id` 作为 cache key（对支持 `prompt_cache_key` 的 provider），可在多轮间复用。
- **工具定义稳定**：tool registry/mount 与 tool spec 输出均为稳定排序，减少“仅顺序变化导致 cache miss”的抖动。
- **历史一致性**：构建 LLM history 时包含持久化的 tool_call/tool_result（避免下一轮 prompt 丢失工具上下文）。

## 3. 不能充分利用 KV cache 的场景（问题清单）
### 3.1 长会话压缩：摘要稳定但可能未进入显式缓存集合
会话压缩会插入一段较长 summary，后续多轮都重复携带。对于需要显式标记的 provider（Anthropic/OpenRouter/Bedrock），如果 summary 没被选入 cacheable（当前策略倾向只缓存 system 与尾部），则会反复重算 summary 这一大段前缀。

### 3.2 Claude 工具调用：tool_use/tool_result block 未被 cache_control 覆盖
当前 Anthropic payload 仅对 text block 注入 cache_control；在工具循环中：
- assistant 的 tool_use block
- user 的 tool_result block
往往恰好处于“下一轮会被复用”的位置，但未被 cache 标记 → 缓存收益被削弱。

### 3.3 动态 system prompt 注入：从根上打碎 prefix cache
未来能力（技能推荐、计划状态、observer 结果等）若把“每轮变化内容”追加到第一个 system message（或被标记 cacheable），将导致每轮都会改变 prompt 前缀，进而让后续全部历史无法命中 prefix cache。

经验法则：**动态内容放得越靠前、越会毁缓存**。

### 3.4 Provider 差异与配置漂移：缺少能力矩阵与回退
`prompt_cache_key`、`cache_control`、`cachePoint`、header 的支持并非一致；如果注入了不支持字段，可能出现：
- 请求失败（400/422）
- 静默忽略（看似开启缓存但毫无收益）

缺少“能力矩阵 + 断言测试 + 降级提示”，会导致缓存体验不稳定。

### 3.5 缺少“命中指标”：无法驱动优化与回归分析
没有 cached tokens/hit/miss 指标，就无法判断：
- 哪个 provider 在命中
- 哪条注入策略有效
- 哪次变更导致命中下降

## 4. 优化设计（建议落地策略）
### 4.1 Prompt Builder：把 prompt 结构化为 Stable/Volatile 两段
建议新增一个库层 Prompt Builder（后续 modules 复用）：
- 固定：`StableSystemPrompt`（不随轮次变化）
- 可选稳定段：`StableSessionSummary`（压缩摘要；若存在，视为稳定前缀）
- 动态段：`TurnContext`（技能推荐/计划状态/执行环境等）
- 末尾：当前 user message

注入原则：
- TurnContext 必须放到 stable prefix 之后（例如独立 user message 或非 cacheable system message）。
- 不在 stable prefix 上“原地编辑/拼接”动态内容。

### 4.2 Cache Selector：从“固定规则”升级为“策略”
默认策略（兼容现状）可以是：
- cache stable system prompt
- cache stable summary（若存在）
- cache tail N messages（用于下一轮）

对不同 provider：
- `prompt_cache_key` 型：继续用 key + prefix 复用为主
- 显式标记型：必须保证“摘要/稳定长段”进入标记集合

### 4.3 Anthropic tool blocks：扩展 cache_control 注入范围
在 Anthropic payload builder 中：
- 若某条 message 被选为 cacheable，则其 content blocks（包括 tool_use/tool_result）都应尝试注入 `cache_control: {type:"ephemeral"}`（或在不支持时保持兼容）。

### 4.4 prompt_cache_key：引入 cache epoch 与 key 版本
建议 key 结构化：
- `v1:<session_id>:<epoch>:<stable_signature_hash>`
其中 `epoch` 在“压缩/模型切换/工具协议切换”等破坏 prefix 的事件发生时递增；`stable_signature_hash` 用于避免不同 stable 前缀误用同一个 key（更安全、更可解释）。

### 4.5 可观测性：把缓存指标变成一等公民
最小指标集：
- 是否启用缓存
- 本次请求使用的 cache key（脱敏/哈希）
- cached tokens（若上游返回）
- hit/miss（若上游可得）

落点：
- trace（开发期）
- DB（可选，用于长期统计）
- UI 展示（用户可确认）

## 5. 与现有 change 的交互建议
- `enable-skills-usage`：避免把“推荐技能摘要（动态）”拼进第一条 system message；改为 TurnContext 段（不污染 stable prefix）。
- `enable-plan-observer-validation`：observer 结果/计划状态属于动态，应放在 TurnContext 段。
- `enable-subagent-orchestration`：handoff/findings 引用本质是稳定化策略，建议与 Prompt Builder 复用同一套“稳定段/动态段”规范。

