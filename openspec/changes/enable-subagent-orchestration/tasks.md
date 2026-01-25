# 任务列表 (Tasks)

- [ ] 明确 agentic 启动策略：主 Agent 何时应/不应启用子 Agent（不写死固定流程） <!-- id: 1 -->
- [ ] 定义 `subagent` 工具的请求/响应模型：输入 task + 上下文引用；输出短总结 + `findings_path` + `trace_log_path`（避免 JSON 作为交接交付件） <!-- id: 2 -->
- [ ] 实现 `internal/subagent`：构建子 Agent prompt、选择工具集合、运行 tool-loop、生成 findings 文件与 trace 日志 <!-- id: 3 -->
- [ ] 交付件约束：`FINDINGS.md` 至少包含“流水账/Findings/变更文件”结构；主 Agent 返回仅保留短总结与引用 <!-- id: 4 -->
- [ ] 可观测性：trace 记录 `subagent` 条目并关联 parent，同时包含 `findings_path`/`trace_log_path` 指针 <!-- id: 5 -->
- [ ] 日志落盘：按日期 + session_id 分类存放子 Agent 完整痕迹（写入 `<workspace>/.logs/`，格式 jsonl；避免未来 SQLite 存储压力） <!-- id: 6 -->
- [ ] 与技能联动：子 Agent 启动时按步骤任务调用技能召回并注入 Top-K skills（若可用） <!-- id: 7 -->
- [ ] 单元测试：
  - [ ] findings 文件生成与结构校验 <!-- id: 8 -->
  - [ ] 日志路径规则（日期 + session_id）与落盘成功 <!-- id: 9 -->
  - [ ] 返回给主 Agent 的内容长度控制（只回短总结与引用） <!-- id: 10 -->
- [ ] E2E 验证：
  - [ ] 4 步任务：第 3 步输入仅含前 1/2 步的短总结/引用，不含细节过程 <!-- id: 11 -->
