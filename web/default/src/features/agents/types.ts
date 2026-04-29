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

export interface AgentOverview {
  id: string;
  enabled: boolean | null;
  hostname: string;
  ip: string;
  version: string;
  online: boolean | null;
  lastHeartbeatAt: string | null;
}
