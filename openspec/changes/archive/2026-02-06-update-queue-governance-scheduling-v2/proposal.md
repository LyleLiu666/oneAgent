# Change: Upgrade queue governance and scheduling for multi-workspace stability

## Why
随着长任务规模增长，单纯“跨 workspace 并行”会出现资源争抢、饥饿和调度不可预测。需要将队列治理从“可配置”升级到“可解释、可观测、可恢复”。

## What Changes
- 强化队列治理策略：并发上限、优先级、公平性（防饥饿）
- 强化 schedule 语义：触发幂等、misfire 策略、补偿行为
- 将治理决策写入证据链（便于复盘为什么某任务没跑/延迟）
- 增加低噪声治理状态可视化（工作台可见但不干扰主路径）

## Impact
- Affected specs:
  - `system-task-queue`
  - `work-ledger-ux`
- Affected code:
  - queue scheduler
  - schedule trigger manager
  - task/workspace governance views
