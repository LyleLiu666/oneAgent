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
- `ONEAGENT_HOME/.oneagent/PLAN.md`（当启用 workspace 时 `ONEAGENT_HOME=<workspace>/`）

### 2) Observer（观察者）
Observer 是一次独立的校验执行单元：
- 输入：单个任务（description + acceptance + scope + workspace_root）
- 工具：默认只读（读文件/搜索/列目录）；默认不允许执行命令验收（仅基于文件内容/结构判定）
- 输出：`pass|fail` + 原因（fail 必须可操作）

Observer 不需要看到主/子 Agent 的对话上下文，也不需要读取 trace/log；它只看交付件。

验收策略（默认）：
- observer 仅基于 `acceptance.files` 与可选的“文件内容断言”（例如 `acceptance.must_contain`）判定 pass/fail
- 所有验收涉及的文件路径必须位于 `ONEAGENT_HOME` 内；若 task 声明了 `scope`，则还必须匹配 `scope`（glob）

## Plan 文件格式（默认）
优先采用“可读 + 易解析”的 Markdown 结构，类似现有 `tasks.md`：

```md
# PLAN

## 1. Backend API
- [ ] 实现 /api/foo <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/internal/foo/foo.go
    - must_contain:
      - backend/internal/foo/foo.go: "func"
```

解析规则（默认）：
- 任务行用 `- [ ]` / `- [x]` 表示状态
- `<!-- id: ... -->` 提供稳定 id
- `scope:`、`acceptance:` 作为语义块（缩进的子项）
- `scope` 使用 glob（相对 `ONEAGENT_HOME` 的相对路径），用于约束该任务允许写入/修改的范围

### Scope（glob）规则（默认）
- scope 是一个 glob 列表（允许 `*`、`?`、`**`），匹配对象为“相对 `ONEAGENT_HOME` 的相对路径”，路径分隔符统一使用 `/`。
- 默认大小写敏感；不支持否定模式（例如 `!foo/**`）。
- glob 不得是绝对路径（不得以 `/` 开头），不得包含 `..` 片段；发现非法 scope 时必须直接报错。
- 判断某个文件是否可写时，系统必须同时满足：
  1) 目标路径解析后位于 `ONEAGENT_HOME` 内（防止 `..` 与 symlink 越界）
  2) 若 scope 非空，则目标相对路径至少匹配一个 glob
- scope 的 path 规范化与 glob 匹配逻辑必须由 plan/subagent/文件工具三方复用同一实现（避免规则漂移）。

## 工具接口 (API Sketch)

### plan tool（供主 Agent/子 Agent 调用）
以单一工具 `plan` 暴露多个动作（避免引入多个 tool id）：
- `plan(action=init, template?, overwrite?)` → 创建默认 `PLAN.md`（路径固定为 `<workspace>/.oneagent/PLAN.md`）
- `plan(action=get)` → 返回任务列表（id/title/status/scope/acceptance 摘要）
- `plan(action=mark_done, task_id)` → 触发 observer 校验；通过则写回 `PLAN.md`，失败则返回失败原因并不写回

### observer runner（系统内部）
- `observer.validate(oneagent_home, task)` → `{pass, reason, evidence?}`

关键约束：
- `plan.mark_done` 必须是“原子操作”：要么校验通过并写回，要么失败且不更改状态
- 校验失败要返回可供 agent 重试的明确原因

## 与 subagent 的集成
- subagent 执行某个任务时，可携带该任务的 `scope`（glob）
- 文件工具层对写/改/删强制校验 scope（越界直接报错）
- subagent 若调用 `plan.mark_done`：
  - 结果必须自动拼接进 subagent handoff（summary/findings）中，便于主 Agent 了解任务是否真正验收通过

> 已知限制：`bash/run_command` 在宿主机上运行，可能绕过文件工具层的 scope 校验。出于灵活性与实现成本考虑（也无法彻底防止通过脚本/编辑器修改文件），当前不做硬性拦截，仅做强引导：默认 `BASH_ROOT_DIR` 对齐 `ONEAGENT_HOME`（workspace），并在提示词/错误信息中强调“优先用文件工具修改文件；bash 主要用于只读/运行命令”。

## 并发策略（后置）
并发 subagent 的必要条件：
1. 计划模块可为每个 subagent 分配不重叠的 scope
2. 文件工具强制 scope 校验（避免越界编辑）

如果仍出现资源冲突：
- 优先拒绝并发（提示 scope 重叠）
- 锁作为最后手段：只允许短时间文件级锁，避免长等待拖垮体验
