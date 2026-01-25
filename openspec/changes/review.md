# openspec/changes 文档 Review

本文 review 了 `openspec/changes/*` 下所有 proposal/design/tasks/specs 文档（共 4 个 change），目标是：逐特性指出依赖/风险/未决细节，再综合分析潜在冲突，并给出推荐开发顺序。

## refactor-container-to-local-tool（容器运行 → 本地工具形应用）
这是影响面最大、对其它 change 具有“地基性质”的架构调整：引入 `oneagent` CLI、profile（local/dev）、`ONEAGENT_HOME` 目录规范、默认 SQLite 持久化、Token Auth（替代 Keycloak/OAuth）、以及关键的 workspace 定义与文件写入边界；它会改动启动链路、配置优先级、认证、存储、工具依赖可用性（doctor）、以及发布流水线，任何后续能力（skills/subagent/plan）几乎都在复用这些底座概念，因此最需要先把“跨平台 home 路径/反代与 IPv6/是否保留 Postgres 兼容/敏感 token 是否落盘加密/bash 安全边界/默认 LAN 监听的安全告知与最小化暴露面”这些开放问题尽早定下来，否则后续模块很容易在路径规范、SQLite driver 选择、权限边界上各自为政并产生返工。

## enable-skills-usage（技能发现、索引、召回与选择）
该特性本质是“把海量外部知识资产以可控方式注入 prompt”，设计里同时涉及多来源扫描（`<workspace>/.oneagent/skills`、`~/.claude/skills`、`~/.codex/skills`）、symlink 跟随与循环检测、按 name 去重与稳定优先级（.oneagent > .claude > .codex）、以及为 1000+ skills 引入 SQLite FTS 索引与 Top-8 召回 + LLM Selector（二段式）——核心风险集中在“索引存储位置/workspace_id 的稳定计算/SQLite 选型（是否 cgo、是否与主业务 SQLite 复用同一 driver）/性能与稳定排序（同分 tie-break）/扫描边界（最大深度、忽略规则、避免误扫超大目录）/Selector 在无 LLM 或离线时的降级语义”；同时它与 subagent 的“按步骤注入 Top-K skills”强联动，建议在接口上把“召回（Top-K）”做成可复用库能力（不仅给主对话，也给 subagent/observer），避免日后出现两套 skill query/score 逻辑分叉。

## enable-subagent-orchestration（子 Agent 编排）
该特性目标明确：把复杂任务切步骤，用隔离上下文的子 Agent 执行，并通过“短总结 + findings 文件引用”避免主上下文膨胀，同时要求完整 trace 落盘（`ONEAGENT_HOME/logs/subagent/...`）以便回溯；潜在不确定点主要在“默认工具权限是否与主 agent 一致、scope 的表达与 enforcement 层级（workspace 级边界 + subagent 级子目录 scope）、日志落盘的轮转/清理策略、失败重试策略、并发是否做 MVP（文档倾向先串行）以及默认 max_steps/max_runtime 偏大的资源治理”；另外它依赖稳定的 `ONEAGENT_HOME` 与 workspace 机制，否则 findings/logs 的落点与文件越界拦截会在容器/本地两套运行方式下表现不一致，而这会直接破坏可追溯性与安全边界。

## enable-plan-observer-validation（计划模块 + 观察者验收）
该特性把“TDD/验收”变成系统机制：在 workspace 下维护 `PLAN.md`（默认 `<workspace>/.oneagent/PLAN.md`），并把“标记 done”升级为必须经 observer 校验通过的原子操作，observer 只读文件/搜索/列目录 + 仅执行 acceptance.commands 白名单命令；最大风险在“计划文件格式的可解析性与可扩展性（Markdown 结构 vs frontmatter）、acceptance.commands 的安全边界（允许哪些命令/超时/工作目录/环境变量/是否需要额外 allowlist）、scope 表达（prefix vs glob）与并发策略（scope 重叠如何处理）”，以及与本地工具默认 LAN 暴露结合后的安全面（远程访问 UI + token 泄露时可触发命令执行）；它与 subagent 的 scope 分工、handoff 注入验收结果属于强耦合，建议把 scope enforcement 统一沉到文件工具层（同一套校验同时服务主 agent/subagent/observer），并把 observer runner 的命令执行实现复用同一套“受控命令执行器”（带 allowlist、cwd、timeout、输出大小上限）。

## 潜在冲突与交叉影响（按优先级）
- **统一的底座概念冲突**：`ONEAGENT_HOME`、workspace、scope、日志目录、配置优先级（flags/env/config/defaults）在多个 change 中重复出现；如果先后由不同模块各自实现，会导致路径与权限语义不一致（尤其是“workspace 外可读不可写”与“subagent scope 子集”叠加时的裁决规则）。
- **SQLite 选型/构建链路冲突**：本地工具默认存储（SQLite）与 skills 索引（SQLite FTS）会把 SQLite driver 变成全局技术决策；若选 `mattn/go-sqlite3`（cgo）会影响 release pipeline 与跨平台分发，若选纯 Go driver 则要确认 FTS5 支持与性能，建议尽早定一套并在两个模块共用。
- **“尽量不污染 repo”与 `.oneagent/` 写入的张力**：refactor 倾向把可变状态放 `ONEAGENT_HOME`，而 skills/plan 明确把项目私有资产放 `<workspace>/.oneagent/*`；需要在产品层面确认这是否可接受（并确保默认写入目录可被一键加入 `.gitignore`，且在“不启用 workspace”的会话里绝不写 repo）。
- **命令执行安全边界叠加**：plan observer 会执行 acceptance.commands；本地工具模式默认 LAN 监听 + token auth，一旦 token 泄露，命令执行面会显著放大风险；需要把“命令 allowlist/超时/输出限制/工作目录与 scope/审计日志”作为强制设计项。
- **skills ↔ subagent 的联动耦合**：skills 设计强调“召回 + 选择器只注入单个推荐技能”，而 subagent 设计倾向“按步骤注入 Top-K skills 摘要”；需要统一“主对话与 subagent 的注入策略（1 个 vs K 个）”与 API（同一召回接口，策略由调用方决定）。

## 推荐开发顺序（含理由）
1. **refactor-container-to-local-tool**：先落地 `oneagent` CLI、`ONEAGENT_HOME`、workspace 与文件写边界、默认 Token Auth、SQLite 默认持久化（并定 SQLite driver）；这些是后续三个 change 的共同依赖与安全边界来源。
2. **enable-plan-observer-validation**：在底座稳定后优先引入“done 必须验收”的机制，把 scope/受控命令执行/原子写回等硬约束提前固化，后续 subagent 与大任务开发能直接复用（也更符合 TDD 的质量目标）。
3. **enable-skills-usage**：在 `ONEAGENT_HOME` 与 SQLite 决策确定后实现技能发现/索引/召回/选择，并把召回能力做成可复用库，为 subagent 的按步骤技能注入打基础。
4. **enable-subagent-orchestration**：最后实现 subagent runner/tool、findings + trace 落盘、scope enforcement（复用第 2 步的 scope 规则）与 skills 注入（复用第 3 步的召回能力），避免在底座未定时就把日志路径/存储策略/权限语义固化到 subagent 里。

## 需要先拍板的关键问题（否则返工概率高）

### 1) `ONEAGENT_HOME`：默认路径、覆盖方式、目录布局是否固定？
**已有文档怎么说**
- `openspec/changes/refactor-container-to-local-tool/design.md`：建议平台默认路径（macOS `~/Library/Application Support/oneagent/`；Linux `~/.local/share/oneagent/` 或 XDG；Windows `%APPDATA%\\oneagent\\`），并允许 `ONEAGENT_HOME` 或 `--home` 覆盖；目录结构示例为 `config/ data/ logs/ tmp/`。
- `openspec/changes/refactor-container-to-local-tool/specs/local-runtime/spec.md`：要求统一的 `ONEAGENT_HOME`，且在首次启动时创建必要子目录（示例包含 `config/data/sandbox/logs/tmp`）。
- `openspec/changes/refactor-container-to-local-tool/specs/auth-mode/spec.md`：要求 token 固定落盘到 `ONEAGENT_HOME/config/auth_token`。
- `openspec/changes/enable-skills-usage/design.md`：技能索引建议落在 `ONEAGENT_HOME/cache/skills/<workspace_id>/skill-index.sqlite`（引入了 `cache/` 子目录概念）。
- `openspec/changes/enable-subagent-orchestration/design.md`：子 agent 完整痕迹落 `ONEAGENT_HOME/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/...`。

**需要进一步澄清/拍板**
- Linux 是否严格遵循 XDG（拆分 `config/data/cache` 到各自 XDG 目录）？还是统一“一个 `ONEAGENT_HOME` 根目录，内部再分 `config/data/cache/logs/tmp/...`”？
- `sandbox/` 子目录是否必须（`local-runtime` spec 示例包含，但 `refactor` design 示例未包含）？如果未来 `BASH_ROOT_DIR` 默认对齐 workspace，那么 `sandbox/` 的定义与用途需要明确，否则目录规范会互相打架。
- 覆盖与优先级需要最终口径：`--home` vs `ONEAGENT_HOME` vs 配置文件（`refactor` design 倾向 flags > env > config > defaults），以及配置文件本身放在哪（`ONEAGENT_HOME/config/config.yaml` 还是别处）。
- 需要确定哪些路径是“契约稳定”的（比如 `config/auth_token`、`logs/subagent/...`、`cache/skills/...`），哪些可以内部调整（避免后续 docs/脚本/用户手工依赖路径后难改）。

### 2) SQLite 选型：cgo、FTS5、以及“主库 vs skills 索引”的关系
**已有文档怎么说**
- `openspec/changes/refactor-container-to-local-tool/design.md`：local profile 默认 SQLite（保持 GORM），并可选兼容 Postgres；强调自动迁移与健康检查。
- `openspec/changes/enable-skills-usage/design.md`：技能索引建议 SQLite + FTS5，用于 1000+ skills 的快速召回；索引落 `ONEAGENT_HOME`，按 workspace 隔离。

**需要进一步澄清/拍板**
- 是否允许 cgo（直接影响能否轻松产出“单文件可执行 + 跨平台 release”）：如果不允许 cgo，需要确认所选纯 Go SQLite driver 是否支持 FTS5、以及在目标平台的稳定性/性能。
- FTS5 是否为硬要求：skills 方案的召回默认依赖 FTS；如果 FTS5 不可用，是否接受替代索引方案（或先降级为扫描/关键词匹配的 MVP）。
- “主业务 SQLite”与“skills 索引 SQLite”是否：
  - 必须共用同一 driver（建议是，减少兼容坑）；
  - 是否共用同一 DB 文件（强烈建议不要：skills 更像可重建 cache，主库是业务数据）；
  - 连接管理是否共用（通常应分开：避免锁竞争、便于把 skills 视为只读/可重建）。
- 是否需要明确主库的运行参数（WAL、busy_timeout、journal_mode、连接池）与 skills 索引的更新策略（增量更新频率/锁策略），否则在“本地长跑 + 并发 tool 调用”场景下容易踩锁与性能抖动。

### 3) workspace/scope：表达方式与裁决顺序（workspace 外可读不可写 + scope 子集）
**已有文档怎么说**
- `openspec/changes/refactor-container-to-local-tool/design.md`（2.5 Workspace）：workspace 是会话级项目根目录；默认仅允许修改 workspace 内文件；workspace 外允许读取任意绝对路径；工具默认作用域对齐 workspace；`BASH_ROOT_DIR` 默认对齐 workspace。
- `openspec/changes/refactor-container-to-local-tool/specs/workspace/spec.md`：把“workspace 外可读不可写”和“搜索默认在 workspace 内”写成 MUST。
- `openspec/changes/enable-plan-observer-validation/design.md`：plan task 可声明 `scope`（workspace 子目录集合），并在文件工具层强制越界写入拦截；为未来并发 subagent 分区做准备。
- `openspec/changes/enable-subagent-orchestration/specs/system-subagent-orchestration/spec.md`：子 agent 也需要可写 scope，并在文件工具层强制执行。

**需要进一步澄清/拍板**
- scope 表达：是“目录前缀集合（prefix，推荐先做）”、还是支持 glob（更灵活但更难实现与解释），或者两者都支持（需要明确优先级与禁止的通配符能力）。
- 裁决顺序需要写成明确算法：`writable = within(workspace_root) AND within(scope_set_if_any)`；当未启用 workspace 时是否“默认禁止任何写工具/命令工具”（`refactor` tasks 有此倾向）也需要最终口径。
- 路径规范化规则必须提前定义：如何处理 `..`、symlink、大小写不敏感文件系统、以及“workspace 本身是 symlink”的情况，避免出现“看起来在 scope 内但 realpath 越界”的绕过。
- scope enforcement 只限制“文件工具”是不够的：`bash/run_command` 类工具可以绕过文件层校验直接改 workspace 外内容；需要明确命令工具的最小边界（至少固定 `cwd` 在 workspace，并配合 observer 的命令白名单策略）。

### 4) observer `acceptance.commands`：安全策略、资源限制、审计与 LAN 风险
**已有文档怎么说**
- `openspec/changes/enable-plan-observer-validation/design.md`：observer 是独立执行单元，默认只读（读文件/搜索/列目录），但**必须允许**执行验收命令；且只能执行来自该任务 `acceptance.commands` 清单的命令；`plan.mark_done` 必须原子（通过才写回）。
- `openspec/changes/enable-plan-observer-validation/specs/system-plan-validation/spec.md`：明确要求“清单外命令不得执行”，并举例 `rm -rf /` 不在清单中则不得执行。
- `openspec/changes/refactor-container-to-local-tool/specs/local-runtime/spec.md`：local 模式默认 LAN 监听（`0.0.0.0`）+ token auth；并强调不要依赖 IP 阻断。

**需要进一步澄清/拍板**
- 是否还需要“全局 allowlist/denylist”（在 `acceptance.commands` 之外再加一层系统级约束）：否则一旦 token 泄露或计划被恶意篡改，observer 的命令执行面会非常危险（尤其是在 local 默认 `0.0.0.0` 的前提下）。
- 命令执行的安全与资源治理策略需要定死：timeout（默认多少）、输出大小上限（截断/落盘）、并发、进程组杀死、工作目录（workspace root or scope root）、以及环境变量隔离（哪些 env 透传、是否禁用交互）。
- 审计与可追溯：observer 每次执行应落哪些日志（命令、退出码、耗时、截断后的输出、关联 task_id/session_id），落点建议与 subagent 同样归入 `ONEAGENT_HOME/logs/...` 并有轮转/清理策略。
- LAN 风险告知与最小化暴露面：除了登录页提示“公网风险大”，是否需要把“会执行本机命令”的能力做更强的 UI/配置开关（例如默认禁用 commands 验收，只允许 files 验收；或必须显式开启 `ENABLE_OBSERVER_COMMANDS=1` 才能跑 commands），否则安全预期很难控。
