# 启用子 Agent 编排 (Enable Sub-Agent Orchestration)

## 摘要 (Summary)
为 oneAgent 增加一套“子 Agent（sub-agent）”能力，用于把复杂工作切分为多个相对独立的步骤，并把每个步骤交给一个隔离上下文的子 Agent 执行，从而显著降低单次推理的上下文负担与“思维污染”。

子 Agent 的 system prompt **默认复用主 Agent** 的提示词，但允许在每个步骤按任务性质注入更聚焦的“skill”（与 `enable-skills-usage` 提案形成联动），实现类似人类在多任务间“清空大脑→切换工作模式”的效果。

## 动机 (Motivation)
1. **上下文切割**：长链路任务（例如 4 步）在第 3 步时往往不需要第 1/2 步的全部细节，只需要可追溯的“流水账 + 关键结论/约束”即可继续推进。
2. **认知负担与漂移**：上下文越长越容易引入干扰信息，导致方案漂移、重复劳动、忽略新约束。
3. **更像人类的工作方式**：人类完成阶段性任务会在阶段间切换心智模型（后端→前端→文档），并保留关键节点记录而非全量记忆。
4. **与技能模块联动**：每个步骤可以自动挑选相关 skill 并注入子 Agent system prompt，提高专注度与执行质量，同时避免把大量技能塞进主提示词。

## 建议方案 (Proposed Solution)
### 1) 子 Agent 启动应当是 agentic（由主 Agent 判断）
系统提供子 Agent 能力，但**不强制**任何任务必须使用子 Agent。主 Agent（LLM）需要先判断：
- 该工作是否可以切成“边界清晰、相对独立”的颗粒（例如一个明确步骤/模块）
- 是否存在强耦合、需要频繁回看大量历史上下文的情况（若是，则不应使用子 Agent）

因此本方案采用“提供工具能力 + 主 Agent 自主选择”的形态（例如 `subagent` tool / 内部组件），而不是把多步编排写死为固定流程。

### 2) 子 Agent 的交付件：Findings 文件（而不是 JSON 文本）
子 Agent 的**主要交付件**应当是一个 `FINDINGS.md`（或等价文件），用于记录：
- 子 Agent 做了哪些事（关键动作/事件）
- 改了哪些文件 / 新增了哪些文件（可追溯）
- 关键结论、约束、下一步建议（给主 Agent 继续推进）

主 Agent 从子 Agent 获得的汇报应当“像人类做完阶段工作后脑子里留下的结论”：
- 一段很短的总结 + 下一步建议
- 再附上 findings 文件的路径引用（需要时再去读），而不是把全部细节塞回主上下文

如果确实需要结构化元信息（例如 run_id、findings_path、trace_log_path），优先使用 XML（默认把节点内容视作 CDATA，避免强校验与转义失败）。

### 3) 与技能召回联动（可选，但建议）
当 `enable-skills-usage` 落地后，子 Agent 在启动前可以：
- 基于“本步骤任务描述”调用技能召回工具获取 Top-K skills
- 仅把 Top-K skills 的中文摘要注入到子 Agent 的 system prompt（而非注入主 Agent 或全量技能）

### 4) 资源限制与安全边界
为避免递归膨胀与失控，子 Agent 需要具备默认限制：
- 默认禁止子 Agent 再次启动子 Agent（或限制最大深度）
- 每次子 Agent 的工具调用步数、运行时长、输出大小均有上限
- 子 Agent 的工具集合可配置为 allowlist（例如默认同主工具集，但可按步骤收敛）

### 5) 可观测性
子 Agent 执行应被记录为可观测事件：
- **必须留下完整痕迹**以便回溯：包括子 Agent 的消息、工具调用、工具结果、关键中间产物等
- 考虑到未来从 Postgres 迁移到 SQLite 可能带来的存储压力，建议将“完整痕迹”落在**日志文件**中，而不是全部塞进数据库
  - 按日期与 session_id 分类存放（类似日志按日期切分）
  - 数据库中的 trace 只存“摘要 + 指针”（例如 log 文件路径、run_id）

示例目录（仅示意，可配置）：
- `ONEAGENT_HOME/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/trace.jsonl`
- `ONEAGENT_HOME/logs/subagent/YYYY-MM-DD/<session_id>/<run_id>/FINDINGS.md`

## 影响范围 (Impact)
- 后端：新增子 Agent 运行组件与工具；需要接入现有 tool-loop、trace 与（可选）会话/消息持久化
- 前端：可选增强（展示子 Agent 执行状态、查看交接摘要/日志入口）
- 与其它变更的关系：与 `enable-skills-usage` 存在强联动（技能注入可作为可选能力先行）

## 成功标准 (Success Criteria)
- 复杂任务可拆分为多个步骤；是否使用子 Agent 由主 Agent **agentic 决策**，不会被系统强制。
- 当主 Agent 选择使用子 Agent 时：每步通过子 Agent 执行，主 Agent 仅保留“短总结 + findings 文件引用”继续推进下一步。
- 连续执行到第 N 步时，主上下文不再携带前 N-1 步的细节过程（仅摘要），上下文长度随步骤数增长保持可控。
- 子 Agent 可按步骤注入不同 skill（例如后端/前端/文档），并在结果中体现更聚焦的执行风格与质量。
- 子 Agent 的完整执行痕迹可被回溯（日志文件 + trace 指针）。

## 开放问题 (Open Questions)
1. 默认工具权限：按当前方向，默认与主 Agent 一致（包含文件类工具），但应受 workspace/scope 限制；是否仍需要按任务类型提供可选的最小权限模板？
2. 并发：是否需要支持并行子 Agent（例如并行调研/对比），还是先做串行（MVP）？
