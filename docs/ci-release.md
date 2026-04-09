# CI 发布说明

本文档说明 `edge-pilot` 当前的 GitHub Actions 发布流程、触发方式、产物类型和日常发布步骤。

## 流程入口

当前发布流水线定义在：

- [release.yml](/Users/laopixu/GolandProjects/edge-pilot/.github/workflows/release.yml)
- [.goreleaser.yaml](/Users/laopixu/GolandProjects/edge-pilot/.goreleaser.yaml)

触发条件：

- 向仓库推送符合 `v*` 规则的 Git tag

示例：

- `v0.1.0`
- `v1.0.0`

## GitHub Actions 流程

当前 `release` workflow 运行在 `ubuntu-latest`，主要步骤如下：

1. `actions/checkout@v4`
   - 拉取完整 git 历史，供 GoReleaser 生成 changelog 和版本信息
2. `actions/setup-go@v5`
   - 使用 `go.mod` 中声明的 Go 版本
3. `docker/setup-qemu-action@v3`
   - 启用多架构镜像构建
4. `docker/setup-buildx-action@v3`
   - 启用 Buildx
5. `docker/login-action@v3`
   - 登录 GHCR
6. `goreleaser/goreleaser-action@v6`
   - 执行 `goreleaser release --clean`

GitHub Actions 权限：

- `contents: write`
- `packages: write`

## GoReleaser 行为

GoReleaser 当前会做三件事：

1. 发布前执行测试
   - `go test ./...`
2. 构建双二进制归档
   - `edge-pilot-control`
   - `edge-pilot-agent`
3. 推送双镜像到 GHCR
   - `edge-pilot-control`
   - `edge-pilot-agent`

## 二进制产物

当前二进制构建矩阵：

- `linux/amd64`
- `linux/arm64`

构建目标：

- `./cmd/control-plane`
- `./cmd/agent`

生成的归档命名规则：

- `edge-pilot_control-plane_<version>_linux_amd64.tar.gz`
- `edge-pilot_control-plane_<version>_linux_arm64.tar.gz`
- `edge-pilot_agent_<version>_linux_amd64.tar.gz`
- `edge-pilot_agent_<version>_linux_arm64.tar.gz`

附加产物：

- `checksums.txt`

## Build Info 注入

发布构建时会通过 `ldflags` 注入以下 build info：

- `Version`
- `Commit`
- `BuildTime`

注入目标在：

- [buildinfo.go](/Users/laopixu/GolandProjects/edge-pilot/internal/shared/buildinfo/buildinfo.go)

启动时，`control-plane` 和 `agent` 都会打印这三项信息。

## 镜像产物

当前 GHCR 镜像名：

- `ghcr.io/<owner>/edge-pilot-control`
- `ghcr.io/<owner>/edge-pilot-agent`

当前镜像 tag：

- `<git tag version>`
- `latest`

示例：

- `ghcr.io/laopixu/edge-pilot-control:v0.1.0`
- `ghcr.io/laopixu/edge-pilot-control:latest`
- `ghcr.io/laopixu/edge-pilot-agent:v0.1.0`
- `ghcr.io/laopixu/edge-pilot-agent:latest`

当前运行镜像基础镜像：

- `debian:bookworm-slim`

Dockerfile：

- [Dockerfile.control-plane](/Users/laopixu/GolandProjects/edge-pilot/Dockerfile.control-plane)
- [Dockerfile.agent](/Users/laopixu/GolandProjects/edge-pilot/Dockerfile.agent)

## 发布前准备

在正式打 tag 前，建议至少确认以下事项：

- 当前 `main` 已通过本地或 CI 测试
- `go test ./...` 通过
- `README`、`AGENTS.md`、相关部署文档已同步
- `CI_SHARED_TOKEN`、`AGENT_SHARED_TOKEN` 或 `AGENT_TOKENS` 的部署侧配置已就绪
- control-plane 与 agent 使用的镜像引用策略已明确

## 推荐发布步骤

推荐操作顺序：

1. 确认目标提交已经合并到发布分支
2. 本地执行：

```bash
go test ./...
```

3. 创建并推送 tag：

```bash
git tag v0.1.0
git push origin v0.1.0
```

4. 等待 GitHub Actions 中的 `release` workflow 完成
5. 在 GitHub Release 页面确认：
   - 二进制归档已上传
   - `checksums.txt` 已上传
   - changelog 已生成
6. 在 GHCR 确认镜像已生成

## 常见失败点

### 1. GHCR 推送失败

常见原因：

- `packages: write` 权限不足
- `GITHUB_TOKEN` 权限策略被仓库或组织限制

排查方向：

- 检查 GitHub Actions 权限设置
- 检查仓库或组织是否允许 package 发布

### 2. GoReleaser 测试失败

常见原因：

- `go test ./...` 未通过

排查方向：

- 先本地执行相同命令复现
- 优先修掉测试和编译错误后再重新打 tag

### 3. 镜像构建失败

常见原因：

- Dockerfile 路径错误
- 二进制名与 GoReleaser 配置不一致
- 多架构构建环境异常

排查方向：

- 检查 `.goreleaser.yaml` 中 `dockers_v2` 配置
- 检查 Dockerfile 中的 `COPY` 路径和入口文件名

## 当前边界

- 当前只支持 tag 触发发布，不支持手工 dispatch workflow
- 当前发布流程不包含部署动作，只负责产出 release artifact 和镜像
- 当前 changelog 由 GoReleaser 的 GitHub 模式生成
