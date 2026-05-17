export interface ManagedInstanceRecord {
  agentId: string;
  agentHost: string;
  containerId: string;
  name: string;
  state: string;
  image: string;
  serviceId: string;
  serviceKey: string;
  releaseId: string;
  slot: string;
  createdAt: number;
}

export interface ManagedInstanceDetailsRecord extends ManagedInstanceRecord {
  running: boolean;
  health: string;
  restartCount: number;
  ipAddress: string;
  labels: Record<string, string>;
  env: Record<string, string>;
  command: string[];
  entrypoint: string[];
  volumes: Array<{ source: string; target: string; readOnly: boolean }>;
  ports: Array<{ hostPort: number; containerPort: number }>;
  cpuLimit: number;
  memoryLimit: number;
}

export interface LogChunk {
  data: string;
  stderr: boolean;
  timestamp: number;
}
