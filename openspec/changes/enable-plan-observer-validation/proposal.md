# 启用计划模块与观察者校验 (Enable Plan + Observer Validation)

## 摘要 (Summary)
为 oneAgent 引入一个可复用的“计划（plan）模块”，以及一个独立的“观察者（observer）”校验机制：
- 主 Agent / 子 Agent 可以生成与维护一个计划文件（面向复杂任务的可追溯 SOP）
- 当某个任务被标记为 done 时，系统自动调用观察者对交付件进行校验（TDD 思想），未达标则拒绝推进并要求重试

该能力不仅用于 subagent 并发分工（划定各自可编辑范围 scope），也适用于任何长任务的阶段性验收与防止“半途而废”。

## 动机 (Motivation)
1. **Agentic 可能松散**：仅依赖 agentic 容易出现推进不稳、遗漏验收、半途停止。
2. **需要可审计的工作流**：复杂任务需要计划文件承载分解、约束、验收标准与进度，便于回溯。
3. **并发编辑冲突**：若未来允许并发 subagent，需要先分区（scope）再执行，否则文件锁等待可能很长且体验差。
4. **TDD 的自动化落地**：把“标记 done”变成“先校验再 done”，避免自我宣称完成但实际未达标。

## 建议方案 (Proposed Solution)
### 1) 计划文件（Plan File）
默认在 workspace 下生成项目私有计划文件：
- 路径建议：`<workspace>/.oneagent/PLAN.md`（不进入 git）
- 计划内容包含：任务列表、每个任务的验收标准（acceptance criteria）、可选的编辑范围（scope）

### 2) Plan 工具（Plan Tool）
新增一个工具/模块（CLI 或内部 tool）用于：
- 创建/读取计划
- 更新任务状态
- 标记任务 done（触发观察者校验）

### 3) 观察者（Observer）
当任务被标记 done 时：
- 系统以“独立角色”运行观察者（独立于主 Agent/子 Agent 上下文）
- 观察者只接收：任务描述 + 验收标准 + workspace 根目录（以及可选的 scope/交付件路径）
- 观察者只校验交付件（主要是文件内容/结构，必要时可运行验收命令），不与主/子 Agent 交互
- 若未达标，返回 `【plan中某个任务标记done失败】` + 原因，任务保持未完成

### 4) 与 subagent 的联动（为并发做准备）
- 计划任务可声明 `scope`（workspace 子目录集合）
- subagent 启动时由系统传入 scope，并在文件工具层强制越界写/改/删失败（促使 subagent 重试/调整计划）
- 如果 subagent 标记任务 done，系统将“校验结果”自动拼接到 subagent 的 handoff 汇报中（不依赖提示词约束）

## 影响范围 (Impact)
- 后端：新增计划解析/写回模块、observer runner、plan tool；与 subagent runner 集成
- 前端（可选）：展示计划文件/任务状态、提供“新建计划/标记 done”入口
- 与其它变更关系：依赖 workspace 概念；可作为 subagent 并发能力的前置基础

## 开放问题 (Open Questions)
1. 计划文件格式：纯 Markdown 还是 Markdown + YAML frontmatter？任务/验收标准如何结构化且便于解析？
2. 观察者工具权限：默认只读文件是否足够？是否允许按验收标准执行有限命令（例如 `go test`）？
3. scope 表达：是目录前缀集合，还是 glob 规则，还是两者都支持？
4. 并发策略：若两个任务 scope 重叠，是拒绝并发、还是引入文件级锁（以及锁等待上限）？

