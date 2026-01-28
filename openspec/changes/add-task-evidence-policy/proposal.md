# Change: Strengthen evidence policy for task deliveries

## Why
oneAgent 的定位是“可交付的类人 agent”：用户不应只得到一句总结，而应得到可复核证据（diff/测试报告/产物路径）。

当前 receipts 已包含 findings/trace 等证据指针，但“测试/验收类证据”仍主要依赖任务作者的自觉。需要把证据链进一步制度化，降低风险并提升用户信任。

## What Changes
- 为 task attempts 增加可选 `test_report_path`（或等价）产物字段，并在适配场景下 best-effort 生成
- Outcome Observer 继续保持只读：不执行测试命令，只读取 test report 文件做判定
- UI/receipt 明确展示证据链入口（findings/trace/test report）

## Impact
- Affected specs: `system-task-queue`, `system-work-ledger`
- Affected code: `backend/internal/taskqueue/*`, `backend/internal/workledger/*`, `backend/internal/handler/*`, `frontend/src/views/*`

