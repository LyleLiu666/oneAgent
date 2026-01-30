# OpenSpec Changes: Priority & Progress

更新时间：2026-01-30

本文件只记录**当前 active changes** 的优先级与进度快照；历史内容不再维护。
已完成变更请看 `openspec/changes/archive/`，真实进度以 `openspec list` 为准。

## Priority（高 → 低）

### P0（先测后改 / 高优先级评估）
- `add-toolcalling-metrics-regression`（2/6）：工具调用指标体系 + 脚本化回归（把成功率变成可回归数字）
- `update-toolcalling-default-temperature`（2/5）：tools 启用时默认 temperature=0（提升结构化输出稳定性）
- `update-toolcalling-arguments-normalization`（2/5）：增强 tool arguments JSON 修复（围栏/夹杂文本/quoted JSON）
- `update-tool-names-openai-compatible`（2/7）：工具 function name 去点号（OpenAI-compatible）+ 保留 alias 兼容
- `add-toolcalling-reliability-specs`（5/5）：整理工具调用专家建议为 OpenSpec，并增加 XML vs JSON 长文本专项回归测试（先测后改）

### P1（Prompt 基建 / 降低踩坑）
- `add-core-tool-manuals`（2/5）：补齐核心工具手册（assets/tools），并用 prompt tests 防回归
- `add-command-profile-coding`（2/6）：新增 `coding` 命令 profile（docker-only），让 coding agent 常用链路跑得起来
- `update-tool-output-envelope`（2/6）：工具输出/错误统一 JSON envelope（可解析 + 可行动），降低自愈回合数

### P2（对齐 Codex / 中长期）
- `update-openai-responses-toolcalling`（2/7）：补齐 OpenAI Responses provider 的 tools loop（先 MVP 非流式，流式 best-effort）

## Snapshot（来自 `openspec list`）
- `add-toolcalling-metrics-regression`：2/6 tasks
- `update-toolcalling-default-temperature`：2/5 tasks
- `update-toolcalling-arguments-normalization`：2/5 tasks
- `add-toolcalling-reliability-specs`：5/5 tasks
- `add-core-tool-manuals`：2/5 tasks
- `add-command-profile-coding`：2/6 tasks
- `update-tool-names-openai-compatible`：2/7 tasks
- `update-tool-output-envelope`：2/6 tasks
- `update-openai-responses-toolcalling`：2/7 tasks
