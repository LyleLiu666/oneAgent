# 规范: 技能召回与挑选 (Skill Recall & Selection)

## ADDED Requirements

### Requirement: 基础召回独立于主 agent 且无需外部网络
技能召回工具必须 (MUST) 在本地运行，独立于主 agent（主对话 LLM 推理），并且不得依赖外部网络服务来完成“基础召回”（Top-8 候选生成）。

#### Scenario: 离线可用
- **GIVEN** 运行环境不可访问互联网
- **WHEN** 执行技能召回（基于本地索引/本地文件）
- **THEN** 工具仍可返回技能候选列表（不因网络不可用而失败）

### Requirement: 支持构建与更新本地技能索引
技能召回工具必须 (MUST) 能从以下来源发现技能，并构建/更新本地索引，以支持上千技能规模的快速查询（对三处来源取并集后按 `name` 去重）：
1) `<workspace>/.oneagent/skills`
2) `~/.claude/skills`
3) `~/.codex/skills`

#### Scenario: 索引不存在时自动创建
- **GIVEN** 工作区存在至少一个技能文件且本地索引文件不存在
- **WHEN** 执行索引构建/更新
- **THEN** 生成索引文件（例如位于 `.oneagent/cache/skill-index.sqlite`）

#### Scenario: 技能变更触发索引更新
- **GIVEN** 索引已存在且某个 `SKILL.md` 内容或元数据发生变化
- **WHEN** 执行索引构建/更新
- **THEN** 索引中的该技能记录被更新（例如 mtime/hash 变化可反映）

#### Scenario: 同名技能按规则去重
- **GIVEN** `~/.codex/skills/translator/SKILL.md` 与 `~/.claude/skills/translator/SKILL.md` 同时存在
- **WHEN** 执行索引构建/更新
- **THEN** 索引中仅存在一个 name="Translator"（或其规范化等价）的技能记录
- **THEN** 该记录的 source 为 `.claude`（同名冲突按稳定规则消解）

### Requirement: 召回接口固定返回 Top-8 并返回可用字段
技能召回工具必须 (MUST) 提供一个查询接口（CLI 或库 API），输入 query，输出 Top-8 技能候选列表，并包含至少以下字段：
`skill_id`, `name`, `description`, `source`, `path`, `score`

#### Scenario: 查询返回 Top-8 并包含必要字段
- **GIVEN** 索引中存在技能 "Translator"（description 包含 "translation"）
- **WHEN** 以 query="translation" 执行召回
- **THEN** 返回结果数量不超过 8
- **THEN** 结果中包含 "Translator"
- **THEN** 每条结果包含 `skill_id/name/description/source/path/score`

### Requirement: 召回结果顺序稳定
在索引内容不变的前提下，技能召回/挑选工具必须 (MUST) 对同一 query 返回稳定的排序结果（避免同分随机顺序导致提示词抖动）。

#### Scenario: 相同输入得到相同输出顺序
- **GIVEN** 索引内容未变化
- **WHEN** 连续两次以相同 query 执行召回
- **THEN** 返回结果的排序顺序一致

### Requirement: 在 Top-8 中执行选择（Selector Prompt）
技能选择器必须 (MUST) 在召回得到的 Top-8 候选中，通过一个明确的 Selector Prompt 选出“最合适的技能”或返回 `none`，并输出结构化结果供主 agent 决策是否使用。选择器必须 (MUST) 复用主对话模型（由调用方传入/复用同一 provider+model 配置）。

#### Scenario: 选择器返回选中的技能
- **GIVEN** Top-8 候选中包含 "code-review-excellence" 与若干其它技能
- **WHEN** 以 query="帮我 review 这个 PR" 执行选择器
- **THEN** 选择器返回 `selected_skill_id`（例如 "code-review-excellence"）或 `none`
- **THEN** 返回结果包含 `selected_skill_id/selected_skill_name/confidence` 字段

#### Scenario: 选择器复用主对话模型
- **GIVEN** 调用方为本轮对话选择了模型 `ModelX`
- **WHEN** 执行选择器
- **THEN** 选择器使用同一个模型 `ModelX`（或等价配置）完成选择

### Requirement: 支持语义显式指定技能名的解析
技能召回/挑选工具必须 (MUST) 支持通过“技能名称”直接解析技能（无需依赖特殊语法），用于处理用户自然语言中明确表达“要使用某技能”的情况。

#### Scenario: 自然语言指定技能名可解析
- **GIVEN** 索引中存在技能 `code-review-excellence`
- **WHEN** 输入 query="请使用 code-review-excellence 这个技能，帮我 review 这个 PR"
- **THEN** 工具可以将 `code-review-excellence` 解析为候选/推荐技能（可作为 override）

#### Scenario: 选择器可配置为不调用 LLM
- **GIVEN** 运行环境无法访问 LLM（或显式关闭 selector LLM）
- **WHEN** 执行选择器
- **THEN** 系统不因无法调用 LLM 而整体失败（例如降级为 Top-1 或返回 none，并给出原因）
