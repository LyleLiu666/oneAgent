# oneAgent Roadmap（Spec-first backlog 与优先级）— 2026-01-29（更新）

目的：把“还没做的工作”固化成 **可执行的 backlog**，避免上下文压缩后遗忘；并按“地基 → 上层”的依赖关系推进。

> Source of truth：
> - 交付/实现进度：`openspec list`
> - 规格内容：`openspec/changes/*` 与 `openspec/specs/*`

---

## 0) 当前状态快照（以 `openspec list` 为准）

### 0.1 Active changes（已写规格，待实现）
- `fix-skill-read-not-found-ux`：skill.read not-found 的可行动 UX
- `add-worktree-attempt-isolation`：git worktree 隔离 attempt（执行根目录 + 生命周期管理）
- `add-mcp-server`：对外暴露 MCP server（local-only + auth/policy + events）
- `add-secretary-mode-chat`：提供“秘书模式”纯聊天体验（低噪声 + 一键切换回完整模式）

### 0.2 已完成（Complete）
- `add-project-scripts`
- `add-diff-review-loop`
- `add-observer-remediation-loop`
- `add-execution-safety-invariants`
- `update-workbench-ux-fold-advanced`
- `add-skill-governance-workbench`
- `add-sop-governance-workbench`
- `add-skill-governance-duplicates`
- `add-skill-governance-edit`
- `add-sidebar-ledger-status-badges`

### 0.3 交付风险：工作区未提交变更
进入实现阶段前，必须把本地改动整理为可回滚提交（否则“可复现/可推广”不成立）。

---

## 1) P0（地基 / 必须先做）

### P0.1 `add-worktree-attempt-isolation`（隔离执行）
**价值**：把“并发/污染/回滚”问题降维为 git 合并问题，天然支持审查与回退。

**关键坑**
- Windows 文件占用/路径长度：worktree 清理与回收要足够鲁棒。
- 非 git workspace：必须给出明确错误或按配置退化（不得 silent fallback）。
- 生命周期：孤儿 worktree 的识别与清理需要证据与可操作提示。

### P0.2 `fix-skill-read-not-found-ux`（可穿插：低成本高收益）
**价值**：减少治理/学习阶段的“读不到 skill 却不知道怎么办”的摩擦，提升可用性。  
**定位**：不阻塞主线（L0/L1/L2），但每次碰到都值得顺手修掉。

---

## 2) P1（核心体验：可审查交付 + 低风险迭代）

### P1.1 `add-secretary-mode-chat`（像“只和秘书说话”一样简单）
**价值**：在不牺牲“复杂交互窗口（Tasks/Governance/Ledger）”的前提下，提供一个极低噪声的纯对话入口，让用户可以像使用 Moltbot/WebChat 那样只通过聊天完成委托与收割。

**关键坑**
- 不能把“简单”做成“功能缺失”：只是把复杂度折叠/隐藏，并确保一键回到完整模式。
- 默认路径不得耦合 UI 文案：测试必须使用 `data-testid`，避免 i18n/措辞变更导致脆弱。
- 证据链不丢：秘书模式隐藏 trace/工具细节，但必须可发现地打开查看（否则排障困难）。

---

## 3) P2（生态/集成：把 oneAgent 变成可编排运行时）

### P2.1 `add-mcp-server`（标准协议入口）
**价值**：让外部客户端通过 MCP 读取/订阅/管理任务与账本，支撑通知/日报与“挂机收割”场景。

**关键坑**
- 默认必须 local-only；远程访问必须显式开启且复用 auth/policy。
- MCP 调用必须进入证据链（否则排障与审计断裂）。

---

## 4) 下一步执行顺序（建议）
按“地基 → 上层（L0→L3）”综合排序：
1) `add-worktree-attempt-isolation`（把隔离执行变成默认路径）
2) `add-mcp-server`（对外入口，可与上面并行实现但建议先只读）
3) `add-secretary-mode-chat`（体验线：低噪声纯对话，可与 L2/L3 并行推进）
4) `fix-skill-read-not-found-ux`（穿插做，随时可落地）
