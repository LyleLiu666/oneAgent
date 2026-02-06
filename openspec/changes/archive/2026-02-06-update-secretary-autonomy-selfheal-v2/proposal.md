# Change: Raise secretary autonomy with bounded self-heal before user escalation

## Why
秘书模式要超过同类产品，关键不是“会说”，而是“少打断用户”。当前在进度问答歧义、协议异常、工程错误等场景下，仍可能把可自愈问题抛给用户。

## What Changes
- 强化秘书“先自愈后升级”策略：
  - 进度类问题优先基于系统快照直接答复
  - 工程类错误优先自动修复/重试
  - 超预算后再升级给用户，且必须给明确下一步
- 明确“需要用户确认”触发条件，减少不必要追问
- 在 chat-ux 中补齐对应的低噪声提示与恢复回执规范

## Impact
- Affected specs:
  - `system-secretary-orchestration`
  - `chat-ux`
- Affected code:
  - secretary triage planner
  - recovery dispatch loop
  - secretary mode chat hints and receipts
