## 1. Inbound relay
- [x] 1.1 设计统一 ingress schema（channel_id/thread_id/message_id/principal mapping）
- [x] 1.2 实现入站 webhook 适配层（v1 先支持 1 个渠道）
- [x] 1.3 实现 message id 幂等去重，防止重复派工
- [x] 1.4 将入站消息安全写入 secretary inbox，并记录 source metadata

## 2. Outbound relay
- [x] 2.1 定义任务终态通知模板（低噪声 + 可追溯）
- [x] 2.2 实现 outbound sender（支持重试与失败留痕）
- [x] 2.3 将通知与 `task_id/attempt_id/artifacts` 建立引用

## 3. Security and governance
- [x] 3.1 webhook 签名/鉴权校验（fail-closed）
- [x] 3.2 渠道 principal 映射与权限策略对齐
- [x] 3.3 记录审计日志（入站、派工、出站）

## 4. Validation
- [x] 4.1 端到端测试：入站消息 -> secretary triage -> task -> 出站通知
- [x] 4.2 幂等测试：重复 webhook 不重复创建任务
- [x] 4.3 异常测试：签名失败、出站失败重试
- [x] 4.4 运行 `openspec validate add-channel-relay-v1 --strict --no-interactive`
