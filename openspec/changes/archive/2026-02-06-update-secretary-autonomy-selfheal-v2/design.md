## Context
秘书模式是默认入口。若秘书频繁反问或把工程错误直接抛给用户，会快速损失“可委托感”。

## Goals / Non-Goals
- Goals:
  - 减少不必要用户介入
  - 把可自愈错误消化在系统内部
  - 升级给用户时保证具体、可执行
- Non-Goals:
  - 不追求零提问（确需偏好/外部信息时仍需提问）
  - 不扩大秘书写权限边界

## Decisions
- Decision: 进度问答采用“结构化意图 + snapshot 优先”
- Decision: 工程错误采用“repair -> retry -> escalate”三级策略
- Decision: 升级消息强制包含 task/attempt 绑定与下一步动作

## Risks / Trade-offs
- 风险：自愈重试增加 token/时延成本。
- 缓解：设置严格预算与重试上限并可观测。

## Migration Plan
1. 实现意图路径与 snapshot 注入
2. 实现自愈重试框架
3. 对齐秘书模式 UX 回执
