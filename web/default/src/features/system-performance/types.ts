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
