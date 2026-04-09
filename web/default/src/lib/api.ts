export interface ApiEnvelope<T> {
  code: number;
  data: T;
  message: string;
  time: string;
}

export interface SessionInfo {
  username: string;
  expiresAt: string;
}

export interface LoginInput {
  username: string;
  password: string;
}

export interface ServiceRecord {
  id: string;
  name: string;
  serviceKey: string;
  agentId: string;
  imageRepo: string;
  containerPort: number;
  currentLiveSlot: number;
  dockerHealthCheck: boolean | null;
  httpHealthPath: string;
  httpExpectedCode: number;
  httpTimeoutSecond: number;
  routeHost: string;
  routePathPrefix: string;
  env: Record<string, string>;
  command: string[];
  entrypoint: string[];
  volumes: Array<{ source: string; target: string; readOnly: boolean }>;
  publishedPorts: Array<{ hostPort: number; containerPort: number }>;
  enabled: boolean | null;
  createdAt: string;
  updatedAt: string;
}

export interface AgentRecord {
  id: string;
  enabled: boolean | null;
  hostname: string;
  version: string;
  online: boolean | null;
  lastHeartbeatAt: string | null;
  lastConnectedAt: string | null;
  lastError: string;
  tokenRotatedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AgentCredentialRecord extends AgentRecord {
  token: string;
}

export interface ReleaseRecord {
  id: string;
  serviceId: string;
  agentId: string;
  imageRepo: string;
  imageTag: string;
  commitSha: string;
  triggeredBy: string;
  traceId: string;
  status: number;
  targetSlot: number;
  previousLiveSlot: number;
  currentTaskId: string | null;
  switchConfirmed: boolean | null;
  isActive: boolean;
  queuePosition: number;
  createdAt: string;
  updatedAt: string;
  completedAt: string | null;
}

export interface TaskSnapshot {
  id: string;
  type: number;
  status: number;
  lastError: string;
  dispatchedAt: string | null;
  startedAt: string | null;
  completedAt: string | null;
}

export interface ReleaseDetail {
  release: ReleaseRecord;
  tasks: TaskSnapshot[];
}

export interface AgentOverview {
  id: string;
  enabled: boolean | null;
  hostname: string;
  version: string;
  online: boolean | null;
  lastHeartbeatAt: string | null;
}

export interface RuntimeInstance {
  id: string;
  serviceId: string;
  releaseId: string;
  slot: number;
  containerId: string;
  imageTag: string;
  listenAddress: string;
  hostPort: number;
  serverName: string;
  healthy: boolean | null;
  acceptingTraffic: boolean | null;
  active: boolean | null;
  updatedAt: string;
}

export interface BackendStat {
  serviceId: string;
  backendName: string;
  serverName: string;
  scur: number;
  rate: number;
  errorRequests: number;
  createdAt: string;
}

export interface ObservabilityRecord {
  serviceId: string;
  runtimeInstances: RuntimeInstance[];
  backendStats: BackendStat[];
}

export interface OverviewRecord {
  agents: AgentOverview[];
  services: ServiceRecord[];
  recentReleases: ReleaseRecord[];
  activeInstances: number;
}

export interface UpsertServiceInput {
  name: string;
  serviceKey: string;
  agentId: string;
  imageRepo: string;
  containerPort: number;
  dockerHealthCheck: boolean;
  httpHealthPath: string;
  httpExpectedCode: number;
  httpTimeoutSecond: number;
  routeHost: string;
  routePathPrefix: string;
  env: Record<string, string>;
  command: string[];
  entrypoint: string[];
  volumes: Array<{ source: string; target: string; readOnly: boolean }>;
  publishedPorts: Array<{ hostPort: number; containerPort: number }>;
  enabled: boolean;
}

class ApiError extends Error {
  status: number;
  code: number;

  constructor(message: string, status: number, code: number) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });
  let payload: ApiEnvelope<T> | null = null;
  try {
    payload = (await response.json()) as ApiEnvelope<T>;
  } catch {
    payload = null;
  }
  if (!response.ok || !payload || payload.code >= 400) {
    throw new ApiError(payload?.message ?? response.statusText, response.status, payload?.code ?? response.status);
  }
  return payload.data;
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Request failed";
}

export const api = {
  login(input: LoginInput) {
    return request<SessionInfo>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  logout() {
    return request<{ ok: boolean }>("/api/auth/logout", {
      method: "POST",
    });
  },
  me() {
    return request<SessionInfo>("/api/auth/me");
  },
  overview() {
    return request<OverviewRecord>("/api/admin/overview");
  },
  listServices() {
    return request<ServiceRecord[]>("/api/admin/services");
  },
  getService(id: string) {
    return request<ServiceRecord>(`/api/admin/services/${id}`);
  },
  createService(input: UpsertServiceInput) {
    return request<ServiceRecord>("/api/admin/services", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  updateService(id: string, input: UpsertServiceInput) {
    return request<ServiceRecord>(`/api/admin/services/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  getServiceObservability(id: string) {
    return request<ObservabilityRecord>(`/api/admin/services/${id}/observability`);
  },
  listAgents() {
    return request<AgentRecord[]>("/api/admin/agents");
  },
  getAgent(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}`);
  },
  createAgent() {
    return request<AgentCredentialRecord>("/api/admin/agents", {
      method: "POST",
    });
  },
  resetAgentToken(id: string) {
    return request<AgentCredentialRecord>(`/api/admin/agents/${id}/reset-token`, {
      method: "POST",
    });
  },
  enableAgent(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}/enable`, {
      method: "POST",
    });
  },
  disableAgent(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}/disable`, {
      method: "POST",
    });
  },
  listReleases() {
    return request<ReleaseRecord[]>("/api/admin/releases");
  },
  getRelease(id: string) {
    return request<ReleaseDetail>(`/api/admin/releases/${id}`);
  },
  startRelease(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/start`, {
      method: "POST",
      body: "{}",
    });
  },
  skipRelease(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/skip`, {
      method: "POST",
      body: "{}",
    });
  },
  confirmSwitch(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/confirm-switch`, {
      method: "POST",
      body: "{}",
    });
  },
  rollbackRelease(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/rollback`, {
      method: "POST",
      body: "{}",
    });
  },
};

export { ApiError };
