## 1. Artifact contract
- [x] 1.1 定义 artifact manifest v1（JSON schema + version 字段）
- [x] 1.2 统一 attempt 终态写入逻辑，确保关键字段稳定存在或给出缺失原因
- [x] 1.3 对非 git、无测试等场景补齐降级字段与解释文案

## 2. Receipt mapping
- [x] 2.1 将 receipt artifacts 映射到 manifest v1 字段（一一对应）
- [x] 2.2 增加 receipt 侧字段一致性校验，避免 UI 读取歧义
- [x] 2.3 保证失败 attempt 仍产出完整证据框架（含缺失原因）

## 3. API and UX compatibility
- [x] 3.1 增加 API 合约测试，验证关键字段稳定返回
- [x] 3.2 增加前端回归测试，验证 deliverable 入口可打开
- [x] 3.3 兼容旧 artifacts（best-effort）并给出迁移策略

## 4. Validation
- [x] 4.1 运行 `openspec validate update-task-deliverable-contract-v1 --strict --no-interactive`
- [x] 4.2 运行 backend + frontend 测试，覆盖成功/失败/降级三类路径
- [x] 4.3 抽样验证至少 5 个 attempt 的 receipt 与 artifacts 一致性
