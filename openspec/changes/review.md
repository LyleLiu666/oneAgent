# OpenSpec Changes: Priority & Progress

更新时间：2026-02-01

本文件只记录**当前 active changes** 的优先级与进度快照；历史内容不再维护。
已完成变更请看 `openspec/changes/archive/`，真实进度以 `openspec list` 为准。

## Priority（高 → 低）

### P0（核心体验 / 高 ROI）
- `update-app-shell-secretary-first`：默认只保留秘书模式；仅完全模式显示 Sidebar/菜单（入口与心智一致性）
- `add-session-context-compression-verification`：会话自动压缩可回归验证（确认“长会话不会失忆”机制生效）
- `add-chat-stream-recovery-and-stop`：对话流可中断/可恢复（“像微信一样随时停/继续”）
- `add-native-command-sandbox`：跨平台 native sandbox（workspaceRoot 边界、真删除、不断网）；不依赖 Docker 也能安全启用写能力命令

### P1（治理与自动化 / 降低损耗）
- （暂无）

### P2（中长期愿景 / 上层形态）
- `add-workflow-orchestration-graph`：工作流编排（显式图）：节点=工作型 agent；交付物=文件集；Hard/Soft Gate

## Snapshot（来自 `openspec list`）
- `update-app-shell-secretary-first`：2/13 tasks
- `add-session-context-compression-verification`：2/9 tasks
- `add-chat-stream-recovery-and-stop`：12/14 tasks
- `add-workflow-orchestration-graph`：✓ Complete
- `add-native-command-sandbox`：11/13 tasks
- `add-trash-file-tool`：✓ Complete
