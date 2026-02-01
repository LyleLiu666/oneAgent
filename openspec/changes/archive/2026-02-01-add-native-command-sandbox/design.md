## Context
命令类工具（`bash`/`run_command`）是本地执行面中风险最高的一层：一旦允许写入/删除，就必须有硬边界保证“不越界、不污染系统”。

当前系统在 `sandbox_mode=none` 时 fail-closed 降级只读，安全但体验差（无法 `rm -rf`）。用户希望在不强依赖 Docker 的前提下，获得“像 Codex 一样”的硬边界执行体验。

## Goals / Non-Goals
- Goals:
  - 提供 `sandbox_mode=native`：跨平台、进程级强约束，所有子进程继承限制
  - 允许真删除（`rm`）但删除范围严格限定在 `workspaceRoot`
  - 不依赖 git worktree 或 workspace 全量复制（适配非软件工程/大文件场景）
  - 不默认断网（网络能力由上层命令 allowlist/profile 治理；后续可扩展）
- Non-Goals:
  - 不在本 change 内做完整的 execve 规则引擎/审批体系
  - 不将 Docker 变为必需依赖

## High-Level Design
### Policy surface
- 在 tool permissions policy 的 `constraints.sandbox_mode` 中新增 `native`
- `native` 视为“硬边界后端”，与 `docker` 同级；当 `sandbox_mode=native` 时，不再强制降级为只读 profile

### Boundary definition
- 以 `workspaceRoot` 为默认唯一 writable root（可选：附带 workspace 内的 `.oneagent/tmp` 等目录）
- 通过设置命令执行环境变量，尽量将临时写入引导回 workspace 内：
  - `HOME=<workspace>/.oneagent/sandbox/home`
  - `TMPDIR=<workspace>/.oneagent/sandbox/tmp`（或平台等价变量）

### Platform backends (best-effort)
- macOS: Seatbelt profile 允许 `file-read*`（可选）与对 workspaceRoot 的 `file-write*`（subpath param），network 保持默认允许。
- Linux: Landlock 将 workspaceRoot 加入允许列表（read/write）。旧内核/不可用时 fail-closed 或降级只读（见 Open Questions）。
- Windows: Restricted Token（低权限）+ 为 workspaceRoot 增加 capability SID 的 ACL allow（并确保其他路径无写权限）。可能需要一次性 setup/refresh（可解释）。

## Risks / Trade-offs
- Windows 实现复杂度与兼容性最高；可能需要额外的 setup 程序或一次性 ACL 初始化。
- “不断网”提升了数据外泄风险；需要与 tool permissions / command allowlist 配合治理（后续可加审批/规则）。

## Migration Plan
- 新增 `sandbox_mode=native` 不影响现有用户：默认仍是 `none`（只读）或 `docker`（若启用）。
- 当 policy 显式要求 `native` 时才启用，并在不可用时返回可操作错误（不得 silent fallback 为宿主写执行）。

## Open Questions
- Linux/Windows 不可用时的 fallback：严格拒绝 vs 降级只读（哪个更符合用户预期/可操作性）？
- 是否需要把 `writable_roots`（多 root）做成 policy 的一部分（类似 Codex），还是先锁定为 workspaceRoot？

