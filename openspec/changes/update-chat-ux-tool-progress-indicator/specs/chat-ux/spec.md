## ADDED Requirements
### Requirement: Secretary mode MUST surface in-flight tool progress
When a tool call is running, Chat UI MUST show a compact “working” indicator even in secretary mode, so users can see the LLM is still working.

#### Scenario: Secretary mode shows tool progress while tool call is in-flight
- **GIVEN** 用户处于秘书模式
- **WHEN** 系统正在执行至少一个 tool call
- **THEN** UI 必须显示一个紧凑的“执行中”提示
- **AND** 提示至少包含工具名或工具数量（best-effort）

#### Scenario: Progress indicator shows streaming response token count
- **GIVEN** 系统正在流式返回 assistant 消息
- **WHEN** 前端收到 response token 统计
- **THEN** “执行中”提示应展示 token 计数（best-effort）
