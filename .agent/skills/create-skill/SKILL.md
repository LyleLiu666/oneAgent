---
name: create-skill
description: 在本仓库内创建并预置一个可被 oneAgent 发现/召回/skill.read 的 Skill 包（SKILL.md + 可选 scripts/references/assets）
tags:
  - oneagent
  - skills
keywords:
  - SKILL.md
  - frontmatter
  - discover
---

# create-skill

## 目标

把一个新的 skill 以“目录包”的方式加进项目，保证：
- 能随 oneAgent 二进制发布（内置 skills）
- 能被 `oneagent skills search` 召回到
- 能在对话中通过 `skill.read` 读取到完整 `SKILL.md`

## 放置位置（推荐顺序）

当会话启用 workspace 后，oneAgent 按优先级发现 skills（同名/同 ID 先出现者生效）：
1) `<workspace>/.oneagent/skills/**/SKILL.md`（workspace 私有覆盖；默认被 `.gitignore` 忽略）
2) `<workspace>/skills/**/SKILL.md`（推荐：随仓库提交的项目 skills）
3) `<workspace>/.claude/skills/**/SKILL.md`（兼容 Claude Code 的项目内 skills）
4) `~/.claude/skills/**/SKILL.md`
5) `~/.codex/skills/**/SKILL.md`
6) `.builtin`（内置 skills，随二进制发布，path 形如 `builtin:skills/<id>/SKILL.md`）

## 创建步骤（Checklist 驱动）

### 1) 选一个稳定的 skill id

- 形式：`kebab-case`（例：`db-migration-helper`）
- 建议：id 就是目录名；`name` 也用同一个字符串（便于显式指定技能名解析）

### 2) 创建目录包

在本仓库内创建：

```
backend/internal/builtinskills/skills/<skill-id>/
└── SKILL.md
```

（可选）如果需要确定性/可复用能力，再加：
- `scripts/`：可执行脚本（bash/python/...）
- `references/`：需要时再读取的参考文档（避免把大段内容塞进 SKILL.md）
- `assets/`：模板/素材（一般不进上下文）

### 3) 编写 SKILL.md（必须包含 YAML frontmatter）

最小 frontmatter：
- `name`（必填）
- `description`（必填，写清“什么时候用/触发词/适用范围”）

建议可选：
- `tags`、`keywords`（帮助召回）
- `requires`（声明依赖/可用性，用于 eligible 过滤与 status/check 输出）
- `install`（声明结构化安装建议，用于 status/check 输出）

`requires` 字段（可选）：
- `os`: ["darwin"|"linux"|"windows"]（非空则限制 OS）
- `bins`: 必需存在的可执行文件名列表（全部满足）
- `any_bins`: 至少存在其一的可执行文件名列表（满足其一）
- `env`: 必需存在的环境变量名列表

`install` 字段（可选）：安装建议列表（机器可读），常用 `kind`：
- `brew`（`formula`）
- `go`（`module`）
- `node`（`package`）
- `uv`（`package`）
- `download`（`url`）
- `command`（`command`）

示例：

```yaml
---
name: apple-notes
description: Manage Apple Notes via memo
requires:
  os: darwin
  bins: [memo]
  any_bins: [rg, grep]
  env: [OPENAI_API_KEY]
install:
  - kind: brew
    formula: antoniorodr/memo/memo
    bins: [memo]
  - kind: brew
    formula: ripgrep
    bins: [rg]
---
```

正文建议：
- 先写“什么时候用/不该用”
- 再写“固定流程（分步）”
- 最后写“验收方式/失败处理”

### 4) 本地验证（TDD）

1. 召回是否能找到（Top-8）：
   - `cd backend && go run ./cmd/oneagent skills search --query "<skill-id>"`
2. 可用性与依赖是否清晰：
   - `cd backend && go run ./cmd/oneagent skills status --workspace "<your-workspace>"`
3. 对话中是否能读到（tool 输出）：
   - 在 chat 输入“请使用 <skill-id> 这个技能 …”
   - 确认模型先调用 `skill.read`，并拿到你写的 `SKILL.md`

## 注意事项

- 不要在 skill 中写入敏感信息（token/私钥/账号等）
- SKILL.md 尽量短；大段资料放到 `references/`
- 需要高可靠重复执行的动作，优先落 `scripts/` 而不是让模型现场“现写现跑”
