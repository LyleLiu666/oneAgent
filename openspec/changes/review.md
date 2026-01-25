# openspec/changes 文档 Review

本文 review 了 `openspec/changes/*` 下 4 个 change 的 proposal/design/tasks/specs。重点：逐特性总结、指出依赖/风险/未决点；再综合分析冲突点并给出推荐开发顺序。

## refactor-container-to-local-tool（容器运行 → 本地工具形应用）
这是其它 3 个 change 的底座：引入 `oneagent` CLI、profile（local/dev）、Token Auth（替代 Keycloak/OAuth）、并重定义 “home/写入边界/目录契约”。
- **home 规则已明确**：默认 `ONEAGENT_HOME=~/.oneagent_default`；启用 workspace 时 `ONEAGENT_HOME=<workspace>/`；home 之外禁止写/改/删。
- **目录契约已统一**：内部状态统一落在 `ONEAGENT_HOME/.oneagent/`（去掉 `sandbox/` 说法）。
- **存储策略已明确**：不支持 Postgres / `DATABASE_URL`；Settings 用 SQLite `ONEAGENT_HOME/.oneagent/settings.db`；其它状态（会话/trace/logs）走文件存储 `ONEAGENT_HOME/.oneagent/{data,logs}`。
- **已知限制**：`bash/run_command` 可能绕过文件工具层的 home/scope 校验（MVP 仅提示词约束“禁止使用 bash 修改文件”）。

主要风险集中在“文件存储的目录结构/命名/并发写策略/清理策略”需要尽早定下来，否则后续 plan/subagent 很容易各自写一套并产生迁移成本。

## enable-skills-usage（技能发现与召回）
该特性目标是“从海量技能里挑少量相关技能注入 prompt”，避免把全量技能塞进上下文。
- **来源**：`<workspace>/.oneagent/skills/**/SKILL.md`、`~/.claude/skills/**/SKILL.md`、`~/.codex/skills/**/SKILL.md`；取并集后按 name 去重（.oneagent > .claude > .codex），必须支持目录 symlink 且避免循环。
- **召回方案已调整**：本阶段不引入 SQLite FTS/索引；采用 `rg`（ripgrep）对 `SKILL.md` 匹配计分并返回 Top-8（稳定排序），system prompt 仅注入 Top-1 推荐技能摘要。
- **为什么不用 go-memdb**：memdb 更偏结构化内存索引，做全文召回仍要自建倒排/分词/权重；相比之下 `rg` 在 macOS/Linux 性能更好、实现成本最低。

主要风险在于：`rg` 缺失时的降级策略（MVP 可先 doctor 提示安装）、扫描边界（避免误扫超大目录/设置超时与输出上限）、以及 symlink 循环处理。

## enable-subagent-orchestration（子 Agent 编排）
该特性通过隔离上下文的 subagent 来执行步骤，并用“短总结 + findings 引用”控制主上下文膨胀。
- **交付件**：每步产出 `FINDINGS.md` + `trace.jsonl`，落 `ONEAGENT_HOME/.oneagent/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/...`；主 agent 仅保留 `summary + findings_path/trace_log_path`。
- **scope**：子 agent 可写范围使用 glob（相对 `ONEAGENT_HOME`），由文件工具层强制越界拦截；并发先后置（MVP 串行）。
- **联动**：按步骤注入 skills（调用 skills recall Top-K）可作为增强项。

主要风险是资源治理（max_steps/max_runtime/log 大小）、日志轮转清理、失败重试语义；这些如果不尽早定，容易出现“能跑但不可控/不可运维”的情况。

## enable-plan-observer-validation（计划模块 + 观察者验收）
该特性把“TDD/验收”变成系统机制：任务 done 必须经 observer 校验通过才写回。
- **PLAN 路径**：`ONEAGENT_HOME/.oneagent/PLAN.md`（启用 workspace 时 `ONEAGENT_HOME=<workspace>/`）。
- **scope**：使用 glob（相对 `ONEAGENT_HOME`），MVP 规则已在 design 中补全；scope enforcement 在文件工具层统一执行。
- **验收策略已调整**：MVP 只做“文件内容/结构”验收（例如 `files` + `must_contain`），不执行命令；如需 `go test` 等命令验收，后续单独开 change 引入。
- **已知限制**：同样存在 `bash/run_command` 可能绕过 scope 的问题（MVP 仅提示词约束）。

## 模块冲突与交叉影响（按优先级）
- **`ONEAGENT_HOME`/`.oneagent/` 目录契约**：token/plan/skills/subagent logs 的路径必须统一，否则 docs 与实现会分叉。
- **文件存储约定**：session/trace/subagent/plan 若各自定义目录结构与命名，会造成迁移与排障困难（建议尽早统一约定）。
- **scope=glob 的统一实现**：plan/subagent/文件工具必须共用同一套“path 规范化 + glob match”规则（含 symlink 与 `..` 处理）。
- **`rg` 依赖**：skills recall（以及未来可能的文件验收）依赖 `rg` 的可用性，需要 doctor 提示与最小降级策略。

## 推荐开发顺序（含理由）
1. **refactor-container-to-local-tool**：先落地 home/目录契约/存储/认证/doctor（这是后续 3 个 change 的共同依赖）。
2. **enable-plan-observer-validation**：固化 glob scope + file-only observer + 原子 `mark_done`，让后续大任务开发天然遵循 TDD。
3. **enable-skills-usage**：实现 skills discovery + `rg` recall，并把召回能力做成可复用库接口（供 subagent 按步骤注入复用）。
4. **enable-subagent-orchestration**：最后实现 subagent runner/tool，复用 scope 与 skills recall，落盘 findings/trace。

## 仍需进一步明确（越早越省返工）
- **文件存储格式**：会话/消息/trace 的目录结构、命名、并发写策略、清理策略。
- **glob 边界**：是否支持否定模式、大小写规则、以及是否要限制 `**` 的使用范围。
- **`rg` 不可用时的降级**：是否必须提供 Go 扫描 fallback，还是 doctor 提示后要求安装。
