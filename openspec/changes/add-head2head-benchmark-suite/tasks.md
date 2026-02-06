## 1. Benchmark dataset
- [x] 1.1 设计并落盘首批基准任务集（目标 100 条，可先以 20 条 MVP 起步）
- [x] 1.2 为每条任务定义：输入、预期交付类型、客观验收规则、证据要求
- [x] 1.3 对任务集做分层标签（coding/docs/research/ops；short/long；single/multi-step）

## 2. Benchmark runner
- [x] 2.1 实现 benchmark runner（固定配置、可重复执行、可中断恢复）
- [x] 2.2 实现统一指标收集器（success/recovery/intervention/evidence/cost）
- [x] 2.3 实现单次运行 JSON 报告落盘与汇总 Markdown 报告
- [x] 2.4 支持与历史基线对比，输出 delta（+/-）

## 3. CI / nightly
- [x] 3.1 增加手动触发 benchmark workflow
- [x] 3.2 增加 nightly 执行（可独立于主 CI）
- [x] 3.3 失败阈值策略：关键指标回退超过阈值时告警（best-effort）

## 4. Validation
- [x] 4.1 运行 `openspec validate add-head2head-benchmark-suite --strict --no-interactive`
- [x] 4.2 在本地跑一次最小基准（>=3 条）并产出 JSON + Markdown 报告
- [x] 4.3 验证指标定义与报告字段一致且可回放
