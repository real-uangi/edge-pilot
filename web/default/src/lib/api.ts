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
  schedulerSdkPort: number;
  schedulerExecutorGroup: string;
  dockerHealthCheck: boolean | null;
  httpHealthPath: string;
  httpHealthHeaders: Record<string, string>;
  httpExpectedCode: number;
  httpTimeoutSecond: number;
  startupGraceSecond: number;
  httpProbeTimeoutSecond: number;
  httpProbeIntervalSecond: number;
  httpSuccessThreshold: number;
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
  ip: string;
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

export interface AgentHAProxyConfigRecord {
  agentId: string;
  config: string;
  fetchedAt: string;
}

export interface RegistryCredentialRecord {
  id: string;
  registryHost: string;
  username: string;
  secretConfigured: boolean;
  createdAt: string;
  updatedAt: string;
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
  trafficPercent: number;
  targetSlot: number;
  previousLiveSlot: number;
  currentTaskId: string | null;
  switchConfirmed: boolean | null;
  verificationUrl: string;
  stickyCookieName: string;
  stickyCookieTtl: number;
  currentReleaseHeaderName: string;
  liveReleaseHeaderName: string;
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
  lastStep: string;
  dockerHealth: string;
  failureLogs: string;
  cleanupCompleted: boolean | null;
  dispatchedAt: string | null;
  startedAt: string | null;
  completedAt: string | null;
}

export interface ReleaseDetail {
  release: ReleaseRecord;
  tasks: TaskSnapshot[];
}

export interface SchedulerJobRecord {
  id: string;
  name: string;
  taskType: string;
  payload: Record<string, unknown>;
  scheduleKind: number;
  cronExpr: string;
  runAt: string | null;
  nextRunAt: string | null;
  enabled: boolean | null;
  dispatchPolicy: number;
  executorGroup: string;
  leaseTimeoutSec: number;
  maxRetries: number;
  metadata: Record<string, string>;
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerRunRecord {
  id: string;
  jobId: string;
  taskType: string;
  payload: Record<string, unknown>;
  status: number;
  attempt: number;
  maxRetries: number;
  scheduledAt: string;
  dispatchedAt: string | null;
  startedAt: string | null;
  completedAt: string | null;
  leaseExpiresAt: string | null;
  leasedBy: string;
  nextRetryAt: string | null;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerExecutorRecord {
  id: string;
  group: string;
  channelMode: number;
  relayAgentId: string;
  relayRoutingKey: string;
  enabled: boolean | null;
  lastSeenAt: string | null;
  liveSlot: number;
  instanceMeta: Record<string, string>;
  token?: string;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertSchedulerJobInput {
  name: string;
  taskType: string;
  payload: Record<string, unknown>;
  scheduleKind: "one_time" | "cron";
  cronExpr?: string;
  runAt?: string | null;
  enabled?: boolean;
  dispatchPolicy?: "round_robin" | "fixed_live_slot";
  executorGroup: string;
  leaseTimeoutSec?: number;
  maxRetries?: number;
  metadata?: Record<string, string>;
}

export interface UpsertSchedulerExecutorInput {
  executorId: string;
  group: string;
  enabled?: boolean;
  liveSlot?: number;
  metadata?: Record<string, string>;
}

export interface AgentOverview {
  id: string;
  enabled: boolean | null;
  hostname: string;
  ip: string;
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

export interface PerformancePoint {
  cpuPercent: number;
  memoryUsedBytes: number;
  memoryLimitBytes: number;
  source: string;
  collectedAt: string;
}

export interface AgentPerformanceLatest {
  id: string;
  hostname: string;
  ip: string;
  enabled: boolean | null;
  online: boolean | null;
  latest: PerformancePoint | null;
}

export interface SystemPerformanceOverview {
  controlPlaneLatest: PerformancePoint | null;
  controlPlaneHistory: PerformancePoint[];
  agents: AgentPerformanceLatest[];
}

export interface AgentPerformanceHistory {
  agentId: string;
  history: PerformancePoint[];
}

export interface UpsertServiceInput {
  name: string;
  serviceKey: string;
  agentId: string;
  imageRepo: string;
  containerPort: number;
  dockerHealthCheck: boolean;
  httpHealthPath: string;
  httpHealthHeaders: Record<string, string>;
  httpExpectedCode: number;
  httpTimeoutSecond: number;
  startupGraceSecond: number;
  httpProbeTimeoutSecond: number;
  httpProbeIntervalSecond: number;
  httpSuccessThreshold: number;
  schedulerSdkPort: number;
  schedulerExecutorGroup: string;
  routeHost: string;
  routePathPrefix: string;
  env: Record<string, string>;
  command: string[];
  entrypoint: string[];
  volumes: Array<{ source: string; target: string; readOnly: boolean }>;
  publishedPorts: Array<{ hostPort: number; containerPort: number }>;
  enabled: boolean;
}

export interface UpsertRegistryCredentialInput {
  registryHost: string;
  username: string;
  secret: string;
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
  return "请求失败";
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
  getSystemPerformanceOverview() {
    return request<SystemPerformanceOverview>("/api/admin/system/performance");
  },
  getAgentPerformanceHistory(id: string) {
    return request<AgentPerformanceHistory>(`/api/admin/system/performance/agents/${id}/history`);
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
  listRegistryCredentials() {
    return request<RegistryCredentialRecord[]>("/api/admin/registry-credentials");
  },
  getRegistryCredential(id: string) {
    return request<RegistryCredentialRecord>(`/api/admin/registry-credentials/${id}`);
  },
  createRegistryCredential(input: UpsertRegistryCredentialInput) {
    return request<RegistryCredentialRecord>("/api/admin/registry-credentials", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  updateRegistryCredential(id: string, input: UpsertRegistryCredentialInput) {
    return request<RegistryCredentialRecord>(`/api/admin/registry-credentials/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  deleteRegistryCredential(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/registry-credentials/${id}`, {
      method: "DELETE",
    });
  },
  getAgent(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}`);
  },
  getAgentHAProxyConfig(id: string) {
    return request<AgentHAProxyConfigRecord>(`/api/admin/agents/${id}/haproxy-config`);
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
  deleteAgent(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/agents/${id}`, {
      method: "DELETE",
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
  setReleaseTraffic(id: string, percent: number) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/traffic`, {
      method: "POST",
      body: JSON.stringify({ percent }),
    });
  },
  rollbackRelease(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/rollback`, {
      method: "POST",
      body: "{}",
    });
  },
  retryRelease(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/retry`, {
      method: "POST",
      body: "{}",
    });
  },
  listSchedulerJobs() {
    return request<SchedulerJobRecord[]>("/api/admin/scheduler/jobs");
  },
  createSchedulerJob(input: UpsertSchedulerJobInput) {
    return request<SchedulerJobRecord>("/api/admin/scheduler/jobs", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  updateSchedulerJob(id: string, input: UpsertSchedulerJobInput) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  deleteSchedulerJob(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/scheduler/jobs/${id}`, {
      method: "DELETE",
    });
  },
  enableSchedulerJob(id: string) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}/enable`, { method: "POST", body: "{}" });
  },
  disableSchedulerJob(id: string) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}/disable`, { method: "POST", body: "{}" });
  },
  triggerSchedulerJob(id: string, overridePayload?: Record<string, unknown>) {
    return request<SchedulerRunRecord>(`/api/admin/scheduler/jobs/${id}/trigger`, {
      method: "POST",
      body: JSON.stringify({ overridePayload: overridePayload ?? {} }),
    });
  },
  listSchedulerRuns(id: string) {
    return request<SchedulerRunRecord[]>(`/api/admin/scheduler/jobs/${id}/runs`);
  },
  listSchedulerExecutors() {
    return request<SchedulerExecutorRecord[]>("/api/admin/scheduler/executors");
  },
  createSchedulerExecutor(input: UpsertSchedulerExecutorInput) {
    return request<SchedulerExecutorRecord>("/api/admin/scheduler/executors", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  resetSchedulerExecutorToken(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/reset-token`, {
      method: "POST",
      body: "{}",
    });
  },
  enableSchedulerExecutor(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/enable`, { method: "POST", body: "{}" });
  },
  disableSchedulerExecutor(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/disable`, { method: "POST", body: "{}" });
  },
  deleteSchedulerExecutor(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/scheduler/executors/${id}`, { method: "DELETE" });
  },
};

export { ApiError };
