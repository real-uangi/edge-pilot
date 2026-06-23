# Edge Pilot

Edge Pilot 是面向单机 Docker 多服务部署的轻量控制面。它把服务配置、镜像发布、蓝绿切流、Agent 执行与运行态观测放到同一套管理流程中，适合需要可控发布但不想引入完整 Kubernetes 集群的场景。

## 适用场景

- 一台或少量 Linux 主机承载多个 HTTP 服务。
- 服务以 Docker 容器运行，镜像由 CI/CD 构建并推送到镜像仓库。
- 发布需要先部署候选版本，再人工确认切流。
- 希望统一管理 Agent 凭证、私有镜像凭据、发布队列、任务进度和运行指标。

Edge Pilot 当前聚焦 HTTP 服务蓝绿发布。它不是通用容器编排平台，也不负责 worker、非 HTTP 协议、HTTPS 证书托管或多 frontend 流量入口。

## 核心能力

- **Control Plane**：提供管理后台、HTTP API、CI 回调入口、发布编排、任务持久化、审计与观测。
- **Agent**：连接 control-plane，访问宿主机 Docker，托管本机 `HAProxy + Data Plane API` 代理栈，并执行部署、探活、切流和清理任务。
- **蓝绿发布**：CI 只创建排队发布请求，管理员在管理后台确认启动、验证候选版本、切流或回滚。
- **私有镜像凭据**：control-plane 按 registry host 管理凭据，发布时自动匹配并下发给目标 agent。
- **调度执行器**：外部 Go 执行器可通过 SDK 接入调度中心，支持 direct 与 agent relay 两种连接模式。

## 快速部署路径

### 1. 部署 control-plane

control-plane 需要 PostgreSQL，并至少配置以下环境变量：

- `DB_DSN`
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `ADMIN_SESSION_SECRET`

常用端口：

- HTTP 管理面：`8080`
- 内部 gRPC：`9090`

Docker 部署示例、非 root 用户和 Linux 权限配置见 [Docker 部署说明](docs/docker-deploy.md)。

### 2. 创建 agent 凭证

agent 不支持首次自动注册。启动 agent 前，需要先在 control-plane 管理后台或 API 中创建 agent，拿到：

- `AGENT_ID`
- `AGENT_TOKEN`

token 明文只会在创建或重置时返回一次，请使用 secret 管理。

### 3. 部署 agent

agent 至少需要：

- `AGENT_ID`
- `AGENT_TOKEN`
- `CONTROL_PLANE_GRPC_ADDR`
- Docker socket 访问权限

agent 启动后会连接 control-plane，并在本机自举共享代理栈：

- Docker 网络：默认 `epNet`
- HAProxy 容器：默认 `edge-pilot-haproxy`
- HTTP 入口：默认监听宿主机 `:80`

agent 只管理自己创建的 Edge Pilot 受管容器，不会接管或删除外部容器。

### 4. 配置服务

在管理后台创建服务时，通常需要提供：

- 服务标识 `serviceKey`
- 镜像仓库 `imageRepo`
- 容器端口 `containerPort`
- 路由域名 `routeHost`
- 路由路径前缀 `routePathPrefix`
- 目标 agent
- Docker health 或 HTTP probe 配置

如果镜像是私有仓库镜像，请先在“镜像仓库凭据”中配置对应 registry host 的用户名和密码或 token。

### 5. 从 CI 创建发布请求

镜像构建并推送完成后，CI 调用：

```http
POST /api/integration/ci/releases
```

该接口只创建 `queued` 发布请求，不会直接启动部署。管理员后续在管理后台选择开始、跳过、切流或回滚。

CI 请求格式、鉴权 header、响应字段和去重语义见 [Control-Plane CI 回调触发说明](docs/control-plane-ci-callback.md)。

### 6. 发布与验证

典型发布流程：

1. CI 创建排队发布请求。
2. 管理员启动发布。
3. agent 拉起目标槽位容器并执行探活。
4. 发布进入可切流状态后，管理员访问候选版本验证。
5. 验证通过后确认切流。
6. 系统保留当前 live 与 rollback 槽位，清理更旧的受管容器。

业务前端可读取 Edge Pilot 注入的发布响应头，并通过 `__ep/beta` / `__ep/normalize` 控制会话进入候选版本或归位到 live 版本。接入方式见 [发布响应头与归位接口接入说明](docs/edge-pilot-release-headers.md)。

## 调度执行器接入

Edge Pilot 的调度中心支持两类执行器：

- **服务实例执行器**：服务配置了 `schedulerSdkPort` 后，agent 部署容器时注入 `EP_EXECUTOR_ID`、`EP_SCHEDULER_*`、`EP_SERVICE_*` 等变量；随后 agent 扫描 live/candidate 受管容器，主动连接容器内的 `SchedulerInstanceControl.Attach` 服务，并通过 agent relay 向 control-plane 注册执行器。此模式不需要用户在管理页复制或配置执行器 token。
- **外部独立执行器**：独立进程使用普通 Go SDK 客户端连接调度中心。此模式需要 `executorId/token/group`，其中 token 属于外部执行器凭证，不是服务实例自动注入路径的一部分。

外部独立执行器支持两种连接方式：

- **direct**：执行器直接连接 control-plane gRPC 地址。
- **agent relay**：执行器连接本机 agent relay，由 agent 代转到 control-plane。

外部独立执行器的 direct / agent relay 连接不需要在管理端预先固定绑定，control-plane 会根据实际连接状态回填展示字段。SDK 使用方式、服务实例链路、错误语义、重试与租约说明见 [调度执行器 SDK 对接说明](docs/scheduler-executor-sdk.md)。

## 配置摘要

control-plane 常用配置：

- `DB_DSN`
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `ADMIN_SESSION_SECRET`
- `CI_SHARED_TOKEN`
- `GRPC_PORT`
- `WEB_THEME`
- `REGISTRY_SECRET_MASTER_KEY`
- `SERVICE_SECRET_MASTER_KEY`

agent 常用配置：

- `AGENT_ID`
- `AGENT_TOKEN`
- `CONTROL_PLANE_GRPC_ADDR`
- `DOCKER_HOST`
- `PROXY_NETWORK_NAME`
- `HAPROXY_IMAGE`
- `SCHEDULER_RELAY_LISTEN_ADDR`
- `SCHEDULER_RELAY_SHARED_TOKEN`

完整部署配置以 [Docker 部署说明](docs/docker-deploy.md) 为准。

## 文档索引

使用文档：

- [Docker 部署说明](docs/docker-deploy.md)
- [Control-Plane CI 回调触发说明](docs/control-plane-ci-callback.md)
- [发布响应头与归位接口接入说明](docs/edge-pilot-release-headers.md)
- [调度执行器 SDK 对接说明](docs/scheduler-executor-sdk.md)

参考文件与排错素材：

- [HAProxy Data Plane API 规格](docs/dataplane-spec.json)
- [Data Plane 失败快照示例](docs/fail.json)
- [HAProxy 失败配置示例](docs/fail.cfg)

## 当前限制

- 当前聚焦 HTTP 服务蓝绿发布，不支持 worker 与非 HTTP 协议。
- 切流由管理员人工确认，不提供自动灰度权重。
- gRPC 当前未启用 TLS。
- 共享入口默认只支持 HTTP `:80`，不包含 HTTPS、证书和多 frontend 托管。
- 业务容器默认不暴露宿主机端口；如需额外暴露，使用服务配置中的 `publishedPorts`。
