# Control-Plane CI 回调触发说明

本文档面向对接方，说明如何从 CI/CD 系统调用 `edge-pilot-control` 的发布回调接口，创建一条新的发布请求。

## 接口概览

当前 control-plane 提供的 CI 回调接口：

- `POST /api/integration/ci/releases`

用途：

- 由外部 CI/CD 在镜像构建完成后调用
- 为指定服务创建一条新的排队发布请求
- 不会直接启动发布，不会直接下发任务给 agent
- 后续由管理员决定开始发布还是跳过该请求

## 触发前置条件

调用前需要满足以下条件：

- 目标服务已经在 control-plane 中完成配置
- 请求中的 `serviceKey` 能匹配到已存在服务
- 服务处于启用状态

若上述任一条件不满足，control-plane 会拒绝创建发布单。

## 鉴权方式

当前接口使用共享 token 鉴权。

control-plane 侧配置：

- `CI_SHARED_TOKEN`

当 `CI_SHARED_TOKEN` 非空时，请求必须带 header：

```http
X-EdgePilot-Token: <your-ci-shared-token>
```

如果没有配置 `CI_SHARED_TOKEN`，当前实现不会强制校验该 header。

## 请求地址

示例：

```text
https://edge-pilot.example.com/api/integration/ci/releases
```

## 请求方法

```http
POST
Content-Type: application/json
```

## 请求体

当前请求体定义对应：

- [CreateReleaseFromCIRequest](/Users/laopixu/GolandProjects/edge-pilot/internal/shared/dto/release.go)

字段说明：

- `serviceKey`
  - 必填
  - control-plane 中已配置的服务标识
- `imageRepo`
  - 选填
  - 若传入，则优先使用本次请求里的镜像仓库
  - 若不传，则使用服务配置中的默认 `imageRepo`
- `imageTag`
  - 必填
  - 本次要发布的镜像 tag
- `commitSha`
  - 选填
  - 本次构建对应的 git commit
- `triggeredBy`
  - 选填
  - 触发来源，如 `github-actions`、`gitlab-ci`
- `traceId`
  - 选填
  - 对接方自带的链路追踪 ID，建议传入

请求示例：

```json
{
  "serviceKey": "edge-api",
  "imageRepo": "ghcr.io/laopixu/edge-api",
  "imageTag": "v1.3.2",
  "commitSha": "8b5a2f7c2d0b6d7e8a9f0123456789abcdef1234",
  "triggeredBy": "github-actions",
  "traceId": "gh-run-123456789"
}
```

## 成功响应

当前 control-plane 使用统一 JSON 响应包装：

```json
{
  "code": 200,
  "data": {},
  "message": "OK",
  "time": "2026-04-09T09:30:00+08:00"
}
```

`data` 部分为新创建的发布单信息，字段对应：

- [ReleaseOutput](/Users/laopixu/GolandProjects/edge-pilot/internal/shared/dto/release.go)

成功响应示例：

```json
{
  "code": 200,
  "message": "OK",
  "time": "2026-04-09T09:30:00+08:00",
  "data": {
    "id": "7e2a6cf0-f0d7-49d1-90d8-9cb6f52e2d2f",
    "serviceId": "4d1fbb68-91f5-4d40-847e-3d3db2df5f7e",
    "agentId": "edge-node-a",
    "imageRepo": "ghcr.io/laopixu/edge-api",
    "imageTag": "v1.3.2",
    "commitSha": "8b5a2f7c2d0b6d7e8a9f0123456789abcdef1234",
    "triggeredBy": "github-actions",
    "traceId": "gh-run-123456789",
    "status": 1,
    "targetSlot": 2,
    "previousLiveSlot": 1,
    "currentTaskId": null,
    "switchConfirmed": false,
    "isActive": false,
    "queuePosition": 1,
    "createdAt": "2026-04-09T09:30:00+08:00",
    "updatedAt": "2026-04-09T09:30:00+08:00",
    "completedAt": null
  }
}
```

说明：

- `status=1` 当前对应 `queued`
- 创建成功只表示请求已经进入队列，不代表发布已经开始
- 真正开始发布需要管理员后续调用 `POST /api/admin/releases/:id/start`

## 常见失败响应

### 1. 鉴权失败

HTTP 状态码：

- `401`

示例：

```json
{
  "code": 401,
  "message": "Unauthorized",
  "data": null,
  "time": "2026-04-09T09:30:00+08:00"
}
```

### 2. 参数错误

常见场景：

- `serviceKey` 缺失
- `imageTag` 缺失
- 请求体不是合法 JSON

通常返回：

- `400`

### 3. 服务不可发布

常见场景：

- 服务不存在
- 服务已禁用

常见返回：

- `404`
- `400`

如果同一服务下已经存在相同镜像的排队请求或活动发布：

- `serviceKey + imageTag + commitSha` 相同，会直接返回已有请求
- 当 `commitSha` 为空时，按 `serviceKey + imageTag` 去重

## 接入建议

建议每次镜像构建完成后，由 CI 在“镜像已经可拉取”这一时刻调用该接口，而不是在代码提交时立即触发。

推荐做法：

1. 先完成镜像构建和镜像推送
2. 再调用 control-plane 回调创建发布请求
3. 等管理员在控制台审核后手动开始发布
4. 将 CI 自己的流水线 ID 或 run ID 写入 `traceId`
5. 将 CI 系统标识写入 `triggeredBy`
6. 如果镜像仓库在服务维度固定，`imageRepo` 可以不传

## curl 示例

```bash
curl -X POST "https://edge-pilot.example.com/api/integration/ci/releases" \
  -H "Content-Type: application/json" \
  -H "X-EdgePilot-Token: your-ci-shared-token" \
  -d '{
    "serviceKey": "edge-api",
    "imageRepo": "ghcr.io/laopixu/edge-api",
    "imageTag": "v1.3.2",
    "commitSha": "8b5a2f7c2d0b6d7e8a9f0123456789abcdef1234",
    "triggeredBy": "github-actions",
    "traceId": "gh-run-123456789"
  }'
```

## GitHub Actions 示例

```yaml
- name: Trigger Edge Pilot Release
  run: |
    curl -X POST "${{ secrets.EDGE_PILOT_URL }}/api/integration/ci/releases" \
      -H "Content-Type: application/json" \
      -H "X-EdgePilot-Token: ${{ secrets.EDGE_PILOT_TOKEN }}" \
      -d "{
        \"serviceKey\": \"edge-api\",
        \"imageRepo\": \"ghcr.io/${{ github.repository_owner }}/edge-api\",
        \"imageTag\": \"${{ github.ref_name }}\",
        \"commitSha\": \"${{ github.sha }}\",
        \"triggeredBy\": \"github-actions\",
        \"traceId\": \"gh-run-${{ github.run_id }}\"
      }"
```

## 当前边界

- 该回调当前只负责“创建排队中的发布请求”，不会自动开始发布
- 发布开始仍需管理员调用 `POST /api/admin/releases/:id/start`
- 管理员也可以调用 `POST /api/admin/releases/:id/skip` 直接跳过该请求
- 切流仍需管理员后续调用确认接口
