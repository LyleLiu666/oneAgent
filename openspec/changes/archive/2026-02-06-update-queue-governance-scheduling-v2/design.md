## Context
任务系统从“能跑”进入“规模化运行”阶段后，瓶颈从功能完整性转向调度治理透明度与可预测性。

## Goals / Non-Goals
- Goals:
  - 防止调度饥饿与不可解释延迟
  - 让 schedule 触发具备幂等与可恢复语义
  - 把治理决策纳入证据链
- Non-Goals:
  - 不引入复杂分布式调度架构
  - 不改变单 workspace FIFO 基本原则

## Decisions
- Decision: 在现有调度器上增加公平性层，而非重写队列系统。
- Decision: schedule 使用显式 misfire policy，避免隐式补偿。
- Decision: 治理信息以“低噪声 + 可展开”呈现。

## Risks / Trade-offs
- 风险：策略维度增多，理解成本上升。
- 缓解：每次调度决策输出结构化原因字段。

## Migration Plan
1. 完成调度器策略层增强
2. 接入 schedule 幂等键
3. 补齐 UI 和事件可观测性
