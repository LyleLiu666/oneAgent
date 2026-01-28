## ADDED Requirements

### Requirement: Task attempts MUST snapshot effective tool policy
系统必须 (MUST) 在 task attempt 启动时固化一份“effective policy snapshot”，运行期间不得漂移。

#### Scenario: Policy change does not affect running attempt
- **GIVEN** 某 task attempt 已启动并固化 policy snapshot
- **WHEN** 管理员在运行中修改 principal 的 policy
- **THEN** 该 attempt 仍使用启动时的 snapshot
- **AND** 新 policy 仅在新 attempt（resume）中生效

