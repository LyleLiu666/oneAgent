# 设计: 启用子 Agent 编排 (Enable Sub-Agent Orchestration)

## 目标 (Goals)
- 将“复杂任务 = 多步骤”的执行拆分为多个隔离上下文的子 Agent 运行单元
- 在步骤间只保留“短总结 + findings 文件引用”，降低主上下文膨胀与思维污染
- 支持按步骤注入不同 skill，模拟人类在不同工作模式间切换
- 可观测、可控、可测试（TDD）

## 核心概念 (Core Concepts)

### 1) 子 Agent（Sub-Agent）
子 Agent 是一次独立的 LLM 执行单元，拥有独立的上下文窗口与（可配置的）工具集合。

### 2) 交接摘要（Handoff）
子 Agent 执行完成后会产出一个**findings 文件**（例如 `FINDINGS.md`），用于可追溯与回溯；同时返回给主 Agent 的只是一段**人类式短总结**（类似“我把后端开发完了，下一步做前端”）+ 该文件的引用。

findings 文件中包含两类核心信息：
- **流水账**：过程的关键节点（可追溯）
- **Findings**：结论/约束/决定/待办（可继续推进）

### 3) 上下文切割（Context Slicing）
子 Agent 的输入默认仅包含：
- 本步骤目标/约束/验收标准（详细）
- 前序步骤的“短总结 + findings 文件引用”（简短）
而不包含前序步骤的完整对话与工具执行过程。

### 4) agentic 的启动决策（不要写死流程）
系统不应把“拆分步骤 → 必走子 Agent”写死成固定流程。主 Agent 需要 agentic 判断是否值得启用子 Agent：
- **适合启用**：边界清晰、可以独立完成、产出明确（代码/文档/文件）且不需要频繁回看历史
- **不适合启用**：强耦合、需要不断复用大量上下文、需要与主 Agent 高频互动的复杂联动修改

实现上：系统只提供 `subagent` 能力与必要约束，是否调用由主 Agent 决定。

## 组件 (Components)

### 1) 子 Agent 运行器（`internal/subagent`）
职责：
- 组装子 Agent 的 system prompt（基于主 prompt + 交接模板 + skills 注入）
- 选择模型与工具集合（allowlist）
- 运行 tool-loop（复用或抽象现有逻辑），并限制最大步数/时长
- 引导子 Agent 生成 findings 文件，并返回“短总结 + 引用路径”给调用方

### 2) 子 Agent 工具（`internal/tool` 新增 `subagent`）
建议以 tool 形式暴露给主 Agent（便于在现有 tool-loop 中直接调用）：
- 参数：`task`、`context_summary`、`k_skills` / `skill_ids`、`tool_ids`、`model_id`、`max_steps` 等
- 返回：`summary`（短总结）、`findings_path`、`trace_log_path`、`run_id`（可选）、`stats`（tokens/耗时/错误）

> 默认：子 Agent 不挂载 `subagent` 工具本身，避免递归。

### 3) 技能注入适配（依赖/联动 `enable-skills-usage`）
当 skill recall 工具可用时：
- `internal/subagent` 在启动前调用“召回工具”获取 Top-K skills
- 将 Top-K 技能摘要块追加到子 Agent system prompt（中文）

### 4) 观测与存储（Trace / 可选 Session）
MVP 推荐：
- **完整痕迹落日志文件**，数据库仅保存摘要与指针，降低未来 SQLite 的压力：
  - `trace_log_path`：记录子 Agent 的完整过程（消息、工具调用/结果、错误、时间等）
  - `findings_path`：记录交付件（流水账 + Findings + 修改文件列表）
- 在主会话的 trace 中记录 `TraceTypeSubAgent` 条目，包含摘要与文件指针：
  - 输入：task + context_summary（受长度限制）
  - 输出：summary + findings_path + trace_log_path（受长度限制）
  - metadata：parent_session_id、run_id、depth、model、tool_ids、耗时等

建议目录结构（仅示意，可配置）：
- `ONEAGENT_HOME/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/trace.jsonl`
- `ONEAGENT_HOME/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/FINDINGS.md`

## 交接格式 (Handoff Format)
子 Agent 的交付以**文件**为主，而不是把所有信息塞回 tool result 文本。

### Findings 文件模板（建议）
`FINDINGS.md` 至少包含：
- `## 流水账`：关键动作/事件（条目）
- `## Findings`：关键结论、约束、决定、待办（条目）
- `## 变更文件`：新增/修改的文件路径（条目，可选但建议）

### 返回给主 Agent 的“短总结”
`subagent` 工具返回给主 Agent 的应当是：
- `summary`：一句/几句总结 + 下一步建议（类似人类阶段性收尾）
- `findings_path`：指向 `FINDINGS.md` 的路径
- `trace_log_path`：指向完整日志的路径（jsonl，用于回溯，不用于塞进上下文）

### 结构化元信息：优先 XML
如确实需要结构化返回（例如前端要解析字段），推荐返回 XML（默认节点内容按 CDATA 处理，避免强校验与转义问题）：
```xml
<subagent_handoff>
  <summary>...</summary>
  <findings_path>...</findings_path>
  <trace_log_path>...</trace_log_path>
  <run_id>...</run_id>
</subagent_handoff>
```

## 数据流 (Data Flow)
1. 主 Agent agentic 判断任务是否适合拆分/是否需要子 Agent
2. 主 Agent（可选）将任务拆为步骤（可由用户/LLM 产生）
3. 主 Agent 调用 `subagent` 工具，传入本步骤 task + 前序短总结/引用
4. `internal/subagent` 构建 prompt（主 prompt + skills + handoff 约束）
5. 子 Agent 运行 tool-loop 执行任务
6. 子 Agent 生成 `FINDINGS.md`（交付件）与完整 trace 日志
7. 系统校验交付件存在与基本结构，并将“短总结 + 引用路径”作为 tool result 返回主 Agent（同时写 trace 指针）
8. 主 Agent 用短总结推进下一步，必要时再读取 findings 文件（完成“清空大脑→切换模式”）

## 安全与限制 (Safety & Limits)
- 最大深度：默认 depth=1（子 Agent 不允许再启动子 Agent）
- 最大步骤/时长/输出：需要配置项并设置**偏大**的默认值（目标是长时间运行与可交付的生产级结果，而非 demo）；同时保留硬上限以避免失控
- 工具权限：默认继承主工具集（包含文件类工具），但必须受 workspace/scope 限制；并支持 allowlist 作为收敛手段

## 并发与文件作用域 (Concurrency & File Scope)
MVP 推荐先实现**串行** subagent（避免并发编辑同一资源带来的锁与长等待）。

如果未来需要并发 subagent，建议依赖一个“计划/分工”模块，在启动并发 subagent 前先划定每个 subagent 的可编辑范围（scope，通常是 workspace 的子目录集合），并在文件工具层强制校验：
- 子 Agent 只能对 scope 内文件做写/改/删；越界则返回可理解错误，促使其重试或调整计划
- 通过“分区”优先规避全局锁；锁仅作为最后手段（例如短时间文件级锁）

## 权衡 (Trade-offs)
- **tool 方式 vs 系统自动编排**：tool 方式更容易融入现有架构；自动编排可更强控但更复杂。
- **保存完整过程 vs 仅保存摘要**：本方案同时做：完整过程写日志（可回溯），主上下文只保留短总结与引用（可控）。
- **技能注入**：提升专注度，但也会增加 prompt 体积，需要 Top-K 与摘要长度控制。

## 开放问题 (Open Questions)
1. findings 文件的固定文件名：`FINDINGS.md` 是否足够，还是需要支持自定义（例如按步骤命名）？
2. 日志文件切分：按 run 单文件（`trace.jsonl`）是否足够？是否需要压缩/轮转策略？
3. 子 Agent 失败重试策略：是系统自动重试（例如补充约束），还是交由主 Agent 决策？
