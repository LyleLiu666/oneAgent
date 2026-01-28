# Change: Add defense-in-depth security (update vision + guardrails)

## Why
oneAgent 是 local-first agent client，但它天然具备“读写本地文件/执行命令/接入外部内容”的能力。
当 oneAgent 从个人工具走向部门分发时，IT/安全/运维会把它当作一套“可执行平台”来审视：入口、身份、数据边界、工具权限、网络暴露、审计与回溯都需要有可解释的答案。

本变更引入一套与产品愿景对齐的“纵深防御”安全框架：把安全做成多层兜底，而不是单点依赖提示词或用户自觉。

## What Changes
- 更新愿景：在 `docs/ux-vision.md` 中补齐“纵深防御安全”作为信任感底座，并给出可验证指标
- 增强框架：将纵深防御拆解为可落地的 specs（入口控制、外部内容包装、可疑模式检测、模型安全档位、工具沙箱、网络隔离）
- 默认策略：默认安全（最小权限 + 可审计 + 可回收），并允许管理员按需放开能力

## Impact
- Affected specs: `auth-mode`, `local-runtime`, `system-prompt-assembly`, `system-tool-permissions`, `llm-provider-management`
- Affected code (planned): `backend/internal/middleware/*`, `backend/internal/prompt/*`, `backend/internal/tool/*`, `frontend/src/*`
- **Potential BREAKING**: 是否要将默认 `bind` 从 LAN 可访问改为 loopback（需要明确产品默认“可访问性 vs 安全”的取舍；见 design）
