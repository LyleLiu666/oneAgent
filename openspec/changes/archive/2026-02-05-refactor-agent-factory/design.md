# Design: Agent Factory (build-agent) extraction

## Context
当前系统里已经存在一套成熟的「Agent Loop」能力（以 Worker Chat 为主）：
- JSON 原生 tools（function calling）+ tool loop
- XML `<tool_data>` fallback loop（toolxml engine + 解析/自愈）
- prompt cache key / cache_control 等 KV-cache 相关策略
- tool_call/tool_result + trace/log 指针的完整留痕

但 Secretary 的实现又新起了一套路径：
- 用“强制输出 JSON”的方式做结构化 planning
- 在代码层面做自然语言关键词路由（progress vs dispatch）
- prompt/协议约束和 Worker 不一致，导致重复踩坑（降智/卡死/不可追溯/难 debug）

从北极星视角（`openspec/project.md`）：oneAgent 是 **agent platform**，不应该靠每个 agent 手写一套“拼 prompt + 选协议 + parse 输出”的逻辑，而应该有一个统一的 build-agent 机制，让各种 agent 只是不同 config/preset。

## Goals
- **统一建造方式**：Worker、Secretary、Subagent 都由同一套 Agent Factory 构建（profile 驱动）。
- **配置化**：挂载哪些工具、是否允许 subagent、是否启用 skills、使用哪种 tool protocol、系统提示词资产，都应是 config。
- **留痕 & 可追溯**：message list 仍然是事实来源；tool_call/tool_result/trace pointers 记录完整过程。
- **KV-cache 友好**：stable prefix 可缓存且 byte-identical；易变内容只进入 volatile turn context。
- **可靠性优先**：不再依赖“严格输出 JSON”的 brittle 方式；结构化输出优先通过 tool-call 或宽松 tag 协议落地（best-effort）。

## Non-Goals
- 不在本设计里改变 UI 的 two-mode（/chat vs /secretary）语义。
- 不要求所有 agent 都必须 tool-call；只要求能力可配置、可复用、可测试。

## Proposed Architecture

### 1) AgentSpec: Declarative agent definition
引入一个 `AgentSpec`（或等价 struct），描述“一个 agent 是什么”：
- `id` / `profile`：例如 `worker-chat` / `secretary-su` / `secretary-sw` / `subagent`
- `session_module`：消息写入哪种 module（assistant/secretary/secretary_sw…）
- `system_prompt_assets`：稳定前缀的资产集合（可复用、可测试）
- `turn_context_providers`：volatile 注入来源（skills recall summary、subagent handoff pointers、observer snapshot…）
- `tool_protocol`：`none | json | xml`（并可声明 fallback 规则）
- `tool_ids`：工具挂载 allowlist（可包含 `subagent`）
- `structured_output`：结构化输出策略（优先 tool-call；否则宽松 tags；禁止强制纯文本 JSON）
- `persistence_policy`：哪些内容要写入 message list；哪些只写 trace/log 指针

### 2) AgentFactory: builds an executable AgentRuntime
`AgentFactory` 接收：
- Runtime（sessions/tasks/memory/tool registry/provider resolver）
- AgentSpec（profile + overrides）
输出一个可执行的 `AgentRuntime`：
- `AssemblePrompt()`：通过统一 assembler 组装 stable prefix + turn context
- `RunLoop()`：根据 tool_protocol（json/xml/none）进入统一的 agent loop
- `Persist()`：按 policy 写 message list（append-only），并记录 trace/log pointers

**关键点**：Factory 内部负责做到：
- stable prefix 不包含 volatile 内容（skills/observer/subagent summaries）
- volatile 内容统一标记（用于 cache selector 排除，best-effort）
- tool schema / protocol 变化会触发 prompt_cache_key 的可解释更新（epoch/signature）

### 3) Structured Output: stop forcing plain-text JSON
对“结构化 planning/decision”（例如 Secretary triage plan）不再要求模型“只输出 JSON”：
- 优先：把 decision 定义成一个内部 tool（例如 `secretary_triage_plan` / `emit_summary`），让模型用 tool-call 返回结构化字段。
- 次选：使用宽松 XML tags（非严格 XML；不要求 CDATA；parser best-effort 修复/兜底）。

这样能避免「为了满足 JSON 约束导致内容降智」以及「arguments JSON string 轻微不合法就 400」的双重风险。

### 4) Migration Strategy (phased)
1. **Phase A：引入 AgentFactory（不改行为）**
   - 用现有 Worker Chat 的实现作为基线，抽取公共部件（prompt assembly / tool protocol selection / persistence hooks）。
   - 用回归测试锁住：KV-cache key、tool loop steps、message list 结构。
2. **Phase B：Secretary SW 迁移**
   - 用 Factory 构建 SW planner，替换“强制输出 JSON + 手写 parse”路径。
   - 去掉代码层关键词路由：由 planner 用 structured output 表达 intent（dispatch/progress/clarify），SU 再落到确定性数据（taskqueue）或继续派工。
3. **Phase C：Subagent/Worker Task 迁移（可选）**
   - 让 subagent 与 worker task 也复用同一套 build-agent（保持隔离与留痕）。

## Test Strategy (TDD-first)
新增/强化三类测试：
1) **Prompt stability tests**：同 config 下 stable prefix 字节级一致；volatile 不进入稳定段；prompt_cache_key 不随 volatile 变化。
2) **Tool protocol regression**：JSON tools / XML `<tool_data>` 两条链路的长文本与自愈能力不回退。
3) **Secretary triage behavior**：planner 在进度询问时返回 progress intent，不再触发“问 workspace/问任务是哪一个”的反问链条；并确保 message list 留痕可恢复。

