# Change: Add chat-friendly tool permission presets

## Why
当前工具权限虽然已经有完整的策略能力，但用户必须进入“工具权限”页面直接编辑 JSON，才能切换“只读 / 沙箱开发 / 本机执行”这类常见执行模式。

这带来 3 个现实问题：

1. 用户很难理解“sandbox / host / full / coding”这些内部术语，更难把它们映射成“我只是想让 agent 能跑测试”这种真实意图。
2. 现在的入口主要在治理页和 hash 链接上，不符合项目“默认简单、渐进展开、秘书优先”的方向。
3. 用户在秘书模式下遇到权限阻塞时，会被迫跳出“像微信一样说事”的心智，转去理解后台治理系统。

这次需要把“权限调整”从 JSON 治理动作，降成用户可理解、可点击、可审计的预设操作，同时保留高级 JSON 作为兜底能力。

## What Changes
- 为工具权限增加用户可理解的简单预设：`只读查看`、`沙箱开发`、`本机执行`
- 后端新增“读取当前简单权限状态 / 应用预设”的用户向 API，但该接口仅作用于当前认证 principal；跨 principal 管理仍保留在现有 admin 接口中
- 在聊天页与秘书模式中增加低噪声的权限切换入口，不再要求用户先进入 JSON 编辑器
- 当请求因权限不足被阻塞，或明显需要更高执行权限时，系统给出推荐预设与风险说明，并明确说明切换只影响后续执行
- 明确秘书 SU/SW 仍保持只读；秘书模式中的权限切换只影响 worker / 完整聊天 / 后续 task attempt
- 为当前默认策略与历史常见策略增加 simple mode 识别与迁移规则，避免大量用户首次进入就看到 `自定义`
- 保留高级 JSON 编辑器，但收敛到“高级 / 自定义策略”入口中
- 为预设切换补齐持久化审计证据，记录来源、旧配置、新配置、principal 与时间

## Impact
- Affected specs:
  - `system-tool-permissions`
  - `chat-ux`
- Affected code:
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/permissions/**`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/handler/**`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/settingsdb/**`
  - `/Users/liu_y/code/goProject/oneAgent/backend/internal/server/server.go`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/api/client.ts`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/views/ToolPermissions.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/components/ChatBox.vue`
  - `/Users/liu_y/code/goProject/oneAgent/frontend/src/components/SecretaryChatBox.vue`
