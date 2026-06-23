# 调度执行器 SDK 对接说明

本文档面向需要接入 Edge Pilot 调度中心的 Go 执行器，并区分两条接入路径：

- **服务实例执行器**：由 Edge Pilot 部署的业务容器启用 `schedulerSdkPort` 后，agent 注入环境变量、主动连接业务容器内的 `SchedulerInstanceControl.Attach` 服务，并通过 agent relay 注册执行器，不需要用户从管理页复制 token。
- **外部独立执行器**：由接入方自行运行的独立进程，使用普通 SDK 客户端连接调度中心，需要显式配置 `executorId/token/group`。

普通 SDK 客户端包路径为：

```go
github.com/real-uangi/edge-pilot/pkg/scheduler/sdk
```

普通 SDK 客户端通信协议为 `SchedulerControl.Connect` gRPC 双向流。当前 `pkg/scheduler/sdk` 只封装这条普通客户端链路。任务投递语义为至少一次，业务侧必须使用 `JobRunID` 或 `IdempotencyKey` 做幂等。

服务实例执行器走 agent 主动连接业务容器的 `SchedulerInstanceControl.Attach` 通道。agent 会读取受管容器环境变量，生成服务实例 hello，并经由 agent relay 向 control-plane 注册执行器；control-plane 不要求这类 hello 携带普通执行器 token。

## 对接前准备

### 服务实例执行器

1. 在服务配置中设置 `schedulerSdkPort`，使 agent 知道业务容器内 `SchedulerInstanceControl.Attach` 服务监听端口。
2. 设置 `schedulerExecutorGroup`；如果启用了 `schedulerSdkPort` 但未设置分组，服务配置会使用 `default`。
3. 业务容器必须在该端口提供兼容 `SchedulerInstanceControl.Attach` 的 gRPC 服务。
4. 发布服务后，agent 会向业务容器注入调度相关环境变量；当该 release 出现在代理快照的 live 或可见 candidate 中时，agent 扫描运行中的受管容器并建立 Attach 流。
5. agent 使用 `EP_EXECUTOR_ID`、服务 ID、release ID、slot、container ID 等信息生成服务实例 hello，并通过 agent relay 注册执行器。

服务实例执行器不需要在执行器页面预先创建，不需要手工保存 token，也不应该依赖管理页展示 token。当前 `pkg/scheduler/sdk` 没有封装服务实例 Attach 服务端；不要把普通 `NewExecutorClient(...)` 误用为服务实例接入方式。

### 外部独立执行器

1. 在管理端创建执行器记录，确认 `executorId` 与 `group`。
2. 为外部执行器准备可用 token。当前管理页只展示执行器状态，不展示明文 token；如果使用 HTTP API 创建或重置执行器，后端响应中的 `token` 只应被视为一次性敏感值并写入 secret。
3. 选择连接方式：
   - direct：SDK 直连 control-plane gRPC 地址。
   - agent relay：SDK 连接本机 agent relay 地址，由 agent 代转。

管理端执行器 API：

- `POST /api/admin/scheduler/executors`
- `POST /api/admin/scheduler/executors/:id/reset-token`
- `POST /api/admin/scheduler/executors/:id/enable`
- `POST /api/admin/scheduler/executors/:id/disable`

外部独立执行器连接时会校验：

- `executorId` 必须存在。
- 执行器必须启用。
- `token` 必须匹配。
- 如果 hello 中传入 `group`，必须与管理端配置一致。

服务实例执行器连接时不走普通 token 校验，而是要求：

- 请求必须来自 agent relay。
- hello 由 agent 生成，metadata 必须包含服务实例标记和 `service_id`、`release_id`、`executor_id` 等元数据。
- metadata 中的 `executor_id` 必须与 hello 的 `executorId` 一致。
- 同一服务实例执行器不能被重新绑定到其他 agent。

## 连接模式

### Direct

direct 模式适用于外部独立执行器，SDK 直接连接 control-plane 的 gRPC 地址：

```go
client := sdk.NewExecutorClient(sdk.ExecutorClientOptions{
	Addr:       "127.0.0.1:9090",
	ExecutorID: "exec-a",
	Token:      "replace-with-issued-token",
	Group:      "default",
	InstanceID: "exec-a-1",
	Metadata: map[string]string{
		"region": "cn-sh",
	},
})
```

适用场景：

- 执行器与 control-plane 网络互通。
- 不需要依赖 agent 本地 relay。

### Agent Relay

agent relay 模式有两种用法：

- 外部独立执行器连接本机 agent 暴露的 relay 地址，agent 再通过已建立的 agent 通道转发到 control-plane。
- 服务实例执行器由 agent 主动连接业务容器的 `schedulerSdkPort`，打开 `SchedulerInstanceControl.Attach` 流，再通过同一 relay 通道向 control-plane 注册。

agent 侧配置：

- `SCHEDULER_RELAY_LISTEN_ADDR`：默认 `127.0.0.1:19091`
- `SCHEDULER_RELAY_SHARED_TOKEN`：可选，建议配置

外部独立执行器 SDK 配置示例：

```go
client := sdk.NewExecutorClient(sdk.ExecutorClientOptions{
	Addr:       "127.0.0.1:19091",
	ExecutorID: "exec-relay-a",
	Token:      "replace-with-issued-token",
	Group:      "default",
	RelayToken: "replace-with-relay-shared-token",
})
```

说明：

- 如果 agent 配置了 `SCHEDULER_RELAY_SHARED_TOKEN`，SDK 必须设置相同的 `RelayToken`。
- `RelayToken` 只用于执行器到本机 agent relay 的本地鉴权，不替代执行器自己的 `token`。
- 外部独立执行器无需在管理端预先绑定 relay agent。control-plane 会根据实际连接自动回填 `channelMode`、`relayAgentId` 与 `relayRoutingKey`，用于管理端展示和调度路由。
- 服务实例执行器不需要配置 `RelayToken` 给 control-plane；其注册 hello 由 agent 生成，并由 agent relay 和服务实例元数据完成校验。

## SDK 核心类型

### `ExecutorClientOptions`

- `Addr`：目标地址，必填；direct 模式通常是 control-plane gRPC 地址，agent relay 模式通常是本机 relay 地址。
- `ExecutorID`：执行器 ID。
- `Token`：执行器 token。
- `Group`：执行器分组；建议始终传入，并与管理端配置保持一致。
- `InstanceID`：可选，会写入 metadata 的 `instanceId`。
- `LiveSlot`：可选，固定 live 槽位调度场景使用；当前类型为内部 gRPC 槽位枚举。
- `Metadata`：可选，透传元信息。
- `RelayToken`：可选，agent relay 模式下用于本地 relay 鉴权。

兼容字段：

- `Mode`：deprecated，不建议新接入使用。
- `RelayAddr`：deprecated；当 `Addr` 为空时作为回退。

### `RunContext`

handler 收到的运行上下文包含：

- `JobRunID`
- `HandlerKey`
- `Payload`
- `IdempotencyKey`
- `Attempt`
- `ServiceID`
- `ReleaseID`
- `Slot`
- `AgentID`
- `ExecutorID`
- `ServiceKey`
- `BackendName`
- `ServerName`

其中 `Payload` 来自调度任务的 JSON payload。服务相关字段主要用于服务实例执行器和 `fixed_live_slot` 场景。

### `RetryableError`

handler 返回 `*sdk.RetryableError` 时，本次运行会被标记为可重试。普通 `error` 会被视为不可恢复失败。

## 外部独立执行器最小示例

以下示例仅适用于外部独立执行器，即使用普通 `SchedulerControl.Connect` 客户端直连 control-plane 或 agent relay 的进程。

```go
package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/real-uangi/edge-pilot/pkg/scheduler/sdk"
)

func main() {
	client := sdk.NewExecutorClient(sdk.ExecutorClientOptions{
		Addr:       "127.0.0.1:9090",
		ExecutorID: "exec-a",
		Token:      "replace-with-issued-token",
		Group:      "default",
		InstanceID: "exec-a-1",
		Metadata: map[string]string{
			"region": "cn-sh",
		},
	})

	client.RegisterHandler("release.deploy", func(ctx context.Context, run sdk.RunContext) error {
		log.Printf(
			"run=%s handler=%s attempt=%d idempotencyKey=%s payload=%v",
			run.JobRunID,
			run.HandlerKey,
			run.Attempt,
			run.IdempotencyKey,
			run.Payload,
		)

		if run.Attempt < 2 {
			return &sdk.RetryableError{Err: errors.New("temporary error")}
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(30 * time.Minute)
		cancel()
	}()

	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("executor stopped: %v", err)
	}
}
```

## 环境变量辅助入口

agent 部署业务容器时会自动注入以下环境变量：

- `EP_SCHEDULER_SDK_ADDR`
- `EP_SCHEDULER_SDK_PORT`
- `EP_EXECUTOR_ID`
- `EP_SCHEDULER_EXECUTOR_GROUP`
- `EP_SLOT`：`blue` 或 `green`
- `EP_SERVICE_ID`
- `EP_SERVICE_KEY`
- `EP_RELEASE_ID`
- `EP_AGENT_ID`
- `EP_BACKEND_NAME`
- `EP_SERVER_NAME`

其中：

- `EP_EXECUTOR_ID` 每次部署容器时由 agent 生成。
- `EP_SCHEDULER_SDK_ADDR` 默认回退为 `127.0.0.1`。
- `EP_SCHEDULER_SDK_PORT` 来自服务配置的 `schedulerSdkPort`。
- `EP_SCHEDULER_EXECUTOR_GROUP` 来自服务配置的 `schedulerExecutorGroup`。

普通 SDK 客户端提供 `NewExecutorClientFromEnv()`，会读取上述变量中的地址、分组、槽位和服务实例元数据。

注意：

- `NewExecutorClientFromEnv()` 仍然创建普通 `SchedulerControl.Connect` 客户端。
- `NewExecutorClientFromEnv()` 不读取执行器 token。
- 外部独立执行器使用普通 `SchedulerControl.Connect` 客户端时，仍应使用 `NewExecutorClient(...)` 显式传入 `Token`。
- Edge Pilot 托管的服务实例执行器不依赖普通 token；它依赖 agent 注入的环境变量、agent 主动 Attach、以及 agent relay 注册。
- 不要在服务自定义环境变量中覆盖上述 `EP_*` 系统变量；当前容器环境合并逻辑允许用户环境变量覆盖系统变量，覆盖后可能导致 agent 无法识别或连接服务实例执行器。

## 调度选择规则

执行器只有满足以下条件才会被派发任务：

- 管理端执行器记录存在。
- 执行器处于启用状态。
- 执行器已连接并持续心跳。
- 最近心跳未超过 control-plane 的执行器心跳超时时间。
- 执行器分组与任务 `executorGroup` 匹配。

普通任务会在同一分组的可调度执行器之间轮转。

`fixed_live_slot` 任务会按以下顺序选择执行器：

1. 根据任务中的 `serviceId`，或 payload 中的 `serviceId`，解析目标服务。
2. 查询该服务当前 live 槽位。
3. 优先选择同时满足 `serviceId` 匹配、`liveSlot` 匹配、并被识别为服务实例的执行器。
4. 如果没有匹配的服务实例执行器，则退回选择同 `liveSlot` 的非服务实例执行器。
5. 如果仍然没有可用执行器，任务保持待调度或记录离线错误，等待后续调度周期。

## 普通客户端运行时行为

普通 SDK 客户端内置行为：

- 自动重连：连接断开后约每 2 秒重试。
- 心跳：每 5 秒上报当前运行中的 `runId`。
- 租约续租：每 10 秒为运行中的 `runId` 续租。
- 开始上报：handler 执行前上报 `running=true`。
- 完成上报：handler 返回后上报成功、失败、是否可重试和错误信息。

control-plane 侧行为：

- 任务只允许当前租约持有执行器完成或续租。
- 租约执行器不匹配时，完成上报会被拒绝。
- 任务已进入终态后，重复完成上报会被拒绝。
- 可重试失败会进入等待重试，直到超过任务最大重试次数。

## 幂等与错误处理

建议：

- 使用 `JobRunID` 或 `IdempotencyKey` 作为业务幂等键。
- 可恢复异常包装为 `RetryableError`。
- 不可恢复异常直接返回普通 `error`。
- handler 内部应尊重 `context.Context` 取消信号。

示例：

```go
return &sdk.RetryableError{Err: err}
```

## 常见问题

### 报错 `scheduler target addr is required`

`Addr` 为空。请设置 control-plane gRPC 地址或 agent relay 地址。

### 连接时报鉴权失败

外部独立执行器优先检查：

- `executorId` 是否存在。
- 执行器是否已启用。
- `token` 是否正确写入执行器运行环境。
- `group` 是否与管理端配置一致。

服务实例执行器优先检查：

- 服务是否配置了正确的 `schedulerSdkPort`。
- 业务容器是否在 `schedulerSdkPort` 上提供 `SchedulerInstanceControl.Attach` gRPC 服务。
- 业务容器内 SDK 监听地址是否能被 agent 解析并访问。
- 容器环境变量中是否存在 `EP_EXECUTOR_ID`、`EP_SERVICE_ID`、`EP_RELEASE_ID`、`EP_SCHEDULER_EXECUTOR_GROUP`。
- 服务自定义环境变量是否覆盖了 Edge Pilot 注入的 `EP_*` 系统变量。
- agent 是否在线，并且 agent relay 是否正常连接 control-plane。

### 管理页创建执行器为什么没有展示 token？

当前管理页只展示执行器配置和状态，不展示明文 token。服务实例执行器也不需要手工 token，它由 agent 生成服务实例 hello 并通过 relay 注册。

只有外部独立执行器使用普通 `SchedulerControl.Connect` 客户端时才需要 token。此时应通过安全的 secret 交付流程配置 token，不要依赖管理页展示。

### relay 模式认证失败

优先检查：

- SDK 的 `RelayToken` 是否与 agent 的 `SCHEDULER_RELAY_SHARED_TOKEN` 一致。
- 执行器自己的 `executorId/token/group` 是否正确。
- agent 是否在线，并且 agent 到 control-plane 的连接是否正常。

### 任务未分配到期望实例

优先检查：

- 任务 `dispatchPolicy` 是否为 `fixed_live_slot`。
- 执行器是否上报了正确的 `EP_SLOT` 或 `LiveSlot`。
- 任务或 payload 是否包含合法 `serviceId`。
- 服务实例元数据中的 `serviceId` 是否与任务目标服务一致。
- 执行器是否启用且心跳未超时。

## 安全建议

- 外部独立执行器 token 视为高敏凭证，使用 secret 管理。
- 外部独立执行器 token 泄露后立即调用 reset-token 接口轮换。
- relay 模式建议始终启用 `SCHEDULER_RELAY_SHARED_TOKEN`。
- 不要把 control-plane gRPC 地址暴露到不可信网络；当前 gRPC 未启用 TLS。
