## Context
oneAgent 当前默认以“本地工具形应用”交付，但实现层存在多处 macOS/Unix-only 依赖：
- workspace chooser 仅支持 macOS（`osascript`）。
- 本地搜索（skills recall/rg 工具）依赖 `rg`，并在缺失时降级到 `grep -R`（Windows 不保证有）。
- 命令执行工具依赖 `bash` 与 Unix `syscall`（进程组/kill 等），Windows 直接无法编译。

这与“部门级分发（大量 Windows 机器）”目标冲突，需要把平台差异收敛到明确的抽象层，并用 build tags 保持单仓库主干一致性。

## Goals / Non-Goals
- Goals
  - Windows 上可构建并运行：`oneagent serve` 能启动 UI+API，`oneagent doctor` 可用于排障。
  - 提供 Windows 可用的命令执行（Git Bash）与本地搜索（无 grep 依赖）。
  - Windows release 默认打包 Git 与 Git Bash（portable），避免要求用户额外安装。
  - 平台差异用 build tags 隔离，不引入长期分叉。
- Non-Goals
  - 一次性把所有 Unix 体验在 Windows 完全复刻（先保证“可用 + 可诊断 + 可扩展”）。
  - 要求用户额外安装 Git/Git Bash/WSL 作为默认路径。
  - 引入 PowerShell 作为默认/主要的命令执行后端（避免体验分裂与行为差异）。

## Decisions
- Decision: 单主干 + build tags
  - 用 `*_windows.go` / `*_unix.go` 隔离 `shell`、workspace chooser、以及可能的 search backend。
- Decision: Windows 命令执行默认使用 Git Bash
  - `run_command`/`bash` 在 Windows 默认使用 `bash.exe -lc <cmd>`（优先使用 Windows release 内置的 Git Bash；系统已安装时可作为 fallback）。
  - 若运行环境缺少可用 `bash.exe`，工具必须返回可操作提示（例如安装 Git for Windows 或使用带 bundled Git Bash 的 release），不得静默失败。
  - 不提供 PowerShell/Nushell 等其它命令执行后端；当 Git Bash 被企业策略禁止/拦截或不可用时，视为不支持的运行环境并显式报错。
- Decision: Windows release 内置 portable Git（含 Git Bash）
  - release zip 内包含一套 portable Git（含 `git.exe` 与 `bash.exe`），oneAgent 在执行相关命令时优先使用该内置路径（或作为系统缺失时的 fallback）。
  - `doctor` 必须输出当前使用的是 “system git/bash” 还是 “bundled git/bash”，便于排障与合规审计。
- Decision: Windows 搜索不依赖 grep
  - `rg` 缺失时的降级策略在 Windows 使用等价实现（内置 walker/逐文件搜索），或随包携带 `rg.exe` 并由 `doctor`/runtime 管理其可用性。
- Decision: workspace chooser 跨平台策略
  - 在 local-tool 场景优先提供 native folder picker；
  - 若不可用，必须保留“手动输入 workspace 路径 + 立即校验/规范化”的工作流（不让用户被卡死）。

## Risks / Trade-offs
- “单二进制”与外部依赖的矛盾
  - 若携带 `rg.exe`，需要明确落盘位置与清理策略；若走纯 Go fallback，性能可能较差但更易分发。
- 打包 Git/Git Bash 的体积与许可证合规
  - Git for Windows / MSYS2 组件较多，需要明确许可证/NOTICE 文件的随包分发策略，并接受产物体积上升。
- Git Bash / MSYS2 语义差异与安全策略
  - 需要复用现有的“危险命令拦截/输出截断/超时”机制，并明确 Windows 下 `bash` 的环境变量/路径转换行为（避免误执行或越界）。

## Open Questions
- `rg.exe` 的交付方式：随 release 附带、还是嵌入到 `oneagent.exe` 并首次运行解压？
- portable Git 的交付方式：随 release 以目录形式附带，还是嵌入到 `oneagent.exe` 并首次运行解压？
