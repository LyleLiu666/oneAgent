# Change: Extract a configurable Agent Factory (build-agent) for workers + secretary + subagents

## Why
目前 oneAgent 里「跟 LLM 打交道」的关键逻辑是分散的：
- Worker Chat（/api/chat）已经有成熟的一套：tool loop（JSON tools / XML `<tool_data>`）、自愈/回归测试、KV-cache（prompt cache key + cache_control）与完整留痕（tool_call/tool_result + trace/log pointers）。
- Secretary（/api/secretary/*）又重新走了一套：强行要求输出 JSON、在代码里做“路由/关键词判断”、以及一些与 Worker 不一致的 prompt/协议约束。

结果就是你看到的「降智/卡住/反问一堆」：模型被格式约束牵着走、流程被硬编码分支决定、而且同样的问题（tool 协议、KV-cache、留痕、loop）在不同 agent 上重复踩坑。

而从 `openspec/project.md` 的北极星出发：**这是一个 agent 系统**。Worker 是 agent，Secretary 也是 agent，Subagent 也是 agent。我们需要一套**统一的 build-agent 能力**，让「挂载哪些 tools / 是否允许 subagent / 是否启用 skills / 使用什么 system prompt / 使用哪种 tool protocol」都变成可配置项，而不是每个 agent 各写一套。

## What Changes
- 提取一套 **Agent Factory**（build-agent）作为后端核心基础设施：用一个声明式 config 构建任意 agent。
  - `system_prompt`：模块化 prompt assets + profile/preset（稳定前缀）
  - `turn_context`：volatile 注入（skills/observer/subagent refs 等）不污染 stable prefix
  - `tool_protocol`：`none | json | xml`（并复用现有 toolcalling reliability 逻辑）
  - `tool_ids`：挂载哪些工具（包括是否挂载 subagent）
  - `skills`：是否启用、启用方式（recall + read gating）、以及可观测注入
  - `persistence`：message list 留痕策略（text/tool_call/tool_result/trace pointers），append-only + KV-cache 友好
- 用同一套 Factory 分阶段接管：
  - Secretary SW（triage planner）
  - Secretary SU（user-facing report）
  - Worker Chat（/api/chat，现有能力保持不回退）
  - Subagent（复用同一套 tool protocol + prompt assembly + caching 策略）

## Non-Goals (this change)
- 不在本 change 内“一口气重写所有 agent”；迁移按阶段推进，优先从 Secretary 开始消除重复坑。
- 不改 UI 入口语义（/chat vs /secretary）与 session module 边界（保持现有 separation 方向）。
- 不对历史混杂会话做迁移/重写（保持 append-only / 可追溯）。

## Impact
- Affected specs (expected):
  - `system-prompt-assembly`（stable prefix / volatile context 的统一装配）
  - `system-llm-prompt-caching`（cache key / cache selector 在多 agent 一致生效）
  - `system-toolcalling-reliability`（JSON tools vs XML `<tool_data>` 的统一策略 + 自愈）
  - `system-subagent-orchestration`（subagent 是否挂载、如何隔离与留痕）
  - `system-secretary-orchestration`（Secretary 作为 agent 的统一建造方式；减少“强制 JSON”）
- Affected code (expected, after approval):
  - `backend/internal/handler/chat.go`
  - `backend/internal/handler/secretary.go`
  - `backend/internal/secretary/**`
  - `backend/internal/subagent/**`
  - `backend/internal/toolxml/**`
  - `backend/internal/prompt/**`（如存在 assembler）
  - `backend/internal/sessionstore/**`

## Risks / Mitigations
- **行为回归风险**：集中抽象可能影响现有 Worker Chat 的稳定性  
  - Mitigation：以 Worker Chat 现有回归测试为基线，加“golden tests”覆盖 build-agent 的 prompt/cache/tool loop 行为；迁移分阶段、可回滚。
- **配置爆炸**：配置项过多导致难用  
  - Mitigation：提供少量 profile/preset（worker-chat / secretary-su / secretary-sw / subagent），高级配置在内部保留但对外渐进暴露。
- **KV-cache 退化**：把易变内容混进 stable prefix 或 cacheable selection  
  - Mitigation：单测验证 stable prefix 字节级稳定、prompt_cache_key 仅绑定 stable signature；volatile 统一标记并从 cacheable selection 排除（best-effort）。

