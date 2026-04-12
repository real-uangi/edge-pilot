# Docker 部署说明

本文档说明如何在 Linux 上以非 root 用户方式部署 `edge-pilot-control` 和 `edge-pilot-agent`，并给出宿主机创建专用用户、授权 `docker.sock` 访问的操作步骤。

## 镜像内运行用户

当前两个镜像都内置了非 root 用户：

- 用户名：`edgepilot`
- UID：`10001`
- GID：`10001`
- 工作目录：`/var/lib/edge-pilot`

容器默认以该用户启动，不需要额外指定 `--user`。

## 宿主机准备

### 1. 创建专用用户

建议在 Linux 宿主机上创建独立运维用户，用于拉镜像、运行容器和维护配置文件：

```bash
sudo useradd \
  --system \
  --create-home \
  --home-dir /var/lib/edge-pilot \
  --shell /usr/sbin/nologin \
  edgepilot
```

如需给该用户预留配置目录，可继续执行：

```bash
sudo install -d -o edgepilot -g edgepilot /etc/edge-pilot
sudo install -d -o edgepilot -g edgepilot /var/lib/edge-pilot
```

### 2. 加入 docker 组

如果准备让宿主机上的 `edgepilot` 用户直接执行 `docker` 命令，需要把它加入 `docker` 组：

```bash
sudo usermod -aG docker edgepilot
```

生效方式二选一：

```bash
sudo loginctl terminate-user edgepilot
```

或重新登录该用户会话。

可用以下命令确认授权已生效：

```bash
id edgepilot
sudo -u edgepilot docker ps
```

## control-plane 部署

### 必填环境变量

- `DB_DSN`
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `ADMIN_SESSION_SECRET`

### 常用可选环境变量

- `HTTP_PORT`：默认 `8080`
- `GRPC_PORT`：默认 `9090`
- `PPROF_PORT`：默认 `18080`
- `CI_SHARED_TOKEN`
- `WEB_THEME`：默认 `default`
- `TRUSTED_PROXY_CIDRS`
- `TRUST_CLOUDFLARE`
- `DB_CONN_MAX_IDLE_TIME`
- `DB_MAX_IDLE_CONN`
- `DB_MAX_OPEN_CONN`

### `docker run` 示例

```bash
docker run -d \
  --name edge-pilot-control \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 9090:9090 \
  -e DB_DSN='postgres://edgepilot:secret@127.0.0.1:5432/edge_pilot?sslmode=disable' \
  -e ADMIN_USERNAME='admin' \
  -e ADMIN_PASSWORD='change-me' \
  -e ADMIN_SESSION_SECRET='replace-with-a-long-random-string' \
  -e CI_SHARED_TOKEN='replace-with-a-long-random-string' \
  -e HTTP_PORT='8080' \
  -e GRPC_PORT='9090' \
  ghcr.io/real-uangi/edge-pilot-control:latest
```

## agent 部署

### 必填环境变量

- `AGENT_ID`
- `AGENT_TOKEN`
- `CONTROL_PLANE_GRPC_ADDR`

### 常用可选环境变量

- `REGISTRY_SECRET_MASTER_KEY`：仅 control-plane 使用；base64 编码后的 32 字节主密钥，用于加密存储私有镜像仓库密码/令牌
- `SERVICE_SECRET_MASTER_KEY`：仅 control-plane 使用；base64 编码后的 32 字节主密钥，用于加密存储服务环境变量以及发布任务中的敏感片段
- `DOCKER_SOCKET_PATH`：默认 `/var/run/docker.sock`
- `HTTP_PROBE_TIMEOUT_SECONDS`：默认 `5`
- `PROXY_NETWORK_NAME`：默认 `epNet`
- `PROXY_NETWORK_SUBNET`：默认 `172.29.0.0/24`
- `HAPROXY_IMAGE`：默认 `haproxytech/haproxy-debian:s6-3.4`
- `PROXY_HELPER_IMAGE`：默认跟 `HAPROXY_IMAGE` 一致
- `HAPROXY_CONTAINER_NAME`：默认 `edge-pilot-haproxy`
- `HAPROXY_IP`：默认 `172.29.0.233`
- `HAPROXY_CONFIG_VOLUME`：默认 `ep_haproxy_cfg`
- `HAPROXY_RUNTIME_PORT`：默认 `19999`
- `DATAPLANEAPI_PORT`：默认 `5555`
- `HAPROXY_DATAPLANE_USERNAME`：默认 `admin`
- `HAPROXY_DATAPLANE_PASSWORD`：默认 `edge-pilot-internal`
- `PROXY_SELF_HEAL_INTERVAL_SECONDS`：默认 `10`

### 关键授权说明

`edge-pilot-agent` 镜像虽然以非 root 用户运行，但它必须访问宿主机的 `/var/run/docker.sock`。仅挂载 socket 还不够，还需要把 socket 对应的宿主机组 GID 追加到容器进程的 supplementary groups。

推荐先取出 `docker.sock` 的组 GID：

```bash
DOCKER_SOCK_GID="$(stat -c '%g' /var/run/docker.sock)"
echo "$DOCKER_SOCK_GID"
```

然后运行 agent 时追加该组：

```bash
docker run -d \
  --name edge-pilot-agent \
  --restart unless-stopped \
  --network host \
  --group-add "${DOCKER_SOCK_GID}" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENT_ID='replace-with-control-plane-issued-uuid' \
  -e AGENT_TOKEN='replace-with-control-plane-issued-token' \
  -e CONTROL_PLANE_GRPC_ADDR='127.0.0.1:9090' \
  -e DOCKER_SOCKET_PATH='/var/run/docker.sock' \
  ghcr.io/real-uangi/edge-pilot-agent:latest
```

说明：

- `--network host` 适合当前 agent 托管代理栈的模型，代理容器需要暴露宿主机 `80` 端口。
- `--group-add "${DOCKER_SOCK_GID}"` 是让镜像内非 root 用户能够访问挂载进来的 `docker.sock`。
- 若不追加该组，agent 启动时的 Docker socket 可访问性检查会失败。

### 先创建 agent 凭证

agent 启动前，必须先在 control-plane 创建凭证：

```bash
curl -u admin:password \
  -X POST \
  http://127.0.0.1:8080/api/admin/agents
```

返回结果中会包含：

- `id`
- `token`

其中：

- `id` 写入 `AGENT_ID`
- `token` 写入 `AGENT_TOKEN`

token 明文只会在创建和重置时返回一次。

## 运行用户与权限边界

### control-plane

- 镜像内运行用户：`edgepilot`
- 不需要访问宿主机 Docker
- 不需要额外 Linux capabilities
- 若需要平台内私有镜像登录能力，需要额外配置 `REGISTRY_SECRET_MASTER_KEY`
- 若服务配置里会填写环境变量，需要额外配置 `SERVICE_SECRET_MASTER_KEY`

### agent

- 镜像内运行用户：`edgepilot`
- 需要访问 `/var/run/docker.sock`
- 需要通过 `--group-add <docker.sock gid>` 追加 socket 对应组
- 不建议把 agent 容器切回 root；默认非 root 模型已经能满足访问 Docker 的需求
- agent 本身不保存镜像仓库凭据；私有镜像发布时由 control-plane 按 registry host 匹配并通过内部 gRPC 下发拉取凭据

## 排错建议

### agent 启动时报 Docker socket 不可访问

优先检查：

1. `/var/run/docker.sock` 是否已挂载
2. `--group-add "$(stat -c '%g' /var/run/docker.sock)"` 是否已添加
3. 宿主机 Docker daemon 是否正常运行

### control-plane 无法连 PostgreSQL

优先检查：

1. `DB_DSN` 是否正确
2. 宿主机或容器网络是否能访问 PostgreSQL
3. PostgreSQL 用户权限与数据库名是否正确
