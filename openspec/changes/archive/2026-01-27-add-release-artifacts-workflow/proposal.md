# Change: Release artifacts workflow (cross-platform bundles + checksums)

## Why
我们已经有本地 release 脚本（`scripts/release_local.sh`）可以生成跨平台二进制和 Windows zip（含 PortableGit 可选）以及 checksums。

但目前缺少一个团队可复用的“标准发布流程”：
- 没有 GitHub Actions 产物发布/归档（部门分发、回滚、对账困难）
- 需要一个可手动触发/按 tag 触发的 release workflow 来生成可复现产物

## What Changes
- 新增 GitHub Actions workflow：
  - `push tags (v*)`：生成 release artifacts，并（可选）发布 GitHub Release 附件
  - `workflow_dispatch`：手动生成 artifacts（用于预发布验证）
- 产物规范：
  - `dist/release/` 下输出矩阵二进制
  - Windows 生成 zip（可选 bundle PortableGit）+ NOTICE 文件
  - 生成 `checksums_<version>.txt`

## Impact
- Affected specs: tooling-only（新增 `project-tooling` delta）
- Affected code:
  - `.github/workflows/release.yml`
  - 复用现有 `scripts/release_local.sh`

