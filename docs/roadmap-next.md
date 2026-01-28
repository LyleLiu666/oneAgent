# oneAgent Roadmap（工作清单与优先级）— 2026-01-28（更新）

目的：把“还没做的工作”固化成可执行的 backlog，避免上下文压缩后遗忘；并确保按“地基 → 上层”的依赖关系推进。

---

## 0) 当前状态快照

### 0.1 OpenSpec changes
以 `openspec list` 为准：
- **已完成（Complete）**：task queue / work ledger / SOP & skill governance 基础闭环已具备
- **待实现（Active changes）**：剩余特性已全部落为 OpenSpec changes（proposal/tasks/specs）

当前待实现的 changes（未完成）：
- `add-office-export`（Markdown → Office 导出）

已完成并归档（2026-01-28）：
- `add-cost-governance-limits`
- `add-task-evidence-policy`
- `add-skill-governance-merge`
- `add-code-intelligence-tools`
- `add-edit-v2-tool`
- `add-prompt-assetization`

### 0.2 交付风险：工作区未提交变更
在进入下一阶段前，必须先把本地改动整理为可回滚的提交（否则“可复现/可推广”不成立）。

---

## 1) P0（地基 / 必须先做）

### P0.1 CI 扩展：把“本机绿”升级为“持续绿”
**状态**：已落地（见 archived change `add-ci-daily-runs` / `openspec/specs/project-tooling/spec.md`）

**验收（本地可跑）**
- `cd backend && go test ./...`
- `cd frontend && npm test -- --run`
- `scripts/e2e_smoke_test.sh`

### P0.2 Release/分发闭环（部门推广前置）
**状态**：release workflow/checksums/PortableGit resolution 已落地（见 archived change `add-release-artifacts-workflow` / `update-release-portablegit-resolution`）

### P0.3 工具权限治理（Policy engine）
**状态**：已归档完成（见 archived change `2026-01-28-add-tool-permissions-system` / `openspec/specs/system-tool-permissions/spec.md`）

### P0.4 OpenSpec 归档
**目标**：完成一个 change 并上线后再归档（保持 active 清爽）。

---

## 2) P1（核心体验：挂机交付与任务工作台）

### P1.1 完成通知/日报
**状态**：UI 内通知 + badge 已落地（见 archived change `add-task-ui-notifications` / `add-ledger-status-badges`）

后续如需外部通知（webhook/IM），再单独开 change。

### P1.2 TaskQueue 工作台深化
**状态**：基础 workbench 已落地（见 archived change `add-taskqueue-workbench-ux`）

**坑**
- workspace 的共享状态污染：需要明确“串行 + 产物/证据驱动 resume”，避免隐式重入。
- 若未来允许并发写入：必须引入 L2（OCC 条件写入）或 L3（worktree）避免“基于旧版本探索→写入到新版本”。

### P1.3 默认 limits / 成本治理
**状态**：steps/runtime 默认 limits 已落地（见 archived change `add-default-task-limits`）

成本/Token 治理（max_cost/token cap）已落地（见 archived change `2026-01-28-add-cost-governance-limits`）。

---

## 3) P2（留存复利：学习→治理→召回的下一层）

### P2.1 “已物化 skill”的治理工作台
**状态**
- 列表/编辑/归档/duplicates：已落地（`add-skill-governance-workbench` / `add-skill-governance-edit` / `add-skill-governance-duplicates`）
- merge/pin canonical：已落地（见 archived change `2026-01-28-add-skill-governance-merge`）

**坑**
- “稀缺性/深度”随时间变化：需要定期重评与人工治理入口。

### P2.2 强制证据进一步制度化
**状态**：已落地（见 archived change `2026-01-28-add-task-evidence-policy`）

---

## 4) P3（上限增强：可后置）

### P3.1 edit_v2（可解释/可证明的编辑）
对应 change：`add-edit-v2-tool`

### P3.2 语义级工具（LSP/AST）
对应 change：`add-code-intelligence-tools`

### P3.3 Prompt 资产化
对应 change：`add-prompt-assetization`

### P3.4 Office “最后一公里”导出
对应 change：`add-office-export`

---

## 5) 执行顺序（下一步从这里开始）
以 `openspec list` 中 active changes 为准，建议顺序：
1) `add-office-export`（最后一公里导出）
