# Design

## 1. 设计北极星（Synthetic Architect）

`Edge Pilot Control Plane` 的视觉语言采用 **Synthetic Architect**：

- 高密度信息优先：同屏容纳更多运行态信息，减少跳页与滚动。
- 层次靠“明度差”而非“分割线”：通过 surface 阶梯与留白定义分组。
- 技术权威感：标题与指标有强对比，正文保持克制，中性且可读。
- 架构化布局：采用 `Asymmetric Rail`（左导航 + 中央工作区 + 右健康栏）。

该规范替代旧的浅色 Uber 风格，作为本项目前端唯一视觉约束。

---

## 2. 颜色与层级 Token

### 2.1 基础面板色阶（Tonal Layering）

- `--ep-bg`: `#10131a`
- `--ep-surface-lowest`: `#0b0e14`
- `--ep-surface`: `#10131a`
- `--ep-surface-low`: `#191c22`
- `--ep-surface-mid`: `#1d2026`
- `--ep-surface-high`: `#272a31`
- `--ep-surface-highest`: `#32353c`

### 2.2 文字与边界

- `--ep-text`: `#e1e2eb`
- `--ep-text-muted`: `#bbc9cf`
- `--ep-text-dim`: `#859399`
- `--ep-outline`: `#3c494e`
- `--ep-outline-ghost`: `rgba(60,73,78,0.15)`

### 2.3 强调与状态色

- 主强调：
  - `--ep-primary`: `#a4e6ff`
  - `--ep-primary-strong`: `#00d1ff`
  - `--ep-primary-ink`: `#003543`
- 成功：`--ep-secondary` / `--ep-secondary-soft`
- 告警：`--ep-warning` / `--ep-warning-soft`
- 错误：`--ep-danger` / `--ep-danger-soft`

### 2.4 设计规则

- **No-Line Rule**：常规分组禁止 1px 实线分割。
- 如必须补边界，仅允许 Ghost Border（`--ep-outline-ghost`）。
- 支持玻璃态容器：半透明 surface + `backdrop-filter: blur(12px)`。

---

## 3. 字体、密度与圆角

### 3.1 字体角色

- Headline / Display：`Space Grotesk`
- Body / UI：`Inter`
- 技术数据（ID、IP、日志、命令）：`JetBrains Mono`（fallback `ui-monospace`）

### 3.2 密度与节奏

- 8px 网格，允许 4px 微调。
- 优先紧凑编排：表格、卡片、过滤控件保持短路径扫描。
- 状态信息尽量就地展示，不做大段解释文案。

### 3.3 圆角约束

- `xs=4` / `sm=6` / `md=8` / `lg=10` / `xl=12`
- 禁止使用过大胶囊化圆角（避免消费级“圆润感”）。

---

## 4. 布局骨架与页面映射

### 4.1 全局骨架（Asymmetric Rail）

- 左侧 Rail：品牌、主导航、会话动作（退出）。
- 中央 Stage：页面主内容区。
- 右侧 Rail：Global Health（在线节点、活动发布、失败告警、最近发布）。

### 4.2 页面对齐策略

- Dashboard：大指标 + 服务/节点/发布高密度摘要。
- Services / ServiceEditor：配置与观测并置，技术字段易扫描。
- Agents / AgentDetail：节点状态、凭据、HAProxy 配置与错误线索。
- Releases / ReleaseDetail：状态流转、风险动作、任务时间线。
- RegistryCredentials / Login：统一深色语义与组件语言。

---

## 5. 组件规范

### 5.1 按钮

- Primary：主色渐变（`primary -> primary-strong`），深色文本。
- Secondary：`surface-high` 到 `surface-highest` 的层级提升。
- Danger：低饱和红底 + 高可读红字。
- Ghost：透明背景 + Ghost Border，用于低优先级动作。

### 5.2 表格与列表

- 禁止横向细线分隔。
- 使用行块背景 + 行间距（`border-spacing`）表达层次。
- 技术值优先等宽字体。

### 5.3 输入

- Track 使用 `surface-mid`。
- 边界采用底部 2px 线；focus 切至 `primary-strong`。
- 兼顾紧凑密度与键盘可访问性。

### 5.4 状态组件

- Loading / Error / Empty 统一深色容器规范。
- InlineNotice 分为 info / error 两类，避免高饱和高噪音。
- StatusPill 使用低辉光背景（soft tone），不使用重描边。

---

## 6. 响应式与可用性

- Desktop：三栏完整布局。
- Tablet：右侧健康栏可折叠为抽屉。
- Mobile：左/右 Rail 均折叠为抽屉，中央 Stage 优先。
- 所有交互组件保留明确 focus-ring（键盘可达）。

---

## 7. Do / Don’t

### Do

- 使用 surface 阶梯建立结构层次。
- 让关键运行指标在首屏可见。
- 对可复制技术数据使用等宽字体。
- 在风险操作中保持清晰状态反馈（pending / disabled / error）。

### Don’t

- 不要回到浅色主题或大面积纯白。
- 不要恢复通用 1px 分割线布局。
- 不要引入过大圆角或消费化装饰风。
- 不要在页面中写入平台策略解释性文案。
