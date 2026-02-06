## Context
项目已经具备长任务、证据链、恢复能力，但缺少“对外竞争”的量化证明。我们需要把“超过谁、超过多少、是否持续领先”做成可重复流程。

## Goals / Non-Goals
- Goals:
  - 给出稳定可复现的评测结果
  - 支持跨版本趋势对比
  - 指标直接服务产品决策与优先级排序
- Non-Goals:
  - 本 change 不覆盖所有业务场景（先覆盖高频任务）
  - 不依赖第三方竞品必须接入同一执行器（可先做本地自基线）

## Decisions
- Decision: 将 benchmark suite 放在 `project-tooling` 体系下。
- Decision: 使用“任务+验收+证据”三元模型作为最小评测单元。
- Decision: 先支持 nightly + manual，避免增加 PR 时延。

## Risks / Trade-offs
- 风险：任务集设计不当会导致“刷题式优化”。
- 缓解：任务分层 + 定期轮换 + 保留隐藏集（best-effort）。

## Migration Plan
1. 先做 20 条 MVP 与 runner
2. 接入 nightly，收集两周基线
3. 扩展到 100 条并引入阈值告警

## Open Questions
- 是否需要按模型/策略维度切分榜单？
