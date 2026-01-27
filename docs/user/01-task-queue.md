# Task Queue（长任务队列）

核心目标：让 oneAgent 可以**挂机数小时完成复杂交付**，并且失败可恢复、过程可追溯、结果可验收。

## 适合什么任务？

- 大规模重构/替换 + 跑测试/修复直到全绿
- 生成报告/文档/脚本 + 自检（lint/test）+ 交付文件落盘
- 需要多轮工具调用、你不想一直盯着对话的工作

## 核心概念

### 1) Task vs Attempt

- `Task`：任务本体（标题、prompt、workspace、模型、limits…）
- `Attempt`：一次执行尝试（同一个 task 可能多次 attempt：失败后 resume 会创建新 attempt）

### 2) 调度规则（重要）

- **同 workspace：串行 FIFO**（避免一个 repo 里并发改文件互相踩踏）
- **跨 workspace：并行执行**（默认不做硬限制）

### 3) 事件与交付件（Trust Evidence）

- 每个 task 会写入事件流（append-only），用于复盘与“失败断点接续”
- attempt 成功必须产出交付件路径：
  - `findings_path`：交付总结（变更文件/运行结果/下一步）
  - `trace_log_path`：过程 trace（工具调用与关键观察）
- Outcome Observer 只读验收：**只读取 findings/trace**，不跑命令、不改文件

## UI 使用方式

1. 先设置 workspace（推荐启动时 `--workspace .`，或在 Chat 顶部设置）
2. 打开 Chat 顶部 `Tasks` 面板
3. `Create task` → 填写 prompt（可选 title、model）
4. 观察状态与事件；需要时 `Cancel` / `Resume`

常见状态：
- `queued`：排队中
- `running`：执行中
- `succeeded`：已交付（有 findings/trace）
- `failed` / `timed_out` / `interrupted`：可 `Resume`
- `canceled`：已取消

## API（可选）

所有请求需带 `Authorization: Bearer <token>`。

- `POST /api/tasks`
  - body: `{ "workspace": "...", "prompt": "...", "title": "...?", "model_id": "...?", "limits": {...}? }`
- `GET /api/tasks?workspace=...`
- `GET /api/tasks/:id`
- `POST /api/tasks/:id/cancel`
- `POST /api/tasks/:id/resume`
- `GET /api/tasks/:id/events`

## 数据落盘位置

默认在：

`ONEAGENT_HOME/.oneagent/data/tasks/<task_id>/`

目录内至少包含：
- `task.json`：任务与 attempts（状态机）
- `events.jsonl`：事件流（append-only）

