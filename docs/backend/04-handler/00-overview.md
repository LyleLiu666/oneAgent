# Handler 模块：纵览

> 代码位置：`backend/internal/handler`  
> 职责：HTTP API 处理器层（Chat SSE、Session、Provider/Model CRUD、OAuth），组织会话读写与流式广播。

## 阅读导航

- [细节](01-details.md)：端点契约、SSE 事件、异常矩阵、持久化格式
- [设计](02-design.md)：StreamChat 全流程、会话压缩、StreamBroadcaster 机制

## 模块能力

- `POST /api/chat`：SSE 流式对话（支持 JSON/XML 工具协议）
- 会话管理：列表/详情/删除/截断
- LLM 配置：Provider/Model CRUD
- OAuth：Keycloak 配置获取与回调交换本地 JWT

## 代码规模

- ~2900 行
- 核心文件：`chat.go` (1698), `llm_provider.go` (432), `session_compress.go` (268)

