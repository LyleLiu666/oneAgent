# Change: Update secretary SU fast-path triage (direct answer, no SW sync)

## Why
当前秘书（Secretary）在用户消息进入后，通常会走 SW 规划 + 派发 worker 的链路。对于“可在 <=5 次只读工具调用内闭环”的小请求（例如查询/解释/核对事实），这种链路会带来不必要的：
- 交接开销与延迟（多一次规划/派工/状态同步）
- 上下文噪声（把过程性文案写进 chat message list 影响心智）
- 资源浪费（token/任务队列占用）

我们希望把“最简单、最节省资源的链路”作为默认：**SU 能自己用只读工具解决，就直接回复；只有需要写/改/跑或明显超预算时，才派工。**

## What Changes
- SU 负责 triage 规划与短路径自解：在 `<=5` 步只读工具预算内尝试拿到证据并直接回复（不派工、不写 SW 会话）
- 仅当需要写/改/跑或超出预算时，SU 才输出 tasks 并由系统创建/入队 worker tasks
- Recovery/待处理提示仅在面板展示，不写入 chat message list（避免过程性提示污染上下文）
- Secretary tool loop 注入必要的只读依赖（例如 SettingsDB），确保 `search` 等只读工具可用

## Impact
- Affected specs:
  - `openspec/specs/system-secretary-orchestration/spec.md`
  - `openspec/specs/chat-ux/spec.md`
- Affected code (expected):
  - `backend/internal/secretary/orchestrator.go`（SU triage 规划与直答）
  - `backend/internal/handler/secretary.go`（为 secretary tool loop 注入 settings 依赖）
  - `frontend/src/components/ChatBox.vue`、`frontend/src/components/SecretaryChatBox.vue`、`frontend/src/components/SecretaryTaskDeliverables.vue`（recovery 提示仅面板展示）
- Tests:
  - backend: triage 直答不派工、settings 注入使 search 可用（可用 stub）
  - frontend: recovery 提示不落盘到 chat message list 的回归测试

