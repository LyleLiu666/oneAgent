# Tool 模块：纵览

> 代码位置：`backend/internal/tool`  
> 职责：可插拔工具系统，供 LLM 调用（bash/edit/search/...），并负责参数解析、路径安全与执行约束。

## 阅读导航

- [细节](01-details.md)：注册表接口、各工具请求/响应规格、错误矩阵、常量
- [设计](02-design.md)：工具调用流程、路径安全验证、UserID 注入机制

## 模块能力

- 工具注册表：`All()` / `Mount(ids)` / `ToolsForLLM(defs)`
- 工具实现：`bash` / `run_command` / `edit` / `write_file` / `glob` / `ls` / `rg` / `search`
- 安全约束：路径限制在 `$BASH_ROOT_DIR`、禁用部分 shell 特性、输出截断等

## 代码规模

- ~2170 行
- 核心文件：`registry.go` (145), `smart_edit.go` (233), `run_command.go` (201), `write_file.go` (219), `glob.go` (247)
