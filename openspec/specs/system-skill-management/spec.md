# system-skill-management Specification

## Purpose
TBD - created by archiving change enable-skills-usage. Update Purpose after archive.
## Requirements
### Requirement: 多来源技能发现 (Multi-Source Skill Discovery)
系统必须 (MUST) 从以下位置发现技能文件（`SKILL.md`），并支持以“技能目录包”的方式组织（目录内包含 `SKILL.md`，可选包含 `scripts/`、`references/` 等）。系统应对以上来源取并集后按技能 `name` 去重（同名冲突按稳定规则消解）：
1) `<workspace>/.oneagent/skills/**/SKILL.md`
2) `<workspace>/skills/**/SKILL.md`
3) `<workspace>/.claude/skills/**/SKILL.md`
4) `ONEAGENT_HOME/.oneagent/skills/**/SKILL.md`（oneAgent-managed personal skills，active）
5) `~/.claude/skills/**/SKILL.md`
6) `~/.codex/skills/**/SKILL.md`
7) 内置 skills（随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

#### Scenario: 发现 ONEAGENT_HOME 下的个人 skills
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills/my-sop/SKILL.md` 存在
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中包含该技能（source=`.oneagent_home` 或等价 source）

#### Scenario: 归档的个人 skills 不参与发现
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills-archived/old-sop/SKILL.md` 存在
- **WHEN** 系统加载技能目录
- **THEN** 技能列表中不包含该技能（archived skills 不得参与 discovery/recall）

#### Scenario: workspace skills 覆盖个人 skills
- **GIVEN** `ONEAGENT_HOME/.oneagent/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** 系统加载技能目录
- **THEN** 仅保留 `<workspace>/.oneagent` 版本作为最终生效技能（同名覆盖）

### Requirement: 召回式技能推荐 (Recall Recommendation)
系统必须 (MUST) 在聊天开始前使用“技能召回工具”从全量技能中召回 Top-8，并从中选择 Top-1 作为“推荐技能”（或无推荐），提供给主 agent。

#### Scenario: 海量技能时仅召回 Top-8 且不注入全量列表
- **GIVEN** 可用技能数量为 1000+
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 技能召回阶段只处理 Top-8 候选（固定为 8）
- **THEN** TurnContext 不得出现“全量技能列表”或“把所有技能都列出”的行为
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

#### Scenario: 无推荐技能时不注入推荐块
- **GIVEN** 技能召回工具返回空结果（无候选）
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** TurnContext 不包含“## 技能建议”块（或包含但明确说明无推荐技能）
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

### Requirement: 显式指定技能名称（语义）(Explicit Skill Selection, Semantic)
系统必须 (MUST) 保留“按技能名称指定”的使用方式：当用户在自然语言中明确表达要使用某个技能（通常会提到技能名称）时，系统应优先解析并推荐该技能（可跳过召回/选择流程或作为 override）。

#### Scenario: 用户自然语言指定技能名优先生效
- **GIVEN** 用户输入包含 “请使用 code-review-excellence 这个技能”
- **WHEN** 构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 系统将 `code-review-excellence` 作为推荐技能（前提：该技能存在）
- **THEN** 稳定 system prompt 不得被回写（避免破坏 KV cache）

### Requirement: 中文上下文注入 (Chinese Context Injection)
系统必须 (MUST) 通过 TurnContext（volatile）消息使用中文提示词注入“推荐技能摘要”，以符合用户偏好，并引导 agent 在使用前先通过 `skill_read` 读取该技能的 `SKILL.md`；系统不得 (MUST NOT) 回写稳定 system prompt。

系统必须 (MUST) 在过渡期内兼容历史名称 `skill.read`（alias，best-effort）。

#### Scenario: 中文提示词标题 (Scenario: Chinese Prompt Header)
- **GIVEN** 系统产生一个推荐技能
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "## 技能建议"
- **THEN** 它**必须**包含 "`skill_read`" 或等价表述（强调先通过 skill 读取工具读到 `SKILL.md` 再执行）

#### Scenario: 列表中的技能描述 (Scenario: Skill Description in List)
- **GIVEN** 系统推荐技能 "Translator"，描述为 "Expert in translation"
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "- Translator: Expert in translation"
- **THEN** 它**必须**包含“如何读取该技能”的提示（例如包含 `skill_read(name="Translator")` 或等价表述）

### Requirement: Skill 读取工具（`skill.read`）
系统必须 (MUST) 提供一个 skill 读取工具（canonical name 为 `skill_read`；并兼容 `skill.read` alias），使 agent 可仅凭技能名称/ID 读取对应技能的 `SKILL.md` 原文；该原文必须作为工具调用输出（tool output）进入对话上下文，以便 agent 在后续思考与回复中遵循该 Skill 的指令。

该工具必须 (MUST) 按技能发现的同名覆盖规则返回“最终生效版本”（例如 `<workspace>/.oneagent` > `<workspace>/skills` > `<workspace>/.claude` > `~/.claude` > `~/.codex` > `.builtin`），且不得要求调用方提供文件路径。

#### Scenario: 仅凭技能名称读取 SKILL.md
- **GIVEN** 技能来源中存在技能 "Translator"
- **WHEN** agent 调用 `skill_read(name="Translator")`
- **THEN** 系统返回该技能的 `SKILL.md` 原文作为 tool output

#### Scenario: 同名冲突时返回最终生效版本
- **GIVEN** `~/.claude/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** agent 调用 `skill_read(name="translator")`
- **THEN** 系统返回 `.oneagent` 版本的 `SKILL.md` 原文作为 tool output

### Requirement: Skill Eligibility Metadata (requires/install)
系统必须 (MUST) 支持在 `SKILL.md` 的 YAML frontmatter 中声明可选的 `requires` 与 `install` 元数据，用于判断技能在当前环境是否可用（eligible）以及向用户展示安装建议。

`requires` 支持以下字段：
- `os`: 允许的 OS 列表（非空时必须包含当前 OS）
- `bins`: 必需存在的可执行文件名列表（全部满足）
- `any_bins`: 至少存在其一的可执行文件名列表
- `env`: 必需存在的环境变量名列表

`install` 支持结构化安装建议（例如 brew/go/node/download/command），用于 status/check 输出。

#### Scenario: 缺少依赖的 skill 不应被自动推荐
- **GIVEN** 存在 skill A，声明 `requires.bins=["memo"]`
- **AND** 当前运行环境缺少 `memo` 可执行文件
- **WHEN** 系统构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** 系统不得把 skill A 作为自动推荐技能

### Requirement: Skill Toolkit Metadata (tool_ids)
系统必须 (MUST) 支持在 `SKILL.md` 的 YAML frontmatter 中声明可选的 `tool_ids` 元数据，用于将某个 skill 作为“工具包技能（toolkit skill）”：
- `tool_ids`: 推荐的工具 ID 列表（用于 subagent 挂载）

注意：`tool_ids` 仅表达“建议工具集合”，不代表权限；最终是否可挂载仍受 tool policy/approval gate 约束（fail-closed）。

#### Scenario: Skill frontmatter tool_ids is discoverable
- **GIVEN** 某 skill 的 `SKILL.md` frontmatter 声明 `tool_ids=["rg","read_file"]`
- **WHEN** 系统执行 skills discovery 并构建 skill catalog
- **THEN** catalog 中该 skill 的元数据包含 `tool_ids=["rg","read_file"]`（best-effort）

#### Scenario: Toolkit skill recommendation surfaces tool_ids in TurnContext
- **GIVEN** 系统自动推荐的 Top-1 skill 是一个 toolkit skill（其 frontmatter 声明了 tool_ids）
- **WHEN** 系统构建本轮 TurnContext（volatile）的“技能建议”注入消息
- **THEN** TurnContext 中包含该 skill 的 `tool_ids` 摘要（best-effort）

### Requirement: Skill Status/Check CLI
系统必须 (MUST) 提供 CLI 用于检查技能可用性，并展示缺失依赖与安装建议：
- `oneagent skills status`：列出技能可用性信息
- `oneagent skills check`：`status` 的别名

#### Scenario: status 输出包含 missing 与 install hints
- **GIVEN** 存在 skill B，声明 `requires.bins=["memo"]` 且 `install` 提供 brew 安装方式
- **AND** 当前环境缺少 `memo`
- **WHEN** 用户运行 `oneagent skills status`
- **THEN** 输出中包含 skill B 的 missing 信息（包含 `memo`）
- **THEN** 输出中包含与 brew 对应的安装提示（例如 `brew install ...` 或 command）

### Requirement: Skill governance MUST support pinning a canonical skill
系统必须 (MUST) 支持将某个 skill candidate “pin”为 oneAgent personal canonical，以便在同名冲突时稳定选择该版本参与 discovery/recall。

#### Scenario: Pin shadows lower-precedence candidates
- **GIVEN** skill `foo` 在 `~/.claude/skills/foo` 存在一个版本
- **WHEN** 用户对该 candidate 执行 pin
- **THEN** 系统在 `ONEAGENT_HOME/.oneagent/skills/foo/SKILL.md` 生成 personal canonical
- **AND** 后续 `skill.Discover` 中 `foo` 的最终生效版本为 personal canonical

#### Scenario: Pin is idempotent for same source
- **GIVEN** 某 candidate 已被 pin 为 personal canonical
- **WHEN** 用户再次对同一个 candidate 执行 pin
- **THEN** 系统不会生成重复 skill
- **AND** 返回的 canonical_path 保持稳定（指向同一位置）

### Requirement: Skill governance MUST support archiving shadowed personal duplicates
系统必须 (MUST) 提供“archive shadowed personal duplicates”的治理动作，用于将同名的旧 personal skill 版本归档，以降低干扰与误召回。

#### Scenario: Archive affects only personal scope
- **GIVEN** skill `foo` 在多个来源存在 duplicates（包含 personal 与非 personal）
- **WHEN** 用户执行 archive shadowed
- **THEN** 系统只归档 personal 范围内被 shadow 的版本
- **AND** 不会尝试修改不可写来源（例如 `~/.claude`）

### Requirement: Skill governance UI MUST use progressive disclosure
系统必须 (MUST) 在技能治理 UI 中使用渐进式披露，默认仅展示高频入口，并将低频/高级操作折叠起来。

高频入口至少包含：
- 技能列表与选择
- 个人技能的保存/归档（当可用时）

低频/高级内容（例如 duplicates/pin/archive-shadowed、路径/sha 等诊断信息）必须 (MUST) 默认折叠，并提供可发现的展开入口。

#### Scenario: Advanced governance actions are collapsed by default
- **GIVEN** 用户打开技能治理页面
- **WHEN** 页面首次渲染完成
- **THEN** 高级区域默认处于折叠状态
- **AND** 页面仍可完成技能查看/编辑/归档等高频操作

#### Scenario: User can expand advanced section to manage duplicates
- **GIVEN** 页面存在 duplicates 治理能力
- **WHEN** 用户展开高级区域
- **THEN** 用户可以执行 pin/archive-shadowed 等低频治理操作（best-effort）

### Requirement: Skill governance workbench MUST support listing and archiving
系统必须 (MUST) 提供一个治理入口，用于列出当前可用技能，并支持对个人技能执行归档操作。

#### Scenario: 列出 skills
- **WHEN** 用户打开 skill 治理页面
- **THEN** 系统返回可发现的 skills 列表（至少包含 id/name/description/source/path）

#### Scenario: 归档 oneAgent personal skill
- **GIVEN** 某 skill 来源为 oneAgent personal skills
- **WHEN** 用户执行 archive
- **THEN** skill 被移动到 archived 位置
- **AND** 后续列出 skills 时不再包含该 skill

### Requirement: Skill governance MUST expose duplicates candidates
系统必须 (MUST) 提供一个治理 API，用于列出“同名冲突”的 skill candidates，并标记每组中最终生效版本。

#### Scenario: 列出 duplicates
- **GIVEN** 同一 `skill_id` 在不同来源存在多个版本
- **WHEN** 客户端请求 `GET /api/skills/duplicates`
- **THEN** 响应按 `skill_id` 分组返回 candidates
- **AND** 每个 candidate 至少包含 `source/path/archivable/effective`

#### Scenario: effective 选择与 Discover 一致
- **GIVEN** `<workspace>/.oneagent/skills/foo` 与 `ONEAGENT_HOME/.oneagent/skills/foo` 同时存在
- **WHEN** 客户端请求 `GET /api/skills/duplicates`
- **THEN** `<workspace>/.oneagent` 版本标记为 `effective=true`
- **AND** 其他版本标记为 `effective=false`

### Requirement: Personal skills MUST be viewable and editable via API
系统必须 (MUST) 为 oneAgent personal skills 提供“查看与编辑 SKILL.md”的能力，作为治理工作台的基础。

#### Scenario: 获取 skill 详情包含 SKILL.md 与 sha
- **GIVEN** 某 skill 为 oneAgent personal skills
- **WHEN** 客户端请求 `GET /api/skills/:id`
- **THEN** 响应包含 `skill_md`（完整 SKILL.md 原文）与 `sha256`（用于 OCC）

#### Scenario: 使用 OCC 更新 SKILL.md
- **GIVEN** 客户端已读取到 `sha256`
- **WHEN** 客户端请求 `PUT /api/skills/:id` 并携带 `expected_sha256`
- **THEN** 当文件未变化时更新成功
- **AND** 当文件已变化时必须拒绝并返回明确错误

### Requirement: The system MUST record skill usage signals (best-effort)
系统必须 (MUST) 记录技能使用信号（best-effort），用于后续 staleness 判断与治理：
- `last_used_at`（最后一次被实际使用/执行的时间 best-effort）
- `used_count`（累计使用次数 best-effort）

#### Scenario: Skill usage updates last_used_at
- **GIVEN** 某 skill 被一次 task/attempt 实际使用（best-effort）
- **WHEN** 该次 attempt 进入终态并写入证据链（best-effort）
- **THEN** 该 skill 的 `last_used_at` 被更新（best-effort）

### Requirement: The system MUST support staleness detection and retirement actions (best-effort)
系统必须 (MUST) 支持对个人 skills 的 staleness 检测与淘汰治理（best-effort）：
- 系统可列出 stale candidates（best-effort）
- 用户可对 stale skill 执行 deprecate/archive，并保留 `reason`（best-effort）
- 被 deprecate/archive 的 skill 不得 (MUST NOT) 参与 discovery/recall（与现有 archived 语义一致）

#### Scenario: Deprecated skill is excluded from recall
- **GIVEN** 某 personal skill 被标记为 deprecated/archive（best-effort）
- **WHEN** 系统进行 skills discovery/recall
- **THEN** 该 skill 不出现在可用 skills 集合中（best-effort）

### Requirement: Skill governance UI MUST display human-readable skill titles
系统必须 (MUST) 在技能治理页面中将 skill 标识展示为更易读的形式（例如 Title Case、人类化分词），并在需要时仍可查看原始 `skill_id`（best-effort）。

#### Scenario: Skill list shows readable name while preserving ID
- **GIVEN** 存在 skill `code-review-excellence`
- **WHEN** 用户浏览技能列表
- **THEN** 列表展示一个可读标题（best-effort）
- **AND** 用户仍可在详情中看到原始 `skill_id`

### Requirement: Skill governance editor MUST have safe empty state and clear save affordance
系统必须 (MUST) 在技能编辑器区域提供明确的空状态；当未选中可编辑 skill 时，“保存”按钮必须不可用且视觉上不应误导用户可点击。

#### Scenario: Save is disabled and not misleading before selection
- **GIVEN** 用户打开技能治理页面且未选中任何 skill
- **WHEN** 页面渲染
- **THEN** 编辑器展示“请选择技能”的空状态
- **AND** 保存按钮处于禁用状态且具有清晰的禁用反馈（best-effort）

### Requirement: The system MUST inject a "skills help" TurnContext when user asks about available skills
当用户显式询问“有哪些/可用的 skills/技能”时，系统必须 (MUST) 在 TurnContext（volatile）注入一个“技能帮助”块，用于：
- 引导用户在 UI 中查看 skills 列表（例如 `/governance/skills`）
- 引导用户在 CLI 中检查可用性（例如 `oneagent skills status`）
- （可选）展示 Top-N（N≤10）技能名称摘要，避免模型/用户靠猜

该注入不得 (MUST NOT) 回写稳定 system prompt（避免破坏 KV cache）。

#### Scenario: User asks about available skills triggers help block
- **GIVEN** 用户输入包含“有哪些技能/skills 列表/可用 skills”等显式询问
- **WHEN** 系统生成本轮 TurnContext（volatile）注入消息
- **THEN** TurnContext 包含“技能帮助”块与下一步指引（UI + CLI）
- **AND** （可选）包含 Top-N（N≤10）技能摘要

### Requirement: Skill read tool (`skill.read`) MUST provide actionable not-found errors
当 agent 调用 `skill.read` 且请求的 skill 不存在时，工具必须 (MUST) 返回**可行动**的错误信息，而不是仅返回“not found”。该错误信息应包含（best-effort）：
- 规范化后的 skill 标识（normalized id）
- 相似候选 suggestions（≤5）
- 下一步指引（例如打开技能治理页面查看列表，或运行 `oneagent skills status` 检查可用性）

#### Scenario: skill.read not-found includes normalized id, suggestions, and next steps
- **GIVEN** skill `translator` 不存在
- **WHEN** agent 调用 `skill.read`（name 或 skill_id 为 `translator`）
- **THEN** 工具返回的错误信息包含 normalized id
- **AND** 错误信息包含 ≤5 个相似候选（best-effort）
- **AND** 错误信息包含可执行的 next steps（best-effort）
