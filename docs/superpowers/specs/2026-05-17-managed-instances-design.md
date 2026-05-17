# 设计文档：受管实例列表与实时日志

日期：2026-05-17

## 1. 需求概述

在 Control Plane 管理面新增**受管实例**全局页面，支持：

- 查看所有 Agent 节点上的受管 Docker 容器列表
- 查看单个容器的完整元数据信息
- 通过 SSE 流式查看容器实时日志

## 2. 关键决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 页面入口 | 全局独立页面 `/instances` | 跨 Agent 视角，不受限于单个节点 |
| 数据获取 | 按需查询，并发聚合 | 无缓存一致性问题，数据实时 |
| 日志实时性 | SSE 流式推送 | 满足"实时"需求 |
| 日志交互 | 弹窗对话框 | 快速查看，不离开列表上下文 |
| 元数据展示 | 点击展开详情面板 | 列表简洁，详情完整 |

## 3. 前端设计

### 3.1 路由与导航

- 路由：`/instances` → `InstancesPage`
- 导航：在 `AppShell.navItems` 中"节点"和"发布"之间插入 `{ to: "/instances", label: "受管实例" }`

### 3.2 页面组件

- **`InstancesPage`**：列表页，包含过滤、搜索、表格
- **`ContainerDetailsPanel`**：展开行详情面板，展示分组元数据
- **`ContainerLogDialog`**：弹窗组件，SSE 连接、日志渲染、过滤、清空

### 3.3 状态管理

- 使用 `react-query` 管理列表和详情数据
- 列表自动刷新间隔：10 秒
- SSE 连接在弹窗打开时建立，关闭时断开

### 3.4 日志渲染

- 使用 `ansi-to-html`（或同类库）渲染 ANSI 颜色码
- 前端本地过滤：stdout/stderr 切换、关键字搜索
- 清空按钮：仅清空前端 DOM，不影响实际日志

## 4. gRPC 协议扩展

在 `agent_control.proto` 中新增：

**请求-响应模式消息：**
- `ContainerListRequest` / `ContainerListResponse`
- `ContainerInspectRequest` / `ContainerInspectResponse`

**流模式消息：**
- `ContainerLogStreamRequest`（`follow=true` 启动流，`follow=false` 停止流）
- `ContainerLogChunk`

修改 `AgentMessage` 和 `ControlMessage` 的 `oneof payload`。

## 5. 后端架构（Control-plane）

### 5.1 领域归属

- **归属域**：`internal/agent`
- **新增文件**：
  - `internal/agent/application/managedcontainer/service.go`（应用服务）
  - `internal/shared/dto/instance.go`（DTO）
  - `adapter/http/controlplane/routes/instances.go`（HTTP 路由）

### 5.2 HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/instances` | 获取所有受管实例列表 |
| GET | `/api/admin/instances/:agentId/:containerId` | 获取实例详情 |
| GET | `/api/admin/instances/:agentId/:containerId/logs/stream` | SSE 实时日志流 |

### 5.3 核心逻辑

**ListContainers：**
1. 获取所有在线 Agent 列表
2. 并发发送 `ContainerListRequest`（单 Agent 5 秒超时）
3. 聚合结果，返回 `[]dto.ManagedInstanceOutput`

**GetContainerDetails：**
1. 检查 Agent 在线状态
2. 发送 `ContainerInspectRequest`（8 秒超时）
3. 返回 `dto.ManagedInstanceDetailsOutput`

**StreamContainerLogs：**
1. 检查 Agent 在线状态
2. 发送 `ContainerLogStreamRequest(follow=true, tail_lines=100)`
3. 通过 channel 接收 `ContainerLogChunk`，写入 SSE response
4. 当客户端断开（ctx 取消），发送 `ContainerLogStreamRequest(follow=false)`

### 5.4 SessionHub 扩展

- 新增 pending request map 用于 `ContainerListResponse` 和 `ContainerInspectResponse`
- 新增 log stream router：将 `ContainerLogChunk` 路由到对应的 SSE writer

## 6. 后端架构（Agent）

### 6.1 新增处理逻辑（`adapter/grpc/agent/client.go`）

**handleContainerListRequest：**
- 调用 `docker.ListManagedContainers(ctx, agentID, "")`
- 转换为 `ContainerSummary` 列表响应

**handleContainerInspectRequest：**
- 调用 `docker.InspectContainer` 获取状态
- 调用 Docker API 获取完整 inspect 信息（需要扩展 docker_client）
- 转换为 `ContainerDetails` 响应

**startLogStream：**
- 启动 goroutine 调用 Docker logs API（`follow=1`）
- 逐行读取，封装为 `ContainerLogChunk` 推送
- 维护 `map[string]context.CancelFunc` 管理活跃流
- 收到 `follow=false` 时调用 cancel

**stopLogStream：**
- 查找并调用对应 containerID 的 cancel 函数
- 从活跃流映射中删除

### 6.2 Docker 客户端扩展

- 新增 `StreamContainerLogs(ctx, containerID, tailLines int, stdout, stderr, follow bool) (io.ReadCloser, error)` 方法
- 返回的 `ReadCloser` 由调用方管理生命周期

## 7. 数据流

**列表查询：**
```
前端 → GET /instances → ManagedContainerService.ListContainers
  → 并发请求各 Agent → Agent.Docker.ListManagedContainers
  → 聚合返回 JSON
```

**日志流：**
```
前端 → SSE /logs/stream → StreamContainerLogs
  → gRPC ContainerLogStreamRequest(follow=true) → Agent
  → Docker logs follow → Agent 推送 ContainerLogChunk
  → control-plane SSE 推送 → 前端
  [弹窗关闭] → SSE 断开 → gRPC follow=false → Agent 停止读取
```

## 8. 安全与边界

- **超时**：列表单 Agent 5 秒，详情 8 秒，日志 chunk 30 秒无数据视为断开
- **并发**：SessionHub 使用 `sync.RWMutex`，Agent 使用 `sync.Mutex`
- **资源清理**：SSE 断开自动停止日志流，Agent 离线清理所有流
- **鉴权**：复用现有 admin session 中间件

## 9. 文件变更清单

**前端：**
- `web/default/src/app/router.tsx`（新增路由）
- `web/default/src/shared/components/AppShell.tsx`（新增导航）
- `web/default/src/features/instances/`（新增目录，包含页面、组件、API、类型）
- `web/default/package.json`（可能新增 `ansi-to-html` 依赖）

**gRPC：**
- `internal/shared/grpcapi/agent_control.proto`（新增消息类型）

**后端（Control-plane）：**
- `internal/shared/dto/instance.go`（新增 DTO）
- `internal/agent/application/managedcontainer/service.go`（新增应用服务）
- `adapter/http/controlplane/routes/instances.go`（新增路由）
- `adapter/grpc/controlplane/server.go`（扩展消息处理）

**后端（Agent）：**
- `adapter/grpc/agent/client.go`（扩展消息处理）
- `internal/agent/infra/runtime/docker_client.go`（扩展日志流方法）
