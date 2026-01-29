# Change: Update task events to a waterfall log view

## Why
当前 Task Workbench 的「事件」视图存在两个体验问题：
- **不清晰**：只展示 `type + message`，`data` 不可见；缺少搜索/过滤/复制等“日志型”操作，用户很难判断系统正在做什么。
- **经常像卡住**：后台轮询刷新时用“加载中”替换列表，导致内容闪烁与滚动位置丢失；用户会误以为系统停止工作。

参考 `controlccx/web/src/App.vue#L4046-4063` 的 logs 面板（Pretty/Raw），本变更将事件做成“瀑布式日志”：把可解释细节展示出来，用低成本的反馈填满用户等待期间的不确定性。

## What Changes
- Frontend（TaskWorkbench / TaskQueuePanel）
  - 「事件」升级为瀑布式日志面板：`Pretty`/`Raw` 切换、搜索、按 attempt/type 过滤、显示过滤数/总数、Copy。
  - 每条事件支持展开详情（包含完整时间戳、`attempt_id`、以及 `data` 的 JSON 展示）。
  - 轮询刷新采用 **stale-while-revalidate**：刷新时不再清空列表，仅显示轻量的“refreshing”指示。
- Backend（for trace/script logs）
  - 扩展 artifacts 读取接口支持 **tail**：`GET /api/tasks/:id/attempts/:attempt_id/artifacts/:kind?tail=1`，用于在运行中查看 `trace.jsonl`（以及 script logs）的最新内容（best-effort）。

## Non-Goals (v1)
- 不引入 SSE/WebSocket。
- 不要求对 trace/log 进行强语义化（先做到可读与不卡）。
- 不改变 task runner 的事件写入内容/频率。

## Impact
- Affected specs: `system-task-queue`
- Affected code (expected):
  - `backend/internal/handler/task_attempt_artifacts.go`
  - `frontend/src/views/TaskWorkbench.vue`
  - `frontend/src/components/TaskQueuePanel.vue`
  - (new) `frontend/src/components/TaskEventLogViewer.vue`（或等价组件）

