# Change: Preflight workspace requirement when tools are enabled

## Why
很多高频工具（read/write/edit/rg/glob/ls/lsp/run_command 等）都依赖 workspaceRoot 作为作用域与安全边界。当前当 workspace 未设置时，这类工具会在工具回合中失败（例如 `scope.ErrWorkspaceNotSet`），模型只能自愈/反复询问，导致体验“卡住、不清晰”。

专家建议明确指出：当工具启用但 workspace 未设置时，应在进入 LLM 前直接返回可读错误，让用户先设置 workspaceRoot，避免模型白跑一轮工具循环。

参考资料（source-of-truth）：
- `docs/oneAgent_toolcall_advice/docs/05_命令沙箱与权限_为什么这会拉低成功率_以及怎么改.md`（workspaceRoot 前置条件）
- `docs/oneAgent_toolcall_advice/docs/02_JSON工具调用_可靠性提升方案_可落地改动.md`（失败信息要可行动）

## What Changes
- 当请求启用 tools（`tool_ids` 非空，或等价开启工具集）但会话未设置 workspace 时：
  - 系统必须在调用 LLM 之前 fail-fast，返回清晰错误（引导用户设置 workspaceRoot 或选择“仅对话模式”）。
  - 该错误应被 UI 显式呈现（best-effort）。

## Impact
- Affected specs: `workspace`
- Affected code (expected): `backend/internal/handler/chat.go`, frontend workspace picker / error surface
- Tests (expected): `backend/internal/handler/*_test.go`

