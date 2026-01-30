## document.export 工具使用说明（简版）
- 将 workspace 内 Markdown 导出为 Office（`docx`/`pptx`，依赖 pandoc）。
- 入参：`{ input_path(必填), format(必填: docx|pptx), output_path?, template_path? }`（路径必须在 workspace 内）。
- 输出：`ok` + `output_path` + `pandoc_command/stdout/stderr`；失败时先检查 workspace 路径与 pandoc 依赖。
