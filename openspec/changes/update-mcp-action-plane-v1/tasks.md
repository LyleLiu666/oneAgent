## 1. MCP action methods
- [ ] 1.1 设计并实现 MCP 动作方法：create/resume/cancel task
- [ ] 1.2 明确参数 schema 与错误码（含 request_id）
- [ ] 1.3 保证动作方法与 HTTP API 语义一致（状态机一致）

## 2. Security and approvals
- [ ] 2.1 复用 principal auth 与 policy 校验（禁止绕过）
- [ ] 2.2 对高风险动作接入审批约束（manual/auto）
- [ ] 2.3 增加授权防重放（attempt/action scope）

## 3. Auditability
- [ ] 3.1 记录 MCP action 调用证据（method/args hash/principal/result）
- [ ] 3.2 将 MCP action 与 task/attempt events 建立双向引用
- [ ] 3.3 增加失败路径审计（policy denied / approval required / invalid args）

## 4. Validation
- [ ] 4.1 MCP 集成测试：未授权拒绝、授权通过、审批流
- [ ] 4.2 task queue 测试：MCP 触发状态机与 HTTP 路径一致
- [ ] 4.3 运行 `openspec validate update-mcp-action-plane-v1 --strict --no-interactive`
