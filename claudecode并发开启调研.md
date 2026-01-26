# 调研总结：Claude Code 自动化封装方案

## 1. 研究目标

验证能否将 Claude Code CLI 封装为异步、安全、可多开的自动化 Agent。

## 2. 结论

**✅ 完全可行**。通过竞品调研和 PoC 验证，我们确认了技术方案的可行性，并发现了区别于现有方案的差异化价值点。

---

## 3. 交付成果

| 序号 | 成果 | 路径 | 描述 |
| :---: | :--- | :--- | :--- |
| 1 | 调研报告 | [research_claude_code.md](./research_claude_code.md) | 架构设计、安全策略、风险评估 |
| 2 | Go Wrapper PoC | [poc_wrapper.go](file:///Users/liu_y/code/goProject/oneAgent/experiments/claude-wrapper/poc_wrapper.go) | 基于 `netflix/go-expect` 的 PTY 封装 |
| 3 | Python Wrapper PoC | [poc_wrapper.py](file:///Users/liu_y/code/goProject/oneAgent/experiments/claude-wrapper/poc_wrapper.py) | 基于 `pexpect` 的原型验证 |
| 4 | 并发策略 | [concurrency_strategy.md](./concurrency_strategy.md) | Git Worktree 隔离方案 |
| 5 | 竞品分析 | [competitive_analysis.md](./competitive_analysis.md) | myclaude / WebCode 对比 |

---

## 4. 核心技术验证

### 4.1 PTY 拦截

成功验证了通过 Go/Python 的 PTY 库包裹 [claude](file:///Users/liu_y/code/goProject/oneAgent/experiments/claude-wrapper/poc_wrapper.py#24-124) CLI 进程，拦截 `[y/N]` 提示的能力。

```text
[Go-Wrapper] Starting 'claude'...
[Go-Wrapper] Trust prompt detected. Accepting...
[Go-Wrapper] 🛡️  INTERCEPTION: Permission requested.
[Go-Observer] ✅ APPROVED: Safe operation.
```

### 4.2 并发隔离

设计了基于 Git Worktree 的多 Agent 隔离策略，每个 Agent 拥有独立的工作目录和分支，互不干扰。

### 4.3 竞品对比

| 能力 | cexll/myclaude | xuzeyu91/WebCode | **我们** |
| :--- | :---: | :---: | :---: |
| Observer LLM 意图审查 | ❌ | ❌ | **✅** |
| 自动 Worktree 管理 | ❌ | ⚠️ | **✅** |

---

## 5. 我们的差异化优势

> **核心卖点：Observer LLM + Git Worktree 自动化**

现有竞品均采用"YOLO 模式"（跳过所有审批）或简单规则白名单，存在安全风险。我们的方案：
1.  **保留原生审批机制**：不使用 `--dangerously-skip-permissions`。
2.  **引入 Observer LLM**：对每次操作进行意图审查，结合用户原始 Goal 判断是否允许。
3.  **自动化 Worktree 生命周期**：Agent 启动时自动创建，结束时自动清理。

---

## 6. 下一步建议

基于调研结论，建议进入 **工程化开发阶段**：

| 阶段 | 任务 | 优先级 |
| :--- | :--- | :---: |
| **Phase 1** | 完善 Go Wrapper，支持 `stream-json` 输出解析 | 🔴 高 |
| **Phase 1** | 实现 Worktree Manager，自动创建/销毁 Agent 工作区 | 🔴 高 |
| **Phase 1** | 接入真实 Observer LLM API (可复用 OneAgent 现有后端) | 🔴 高 |
| **Phase 2** | 实现任务队列与并行调度 | 🟡 中 |
| **Phase 2** | 开发 Web Dashboard (可选) | 🟢 低 |

---

## 7. 技术选型确认

| 维度 | 选择 | 理由 |
| :--- | :--- | :--- |
| 核心语言 | **Go** | 单二进制部署、并发性能好、竞品验证可行 |
| PTY 库 | `netflix/go-expect` + `creack/pty` | Netflix 生产级验证 |
| 输出解析 | **stream-json** 优先 | 比 PTY 正则更稳定，竞品都在用 |
| 并发隔离 | **Git Worktree** | 轻量、原生 Git 支持、共享历史 |
| Observer | **复用 OneAgent LLM API** | 无需额外部署 |

---

*调研完成于 2026-01-26*
