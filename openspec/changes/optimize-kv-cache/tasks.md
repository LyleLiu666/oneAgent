# 任务列表 (Tasks)

- [x] 定义“缓存友好 Prompt 构建”规范：Stable Prefix vs Volatile Context，并给出推荐注入位置（为 skills/plan/subagent 提供约束） <!-- id: 1 -->
- [x] 引入可复用的 cache policy 抽象（capability matrix + cache selector），并为各 provider 明确默认策略 <!-- id: 2 -->
- [x] 优化长会话压缩后的缓存命中：让 summary 进入显式缓存集合（或采用等价机制） <!-- id: 3 -->
- [x] 完善 Claude(Anthropic) 工具调用场景的缓存覆盖：支持对 tool_use/tool_result block 注入 cache_control（或可配置） <!-- id: 4 -->
- [x] 统一 `prompt_cache_key` 策略与回退机制：
  - [x] key 计算纳入 model/tool/system_prompt 变化（避免不必要 miss 或潜在碰撞） <!-- id: 5 -->
  - [x] 上游返回“不支持字段/参数”时自动降级并提示 <!-- id: 6 -->
- [x] 增加缓存可观测性：
  - [x] 在 trace/log 中记录 cache enabled、cache key（脱敏/哈希）、cached tokens（若可得）；每次 LLM 调用完整 request/response（含 messages）写入 log 文件，trace 仅保存摘要与指针 <!-- id: 7 -->
  - [x] 前端最小展示（Trace 或 Settings 面板） <!-- id: 8 -->
- [ ] 测试与验证：
  - [x] 单元测试：cache selector 在典型消息序列（含压缩摘要、含工具消息）下输出稳定 <!-- id: 9 -->
  - [x] 集成测试（mock provider）：确保各 provider 的请求 payload 注入符合 capability matrix <!-- id: 10 -->
