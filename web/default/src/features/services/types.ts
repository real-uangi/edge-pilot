export interface ServiceRecord {
  id: string;
  name: string;
  serviceKey: string;
  agentId: string;
  imageRepo: string;
  containerPort: number;
  cpuLimitCores: number;
  memoryLimitMB: number;
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
  networkAliases: string[];
  publishedPorts: Array<{ hostPort: number; containerPort: number }>;
  enabled: boolean | null;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertServiceInput {
  name: string;
  serviceKey: string;
  agentId: string;
  imageRepo: string;
  containerPort: number;
  cpuLimitCores: number;
  memoryLimitMB: number;
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
  networkAliases: string[];
  publishedPorts: Array<{ hostPort: number; containerPort: number }>;
  enabled: boolean;
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
