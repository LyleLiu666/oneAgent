# 任务列表 (Tasks)

- [ ] 创建 `internal/skill` 包用于技能发现 <!-- id: 1 -->
- [ ] 实现 `GetSystemSkillsDir()` 以解析 `~/.oneagent/skills` <!-- id: 2 -->
- [ ] 实现 `LoadSystemSkills()` 以扫描系统目录 <!-- id: 3 -->
- [ ] 更新 `backend/internal/handler/chat.go` 以使用 `SkillLoader` <!-- id: 4 -->
- [ ] 更新提示词构建逻辑，以**中文**追加技能列表 <!-- id: 5 -->
    - [ ] 创建提示词模板: "## 可用技能..." <!-- id: 6 -->
- [ ] 使用模拟的系统技能进行端到端 (E2E) 验证 <!-- id: 7 -->
