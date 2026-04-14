## 1. Spec And Design
- [x] 1.1 为网页目录浏览补齐 proposal / design / workspace spec delta
- [x] 1.2 运行 `openspec validate add-web-workspace-browser --strict --no-interactive`

## 2. Backend
- [x] 2.1 增加 workspace 浏览能力判定与允许根目录计算
- [x] 2.2 新增 workspace browse API，并限制到允许根目录内
- [x] 2.3 为 browse API 增加测试，覆盖根目录列表、子目录浏览、越界拒绝、不可访问路径
- [x] 2.4 在 `/api/config` 返回网页目录浏览能力信息

## 3. Frontend
- [x] 3.1 增加统一的 workspace 目录浏览弹窗与 API 封装
- [x] 3.2 将聊天、秘书、任务台、workflow、文档导出接到新弹窗
- [x] 3.3 更新前端测试，覆盖打开弹窗、浏览目录、选择目录、降级提示

## 4. Validation
- [x] 4.1 运行相关 Go 测试
- [x] 4.2 运行相关前端测试与构建
- [x] 4.3 重新执行 `openspec validate add-web-workspace-browser --strict --no-interactive`
- [x] 4.4 使用新代码重建 Docker 并验证目录浏览接口
