# Design: Per-user tool permissions system

## Context
oneAgent 目前的 tool 权限控制主要是两类：
1) **进程级开关**：`ONEAGENT_DISABLE_TOOL_<TOOL_ID>=1`（对所有用户生效）
2) **bash 特例**：`ONEAGENT_BASH_ALLOW_RM`（意图限制破坏性命令）

问题在于：当 agent 拥有“对环境施加副作用”的能力时，风险并不止 `rm`，并且“字符串检测 + 特例开关”天然容易被绕过（例如通过解释器/脚本/间接路径构造）。同时，未来存在多用户、后台任务与子 Agent 时，权限必须是 **可传播、可收敛、可审计、可解释** 的。

## Goals / Non-Goals
### Goals
- **按 principal（用户）控制工具权限**：不同用户可拥有不同的 tool allow/deny 与约束。
- **统一的策略模型**：覆盖 tool 暴露（LLM tool list）与执行（runtime enforcement）。
- **可组合**：与 workspace、subagent scope、task queue、OCC 等机制可组合且边界清晰。
- **解释与审计**：每次 allow/deny 都可追溯到“哪个 principal、哪条规则、为何拒绝”。
- **默认安全**：提供保守默认值与最小破坏面，允许逐步放开（而不是默认全开再靠黑名单补洞）。

### Non-Goals（本次不追求一次到位）
- 在所有平台上实现“OS 级强沙箱（seccomp/sandbox-exec/Job Object）”的完备隔离。
- 试图通过黑名单穷举所有危险命令/绕过链路（黑名单不可持续）。

## Policy Model
### Entities
- **Principal**：请求的身份主体（`principal_id`），从认证层注入到所有 handler/tool ctx。
- **Role**：权限集合的复用单元（例如 `admin`/`developer`/`viewer`）。
- **Policy**：由多条规则组成，最终产出“允许哪些工具 + 这些工具的约束”。
- **Rule**：`effect=allow|deny` + `tool_id` + 可选 `action` 与 `constraints`。

### Constraints（示例）
- `file_scope`: `["backend/**", "docs/**"]`（对 write/edit/delete 生效）
- `read_outside_workspace`: `allow|deny`
- `command_profile`: `readonly|dev|full`（对 `bash`/`run_command` 生效；建议 allowlist）

### Resolution & Precedence
建议顺序（从高到低，遇到 deny 即短路）：
1) **Break-glass**：`ONEAGENT_DISABLE_TOOL_*`（全局 kill switch，最高优先级）
2) **Task Attempt snapshot**：后台任务在 attempt 启动时固定一份 effective policy（hash + 解析后的规则）；运行中不漂移
3) **Session override**：会话级选择的 profile（例如 UI 选择“只读/开发/全量”）
4) **Principal policy**：用户绑定的 role/policy
5) **Default policy**：系统默认值（local 模式建议较宽但仍避免 “shell 全开”）

## Enforcement Points
- **Tool mounting**：给 LLM 的 tool list 必须是 policy 过滤后的结果（减少误触发）。
- **Tool execution**：即使 LLM “猜到” tool 名称或缓存了旧列表，执行时仍必须二次校验（不可仅靠暴露列表）。
- **Subagent**：子 Agent 的 tools = `parent_effective_tools ∩ requested_tools`，并继承同一 policy snapshot 上限。
- **Task runner**：attempt 启动时固化 policy snapshot；要提升权限必须通过新的 attempt（resume）。

## Command Tool Hardening (bash/run_command)
核心结论：**黑名单不可持续**。策略系统应让管理员选择更安全的模式：
- `readonly`：仅允许显式 allowlist 的只读命令（例如 `ls`, `cat`, `rg`…），禁止解释器/编译器/网络
- `dev`：在 `readonly` 基础上逐步放开（仍建议以 allowlist 管理）
- `full`：仅限高信任 principal（明确风险提示 + 审计）

并且：
- 禁止“通过脚本执行绕过”：执行脚本时必须可静态检查，且脚本自身受同一 allowlist 约束。
- 对解释器（`python/node/...`）默认 deny（除非 `full` 且明确允许）。

## Storage & Admin Surface
最小可交付路径：
- **存储**：优先放入 `settingsdb`（可按 user_id 读写），并可导出为 JSON 备份。
- **管理入口**：提供 API/CLI（create/list/revoke token；assign role；inspect effective policy）。
- **UI**：后续提供 workbench，至少能“看见当前 principal 的 effective policy”。

## Migration
- 移除 `ONEAGENT_BASH_ALLOW_RM`，并在拒绝信息中提示“请通过 tool permissions policy/profile 配置”。
- 保留 `ONEAGENT_DISABLE_TOOL_*` 作为全局 kill switch（便于紧急止血）。
- 对现有单用户 local 模式：默认 principal=`local`，默认 policy 兼容现有可用工具集。

## Risks / Trade-offs
- **复杂度上升**：需要明确 policy precedence，避免“以为允许但实际被上层 deny”的困惑 → 依赖 explainability。
- **安全/灵活性的张力**：allowlist 会降低自由度 → 通过 session/task profile + JIT grant（后续）缓解。
- **向后兼容**：auth token 与用户映射改动必须兼容老的单 token 文件。

## Open Questions
- multi-user 的边界：是否需要“真正的用户体系”（用户名/密码/管理面）还是“多 token 即多 principal”即可？
- 默认 policy 的取舍：local 模式是否默认允许 `bash/run_command`？如果允许，允许到什么程度（readonly vs dev）？
- 权限变更是否允许“运行中的 task 立即生效”？本设计默认“attempt 固化，变更通过 resume 生效”，是否接受？

