## ADDED Requirements

### Requirement: Task artifacts MUST support a test report artifact
系统必须 (MUST) 支持为 task attempt 产出一个测试报告文件，并将其作为 artifacts 指针暴露（例如 `test_report_path`）。

系统应该 (SHOULD) 在检测到可运行的 tests 时 best-effort 生成该报告；当无法生成时应返回可解释原因（例如缺少依赖/未检测到测试框架）。

#### Scenario: test_report_path is present for Go workspace
- **GIVEN** workspace 是 Go 项目且存在可运行的 tests
- **WHEN** task attempt 执行完成并进入终态
- **THEN** artifacts 包含 `test_report_path`

#### Scenario: test_report_path is omitted with an explanation when tests are unavailable
- **GIVEN** workspace 不包含可运行的 tests（或缺少依赖导致无法执行）
- **WHEN** task attempt 执行完成并进入终态
- **THEN** artifacts MAY 不包含 `test_report_path`
- **AND** attempt summary/trace/receipt 中包含可解释原因（best-effort）

### Requirement: Outcome Observer MUST remain read-only for test evidence
系统必须 (MUST) 保持 Observer 的只读属性：Observer 只能读取 `findings/trace/test_report` 等产物进行判定，不得执行任何测试命令。

#### Scenario: Observer reads test report file only
- **GIVEN** attempt artifacts 中包含 `test_report_path`
- **WHEN** Observer 对该 attempt 做 outcome 判定
- **THEN** Observer 只读取该文件内容用于判定
- **AND** 系统不产生任何“执行测试命令”的 tool call/trace 记录
