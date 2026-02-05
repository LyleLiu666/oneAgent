## ADDED Requirements
### Requirement: Secretary mode MUST surface pending triage questions as a low-noise, replayable UI affordance (best-effort)
当秘书模式下 triage 产生 `questions[]` 时，Chat UI 必须 (MUST) 以低噪声方式让用户发现并查看这些待确认问题（best-effort），避免对话中只留下模糊提示导致用户无法继续。

至少包括（best-effort）：
- 一个可发现的“待确认”提示入口（例如 badge）
- 打开后可查看 `questions[]` 的具体内容（例如弹窗/侧边面板）
- 刷新/重进会话后仍能恢复并展示同一批待确认问题（replayable，best-effort）

#### Scenario: Pending triage questions are discoverable in secretary mode (best-effort)
- **GIVEN** triage 响应包含 `questions[]` 且非空（best-effort）
- **WHEN** 用户处于秘书模式（best-effort）
- **THEN** UI 展示一个低噪声“待确认”入口（best-effort）
- **AND** 用户可以打开并看到每条问题内容（best-effort）

#### Scenario: Pending triage questions are replayable after refresh (best-effort)
- **GIVEN** 某 session 存在未解决的 `questions[]`（best-effort）
- **WHEN** 用户刷新页面或重新进入秘书模式（best-effort）
- **THEN** UI 通过恢复 secretary state 再次显示这些待确认问题（best-effort）
