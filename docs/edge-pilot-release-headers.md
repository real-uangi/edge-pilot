# Edge Pilot 发布响应头接入说明

本文档面向业务前端，说明如何利用 Edge Pilot 注入的响应头判断当前页面是否仍然命中最新发布版本，并在需要时触发刷新或重载。

## 目标

在蓝绿发布和回滚期间，同一浏览器会话可能因为历史页面、旧静态资源、粘滞 Cookie 或预发布验证入口而停留在非最新版本。

前端需要完成两件事：

- 判断当前请求实际命中的发布版本
- 判断当前命中的发布版本是否已经落后于最新 live 版本

## 响应头定义

Edge Pilot 会在业务请求响应中注入以下两个响应头：

- `X-Edge-Pilot-Current-Release-Id`
  - 含义：当前这次请求实际命中的发布单 ID
  - 类型：UUID 字符串
- `X-Edge-Pilot-Live-Release-Id`
  - 含义：当前服务最新 live 的发布单 ID
  - 类型：UUID 字符串

只要这两个响应头存在，前端就可以做版本检查，不需要读取粘滞 Cookie。

## 粘滞 Cookie 定义

前端通常不需要直接读取 Cookie，但接入调试时可以了解其定义：

- 名称：`ep_release_id_<serviceKey规范化后>`
- 值：当前会话粘滞到的 `Release ID`
- `Max-Age`：`600`
- `Path`：服务自己的 `routePathPrefix`
- 属性：
  - `HttpOnly`
  - `SameSite=Lax`

因为主 Cookie 是 `HttpOnly`，业务前端不要依赖 `document.cookie` 做版本判断，应该始终以响应头为准。

## 判定规则

### 1. 判断是否命中最新版本

当以下条件成立时，说明当前页面或当前请求已经落后于最新 live 版本：

```text
X-Edge-Pilot-Current-Release-Id !== X-Edge-Pilot-Live-Release-Id
```

建议行为：

- 对用户显示“检测到页面版本已更新，建议刷新”的提示
- 对纯后台轮询页面，可直接触发整页刷新

### 2. 判断会话是否被网关重新归位

页面初始化后，前端应缓存一次 `currentReleaseId`。若后续任意同源请求返回的 `currentReleaseId` 与页面初始化记录不同，说明发生了以下情况之一：

- 旧 Cookie 已失效，被网关自动归位到当前 live
- 发布切流或回滚已经完成
- 当前页面最初来自预发布验证入口，但会话已不再命中原发布

建议行为：

- 直接整页刷新
- 或者在 SPA 中先清理本地状态，再执行一次 `window.location.reload()`

## 推荐接入方式

推荐在前端统一的 HTTP 客户端层做检查，而不是分散在各个页面里。

### Fetch 示例

```ts
const CURRENT_RELEASE_HEADER = "x-edge-pilot-current-release-id";
const LIVE_RELEASE_HEADER = "x-edge-pilot-live-release-id";

let initialReleaseId: string | null = null;

export async function request(input: RequestInfo | URL, init?: RequestInit) {
  const response = await fetch(input, {
    credentials: "include",
    ...init,
  });

  const currentReleaseId = response.headers.get(CURRENT_RELEASE_HEADER);
  const liveReleaseId = response.headers.get(LIVE_RELEASE_HEADER);

  if (!initialReleaseId && currentReleaseId) {
    initialReleaseId = currentReleaseId;
  }

  if (currentReleaseId && liveReleaseId && currentReleaseId !== liveReleaseId) {
    showRefreshBanner();
  }

  if (initialReleaseId && currentReleaseId && initialReleaseId !== currentReleaseId) {
    window.location.reload();
  }

  return response;
}
```

### Axios 示例

```ts
const CURRENT_RELEASE_HEADER = "x-edge-pilot-current-release-id";
const LIVE_RELEASE_HEADER = "x-edge-pilot-live-release-id";

let initialReleaseId: string | null = null;

axios.interceptors.response.use((response) => {
  const currentReleaseId = response.headers[CURRENT_RELEASE_HEADER];
  const liveReleaseId = response.headers[LIVE_RELEASE_HEADER];

  if (!initialReleaseId && currentReleaseId) {
    initialReleaseId = currentReleaseId;
  }

  if (currentReleaseId && liveReleaseId && currentReleaseId !== liveReleaseId) {
    showRefreshBanner();
  }

  if (initialReleaseId && currentReleaseId && initialReleaseId !== currentReleaseId) {
    window.location.reload();
  }

  return response;
});
```

## 推荐策略

推荐把前端行为分成两级：

- 软提示：
  - `currentReleaseId !== liveReleaseId`
  - 说明页面落后，但仍然可用
  - 建议显示刷新提示，由用户决定何时刷新
- 强制刷新：
  - `initialReleaseId !== currentReleaseId`
  - 说明页面运行时所处版本已经变化
  - 建议直接整页刷新，避免继续运行旧 JS 状态

## 适用请求范围

前端应优先选择以下请求做检查：

- 页面初始化时的首个业务 API
- 全局轮询请求
- 路由切换后必经的同源接口

如果业务前端与 API 跨域部署，需要先确认中间层是否透传这两个响应头。

## 注意事项

- 不要依赖 `document.cookie` 读取主粘滞 Cookie
- 不要用 slot（blue/green）做前端版本判断，前端应始终以 `Release ID` 为准
- 不要只在局部组件里做版本检查，最好统一收敛到请求层
- 如果页面包含长连接、SSE、WebSocket，建议在连接重建后同步执行一次版本检查

## 调试建议

联调时可以同时观察以下信息：

- 浏览器响应头：
  - `X-Edge-Pilot-Current-Release-Id`
  - `X-Edge-Pilot-Live-Release-Id`
- 浏览器 Cookie：
  - `ep_release_id_<serviceKey规范化后>` 是否被更新
- 管理端发布详情页：
  - 当前展示的验证链接
  - 当前暴露的响应头名称

如果响应头中的 `currentReleaseId` 长时间不变，但 `liveReleaseId` 已变化，通常说明当前页面仍停留在旧版本，应提示刷新或强制刷新。
