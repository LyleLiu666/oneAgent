# Proposal: Secretary 双分身（User/Worker）+ 共享 Memory + 永久在线话（单机）

## Why
当前“秘书模式”仍偏像一个 worker：上下文容易被工具细节/失败排障污染，导致对用户的汇报显得机械、噪声高、且难以长期保持一致性（越聊越乱）。

我们需要把“秘书”变成真正的系统级胶水层：
- **放权**：写/改/跑命令交给 worker；秘书默认只读与编排
- **信任**：秘书像人一样问关键确认、查进度、解释“卡在哪”
- **可追溯**：所有过程沉淀到可查询的流水账与 findings（而不是散落在聊天里）

同时，你已确认以下产品约束：
- 单机优先（无跨机/无加密脱敏诉求）
- 秘书与用户/系统是**永久关系**：只有一个会话，不切换
- memory 是时效性的，需要按时间维度快速查询
- SU/SW 的信息同步采用**触发式 pull-sync**（只同步最后 10 条并告知省略条数）
- 固定 80k 上下文阈值，超出触发压缩

## What Changes
- 引入“秘书双分身”概念：
  - `Secretary(User)`（SU）：面向用户汇报/确认/查进度（只读）
  - `Secretary(Worker)`（SW）：面向 worker 派工/恢复/排障（可调度 Task Queue；尽量不直达用户）
- 引入共享 Memory（append-only）：
  - 记录 worklog + findings + context_summary
  - 支持按时间窗查询（本地 SQLite）
  - 支持 SU/SW 的游标去重与“触发式 pull-sync（last 10 + 省略计数）”
- 引入“永久单 session”的秘书入口：
  - 客户端不需要也不应该创建/切换 secretary session
  - 重启/刷新后仍可继续同一会话（依赖上下文压缩与 memory）
- 固定 80k 的上下文压缩机制应用到 SU/SW 各自通道

## Impact
- Affected specs:
  - `system-secretary-orchestration`
  - `system-session-context-compression`
  - `data-storage`
  - `chat-ux`
- Affected code (expected):
  - Backend: `backend/internal/secretary/*`, `backend/internal/sessionstore/*`, `backend/internal/settingsdb/*`（如复用 sqlite 工具链）
  - Frontend: `/secretary` 路由与秘书聊天入口（保证“永久单会话”心智）

## Non-goals
- 不做跨机/远程 memory 同步与安全出域治理（当前单机）
- 不做加密/脱敏（按你的决定）
- “worker 直通用户”在本设计中优先级最低，只保留折叠入口（未来再做）

