# AGENTS

## Go 代码修改规范

- 修改 Go 代码后，提交前必须执行：
  - `goimports`
  - 必要的构建/测试检查（如 `go test`、按模块定向检查等）
- 若改动涉安全、鉴权、风控等核心业务逻辑，建议补充或更新单元测试，覆盖关键分支与异常路径。
- 在新增数据库实体model时，要注意避免零值歧义，原则上表示状态的bool类型都需要定义为指针，iota枚举类型必须从iota+1开始
- 新增model时必须重写对应`TableName() string`方法，名称格式固定`ep_`前缀

## 前端代码修改规范

- 前端改动需与项目现有视觉与交互风格保持一致，不引入突兀的样式或不统一的组件用法。
  - 遵循DESIGN.md
- **严禁**将流程编排和平台内部策略写进前端页面。
- 禁止添加无意义的解释文案。
- 提交前需执行必要检查（如类型检查、lint、构建或项目约定的前端校验命令），确保页面可正常运行。

## 项目结构速览

- 入口与装配：
  - `cmd/control-plane`：control-plane 独立二进制入口，负责 HTTP 管理面、Web、CI 集成、内部 gRPC 服务与持久化装配。
  - `cmd/agent`：agent 独立二进制入口，负责本机执行器与内部 gRPC 客户端装配。
  - `internal/bootstrap`：两种角色的 `fx` 装配入口。
- 适配器层：
  - `adapter/http/controlplane`：control-plane HTTP 路由、中间件装配、CI 集成接口、静态站点挂载。
  - `adapter/http/routes`：共享基础路由（如 metrics）。
  - `adapter/grpc/controlplane`：control-plane 内部 gRPC 服务端与 agent 会话管理。
  - `adapter/grpc/agent`：agent 到 control-plane 的长连接客户端。
  - `adapter/schedule`：定时任务与事件订阅注册。
- 业务层（按领域拆分）：
  - `internal/servicecatalog/{application,domain,infra}`：服务定义、镜像、端口、探活、host/path 路由配置与代理快照发布。
  - `internal/release/{application,domain,infra}`：发布单、任务、切流、回滚、审计。
  - `internal/agent/{application,domain,infra}`：agent 注册、鉴权、心跳、执行器、共享代理栈自举/自愈、本机 Docker/HAProxy 适配。
  - `internal/observability/{application,domain,infra}`：总览、实例状态、后端指标快照查询与上报入库。
- 共享层：
  - `internal/shared/config`：环境配置读取。
  - `internal/shared/grpcapi`：control-plane 与 agent 共用的 gRPC 协议与 codec。
  - `internal/shared/model`：ORM 模型定义。
  - `internal/shared/dto`：通用 DTO。
- 前端：
  - 目录`web/<theme>`，将通过`go embed.FS`内嵌构建产物
  - 默认Theme：`default`

## 核心架构说明

- 架构风格：
  - 以后端 `fx` 模块化 DI 为核心，按领域拆分为 `application/domain/infra` 三层。
  - 标准架构依赖为 `github.com/real-uangi/allingo`，如需数据库、缓存、定时任务、消息队列等基础能力，优先使用其中的公共架构，禁止手搓连接。
- 关键交互：
  - control-plane 同步链路：HTTP Route -> Application Service -> Domain Port/Infra。
  - agent 执行链路：Agent gRPC Stream -> 代理配置快照/Task Executor -> Docker/HAProxy Infra。
  - control-plane 与 agent 之间使用内部 gRPC 双向流传输任务、心跳、任务状态、代理配置快照和观测上报。
- 持久化与基础设施：
  - PostgreSQL 仅由 control-plane 持有，用于服务配置、发布单、任务、审计、agent 节点状态与观测快照固化。
  - agent 不连接数据库，只连接 control-plane 内部 gRPC 通道。
  - agent 必须使用 control-plane 预先签发的 UUID `AGENT_ID` 与独立随机 `AGENT_TOKEN` 建连，不允许共享 token 或首次自动注册。
  - agent 仅依赖 Docker Socket，并负责自举共享 `haproxy(s6, 内含 dataplaneapi) + epNet` 代理栈。
  - Runtime API 只做运行态切流/摘挂与 stats，Data Plane API 负责 frontend、route、backend、blue/green server 结构管理。
- HTTP API相关：
  - 对于单值入参的方法，一般使用api.SingleQueryFunc。对于复杂结构体入参，优先POST+api.JsonFunc
  - 新增API时需要注意水平与垂直越权问题

## DDD 约束

- `application` 层只负责编排业务用例，不直接做基础设施细节实现。
- 禁止在 `application` 包中出现手写 SQL（包括 `Raw(...)`、拼接 SQL 字符串、直接 `Where("...")` 写复杂查询等）。
- 涉及 IO 的实现必须下沉到 `infra` 层：
  - 数据库访问与查询实现
  - 缓存/Redis 访问
  - 外部 HTTP/RPC/SMTP/文件系统等
- `application` 通过端口接口依赖 `infra`，由 `fx` 在模块层完成注入。
- 新增功能时优先先定义 `domain` 接口与模型，再在 `infra` 实现，最后由 `application` 编排调用。

## 域边界约束

- 禁止跨域直接访问他域 `infra`、他域 `mapper` 或他域数据库表。
- 跨域协作仅允许以下方式：
  1. 在使用方定义接口port，由能力的归属域实现并注入。
  2. 使用领域事件（发布/订阅）进行最终一致性协作。
- 不允许在本域 `application` 中直接引用他域持久化模型做查询或更新。
- 如需共享结构，优先抽象到 `internal/shared` 的最小公共 DTO/事件，避免复用他域内部实现对象。
- 新需求先判定“领域归属”，策略归策略域，数据归数据拥有域，避免边界漂移。

## 工程守则（精简）

- 事务边界：仅在单域内使用 DB 事务；跨域流程通过事件与补偿保证最终一致。
- 幂等优先：支付、回调、发货、通知等关键入口必须具备幂等键与可重试设计。
- 错误分层：区分可重试/不可重试错误；对外返回稳定业务语义，不暴露基础设施细节。
- 可观测性：关键链路日志应包含 `traceId/userId/orderId/eventId` 等定位字段。
- 配置治理：新增环境变量必须同步到部署/CI模板；敏感值仅使用 secret 管理。
- 兼容演进：接口与数据结构变更优先向前兼容，遵循“先加后切再删”。
- 前端质量：关键交互必须有加载、失败、重试状态；改动后执行必要构建/检查。
- 完成标准（DoD）：代码 + 格式化 + 必要测试 + 必要文档/注释 + 部署配置同步。
