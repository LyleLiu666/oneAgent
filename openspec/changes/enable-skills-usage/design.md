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

### 3) 技能索引（`internal/skillindex`）
为支持“几千个技能”的场景，引入本地索引层（建议 SQLite + FTS5）：
- 索引位置：默认存放在 `ONEAGENT_HOME`（避免污染 repo），并按 workspace 做隔离（例如 `ONEAGENT_HOME/cache/skills/<workspace_id>/skill-index.sqlite`）
- 索引字段：`skill_id/name/description/tags/source/path/mtime/hash`
- 更新策略（MVP）：启动时或首次请求时“增量构建/更新”
  - 可用 `mtime+size` 或内容 hash 判断变更
  - 后续可扩展 fsnotify 监听

### 4) 独立的技能召回/挑选工具（`cmd/skill-recall` 或 `internal/skillrecall`）
新增一个独立组件负责“从索引中召回 Top-8 技能 + 从 Top-8 中选择最合适的技能”，其目标是：
- **独立于主 agent**：技能筛选流程独立于主对话 LLM（避免把“选技能”混进主对话上下文）
- **可解释/可测试**：召回阶段可复现；选择阶段可输出结构化结果与可选理由

建议提供两种入口（同一逻辑复用）：
- **CLI**：`oneagent skills search --query "..."`
- **库接口**：供 `ChatHandler` 直接调用（避免频繁 fork 进程）

召回策略（MVP，Top-8 固定）：
- 使用 FTS/关键词匹配，返回 Top-8 + score
- 可选：对 name/tags 提升权重

选择策略（第二阶段，Selector）：
- 将 Top-8 候选（仅元数据：name/description/tags/path/source/score）封装为“选择提示词”（Selector Prompt）
- 由选择器执行一次**独立的 LLM 调用（复用主对话模型）**，在 Top-8 中选出 `selected_skill` 或 `none`
- 输出结构化结果：`selected_skill_id`, `selected_skill_name`, `confidence`, `reason`（可选）

> 说明：召回阶段不依赖外部网络；选择阶段可能依赖 LLM（可配置关闭/降级为 Top-1）。

### 5) 上下文注入（`internal/handler`）
修改 `ChatHandler` 的 system prompt 构建逻辑：不再注入“全部技能”，而是注入“选择器输出（推荐技能）”。

提示词模板（中文）示例：
```text
## 技能建议（自动挑选）
以下是为本轮任务自动挑选的“最相关技能建议”。你可以自行判断是否需要使用。

- 推荐技能: [技能名称]
  - 描述: [description]
  - 来源: [.oneagent|.claude|.codex]
  - 路径: [SKILL.md 路径]

如果你决定使用该技能，你需要先读取对应技能文件以了解具体操作协议，然后再执行。
```

显式指定技能（保留能力）：
- 当用户在自然语言中明确表达“希望使用某个技能”（通常会提到技能名称）时，系统应优先解析该技能名并作为推荐技能（可跳过召回/选择，或作为 override）。

## 数据流 (Data Flow)
1. **启动 / 首次请求**：构建/更新技能索引（从 `<workspace>/.oneagent`、`~/.claude`、`~/.codex` 扫描；取并集后按 name 去重）。
2. **聊天请求**：`ChatHandler` 获取用户输入（以及可选上下文，如系统提示词/会话摘要）。
3. **显式技能解析（可选）**：若用户语义上明确指定技能名称，则解析为 `selected_skill`（可作为 override）。
4. **召回（无显式指定时）**：调用召回工具 `Search(query)` 得到 Top-8 候选。
5. **选择（Selector）**：用 Selector Prompt 在 Top-8 中选出 `selected_skill` 或 `none`。
6. **提示词构建**：将“中文技能建议（推荐技能摘要）”追加到 `SystemPrompt`。
7. **Agent 行动**：主对话 LLM 看到推荐技能后，自行判断是否需要使用；若需要，通过现有工具读取 `SKILL.md` 并按协议执行。

## 权衡 (Trade-offs)
- **扫描 vs 索引**：直接扫描文件夹简单但不适合几千技能；索引增加复杂度但换来性能与可控性。
- **FTS vs Embedding**：FTS 可解释、无外部依赖；Embedding 召回质量可能更好，但需要模型/成本与隐私权衡。建议先 FTS（MVP），后续再扩展。
- **目录兼容性**：`.claude` 的真实组织结构可能存在差异；设计上应允许通过配置扩展扫描规则。

## 开放问题 (Open Questions)
1. 召回 query 的构成：仅用“用户最后一句”，还是包含系统提示词/会话摘要/最近 N 轮？
2. 语义指定技能名的解析策略：只支持“直接提到技能名称”，还是要支持别名/中文名映射与模糊匹配？
