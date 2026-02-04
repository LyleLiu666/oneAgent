# 秘书（Secretary）作为 Agent 的设计复盘：从“路由器”到“中层管理者”

> 目的：对齐 `openspec/project.md` 的北极星（放权与信任 + 可追溯过程），重新梳理“秘书应该如何 agentic”，并明确哪些实现细节仍在拖后腿（尤其是：关键字路由、严格纯文本 JSON、KV-cache 破坏、把工程错误直接抛给用户）。

---

## TL;DR（结论）

- **秘书必须是 agent**：能自主调用工具、能自愈、能派工、能解释下一步；而不是“计数器/模板机/关键词路由器”。
- **代码不应该做语义路由**：不要在 orchestrator 里写一堆 `strings.Contains(...)` 来判断“用户在问什么”。意图判断应交给 SW（Secretary(Work)）planning agent，通过结构化输出（tool-call / 宽松 tags）表达 `intent`。
- **结构化输出不靠纯文本 JSON**：优先 tool-call；fallback 用宽松 tags（XML-like，不要求 CDATA），避免 “invalid function arguments json” 这类脆弱失败。
- **KV-cache 以稳定前缀为原则**：稳定 system prompt / tool schema 由 AgentFactory 装配；每轮变化信息只进入 TurnContext（volatile）或 append-only 的新消息，不重写历史。
- **所有过程必须可追溯**：用户默认看到“像人一样的汇报”，但随时能展开 evidence（trace/findings/diff/test_report/llm_log_path）；并且 **秘书自己的 LLM 调用也要能定位**（否则 debug 只能靠猜）。

---

## 1. 这是一个 agent 系统：角色与边界

按照项目定义（`openspec/project.md`），系统里不是“一个 chat assistant”，而是多个 agent 的协作系统：

| 角色 | 面向谁说话 | 是否可用工具 | 是否能写用户目录 | 主要职责 |
|---|---|---:|---:|---|
| **Worker Chat（完全模式）** | 用户 | ✅ | ✅（受 policy/approval gate） | 交互式做事，带 tool loop |
| **Secretary(User) = SU** | 用户 | ✅（通常不需要） | ❌（默认不写） | 解释、确认、放权、交付入口、下一步 |
| **Secretary(Work) = SW** | 执行层/系统 | ✅ | ❌（默认不写） | 归并意图、派工、排障、恢复、自愈 |
| **Worker Task（TaskQueue/subagent）** | 系统 | ✅ | ✅（受 scope/rollback/证据要求） | 真正干活，产出 artifacts |

关键点：
- **完整模式**：用户与 Worker Chat 对话。
- **秘书模式**：用户与 SU 对话；SU 背后由 SW 进行 planning 与编排，必要时派发 Worker Tasks。
- **“秘书能不能用工具”不是核心争议**：核心是 **“秘书不应该靠代码路由做智力替代”**，而应通过 **agent loop + 工具 + 结构化输出** 自主判断。

---

## 2. agentic 的判定标准（在 oneAgent 的语境里）

把“agentic”落到可实现/可验收的标准，避免抽象口号：

1) **自主（Autonomy）**：在权限边界内，能自己决定下一步（查、问、派工、等待、恢复）。
2) **工具优先（Tools-first）**：遇到不确定事实，先通过工具获取证据，而不是靠猜或逼用户反复补信息。
3) **自愈（Self-heal）**：工具调用失败/输出解析失败，先把错误以 `tool_result` 反馈给模型，允许它修复参数/换路再试（受 max steps 限制）。
4) **留痕（Append-only traceability）**：消息列表是事实来源；重要过程有 trace/log pointers；不靠“我记得”。
5) **KV-cache 友好（Stable prefix）**：稳定前缀字节级稳定；易变内容只进入 volatile TurnContext 或追加消息。
6) **预算与降级（Budget & degrade）**：秘书应有明确的“自处理上限”（例如 ≤5 轮 tool loop / ≤N 秒 / ≤N tokens）。超过预算或涉及写入时，自动降级为派工（Worker Task）或引导切换到完整模式。

---

## 3. 当前实现已经对齐愿景的部分（值得保留）

下面这些点，已经从“上个世纪路由器”向“真正的 agent”迈出了关键一步：

### 3.1 秘书规划不再强依赖纯文本 JSON

SW planning 现在通过统一的结构化输出通道产生 triage plan：
- 优先 tool-call（function calling）返回 `intent/summary_message/tasks/questions`
- fallback 宽松 tags 解析（XML-like）

代码入口：
- `backend/internal/agent/structured_output.go`：结构化输出（tool-call → tags）与失败 fallback
- `backend/internal/secretary/orchestrator.go`：SW 使用 AgentFactory + structured output 产出 triage plan

### 3.2 KV-cache 基础设施：稳定前缀 + TurnContext（volatile）

稳定前缀由 AgentFactory 统一装配，易变信息（任务看板快照、memory sync）放在 TurnContext：
- `backend/internal/agent/factory.go` / `runtime.go`
- `backend/internal/llm/cache.go`：`ChatMessage.Volatile` 参与 cache selector（避免污染稳定前缀）

SW 的“内部 session”也遵守 append-only：压缩通过追加 summary 消息而不是重写历史（利于 cache epoch 管理与可追溯）。

### 3.3 “不问蠢问题”的方向是对的：用确定性进度快照兜底

秘书在 planning 前会生成一份确定性的“任务看板快照”（来自 TaskQueue store），并注入给 SW：
- 用户问进度时，SW 应优先用快照回答，不要反问 workspace/任务是哪一个（在只有一个工作线时尤其明显）。

实现入口：
- `backend/internal/secretary/orchestrator.go`：`buildProgressReply(...)` + `progressSnapshot` 注入

---

## 4. 仍然不够 agentic 的点（Gap & 反模式）

> 重点：这里不是“挑实现细节”，而是指出 **会让模型降智/卡死/反问** 的结构性原因。

### 4.1 反模式：用关键字/规则在代码里做意图判断

典型症状：
- 用户问“任务完成得怎么样”，系统先走字符串判断分支，进入错误的处理逻辑；
- 由于规则覆盖不全，最后只能反问“你指的是哪个任务/哪个 workspace？”；
- 模型被迫在一堆 guardrail 中找出路，表现出“降智”。

正确方向：
- 意图判断必须由 **SW planning agent** 统一完成（同一套输入：用户新消息 + 任务看板快照 + 可用工具）。
- 代码层只保留 **安全边界与幂等性**：session module boundary、max tool steps、policy deny、幂等 cursor、append-only。

### 4.2 工具挂载仍然“碎片化”：秘书 vs worker 的工具/权限不一致

现在 Worker Chat、Worker Task、Secretary 各自维护工具集合与权限策略，容易导致：
- 同类错误在不同 agent 上重复踩坑（输出格式、解析失败、权限差异导致“看不到/做不了”）。

正确方向：
- **工具集合与权限策略必须配置化**，并通过 AgentFactory/preset 统一装配：
  - `tool_ids`：挂哪些工具
  - `tool_policy`：允许/禁止哪些效果（例如禁止写文件）
  - `workspace_config`：是否给 workspace（秘书默认禁用）
  - `subagent_enabled` / `skills_enabled`：是否挂载

### 4.3 反模式：把工程错误直接当作“需要用户确认”

典型错误：
- “invalid function arguments json”
- “invalid observer output (expected XML or JSON)”

这类错误对用户没有意义，应该先交给 agent loop 自愈：
- 将错误作为 `tool_result` 或 “repair user message” 反馈给模型，让它修复结构/改用 tags；
- 达到重试上限后，才把**可行动**的信息呈现给用户（例如：让用户点开 trace、贴出 error + request_id）。

### 4.4 “放权与信任”落地还不够：SU 的汇报需要更一致的证据指针

目前 SU 的总结更自然了，但仍需要保证：
- 对“需要处理/失败”的条目，默认给出一个 **可点击的 evidence 入口**（trace/findings/diff/test_report/llm_log_path）。
- 不改变 message list 的前提下，尽量把“关键指针”放进可恢复的 state 或 work ledger 里（渐进披露）。

### 4.5 关键缺口：秘书的“工具执行”与“低噪声留痕”的边界需要写清楚

这里容易产生误解：**“不要把工具细节塞进用户对话”** ≠ **“秘书不能执行工具”**。

更 agentic、也更符合“放权与信任 + 可追溯过程”的边界应该是：
- SU（用户看到的 message list）保持低噪声：默认只追加 `role=assistant,type=text` 的自然语言总结。
- SW（内部 session / trace / ledger）可以保留 tool_call/tool_result 与中间决策，作为可追溯证据（append-only）。
- 工具失败必须回注给 LLM（`tool_result`/repair prompt）做自愈；不要把工程错误升级为“需要用户确认”。

同时，这也意味着：如果现有 spec 写了 “triage MUST NOT execute tools”，更准确的表述应是：
- **MUST NOT 污染 SU 对话**（不出现 tool_call/tool_result），而不是禁止 SW 在 planning 时做只读查询。

### 4.6 关键缺口：秘书侧的 LLM 可观测性仍不一致（难 debug）

当前 Worker Chat 的 LLM 调用具备较完整的可观测性（例如 `llm_log_path`、KV-cache key hash、cacheable indexes、tool_protocol 等）。
但秘书（SW/SU）的 LLM 调用如果缺少同等级的记录，会导致：
- 用户明明点了 trace/留痕，但定位不到“是哪次模型调用/哪套 prompt/哪个 tool protocol”导致的问题；
- 只能靠截图和猜测，debug 成本指数上升（这也是“看起来降智”的重要根因之一）。

建议补齐的“秘书 LLM 调用证据”（best-effort）：
- `llm_log_path`（或等价的 call record path）
- `tool_protocol`（json/xml/none + fallback 事件）
- `prompt_cache_enabled` + `prompt_cache_key_hash` + cacheable indexes
- `request_id`（透传 provider 的 request id，或生成本地 call id）

### 4.7 关键缺口：Inbox/Triage 的节奏与 “ack” 策略需要稳定化（避免机械话术）

你已经明确不喜欢“收到/我继续推进”等机械 ack；在秘书模式里更自然的做法是：
- 允许连续输入（Append-only），而不是强行一问一答；
- 由一次 triage 汇总回复来承担“我看到了、我理解了、下一步是什么”；
- 如果确实需要即时反馈，也应是 **可关闭/可降噪的轻量提示**（例如 UI 层状态，不必落盘成助手发言）。

也就是说：**“是否 quick ack”应是策略项**，而不是写死在系统流程里。

### 4.8 关键缺口：Observer 仍是“严格 XML 解析”路径，仍可能成为系统性脆点

你遇到的 “invalid observer output (expected XML or JSON)” 本质上是同一类问题：**把“结构化输出”押注在单一、严格的文本格式**。

更 agentic 的方向是把 Observer 也视为一个 agent profile：
- tool-call 优先（或至少是宽松 tags + 自愈重试）
- 输出失败时先自愈，再暴露给上层；并且错误要可追溯（trace/log pointer）

---

## 5. 下一步改造建议（地基 → 上层）

下面是按“地基优先”的顺序，最符合 agent system 长期收益的推进路线：

### 5.0 Phase 0：补齐秘书 LLM 调用的可观测性（先让 debug 变得便宜）

目标：当秘书“看起来降智”时，能在 trace/ledger 里定位：
1) 具体是哪次 LLM call
2) 当时使用了哪个 tool protocol / 是否 fallback
3) KV-cache 是否启用/是否 downgrade
4) 结构化输出是 tool-call 还是 tags fallback

没有这层证据，后续的“更 agentic”改造会继续被“定位困难”拖累。

### 5.1 Phase 1：把“秘书也是 agent”真正配置化（AgentFactory v2）

目标：Secretary/SW/SU/WorkerTask 都从同一套 AgentSpec 构建，减少重复坑。

建议扩展/固化的配置项（best-effort）：
- `session_module`：写入哪个 message list（SU / SW / assistant）
- `tool_ids`：工具挂载（同一 registry）
- `policy_snapshot`：权限策略（允许但不信任；用 policy 来兜底）
- `workspace_config`：是否提供 workspace（秘书默认禁用）
- `structured_output`：tool-call 优先、tags fallback
- `skills` / `subagent`：是否启用（作为配置项而不是写死）
- `persistence_policy`：哪些记录进 message list，哪些只写入 trace/ledger（append-only）

### 5.2 Phase 2：秘书工具策略从“只读 allowlist”升级为“禁止文件变更（但允许查询/排障）”

你最新的要求可以表述为一个清晰 policy：
- 允许：查询类、检索类、解释类、排障类工具（甚至可以包含 `bash/run_command`，但需要 workspace 禁用或更强的 sandbox）
- 禁止：任何对用户目录的增删改（`write_file/edit/multiedit/trash_file` 等）

落地关键不是“让模型自律”，而是：
- **策略可配置 + 强制执行（fail-closed）**；
- 失败以 `tool_result` 反馈给模型，自行换方案/派工。

### 5.3 Phase 3：彻底移除语义路由（只保留安全边界）

验收标准（Hard Gate）建议写进 OpenSpec：
- “用户问进度/数量/完成情况”时：
  - **不会反问**“哪个任务/哪个 workspace”在只有单一上下文时；
  - **不会输出**“有 N 个问题需要确认”这种空话；
  - 一定给出下一步（等待/点 trace/回复编号/派发新任务）。

### 5.4 Phase 4：可追溯过程的“一等公民”化

把下面这些指针作为 first-class evidence：
- `trace_log_path`
- `findings_path`
- `diff_patch_path`
- `test_report_path`
- `llm_log_path`（包含 KV-cache key hash / cacheable indexes / tool_protocol）

秘书默认低噪声汇报，但每个“需要处理/失败”的点都要能一键展开证据。

---

## 6. 关联资料

- 北极星与 Done 定义：`openspec/project.md`
- 模式分离（前后端边界）：`openspec/changes/update-secretary-worker-mode-separation/design.md`
- Agent Factory（抽取 build-agent）：`openspec/changes/refactor-agent-factory/design.md`
- Secretary 编排能力规范：`openspec/specs/system-secretary-orchestration/spec.md`
- Toolcalling 可靠性规范：`openspec/specs/system-toolcalling-reliability/spec.md`
