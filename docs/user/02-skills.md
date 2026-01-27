# Skills（知识包）

核心洞察：**Tools 决定“能做什么”，Skills 决定“怎么做得更好”**。

在 oneAgent 里，skill 是一个“目录包”，至少包含一个 `SKILL.md`（可选 `scripts/`、`references/`、`assets/`）。

## 怎么使用 skills？

### 1) 让系统发现你的 skill

当会话启用 workspace 后，oneAgent 会按优先级发现 skills（同名/同 ID 先出现者生效）：

1. `<workspace>/.oneagent/skills/**/SKILL.md`（workspace 私有覆盖；默认被 `.gitignore` 忽略）
2. `<workspace>/skills/**/SKILL.md`（推荐：随仓库提交的项目 skills）
3. `<workspace>/.claude/skills/**/SKILL.md`（兼容 Claude Code 项目内 skills）
4. `~/.claude/skills/**/SKILL.md`
5. `~/.codex/skills/**/SKILL.md`
6. `.builtin`（内置 skills，随二进制发布）

### 2) 验证可用性（eligible）与缺失依赖

```bash
cd backend
go run ./cmd/oneagent skills status --workspace "$(pwd)/.."
```

status/check 会展示：
- 是否 eligible（当前 OS/PATH/env 是否满足）
- 缺失的 bins/env
- `install` 给出的安装建议（如果 skill 声明了）

### 3) 在对话中触发

你可以直接说“请使用 `<skill-name>` 这个技能……”，模型会通过 `skill.read` 读取完整 `SKILL.md` 后再执行。

## 写一个最小 skill（模板）

目录结构：

```
skills/my-skill/
└── SKILL.md
```

`SKILL.md` 最小示例：

```markdown
---
name: my-skill
description: 什么时候用这个技能（触发条件/适用范围）
---

# my-skill

## 使用时机
- …

## 固定流程（步骤）
1. …

## 验收方式
- …
```

（可选）声明依赖与安装建议：
- `requires`: `{ os, bins, any_bins, env }`
- `install`: `[{ kind: brew|go|node|uv|download|command, ... }]`

## 最佳实践（重要）

- SKILL.md 尽量短：把长资料放 `references/`，需要时再让模型读取
- 高可靠/可复用动作优先写进 `scripts/`（而不是让模型每次现写现跑）
- 不要写入敏感信息（token/私钥/账号等）

