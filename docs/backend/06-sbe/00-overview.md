# SBE 模块：纵览

> 代码位置：`backend/internal/sbe`  
> 职责：智能块编辑器（Smart Block Editor），为 `edit/multiedit` 工具提供模糊匹配 + 应用替换能力。

## 阅读导航

- [细节](01-details.md)：核心接口、错误矩阵、辅助函数
- [设计](02-design.md)：9 层瀑布匹配算法、Apply 流程、Anchor Match 伪码

## 代码规模

- ~485 行
- 核心文件：`matcher.go` (314), `actuator.go` (99), `levenshtein.go` (56), `types.go` (16)

