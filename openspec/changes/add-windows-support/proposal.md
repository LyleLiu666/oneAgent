# Change: Windows support (runtime + tooling)

## Why
我们的商业化目标需要覆盖大量 Windows 用户。但当前 oneAgent 存在明显的 macOS/Unix 偏置：依赖 `bash`、`rg/grep`、以及 macOS 专用的 workspace 选择器（AppleScript/`osascript`），并且后端 `shell` 实现包含 Unix-only 的 `syscall` 逻辑，导致 Windows 无法编译/运行。

## What Changes
- 支持在 Windows 上构建并运行 `oneagent`（`serve/doctor` 至少可用）。
- 工具依赖跨平台化：
  - 命令执行能力在 Windows 默认使用 Git Bash（优先使用 Windows release 内置的 Git Bash）实现 `run_command`/`bash`。
  - 本地搜索能力在 Windows 不得依赖 `grep`；`rg` 缺失时必须提供等价降级实现（例如内置实现或随包携带 `rg.exe`）。
- Windows release 内置 Git 与 Git Bash（portable），用于提供一致的 `git`/`bash` 运行环境，降低用户安装与环境差异成本。
- 不支持 PowerShell/Nushell 等其它命令执行后端；当 Git Bash 不可用时以显式报错 + 指引为准。
- workspace 选择体验跨平台化：
  - Windows 上提供可用的 workspace 选择路径（优先 native folder picker；否则必须允许手动输入并验证）。
- release 产物支持 Windows（zip/校验文件），并在 `doctor` 中输出平台与依赖可用性信息，方便部门内分发与排障。

## Impact
- Affected specs:
  - `local-runtime`（Windows 运行与 release/doctor 约定）
  - `workspace`（workspace 选择/校验的跨平台约定）
  - `skill-recall`（rg 缺失降级策略在 Windows 的等价实现）
  - **New**: `platform-support`（平台支持矩阵与工具可用性约定）
- Affected code (expected):
  - `backend/internal/shell/**`（Windows build tags + Git Bash runner）
  - `backend/internal/handler/workspace.go`（Windows chooser）
  - `backend/internal/tool/rg.go`（Windows 无 grep 降级）
  - `backend/internal/tool/run_command.go`（跨平台命令执行）
  - `scripts/release_local.sh` / `Makefile`（Windows release target）
