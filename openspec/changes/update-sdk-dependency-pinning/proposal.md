# Change: Pin External SDK Dependencies By Default

## Why
`oneAgent` 当前虽然在 `backend/go.mod` 中声明了 `agentsdk` 和 `memorySdk` 的版本，但仓库内提交的 `replace` 与 Docker 的本地 build contexts 会把实际解析结果切回开发机上的外部目录。

这会带来两个直接问题：

1. 主仓库的默认构建结果会被外部 SDK 工作目录的最新代码直接影响，稳定性不可预期。
2. 用户日常开发与 review 时，很难判断当前问题来自 `oneAgent` 还是来自本地 SDK 临时变更。

用户的目标已经很明确：主仓库默认应该依赖稳定快照；只有在明确联调 SDK 时，才显式切换到本地源码模式。

本次尝试还发现一个现实约束：`memorySdk` 当前并不是一个可以直接按稳定 Go module 版本被 clean consumer 消费的完整发布物。因此主仓库默认模式需要一个比“远程 go get”更稳的落地方式。

## What Changes
- 将两个 SDK 以固定 commit 的 git submodule 形式钉在仓库内，作为默认稳定快照
- 将默认 Go / Docker 构建切回仓库内固定快照，而不是开发机外部目录
- 提供一个显式的 `sdk-local` Docker override，用于本地联调 `agentsdk` / `memorySdk`
- 文档化本地 `go.work` 联调方式，避免把本地 override 再次提交进仓库

## Impact
- Affected specs:
  - `project-tooling`
- Affected code:
  - `/Users/liu_y/code/goProject/oneAgent/backend/go.mod`
  - `/Users/liu_y/code/goProject/oneAgent/Dockerfile`
  - `/Users/liu_y/code/goProject/oneAgent/docker-compose.yml`
  - `/Users/liu_y/code/goProject/oneAgent/docker-compose.sdk-local.yml`
  - `/Users/liu_y/code/goProject/oneAgent/README.md`
  - `/Users/liu_y/code/goProject/oneAgent/Makefile`
