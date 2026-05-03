# Edge Pilot 发布响应头与归位接口接入说明

本文档面向业务前端，说明 Edge Pilot 提供的两个能力：

- 通过响应头识别当前请求命中的发布信息
- 通过响应头识别是否存在未切流候选发布
- 通过 `__ep/beta` 接口将会话粘滞 Cookie 切到未切流候选版本
- 通过 `__ep/normalize` 接口将会话粘滞 Cookie 归位到当前 live 版本

## 响应头定义

Edge Pilot 会在业务请求响应中注入以下响应头：

- `X-Edge-Pilot-Current-Release-Id`
  - 含义：当前这次请求实际命中的发布单 ID
  - 类型：UUID 字符串
- `X-Edge-Pilot-Live-Release-Id`
  - 含义：当前服务最新 live 的发布单 ID
  - 类型：UUID 字符串
- `X-Edge-Pilot-Beta-Release-Id`
  - 含义：当前服务存在的未切流候选发布单 ID
  - 类型：UUID 字符串
  - 不存在未切流候选发布时不返回
- `X-Edge-Pilot-Release-Role`
  - 含义：当前命中版本角色
  - 可选值：
    - `live`
    - `canary`
    - `historical`

## Beta 接口：`__ep/beta`

### 接口能力

- 路径：`<routePathPrefix>/__ep/beta`
  - 例如：
    - `routePathPrefix=/` -> `/__ep/beta`
    - `routePathPrefix=/v1` -> `/v1/__ep/beta`
- 方法：`GET`
- 成功响应：`204 No Content`
- 存在未切流候选发布时：
  - 更新粘滞 Cookie `ep_release_id_<serviceKey规范化后>` 到候选 release
  - 返回 `Current Release ID=候选 release`
  - 返回 `Live Release ID=当前 live release`
  - 返回 `Beta Release ID=候选 release`
  - 返回 `Release Role=canary` 或 `historical`
- 不存在未切流候选发布时：
  - 返回 `204 No Content`
  - 不写入粘滞 Cookie

### 判断是否存在 Beta 版本

```ts
const betaReleaseId = response.headers.get("x-edge-pilot-beta-release-id");
const currentReleaseId = response.headers.get("x-edge-pilot-current-release-id");

const hasBeta = Boolean(betaReleaseId);
const canEnterBeta = Boolean(betaReleaseId && currentReleaseId !== betaReleaseId);
```

## 归位接口：`__ep/normalize`

### 接口能力

- 路径：`<routePathPrefix>/__ep/normalize`
  - 例如：
    - `routePathPrefix=/` -> `/__ep/normalize`
    - `routePathPrefix=/v1` -> `/v1/__ep/normalize`
- 方法：`GET`
- 成功响应：`204 No Content`
- 成功时副作用：
  - 更新粘滞 Cookie `ep_release_id_<serviceKey规范化后>` 到当前 live release
  - 返回响应头中的 `Current Release ID`、`Live Release ID`、`Release Role=live`

### 粘滞 Cookie 定义

- 名称：`ep_release_id_<serviceKey规范化后>`
- 值：会话当前粘滞的 `Release ID`
- `Max-Age`：`600`
- `Path`：服务自己的 `routePathPrefix`
- 属性：`HttpOnly`、`SameSite=Lax`

## 最小接入示例

### 读取响应头

```ts
const CURRENT_RELEASE_HEADER = "x-edge-pilot-current-release-id";
const LIVE_RELEASE_HEADER = "x-edge-pilot-live-release-id";
const BETA_RELEASE_HEADER = "x-edge-pilot-beta-release-id";
const RELEASE_ROLE_HEADER = "x-edge-pilot-release-role";

export function readEdgePilotHeaders(response: Response) {
  return {
    currentReleaseId: response.headers.get(CURRENT_RELEASE_HEADER),
    liveReleaseId: response.headers.get(LIVE_RELEASE_HEADER),
    betaReleaseId: response.headers.get(BETA_RELEASE_HEADER),
    releaseRole: response.headers.get(RELEASE_ROLE_HEADER),
  };
}
```

### 静默进入 Beta 并刷新页面

```ts
export async function enterBetaAndReload(routePathPrefix: string) {
  const prefix = routePathPrefix === "/" ? "" : routePathPrefix.replace(/\/+$/, "");
  const betaPath = `${prefix}/__ep/beta`;

  const response = await fetch(betaPath, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  if (!response.ok || response.status !== 204) {
    throw new Error(`enter beta failed: ${response.status}`);
  }

  window.location.reload();
}
```

### 静默归位并刷新页面

```ts
export async function normalizeAndReload(routePathPrefix: string) {
  const prefix = routePathPrefix === "/" ? "" : routePathPrefix.replace(/\/+$/, "");
  const normalizePath = `${prefix}/__ep/normalize`;

  const response = await fetch(normalizePath, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  if (!response.ok || response.status !== 204) {
    throw new Error(`normalize failed: ${response.status}`);
  }

  window.location.reload();
}
```

## 接入注意事项

- `__ep/normalize` 请求必须携带凭据（`credentials: "include"`）
- `__ep/beta` 请求必须携带凭据（`credentials: "include"`）
- 不要依赖 `document.cookie` 读取主粘滞 Cookie（`HttpOnly`）
- 如果业务前端与 API 跨域部署，需要确认中间层透传并暴露上述响应头
