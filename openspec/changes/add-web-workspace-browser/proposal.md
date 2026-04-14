# Change: Add a web workspace directory browser for Docker and remote-like local deployments

## Why
当前 workspace 体验在 macOS / Windows 本机模式下可以调用原生目录选择器，但在 Linux / Docker 场景里只能退回手动输入。

这会带来两个实际问题：

1. 用户明明是在容器里运行 oneAgent，却完全不能点选容器内可见目录，体验上像是“坏了”。
2. 手动输入虽然能兜底，但对首次使用者不友好，也不符合项目“默认简单、像客户端一样可直接开工”的方向。

因此这次需要补一个网页侧目录浏览器，让 Docker / Linux 场景也能选择 **服务端真实可见** 的目录，同时保留现有手动输入与原生 picker。

## What Changes
- 为 workspace 增加网页目录浏览能力，供 Docker / Linux / 非同机场景使用
- 后端新增受限目录浏览 API，只暴露明确允许的根目录及其子目录
- 前端新增轻量目录选择弹窗，支持进入子目录、返回上级、选择当前目录
- 运行时配置补充目录浏览能力标记与可用提示文案
- 保留现有手动输入能力，以及 macOS / Windows 的原生目录选择器

## Impact
- Affected specs:
  - `workspace`
- Affected code:
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/handler/workspace.go`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/handler/config.go`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/server/server.go`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/api/client.ts`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/lib/workspaceChooser.ts`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/components/ChatBox.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/components/SecretaryChatBox.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/views/TaskWorkbench.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/views/Workflows.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/views/DocumentExport.vue`
