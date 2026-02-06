## Context
渠道中继是生态扩展入口，但也是安全和一致性高风险点。v1 需要最小闭环，优先保证可追溯与幂等，不追求全渠道一次覆盖。

## Goals / Non-Goals
- Goals:
  - 外部消息可安全进入 secretary 编排
  - 任务结果可回推到来源渠道线程
  - 全链路可追溯（source -> task -> deliverable -> notification）
- Non-Goals:
  - v1 不支持多渠道复杂路由策略
  - 不支持渠道侧直接执行高风险本地工具

## Decisions
- Decision: 先单渠道适配，抽象统一 relay schema。
- Decision: 入站以 provider message id 做幂等主键。
- Decision: 出站通知仅发送摘要与指针，不发送敏感原文。

## Risks / Trade-offs
- 风险：渠道消息延迟/重试导致重复处理。
- 缓解：幂等键 + 可重放审计日志。

## Migration Plan
1. 完成 ingress + idempotency
2. 接通 secretary/task 生命周期
3. 上线 outbound 通知与失败重试
