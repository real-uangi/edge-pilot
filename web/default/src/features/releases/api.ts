import { request } from "../../shared/lib/api-client";
import type { ReleaseRecord, ReleaseDetail } from "./types";

export const releasesApi = {
  list() {
    return request<ReleaseRecord[]>("/api/admin/releases");
  },
  get(id: string) {
    return request<ReleaseDetail>(`/api/admin/releases/${id}`);
  },
  start(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/start`, {
      method: "POST",
      body: "{}",
    });
  },
  skip(id: string) {
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
  setTraffic(id: string, percent: number) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/traffic`, {
      method: "POST",
      body: JSON.stringify({ percent }),
    });
  },
  rollback(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/rollback`, {
      method: "POST",
      body: "{}",
    });
  },
  retry(id: string) {
    return request<ReleaseRecord>(`/api/admin/releases/${id}/retry`, {
      method: "POST",
      body: "{}",
    });
  },
};
