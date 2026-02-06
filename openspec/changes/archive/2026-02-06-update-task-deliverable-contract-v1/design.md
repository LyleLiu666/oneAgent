## Context
任务交付的核心价值是“可直接验收”。没有稳定契约，前端和复盘流程都会反复适配，最终吞噬研发效率并削弱信任。

## Goals / Non-Goals
- Goals:
  - 构建 schema-versioned 的 artifacts 合约
  - 保证成功/失败都能输出可追溯交付证据
  - 让 receipt 与 artifacts 保持强一致
- Non-Goals:
  - 本 change 不重构 task queue 调度逻辑
  - 不引入新的交付类型（先统一现有类型）

## Decisions
- Decision: 采用 `artifact_manifest_version=v1` 显式版本化。
- Decision: 缺失字段必须写明原因（reason code + hint）。
- Decision: receipt 只保存指针，不复制大型内容。

## Risks / Trade-offs
- 风险：历史数据兼容成本增加。
- 缓解：增加兼容层并逐步迁移。

## Migration Plan
1. 引入 v1 schema 与 writer
2. 兼容读取旧数据
3. 全量切换 API 输出
