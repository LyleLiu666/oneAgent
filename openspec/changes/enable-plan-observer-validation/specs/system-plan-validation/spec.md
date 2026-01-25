# 规范: 系统计划与观察者校验 (System Plan & Observer Validation)

## ADDED Requirements

### Requirement: 支持计划文件（Plan File）
系统必须 (MUST) 支持在启用 workspace 的情况下创建并维护一个计划文件，用于承载复杂任务的分解、验收标准与进度。

#### Scenario: 默认创建计划文件
- **GIVEN** 会话启用 workspace，根目录为 `<workspace>/`
- **WHEN** 系统初始化计划
- **THEN** 在 `<workspace>/.oneagent/PLAN.md` 创建计划文件

### Requirement: 标记 done 需要观察者校验（TDD）
系统必须 (MUST) 在任务被标记为 done 时引入观察者校验：只有当观察者判定任务达标时，该任务才会被真正写为 done；否则必须拒绝标记并返回原因。

#### Scenario: 未达标时 done 失败
- **GIVEN** 计划中存在任务 T1，且其验收标准尚未满足
- **WHEN** 主 Agent 或子 Agent 尝试将 T1 标记为 done
- **THEN** 系统返回 `【plan中某个任务标记done失败】` 与原因
- **THEN** 计划文件中 T1 仍保持未完成状态

#### Scenario: 达标时 done 成功并写回
- **GIVEN** 计划中存在任务 T1，且其验收标准已满足
- **WHEN** 主 Agent 或子 Agent 尝试将 T1 标记为 done
- **THEN** 系统将 T1 写为 done
- **THEN** 系统返回成功结果

### Requirement: 观察者独立于主/子 Agent 上下文
系统必须 (MUST) 将观察者作为独立执行单元运行：观察者不得依赖主/子 Agent 的对话上下文，仅根据“任务描述 + 验收标准 + 交付件”做判断。

#### Scenario: 观察者不读取对话上下文
- **GIVEN** 主 Agent 与子 Agent 在执行中产生了大量对话与 trace
- **WHEN** 观察者对某个任务执行校验
- **THEN** 观察者的输入不包含主/子 Agent 的完整对话上下文（仅包含该任务信息与必要引用）

### Requirement: 任务可声明 scope 并用于约束写入范围
系统必须 (MUST) 支持在计划任务中声明可编辑范围（scope，workspace 的子目录集合），并在执行该任务的 agent/subagent 文件写操作时强制执行越界拦截。

#### Scenario: scope 越界写入被拒绝
- **GIVEN** 任务 T1 的 scope 为 `<workspace>/backend/`
- **WHEN** 执行 T1 的 subagent 尝试修改 `<workspace>/frontend/App.vue`
- **THEN** 系统拒绝该写/改/删操作并返回清晰错误

### Requirement: subagent 标记 done 时在 handoff 中体现
系统必须 (MUST) 在子 Agent 标记任务 done（无论成功或失败）时，将“标记结果/校验结果”自动体现在其 handoff（summary/findings）中，便于主 Agent 决策下一步。

#### Scenario: handoff 包含标记 done 的结果
- **GIVEN** 子 Agent 执行完任务并尝试 `plan.mark_done`
- **WHEN** 子 Agent 返回 handoff 给主 Agent
- **THEN** handoff 中包含“任务 id + done 成功/失败 + 失败原因（若有）”的信息
