# openspec/changes 文档 Review

本文 review 了 `openspec/changes/*` 下的 change proposal/design/tasks/specs。重点：逐特性总结、指出依赖/风险/未决点；再综合分析冲突点并给出推荐开发顺序。

> 说明：本文包含两部分：
> 1) **尚未开始实现（active changes）**：当前目录下的 change（需要落地实现）
> 2) **历史 review（archived）**：上一轮开发遗留的 review 记录（供背景参考）

## add-windows-support（Windows 兼容：runtime + tooling）
该变更面向商业化分发的硬前置：当前实现明显偏 macOS/Unix（`osascript` workspace picker、`bash`/Unix `syscall`、`rg→grep` 降级），Windows 无法编译/运行，更无法在部门内推广。

关键点与风险：
- **必须用 build tags 收敛平台差异**：`shell`、workspace chooser、search backend 都需要 `*_windows.go` 分离，否则会在主干不断引入条件分支与不可维护的兼容层。
- **搜索降级在 Windows 不能依赖 grep**：skills recall/rg 工具必须提供“无 rg/无 grep 仍可用”的等价降级（允许慢），并把“如何启用更快后端”的指引放到 doctor 与 tool result 中。
- **内置 Git/Git Bash 要把合规与可诊断做扎实**：打包 third-party binaries 会带来许可证/NOTICE、体积、以及“使用 system 还是 bundled”的排障需求；必须让 doctor 明确输出来源与版本。
- **命令执行的语义要尽早定**：`bash/run_command` 在 Windows 默认走 Git Bash（优先 bundled）；若环境策略禁止/缺失 Git Bash，必须显式失败并给出替代路径（例如安装 Git for Windows/使用官方 release，或提示该环境不支持）。
- **workspace 选择不能卡用户**：picker 可选，但“手动输入 + 校验/规范化”必须跨平台可用，否则 onboarding 会失效。

## update-workspace-first-onboarding（workspace-first onboarding + quick-start serve）
该变更解决 “进来就能开干” 的第一步：把 workspace 选择做成强引导（但仍可跳过），并补齐 `oneagent serve` 的 quick-start 能力（`--open/--workspace`），让安装后第一次使用尽量“零摩擦”。

现状/已有基础（避免重复造轮子）：
- **workspace 传递已打通**：Chat 请求已支持携带 `workspace`，并写入 session metadata；前端也已有 workspace 输入框 + Browse 选择目录能力（`/api/workspace/choose`）。
- **模型/Provider 管理基本可用**：Settings 页已支持 provider/model CRUD；聊天页 header 已可选择 model 并随会话持久化到 metadata。

本变更尚缺的关键体验闭环：
- **workspace-first onboarding 缺失**：目前 workspace 入口在 header，不够显眼；且后端对“已存在 session 禁止更改 workspace”（安全合理）意味着：用户一旦先发了第一句话、再想改 workspace 会被拒绝 → 必须在 UX 上把“先选 workspace”提到会话开始之前，并提供“新会话重新选”的路径。
- **server default workspace 缺口**：要实现 `oneagent serve --workspace <dir>`，需要后端暴露 default workspace（例如 `/api/config` 或等价机制）并让 UI 在新会话/首次进入时自动填充。
- **`--open` 的跨平台 best-effort**：需要清晰定义打开的 URL（建议始终打开 `http://localhost:<port>`，即使 bind=0.0.0.0），并保证失败不影响服务启动。

潜在歧义/需要提前定口径（避免实现返工）：
- **workspace 优先级**：localStorage（用户上次） vs 会话 metadata（服务端锁定） vs `--workspace`（服务端默认），需要明确最终规则；否则会出现“UI 显示一个路径但实际工具作用域是另一个路径”的致命混乱。
- **model default 语义**：当前后端允许多个 model 同时 `is_default=true`（UpdateModel 未做互斥），需要明确 default 是“全局唯一/按 provider 唯一/允许多个”，并在 spec 与实现对齐。
- **新手模式 vs 专家模式**：header 上的 tool 勾选、protocol、workspace、model 都是高频但会挤爆空间；建议以“默认收起/一键展开”的方式降低视觉负担（不应把聊天页变成设置页）。

## add-autonomous-task-queue（后台任务队列 + Outcome Observer + Resume）
该变更定义 oneAgent 的核心定位升级：从“需要人盯着的工具”升级为“可持续数小时、类人交付”的 agentic 系统。关键是引入 Task 一等公民：可后台运行、可排队、可回溯、可恢复（resume），并以 Outcome Observer 做“是否达成用户预期”的强判定。

现状/已有基础（可直接复用）：
- **长时执行能力已有雏形**：chat handler 使用 `context.Background()`，断开 UI 也能继续；同时 `subagent` 支持更长的 `max_runtime_seconds`、更大的 `max_steps`，并会写 findings/trace。
- **TDD/验收机制已有**：plan observer 已明确“只读验收（文件/内容），不执行命令”，为 Outcome Observer 的设计打了样。

本变更需要补齐的能力与风险点：
- **Task 数据模型与落盘**：需要在 `ONEAGENT_HOME/.oneagent/data/tasks/<task_id>/` 下落 `task.json` + `events.jsonl`，并引入 attempts（run/attempt id）历史；否则无法实现可恢复与可审计。
- **调度模型的现实约束**：要求“同 workspace 串行、不同 workspace 不设固定上限并行”。这在产品语义上正确，但实现上必须有 best-effort 的资源治理（例如当机器资源不足时退化为排队），否则容易把机器打爆并让所有任务一起失败。
- **Outcome Observer 的“强但可控”**：已定为**只读**（不跑命令）。若需要 `go test`，由主/子 agent 生成测试报告文件供 Observer 读取。实现上要明确 Observer 允许的只读工具集合与证据引用格式（否则判定会飘）。
- **断点接续的真实语义**：已定为 attempt-based resume（新 attempt 继承上一轮 summary + findings/trace 引用），并且服务重启后 running attempt 标记为 `interrupted`，默认等待用户 resume。这能最大化“可实现性”，但要在 UX 上把“为什么中断/如何继续/会不会重复修改”讲清楚。

建议的最小可交付拆分（降低一次性大爆炸）：
1) 先做 Task store + API（create/list/get/events），runner 暂时只写假事件；
2) 再接入 subagent 作为 worker（一次 task = 一次 subagent run），把 findings/trace 链接串起来；
3) 再做 resume（attempt 历史 + 复用上次产物的 context_summary）；
4) 最后做前端任务面板（队列/详情/产物入口），把“无人值守”体验做出来。

## 模块冲突与交叉影响（active changes，按优先级）
- **Windows 兼容是平台地基**：`shell/rg/workspace chooser` 的跨平台抽象如果不先做，后续 onboarding/task queue 会在 Windows 上全部不可用。
- **workspace 是一切自动化的地基**：Task/plan/subagent 的写入边界必须一致；workspace-first onboarding 是 task queue 的 UX 前置条件（否则用户一进来就创建 task，但工具作用域不清晰）。
- **Observer 的边界要统一**：plan observer 与 Outcome Observer 都走“只读验收”；命令验收通过“产出测试报告文件”间接完成，避免扩大执行面。
- **并发与文件冲突**：同 workspace 串行是硬要求；不同 workspace 并行是软承诺（best-effort）。实现层必须有“资源不足时降级排队”的策略与清晰事件记录，否则会导致不可诊断的随机失败。
- **留痕与保留策略**：Task events + subagent trace + llm logs 叠加后日志量会很大，需要与 `LOG_RETENTION_DAYS` 的清理策略联动，否则长期运行必然膨胀。

## 推荐开发顺序（active changes，含理由）
1. **add-windows-support**：先把 Windows 的 build/run 与关键工具（command/search/workspace chooser）跑通，否则后续任何“进来就开干/无人值守交付”在 Windows 都不可用。
2. **update-workspace-first-onboarding**：在跨平台基础可用后，把 workspace 入口前置并形成“新会话选 workspace”的闭环，再加 `--open/--workspace` 降低首次使用成本。
3. **add-autonomous-task-queue**：在 workspace UX 稳定后引入 Task/Queue/Resume/Observer；否则会被“工具作用域不明确”拖垮整体可靠性。

## 历史 review（archived changes，供背景参考）

## refactor-container-to-local-tool（容器运行 → 本地工具形应用）
这是其它 4 个 change 的底座：引入 `oneagent` CLI、profile（local/dev）、Token Auth（替代 Keycloak/OAuth）、并重定义 “home/写入边界/目录契约”。
- **home/workspace 规则已明确**：默认 `ONEAGENT_HOME=~/.oneagent_default` 用于承载内部状态目录；workspace 是会话级可选“工具根目录/写入边界”，文件写/改/删默认只能发生在 `<workspace>/` 内（系统不强制自动切换 `ONEAGENT_HOME`；如需“项目私有数据”，可通过 `--home <workspace>` 让两者一致）。
- **目录契约已统一**：内部状态统一落在 `ONEAGENT_HOME/.oneagent/`（去掉 `sandbox/` 说法）。
- **存储策略已明确**：不支持 Postgres / `DATABASE_URL`；Settings 用 SQLite `ONEAGENT_HOME/.oneagent/settings.db`；其它状态（会话/trace/logs）走文件存储 `ONEAGENT_HOME/.oneagent/{data,logs}`。
- **已知限制**：`bash/run_command` 可能绕过文件工具层的 home/scope 校验；出于灵活性与实现成本考虑，当前采取“强引导而非硬拦截”（默认 root 对齐 workspace/home + 清晰提示），不做强制禁止。

存储约定建议尽早固化为“按 `session_id` 分目录/分文件”，让会话之间天然隔离，显著降低并发写冲突与排障成本。

## enable-skills-usage（技能发现与召回）
该特性目标是“从海量技能里挑少量相关技能注入 prompt”，避免把全量技能塞进上下文。
- **来源**：`<workspace>/.oneagent/skills/**/SKILL.md`、`~/.claude/skills/**/SKILL.md`、`~/.codex/skills/**/SKILL.md`；取并集后按 name 去重（.oneagent > .claude > .codex），必须支持目录 symlink 且避免循环。
- **召回方案已调整**：本阶段不引入 SQLite FTS/索引；采用 `rg`（ripgrep）对 `SKILL.md` 匹配计分并返回 Top-8（稳定排序），TurnContext（volatile）仅注入 Top-1 推荐技能摘要（不回写稳定 system prompt，保持 KV cache 友好）。
- **生效方式**：agent 仅凭技能名称调用 `skill.read` 读取对应 `SKILL.md`，其内容以 tool output 进入对话上下文，后续推理遵循其中指令。
- **为什么不用 go-memdb**：memdb 更偏结构化内存索引，做全文召回仍要自建倒排/分词/权重；相比之下 `rg` 在 macOS/Linux 性能更好、实现成本最低。

主要风险在于：`rg` 缺失时的降级策略（`doctor` 提示安装，同时允许降级为 `grep -R`）、扫描边界（避免误扫超大目录/设置超时与输出上限）、以及 symlink 循环处理。

## enable-subagent-orchestration（子 Agent 编排）
该特性通过隔离上下文的 subagent 来执行步骤，并用“短总结 + findings 引用”控制主上下文膨胀。
- **交付件**：每步产出 `FINDINGS.md` + `trace.jsonl`，落 `ONEAGENT_HOME/.oneagent/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/...`；主 agent 仅保留 `summary + findings_path/trace_log_path`。
- **scope**：子 agent 可写范围使用 glob（相对 `<workspace>/` 的相对路径），由文件工具层强制越界拦截；默认串行执行，避免并发编辑引入锁与长等待。
- **联动**：按步骤调用 skills recall Top-K，并把 skills 摘要写入子 agent TurnContext（volatile）可作为增强项（不得回写稳定 system prompt）。

主要风险是资源治理（max_steps/max_runtime/log 大小）、日志轮转清理、失败重试语义；这些如果不尽早定，容易出现“能跑但不可控/不可运维”的情况。

## enable-plan-observer-validation（计划模块 + 观察者验收）
该特性把“TDD/验收”变成系统机制：任务 done 必须经 observer 校验通过才写回。
- **PLAN 路径**：`<workspace>/.oneagent/PLAN.md`（项目私有；不进入 git）。
- **scope**：使用 glob（相对 `<workspace>/` 的相对路径），规则已在 design 中补全；scope enforcement 在文件工具层统一执行，且 plan/subagent/文件工具三方必须复用同一实现。
- **验收策略已明确**：默认只做“文件内容/结构”验收（例如 `files` + `must_contain`），不执行命令；如需 `go test` 等命令验收，后续以单独变更引入（避免扩大执行面）。
- **已知限制**：同样存在 `bash/run_command` 可能绕过 scope 的问题；当前采取“强引导而非硬拦截”（默认 root 对齐 workspace/home + 清晰提示），不做强制禁止。

## optimize-kv-cache（KV Cache 与成本优化）
该特性目标是：在不改变对话语义的前提下，系统化提升 KV cache 命中率，并补齐可观测性，避免 skills/plan/subagent 等动态注入破坏缓存。
- **缓存策略要点**：采用“两头锁定”（前两条 system + 最近两条 message）作为默认 cacheable 选择，并将动态信息统一放入 TurnContext（volatile）以避免污染 stable prefix。
- **厂商覆盖**：所有已集成 provider 必须有明确缓存策略与降级策略（Anthropic/OpenRouter/Bedrock/OpenAI/DeepSeek 等），并通过 capability matrix + 测试保证“开了缓存也不会把请求打挂”。
- **观测落点**：缓存指标进入 trace，同时将每次 LLM 调用的完整 request/response（含 messages）写入日志文件（替代原先入库的做法），trace 仅保存摘要与指针。

## 模块冲突与交叉影响（按优先级）
- **目录契约**：`ONEAGENT_HOME/.oneagent/` 承载内部状态（token/settings/sessions/logs）；`<workspace>/.oneagent/` 承载项目私有数据（skills/PLAN）。两者不要混用口径，否则 docs 与实现会分叉。
- **文件存储约定**：会话相关文件建议按 `session_id` 分目录/分文件；trace/llm/subagent logs 统一在 `ONEAGENT_HOME/.oneagent/logs/` 下分层，避免各模块各写一套。
- **scope=glob 的统一实现**：plan/subagent/文件工具必须共用同一套“path 规范化 + glob match”规则（含 symlink 与 `..` 处理）。
- **`rg` 依赖**：skills recall（以及未来可能的文件验收）依赖 `rg` 的可用性，需要 `doctor` 提示安装，并在缺失时允许降级为 `grep -R`。

## 推荐开发顺序（含理由）
1. **refactor-container-to-local-tool**：先落地 home/目录契约/存储/认证/doctor（其它 change 的共同依赖）。
2. **optimize-kv-cache**：先固化 Stable Prefix vs TurnContext 规范与 provider capability matrix，避免后续动态注入把缓存打碎。
3. **enable-plan-observer-validation**：固化 glob scope + file-only observer + 原子 `mark_done`，让后续大任务开发天然遵循 TDD。
4. **enable-skills-usage**：实现 skills discovery + recall（`rg` 优先、`grep -R` 降级），并将召回能力做成可复用库接口（供 subagent 按步骤注入复用）。
5. **enable-subagent-orchestration**：最后实现 subagent runner/tool，复用 scope 与 skills recall，落盘 findings/trace。

## 仍需进一步明确（越早越省返工）
- **文件存储格式细节**：按 `session_id` 分目录后，trace/llm/log 的文件名规则与清理策略（保留天数/压缩/轮转）。
- **glob 边界**：否定模式（默认不支持）、大小写规则、以及 `**` 的性能边界/滥用防护。
