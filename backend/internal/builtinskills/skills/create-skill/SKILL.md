---
name: create-skill
description: 在本项目中创建一个新的 oneAgent Skill（会被打包进二进制），并确保可被发现/召回/skill_read（兼容 skill.read）
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
- 能被 oneAgent 发现（内置技能会随二进制发布，不依赖 workspace 路径）
- 能被 `oneagent skills search` 召回到
- 能在对话中通过 `skill_read` 读取到完整 `SKILL.md`（兼容旧名：`skill.read`）

## 创建步骤（Checklist 驱动）

### 1) 选一个稳定的 skill id

- 形式：`kebab-case`（例：`db-migration-helper`）
- 建议：`name` 就用这个 id（便于“显式指定技能名”解析）

### 2) 创建目录包

在代码仓库中创建（目录名就是 skill id）：

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

正文建议：
- 先写“什么时候用/不该用”
- 再写“固定流程（分步）”
- 最后写“验收方式/失败处理”

### 4) 本地验证（TDD）

1. 召回是否能找到（Top-8）：
   - `cd backend && go run ./cmd/oneagent skills search --query "<skill-id>"`
2. 对话中是否能读到（tool 输出）：
   - 在 chat 输入“请使用 <skill-id> 这个技能 …”
   - 确认模型先调用 `skill_read`，并拿到你写的 `SKILL.md`（兼容旧名：`skill.read`）

## 注意事项

- 不要在 skill 中写入敏感信息（token/私钥/账号等）
- SKILL.md 尽量短；大段资料放到 `references/`
- 需要高可靠重复执行的动作，优先落 `scripts/` 而不是让模型现场“现写现跑”
