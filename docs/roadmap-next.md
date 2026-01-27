# oneAgent Roadmap（工作清单与优先级）— 2026-01-27

目的：把“还没做的工作”固化成可执行的 backlog，避免上下文压缩后遗忘；并确保按“地基 → 上层”的依赖关系推进。

---

## 0) 当前状态快照

### 0.1 OpenSpec changes
运行 `openspec list` 时，以下 changes 均显示 **Complete**：
- `add-work-ledger-sop-learning`
- `add-autonomous-task-queue`
- `add-windows-support`
- `update-workspace-first-onboarding`
- `add-skill-eligibility-status`
- `add-read-file-tool`
- `add-atomic-write-file`

结论：**当前未完成工作主要来自“交付闭环 + 新需求/愿景 + 工程化治理”，而不是 tasks.md 的漏项。**

### 0.2 交付风险：工作区未提交变更
在进入下一阶段前，必须先把本地改动整理为可回滚的提交（否则“可复现/可推广”不成立）。

---

## 1) P0（地基 / 必须先做）

### P0.1 CI 扩展：把“本机绿”升级为“持续绿”
**目标**
- CI 覆盖 backend + frontend + e2e smoke
- 每天自动跑（schedule）+ 允许手动触发（workflow_dispatch）

**建议落地**
- 扩展 `.github/workflows/ci.yml`：
  - `backend`：继续 `scripts/ci_build.sh`
  - `frontend`：`npm ci` + `npm test -- --run`
  - `e2e`：`scripts/e2e_smoke_test.sh`（依赖 `make build`，以及 `python3/curl`）

**坑**
- Vitest 默认 watch，必须用 `npm test -- --run`（否则 CI 卡死）。
- e2e 需要端口选择、临时 HOME、以及 server 启动等待；脚本已处理但要确保 CI 环境有 `python3`。

**验收（本地可跑）**
- `cd backend && go test ./...`
- `cd frontend && npm test -- --run`
- `scripts/e2e_smoke_test.sh`

### P0.2 Release/分发闭环（部门推广前置）
**目标**
- Windows zip + checksums 在 CI 可产出（build-only 或 release workflow）
- 明确 bundled vs system Git Bash 的诊断输出（doctor）

**坑**
- PortableGit 下载与许可证合规（NOTICE 已有，但需要在 release 产物内稳定包含）。
- Windows 真实环境验证：路径空格、杀软拦截、权限/文件锁导致 rename 失败等。

### P0.3 OpenSpec 归档
**目标**
- 将已完成 changes 移入 `openspec/changes/archive/YYYY-MM-DD-*`，保持 active 清爽。

---

## 2) P1（核心体验：挂机交付与任务工作台）

### P1.1 完成通知/日报
**目标**
- Task attempt / learning / digest 在用户不在线时也能“可被看见”（UI badge 或通知）

**建议落地路径（从低到高）**
1) UI 上增加 “今日 digest / learning job 状态 / SOP proposed 数量” 的明显入口与 badge（已落地：`add-ledger-status-badges`）
2) schedule job 输出日报文件（本地），并在 UI 提示“有新日报”
3) webhook/企业 IM 通知（后续 change）

### P1.2 TaskQueue 工作台深化
**目标**
- 同 workspace 串行、跨 workspace 并行的策略可视化
- 批量操作（cancel/resume）、失败自动转“下一步可 resume 的任务”

**坑**
- workspace 的共享状态污染：需要明确“串行 + 产物/证据驱动 resume”，避免隐式重入。

### P1.3 默认 limits / 成本治理
**目标**
- 默认 max_runtime/max_steps/max_cost（或 token cap）
- 超限行为可解释（被取消/被降级/需要人工确认）

**坑**
- provider 之间 usage/cost 字段不一致；需要先统一抽象与日志留痕。

---

## 3) P2（留存复利：学习→治理→召回的下一层）

### P2.1 “已物化 skill”的治理工作台
**目标**
- 对已 materialize 的 skills：去重/合并/过时 drop/归档（skills-archived 不参与 recall）

**坑**
- “稀缺性/深度”随时间变化：需要定期重评与人工治理入口。

### P2.2 强制证据进一步制度化
**目标**
- 把“强制证据”从学习侧扩展到更多交付路径（比如任务执行默认生成测试报告并入 artifacts）。

---

## 4) P3（上限增强：可后置）

### P3.1 edit_v2（可解释/可证明的编辑）
- 命中区间/策略/score、多候选强失败、occurrence/anchor 支持

### P3.2 语义级工具（LSP/AST）
- 最小子集：definition/references/rename-preview（输出改动清单，不直接写）

### P3.3 Prompt 资产化
- 把散落经验固化为 prompts modules，并加测试保证关键约束存在

### P3.4 Office “最后一公里”导出
- Markdown→docx/pptx/xlsx（pandoc + 模板），作为非开发者交付闭环

---

## 5) 执行顺序（下一步从这里开始）
1) **P0.1 CI 扩展 + 每天自动跑**
2) P0.2 release/分发（至少 build artifacts / checksums）
3) P1.1 完成通知/日报（先 UI badge）
4) P1.3 默认 limits（防失控）
5) P2.* skill 治理与复利强化
