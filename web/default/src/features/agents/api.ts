import { request } from "../../shared/lib/api-client";
import type { AgentRecord, AgentCredentialRecord, AgentHAProxyConfigRecord } from "./types";

export const agentsApi = {
  list() {
    return request<AgentRecord[]>("/api/admin/agents");
  },
  get(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}`);
  },
  getHAProxyConfig(id: string) {
    return request<AgentHAProxyConfigRecord>(`/api/admin/agents/${id}/haproxy-config`);
  },
  create() {
    return request<AgentCredentialRecord>("/api/admin/agents", {
      method: "POST",
    });
  },
  resetToken(id: string) {
    return request<AgentCredentialRecord>(`/api/admin/agents/${id}/reset-token`, {
      method: "POST",
    });
  },
  enable(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}/enable`, {
      method: "POST",
    });
  },
  disable(id: string) {
    return request<AgentRecord>(`/api/admin/agents/${id}/disable`, {
      method: "POST",
    });
  },
  delete(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/agents/${id}`, {
      method: "DELETE",
    });
  },
};
