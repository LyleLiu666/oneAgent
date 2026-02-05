# Design: Secretary-guided recovery as a conversation

## UX North Star (re-stated)
“用好 agent 的方式是放权和信任，以及可追溯过程。”

对应到 recovery：
- **尽量不限制**：不强行限制用户怎么排障、怎么修复；允许多样化路径（重试/换模型/分步/手动修改）。
- **必须可追溯**：用户为什么要介入、介入了什么（回复/决策），如何导致下一次 attempt 的继续，全部留痕。
- **默认低噪声**：用户不应该被迫去 /tasks 才能知道为什么失败；默认就在聊天里看到“发生了什么 + 怎么继续”。
- **工程错误先自愈**：协议解析失败/工具参数不合法等工程性错误，优先回到 agent loop 里自愈重试；不要把“工具/解析失败”当成用户需要拍板的问题。

## Problem framing
现在“需要处理”卡片虽然能看到 `attempt.summary/error/observer.next_steps`，但存在几个断点：
1) 信息在卡片里，不在对话里：用户心理模型是“秘书会在对话里告诉我发生了什么”。
2) 用户不知道“我该回复什么才能继续”：回复与 resume 之间缺少桥梁。
3) 排障入口太重：跳转 /tasks 会打断“只聊天”的低噪声体验。

## Proposed interaction model

### 1) Failure → secretary relay message (low-noise)
当某 task 的 latest attempt 从 `queued/running` 跃迁到“需要处理终态”时（不回放历史）：
- UI 在聊天中追加一条 assistant 消息（“秘书转达”），内容由确定性模板构造（不依赖 LLM；避免 LLM 二次失败/幻觉）：
  - `任务：<title>` + `状态：<status> · attempt=<short>`
  - `原因：`优先使用 `attempt.summary`；若为空则使用 `attempt.observer.reason`；再不行用用户安全化的 `attempt.error`（best-effort）
  - `下一步：`优先 `attempt.observer.next_steps`（截断到合理长度）
  - `需要你确认：`若 `attempt.observer.questions_for_user[]` 非空，则逐条列出
- `证据：`默认不展开（避免管理系统感）；在“更多/展开”里提供 findings/diff/trace 的入口（引用路径指针，best-effort）
- 该 assistant 消息携带可机读的 metadata（例如 `task_id/attempt_id`），以便将用户下一条回复绑定到该任务。

### 2) User reply → resume with review_notes
用户对该“秘书转达”消息的回复，视为 recovery input：
- 系统将用户回复作为 `review_notes` 调用 `POST /api/tasks/:id/resume`（best-effort）
- 在聊天里追加一条“回执”assistant 消息：`已继续推进：task=<id> attempt=<new>（已记录你的补充）`
- 多任务时，系统维护一个“待处理事项队列”，并确保用户回复绑定到当前聚焦事项（best-effort）。

### 3) Multi-task strategy
如果同一时间有多个“需要处理”的任务：
- 秘书采用人类助理汇报方式：**不先报数**，而是直接给出清单与当前聚焦项（低噪声，best-effort）：
  - 每个事项都包含“原因/下一步/需要你确认什么（若有）”与证据入口（best-effort）
  - 默认聚焦到一个事项（例如最新/最相关），并清晰说明“你只要回复这个就能继续”（best-effort）
  - 允许用户在对话/卡片上切换要处理的事项（best-effort）
- Recovery 卡片仅作为“直通入口”并默认折叠：用户仍可手动查看 artifacts/事件，但不作为默认路径。

### 4) Troubleshoot strategy (progressive disclosure)
“排障”在秘书模式下优先轻量：
- 若存在 `trace`：在当前页面直接打开 trace 弹窗（默认显示末尾 tail；best-effort）
- 否则才切换到 full mode 并跳转 `/tasks`

### 5) “worker 只和秘书交谈”与“操作同步给秘书”
- Worker 的产出（summary/observer reason/next_steps/questions + artifacts 指针）默认只进入 secretary 视图/状态，由秘书在聊天中转达。
- 任何“直通用户”的高级入口（例如 artifacts 预览、跳转工作台）必须默认折叠（progressive disclosure）。
- 用户通过直通入口触发的关键动作（例如 resume/rollback）必须生成一条可追溯的 chat receipt（由秘书发出），确保“秘书知道用户做了什么”。

## Safety / compliance (record over restrict)
- recovery 阶段允许秘书使用只读/排障类工具获取证据指针与定位信息（best-effort），但必须：
  - 禁止对用户目录增删改（由 policy fail-closed 强制执行，best-effort）
  - 工具失败/解析失败优先回到 agent loop 自愈重试，避免把工程错误抛给用户（best-effort）
- 所有对外展示遵循 `system-error-surface`：默认呈现用户安全文案，技术细节需显式展开（request_id/trace）。
- 用户回复会进入 task attempt 的 events/receipt 证据链（review_notes 记录来源与时间）。

## Decisions captured
1) 多任务汇报方式：人怎么汇报，秘书就怎么汇报——**不先报数**，而是直接给出具体事项与当前聚焦项（可切换）。
2) 默认不显式展示 artifacts/排障细节：交给秘书处理；直通入口至少折叠掉（progressive disclosure）。
3) 原则上 worker 只和秘书交谈：尽量不绕过秘书直达用户；如有直通操作，需折叠，并把用户操作同步给秘书（可追溯）。
4) 工程错误先自愈：tool/observer 的协议失败优先在 agent loop 内重试修复；只有超过预算才升级为“需要处理”并向用户解释（best-effort）。
