# openspec/changes 评审：潜在冲突与开发顺序（2026-01-25）

评审范围：`refactor-container-to-local-tool`、`enable-skills-usage`、`enable-subagent-orchestration`，以及新增的 `enable-plan-observer-validation`（含各自目录下的 `proposal.md`/`design.md`/`tasks.md`/`specs/**/spec.md`）。

## 建议开发顺序（按 change 粒度）
1) `refactor-container-to-local-tool`（先把 profile/配置层级/ONEAGENT_HOME+workspace 边界、本地认证、IPv6/反代立场等“底座”定下来）  
2) `enable-plan-observer-validation`（把 plan 与 observer 校验做成通用能力，后续复杂变更都能用它强制验收/TDD，且为 subagent 并发的 scope 分工铺路）  
3) `enable-skills-usage`（复用 SQLite 选型做 FTS 索引与召回；为后续按步骤技能注入提供能力）  
4) `enable-subagent-orchestration`（对 tool-loop、trace/log、文件交付与安全限制的压力最大；依赖 workspace/scope 与 plan 校验后实现更稳妥）

## 主要冲突/耦合面（跨特性）
- **路径与数据归属**（已给出方向）：`ONEAGENT_HOME` 存放 db/log/cache 等全局数据；workspace 对齐 coding 的 project 概念，默认仅允许修改 workspace 内文件；workspace 外文件“必要时可读，尽量不写”。这要求所有文件工具/命令工具具备“workspace/scope”约束能力。
- **SQLite/FTS 选型耦合**：local-tool 的主库（GORM + SQLite）与 skills 的索引（SQLite FTS5）共享底层依赖与跨平台打包约束；SQLite driver（是否 CGO、是否支持 FTS5、并发/锁策略）会同时影响两项。
- **认证语义**（已改为 token）：默认启动生成不过期本地访问令牌（Bearer token），并在登录页提示“仅建议局域网/公网风险极大”；不依赖 IP 阻断（IPv6/反代场景不可靠）。
- **代码热点冲突**：`ChatHandler`/system prompt 构建、tool-loop、trace/logging、配置加载是多个 change 的共同改动点；并且 plan/observer 会与 subagent 的 handoff、scope 限制、done 校验深度耦合。

## 逐特性评审（每个特性一段）

### refactor-container-to-local-tool
这是影响面最大且最“底座化”的变更：它把默认交付从 Docker/Compose 改为本地 `oneagent` CLI，并引入 profile（local/dev；server 废弃）、统一的 `ONEAGENT_HOME` 目录规范、workspace（project）概念与文件作用域约束、默认 SQLite 持久化（可选 Postgres 兼容）、token/none auth 并废弃 OAuth/Keycloak 主路径、以及 doctor/发布流水线等；它会直接决定后续所有“文件/日志/缓存/索引/交付件”的落点与边界，建议优先把 workspace 定义与工具越界拦截、token 认证与登录风险提示、IPv6/反代立场（不做 IP 阻断）先闭环，再做存储与发布流水线；当前仍容易返工的点主要集中在 SQLite driver/FTS5 的跨平台选型、token 的呈现与轮换机制、workspace 外“只读能力”的实现边界，以及 Postgres 兼容路径是否保留。

### enable-skills-usage
这项变更把“技能生态”引入系统：从 `<workspace>/.oneagent/skills`、`~/.claude/skills`、`~/.codex/skills` 三处扫描 `SKILL.md`（支持 symlink 且避免循环）、解析 YAML frontmatter（并提供回退策略）、按规范化 name 去重并给出稳定覆盖优先级（.oneagent > .claude > .codex），再用 SQLite FTS 建索引以支持海量技能的 Top-8 稳定召回，并通过一个独立的 Selector（可复用主模型、可降级）在 Top-8 中选出 1 个或 none；结合“home 存 cache、workspace 存 project 私有数据”的方向，索引/缓存应默认落在 `ONEAGENT_HOME` 并按 workspace 隔离，同时 `<workspace>/.oneagent/skills` 仍作为 project-private skills source；仍需尽快锁定的是 SQLite FTS5 在跨平台（尤其 Windows）下的实现路径，以及“显式技能名解析/别名/模糊匹配”的策略边界。

### enable-subagent-orchestration
子 Agent 编排是对 oneAgent 执行模型的一次大扩展：主 Agent agentic 判断是否启动 subagent，子 Agent 在隔离上下文中跑一段受限的 tool-loop，主要交付写入 `FINDINGS.md`（至少包含 `## 流水账` 与 `## Findings`），系统同时把完整过程写入可回溯的 jsonl 日志并在 trace 里只存摘要与指针，返回主 Agent 的结果严格控制为“短总结 + 路径引用”，且默认禁止递归启动 subagent；结合你的方向，subagent 默认工具权限应与主一致（文件类工具默认可用），但必须受 workspace/scope 限制，并设置偏大的默认 max_steps/max_runtime（面向长任务交付）；并发 subagent 建议后置到 plan/scope 分工成熟后再做，优先用 scope 分区规避长等待锁。

### enable-plan-observer-validation
这是一个新的“通用底座能力”：把复杂任务的分解与验收固化为 `PLAN.md`，并在标记 done 时自动引入 observer 做交付件校验，未达标则拒绝推进并要求重试；它同时为未来并发 subagent 提供 scope 分工与越界拦截的抓手（先分区再并发，而不是靠全局锁等待），也能让主 Agent 的工作流更接近 TDD（先写验收标准→干活→校验通过才能进入下一步）；关键未定点集中在计划文件格式（Markdown vs frontmatter）、observer 的默认工具权限（纯只读 vs 允许执行验收命令）、以及 scope 表达方式（目录前缀 vs glob）。

## 已确认的方向（来自补充约束）
1) **home vs workspace**：`ONEAGENT_HOME` 存 db/log/cache；workspace 对应 project，可选启用/复用；默认仅允许修改 workspace 内文件；workspace 外允许读取任意绝对路径，但原则上不写。  
2) **跨平台优先**：尽量避免把关键能力绑定到特定 OS/依赖（尤其是 SQLite/FTS 与路径处理）。  
3) **认证**：启动默认生成不过期本地访问令牌（Bearer token）；token 固定写入 `ONEAGENT_HOME/config/auth_token`，并通过 `doctor` 提示路径；登录页明确“仅建议局域网/公网风险大”。  
4) **网络**：支持 IPv6、尊重反代；“内网使用”不靠 IP 禁止，而靠 token 认证与使用约定。  
5) **权限与限制**：默认可用全部文件操作工具；subagent 默认限制应偏大（例如 max_steps>=200、max_runtime>=1h）；并发应依赖 plan+scope 分区优先规避锁等待。

## 仍需尽快拍板/澄清的问题（跨特性）
1) **跨平台现状核查（bash 工具链）**：当前 `bash/run_command` 基础设施依赖 POSIX 进程组（例如 `syscall.SysProcAttr{Setpgid:true}`、`SIGKILL`），因此 Windows 目前无法直接编译/运行；如果短期不做 Windows，则 SQLite/FTS5 可以优先选择在 macOS/Linux 上可用且实现最稳的方案（允许 CGO）。  
2) **SQLite driver/FTS5（macOS/Linux 优先）**：在不强求 Windows 的前提下，是选 CGO 的 `go-sqlite3`（FTS5 成熟）还是纯 Go driver（需确认 FTS5/性能/稳定性）？
