# 启用技能使用 (Enable Skills Usage)

## 摘要 (Summary)
增加一种机制，使 oneAgent 能够发现并使用“技能”(skills)。技能以 Markdown 文件（`SKILL.md`）为核心，通常以“技能目录包”的形式存在（目录内可包含 `scripts/`、`references/` 等资源），并且允许以符号链接（symlink）方式指向真实技能目录。

本方案需要同时支持：
1) Claude Code 的全局技能目录：`~/.claude/skills/`
2) oneAgent 的项目私有用户数据目录：`<workspace>/.oneagent/skills/`
3) Codex 的全局技能目录：`~/.codex/skills/`（结构与 Claude 类似，含 `.system` 等子目录）

三处来源默认全部启用；系统对三处来源取**并集**后按技能 `name` 去重（同名冲突的消解规则在 design 中定义）。

考虑到未来可能存在几千个技能，本方案引入一个**独立于主 agent** 的“技能召回/挑选工具”（Skill Recall Tool），用以在聊天开始前从海量技能中挑选出少量最相关的技能，并把“推荐技能摘要”写入 TurnContext（volatile）（而非回写稳定 system prompt），避免把完整技能列表塞进上下文导致膨胀，同时保持 KV cache 友好。

## 动机 (Motivation)
1. **兼容 Claude Code**：希望复用/迁移 Claude Code 的 `~/.claude/skills` 目录下已有技能资产。
2. **项目私有技能**：希望项目级私有用户数据（`<workspace>/.oneagent`）也能承载技能（不进入 git，便于个性化）。
3. **规模化可用**：当技能数量达到几千时，扫描并把全部技能注入提示词将不可行，需要“召回 + 精选”的机制。
4. **独立性**：技能召回/挑选应独立于主 agent（LLM 推理），尽可能以可解释、可测试、可控的方式运行。
5. **中文偏好**：技能相关提示词与说明采用中文，便于用户理解与维护。

## 建议方案 (Proposed Solution)
### 1) 技能来源（Sources）
默认启用以下三处来源，扫描 `**/SKILL.md`：
1. `<workspace>/.oneagent/skills/**/SKILL.md`（项目私有技能）
2. `~/.claude/skills/**/SKILL.md`（Claude Code 全局技能）
3. `~/.codex/skills/**/SKILL.md`（Codex 全局技能）

系统将三处来源取并集后，以技能 `name` 去重。

> 注：从实际目录结构看（`~/.claude/skills` 与 `~/.codex/skills`），技能通常是“目录包”，并且常见为 symlink 指向其它目录（例如 `~/.agents/skills/...`）。扫描时必须正确处理 symlink。

### 2) 技能元数据（Metadata）
每个技能文件支持解析 YAML frontmatter（若存在），至少包含：
- `name`：技能名称
- `description`：一句话描述
- `tags`/`keywords`：用于召回的关键词（可选）

若未提供 frontmatter，则回退到文件名/目录名作为 `name`，并截取首段文本作为 `description`（安全截断）。

### 3) 不引入持久化索引：基于 grep/ripgrep 的召回
为支持上千技能规模，本阶段不引入 SQLite FTS 索引，而是采用“扫描 + grep”：
- 由技能召回工具在本地扫描三处来源得到 `SKILL.md` 列表（含 metadata）
- 对 query 优先使用 `rg`（ripgrep）在 `SKILL.md` 上做匹配计分，返回 Top-8（稳定排序）；当 `rg` 缺失时允许降级为 `grep -R` 以保持基本可用

> 说明：不支持 Windows 的前提下，`rg` 在 macOS/Linux 性能与可用性更好；若 `rg` 缺失，应由 `doctor` 提示安装，同时系统允许降级为 `grep -R`。

### 4) 独立的技能召回工具（Skill Recall Tool）
新增一个独立组件（可作为可执行工具或内部库 + CLI）：
- 输入：本轮任务描述（用户消息）与可选上下文（例如系统提示词、会话摘要）
- 输出：Top-8 技能候选（`id/name/description/path/score`），并可带上简短“命中原因”（可选）
- 召回策略：基于 metadata + 内容匹配计分（优先 `rg`，缺失时降级 `grep -R`），后续可扩展 embedding / rerank

> 本阶段不引入“Selector Prompt + 二次 LLM 调用”的选择器；推荐技能直接取 Top-1（或无推荐）。

### 5) 提示词注入（Injection）
`ChatHandler` 在构建 prompt 时：
- 调用技能召回工具召回 Top-8，并取 Top-1 作为推荐技能（或 `none`）
- 将“推荐技能摘要”以中文形式写入 TurnContext（volatile）消息（避免注入全量或大列表；不得回写稳定 system prompt，避免破坏 KV cache）
- 技能生效方式：如需使用某技能，agent 必须先通过 skill 读取工具（`skill.read`）按技能名称读取对应 `SKILL.md`；该内容作为工具调用输出（tool output）进入对话上下文，agent 在后续思考与回复中遵循其操作协议

### 6) 保留“指定技能名称”的使用方式（Explicit Skill）
除“召回 + 选择”路径外，系统仍需保留“按技能名称指定”的使用方式：
- 当用户在请求中以**自然语言语义**明确表达要使用某个技能（通常会提到技能名称）
- 系统应解析并定位该技能（可通过索引按 `name` 查找/模糊匹配），并将其作为“推荐技能”返回给主 agent
- 若指定的技能不存在，则给出明确提示并回退到召回流程（或不使用技能）
