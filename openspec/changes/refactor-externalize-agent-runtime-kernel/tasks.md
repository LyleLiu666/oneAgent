## 1. Phase 1 Implementation
- [x] 1.1 在 `backend` 接入 `agentsdk v0.5.2`，并使用本地 `replace` 指向外部仓库
- [x] 1.2 将 prompt cache key 生成切到 SDK shared helper
- [x] 1.3 将 prompt cache 的稳定/易变消息选择逻辑切到 SDK 语义
- [x] 1.4 将坏 tool arguments 容错切到 SDK shared implementation
- [x] 1.5 增加与 SDK 契约对齐的回归测试
- [x] 1.6 只改 turn context 时 cache key 不变
- [x] 1.7 改 summary / tool schema / prefix version 时 cache key 改变
- [x] 1.8 普通 4xx 不误触发 cache downgrade retry
- [x] 1.9 坏 JSON tool args 保留 `_raw` 且不直接中断 loop 入口
- [x] 1.10 运行 OpenSpec 校验与相关 Go 测试

## 2. Follow-up Runtime Swap
- [ ] 2.1 设计并实现 `oneAgent -> agentsdk` 的薄适配层（保留 session/SSE/trace）
- [ ] 2.2 将 worker chat JSON tool loop 切到 `agentsdk/toolloop` 或 `jsonprotocol`
- [ ] 2.3 将 secretary XML loop 切到 `agentsdk/toolloop` 或 `xmlprotocol`
- [ ] 2.4 删除已被适配层覆盖的本地重复运行内核代码
