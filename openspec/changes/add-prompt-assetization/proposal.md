# Change: Prompt assetization (modular prompt assembly)

## Why
目前提示词经验分散在代码与文档中（例如 `docs/tool-call-failures.md`、tool xml prompt、chat/task/subagent 的不同注入），容易出现：
- 同类约束在不同入口不一致（行为漂移）
- 经验无法被“测试/治理”（改了 prompt 不知道是否破坏关键约束）

将提示词组件化为可复用资产，并建立最小测试，可以把“经验”变成系统能力，提升长期稳定性。

## What Changes
- 引入 prompt modules（文件化）：
  - base persona（TDD/反思/提问机制）
  - tool manuals（bash/edit/write_file/...）
  - provider/model profiles（针对不同 provider 的差异化约束）
- 统一 prompt assembly：chat/task/subagent 使用同一套组装逻辑
- 添加 prompt unit tests：确保关键约束存在（例如不输出 CDATA、不用 heredoc 写文件）

## Impact
- Affected specs: `system-prompt-assembly` (new), `system-llm-prompt-caching` (integration)
- Affected code: `backend/internal/prompt/*` (new), `backend/internal/handler/*`, `backend/internal/subagent/*`

