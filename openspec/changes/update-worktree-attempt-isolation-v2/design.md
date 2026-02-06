## Context
worktree 隔离是“可回滚执行”的地基能力。现阶段需要从“可用”提升到“鲁棒可审计”，特别是跨平台清理和异常恢复。

## Goals / Non-Goals
- Goals:
  - 保证每个 mutating attempt 都有稳定执行根和可回退边界
  - 清理失败可追溯且可恢复
  - non-git 行为不再含糊
- Non-Goals:
  - 本 change 不引入新的业务 UI
  - 不改变 task queue 的宏观调度策略

## Decisions
- Decision: 以 attempt artifact 记录 worktree 关键元信息，作为审计主锚点。
- Decision: 清理策略采用“默认清理 + 可配置保留”双态。
- Decision: non-git 模式不 silent fallback，必须返回明确语义。

## Risks / Trade-offs
- 风险：清理与回收逻辑复杂度上升。
- 缓解：生命周期事件标准化 + 针对失败路径的测试优先级提升。

## Migration Plan
1. 增加元信息字段与落盘
2. 接入清理器与重试机制
3. 补充跨平台回归测试
