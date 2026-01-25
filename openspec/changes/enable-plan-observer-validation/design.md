# 设计: 计划模块与观察者校验 (Plan + Observer Validation)

## 目标 (Goals)
- 为复杂任务提供可持久化的计划文件（可审计、可回溯）
- 将“标记 done”变成“先验收再 done”（TDD 思想）
- 观察者独立于主/子 Agent 上下文，仅根据交付件判断是否达标
- 为未来并发 subagent 提供 scope 分区与越界写入拦截机制

## 核心概念 (Core Concepts)

### 1) Plan（计划）
Plan 是一个文件化的任务清单，每个任务至少包含：
- `id`：稳定标识
- `title/description`：任务描述
- `acceptance`：验收标准（必须可执行/可检查）
- `status`：todo / doing / done（done 只能由 observer 校验通过后写入）
- `scope`（可选）：允许修改的目录范围（用于 subagent 分工与越界拦截）

默认路径（项目私有、避免进 git）：
- `<workspace>/.oneagent/PLAN.md`

### 2) Observer（观察者）
Observer 是一次独立的校验执行单元：
- 输入：单个任务（description + acceptance + scope + workspace_root）
- 工具：默认只读（读文件/搜索/列目录）；可选允许执行验收命令（仅来自 acceptance 列表）
- 输出：`pass|fail` + 原因（fail 必须可操作）

Observer 不需要看到主/子 Agent 的对话上下文，也不需要读取 trace/log；它只看交付件。

## Plan 文件格式 (MVP)
优先采用“可读 + 易解析”的 Markdown 结构，类似现有 `tasks.md`：

```md
# PLAN

## 1. Backend API
- [ ] 实现 /api/foo <!-- id: 1 -->
  - scope: backend/
  - acceptance:
    - files:
      - backend/internal/foo/foo.go
    - commands:
      - go test ./backend/...
```

解析规则（MVP）：
- 任务行用 `- [ ]` / `- [x]` 表示状态
- `<!-- id: ... -->` 提供稳定 id
- `scope:`、`acceptance:` 作为语义块（缩进的子项）

## 工具接口 (API Sketch)

### plan tool（供主 Agent/子 Agent 调用）
- `plan.init(workspace_root, template?)` → 创建默认 `PLAN.md`
- `plan.get(workspace_root)` → 返回任务列表（id/title/status/scope/acceptance 摘要）
- `plan.mark_done(workspace_root, task_id)` → 触发 observer 校验；通过则写回 `PLAN.md`，失败则返回失败原因并不写回

### observer runner（系统内部）
- `observer.validate(workspace_root, task)` → `{pass, reason, evidence?}`

关键约束：
- `plan.mark_done` 必须是“原子操作”：要么校验通过并写回，要么失败且不更改状态
- 校验失败要返回可供 agent 重试的明确原因

## 与 subagent 的集成
- subagent 执行某个任务时，可携带该任务的 `scope`（目录范围）
- 文件工具层对写/改/删强制校验 scope（越界直接报错）
- subagent 若调用 `plan.mark_done`：
  - 结果必须自动拼接进 subagent handoff（summary/findings）中，便于主 Agent 了解任务是否真正验收通过

## 并发策略（后置）
并发 subagent 的必要条件：
1. 计划模块可为每个 subagent 分配不重叠的 scope
2. 文件工具强制 scope 校验（避免越界编辑）

如果仍出现资源冲突：
- 优先拒绝并发（提示 scope 重叠）
- 锁作为最后手段：只允许短时间文件级锁，避免长等待拖垮体验

