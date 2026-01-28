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
系统必须 (MUST) 通过 TurnContext（volatile）消息使用中文提示词注入“推荐技能摘要”，以符合用户偏好，并引导 agent 在使用前先通过 `skill.read` 读取该技能的 `SKILL.md`；系统不得 (MUST NOT) 回写稳定 system prompt。

#### Scenario: 中文提示词标题 (Scenario: Chinese Prompt Header)
- **GIVEN** 系统产生一个推荐技能
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "## 技能建议"
- **THEN** 它**必须**包含 "`skill.read`" 或等价表述（强调先通过 skill 读取工具读到 `SKILL.md` 再执行）

#### Scenario: 列表中的技能描述 (Scenario: Skill Description in List)
- **GIVEN** 系统推荐技能 "Translator"，描述为 "Expert in translation"
- **WHEN** 生成 TurnContext（volatile）注入消息时
- **THEN** 它**必须**包含 "- Translator: Expert in translation"
- **THEN** 它**必须**包含“如何读取该技能”的提示（例如包含 `skill.read("Translator")` 或等价表述）

### Requirement: Skill 读取工具（`skill.read`）
系统必须 (MUST) 提供一个 skill 读取工具（例如 `skill.read`），使 agent 可仅凭技能名称/ID 读取对应技能的 `SKILL.md` 原文；该原文必须作为工具调用输出（tool output）进入对话上下文，以便 agent 在后续思考与回复中遵循该 Skill 的指令。

该工具必须 (MUST) 按技能发现的同名覆盖规则返回“最终生效版本”（例如 `<workspace>/.oneagent` > `<workspace>/skills` > `<workspace>/.claude` > `~/.claude` > `~/.codex` > `.builtin`），且不得要求调用方提供文件路径。

#### Scenario: 仅凭技能名称读取 SKILL.md
- **GIVEN** 技能来源中存在技能 "Translator"
- **WHEN** agent 调用 `skill.read("Translator")`
- **THEN** 系统返回该技能的 `SKILL.md` 原文作为 tool output

#### Scenario: 同名冲突时返回最终生效版本
- **GIVEN** `~/.claude/skills/translator/SKILL.md` 与 `<workspace>/.oneagent/skills/translator/SKILL.md` 同时存在
- **WHEN** agent 调用 `skill.read("translator")`
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

