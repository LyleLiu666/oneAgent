# Change: Add head-to-head benchmark suite for competitive execution quality

## Why
“超过竞品”必须可量化。当前缺少可复现、可比较、可回归的统一评测基线，导致功能迭代无法证明是“真实进步”还是“主观体感”。

## What Changes
- 新增一套 head-to-head 基准评测框架（任务集 + 评分器 + 报告）
- 统一关键指标：
  - first pass success rate
  - recovery success rate
  - human intervention count
  - evidence completeness
  - cost per successful delivery
- 输出机器可读报告（JSON）和人类可读报告（Markdown）
- 提供 nightly 与手动触发执行方式（默认不阻塞普通 PR）

## Impact
- Affected specs:
  - `project-tooling`
- Affected code/scripts:
  - `scripts/benchmark_*`
  - 测试工件输出目录（例如 `.oneagent/tmp/benchmarks`）
- Affected docs:
  - 基准任务集维护说明与评分解释文档
