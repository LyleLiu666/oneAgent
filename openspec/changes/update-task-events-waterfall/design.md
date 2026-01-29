# Design: Waterfall task events/log view

## Goals
- 让用户“看到正在发生什么”：事件需要可读、可搜索、可过滤、可复制。
- “刷新不打断阅读”：后台轮询更新时不应清空列表或丢失滚动位置。
- 支持运行中查看 `trace.jsonl` 的最新内容（best-effort），用于满足用户对过程细节的好奇心。

## Data Sources
1) Task events（已有）：`GET /api/tasks/:id/events` → `TaskEvent[]`
2) Attempt trace/script logs（新增 tail 支持，best-effort）：
   - `GET /api/tasks/:id/attempts/:attempt_id/artifacts/trace?tail=1`
   - `GET /api/tasks/:id/attempts/:attempt_id/artifacts/{copy_files_log|setup_script_log|test_script_log|cleanup_script_log}?tail=1`

> v1 默认只要求“事件瀑布”可用；trace/script logs 可以作为同一面板的可选来源（例如通过 source 切换或过滤实现）。

## UI/UX (TaskWorkbench)
- 事件面板顶部提供：
  - `Pretty`/`Raw` 切换（默认 `Pretty`）
  - Filter：`attempt_id`（latest / all / 选择某一个）、`type`（多选或 quick filters）
  - Search：在 `type/message/data` 中模糊匹配（best-effort）
  - Meta：`filtered / total`、`refreshing` 指示
  - Actions：Copy filtered
- 列表项：
  - 左侧：时间（本地时间；hover 显示完整 RFC3339）
  - 中间：tag（`type` 或 source）
  - 右侧：summary（`message` 或从 `data` 推导的短摘要）
  - 可展开：展示 `attempt_id` + `data` 的 JSON pretty print（`<details>/<summary>`）

## Refresh Policy
- 初次加载（selectedTask 切换）允许显示 loading。
- 后台轮询更新（timer tick）采用 stale-while-revalidate：
  - 保留旧数据继续展示
  - 仅显示轻量 “refreshing” 状态
  - 成功后替换/合并数据（按时间排序；默认 newest-first）

## Backend Tail Semantics
- `tail=1` 时：
  - 返回文件末尾的至多 `maxArtifactBytes` 内容（与现有上限一致）
  - `truncated=true` 表示“只返回了部分内容”（文件超过上限）
  - 保持原响应结构 `{ path, content, truncated }`，以确保前端低改动接入

## Acceptance Checklist
- **Waterfall**：事件列表支持 `Pretty/Raw`、搜索、过滤、计数、Copy，且单条可展开查看 `attempt_id + data`。
- **No flicker**：task 仍在 running 时，后台轮询刷新不会清空事件列表或打断滚动位置（仅显示 refreshing 提示）。
- **Tail (best-effort)**：运行中请求 `.../artifacts/trace?tail=1` 能拿到最新尾部内容；大文件返回 `truncated=true`。

## Risks / Mitigations
- 大日志渲染卡顿：限制展示条数（例如仅渲染最近 N 条/最近 M KB），并提供“Load more”或“仅显示过滤后”的可控入口（后续）。
- JSON 展示可读性：v1 先用 `JSON.stringify(data, null, 2)`；语法高亮留待后续。
