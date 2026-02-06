## Context
MCP 是外部生态的关键接口层。仅只读会限制自动化场景，直接放开写入又会引入安全风险。需要“可执行但可控”。

## Goals / Non-Goals
- Goals:
  - 支持最小动作集（create/resume/cancel）
  - 保持与现有权限/审批一致
  - 实现端到端可审计
- Non-Goals:
  - 不在 v1 引入全量 task/workflow 写操作
  - 不绕开已有 HTTP 层语义

## Decisions
- Decision: action plane 先覆盖 task 生命周期高频动作。
- Decision: 通过 policy + approval 双重门控实现 fail-closed。
- Decision: 审计记录采用统一调用指纹，支持回放定位。

## Risks / Trade-offs
- 风险：外部客户端误用导致动作风暴。
- 缓解：配额与速率限制后续迭代补齐（v1 先做审计和权限兜底）。

## Migration Plan
1. 上线只读 + 动作并行路径
2. 逐步放开动作权限给受控 principal
3. 观察审计数据后再扩动作范围
