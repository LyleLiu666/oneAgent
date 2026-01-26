# openspec/changes 文档 Review

本文 review 了 `openspec/changes/*` 下 5 个 change 的 proposal/design/tasks/specs。重点：逐特性总结、指出依赖/风险/未决点；再综合分析冲突点并给出推荐开发顺序。

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
