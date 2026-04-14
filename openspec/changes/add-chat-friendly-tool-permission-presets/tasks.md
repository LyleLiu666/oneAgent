## 1. Spec And Design
- [x] 1.1 为聊天友好的权限预设补齐 proposal / design / spec deltas
- [x] 1.2 运行 `openspec validate add-chat-friendly-tool-permission-presets --strict --no-interactive`

## 2. Backend
- [x] 2.1 增加简单权限模式定义与 `ToolPolicy <-> simple mode` 映射，并覆盖默认策略/历史常见形态识别
- [x] 2.2 新增读取 / 应用简单权限模式的用户向 API，并限定为“仅当前认证 principal”
- [x] 2.3 将 `command_approval_mode` 纳入简单权限接口返回与更新流程
- [x] 2.4 在 `settingsdb` 增加简单权限切换审计存储，并补齐日志/测试
- [x] 2.5 为 `sandbox_coding` 的环境可用性、ownership 边界与错误提示补齐测试
- [x] 2.6 明确 simple mode 对已运行 attempt 与新 attempt 的生效边界，并补齐测试

## 3. Frontend
- [x] 3.1 在聊天页 header 增加当前执行权限 chip 与简单切换面板
- [x] 3.2 在秘书模式增加权限建议卡片，覆盖“权限不足 / 明显需要写改跑”场景，并明确“仅影响后续执行”
- [x] 3.3 将工具权限页改为“简单预设优先，高级 JSON 折叠展开”
- [x] 3.4 为审批模式切换补齐简单入口与测试
- [x] 3.5 为非 `local` 用户隐藏或降级高级设置入口，避免进入 admin-only 死路

## 4. Validation
- [x] 4.1 运行相关 Go 测试
- [x] 4.2 运行相关前端测试
- [x] 4.3 重新执行 `openspec validate add-chat-friendly-tool-permission-presets --strict --no-interactive`
- [ ] 4.4 人工验证 3 条主流程：聊天页切换、秘书推荐切换、高级 JSON 兜底
