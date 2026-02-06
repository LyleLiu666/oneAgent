## 1. Progress intent path
- [ ] 1.1 为 triage planner 增强进度意图识别（progress/dispatch/clarify）
- [ ] 1.2 增加 deterministic progress response 路径，优先消费 tasks snapshot
- [ ] 1.3 当只有单一相关任务时，禁止无意义反问

## 2. Self-heal path
- [ ] 2.1 对协议解析/参数归一化类错误增加 bounded retry + repair
- [ ] 2.2 将自愈重试事件写入可追溯链路（避免黑盒）
- [ ] 2.3 仅在达到预算上限后升级为用户可见“需要确认”事项

## 3. UX alignment
- [ ] 3.1 秘书模式中统一“失败转达 + 下一步动作”文案结构
- [ ] 3.2 增加恢复回执规范（task_id/attempt_id/source）
- [ ] 3.3 对多事项场景保持聚焦项并避免只报数量

## 4. Validation
- [ ] 4.1 后端测试覆盖：单任务进度问答、多任务聚焦、自愈重试上限
- [ ] 4.2 前端测试覆盖：恢复转达、低噪声状态提示、回执展示
- [ ] 4.3 运行 `openspec validate update-secretary-autonomy-selfheal-v2 --strict --no-interactive`
