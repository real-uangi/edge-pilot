import { request } from "../../shared/lib/api-client";
import type { ServiceRecord, UpsertServiceInput, ObservabilityRecord, ServiceImageTagsInput, ServiceImageTagsPage } from "./types";

export const servicesApi = {
  list() {
    return request<ServiceRecord[]>("/api/admin/services");
  },
  get(id: string) {
    return request<ServiceRecord>(`/api/admin/services/${id}`);
  },
  listImageTags(input: ServiceImageTagsInput, signal?: AbortSignal) {
    return request<ServiceImageTagsPage>("/api/admin/services/image-tags", {
      method: "POST",
      body: JSON.stringify(input),
      signal,
    });
  },
  create(input: UpsertServiceInput) {
    return request<ServiceRecord>("/api/admin/services", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  update(id: string, input: UpsertServiceInput) {
    return request<ServiceRecord>(`/api/admin/services/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  delete(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/services/${id}`, {
      method: "DELETE",
    });
  },
  getObservability(id: string) {
    return request<ObservabilityRecord>(`/api/admin/services/${id}/observability`);
  },
};
