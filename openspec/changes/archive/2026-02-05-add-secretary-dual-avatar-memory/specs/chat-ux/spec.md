## ADDED Requirements

### Requirement: Secretary UI MUST behave as a permanent single conversation (no session switching) (best-effort)
系统必须 (MUST) 将 `/secretary`（或等价入口）的用户心智固定为“永久单会话”：秘书与用户之间只有一个对话，不创建/不切换会话（best-effort）。

秘书模式下的 UI 必须 (MUST) 满足（best-effort）：
- 默认不展示会话列表/新建会话/切换会话等入口
- 刷新页面后仍回到同一个秘书对话（best-effort）
- 发送消息时不要求用户感知 `session_id`（best-effort）

#### Scenario: Reload keeps the same secretary conversation (best-effort)
- **GIVEN** 用户通过 `/secretary` 进入秘书对话并发送过消息（best-effort）
- **WHEN** 用户刷新页面或重新打开应用（best-effort）
- **THEN** UI 仍呈现同一个秘书对话历史（best-effort）
- **AND** UI 不要求用户选择或创建新的会话（best-effort）

