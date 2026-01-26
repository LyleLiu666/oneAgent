# 任务列表 (Tasks)

- [x] 明确 agentic 启动策略：主 Agent 何时应/不应启用子 Agent（不写死固定流程） <!-- id: 1 -->
- [x] 定义 `subagent` 工具的请求/响应模型：输入 task + 上下文引用；输出短总结 + `findings_path` + `trace_log_path`（避免 JSON 作为交接交付件） <!-- id: 2 -->
- [x] 实现 `internal/subagent`：构建子 Agent prompt、选择工具集合、运行 tool-loop、生成 findings 文件与 trace 日志 <!-- id: 3 -->
- [x] 交付件约束：`FINDINGS.md` 至少包含“流水账/Findings/变更文件”结构；主 Agent 返回仅保留短总结与引用 <!-- id: 4 -->
- [x] 可观测性：trace 记录 `subagent` 条目并关联 parent，同时包含 `findings_path`/`trace_log_path` 指针 <!-- id: 5 -->
- [x] 日志落盘：按日期 + session_id 分类存放子 Agent 完整痕迹（写入 `ONEAGENT_HOME/.oneagent/logs/`，格式 jsonl） <!-- id: 6 -->
- [x] 与技能联动：子 Agent 启动时按步骤任务调用技能召回，并将 Top-K skills 摘要写入子 Agent TurnContext（volatile）（不得回写稳定 system prompt；若不可用则跳过） <!-- id: 7 -->
- [x] 文件作用域：支持对子 Agent 传入可写 scope（glob 规则，基于 `<workspace>/` 的相对路径），并在文件工具层强制校验越界写/改/删 <!-- id: 12 -->
- [x] 限制参数：max_steps/max_runtime/log 大小等提供配置项，并设置“偏大”的默认值（面向生产级长任务） <!-- id: 13 -->
- [x] 单元测试：
  - [x] findings 文件生成与结构校验 <!-- id: 8 -->
  - [x] 日志路径规则（日期 + session_id）与落盘成功 <!-- id: 9 -->
  - [x] 返回给主 Agent 的内容长度控制（只回短总结与引用） <!-- id: 10 -->
- [x] E2E 验证：
  - [x] 4 步任务：第 3 步输入仅含前 1/2 步的短总结/引用，不含细节过程 <!-- id: 11 -->
