# Change: Add toolcalling metrics & scripted regression

## Why
当前“工具调用不顺滑/会卡住”的体验很难被客观评估：我们缺少统一的失败口径与可重复的回归任务集，导致每次改动只能靠主观感受判断“变好了/变差了”。

专家建议（`docs/oneAgent_toolcall_advice/docs/06_测试与指标_把成功率变成可回归的数字.md`）明确指出需要把工具调用拆成可观测指标，并用脚本化回归把成功率变成可回归的数字。

## What Changes
- 定义并落地一套可回归的工具调用指标（最小集）：
  - tool selection accuracy（best-effort）
  - tool arguments validity rate（JSON 合法率/required 缺失率）
  - tool execution success rate（区分：policy denied / arguments invalid / tool error）
  - steps per task（同类任务平均工具回合数）
- 统一记录口径：无论 tool protocol（JSON 原生 tools / XML `<tool_data>`）都要以一致结构记录 tool failure / tool success（best-effort）。
- 增加“脚本化 agent 回归测试”：固定 prompt + mock provider/tool outputs，输出机器可解析的 metrics report（JSONL/JSON/SQLite 任选其一，先小步可用）。
- 增加工具参数 schema 验证器（至少 required 字段），在执行工具前拦截 arguments invalid 并返回结构化错误（best-effort）。

## Impact
- Affected specs: `project-tooling`
- Affected code (expected): `backend/internal/handler/chat.go`, `backend/internal/toolxml/*`, `backend/internal/tool/*`
- Tests (expected): `backend/internal/handler/*_test.go`, `backend/internal/server/e2e_*_test.go`
- References: `docs/oneAgent_toolcall_advice/docs/06_测试与指标_把成功率变成可回归的数字.md`

