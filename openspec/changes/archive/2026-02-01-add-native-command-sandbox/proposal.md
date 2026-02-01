# Change: Add cross-platform native sandbox for command tools (workspaceRoot, allow true delete)

## Why
当前 `bash/run_command` 在 `sandbox_mode=none` 时会强制降级为只读（fail-closed），导致 agent 无法用控制台命令完成常见的工程操作（例如 `rm -rf` 清理目录、重构时删除文件等）。

我们希望：
- **最高效率**：以 `workspaceRoot` 为边界直接工作（不依赖 git worktree，也不复制 workspace，避免非 git/大文件场景的灾难性成本）。
- **真删除**：允许 `rm` 等删除行为，但必须被强约束在 workspaceRoot 内（进程级强边界）。
- **跨平台**：macOS/Linux/Windows 行为一致（best-effort）。
- **不断网**：sandbox 不默认断网（网络能力由 command allowlist/profile 另行治理）。

## What Changes
- 为命令类工具（`bash`/`run_command`）新增一个硬边界后端：`sandbox_mode=native`。
- `sandbox_mode=native` 在不同 OS 上采用不同实现，但对上层表现一致：
  - macOS: Seatbelt（`/usr/bin/sandbox-exec`）
  - Linux: Landlock（必要时配合最小 seccomp；best-effort）
  - Windows: Restricted Token + ACL（best-effort；可能需要一次性初始化/权限设置）
- 当 policy 指定 `sandbox_mode=native` 时：
  - 系统在该硬边界内执行命令，允许启用写能力的 command_profile（例如 `coding`），从而支持 `rm -rf` 等操作
  - 所有读写删除都必须被约束在 workspaceRoot（或等价的 writable_roots）内

## Impact
- Affected specs:
  - `system-tool-permissions`（扩展 `sandbox_mode` 语义，支持 `native`）
  - `local-runtime`（“不依赖 Docker 的硬边界后端”落地路径；保证无 Docker 也可启用写能力命令）
- Affected code (expected):
  - `backend/internal/tool/{bash.go,run_command.go}`（接受 `sandbox_mode=native`）
  - `backend/internal/shell/*`（新增 native sandbox backend + OS 分发）
  - `backend/internal/permissions/command_profiles.go`（允许 `rm` 等基础删除命令，仅在硬边界模式下生效）
  - `backend/internal/doctor/*`（新增 native sandbox 可用性诊断；best-effort）

## Non-Goals
- 本变更不实现 “execve 级别命令治理/规则/审批” 的完整体系（可在后续 change 做）。
- 本变更不引入 Docker 作为必需依赖（Docker sandbox 仍可保留为可选后端）。

## Open Questions
- Windows 侧是否需要一次性“ACL 初始化/刷新”流程（可能涉及管理员权限提示）？如何做成 best-effort 且可解释？
- Linux 侧 Landlock 在旧内核上的可用性与 fallback 策略：是拒绝执行还是降级只读？
- `sandbox_mode=native` 是否需要额外暴露 `writable_roots`/`network_access` 等参数（类似 Codex），还是先固定为 `workspaceRoot` + network unchanged？

