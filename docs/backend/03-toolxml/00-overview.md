# ToolXML 模块：纵览

> 代码位置：`backend/internal/toolxml`  
> 职责：XML 形式的工具调用协议解析与执行引擎，支持流式处理、thinking 过滤、多轮 RunLoop。

## 阅读导航

- [细节](01-details.md)：RunLoop 入参、XML 协议、字段归一化、错误矩阵、StepRecord 结构
- [设计](02-design.md)：RunLoop 执行图、StreamFilter 状态机、Parser 流程

## 模块能力

- XML 工具协议：`<tool_data>` / `<tool_result>`
- 流式过滤：抑制 `<tool_data>` 与 `<thinking>` 输出到用户
- 字段别名归一化：兼容多种字段命名（snake/camel/旧字段）
- 多轮循环：最多 20 步（可配置常量）

## 代码规模

- ~1590 行
- 核心文件：`engine.go` (537), `parser.go` (271), `stream_filter.go` (177), `prompt.go` (110)

