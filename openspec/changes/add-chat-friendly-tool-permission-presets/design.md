## Context
oneAgent 当前已经有成熟的工具权限底座：
- `ToolPolicy` 支持 `command_profile`、`sandbox_mode`、`approval` 等约束
- 命令执行已经区分 `readonly / dev / coding / full`
- 还存在单独的 `command_approval_mode=auto|manual`

问题不在“底层做不到”，而在“用户根本不该直接操作这些底层概念”。

这与项目的 3 条核心约束有直接冲突：
- `openspec/project.md` 要求默认简单、渐进展开
- `docs/ux-vision.md` 强调工具要服务用户，而不是让用户适应工具
- `openspec/roadmap-secretary-first.md` 明确要求秘书模式默认隐藏治理系统细节

因此，这次不应该重写权限引擎，而应该在现有 policy 之上补一层“简单且安全的用户交互壳”。

## Goals
- 让用户在聊天里就能完成 80% 的权限切换需求
- 保留现有 policy/approval/sandbox 作为真实执行与审计基础
- 让秘书模式在权限阻塞时仍保持低心智、可继续推进
- 保持现有 principal ownership 边界不被放松
- 不允许 silent escalation；所有提权都必须是明确用户操作

## Non-Goals
- 不删除高级 JSON 编辑器
- 不让 LLM 自由生成任意 policy JSON 并直接生效
- 不改变当前命令安全模型与审批模型的底层语义
- 不把“是否切到秘书模式”本身当成权限问题的解决方案

## Options Considered

### Option A: 聊天内权限预设卡片 + 高级 JSON 兜底
推荐方案。

做法：
- 用户看到的是 3 个可理解的预设，而不是 profile/sandbox 术语
- 点击后由后端把预设映射成真实 `ToolPolicy`
- 高级需求仍然可以进入 JSON 编辑器

优点：
- 最符合秘书优先与 progressive disclosure
- 不破坏现有底层权限实现
- 可以自然复用到聊天页、秘书模式、工具权限页

缺点：
- 需要新增“预设 <-> policy”的双向识别与审计

### Option B: 只优化“工具权限”页面，改成表单而不是 JSON
优点是实现相对集中；缺点是仍然要求用户跳出聊天心智，不符合“秘书优先”。

### Option C: 允许用户用自然语言直接说“给我 full access”
最省输入，但风险太高：含糊、不可预测、不可审计，也容易把权限调整做成 LLM 幻觉问题。

## Recommended Approach
采用 Option A，并分两层实现：

### Layer 1: 简单预设层
对用户只暴露 3 个模式：

1. `只读查看`
   - 语义：可读、可查、不可改
   - 对应：`command_profile=readonly`

2. `沙箱开发`
   - 语义：允许改代码/跑测试，但尽量放在隔离环境里
   - 对应：`command_profile=coding`
   - `sandbox_mode` 不直接暴露给用户；后端按环境选择 `native` 或 `docker`
   - 若硬边界不可用，返回可操作错误，而不是悄悄降级到 host

3. `本机执行`
   - 语义：直接在当前机器执行，能力最强，风险最高
   - 对应：`command_profile=full` + `sandbox_mode=host`

同时暴露一个独立但简单的开关：
- `高风险命令审批：自动 / 手动`

### Layer 2: 高级自定义层
保留现有 JSON 编辑器，但改成“高级 / 自定义策略”入口：
- 普通用户默认看不到 JSON
- 当前 policy 无法被安全识别为预设时，显示为 `自定义`
- 用户仍可查看当前 principal、policy hash、JSON 内容并手工修改

## Ownership Boundary
simple mode 是“当前用户的自助权限切换”，不是新的全局治理后门。

因此必须固定边界：
- simple API 只允许读取和修改**当前认证 principal** 的权限状态
- 客户端不得通过 simple API 指定其它 `principal_id`
- 现有 `/api/admin/tool_policies/:principal_id` 继续承担管理员跨 principal 治理能力
- 若当前用户不是 `local`，聊天内的“高级设置”不得直接把用户送到一个必然 403 的页面；应隐藏入口或给出明确说明

这样可以避免两类问题：
- 普通用户通过“简单入口”意外拥有管理员能力
- 前端做了入口，但点击后只是报错，体验更差

## Secretary Boundary
秘书模式里的权限切换，必须和“秘书本身默认只读”这条红线分开。

本次 simple mode 的作用域必须明确为：
- 影响完整聊天（worker chat）的后续回合
- 影响任务队列中新建或 resume 出来的后续 attempt
- 影响普通工具调用的后续 policy snapshot 解析

本次 simple mode **不得** 改变：
- Secretary SU/SW 自身的只读策略
- 当前已经在运行中的 task attempt 的 `policy_snapshot`

也就是说，秘书模式里出现的“切到沙箱开发”推荐卡片，语义应该是：
- “允许系统后续派给 worker 的执行更有能力”
- 而不是“秘书自己马上获得写权限”

这点需要在 UI 文案和 API 响应里都写清楚，避免用户产生错误预期。

## Policy Classification And Migration
simple mode 识别不能只靠“policy JSON 完全相等”，要基于**有效执行语义**做 best-effort 分类。

至少要覆盖这些内置/历史形态：

1. `readonly`
   - 显式 `command_profile=readonly`
   - 或当前 policy 在当前平台上的有效命令执行语义已经退化为只读

2. `sandbox_coding`
   - 显式 `command_profile=coding` 且要求 `sandbox_mode=native|docker`
   - 或内置默认策略在当前环境下会稳定解析为硬边界 `coding`

3. `host_full`
   - 显式 `command_profile=full` + `sandbox_mode=host`

4. `custom`
   - 以上规则都无法安全匹配时才使用

特别说明：
- 当前 `DefaultPolicy()` 在不同平台的默认形态不同
- 命令工具的 `sandbox_mode` 还存在“未写在 policy 中、由执行层隐式推导”的情况

因此 simple mode 识别必须调用统一的后端分类逻辑，而不是由前端通过 JSON 猜。

## UX Flow

### 1. 全量聊天页（full mode）
- 现有 header 里已经显示 `policy hash + 工具权限` 链接，但太隐蔽，也不表达语义
- 改成显示一个可点击的权限状态 chip：
  - `执行权限：只读查看`
  - `执行权限：沙箱开发`
  - `执行权限：本机执行`
  - `执行权限：自定义`
- 点击后打开轻量浮层或底部面板，展示：
  - 当前模式
  - 3 个预设按钮
  - 风险说明
  - 审批模式切换
  - “高级设置”链接（仅在当前用户可访问时展示）

### 2. 秘书模式（secretary mode）
秘书模式不应该要求用户进入治理页。

当出现以下情况时，展示低噪声权限卡片：
- 用户请求明显需要写/改/跑
- 或系统返回 `tool_permission_denied`
- 或秘书已经判断“只靠只读工具闭环不了”

卡片文案示例：
- “这件事需要更高执行权限。我建议用‘沙箱开发’，更稳一些。切换后会影响后续执行，不会让秘书直接改文件。”
- 操作按钮：
  - `切到沙箱开发（推荐）`
  - `改为本机执行`
  - `继续只读`
  - `高级设置`

秘书模式下：
- 不自动提权
- 不把这类系统动作写成普通 assistant 文本消息
- 可以在面板里展示一个轻量“已切换为沙箱开发”的状态提示或 toast
- 必须提示“当前运行中的任务不会被立刻改权限；新任务或后续重试会使用新配置”

## API Design

保留现有 `/api/admin/tool_policies/:principal_id` 作为高级管理员接口。

新增简单接口：

- `GET /api/tool_permissions/simple`
  - 输入：无 `principal_id` 参数；后端从当前认证上下文解析 principal
  - 输出：
    - `principal_id`
    - `current_mode`: `readonly|sandbox_coding|host_full|custom`
    - `command_approval_mode`: `auto|manual`
    - `available_modes`: 每个模式的标签、描述、风险级别、是否当前环境可用
    - `effective_scope`: 明确说明“影响后续 worker / 新 attempt，不影响秘书自身只读和已运行 attempt”
    - `snapshot`: 便于审计/显示 hash

- `PUT /api/tool_permissions/simple`
  - 输入：
    - `mode`
    - `command_approval_mode`（可选）
    - `source`：`chat_header|secretary_prompt|tool_permissions_page`
  - 行为：
    - 后端将 mode 映射成真实 `ToolPolicy`
    - 后端只修改当前认证 principal 的策略
    - 持久化 policy
    - 必要时更新 `command_approval_mode`
    - 返回新的简单状态、`effective_scope` 与 snapshot

## Audit And Safety
- 每次预设切换都必须记录：
  - `principal_id`
  - `source`
  - `old_policy_hash`
  - `new_policy_hash`
  - `selected_mode`
  - `command_approval_mode`
  - `changed_at`
- 审计记录必须持久化到 `settingsdb`（新增简单权限切换审计表），并同时输出结构化日志
- `本机执行` 必须有更明显的风险样式与二次确认（best-effort）
- `沙箱开发` 若环境不可用，必须返回可操作错误：
  - “当前机器没有可用的隔离执行环境；可改用本机执行，或先安装 / 启用 Docker”

## Failure Semantics
- simple API 若收到外部 `principal_id`：
  - `hard-fail`，返回明确错误；或直接忽略该字段且不暴露跨 principal 行为
- `sandbox_coding` 不可用：
  - `hard-fail`，不保存策略，不静默降级
- 当前策略无法识别：
  - `degrade` 为 `custom`
- 当前正在运行的 attempt：
  - `degrade`，保持原 `policy_snapshot` 不变；新策略仅影响后续执行

## Rollout Plan
分 3 步落地：

1. Backend first
   - 完成 simple mode 分类、simple API、审计表、测试
   - 先不改 UI，仅供开发验证

2. Full chat UI
   - 在完整聊天页替换现有 `policy hash + 工具权限` 链接
   - 验证 simple mode 切换、审批模式切换、`custom` 展示

3. Secretary UI
   - 接入低噪声权限建议卡片
   - 明确“秘书仍只读 / 仅影响后续执行”

回滚方式：
- 前端隐藏 simple mode chooser 即可停止用户入口
- 后端保留 admin JSON 接口作为稳定兜底
- simple API 即使回滚，也不影响既有 policy 执行链路

## Testing Strategy
- 后端单测：
  - 预设到 policy 的映射
  - policy 到简单模式的识别
  - 默认 policy 与历史常见形态到 simple mode 的识别
  - sandbox preset 在不同环境能力下的可用性判断
  - 应用预设后的审计记录
  - simple API 只能修改当前 principal
  - 切换后不影响已运行 attempt 的 `policy_snapshot`
- 前端组件测试：
  - 聊天页权限 chip 展示与切换
  - 秘书模式下的推荐卡片
  - 高级设置入口仍可达
- E2E：
  - 从秘书模式收到权限建议
  - 一键切到 `沙箱开发`
  - 继续执行同类任务不再被 JSON 配置阻塞
