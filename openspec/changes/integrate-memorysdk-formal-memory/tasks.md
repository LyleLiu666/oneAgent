## 1. Spec And Design
- [x] 1.1 为 formal memory 接入补齐 proposal / design / spec deltas
- [x] 1.2 运行 `openspec validate integrate-memorysdk-formal-memory --strict --no-interactive`

## 2. Phase 1 Runtime Integration
- [x] 2.1 在 `backend` 中引入外部 `memorySdk` 依赖与本地 `replace`
- [x] 2.2 新增薄的 `formalmemory` 适配层，直接复用 external store 与 bridge
- [x] 2.3 为 `runtime` 增加可选 formal memory 初始化与关闭逻辑
- [x] 2.4 将 chat pre-recall 结果注入现有 TurnContext
- [x] 2.5 调整 docker 调试配置，使 memorySdk 使用专用 DSN 配置

## 3. Validation
- [x] 3.1 增加 unit tests，覆盖 disabled/enabled/degraded 路径
- [x] 3.2 运行相关 Go 测试
- [x] 3.3 重新校验 OpenSpec 变更

## 4. Phase 2 Chat Memory Tools
- [x] 4.1 补齐 spec deltas，明确 memory tools 只在 chat JSON surface 可见，且 XML fallback 不开放
- [x] 4.2 为 chat tool loop 增加稳定 invocation meta（run_id / turn_id / tool_call_id / protocol）
- [x] 4.3 在 `formalmemory` 中接入 external bridge tool execution helper，避免宿主复制 remember / forget 规则
- [x] 4.4 在 chat 中按条件挂载 chat-local `memory.recall / memory.remember / memory.forget`
- [x] 4.5 确保 memory tools 不进入全局 registry、`/tools`、secretary 默认工具集、subagent 继承工具集
- [x] 4.6 增加 prompt manuals 与 tool alias 支撑

## 5. Phase 2 Runtime Observability
- [x] 5.1 增加 `MEMORYSDK_ENABLE_TOOLS` 与 `MEMORYSDK_ENABLE_TURN_END_JOBS` 配置开关
- [x] 5.2 在 runtime health 中暴露 formal memory / pre-recall / tools / turn-end jobs 状态
- [x] 5.3 在 doctor 输出中展示 formal memory 状态摘要且不泄露敏感 DSN

## 6. Phase 2 Validation
- [x] 6.1 增加 unit tests，覆盖 memory tools gating、invocation meta、remember hard-fail、doctor/health 状态
- [x] 6.2 运行 `go test ./internal/formalmemory ./internal/handler ./internal/runtime ./internal/doctor`
- [x] 6.3 运行 `openspec validate integrate-memorysdk-formal-memory --strict --no-interactive`

## 7. Phase 3 Turn-End Jobs
- [x] 7.1 在 `formalmemory` 中新增 turn-end enqueue 薄封装，继续直接复用 external bridge job helper
- [x] 7.2 在 chat 主流程中于 turn durable persisted 后触发 turn-end enqueue，并保持失败只降级
- [x] 7.3 为 turn-end payload 补齐稳定引用，并明确默认不把 host-local 绝对路径写入 external formal memory
- [x] 7.4 增加测试，覆盖 enqueue 成功、开关关闭、enqueue 失败不打断聊天
- [x] 7.5 运行相关 Go 测试并重新执行 `openspec validate integrate-memorysdk-formal-memory --strict --no-interactive`
