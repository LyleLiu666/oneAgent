## ADDED Requirements

### Requirement: TaskQueue Workbench MUST provide a waterfall-style events/log view
系统必须 (MUST) 在 TaskQueue Workbench 中提供一个“瀑布式”的事件/日志视图，用于降低长任务等待期间的不确定性与用户流失风险。

该视图至少应 (SHOULD) 支持：
- `Pretty` / `Raw` 两种展示模式（默认 `Pretty`）
- 按 `attempt_id` 与 `type` 过滤（best-effort）
- 文本搜索（best-effort）
- 展示 `filtered / total` 计数
- 查看单条事件的完整信息（包含完整时间戳、`attempt_id`、以及 `data`）
- Copy filtered（best-effort）

#### Scenario: User filters and inspects event details
- **GIVEN** 某 task 存在多条 events，且部分 events 包含 `data`
- **WHEN** 用户打开该 task 的事件视图并按 `type` 过滤
- **THEN** 列表仅显示匹配的事件（best-effort）
- **WHEN** 用户展开其中一条事件
- **THEN** UI 展示该事件的完整时间戳、`attempt_id` 与 `data`（best-effort）

#### Scenario: Background refresh does not blank the list
- **GIVEN** 用户已打开事件视图并看到事件列表
- **WHEN** 客户端执行后台轮询刷新
- **THEN** UI 不应 (SHOULD NOT) 用“加载中”清空/替换已有列表
- **AND** UI 以轻量方式提示正在刷新（best-effort）

### Requirement: Task attempt artifacts MUST support tail reading for log-like artifacts (best-effort)
系统必须 (MUST) 支持通过 API 获取 attempt 的 log-like artifacts 的“尾部内容”（tail），用于运行中查看最新日志（best-effort）。

系统必须 (MUST) 至少支持对 `trace_log_path`（`trace` artifact）进行 tail 读取；系统应该 (SHOULD) 同样支持 project scripts 的 log artifacts（例如 `setup_script_log` / `test_script_log`）。

#### Scenario: Client requests tail of trace log
- **GIVEN** 某 attempt 已生成并持续写入 `trace_log_path`
- **WHEN** 客户端请求 `GET /api/tasks/:id/attempts/:attempt_id/artifacts/trace?tail=1`
- **THEN** 返回内容包含 trace 文件的最新部分（best-effort）
- **AND** 当文件过大时，响应标记 `truncated=true`（best-effort）

