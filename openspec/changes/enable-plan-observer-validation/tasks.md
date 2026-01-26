# 任务列表 (Tasks)

- [x] 定义 `PLAN.md` 的最小格式（任务 id / status / acceptance / scope=glob），并给出示例模板 <!-- id: 1 -->
- [x] 实现 plan 解析与写回：读取任务列表、更新 checkbox、保持格式稳定 <!-- id: 2 -->
- [x] 增加 `plan` 工具：`init/get/mark_done`（mark_done 内部触发 observer 校验） <!-- id: 3 -->
- [x] 实现 observer runner：独立执行、最小上下文、仅基于文件内容/结构校验交付件（不执行命令验收），返回 pass/fail + 原因 <!-- id: 4 -->
- [x] 与 subagent 集成（本变更仅提供基础能力，handoff 拼接在 `enable-subagent-orchestration` 完成）：
  - [x] 子 Agent 可携带 task_id 与 scope 运行（plan.get 返回 scope；文件工具通过 workspace writeScope 强制 scope） <!-- id: 5 -->
- [x] 单元测试：
  - [x] plan 解析/写回稳定性 <!-- id: 7 -->
  - [x] mark_done 通过/失败的原子性（失败不写回） <!-- id: 8 -->
- [x] observer 权限：默认只读（读文件/搜索/列目录），不允许执行命令 <!-- id: 9 -->
- [x] E2E：
  - [x] 创建 PLAN → 执行任务 → observer 校验通过后才能变为 done <!-- id: 10 -->
  - [x] scope 越界写入被拒绝并能重试 <!-- id: 11 -->
