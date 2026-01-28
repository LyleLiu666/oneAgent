# Change: Fix skill.read "skill not found" UX

## Why
当前用户在对话里询问“有哪些/是否有 skills 可以用”时，模型容易去调用 `skill.read` 读取一些并不存在的 skill（例如 `translator/python/bash`），导致连续 tool error：
- 用户无法获知“当前环境到底有哪些 skills 可用”
- 体验上像是系统能力故障（但实际可能只是没有安装对应 skill）
- 错误信息缺少下一步指引（去哪里看列表/如何确认可用性/如何修正拼写）

## What Changes
- `skill.read` 在未找到 skill 时，返回**可行动**的错误信息（包含规范化后的标识、相似候选 suggestions、以及查看 skills 列表/可用性的 next steps）
- 当用户显式询问“有哪些 skills/skill 可用”时，系统在 TurnContext（volatile）注入“技能帮助”块，指引用户：
  - 打开技能治理页面查看列表
  - 或运行 `oneagent skills status` 查看可用性/缺失依赖
  - （可选）展示 Top-N（N≤10）技能名称摘要，避免让模型/用户靠猜

## Impact
- Affected specs: `system-skill-management`
- Affected code (expected): `backend/internal/tool/skill_read.go`, `backend/internal/handler/skills_context.go`, `frontend/src/views/*`（如需在 UI 做入口提示）

