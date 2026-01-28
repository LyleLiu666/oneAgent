# oneAgent Roadmap（Spec-first backlog 与优先级）— 2026-01-28（更新）

目的：把“还没做的工作”固化成 **可执行的 backlog**，避免上下文压缩后遗忘；并按“地基 → 上层”的依赖关系推进。

> Source of truth：
> - 交付/实现进度：`openspec list`
> - 规格内容：`openspec/changes/*` 与 `openspec/specs/*`

---

## 0) 当前状态快照（以 `openspec list` 为准）

### 0.1 Active changes（已写规格，待实现）
- `add-project-scripts`：workspace project config（setup/test/cleanup/dev_server/copy_files）+ 证据留存
- `fix-skill-read-not-found-ux`：skill.read not-found 的可行动 UX
- `add-diff-review-loop`：attempt diff artifacts + review comments + succeeded 后 follow-up attempt
- `add-worktree-attempt-isolation`：git worktree 隔离 attempt（执行根目录 + 生命周期管理）
- `add-mcp-server`：对外暴露 MCP server（local-only + auth/policy + events）

### 0.2 已完成（Complete）
- `add-skill-governance-workbench`
- `add-sop-governance-workbench`
- `add-skill-governance-duplicates`
- `add-skill-governance-edit`
- `add-sidebar-ledger-status-badges`

### 0.3 交付风险：工作区未提交变更
进入实现阶段前，必须把本地改动整理为可回滚提交（否则“可复现/可推广”不成立）。

---

## 1) P0（地基 / 必须先做）

### P0.1 `add-project-scripts`（进来就能干活）
**价值**：把 setup/test/cleanup/dev server/copy_files 显性化为配置资产，并纳入证据链，减少“每次都探索环境”。

**关键坑**
- 脚本执行必须严格受 `system-tool-permissions` 约束（不能绕过 policy）。
- `copy_files` 必须做路径逃逸校验（防止复制 workspace 外敏感文件）。
- 失败留痕：stdout/stderr 与可操作错误必须齐全，否则排障成本爆炸。

### P0.2 `fix-skill-read-not-found-ux`（低成本高收益）
**价值**：减少治理/学习阶段的“读不到 skill 却不知道怎么办”的摩擦，提升可用性。

---

## 2) P1（核心体验：可审查交付 + 低风险迭代）

### P1.1 `add-diff-review-loop`（审查闭环）
**价值**：把交付从“聊天输出”升级为“可审查产物”，review comment 直接进入下一轮 attempt。

**关键坑**
- diff artifacts 可能很大：需要大小上限与降级路径（只列 changed files + explain）。
- follow-up attempt 从 `succeeded` 创建时要避免“覆盖历史结论”；必须保留 attempt history 与来源链路。

### P1.2 `add-worktree-attempt-isolation`（隔离执行）
**价值**：把“并发/污染/回滚”问题降维为 git 合并问题，天然支持审查与回退。

**关键坑**
- Windows 文件占用/路径长度：worktree 清理与回收要足够鲁棒。
- 非 git workspace：必须给出明确错误或按配置退化（不得 silent fallback）。
- 生命周期：孤儿 worktree 的识别与清理需要证据与可操作提示。

---

## 3) P2（生态/集成：把 oneAgent 变成可编排运行时）

### P2.1 `add-mcp-server`（标准协议入口）
**价值**：让外部客户端通过 MCP 读取/订阅/管理任务与账本，支撑通知/日报与“挂机收割”场景。

**关键坑**
- 默认必须 local-only；远程访问必须显式开启且复用 auth/policy。
- MCP 调用必须进入证据链（否则排障与审计断裂）。

---

## 4) 下一步执行顺序（建议）
按“收益/依赖/风险”综合排序：
1) `add-project-scripts`（unblock 环境可复现）
2) `add-diff-review-loop`（把交付变成可审查）
3) `add-worktree-attempt-isolation`（把隔离执行变成默认路径）
4) `add-mcp-server`（对外入口，可与上面并行实现但建议先只读）
5) `fix-skill-read-not-found-ux`（穿插做，随时可落地）
