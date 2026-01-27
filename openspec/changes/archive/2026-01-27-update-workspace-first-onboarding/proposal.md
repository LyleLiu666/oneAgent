# Change: Workspace-first onboarding + quick-start serve flags

## Why
当前 UX 的关键阻塞是：用户进入 UI 后不知道/不愿意先配置 workspace，导致文件/命令工具无法“进来就开干”，整体体验不像 Cherry Studio 这种“打开就能干活”的客户端。

## What Changes
- 增加 workspace-first onboarding：首次进入或新建会话时，优先引导选择一个文件夹作为 workspace（仍保留“跳过 workspace 仅对话”）。
- 在 Chat 页将 workspace / model 的关键状态做成更明确、可发现、低打扰的控件（减少“要先去 Settings 才能用”的成本）。
- `oneagent serve` 增加 `--open` 与 `--workspace` 两个 quick-start flags：
  - `--open`：启动后自动打开默认浏览器到 UI
  - `--workspace`：设置“默认 workspace”，供 UI 首次进入自动填充（可被用户在会话级覆盖）

## Impact
- Affected specs:
  - `workspace`（新增 workspace-first onboarding 行为）
  - `local-runtime`（新增 quick-start serve flags）
  - `llm-provider-management`（补齐 provider/model 管理与会话级模型选择的规格）
- Affected code (expected):
  - Frontend: `frontend/src/components/ChatBox.vue`, `frontend/src/components/Welcome.vue`, `frontend/src/api/client.ts`
  - Backend: `backend/cmd/oneagent/main.go`, server config + handlers（新增默认 workspace / open 支持）

