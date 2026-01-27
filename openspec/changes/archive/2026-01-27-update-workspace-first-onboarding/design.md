## Context
目标是把 oneAgent 的“本地工具形应用”体验往 Cherry Studio 的交互靠拢：用户打开就能开始工作（选 workspace → 选模型 → 开始对话/执行工具），而不是先理解一堆概念、到处找设置。

现状：
- UI 已有 workspace 输入框 + Browse（`/api/workspace/choose`），但不够显眼且缺少“必须先选 workspace 才能用工具”的引导。
- 系统已有 LLM provider/model 的后端能力与 Settings 页面，但与核心聊天流的衔接不够顺滑。
- `oneagent serve` 目前没有 “启动后自动打开 UI” 以及 “默认 workspace” 的 quick-start 能力。

## Goals / Non-Goals
- Goals
  - 首次进入即可通过引导完成 workspace 选择，进入可用状态。
  - 支持会话级模型选择，且与 provider 管理形成闭环。
  - 提供 quick-start CLI：`oneagent serve --open --workspace <dir>`。
- Non-Goals（本次不做）
  - “卖 skills/provider”的计费、授权、商店、分发中心等（作为后续 change）。
  - 桌面壳（Tauri/Electron）落地（先把 Web UX 打磨到位；桌面壳属于包装层）。

## Decisions
- Decision: 保留“本地 Go 服务 + Web UI”架构
  - 原因：与现有 local-runtime 方向一致；安装与发布保持单二进制；后续再加桌面壳不会推翻现有内核。
- Decision: 引入“默认 workspace”概念（运行时级别），但允许会话级覆盖
  - UI 逻辑：若本地未保存 workspace、会话未设置 workspace、且服务端提供 default workspace，则自动填充并提示用户确认/可修改。
- Decision: onboarding 只做 1 层轻量引导（modal/empty-state），避免把聊天页变成设置页
  - workspace / model / tools：关键入口保留在 header；更重的 provider/model 管理仍在 Settings。

## Risks / Trade-offs
- workspace-first 可能与 “workspace 可选” 产生冲突
  - 处理：保留 “Skip（仅对话）” 选项，并在 spec 中明确兼容。
- `--open` 在不同 OS 上行为不一致
  - 处理：实现为 best-effort；失败时只打印提示，不影响服务启动。
- 默认 workspace 可能带来“误操作修改文件”的风险
  - 处理：默认仅填充不自动启用 destructive 工具；首次设置时提示“工具默认作用域 = workspace”。

## Migration Plan
1) 后端：增加 serve flags 与 `/api/config`（或等价）返回 default workspace / server base url 信息。
2) 前端：在 empty-state（Welcome）增加 workspace-first 引导；在 Chat header 强化 workspace 状态与切换。
3) 增量测试：后端 e2e 覆盖新 flags；前端 unit test 覆盖 onboarding 状态机。

## Open Questions
- `--open` 的目标 URL：当 bind=0.0.0.0 时，是否始终打开 `http://localhost:<port>`？
- 默认 workspace 的优先级：localStorage（用户） vs 会话 metadata vs 服务端 default workspace，最终规则以哪个为准？
- onboarding 触发条件：仅在“没有 workspace 且工具需要 workspace”时强提示，还是每次新会话都提示？

