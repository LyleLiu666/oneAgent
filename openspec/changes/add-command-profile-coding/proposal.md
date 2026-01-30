# Change: Add `coding` command profile for bash/run_command (docker-only)

## Why
当前 `dev` profile 的 allowlist 过窄（缺 `find/git/go/python/node/npm/...`），导致 coding agent 常见链路“能调用但跑不起来”，进入自愈循环，最终体感为工具调用成功率低。

专家建议详见：`docs/oneAgent_toolcall_advice/docs/05_命令沙箱与权限_为什么这会拉低成功率_以及怎么改.md`。

同时需要保持安全边界：仅在 `sandbox_mode=docker` 等强隔离环境下才应该放宽命令能力。

## What Changes
- 新增 `coding` profile：
  - 允许一组常见开发命令（best-effort，按需裁剪）
  - 仅在 policy 指定 `sandbox_mode=docker` 时允许使用（否则拒绝或降级）
- 增加测试，覆盖：
  - `coding` profile 命令 allowlist
  - `sandbox_mode` 约束（docker-only）

## Impact
- Affected specs: `system-tool-permissions`
- Affected code (expected): `backend/internal/permissions/command_profiles.go`, `backend/internal/tool/bash.go`, `backend/internal/tool/run_command.go`（best-effort）
- Reference docs: `docs/oneAgent_toolcall_advice/docs/05_命令沙箱与权限_为什么这会拉低成功率_以及怎么改.md`

