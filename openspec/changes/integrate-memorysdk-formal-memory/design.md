## Context

`oneAgent` 的愿景不是一个“只会聊天的助手”，而是一个工业级 agent 平台。为了走到这一步，宿主层和共享运行内核 / 共享能力层必须逐步拆开：

- 宿主层负责会话、权限、SSE、任务、秘书编排、最终 prompt 总装
- 共享层负责不同宿主都应该共用的能力

这次的 `memorySdk` 属于第二类。它不是本地 `memorydb` 的“高级版本”，而是另一层能力：正式记忆的召回、候选写入、失效与后台整理。

到本次设计更新时，Phase 1 已经完成：

- runtime 可选初始化 external `memorySdk`
- chat 每轮 pre-recall
- recall 结果注入 volatile `TurnContext`

这份设计稿除了记录已经落地的 Phase 1，也要把 Phase 2 的关键边界写实。否则下一步实现很容易把 memory tools 误接成全局工具，或者在宿主里再复制一套 bridge 规则。

## Goals

- 直接复用外部 `memorySdk` 的正式记忆能力，而不是在 `oneAgent` 里复制一套
- 不破坏当前 SU / SW 的本地共享记忆链路
- 让 chat 主 Agent 在 JSON/native tool calling 下可以主动使用 `memory.recall / remember / forget`
- 保持宿主只负责身份、scope、prompt 装配、trace 与开关治理
- 在进入 Phase 3 之前，就补齐最小 kill switch 与 health / doctor 可见性

## Non-Goals

- 本次不删除 `memorydb`
- 本次不把 secretary memory 迁移到 Postgres
- 本次不把 memory tools 注册进全局 registry
- 本次不为 memory tools 做 XML 兼容协议
- 本次不让 secretary / subagent / taskqueue 自动获得 memory tools
- 本次不在宿主层重写 `memorySdk` 的 remember / forget 校验规则

## Current Constraints From Code

当前代码里有 4 个结构性约束，决定了 Phase 2 不能用“最省事”的方式做：

1. `backend/internal/tool/registry.go` 是全局静态 registry。
   它不只影响 chat，还会影响 `/tools` 列表和其他工具入口。

2. `backend/internal/secretary/orchestrator.go` 的 `secretaryDefaultToolIDs(...)` 会基于当前可见工具集合生成 secretary 默认工具面。
   也就是说，只要某个工具进入全局可见面，secretary 就有机会看到它。

3. `backend/internal/tool/subagent.go` 会从父上下文继承 mounted tool IDs。
   也就是说，chat surface 上的工具如果不单独隔离，很容易被 subagent 被动继承。

4. `backend/internal/handler/chat.go` 的 `runToolLoop(...)` 已经掌握了 `tool_call_id` 和 `protocol`，也会把它们写进 trace。
   但当前 tool handler 自身还拿不到这些调用元数据。

再加上一个外部约束：

- `memorySdk` v1 官方画像明确要求 memory tools 只支持 JSON/native tool calling，不做 XML memory tool compatibility。

这几条叠加起来，意味着 Phase 2 必须显式回答 3 个问题：

- memory tools 到底在哪些 surface 可见
- `memory.remember` 的稳定 invocation id 从哪来
- XML fallback 时要怎么表现

## Decision 1: 保留双层 memory 边界，而不是替换

保留：

- `memorydb` = 秘书共享记忆，本地 SQLite append-only
- `memorySdk` = 正式记忆层，外部 Postgres + SDK bridge

原因：

- 这两类数据职责不同
- 现有 secretary 编排直接依赖 `AppendEntry` / `PullSync`
- 立刻替换会碰 SU / SW 主链路，迁移风险远高于收益

这一边界在后续阶段也保持不变：

- `memorydb` 是 secretary shared memory 的 source of truth
- `memorySdk` 是 formal memory 的 source of truth

## Decision 2: Phase 2 的 memory tools 只属于 chat 主 Agent，不进入全局 registry

Phase 2 的 memory tools surface 固定如下：

- chat 主 Agent，且本轮最终协议为 JSON/native tool calling：可见
- chat 主 Agent，但本轮最终协议为 XML：不可见
- secretary：不可见
- subagent：不可见
- taskqueue：不可见
- `/tools` 全局列表：不纳入

为什么不能直接注册进 `backend/internal/tool/registry.go`：

- 会自动进入 `tool.All()` / `tool.InfosWithSnapshot()`
- 会自动影响 `/tools`
- 会自动扩大 secretary 默认工具面
- 会给 subagent 继承制造额外泄漏风险

因此 Phase 2 的实现形态必须是：

- 先用现有 `tool.MountWithSnapshot(...)` 挂载用户选择的常规工具
- 再在 `backend/internal/handler/chat.go` 内，按条件追加 chat-local memory tool definitions
- 这些 definitions 只参与当前 chat 回合的 tool protocol 选择、prompt manual 和 JSON tool loop
- 不进入全局 registry，不进入 session metadata 的显式 `tool_ids`

额外约束：

- 传给 `tool.ContextWithMountedToolIDs(...)` 的“可继承工具集合”不能包含这些 chat-local memory tool IDs
- 否则 `subagent` 从父上下文继承时会拿到它们，破坏 surface 边界

## Decision 3: memory tools 采用“宿主上下文扩展 + bridge 直连”的方案

Phase 2 不重写 oneAgent 的工具体系，也不整体迁移到 `agentsdk.ToolCatalog`。

选择方案：

- 保留 oneAgent 当前 `Handler func(ctx context.Context, raw json.RawMessage)` 形态
- 新增一个轻量的 invocation meta context，例如 `tool.ToolInvocationMeta`
- 在 JSON tool loop 调用具体 handler 之前，把本次 tool call 的元信息塞进 `ctx`
- formal memory 的 tool handler 再从 `ctx` 中取出这些元信息，转交给 `memorySdk/bridge/agentsdk`

这个 meta 至少必须包含：

- `RunID`
- `TurnID`
- `ToolCallID`
- `ToolName`
- `Protocol`

来源固定为：

- `ToolCallID`：当前 JSON tool call 的稳定 ID
- `ToolName`：当前实际执行的 tool 名称
- `Protocol`：当前回合实际生效协议，Phase 2 只会是 `json`
- `RunID`：当前 chat 会话级 formal memory run，例如 `chat:<session_id>`
- `TurnID`：当前用户轮次的稳定 ID，贯穿 pre-recall、memory tool 调用、turn-end 生命周期

选择这个方案而不直接改 Handler 签名，原因有 3 个：

- 与 oneAgent 现有大量 `ContextWithX / XFromContext` 模式一致，改动最小
- 普通工具不需要跟着重写
- 可以把 memorySdk 所需的稳定元信息补齐，而不必重构整个宿主 runtime

## Decision 4: `memory.remember` 必须使用稳定的 tool call ID，缺失时 hard-fail

这是 Phase 2 最重要的行为约束。

`memory.remember` 的幂等性和 candidate ID 依赖：

- `run_id`
- `turn_id`
- `invocation_id`

其中 `invocation_id` 的官方来源，只能是 JSON/native tool calling 的稳定 tool call ID。

oneAgent 宿主必须满足：

- 不得自己生成随机 invocation id
- 不得用时间戳、计数器或 payload hash 冒充 invocation id
- 如果本次 `memory.remember` 调用拿不到稳定 `tool_call_id`，必须 hard-fail

这条规则的实际价值是：

- 避免同一轮重试时写出重复 candidate
- 避免宿主和 `memorySdk` 对“同一次 remember”有不同判断
- 避免后期治理和审计时找不到明确调用来源

## Decision 5: 直接复用 `memorySdk/bridge/agentsdk`，宿主不复制语义

Phase 2 必须继续守住“宿主只做接线，不复制正式记忆规则”的方向。

推荐实现形态：

- `backend/internal/formalmemory/service.go` 内部持有一个 `agentsdk.ToolCatalog`
- 启动时调用 `agentsdkbridge.Install(...)`，把 `memory.recall / remember / forget` 注册到这个 catalog
- oneAgent 自己新增的 memory tool definitions 只是薄包装
- 包装层负责：
  - 从 `ctx` 提取 host context 和 invocation meta
  - 组装 `agentsdk.ToolInvocation`
  - 调用 `catalog.Execute(...)`

oneAgent 不应在宿主层重写这些语义：

- input schema 校验
- removed field 拒绝规则
- remember scope allowlist 校验
- candidate id / idempotency key 规则
- forget 目标查找与 scope 校验

这些规则都已经属于 `memorySdk/bridge/agentsdk` 的正式合同，宿主不应再复制一份。

## Decision 6: 名称分层要显式区分“宿主内部 ID”和“SDK 对外工具名”

oneAgent 当前工具系统对内部 ID 和对外函数名有分层：

- 内部工具 ID 用于 mount、policy、env 开关、session metadata
- `Spec.Function.Name` 用于模型实际调用

Phase 2 对 memory tools 的推荐命名是：

- 内部 ID：
  - `memory_recall`
  - `memory_remember`
  - `memory_forget`
- 对外 JSON tool names：
  - `memory.recall`
  - `memory.remember`
  - `memory.forget`

这样做的原因：

- 对外继续对齐 `memorySdk` 官方工具名
- 宿主内部仍保留适合自身配置和别名系统的 canonical ID

对应需要同步处理两件事：

- `tool.CanonicalToolName(...)` 增加 dotted / underscore 互认
- tool dispatch 时，definition lookup 不能只按原始函数名匹配，必须兼容 canonical lookup

## Decision 7: 协议支持矩阵必须与 `memorySdk` v1 边界保持一致

`memorySdk` v1 官方边界已经写死：

- memory tools 只支持 JSON/native tool calling
- 不做 XML memory tool compatibility

因此 Phase 2 的协议矩阵固定如下：

| 能力 | chat JSON | chat XML | secretary | subagent | taskqueue |
| --- | --- | --- | --- | --- | --- |
| formal memory pre-recall | 支持 | 支持 | 暂不接 | 暂不接 | 暂不接 |
| `memory.recall` | 支持 | 不支持 | 不支持 | 不支持 | 不支持 |
| `memory.remember` | 支持 | 不支持 | 不支持 | 不支持 | 不支持 |
| `memory.forget` | 支持 | 不支持 | 不支持 | 不支持 | 不支持 |

当 provider 不支持 native tools，oneAgent 当前会把 chat tool loop 从 JSON 回退到 XML。

在这种情况下，Phase 2 的行为必须是：

- 继续保留 pre-recall
- 不挂 memory tools
- trace / log 明确记录“由于 XML fallback，本轮 formal memory tools 未启用”

不能做的事：

- 不能为了“看起来功能完整”在宿主里临时发明 XML memory tool schema
- 不能把 memory tool 输入拆成 XML fields 再喂给 bridge

## Decision 8: 双层 memory 写入矩阵必须固定，禁止隐式双写

Phase 2 和 Phase 3 都要遵守下面这张写入矩阵：

| 事件 / 数据 | `memorydb` | `memorySdk` | 主要读取方 | source of truth |
| --- | --- | --- | --- | --- |
| SU / SW 共享 append-only 记忆 | 写 | 不写 | secretary / SW | `memorydb` |
| chat pre-recall | 不写 | 只读 recall | chat 主 Agent | `memorySdk` |
| `memory.recall` | 不写 | 只读 recall | chat 主 Agent | `memorySdk` |
| `memory.remember` | 不写 | 写 candidate | chat / worker | `memorySdk` |
| `memory.forget` | 不写 | 写 invalidate / delete_candidate | chat / worker | `memorySdk` |
| turn-end extract job | 不写 | 写 jobs | worker | `memorySdk` |

显式禁止：

- 不做 `memorydb -> memorySdk` 自动镜像
- 不做 `memorySdk -> memorydb` 反向同步
- 不用“双写”掩盖职责不清

这条规则的意义是，后续只要看到某类数据写到了两个 memory 系统里，就知道那是设计偏了。

## Decision 9: 最小 observability 和 kill switch 必须前移到 Phase 2

为了避免“看起来接了，实际线上没开”这种问题，Phase 2 就必须落地最小 operator surface。

### Kill Switch

新增独立开关：

- `MEMORYSDK_ENABLE_TOOLS`
- `MEMORYSDK_ENABLE_TURN_END_JOBS`

现有配置继续沿用：

- `MEMORYSDK_POSTGRES_DSN`
- `MEMORYSDK_PRE_RECALL_POLICY`

### health / doctor

至少暴露下面这些信息：

- formal memory 是否启用
- formal memory store 是否已连接成功
- 当前 pre-recall policy
- memory tools 是否启用
- turn-end jobs 是否启用

要求：

- 不能输出原始 DSN
- 只能输出布尔、策略值、状态摘要

### 日志 / trace

宿主至少要保留这些桥接事件：

- `memory_prerecall_degraded`
- `memory_tool_rejected`
- `memory_write_committed`

并补充这些上下文：

- `tool_call_id`
- `protocol`
- `scope_kind`
- `scope_id`
- `candidate_id` 或 `target_id`

如果 bridge event sink 写失败：

- 不得影响主工具结果
- 但 warning 必须能在日志或结构化结果里看见

## Alternatives Considered

### Alternative A: 直接把 memory tools 注册进全局 registry

不选。

原因：

- 会自动扩大 secretary / subagent / `/tools` 的可见面
- 风险不是“多改几个 if”，而是把 surface 边界彻底打散

### Alternative B: 整体把 oneAgent tool runtime 迁移到 `agentsdk.ToolCatalog`

Phase 2 不选。

原因：

- oneAgent 当前仍有自己的 tool registry、权限包装、workspace 上下文和 XML loop
- 一步到位迁移会把当前“补 formal memory 合同”这个任务膨胀成“重做宿主工具 runtime”

### Alternative C: 为 memory tools 临时补一套 XML 兼容层

不选。

原因：

- 与 `memorySdk` v1 官方画像冲突
- 宿主会重新背上一套临时协议维护成本

## Runtime Shape

### Config

现有：

- `MEMORYSDK_POSTGRES_DSN`
- `MEMORYSDK_PRE_RECALL_POLICY`

新增：

- `MEMORYSDK_ENABLE_TOOLS`
- `MEMORYSDK_ENABLE_TURN_END_JOBS`

### Formal Memory Service

`backend/internal/formalmemory` 继续作为薄适配层：

- 打开 `memorySdk/store/postgres`
- 启动时执行 migrations
- 安装 `memorySdk/bridge/agentsdk`
- 构建 host memory context
- 当存在 workspace 时，为 external `project` scope 生成稳定 opaque ID，而不是直接把宿主绝对路径作为 scope 主键
- 记录 bridge event
- 暴露：
  - `PreRecallTurnContext(...)`
  - Phase 2 所需的 tool execution helper
  - Phase 3 所需的 turn-end helper

### Chat Integration

chat 请求进入后，推荐执行顺序：

1. 解析 session / workspace / model / base tools
2. 生成当前 formal memory `RunID / TurnID`
3. 先做 pre-recall，并把 recall 结果注入 volatile `TurnContext`
4. 如果 `MEMORYSDK_ENABLE_TOOLS=1` 且最终协议是 JSON，则追加 chat-local memory tool definitions
5. 进入 JSON tool loop 时，把 invocation meta 注入 `ctx`
6. memory tool wrapper 通过 external bridge 执行

### Turn-End Integration

Phase 3 前不做 inline formal promotion。

Phase 3 只允许：

- none
- enqueue extract job

不允许：

- 在 chat 主链路里同步做重型 consolidation

### Turn-End Payload Contract

Phase 3 的最小 turn-end payload 必须保持“宿主只提供稳定引用，不复制 formal memory 语义”的边界。

宿主至少应提供：

- `turn_ref`
- `runlog_ref`
- `run_id`
- `turn_id`
- `session_id`
- `tool_protocol`
- `status`

可选提供：

- `agent_id`
- `error`

其中：

- `turn_ref` 必须稳定对应当前 chat turn，例如 `chat:<session_id>:<turn_id>`
- `runlog_ref` 必须稳定对应当前 turn 的 host runlog 引用，例如 `runlog:<run_id>:<turn_id>`
- turn-end payload 默认不写入宿主机绝对路径（例如 workspace / logs / sessions 路径），避免把本地目录结构泄露到 external formal memory store

这样做的原因：

- 后续 worker / host adapter 可以继续围绕稳定 ref 扩展，而不是把绝对路径写死成协议核心
- external formal memory store 不应承担宿主机目录结构泄露风险

### Turn-End Trigger Point

turn-end enqueue 的推荐触发点固定为：

- 当前用户消息已经成功落盘
- 本轮 assistant 侧结果（正常回复、tool loop 结果或 error bubble）已经完成持久化
- 当前回合未被用户取消
- 请求即将结束，但主链路已不再依赖 formal memory enqueue 结果

这样可以保证：

- background extract job 看到的是 durably persisted turn，而不是半成品
- turn-end 失败不会把主聊天回复拖死
- JSON / XML 两条 chat 协议都能共享同一 turn-end 入口

## Failure Strategy

- 未配置 `MEMORYSDK_POSTGRES_DSN`：formal memory 视为未启用，不影响启动
- 已配置但连接 / 迁移失败：启动失败，避免“明明说接了，实际没接上”
- 单轮 pre-recall 失败：由 SDK degrade，聊天继续；如果 trace 已开启，则向当前 chat trace 暴露降级提示
- `memory.remember` 缺少稳定 `tool_call_id`：hard-fail
- provider 回退到 XML：本轮 memory tools 不可用，但聊天继续
- turn-end enqueue 失败：只记录日志 / trace，聊天继续
- bridge event sink 写失败：主调用继续，但 warning 要可见

## Rollout / Rollback

建议 rollout 顺序：

1. 上线 Phase 2 代码，但 `MEMORYSDK_ENABLE_TOOLS=0`
2. 先验证 doctor / health / pre-recall / JSON protocol gating
3. 再打开 `MEMORYSDK_ENABLE_TOOLS`
4. Phase 3 再单独引入 `MEMORYSDK_ENABLE_TURN_END_JOBS`

建议 rollback 顺序：

1. 关 `MEMORYSDK_ENABLE_TOOLS`
2. 关 `MEMORYSDK_ENABLE_TURN_END_JOBS`
3. 如有需要，把 `MEMORYSDK_PRE_RECALL_POLICY` 设为 `none`
4. 最后才移除 `MEMORYSDK_POSTGRES_DSN`

回滚时不做：

- 不删除 `memorydb`
- 不清理 Postgres formal memory 数据
- 不修改 secretary 当前共享记忆链路

## Testing

最小验收覆盖应扩展为：

- 未配置 formal memory 时，runtime / chat / doctor 正常工作
- pre-recall 在 JSON / XML 两条聊天协议下都能继续工作
- JSON 模式且 `MEMORYSDK_ENABLE_TOOLS=1` 时，chat 会挂上 3 个 memory tools
- XML 模式或 JSON fallback 到 XML 时，不会挂 memory tools
- `memory.remember` 缺少稳定 `tool_call_id` 时 hard-fail
- memory tool wrapper 直接复用 external bridge 规则，而不是宿主自写一套校验
- memory tools 不进入 secretary / subagent 默认 surface
- health / doctor 能看出 formal memory、pre-recall policy、tools、turn-end jobs 的状态
- `MEMORYSDK_ENABLE_TURN_END_JOBS=1` 且 turn 成功持久化后，会 enqueue extract job
- turn-end enqueue 失败不会打断 chat 主流程
- `openspec validate integrate-memorysdk-formal-memory --strict --no-interactive` 通过

## Phase 2 Readiness

在这次设计补齐之后，Phase 2 的 readiness gates 应为：

- Completeness: PASS
- Traceability: PASS
- Executability: PASS
- Ambiguity: PASS
- Safety: PASS

前提是后续实现严格遵守这份设计稿，不把 memory tools 注册进全局 registry，也不在宿主里再复制一套 XML 或 remember 规则。
