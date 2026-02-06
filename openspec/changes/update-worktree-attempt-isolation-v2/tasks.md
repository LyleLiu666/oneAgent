## 1. Attempt bootstrap and boundaries
- [ ] 1.1 统一 attempt 启动路径，确保 worktree mode 下总是创建独立执行根目录
- [ ] 1.2 在 attempt artifacts 中持久化 `worktree_root`、`base_commit_sha`、`worktree_mode`
- [ ] 1.3 明确 non-git workspace 的 fail-fast 错误（或显式降级路径）

## 2. Lifecycle and cleanup
- [ ] 2.1 实现 worktree 终态清理策略（可配置保留/默认清理）
- [ ] 2.2 实现清理失败重试与可解释错误（含建议手工清理命令）
- [ ] 2.3 增加 orphan worktree sweeper（启动时/定时 best-effort）

## 3. Rollback and evidence
- [ ] 3.1 将 rollback checkpoint 与 worktree 元信息绑定
- [ ] 3.2 保证 rollback 不删除 trace/findings/diff 证据
- [ ] 3.3 在 receipt 中补齐 worktree lifecycle 关键事件

## 4. Validation
- [ ] 4.1 单元测试：git/non-git、路径越界、base SHA 记录
- [ ] 4.2 集成测试：并发 attempts、终态清理、orphan 清理
- [ ] 4.3 跨平台重点测试：Windows 文件占用清理失败场景
- [ ] 4.4 运行 `openspec validate update-worktree-attempt-isolation-v2 --strict --no-interactive`
