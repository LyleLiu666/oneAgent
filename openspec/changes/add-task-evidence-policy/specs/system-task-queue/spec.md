## ADDED Requirements

### Requirement: Task artifacts MUST support a test report artifact
系统必须 (MUST) 支持为 task attempt 产出一个测试报告文件，并将其作为 artifacts 指针暴露（例如 `test_report_path`）。

系统应该 (SHOULD) 在检测到可运行的 tests 时 best-effort 生成该报告；当无法生成时应返回可解释原因（例如缺少依赖/未检测到测试框架）。

#### Scenario: test_report_path is present for Go workspace
- **GIVEN** workspace 是 Go 项目且存在可运行的 tests
- **WHEN** task attempt 执行完成并进入终态
- **THEN** artifacts 包含 `test_report_path`
