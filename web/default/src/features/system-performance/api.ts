import { request } from "../../shared/lib/api-client";
import type { SystemPerformanceOverview, AgentPerformanceHistory } from "./types";

export const systemPerformanceApi = {
  getOverview() {
    return request<SystemPerformanceOverview>("/api/admin/system/performance");
  },
  getAgentHistory(id: string) {
    return request<AgentPerformanceHistory>(`/api/admin/system/performance/agents/${id}/history`);
  },
};
