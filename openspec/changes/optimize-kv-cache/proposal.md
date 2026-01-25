# 优化 KV Cache（Prompt Caching）利用率 (Optimize KV Cache Utilization)

## 摘要 (Summary)
当前项目已支持按模型开关 `enable_kv_cache`，并在 `backend/internal/llm` 内按 provider 注入不同的 Prompt Cache（KV cache）提示（`prompt_cache_key` / `cache_control` / `cachePoint` / header）。但在一些关键路径与场景下，KV cache 可能无法被充分命中或被部分绕过（尤其是长会话压缩、Claude 工具调用、以及未来“动态系统提示词注入”类能力）。

本提案目标是在**不改变对话正确性**的前提下，系统化提升 KV cache 的命中率、可观测性与可维护性，形成一个“缓存友好”的 Prompt 构建与注入规范，供后续 skills/subagent/plan 等能力复用。

## 动机 (Motivation)
1. **降低延迟与成本**：多轮对话、工具循环与长会话场景中，prompt 复用度高，KV cache 命中可显著降低首 token 延迟与推理成本。
2. **避免后续能力引入的回归**：skills/plan/subagent 等特性倾向把动态内容塞进 system prompt；如果缺乏规范，会在不知不觉中把缓存“打碎”。
3. **可诊断性缺失**：当前缺少“缓存是否生效”的指标与日志，导致“看起来开了缓存但实际上没命中/没注入”的问题难以定位。
4. **Provider 差异大**：Anthropic/OpenRouter/Bedrock 的缓存标记与 OpenAI-compat 的 `prompt_cache_key` 语义不同，需要统一抽象与测试覆盖，避免出现某些 provider 下缓存形同虚设。

## 现状 (Current State)
以当前仓库代码为基线：
- `enable_kv_cache` 存在于模型配置，并在 chat handler 内映射为 `llm.ChatCompletionOptions.EnablePromptCache`（并按 provider 选择是否填充 `PromptCacheKey=session_id`）。
- `backend/internal/llm/cache.go` 通过 `cacheMessageIndexes()` 默认标记：
  - 前两条 system message
  - 最近两条 message
- Anthropic：设置 `anthropic-beta: prompt-caching`，并在 `buildAnthropicPayload()` 的 text block 上按索引注入 `cache_control: {type: "ephemeral"}`。
- OpenRouter/Bedrock：按索引在 message 上注入 `cache_control` / `cachePoint`。
- OpenAI-compat / Responses：发送 `prompt_cache_key`（当前以 `session_id` 作为 key）。
- DeepSeek（或其它“自动前缀识别”类 provider）：无需客户端显式标记，但仍强依赖“稳定前缀不变”的 prompt 结构才能获得收益。
- `backend/internal/handler/session_compress.go` 会在上下文过长时生成摘要并重建 prompt（summary 作为 assistant 文本消息插入）。

## 主要问题与风险 (Problems / Risks)
### 1) 长会话压缩后，缓存标记可能无法覆盖“摘要”这段稳定大前缀
压缩摘要通常会在后续多轮内保持稳定，但当前缓存索引策略仅覆盖“前两条 system + 最近两条”，导致对 Anthropic/OpenRouter/Bedrock 这类“显式标记型缓存”而言，摘要不一定进入可缓存集合，从而反复消耗。

### 2) Claude 工具调用场景的缓存覆盖不完整
当前 `buildAnthropicPayload()` 仅对 text block 注入 `cache_control`，而 tool_use/tool_result block 未覆盖；在工具循环/工具结果较大时，会显著削弱缓存收益。

### 3) 动态 system prompt 注入会直接破坏 cache 友好前缀
未来提案（skills/plan/subagent）普遍会把“动态上下文”追加到 system prompt；若放在 prompt 前缀（或被标记为 cacheable），每次变动都会导致缓存失效，且对所有 provider 都是高损。

### 4) Provider 能力与字段支持差异：需要“明确能力表 + 回退策略”
不同 OpenAI-compat provider 对 `prompt_cache_key` 的支持并不一致；同理，header / cache marker 的字段名也有差异。缺少明确的能力矩阵与测试，容易出现“开启缓存=发送不被支持字段=请求失败/静默忽略”的情况。

### 5) 缺少可观测性：无法量化“缓存命中率”
当前 trace/DB 记录未包含 cached tokens、cache hit、或“是否注入缓存标记”的可视化信息，难以驱动优化与回归分析。

## 建议方案 (Proposed Solution)
### 1) 引入“缓存友好 Prompt 构建”规范与抽象
新增一个可复用的 Prompt Builder/Policy（库层能力）：
- 明确区分：
  - **Stable Prefix**：会话内稳定、适合缓存的内容（基础 system prompt、静态工具规范、压缩摘要等）
  - **Volatile Context**：每轮变化的内容（技能推荐、计划状态、动态环境信息等）
- 约束注入顺序：把 Volatile Context 放在“缓存断点之后”（例如作为 user message 或非 cacheable system message），避免污染 stable prefix。
- 对未来变更（skills/plan/subagent）给出统一落点：默认不要把动态内容写入第一条 system message。

### 2) 调整缓存标记选择策略（Cache Selector）
将 `cacheMessageIndexes()` 从“固定前 2 system + 后 2”升级为策略化选择（默认策略仍保持简单），并至少覆盖：
- 基础 system prompt（稳定）
- 长会话压缩摘要（稳定且可能很长）
- 最近 N 条消息（用于下一轮复用）

### 3) 完善 Anthropic 工具块缓存覆盖
在 Anthropic payload 构建时，将 cache_control 的注入扩展到 tool_use/tool_result block（或提供可配置策略），在工具循环中提升缓存收益。

### 4) 统一 Provider 缓存能力表与回退策略
引入显式的 provider capability 矩阵，并覆盖所有已集成 provider（不能只在部分 provider 下生效）：
- 是否支持 `prompt_cache_key`
- 是否支持 message-level `cache_control`
- 是否支持 `cachePoint`
- 是否需要/支持特定 header（如 Anthropic beta header）

对不支持的字段：不注入、或在检测到上游返回“不支持字段”错误时自动回退（并在 UI/日志提示）。

### 5) 增加可观测性（Metrics & Tracing）
在 LLM call trace / 日志中新增并展示：
- `prompt_cache_enabled`（是否开启）
- `prompt_cache_key`（可脱敏/哈希）
- `cached_prompt_tokens`（若 provider 返回）
- `cache_hit`/`cache_write`（若 provider 支持）
并将每次 LLM 调用的完整 request/response（含 messages）写入日志文件（替代原先入库的做法）；trace 仅保存指标摘要与日志指针。前端提供最小可视化（例如 Trace 面板），让用户能确认“缓存真的在工作”。

## 影响范围 (Impact)
- 后端：
  - `backend/internal/llm`：cache policy、Anthropic payload、provider capability、metrics 抽取
  - `backend/internal/handler/chat.go` / `session_compress.go`：prompt builder 接入、摘要缓存策略、cache key 策略
  - trace/log：缓存指标落 trace，完整 LLM request/response（含 messages）落日志文件，并在 trace 中记录指针
- 前端：
  - Settings/Trace：展示缓存开关与命中指标（最小必要展示）
- 文档：
  - 补充“缓存友好 Prompt 写法”最佳实践，避免后续能力破坏缓存

## 成功标准 (Success Criteria)
- 在启用 `enable_kv_cache` 且多轮对话/工具循环场景下：
  - 平均首 token 延迟降低（相对基线）
  - cached tokens 指标可见（至少对支持的 provider）
  - 长会话压缩后的后续轮次仍可获得明显缓存收益
- 引入 skills/plan/subagent 等动态注入能力时，不出现“缓存命中率断崖式下降”的回归。

## 默认约定 (Defaults)
1. `cache epoch`：默认启用，用于在会话压缩/模型切换/工具协议切换等“稳定前缀结构变化”事件发生后更新 cache key，提升可预测性并避免不可解释的 miss。
2. 日志落盘：LLM payload 默认采用“按调用单文件 JSON”（位于 `ONEAGENT_HOME/.oneagent/logs/llm/YYYY-MM-DD/<session_id>/<llm_call_id>.json`），trace 记录指标摘要与日志指针；日志默认保留 30 天（可配置），启动时 best-effort 清理超期目录。
