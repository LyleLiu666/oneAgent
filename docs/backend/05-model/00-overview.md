# Model 模块：纵览

> 代码位置：`backend/internal/model`  
> 职责：数据模型定义（用户、会话、消息、LLM 配置、用户设置等），并提供常用查询/写入辅助方法。

## 阅读导航

- [细节](01-details.md)：模型字段定义、异常矩阵、SQL 表结构、辅助函数
- [设计](02-design.md)：实体关系图、消息层级关系

## 模块能力

- GORM Model：`User` / `ChatSession` / `ChatMessage` / `LLMProvider` / `LLMModel` / `UserSettings`
- 关键关系：User → Session → Message；Provider → Model
- Settings：用于保存 bocha API key、UI 配置等（按 user_id + key 唯一）

