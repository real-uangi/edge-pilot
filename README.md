# Edge Pilot

Edge Pilot 是一个面向单机 Docker 多服务场景的控制面，用来解决三类问题：

- 多服务发布过程可控
- 蓝绿发布流量切换自动化
- API 与运行态观测沉淀

当前仓库采用单仓库双二进制形态：

- `edge-pilot-control`：control-plane，负责管理 API、CI 集成、Web 控制台、发布编排、任务持久化、审计和观测
- `edge-pilot-agent`：agent，负责连接 control-plane，托管本机代理栈，执行本机 Docker 与 HAProxy 操作，并回报任务进度与运行指标

## 架构概览

- `control-plane` 是唯一持久化中心，连接 PostgreSQL
- `agent` 不连接数据库，只通过内部 gRPC 双向流连接 `control-plane`
- `agent` 必须先在 control-plane 中创建独立凭证，并手动配置中心签发的 UUID `AGENT_ID` 与随机 `AGENT_TOKEN`
- CI 回调只负责创建排队中的发布请求，真正开始发布由管理员显式触发
- `agent` 仅依赖 `docker.sock`，并在本机自举共享 `haproxy(s6, 内含 dataplaneapi) + epNet` 代理栈
- `control-plane` 在 agent 建连后和服务配置变更后，会把该 agent 负责的完整代理配置快照立即推送给 agent

当前核心链路：

1. CI 调用 `POST /api/integration/ci/releases`
2. `control-plane` 校验服务配置后创建 `queued` 状态的发布请求
3. 同一服务允许多个请求排队，但同一时刻只能有一个活动发布
4. 管理员调用 `POST /api/admin/releases/:id/start` 开始指定请求
5. `control-plane` 创建首个 `deploy_green` 任务并通过 gRPC 长连接推送给目标 `agent`
6. `agent` 先确保共享代理栈健康并且最近一次代理配置快照已经应用成功
7. `agent` 用固定名称拉起目标槽位容器，业务容器接入 `epNet`，按配置执行 Docker health + HTTP probe
8. 健康通过后，发布单进入 `ready_to_switch`
9. 管理员调用确认切流接口
10. `agent` 通过 HAProxy Runtime API 切换流量
11. 保留当前 live 槽位和当前 rollback 槽位容器，并清理更旧的受管容器

## 当前发布与恢复语义

### 蓝绿发布

- CI 回调只会创建排队请求，不会直接启动发布
- 管理员可以对 `queued` 请求执行：
  - `start`
  - `skip`
- `skip` 是终态，不会重新入队
- 容器固定命名：
  - `ep-<serviceKey>-blue`
  - `ep-<serviceKey>-green`
- 受管容器固定标签：
  - `ep.managed=true`
  - `ep.agent_id`
  - `ep.service_id`
  - `ep.service_key`
  - `ep.slot`
  - `ep.release_id`
- agent 只管理当前 agent 自己创建的受管容器，不会接管或删除外部容器
- 若宿主机上存在同名但非受管容器，任务直接失败，不做接管

### 代理栈托管

- 每个 agent 维护一套共享代理栈：
  - `edge-pilot-haproxy`
  - Docker 网络 `epNet`
- `haproxytech/haproxy-debian:s6-3.4` 单容器同时承载 HAProxy 和 Data Plane API
- 共享 frontend 固定监听 HTTP `:80`
- Runtime API 只负责运行态切流、摘挂 server 和 stats
- Data Plane API 负责 frontend、route、backend、blue/green server 结构同步
- 服务只需要配置：
  - `containerPort`
  - `routeHost`
  - `routePathPrefix`
- 服务可选配置：
  - `publishedPorts`
- backend 名称由系统按 `serviceId` 稳定派生，server 名称固定为 `blue` / `green`
- HAProxy upstream 固定使用受管容器名，不使用业务容器固定 IP

### 断线恢复

- `agent` 每 `5s` 发送一次 heartbeat
- `control-plane` 在 heartbeat 时使用 `running_task_ids` 对账
- 未完成但不在运行中的当前任务会被重放一次
- 同一 session 内同一任务只会重放一次
- `15s` 无心跳会把 agent 标记为 offline
- `10m` 无进展的任务会被标记为 `timed_out`，对应发布单进入失败态

### 旧容器清理

- 当前稳态只保留两类受管容器：
  - 当前 live 槽位容器
  - 当前 rollback 槽位容器
- 切流成功后会 best-effort 清理更旧的受管容器

## 目录结构

- `cmd/control-plane`：control-plane 二进制入口
- `cmd/agent`：agent 二进制入口
- `adapter/http/controlplane`：管理 API、CI 集成与静态站点挂载
- `adapter/grpc/controlplane`：内部 gRPC 服务端与 agent session 管理
- `adapter/grpc/agent`：agent gRPC 长连接客户端
- `adapter/schedule`：离线扫描、任务超时扫描等后台调度
- `internal/servicecatalog`：服务定义、镜像、端口、探活、host/path 路由配置
- `internal/release`：发布单、任务、切流、回滚、审计
- `internal/agent`：agent 注册、心跳、恢复、执行器、代理栈自举、自愈、Docker/HAProxy 适配
- `internal/observability`：总览、实例状态和 backend 指标快照

## HTTP 接口

### CI 集成

- `POST /api/integration/ci/releases`

认证方式：

- 若配置了 `CI_SHARED_TOKEN`，请求头必须带 `X-EdgePilot-Token`

### 管理接口

- `POST /api/admin/services`
- `PUT /api/admin/services/:id`
- `GET /api/admin/services`
- `GET /api/admin/services/:id`
- `POST /api/admin/agents`
- `GET /api/admin/agents`
- `GET /api/admin/agents/:id`
- `POST /api/admin/agents/:id/reset-token`
- `POST /api/admin/agents/:id/enable`
- `POST /api/admin/agents/:id/disable`
- `GET /api/admin/releases`
- `GET /api/admin/releases/:id`
- `POST /api/admin/releases/:id/start`
- `POST /api/admin/releases/:id/skip`
- `POST /api/admin/releases/:id/confirm-switch`
- `POST /api/admin/releases/:id/rollback`
- `GET /api/admin/overview`
- `GET /api/admin/services/:id/observability`
- `GET /metrics`

## 运行配置

### control-plane

关键配置：

- PostgreSQL 连接配置：由 `allingo` 的数据库模块提供
- `ADMIN_USERNAME`：管理后台登录用户名
- `ADMIN_PASSWORD`：管理后台登录密码
- `ADMIN_SESSION_SECRET`：管理后台 Cookie 会话签名密钥
- `TRUSTED_PROXY_CIDRS`：可信反代网段或 IP，逗号分隔，用于识别 `X-Forwarded-Proto`
- `TRUST_CLOUDFLARE`：是否启用 Cloudflare 平台真实 IP 识别
- `GRPC_PORT`：内部 gRPC 监听端口，默认 `9090`
- `CI_SHARED_TOKEN`：CI 回调鉴权
- `WEB_THEME`：Web 主题，默认 `default`

### agent

关键配置：

- `AGENT_ID`：必填，必须是 control-plane 创建 agent 后签发的 UUID
- `AGENT_TOKEN`：必填，必须是 control-plane 创建或重置 agent 后一次性拿到的随机 token
- `CONTROL_PLANE_GRPC_ADDR`：control-plane gRPC 地址，默认 `127.0.0.1:9090`
- `DOCKER_SOCKET_PATH`：默认 `/var/run/docker.sock`
- `HTTP_PROBE_TIMEOUT_SECONDS`：默认 `5`
- `PROXY_NETWORK_NAME`：默认 `epNet`
- `PROXY_NETWORK_SUBNET`：默认 `172.29.0.0/24`
- `HAPROXY_IMAGE`：默认 `haproxytech/haproxy-debian:s6-3.4`
- `HAPROXY_IP`：默认 `172.29.0.233`
- `HAPROXY_RUNTIME_PORT`：默认 `19999`
- `DATAPLANEAPI_PORT`：默认 `5555`
- `HAPROXY_DATAPLANE_USERNAME`：默认 `admin`
- `HAPROXY_DATAPLANE_PASSWORD`：默认 `edge-pilot-internal`
- `PROXY_SELF_HEAL_INTERVAL_SECONDS`：默认 `10`

`AGENT_VERSION` 不再通过环境变量读取，而是直接使用编译时注入的 build info。

推荐运维流程：

1. 先调用 `POST /api/admin/agents` 创建 agent 凭证
2. 把返回的 `id` 和 `token` 手动写入 agent 环境变量
3. 如 token 泄露或轮换，调用 `POST /api/admin/agents/:id/reset-token`

## 本地构建

```bash
cd web/default && pnpm install
cd ../..
make proto
make build VERSION=v0.1.0
```

产物输出到 `dist/`：

- `dist/edge-pilot-control`
- `dist/edge-pilot-agent`

启动时会打印编译信息：

- `version`
- `commit`
- `build_time`

## 发布产物

GitHub Actions 对 `v*` tag 触发 release：

- 产出 GitHub Release 二进制归档
- 推送两个 GHCR 镜像：
  - `ghcr.io/<owner>/edge-pilot-control`
  - `ghcr.io/<owner>/edge-pilot-agent`

镜像基础镜像当前为 `debian:bookworm-slim`。

## 当前限制

- 当前聚焦 HTTP 服务的蓝绿发布，不支持 worker 与非 HTTP 协议
- 切流仍然是人工确认，不做灰度权重
- gRPC 当前未启用 TLS
- 当前共享入口只支持 HTTP `:80`，不支持 HTTPS、证书和多 frontend
- 业务容器默认不暴露宿主机端口；如需额外暴露，使用服务配置中的 `publishedPorts`
- `cleanup_old` 未单独扩展为独立发布步骤，旧容器清理作为 agent 的后处理执行
