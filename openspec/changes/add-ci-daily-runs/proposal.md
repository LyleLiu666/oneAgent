# Change: CI coverage + daily scheduled runs

## Why
当前 oneAgent 的“可交付”能力已经具备（task queue / ledger / SOP / Windows support 等），但工程层面仍处于“本机绿”的阶段：
- CI 仅跑 backend tests + windows cross-build
- frontend unit tests、e2e smoke 未纳入 CI
- 缺少每日自动跑，导致回归只能靠人工发现

这会直接影响“部门级分发/推广”的可信度（稳定性、可诊断性、可复现性）。

## What Changes
- 扩展 GitHub Actions CI：
  - 增加 frontend test job：`npm test -- --run`
  - 增加 e2e smoke job：`scripts/e2e_smoke_test.sh`
  - 增加 `schedule`（每日自动跑）与 `workflow_dispatch`（手动触发）
- 明确 CI 的关键约束：
  - Vitest 禁止 watch（必须用 `--run`）
  - e2e 依赖 `python3` 与 `curl`（ubuntu-latest 默认具备）

## Impact
- Affected specs: none (tooling-only)
- Affected code: `.github/workflows/ci.yml`

