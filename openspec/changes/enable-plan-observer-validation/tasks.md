# 任务列表 (Tasks)

- [ ] 定义 `PLAN.md` 的最小格式（任务 id / status / acceptance / scope），并给出示例模板 <!-- id: 1 -->
- [ ] 实现 plan 解析与写回：读取任务列表、更新 checkbox、保持格式稳定 <!-- id: 2 -->
- [ ] 增加 `plan` 工具：`init/get/mark_done`（mark_done 内部触发 observer 校验） <!-- id: 3 -->
- [ ] 实现 observer runner：独立执行、最小上下文、校验交付件并可执行 acceptance.commands（仅允许清单内命令），返回 pass/fail + 原因 <!-- id: 4 -->
- [ ] 与 subagent 集成：
  - [ ] 子 Agent 可携带 task_id 与 scope 运行 <!-- id: 5 -->
  - [ ] 子 Agent 调用 `plan.mark_done` 时，将校验结果自动拼接到 handoff 中 <!-- id: 6 -->
- [ ] 单元测试：
  - [ ] plan 解析/写回稳定性 <!-- id: 7 -->
  - [ ] mark_done 通过/失败的原子性（失败不写回） <!-- id: 8 -->
- [ ] observer 权限：默认只读 + 仅允许执行 acceptance.commands <!-- id: 9 -->
- [ ] E2E：
  - [ ] 创建 PLAN → 执行任务 → observer 校验通过后才能变为 done <!-- id: 10 -->
  - [ ] scope 越界写入被拒绝并能重试 <!-- id: 11 -->
