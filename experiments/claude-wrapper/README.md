# Claude Code Wrapper & Observer PoC

这是 `Safe-Claude-Wrapper` 的概念验证实现了 (Proof of Concept)。
本实验旨在证明：可以通过 PTY 包裹 `claude` CLI，并由 Observer (LLM) 自动拦截和审批其操作，从而实现安全的多实例并发运行。

## 目录结构

```
experiments/claude-wrapper/
├── poc_wrapper.py    # 核心 Wrapper 脚本 (Python + pexpect)
└── README.md         # 本文档
```

## 功能特性

1.  **进程托管**：使用 `pexpect` 启动并托管 `claude` 伪终端即交互。
2.  **状态感知**：自动识别 "Trust this folder"、"Auth Required"、"Permission [y/N]" 等状态。
3.  **安全拦截**：当检测到 `(y/n)` 或 `[y/N]` 等敏感操作确认请求时，暂停并调用 Observer。
4.  **Mock Observer**：当前实现了一个基于关键词的 `Observer` 类：
    - 自动拦截 `rm -rf` 等高危命令。
    - 自动批准 `create` / `write` 等文件操作。
    - 其他操作记录日志并试图批准 (可扩展接入真实 DeepSeek/GPT-4 模型)。

## 快速开始

### 1. 依赖安装

```bash
pip install pexpect
```

### 2. 运行 PoC

```bash
# 默认 Prompt: "echo 'Hello Wrapper'"
python3 experiments/claude-wrapper/poc_wrapper.py

# 自定义指令
python3 experiments/claude-wrapper/poc_wrapper.py "Create a file named safe_test.txt"
```

### 3. 注意事项

- **首次运行**：Claude Code 首次运行通常需要浏览器登录认证。如果 `poc_wrapper.py` 提示 `🚨 Auth Required`，请先在终端直接运行一次 `claude` 完成登录。
- **YOLO 模式**：本 Wrapper **不** 使用 `--dangerously-skip-permissions`，而是利用默认的安全询问机制作为拦截点，这更加安全。
- **并发多开**：通过运行多个 `poc_wrapper.py` 进程即可实现多 Agent 并发，Wrapper 负责处理各自的 I/O 阻塞。

## 下一步计划 (Roadmap)

1.  **接入真实 LLM**：将 `Observer.evaluate()` 这里的逻辑替换为调用 API (如 user 使用的现有 LLM)，传入 `tool_output` 上下文进行语义判断。
2.  **Web 界面**：开发一个简单的 WebUI 来管理这堆 Wrapper 进程，查看它们的实时日志和干预决策。
3.  **持久化配置**：支持 `config.yaml` 定义白名单/黑名单策略。
