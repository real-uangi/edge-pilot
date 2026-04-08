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
  - `main.go`：使用 `fx` 组装所有模块（`app.Current().Option(...)`）。
- 适配器层：
  - `adapter/http`：HTTP 路由、中间件、静态资源。
  - `adapter/schedule`：定时任务与事件订阅注册。
- 业务层（按领域拆分）：
  - `internal/<domain>/{application,domain,infra}`
- 共享层：
  - `internal/shared/config`：环境配置读取。
  - `internal/shared/events`：领域事件定义。
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
  - 同步链路：HTTP Route -> Application Service -> Mapper/Infra。
  - 异步链路：通过 `eventbus` 发布/订阅事件（订阅集中在 `adapter/schedule/fx.go`）。
- 持久化与基础设施：
  - 主要使用 PostgreSQL（GORM + BaseMapper）。
  - 使用 KV（本地/Redis）做缓存、锁、短期状态（如验证码、幂等标记）。
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
