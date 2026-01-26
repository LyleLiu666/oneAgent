# 设计: 启用技能使用 (Enable Skills Usage)

## 组件 (Components)

### 1) 技能目录与来源管理（`internal/skill`）
一个新的包，负责从多个来源发现技能，并抽象成统一的 `Skill` 列表。

#### 支持的来源（本阶段）
- **oneAgent 项目私有目录**：`<workspace>/.oneagent/skills/**/SKILL.md`（仅当会话启用 workspace 时生效）
- **Claude Code 全局目录**：`~/.claude/skills/**/SKILL.md`（`~` 表示用户主目录）
- **Codex 全局目录**：`~/.codex/skills/**/SKILL.md`（`~` 表示用户主目录）

> 参考实际结构：`~/.claude/skills` 与 `~/.codex/skills` 下常见“技能目录为 symlink 指向真实目录（例如 `~/.agents/skills/...`）”。扫描时必须正确处理 symlink（需要跟随目录 symlink），并避免循环引用。

#### 并集与去重（按名称）
三处来源默认全部启用，系统取并集后以技能 `name` 去重（建议使用“规范化 name”去重，例如 trim + lowercase + 将空格归一化）。

同名冲突时需可预测、稳定：
- 优先保留 `<workspace>/.oneagent` 版本（项目私有覆盖全局）
- 其次保留 `~/.claude` 版本
- 最后保留 `~/.codex` 版本

`skill_id` 建议为 `normalized_name`（或其稳定 hash），以便：
- 召回/选择/提示词注入使用同一稳定标识
- 同名跨来源仍能聚合为同一 skill

### 2) 技能元数据解析（`internal/skill`）
技能文件以 Markdown 为主，推荐使用 YAML frontmatter：
- `name`: 技能名称
- `description`: 简述
- `tags`/`keywords`: 召回关键词（可选）

回退策略：
- `name`: 文件夹名或文件名
- `description`: 取首段文本（做长度截断）

### 3) 技能召回：基于 grep/ripgrep（不引入持久化索引）
本阶段不引入 SQLite FTS 索引（避免把 SQLite 扩展成“通用存储/索引”，也降低发布复杂度）。技能召回采用“扫描 + grep”方式即可覆盖 1000+ skills：
- `internal/skill` 负责发现三处来源的 `SKILL.md` 并解析元数据（name/description/tags/path/source）。
- `internal/skillrecall` 负责对 query 做召回与排序：
  - **优先使用 `rg`**（ripgrep）对候选 `SKILL.md` 做内容匹配计分（可跟随 symlink；并发/超时可控）。
  - 若 `rg` 不可用，降级为 `grep -R`（或等价实现）以保持基本可用，并在 `doctor` 中提示安装 `rg`。
  - 排序必须稳定（score 相同用 `skill_id` 或 `path` 做 tie-break），避免 prompt 抖动。

> 评估：`go-memdb` 更适合结构化查询与内存索引，但对“全文召回”帮助有限（仍需自己实现倒排/分词/权重），复杂度与收益不匹配；`rg` 在 macOS/Linux 下性能极佳且实现成本最低，因此本阶段选择 `rg` 方案。

### 4) 独立的技能召回工具（`cmd/skill-recall` 或 `internal/skillrecall`）
新增一个独立组件负责“从技能集合中召回 Top-8 技能”，其目标是：
- **独立于主 agent**：技能筛选流程独立于主对话 LLM（避免把“选技能”混进主对话上下文）
- **可解释/可测试**：召回结果可复现（同一输入同一输出顺序）

建议提供两种入口（同一逻辑复用）：
- **CLI**：`oneagent skills search --query "..."`
- **库接口**：供 `ChatHandler` 直接调用（避免频繁 fork 进程）

召回策略（Top-8 固定）：
- 对 name/description/tags 做基础匹配计分
- 使用 `rg` 对 `SKILL.md` 内容做补充计分（可选，但建议）
- 返回 Top-8 + score（稳定排序）

> 本阶段不引入“Selector Prompt + 二次 LLM 调用”的选择器；推荐技能仅以 Top-1 摘要形式注入到 TurnContext（volatile），不得破坏 Stable Prefix（见 `optimize-kv-cache`）。

### 5) 上下文注入（`internal/handler`）
修改 `ChatHandler` 的 prompt 构建逻辑：不再把“技能信息”拼进稳定 system prompt，而是在 **Stable Prefix 之后**追加一个 TurnContext（volatile）消息，写入“推荐技能摘要”（Top-1 或 none）。

该 TurnContext 必须被视为动态上下文：
- 不得被 cache selector 选为 cacheable（当 provider 需要显式 cache 标记时）
- 不得导致稳定前缀发生变化（不得回写/编辑 system message）

提示词模板（中文）示例：
```text
## 技能建议（自动挑选）
以下是为本轮任务自动挑选的“最相关技能建议”。你可以自行判断是否需要使用。

- 推荐技能: [技能名称]
  - 描述: [description]
  - 来源: [.oneagent|.claude|.codex]
  - 使用方式: 调用 `skill.read`（按技能名称）读取该技能的 `SKILL.md`

如果你决定使用该技能，你需要先通过 skill 读取工具（`skill.read`）按技能名称读取对应 `SKILL.md`；其内容会作为工具调用输出（tool output）进入对话上下文，你在后续思考与回复中需要遵循其中的指令再执行。
```

显式指定技能（保留能力）：
- 当用户在自然语言中明确表达“希望使用某个技能”（通常会提到技能名称）时，系统应优先解析该技能名并作为推荐技能（可跳过召回/选择，或作为 override）。

## 数据流 (Data Flow)
1. **启动 / 首次请求**：扫描技能来源（从 `<workspace>/.oneagent`、`~/.claude`、`~/.codex` 扫描；取并集后按 name 去重）。
2. **聊天请求**：`ChatHandler` 获取用户输入（以及可选上下文，如系统提示词/会话摘要）。
3. **显式技能解析（可选）**：若用户语义上明确指定技能名称，则解析为 `selected_skill`（可作为 override）。
4. **召回（无显式指定时）**：调用召回工具 `Search(query)` 得到 Top-8 候选（基于 metadata + `rg` 计分）。
5. **提示词构建**：将“中文技能建议（Top-1 推荐技能摘要）”写入 TurnContext（volatile）消息（不得注入全量列表、不得修改 stable system prompt）。
6. **Agent 行动**：主对话 LLM 看到推荐技能后，自行判断是否需要使用；若需要，通过 skill 读取工具读取对应 `SKILL.md`，其内容将作为工具调用输出（tool output）进入对话上下文。

## 权衡 (Trade-offs)
- **grep vs 自建索引**：`rg` 方案实现成本低、性能高、无需引入持久化索引；代价是每次召回需要跑一次搜索（可通过缓存与 Top-K 限制控制）。
- **目录兼容性**：`.claude` / `.codex` 的真实组织结构可能存在差异；扫描需支持 symlink 且避免循环。

## 默认约定 (Defaults)
1. 召回 query：默认使用“用户最后一句 + 会话摘要（若存在）”，以提升召回相关性且保持可复现。
2. 显式技能名解析：默认按“规范化 skill name”的 case-insensitive 精确匹配（将空格/`_`/`-` 归一化），命中多个时取最长匹配；不做别名库与复杂模糊匹配（后续再扩展）。
