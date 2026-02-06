## 1. Implementation
- [x] 1.1 盘点所有 `openspec/specs/*/spec.md`，列出 `Purpose: TBD` 清单并逐项补齐
- [x] 1.2 在 `scripts/` 增加 OpenSpec truth check 脚本：
  - [x] 扫描 capability spec 的 `Purpose` 占位问题
  - [x] 扫描 roadmap 的状态快照与 `openspec list` 的一致性（best-effort）
- [x] 1.3 在本地可复用入口中挂载该校验（例如 Makefile 或脚本聚合入口）
- [x] 1.4 在 CI 中加入 truth check（不阻塞已有测试矩阵）
- [x] 1.5 更新 roadmap 文档中的“状态来源”说明，明确以命令输出为准

## 2. Validation
- [x] 2.1 运行 `openspec validate update-openspec-truth-alignment --strict --no-interactive`
- [x] 2.2 运行新增 truth check 脚本并确认 exit code 正确
- [x] 2.3 抽样验证至少 3 个 capability 的 Purpose 非占位且语义清晰

## 3. Rollout
- [x] 3.1 在开发文档中增加“提交前必须执行 truth check”的说明
- [x] 3.2 将该 change 的执行结果回写到 roadmap 的状态快照规则中
