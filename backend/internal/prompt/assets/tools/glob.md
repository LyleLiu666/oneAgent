## glob 工具使用说明（简版）
- 用 glob 模式找文件（比反复 `ls` 更快）；例如 `**/*.go`、`frontend/src/**/*.vue`。
- 入参：`{ pattern }`（必填，建议相对 workspace）。
- 输出：`matches[]`（绝对路径）；结果太多就缩小 pattern 或加目录前缀。
