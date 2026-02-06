## 1. Scheduler policy
- [ ] 1.1 增加公平性策略（防止低优先级 workspace 永久饥饿）
- [ ] 1.2 统一 priority 与并发上限决策路径，避免策略冲突
- [ ] 1.3 将“未调度原因”结构化输出（例如资源上限、pause、优先级压制）

## 2. Scheduling semantics
- [ ] 2.1 定义 schedule misfire 策略（skip/catch-up/one-shot）
- [ ] 2.2 增加 schedule 触发幂等 key，避免重复入队
- [ ] 2.3 补齐 schedule 失败重试与可解释告警（best-effort）

## 3. Observability and UX
- [ ] 3.1 在 task/workspace 层面暴露 queue governance 决策快照
- [ ] 3.2 在工作台提供低噪声治理状态入口（不干扰默认路径）
- [ ] 3.3 在事件流中记录调度关键节点（picked/skipped/deferred）

## 4. Validation
- [ ] 4.1 调度测试：多 workspace 并发、公平性、pause/resume、priority
- [ ] 4.2 schedule 测试：misfire、幂等、重复触发
- [ ] 4.3 运行 `openspec validate update-queue-governance-scheduling-v2 --strict --no-interactive`
