# LLM 模块：纵览

> 代码位置：`backend/internal/llm`  
> 职责：LLM 客户端抽象层，支持 OpenAI、Anthropic 及兼容 API；提供流式响应、工具调用、Prompt Cache 等能力。

## 阅读导航

- [细节](01-details.md)：接口、数据结构、Provider 列表、错误矩阵、请求格式、辅助函数
- [设计](02-design.md)：核心流程与时序图、缓存风格、工具调用流式累积

## 关键能力

- Client 抽象：`ChatCompletion` / `ChatCompletionStream`
- Provider 工厂：`NewClientForProvider(cfg ProviderConfig)`
- Tool calling：OpenAI function calling / Anthropic tools 形态
- Prompt 缓存：支持 provider 特定的 `cache_control` / `cachePoint` / header 注入策略
- Trace：请求开始、首 token、逐 token、完成回调

## 代码规模

- ~1920 行
- 核心文件：`llm.go` (756), `anthropic.go` (667), `openai_responses.go` (260), `factory.go` (106)

