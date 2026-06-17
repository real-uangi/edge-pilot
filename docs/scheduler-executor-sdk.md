# 调度执行器 SDK 对接说明

本文档说明如何使用 `pkg/scheduler/sdk` 接入调度中心执行器。

## 适用范围

- 语言：Go
- SDK 包：`edge-pilot/pkg/scheduler/sdk`
- 通信协议：`SchedulerControl.Connect` gRPC 双向流
- 投递语义：至少一次（`JobRunID`/`IdempotencyKey` 用于业务幂等）

## 对接前准备

1. 在管理端创建执行器凭证（调度中心页面或 API）
2. 记录以下信息：
- `executorId`
- `token`（只在创建/重置时返回一次）
- `group`
- `liveSlot`（仅 `fixed_live_slot` 策略会用到）
3. 选择连接模式：
- direct：SDK 直连 control-plane
- agent relay：SDK 连接本地 agent relay 地址

管理端创建执行器 API：

- `POST /api/admin/scheduler/executors`
- `POST /api/admin/scheduler/executors/:id/reset-token`

## SDK 核心类型

### `ExecutorClientOptions`

- `Addr`: 目标地址（必填），例如 `127.0.0.1:9090` 或 `127.0.0.1:19091`
- `ExecutorID`: 执行器 ID
- `Token`: 执行器 token
- `Group`: 执行器分组（建议与管理端保持一致）
- `InstanceID`: 可选，SDK 会自动写入 metadata 的 `instanceId`
- `LiveSlot`: 可选，`fixed_live_slot` 场景建议设置（`SLOT_BLUE` / `SLOT_GREEN`）
- `Metadata`: 可选，透传元信息
- `RelayToken`: 可选，relay 模式下用于 agent 本地 relay 鉴权

兼容字段（不建议新接入使用）：

- `Mode`（deprecated）
- `RelayAddr`（deprecated；当 `Addr` 为空时作为回退）

### `RunContext`

- `JobRunID`
- `HandlerKey`
- `Payload`
- `IdempotencyKey`
- `Attempt`

### `RetryableError`

handler 返回 `*sdk.RetryableError` 时，SDK 会把本次失败标记为可重试。

## 最小对接示例

```go
package main

import (
	"context"
	"edge-pilot/pkg/scheduler/sdk"
	"errors"
	"log"
	"time"
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
		// 建议使用 run.JobRunID 或 run.IdempotencyKey 做业务幂等
		log.Printf("run=%s taskType=%s attempt=%d payload=%v", run.JobRunID, run.TaskType, run.Attempt, run.Payload)
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

## Direct 模式

配置建议：

- `Addr`: control-plane gRPC 地址（默认常见 `127.0.0.1:9090`）
- 不需要设置 `RelayToken`

说明：

- SDK 与 control-plane 直接握手认证
- 适用于执行器与 control-plane 网络互通场景

## Agent Relay 模式

### 1) 配置 agent relay 监听

agent 侧环境变量：

- `SCHEDULER_RELAY_LISTEN_ADDR`（默认 `127.0.0.1:19091`）
- `SCHEDULER_RELAY_SHARED_TOKEN`（可选但推荐）

### 2) 管理端执行器配置

执行器无需手工配置 relay 绑定。  
`channelMode/relayAgentId/relayRoutingKey` 由中心根据实际连接状态自动回填用于展示。

### 3) SDK 配置

- `Addr=<SCHEDULER_RELAY_LISTEN_ADDR>`
- 若 agent 配置了共享 token，则设置 `RelayToken=<SCHEDULER_RELAY_SHARED_TOKEN>`

示例：

```go
client := sdk.NewExecutorClient(sdk.ExecutorClientOptions{
	Addr:       "127.0.0.1:19091",
	ExecutorID: "exec-relay-a",
	Token:      "replace-with-issued-token",
	Group:      "default",
	RelayToken: "replace-with-relay-shared-token",
})
```

## 运行时行为

SDK 内置行为：

- 自动重连：连接断开后约每 2 秒重试
- 心跳：每 5 秒上报运行中的 `runId`
- 租约续租：每 10 秒对运行中的 `runId` 续租
- 执行上报：开始执行时上报 `running=true`；结束后上报成功/失败

## 幂等与重试建议

建议：

- 以 `JobRunID` 或 `IdempotencyKey` 作为业务幂等键
- 将可恢复异常包装为 `RetryableError`
- 不可恢复异常直接返回普通 `error`

## 常见问题

### 1) 报错 `scheduler target addr is required`

未设置 `Addr`，或值为空字符串。

### 2) relay 模式认证失败

优先检查：

- `RelayToken` 是否与 agent 的 `SCHEDULER_RELAY_SHARED_TOKEN` 一致
- 执行器 `executorId/token/group` 是否与管理端一致

### 3) 任务未分配到期望实例

优先检查：

- 任务 `dispatchPolicy` 是否为 `fixed_live_slot`
- 执行器 `LiveSlot` 是否正确
- 任务 payload 是否包含合法 `serviceId`（`fixed_live_slot` 必需）

## 安全建议

- 执行器 token 视为高敏凭证，使用 secret 管理
- token 泄露后立即调用 reset-token 接口轮换
- relay 模式建议始终启用 `SCHEDULER_RELAY_SHARED_TOKEN`
