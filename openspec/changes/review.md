# openspec/changes 评审：潜在冲突与开发顺序（2026-01-25）

评审范围：`enable-skills-usage`、`enable-subagent-orchestration`、`refactor-container-to-local-tool`（含各自目录下的 `proposal.md`/`design.md`/`tasks.md`/`specs/**/spec.md`）；三项变更均已通过 `openspec validate <change-id> --strict --no-interactive` 校验。

## 建议开发顺序（按 change 粒度）
1) `refactor-container-to-local-tool`（先把 profile/配置层级/ONEAGENT_HOME/日志目录/SQLite 选型这类“底座”定下来）  
2) `enable-skills-usage`（复用 SQLite 选型做 FTS 索引与召回；为后续 subagent 的按步骤技能注入提供能力）  
3) `enable-subagent-orchestration`（对 tool-loop、trace/log、文件交付与安全限制的压力最大，最后做更稳妥）

## 主要冲突/耦合面（跨特性）
- **路径与数据归属冲突**：skills 的 `<workspace>/.oneagent/...`、subagent 的 `<workspace>/.logs/...` 与 local-tool 的 `ONEAGENT_HOME/...` “不污染 repo”目标需要统一口径（建议抽象 PathResolver，默认落 `ONEAGENT_HOME`，workspace 仅作为可选 source/namespace）。
- **SQLite/FTS 选型耦合**：local-tool 的主库（GORM + SQLite）与 skills 的索引（SQLite FTS5）共享底层依赖与跨平台打包约束，SQLite driver（是否 CGO、是否支持 FTS5、并发/锁策略）会同时影响两项。
- **认证语义耦合**：local-tool 计划用 `Authorization: Bearer <password>` 做 password auth，容易与现有 Bearer JWT 语义混淆；需要在 middleware/前端凭证存储/会话模型上先定一致方案，否则后续改动会波及 Settings/对话链路与 trace。
- **代码热点冲突**：`ChatHandler`/system prompt 构建、tool-loop、trace/logging、配置加载是多个 change 的共同改动点，先做底座（local-tool）再叠加能减少反复迁移与重构。

## 逐特性评审（每个特性一段）

### refactor-container-to-local-tool
这是影响面最大且最“底座化”的变更：它把默认交付从 Docker/Compose 改为本地 `oneagent` CLI，并引入 profile（local/dev；server 废弃）、统一的 `ONEAGENT_HOME` 目录规范（config/data/sandbox/logs/tmp）、默认 SQLite 持久化（可选 Postgres 兼容）、password/none auth 并废弃 OAuth/Keycloak 主路径、以及 doctor/发布流水线等；因此它会直接与另外两项在“日志/缓存/交付文件落盘位置（workspace vs ONEAGENT_HOME）”“SQLite driver/FTS5 选型”“Authorization header 语义（Bearer password vs Bearer JWT）”“宿主机工具调用安全边界（BASH_ROOT_DIR/sandbox 默认与限制）”上形成冲突或返工风险——建议在本 change 内先锁定这些关键决策并实现最小闭环（profile+配置优先级+home/log/path 抽象+SQLite 选型+auth middleware），再让 skills index 与 subagent trace 都依赖同一套路径与存储底座；目前仍有较多细节未定且容易引发争议/返工（共享密码的配置与轮换机制、LAN/公网判定策略含 IPv6/反向代理、是否保留 Postgres 兼容路径、敏感 token 是否需要落盘加密与加密方案、release pipeline 的交付形态与签名校验等），这些应在进入大规模实现前尽量拍板。

### enable-skills-usage
这项变更把“技能生态”引入系统：从 `<workspace>/.oneagent/skills`、`~/.claude/skills`、`~/.codex/skills` 三处扫描 `SKILL.md`（支持 symlink 且避免循环）、解析 YAML frontmatter（并提供回退策略）、按规范化 name 去重并给出稳定覆盖优先级（.oneagent > .claude > .codex），再用 SQLite FTS 建索引以支持海量技能的 Top-8 稳定召回，并通过一个独立的 Selector（可复用主模型、可降级）在 Top-8 中选出 1 个或 none，最后把“中文推荐技能摘要”注入 system prompt，同时保留自然语言显式指定 skill 名的路径；它与 local-tool 的核心冲突在于索引/缓存的默认落点（proposal/design 偏向 `<workspace>/.oneagent/cache/skill-index.sqlite`，但 local-tool 强调默认不污染 repo 并改用 `ONEAGENT_HOME`），以及 SQLite driver/FTS5 的统一选型——建议把 index/cache 位置改为“可配置 + 默认随 `ONEAGENT_HOME`（并按 workspace 做 namespace/隔离）”，同时把 `<workspace>/.oneagent/skills` 处理为“若存在则扫描”的 source（满足 MUST 发现，但不要求默认创建目录）；此外仍有一些实现细节未完全敲定（召回 query 的构成：仅最后一句 vs 包含会话摘要/系统提示词，显式 skill 名的别名/模糊匹配策略，以及 system prompt 中推荐技能摘要的具体格式与 spec 示例的对齐），建议在写实现前先把“可测试的输入/输出契约”定清楚以免后续 prompt 形态来回改。

### enable-subagent-orchestration
子 Agent 编排是对 oneAgent 执行模型的一次大扩展：主 Agent agentic 判断是否启动 subagent，子 Agent 在隔离上下文中跑一段受限的 tool-loop，主要交付写入 `FINDINGS.md`（至少包含 `## 流水账` 与 `## Findings`，并建议列出变更文件），系统同时把完整过程写入可回溯的 jsonl 日志并在 trace 里只存摘要与指针，返回主 Agent 的结果严格控制为“短总结 + 路径引用”，且默认禁止递归启动 subagent；它与 local-tool 的强耦合在于日志目录/数据目录的统一（spec 里给出 `<workspace>/.logs/...` 示例但允许等价可配置目录）以及宿主机工具调用的默认安全边界（sandbox、步数/时长/输出限制、工具 allowlist），并且与 skills change 存在明确联动（按步骤注入 Top-K skills 的中文摘要）——因此更适合在 local-tool 的路径/日志/安全底座稳定、skills recall 能力可复用后再实现；仍需提前澄清的细节包括：子 Agent 默认工具权限是否与主一致还是最小权限、是否需要并发 subagent（以及并发下的 session/run_id、日志隔离、rate limit 与成本控制）、findings 文件命名与日志轮转/压缩策略、失败重试由系统还是主 Agent 决策、以及 XML 元信息在后端/前端/trace 中的解析与兼容策略。

## 需要你先拍板的关键问题（跨特性）
1) `ONEAGENT_HOME` 与 `<workspace>/.oneagent` 的关系：哪些数据必须跟随 workspace（例如 project-private skills），哪些默认必须落在 home（logs/cache/db），以及“默认不污染 repo”具体指哪些目录/文件。  
2) SQLite driver 选型：是否接受 CGO（影响跨平台分发与 CI），以及对 FTS5、并发写入/锁策略（主库 + skills index + trace/日志写入）的一致要求。  
3) password auth 与现有 JWT 的兼容/替换策略：`Authorization` 头的语义如何区分（password vs jwt），前端凭证持久化如何做，后端如何生成/注入单用户身份以不破坏 Settings/Session 模型。  
4) LAN/公网判定策略：是否支持 IPv6、是否尊重/忽略 `X-Forwarded-For`（反代场景）、默认阻止公网访问的“安全边界”究竟靠 bind 地址还是靠来源 IP 白名单。  
5) subagent 的默认权限与资源限制：默认 tool allowlist、最大步数/时长/输出，是否允许并发，以及失败重试策略（自动 vs agentic）。

