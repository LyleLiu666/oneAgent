## ADDED Requirements

### Requirement: Secretary mode MUST support handing off a message to Task Queue (best-effort)
系统必须 (MUST) 在 Chat 的秘书模式下提供一种低噪声方式，将“长任务”从即时对话中手动交给后台任务队列执行（best-effort），避免用户为了入队而切回管理系统外观。

至少包括（best-effort）：
- 在输入区可发现的 handoff 动作（仅在秘书模式出现）
- handoff 使用当前输入作为 task `prompt` 创建任务（`POST /api/tasks`）
- 成功后清空输入并给出低噪声确认；失败时保留输入并给出可解释错误

#### Scenario: Handoff action is visible in secretary mode
- **GIVEN** 用户处于秘书模式（best-effort）
- **WHEN** 用户在输入框中输入非空内容
- **THEN** 页面展示一个“交给后台/入队任务”的低噪声动作（best-effort）

#### Scenario: Handoff creates a task without sending chat
- **GIVEN** 用户处于秘书模式且输入框非空（best-effort）
- **WHEN** 用户触发 handoff 动作
- **THEN** 系统创建一个后台任务，其 `prompt` 等于输入内容（best-effort）
- **AND** 系统不发起 chat stream（best-effort）

