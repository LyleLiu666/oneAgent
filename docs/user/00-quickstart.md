# 快速开始（Workspace-first）

目标：**进来就能干活**——选一个文件夹作为 workspace，立刻下指令让 oneAgent 开始改代码/写文档/跑测试。

## 1) 启动（推荐）

```bash
make build
./dist/oneagent serve --open --workspace .
```

- `--open`：自动打开浏览器
- `--workspace .`：设置默认 workspace（文件工具/命令工具的默认作用域与写入边界）

## 2) 登录（本地 token）

```bash
./dist/oneagent doctor
```

在输出里找到 `auth_token` 文件路径（不会输出明文 token），然后在 UI 的登录页粘贴 token。

## 3) 配置模型 / Provider

在 UI 的 `Settings -> LLM Providers`：

1. 新增一个 Provider（OpenAI-compatible/Anthropic 等）
2. 在该 Provider 下添加可用 Models
3. 回到 Chat 顶部选择模型开始使用

## 4) 两种“干活”方式（建议先记住这个分叉）

### A. 只对话（不设 workspace）

适合：头脑风暴、写方案、非本机文件任务。

### B. 工具化执行（必须设 workspace）

适合：改代码、搜 repo、跑命令、生成交付物文件。

要点：workspace 不是“建议”，是**边界**。不设 workspace 时，文件相关工具会直接提示 `workspace is not set`。

## 5) 长任务（后台队列）

当你希望 oneAgent “去干一小时，然后把结果和证据交付给我”，用 Chat 顶部的 `Tasks` 面板创建任务。

详见：`docs/user/01-task-queue.md`

