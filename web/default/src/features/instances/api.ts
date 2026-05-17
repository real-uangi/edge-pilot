import { request } from "../../shared/lib/api-client";
import type { ManagedInstanceRecord, ManagedInstanceDetailsRecord } from "./types";

export const instancesApi = {
  list() {
    return request<ManagedInstanceRecord[]>("/api/admin/instances");
  },
  get(agentId: string, containerId: string) {
    return request<ManagedInstanceDetailsRecord>(`/api/admin/instances/${agentId}/${containerId}`);
  },
  streamLogs(agentId: string, containerId: string, onChunk: (chunk: { data: string; stderr: boolean }) => void, onError: () => void) {
    const url = `/api/admin/instances/${agentId}/${containerId}/logs/stream`;
    const eventSource = new EventSource(url);
    
    eventSource.onmessage = (event) => {
      try {
        const chunk = JSON.parse(event.data);
        onChunk(chunk);
      } catch {
        onChunk({ data: event.data, stderr: false });
      }
    };
    
    eventSource.onerror = () => {
      onError();
    };
    
    return () => {
      eventSource.close();
    };
  },
};
