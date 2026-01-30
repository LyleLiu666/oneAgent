## ls 工具使用说明（简版）
- 列出目录/文件；用于快速了解项目结构（通常比 bash 更稳）。
- 入参：`{ path? }`，默认 `.`；路径需在沙箱根目录/ workspace 内。
- 输出：`entries[]`（`name/path/is_dir`）；配合 `glob/rg` 继续定位目标文件。
